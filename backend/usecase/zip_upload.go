package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const zipUploadPartSize int64 = 64 * 1024 * 1024

type ZipUploadUsecase struct {
	repo        *pg.ZipUploadRepository
	nodeRepo    *pg.NodeRepository
	objectStore *s3.MinioClient
	logger      *log.Logger
}

func NewZipUploadUsecase(repo *pg.ZipUploadRepository, nodeRepo *pg.NodeRepository, objectStore *s3.MinioClient, logger *log.Logger) *ZipUploadUsecase {
	return &ZipUploadUsecase{repo: repo, nodeRepo: nodeRepo, objectStore: objectStore, logger: logger.WithModule("usecase.zip-upload")}
}

type ZipUploadCreate struct {
	KBID, NavID, ParentID, UserID, Filename string
	Size                                    int64
}

func (u *ZipUploadUsecase) Create(ctx context.Context, req ZipUploadCreate) (*domain.ZipUploadTask, error) {
	if req.KBID == "" || req.NavID == "" || req.UserID == "" || !strings.EqualFold(path.Ext(req.Filename), ".zip") {
		return nil, fmt.Errorf("only .zip archives are supported")
	}
	if req.Size <= 0 || req.Size > 20*1024*1024*1024 {
		return nil, fmt.Errorf("zip file size must be between 1 byte and 20 GB")
	}
	total := int((req.Size + zipUploadPartSize - 1) / zipUploadPartSize)
	key := fmt.Sprintf("%s/imports/%s/%s.zip", req.KBID, uuid.NewString(), uuid.NewString())
	uploadID, err := u.objectStore.MultipartUpload(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("start multipart upload: %w", err)
	}
	t := &domain.ZipUploadTask{ID: uuid.NewString(), KBID: req.KBID, NavID: req.NavID, ParentID: req.ParentID, CreatedBy: req.UserID, Filename: path.Base(req.Filename), FileKey: key, Size: req.Size, PartSize: zipUploadPartSize, TotalParts: total, UploadID: uploadID, UploadedParts: "{}", Status: domain.ZipUploadUploading}
	if err := u.repo.Create(ctx, t); err != nil {
		_ = u.objectStore.AbortMultipartUpload(ctx, key, uploadID)
		return nil, err
	}
	return t, nil
}

func (u *ZipUploadUsecase) Get(ctx context.Context, id, userID string) (*domain.ZipUploadTask, error) {
	t, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.CreatedBy != userID {
		return nil, fmt.Errorf("upload task access denied")
	}
	if t.Status != domain.ZipUploadCompleted {
		if err := u.refreshParts(ctx, t); err != nil {
			u.logger.Warn("refresh multipart parts failed", log.String("task_id", t.ID), log.Error(err))
		}
	}
	return t, nil
}

func (u *ZipUploadUsecase) refreshParts(ctx context.Context, task *domain.ZipUploadTask) error {
	parts, err := u.objectStore.ListMultipartParts(ctx, task.FileKey, task.UploadID)
	if err != nil {
		return err
	}
	known := map[string]string{}
	_ = json.Unmarshal([]byte(task.UploadedParts), &known)
	if known == nil {
		known = map[string]string{}
	}
	var uploadedBytes int64
	for _, part := range parts {
		known[fmt.Sprintf("%d", part.PartNumber)] = part.ETag
		uploadedBytes += part.Size
	}
	b, _ := json.Marshal(known)
	changed := task.UploadedParts != string(b) || task.UploadedBytes != uploadedBytes
	task.UploadedParts = string(b)
	task.UploadedBytes = uploadedBytes
	if changed {
		return u.repo.Update(ctx, task)
	}
	return nil
}
func (u *ZipUploadUsecase) List(ctx context.Context, kbID, userID string) ([]*domain.ZipUploadTask, error) {
	tasks, err := u.repo.List(ctx, kbID, userID)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.Status != domain.ZipUploadCompleted {
			if refreshErr := u.refreshParts(ctx, task); refreshErr != nil {
				u.logger.Warn("refresh multipart parts failed", log.String("task_id", task.ID), log.Error(refreshErr))
			}
		}
	}
	return tasks, nil
}

func (u *ZipUploadUsecase) PresignPart(ctx context.Context, id, userID string, part int) (string, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return "", err
	}
	if part < 1 || part > t.TotalParts {
		return "", fmt.Errorf("invalid upload part")
	}
	return u.objectStore.PresignedUploadPartURL(ctx, t.FileKey, t.UploadID, part, 30*time.Minute)
}

func (u *ZipUploadUsecase) UploadPart(ctx context.Context, id, userID string, part int, reader io.Reader, size int64) (string, int64, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return "", 0, err
	}
	if part < 1 || part > t.TotalParts {
		return "", 0, fmt.Errorf("invalid upload part")
	}
	expectedSize := t.PartSize
	if part == t.TotalParts {
		expectedSize = t.Size - int64(part-1)*t.PartSize
	}
	if size >= 0 && size != expectedSize {
		return "", 0, fmt.Errorf("upload part size mismatch: got %d, expected %d", size, expectedSize)
	}
	if size < 0 {
		size = expectedSize
	}
	objectPart, err := u.objectStore.UploadMultipartPart(ctx, t.FileKey, t.UploadID, part, reader, size)
	if err != nil {
		return "", 0, err
	}
	return objectPart.ETag, size, nil
}

func (u *ZipUploadUsecase) CompletePart(ctx context.Context, id, userID string, part int, etag string, size int64) (*domain.ZipUploadTask, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if part < 1 || part > t.TotalParts || etag == "" {
		return nil, fmt.Errorf("invalid uploaded part")
	}
	parts := map[string]string{}
	_ = json.Unmarshal([]byte(t.UploadedParts), &parts)
	if parts == nil {
		parts = map[string]string{}
	}
	key := fmt.Sprintf("%d", part)
	if _, exists := parts[key]; !exists {
		t.UploadedBytes += size
	}
	parts[key] = etag
	b, _ := json.Marshal(parts)
	t.UploadedParts = string(b)
	t.Status = domain.ZipUploadUploading
	if err := u.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *ZipUploadUsecase) Complete(ctx context.Context, id, userID string) (*domain.ZipUploadTask, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if t.Status == domain.ZipUploadCompleted || t.Status == domain.ZipUploadProcessing {
		return t, nil
	}
	parts := map[string]string{}
	_ = json.Unmarshal([]byte(t.UploadedParts), &parts)
	if len(parts) != t.TotalParts {
		return nil, fmt.Errorf("upload is incomplete: %d/%d parts", len(parts), t.TotalParts)
	}
	complete := make([]minio.CompletePart, 0, t.TotalParts)
	for i := 1; i <= t.TotalParts; i++ {
		complete = append(complete, minio.CompletePart{PartNumber: i, ETag: parts[fmt.Sprintf("%d", i)]})
	}
	if err := u.objectStore.CompleteMultipartUpload(ctx, t.FileKey, t.UploadID, complete); err != nil {
		t.Status = domain.ZipUploadFailed
		t.ErrorMessage = err.Error()
		_ = u.repo.Update(ctx, t)
		return nil, err
	}
	info, err := u.objectStore.StatObject(ctx, t.FileKey)
	if err != nil || info.Size != t.Size {
		if err == nil {
			err = fmt.Errorf("uploaded zip size mismatch: got %d, expected %d", info.Size, t.Size)
		}
		t.Status = domain.ZipUploadFailed
		t.ErrorMessage = err.Error()
		_ = u.repo.Update(ctx, t)
		return nil, err
	}
	nodeID, err := u.nodeRepo.Create(ctx, &domain.CreateNodeReq{KBID: t.KBID, NavId: t.NavID, ParentID: t.ParentID, Type: domain.NodeTypeDocument, Name: t.Filename, Meta: &domain.NodeMeta{ContentType: "file", FileKey: t.FileKey, FileName: t.Filename, FileType: "zip", FileReadonly: true, ArchivePending: true}}, t.CreatedBy)
	if err != nil {
		t.Status = domain.ZipUploadFailed
		t.ErrorMessage = err.Error()
		_ = u.repo.Update(ctx, t)
		return nil, err
	}
	// The archive is uploaded at this point, but extraction is deliberately
	// asynchronous. The task becomes completed only after all archive entries
	// have been materialized as nodes and OSS objects.
	t.Status = domain.ZipUploadProcessing
	t.TotalFiles = 0
	t.ProcessedFiles = 0
	t.TotalBytes = 0
	t.ProcessedBytes = 0
	t.NodeID = nodeID
	t.UploadedBytes = t.Size
	if err := u.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *ZipUploadUsecase) UpdateProcessingProgress(ctx context.Context, id string, totalFiles, processedFiles int, totalBytes, processedBytes int64) error {
	t, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Status == domain.ZipUploadCompleted || t.Status == domain.ZipUploadFailed {
		return nil
	}
	t.Status = domain.ZipUploadProcessing
	t.TotalFiles = totalFiles
	t.ProcessedFiles = processedFiles
	t.TotalBytes = totalBytes
	t.ProcessedBytes = processedBytes
	return u.repo.Update(ctx, t)
}

func (u *ZipUploadUsecase) CompleteProcessing(ctx context.Context, id string) error {
	t, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Status == domain.ZipUploadFailed {
		return nil
	}
	t.Status = domain.ZipUploadCompleted
	t.ProcessedFiles = t.TotalFiles
	t.ProcessedBytes = t.TotalBytes
	return u.repo.Update(ctx, t)
}

func (u *ZipUploadUsecase) Retry(ctx context.Context, id, userID string) (*domain.ZipUploadTask, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if t.Status != domain.ZipUploadFailed {
		return t, nil
	}
	t.Status = domain.ZipUploadUploading
	t.ErrorMessage = ""
	t.RetryCount++
	if err := u.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *ZipUploadUsecase) Fail(ctx context.Context, id, userID, reason string) (*domain.ZipUploadTask, error) {
	t, err := u.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if t.Status == domain.ZipUploadCompleted {
		return t, nil
	}
	t.Status = domain.ZipUploadFailed
	t.ErrorMessage = reason
	if err := u.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
