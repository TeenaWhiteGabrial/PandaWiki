package usecase

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/chaitin/panda-wiki/utils"
)

func buildInternalStaticFileURL(endpoint, key string) string {
	if value := os.Getenv("ANYDOC_STATIC_FILE_BASE_URL"); value != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(value, "/"), key)
	}
	base := strings.TrimSpace(endpoint)
	base = strings.TrimPrefix(base, "http://")
	base = strings.TrimPrefix(base, "https://")
	return fmt.Sprintf("http://%s/static-file/%s", strings.TrimSuffix(base, "/"), key)
}

type FileUsecase struct {
	logger            *log.Logger
	s3Client          *s3.MinioClient
	imageOSSClient    *s3.ImageOSSClient
	config            *config.Config
	systemSettingRepo *pg.SystemSettingRepo
	httpClient        *http.Client
}

var officePreviewExtensions = map[string]struct{}{
	".doc":  {},
	".docx": {},
	".ppt":  {},
	".pptx": {},
	".xls":  {},
	".xlsx": {},
}

func NewFileUsecase(logger *log.Logger, s3Client *s3.MinioClient, imageOSSClient *s3.ImageOSSClient, config *config.Config, systemSettingRepo *pg.SystemSettingRepo) *FileUsecase {
	return &FileUsecase{
		s3Client:          s3Client,
		imageOSSClient:    imageOSSClient,
		logger:            logger.WithModule("usecase.file"),
		config:            config,
		systemSettingRepo: systemSettingRepo,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Prevent redirects to bypass SSRF checks
				return http.ErrUseLastResponse
			},
		},
	}
}

func (u *FileUsecase) UploadImage(ctx context.Context, kbID string, file *multipart.FileHeader) (string, string, error) {
	key, err := u.uploadValidatedFile(ctx, kbID, file, true)
	if err != nil {
		return "", "", err
	}
	if publicURL := u.s3Client.PublicURL(key); publicURL != "" {
		return key, publicURL, nil
	}
	return key, "/static-file/" + key, nil
}

func (u *FileUsecase) IsManagedImageURL(rawURL string) bool {
	if strings.HasPrefix(rawURL, "/static-file/") {
		return true
	}
	return u.s3Client.IsPublicURL(rawURL) || u.imageOSSClient.IsManagedURL(rawURL)
}

func (u *FileUsecase) UploadFileGetUrl(ctx context.Context, kbID string, file *multipart.FileHeader) (string, error) {
	key, err := u.UploadFile(ctx, kbID, file)
	if err != nil {
		return "", err
	}
	if publicURL := u.s3Client.PublicURL(key); publicURL != "" {
		return publicURL, nil
	}
	return buildInternalStaticFileURL(u.config.S3.Endpoint, key), nil
}

func (u *FileUsecase) SignFileURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	key, err := normalizeObjectKey(key)
	if err != nil {
		return "", err
	}
	if u.s3Client == nil {
		return "", fmt.Errorf("object storage is not configured")
	}
	return u.s3Client.SignURL(ctx, domain.Bucket, key, expires)
}

func (u *FileUsecase) SignFileDownloadURL(ctx context.Context, key, filename string, expires time.Duration) (string, error) {
	key, err := normalizeObjectKey(key)
	if err != nil {
		return "", err
	}
	if u.s3Client == nil {
		return "", fmt.Errorf("object storage is not configured")
	}
	return u.s3Client.SignDownloadURL(ctx, key, filename, expires)
}

func (u *FileUsecase) GetFileStorageInfo(ctx context.Context, key string, expires time.Duration) (string, string, string, string, bool, error) {
	key, err := normalizeObjectKey(key)
	if err != nil {
		return "", "", "", "", false, err
	}
	if u.s3Client == nil {
		return "", "", "", "", false, fmt.Errorf("object storage is not configured")
	}
	exists, err := u.s3Client.ObjectExists(ctx, key)
	if err != nil {
		return "", "", "", "", false, err
	}
	if !exists {
		return "", "", "", u.s3Client.Type(), false, nil
	}
	url, err := u.s3Client.SignURL(ctx, domain.Bucket, key, expires)
	if err != nil {
		return "", "", "", "", false, err
	}
	return url, u.s3Client.PublicURL(key), u.s3Client.ObjectLocation(key), u.s3Client.Type(), true, nil
}

func normalizeObjectKey(key string) (string, error) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "/static-file/"))
	key = strings.TrimLeft(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("file key is invalid")
	}
	return key, nil
}

func (u *FileUsecase) UploadFile(ctx context.Context, kbID string, file *multipart.FileHeader) (string, error) {
	return u.uploadValidatedFile(ctx, kbID, file, false)
}

func (u *FileUsecase) UploadAttachment(ctx context.Context, kbID string, file *multipart.FileHeader) (string, error) {
	return u.uploadValidatedFile(ctx, kbID, file, false)
}

func (u *FileUsecase) uploadValidatedFile(ctx context.Context, kbID string, file *multipart.FileHeader, imageOnly bool) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.Filename))

	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}
	contentType, err := validateUpload(file.Filename, src, file.Size, imageOnly)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%s/%s%s", kbID, uuid.New().String(), ext)

	size := file.Size

	opts := minio.PutObjectOptions{ContentType: contentType, UserMetadata: map[string]string{"originalname": file.Filename}}
	var resp minio.UploadInfo
	if imageOnly {
		// Display assets have a dedicated upload path and actual image decoding.
		// Do not expose normal document uploads through that path.
		filename = fmt.Sprintf("%s/assets/%s%s", kbID, uuid.New().String(), ext)
		resp, err = u.s3Client.PutDisplayImage(ctx, filename, src, size, opts)
	} else {
		resp, err = u.s3Client.PutObject(ctx, domain.Bucket, filename, src, size, opts)
	}

	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return resp.Key, nil
}

func (u *FileUsecase) UploadFileFromBytes(ctx context.Context, kbID string, filename string, fileBytes []byte) (string, error) {
	// Create a reader from the byte slice
	reader := bytes.NewReader(fileBytes)

	ext := strings.ToLower(filepath.Ext(filename))

	// Check denied extensions
	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}

	s3Filename := fmt.Sprintf("%s/%s%s", kbID, uuid.New().String(), ext)

	size := int64(len(fileBytes))

	contentType, err := validateUpload(filename, reader, size, false)
	if err != nil {
		return "", err
	}

	resp, err := u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		s3Filename,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"originalname": filename,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return resp.Key, nil
}

// UploadBytesAtKey stores a file under a caller-provided object key. It is
// used by structured imports where the object prefix must mirror the archive
// directory structure instead of using the legacy random key layout.
func (u *FileUsecase) UploadBytesAtKey(ctx context.Context, key, filename string, fileBytes []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}
	contentType, err := validateUpload(filename, bytes.NewReader(fileBytes), int64(len(fileBytes)), false)
	if err != nil {
		return "", err
	}
	if _, err := u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		strings.TrimLeft(key, "/"),
		bytes.NewReader(fileBytes),
		int64(len(fileBytes)),
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"originalname": filename,
			},
		},
	); err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	return strings.TrimLeft(key, "/"), nil
}

// PutObjectAtKey creates a small object such as a directory marker. Object
// storage directories are virtual prefixes, but markers make imported empty
// directories visible in storage consoles when needed.
func (u *FileUsecase) PutObjectAtKey(ctx context.Context, key, contentType string, fileBytes []byte) error {
	_, err := u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		strings.TrimLeft(key, "/"),
		bytes.NewReader(fileBytes),
		int64(len(fileBytes)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	return err
}

func (u *FileUsecase) ConvertOfficeToPDF(ctx context.Context, kbID string, filename string, fileBytes []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := officePreviewExtensions[ext]; !ok {
		return "", nil
	}

	previewBaseURL := strings.TrimSpace(u.config.Preview.BaseURL)
	if previewBaseURL == "" {
		return "", fmt.Errorf("preview service is not configured")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		return "", fmt.Errorf("create preview form file failed: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return "", fmt.Errorf("write preview form file failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close preview form writer failed: %w", err)
	}

	previewURL, err := url.JoinPath(previewBaseURL, "/forms/libreoffice/convert")
	if err != nil {
		return "", fmt.Errorf("build preview url failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, previewURL, body)
	if err != nil {
		return "", fmt.Errorf("create preview request failed: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if u.config.Preview.Username == "" || u.config.Preview.Password == "" {
		return "", fmt.Errorf("preview service credentials are not configured")
	}
	req.SetBasicAuth(u.config.Preview.Username, u.config.Preview.Password)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request preview service failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("preview service failed: %s", strings.TrimSpace(string(respBody)))
	}

	pdfBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read preview pdf failed: %w", err)
	}

	previewFilename := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".preview.pdf"
	return u.UploadFileFromBytes(ctx, kbID, previewFilename, pdfBytes)
}

func (u *FileUsecase) GeneratePreviewByKey(ctx context.Context, kbID string, fileKey string, fileName string) (string, error) {
	object, err := u.s3Client.GetObject(ctx, domain.Bucket, fileKey, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("get source file failed: %w", err)
	}
	defer object.Close()

	fileBytes, err := io.ReadAll(object)
	if err != nil {
		return "", fmt.Errorf("read source file failed: %w", err)
	}

	return u.ConvertOfficeToPDF(ctx, kbID, fileName, fileBytes)
}

func (u *FileUsecase) UploadFileFromReader(
	ctx context.Context,
	kbID string,
	filename string,
	reader io.Reader,
	size int64, // 必须提供对象大小
) (string, error) {
	// 生成唯一文件名
	ext := strings.ToLower(filepath.Ext(filename))

	// Check denied extensions
	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}

	s3Filename := fmt.Sprintf("%s/%s%s", kbID, uuid.New().String(), ext)

	staged, err := stageUpload(reader, size)
	if err != nil {
		return "", err
	}
	defer func() { staged.Close(); os.Remove(staged.Name()) }()
	contentType, err := validateUpload(filename, staged, size, false)
	if err != nil {
		return "", err
	}

	// 上传到 S3
	_, err = u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		s3Filename,
		staged,
		size, // 必须提供对象大小
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"originalname": filename,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("S3 upload failed: %w", err)
	}

	return s3Filename, nil
}

func (u *FileUsecase) PresignObjectUpload(ctx context.Context, key string, expires time.Duration) (string, error) {
	return u.s3Client.PresignedPutURL(ctx, key, expires)
}

func (u *FileUsecase) StatObject(ctx context.Context, key string) (minio.ObjectInfo, error) {
	return u.s3Client.StatObject(ctx, key)
}

// DownloadObjectToTemp is used only during publication. The source remains in
// OSS during upload and is staged locally temporarily while archive.NewReader
// walks its central directory.
func (u *FileUsecase) DownloadObjectToTemp(ctx context.Context, key string) (*os.File, int64, error) {
	object, err := u.s3Client.GetObject(ctx, domain.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, err
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		return nil, 0, err
	}
	tmp, err := os.CreateTemp("", "pandawiki-zip-*")
	if err != nil {
		return nil, 0, err
	}
	if _, err = io.Copy(tmp, object); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	return tmp, info.Size, nil
}

func (u *FileUsecase) AnyDocUploadFile(ctx context.Context, file *multipart.FileHeader, path string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.Filename))

	// Check denied extensions
	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}

	size := file.Size

	contentType, err := validateUpload(file.Filename, src, size, false)
	if err != nil {
		return "", err
	}

	resp, err := u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		path,
		src,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"originalname": file.Filename,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return resp.Key, nil
}

func (u *FileUsecase) UploadFileByUrl(ctx context.Context, kbID string, fileURL string) (string, error) {
	// Validate URL to prevent SSRF attacks
	if err := utils.ValidateURLForSSRF(fileURL); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Handle redirects manually to re-validate each redirect target
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return "", fmt.Errorf("redirects are not allowed for security reasons")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file, status: %d", resp.StatusCode)
	}

	const maxRemoteFileSize = 50 * 1024 * 1024 // 50MB
	lr := io.LimitReader(resp.Body, maxRemoteFileSize+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if len(data) > maxRemoteFileSize {
		return "", fmt.Errorf("failed to read response body: file size exceeds limit of %d bytes", maxRemoteFileSize)
	}

	urlPath := fileURL
	if idx := strings.Index(urlPath, "?"); idx != -1 {
		urlPath = urlPath[:idx]
	}
	ext := strings.ToLower(filepath.Ext(urlPath))

	if err := u.checkDeniedExtension(ctx, ext); err != nil {
		return "", err
	}

	s3Filename := fmt.Sprintf("%s/%s%s", kbID, uuid.New().String(), ext)

	contentType, err := validateUpload("download"+ext, bytes.NewReader(data), int64(len(data)), false)
	if err != nil {
		return "", err
	}

	putResp, err := u.s3Client.PutObject(
		ctx,
		domain.Bucket,
		s3Filename,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	return putResp.Key, nil
}

// checkDeniedExtension checks if the file extension is in the denied list
func (u *FileUsecase) checkDeniedExtension(ctx context.Context, ext string) error {
	// Remove leading dot from extension
	ext = strings.TrimPrefix(ext, ".")
	if _, ok := uploadTypes["."+ext]; !ok {
		return fmt.Errorf("不支持上传此文件类型：%s", ext)
	}
	if u.systemSettingRepo == nil {
		return nil
	}

	// Get denied extensions from system settings
	setting, err := u.systemSettingRepo.GetSystemSetting(ctx, consts.SystemSettingUpload)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		u.logger.Error("failed to get upload denied extensions setting", "error", err)
		return fmt.Errorf("无法读取上传策略，已拒绝上传")
	}

	var deniedSetting domain.UploadDeniedExtensionsSetting
	if err := json.Unmarshal(setting.Value, &deniedSetting); err != nil {
		u.logger.Error("failed to unmarshal denied extensions setting", "error", err)
		return fmt.Errorf("上传策略无效，已拒绝上传")
	}

	// Check if extension is denied
	for _, deniedExt := range deniedSetting.DeniedExtensions {
		if strings.EqualFold(ext, deniedExt) {
			return fmt.Errorf("file extension '.%s' is not allowed for upload", ext)
		}
	}

	return nil
}
