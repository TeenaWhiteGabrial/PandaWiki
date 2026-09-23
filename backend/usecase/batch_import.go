package usecase

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	v1 "github.com/chaitin/panda-wiki/api/crawler/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
)

const (
	batchImportMaxArchiveSize  = int64(20 * 1024 * 1024 * 1024)
	batchImportMaxExpandedSize = int64(100 * 1024 * 1024 * 1024)
	batchImportMaxFileSize     = int64(100 * 1024 * 1024)
	batchImportMaxFiles        = 5000
	batchImportExtractTimeout  = 30 * time.Second
)

type BatchImportItem struct {
	Path      string `json:"path"`
	NodeID    string `json:"node_id,omitempty"`
	FileKey   string `json:"file_key,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	ContentOK bool   `json:"content_ok"`
}

type BatchImportResult struct {
	BatchID  string            `json:"batch_id"`
	Total    int               `json:"total"`
	Imported int               `json:"imported"`
	Failed   int               `json:"failed"`
	Items    []BatchImportItem `json:"items"`
}

type ZipUploadTicket struct {
	UploadURL string `json:"upload_url"`
	FileKey   string `json:"file_key"`
	ExpiresIn int64  `json:"expires_in"`
}

type ZipUploadCommit struct {
	KBID     string
	NavID    string
	ParentID string
	UserID   string
	FileKey  string
	Name     string
	Size     int64
}

type ArchiveProgress struct {
	TotalFiles     int
	ProcessedFiles int
	TotalBytes     int64
	ProcessedBytes int64
}

// ExpandArchives expands uploaded ZIP placeholder nodes at publication time.
// The archive is downloaded to a temporary file only for this operation and is
// removed immediately afterwards.
func (u *BatchImportUsecase) ExpandArchives(ctx context.Context, kbID string, nodeIDs []string, userID string, parseContent bool) ([]string, error) {
	return u.ExpandArchivesWithProgress(ctx, kbID, nodeIDs, userID, parseContent, nil)
}

func (u *BatchImportUsecase) ExpandArchivesWithProgress(ctx context.Context, kbID string, nodeIDs []string, userID string, parseContent bool, onProgress func(ArchiveProgress)) ([]string, error) {
	nodes, err := u.nodeRepo.GetNodesByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("read archive nodes failed: %w", err)
	}
	created := make([]string, 0)
	for _, nodeID := range nodeIDs {
		node := nodes[nodeID]
		if node == nil || !node.Meta.ArchivePending || node.Meta.FileKey == "" {
			continue
		}
		tmp, _, err := u.fileUsecase.DownloadObjectToTemp(ctx, node.Meta.FileKey)
		if err != nil {
			return nil, fmt.Errorf("download archive %q failed: %w", node.Name, err)
		}
		func() {
			defer func() {
				tmp.Close()
				_ = os.Remove(tmp.Name())
			}()
			boundary := "pandawiki-" + uuid.NewString()
			prefix := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: application/zip\r\n\r\n", boundary, path.Base(node.Name))
			suffix := fmt.Sprintf("\r\n--%s--\r\n", boundary)
			reader := multipart.NewReader(io.MultiReader(strings.NewReader(prefix), tmp, strings.NewReader(suffix)), boundary)
			form, formErr := reader.ReadForm(32 << 20)
			if formErr != nil {
				err = fmt.Errorf("read archive upload failed: %w", formErr)
				return
			}
			defer form.RemoveAll()
			files := form.File["file"]
			if len(files) != 1 {
				err = fmt.Errorf("archive upload part is missing")
				return
			}
			result, importErr := u.importZip(ctx, kbID, node.NavId, node.ParentID, userID, files[0], parseContent, onProgress)
			if importErr != nil {
				err = importErr
				return
			}
			if result.Imported == 0 {
				err = fmt.Errorf("archive %q contains no importable files: %s", node.Name, summarizeImportFailures(result))
				return
			}
			// A valid archive may contain auxiliary files such as .DS_Store or
			// unsupported files. Do not discard valid documents because of them.
			for _, item := range result.Items {
				if item.NodeID != "" {
					created = append(created, item.NodeID)
				}
			}
			_, err = u.nodeRepo.Delete(ctx, kbID, []string{node.ID})
		}()
		if err != nil {
			return nil, err
		}
	}
	return created, nil
}

type BatchImportUsecase struct {
	logger      *log.Logger
	fileUsecase *FileUsecase
	crawler     *CrawlerUsecase
	nodeRepo    *pg.NodeRepository
}

func NewBatchImportUsecase(
	logger *log.Logger,
	fileUsecase *FileUsecase,
	crawler *CrawlerUsecase,
	nodeRepo *pg.NodeRepository,
) *BatchImportUsecase {
	return &BatchImportUsecase{
		logger:      logger.WithModule("usecase.batch-import"),
		fileUsecase: fileUsecase,
		crawler:     crawler,
		nodeRepo:    nodeRepo,
	}
}

// PresignUpload creates a direct-to-OSS upload ticket. No ZIP bytes pass
// through the API process, so the server's temporary directory is not used by
// large uploads.
func (u *BatchImportUsecase) PresignUpload(ctx context.Context, kbID, filename string, size int64) (*ZipUploadTicket, error) {
	if kbID == "" || strings.ToLower(path.Ext(filename)) != ".zip" {
		return nil, fmt.Errorf("only .zip archives are supported")
	}
	if size <= 0 {
		return nil, fmt.Errorf("zip file is empty")
	}
	if size > 20*1024*1024*1024 {
		return nil, fmt.Errorf("zip file exceeds 20 GB")
	}
	key := fmt.Sprintf("%s/imports/%s/%s.zip", kbID, uuid.NewString(), uuid.NewString())
	url, err := u.fileUsecase.PresignObjectUpload(ctx, key, 30*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("create upload ticket failed: %w", err)
	}
	return &ZipUploadTicket{UploadURL: url, FileKey: key, ExpiresIn: 1800}, nil
}

// CommitUpload only creates a pending archive node. Parsing is deliberately
// deferred until publication.
func (u *BatchImportUsecase) CommitUpload(ctx context.Context, req ZipUploadCommit) (string, error) {
	if req.FileKey == "" || req.Name == "" || req.KBID == "" || req.NavID == "" {
		return "", fmt.Errorf("invalid zip upload commit")
	}
	if !strings.HasPrefix(strings.TrimLeft(req.FileKey, "/"), req.KBID+"/imports/") || batchImportExt(req.FileKey) != ".zip" {
		return "", fmt.Errorf("invalid zip object key")
	}
	info, err := u.fileUsecase.StatObject(ctx, req.FileKey)
	if err != nil {
		return "", fmt.Errorf("uploaded zip not found: %w", err)
	}
	if info.Size <= 0 || info.Size > batchImportMaxArchiveSize {
		return "", fmt.Errorf("uploaded zip exceeds 20 GB")
	}
	if req.Size > 0 && info.Size != req.Size {
		return "", fmt.Errorf("uploaded zip size mismatch")
	}
	return u.nodeRepo.Create(ctx, &domain.CreateNodeReq{
		KBID: req.KBID, NavId: req.NavID, ParentID: req.ParentID,
		Type: domain.NodeTypeDocument, Name: path.Base(req.Name),
		Meta: &domain.NodeMeta{
			ContentType: "file", FileKey: req.FileKey, FileName: path.Base(req.Name),
			FileType: "zip", FileReadonly: true, ArchivePending: true,
		},
	}, req.UserID)
}

type batchImportFolder struct {
	id string
}

func cleanBatchImportPath(raw string) (string, error) {
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("invalid archive path")
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive path escapes import root")
	}
	return strings.TrimPrefix(clean, "./"), nil
}

func batchImportExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(name[idx:])
}

func isArchiveMetadataFile(name string) bool {
	clean := strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	base := path.Base(clean)
	return base == ".DS_Store" || base == "Thumbs.db" ||
		strings.HasPrefix(clean, "__MACOSX/") || strings.HasPrefix(clean, "._") ||
		strings.HasPrefix(base, "._")
}

func summarizeImportFailures(result *BatchImportResult) string {
	const maxDetails = 3
	details := make([]string, 0, maxDetails)
	for _, item := range result.Items {
		if item.Status != "failed" {
			continue
		}
		detail := item.Path
		if item.Message != "" {
			detail += ": " + item.Message
		}
		details = append(details, detail)
		if len(details) == maxDetails {
			break
		}
	}
	if len(details) == 0 {
		return "未发现支持的文档文件"
	}
	return strings.Join(details, "; ")
}

func (u *BatchImportUsecase) ImportZip(
	ctx context.Context,
	kbID, navID, parentID, userID string,
	file *multipart.FileHeader,
	parseContent bool,
) (*BatchImportResult, error) {
	return u.importZip(ctx, kbID, navID, parentID, userID, file, parseContent, nil)
}

func (u *BatchImportUsecase) importZip(
	ctx context.Context,
	kbID, navID, parentID, userID string,
	file *multipart.FileHeader,
	parseContent bool,
	onProgress func(ArchiveProgress),
) (*BatchImportResult, error) {
	if file == nil || file.Size <= 0 {
		return nil, fmt.Errorf("zip file is empty")
	}
	if file.Size > batchImportMaxArchiveSize {
		return nil, fmt.Errorf("zip file exceeds %d MB", batchImportMaxArchiveSize/(1024*1024))
	}
	if strings.ToLower(path.Ext(file.Filename)) != ".zip" {
		return nil, fmt.Errorf("only .zip archives are supported")
	}
	archiveFile, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open zip failed: %w", err)
	}
	defer archiveFile.Close()
	zipReader, err := zip.NewReader(archiveFile, file.Size)
	if err != nil {
		return nil, fmt.Errorf("read zip failed: %w", err)
	}
	if len(zipReader.File) > batchImportMaxFiles {
		return nil, fmt.Errorf("zip contains too many files")
	}

	result := &BatchImportResult{BatchID: uuid.NewString(), Items: make([]BatchImportItem, 0)}
	totalFiles := 0
	var totalBytes int64
	for _, entry := range zipReader.File {
		if entry.FileInfo().IsDir() || strings.HasSuffix(entry.Name, "/") {
			continue
		}
		totalFiles++
		totalBytes += int64(entry.UncompressedSize64)
	}
	processedFiles := 0
	var processedBytes int64
	if onProgress != nil {
		onProgress(ArchiveProgress{TotalFiles: totalFiles, TotalBytes: totalBytes})
	}
	// Keep extracted objects under the same KB namespace as the uploaded ZIP.
	// The relative path after this prefix mirrors the archive directory tree.
	rootPrefix := fmt.Sprintf("%s/imports/%s", kbID, result.BatchID)
	folders := map[string]batchImportFolder{"": {id: parentID}}
	seenFiles := map[string]struct{}{}
	var expandedSize int64

	var ensureFolder func(folderPath string) (string, error)
	ensureFolder = func(folderPath string) (string, error) {
		if existing, ok := folders[folderPath]; ok {
			return existing.id, nil
		}
		parentPath := path.Dir(folderPath)
		if parentPath == "." {
			parentPath = ""
		}
		parentNodeID, err := ensureFolder(parentPath)
		if err != nil {
			return "", err
		}
		name := path.Base(folderPath)
		nodeID, err := u.nodeRepo.Create(ctx, &domain.CreateNodeReq{
			KBID: kbID, NavId: navID, ParentID: parentNodeID,
			Type: domain.NodeTypeFolder, Name: name,
		}, userID)
		if err != nil {
			return "", fmt.Errorf("create folder %q failed: %w", folderPath, err)
		}
		folders[folderPath] = batchImportFolder{id: nodeID}
		markerKey := rootPrefix + "/" + folderPath + "/.keep"
		if err := u.fileUsecase.PutObjectAtKey(ctx, markerKey, "application/x-directory", nil); err != nil {
			return "", fmt.Errorf("create OSS folder marker %q failed: %w", folderPath, err)
		}
		return nodeID, nil
	}

	for _, entry := range zipReader.File {
		relPath, pathErr := cleanBatchImportPath(entry.Name)
		if pathErr != nil {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: entry.Name, Status: "failed", Message: pathErr.Error()})
			continue
		}
		isDir := entry.FileInfo().IsDir() || strings.HasSuffix(entry.Name, "/")
		if isDir {
			if _, err := ensureFolder(strings.TrimSuffix(relPath, "/")); err != nil {
				return nil, err
			}
			continue
		}
		if isArchiveMetadataFile(relPath) {
			continue
		}
		processedFiles++
		processedBytes += int64(entry.UncompressedSize64)
		if onProgress != nil {
			onProgress(ArchiveProgress{TotalFiles: totalFiles, ProcessedFiles: processedFiles, TotalBytes: totalBytes, ProcessedBytes: processedBytes})
		}
		if _, exists := seenFiles[relPath]; exists {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: "duplicate path in archive"})
			continue
		}
		seenFiles[relPath] = struct{}{}
		if entry.UncompressedSize64 > uint64(batchImportMaxFileSize) || expandedSize+int64(entry.UncompressedSize64) > batchImportMaxExpandedSize {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: "expanded file size exceeds limit"})
			continue
		}
		expandedSize += int64(entry.UncompressedSize64)
		contentReader, err := entry.Open()
		if err != nil {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: err.Error()})
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(contentReader, batchImportMaxFileSize+1))
		contentReader.Close()
		if readErr != nil || int64(len(content)) > batchImportMaxFileSize {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: "read file failed or file is too large"})
			continue
		}

		folderPath := path.Dir(relPath)
		if folderPath == "." {
			folderPath = ""
		}
		parentNodeID, err := ensureFolder(folderPath)
		if err != nil {
			return nil, err
		}
		fileKey := rootPrefix + "/" + relPath
		if _, err := u.fileUsecase.UploadBytesAtKey(ctx, fileKey, path.Base(relPath), content); err != nil {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, FileKey: fileKey, Status: "failed", Message: err.Error()})
			continue
		}

		ext := batchImportExt(relPath)

		markdown := ""
		contentOK := false
		contentMessage := ""
		if parseContent {
			markdown, contentOK, contentMessage = u.extractFileContent(ctx, kbID, fileKey, path.Base(relPath))
		}

		nodeID, createErr := u.nodeRepo.Create(ctx, &domain.CreateNodeReq{
			KBID: kbID, NavId: navID, ParentID: parentNodeID, Type: domain.NodeTypeDocument,
			Name: path.Base(relPath), Content: markdown,
			Meta: &domain.NodeMeta{ContentType: "file", FileKey: fileKey, FileName: path.Base(relPath), FileType: strings.TrimPrefix(ext, "."), FileReadonly: true},
		}, userID)
		if createErr != nil {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, FileKey: fileKey, Status: "failed", Message: createErr.Error()})
			continue
		}
		result.Imported++
		if !contentOK && contentMessage == "" {
			contentMessage = "原文件已导入，但内容提取失败，暂不可参与问答"
		}
		result.Items = append(result.Items, BatchImportItem{Path: relPath, NodeID: nodeID, FileKey: fileKey, Status: "imported", ContentOK: contentOK, Message: contentMessage})
	}
	result.Total = len(result.Items)
	return result, nil
}

func (u *BatchImportUsecase) extractFileContent(ctx context.Context, kbID, fileKey, filename string) (string, bool, string) {
	extractCtx, cancelExtract := context.WithTimeout(ctx, batchImportExtractTimeout)
	defer cancelExtract()
	parseResp, parseErr := u.crawler.ParseUrl(extractCtx, &v1.CrawlerParseReq{
		Key: fileKey, KbID: kbID, CrawlerSource: consts.CrawlerSourceFile, Filename: filename,
	})
	if parseErr != nil {
		return "", false, "内容解析失败：" + parseErr.Error()
	}
	if parseResp == nil || parseResp.Docs.Value.ID == "" {
		return "", false, "内容解析未返回文档"
	}
	exportResp, exportErr := u.crawler.ExportDoc(extractCtx, &v1.CrawlerExportReq{KbID: kbID, ID: parseResp.ID, DocID: parseResp.Docs.Value.ID})
	if exportErr != nil {
		return "", false, "内容转换失败：" + exportErr.Error()
	}
	if exportResp.TaskId == "" {
		return "", false, "内容转换未生成任务"
	}
	scrapeResp, scrapeErr := u.crawler.ScrapeGetResult(extractCtx, exportResp.TaskId)
	if scrapeErr != nil {
		return "", false, "内容提取失败：" + scrapeErr.Error()
	}
	if scrapeResp == nil {
		return "", false, "内容提取未返回结果"
	}
	return scrapeResp.Content, true, ""
}
