package usecase

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
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
	batchImportMaxArchiveSize  = int64(512 * 1024 * 1024)
	batchImportMaxExpandedSize = int64(2 * 1024 * 1024 * 1024)
	batchImportMaxFileSize     = int64(100 * 1024 * 1024)
	batchImportMaxFiles        = 5000
	batchImportExtractTimeout  = 30 * time.Second
)

var batchImportExtensions = map[string]struct{}{
	".md": {}, ".markdown": {}, ".txt": {}, ".html": {}, ".htm": {},
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {},
	".ppt": {}, ".pptx": {},
}

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

func (u *BatchImportUsecase) ImportZip(
	ctx context.Context,
	kbID, navID, parentID, userID string,
	file *multipart.FileHeader,
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
	rootPrefix := "imports/" + result.BatchID
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
		if _, exists := seenFiles[relPath]; exists {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: "duplicate path in archive"})
			continue
		}
		seenFiles[relPath] = struct{}{}
		if _, ok := batchImportExtensions[batchImportExt(relPath)]; !ok {
			result.Failed++
			result.Items = append(result.Items, BatchImportItem{Path: relPath, Status: "failed", Message: "unsupported file type"})
			continue
		}
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

		previewKey := ""
		ext := batchImportExt(relPath)
		if ext == ".pdf" {
			previewKey = fileKey
		} else if preview, previewErr := u.fileUsecase.ConvertOfficeToPDF(ctx, kbID, path.Base(relPath), content); previewErr == nil {
			previewKey = preview
		}

		markdown := ""
		contentOK := false
		contentMessage := ""
		extractCtx, cancelExtract := context.WithTimeout(ctx, batchImportExtractTimeout)
		parseResp, parseErr := u.crawler.ParseUrl(extractCtx, &v1.CrawlerParseReq{
			Key: fileKey, KbID: kbID, CrawlerSource: consts.CrawlerSourceFile, Filename: path.Base(relPath),
		})
		if parseErr != nil {
			contentMessage = "内容解析失败：" + parseErr.Error()
		} else if parseResp == nil || parseResp.Docs.Value.ID == "" {
			contentMessage = "内容解析未返回文档"
		} else {
			exportResp, exportErr := u.crawler.ExportDoc(extractCtx, &v1.CrawlerExportReq{KbID: kbID, ID: parseResp.ID, DocID: parseResp.Docs.Value.ID})
			if exportErr != nil {
				contentMessage = "内容转换失败：" + exportErr.Error()
			} else if exportResp.TaskId == "" {
				contentMessage = "内容转换未生成任务"
			} else if scrapeResp, scrapeErr := u.crawler.ScrapeGetResult(extractCtx, exportResp.TaskId); scrapeErr == nil && scrapeResp != nil {
				markdown, contentOK = scrapeResp.Content, true
			} else if scrapeErr != nil {
				contentMessage = "内容提取失败：" + scrapeErr.Error()
			} else {
				contentMessage = "内容提取未返回结果"
			}
		}
		cancelExtract()

		nodeID, createErr := u.nodeRepo.Create(ctx, &domain.CreateNodeReq{
			KBID: kbID, NavId: navID, ParentID: parentNodeID, Type: domain.NodeTypeDocument,
			Name: path.Base(relPath), Content: markdown,
			Meta: &domain.NodeMeta{ContentType: "file", FileKey: fileKey, FileName: path.Base(relPath), FileType: strings.TrimPrefix(ext, "."), PreviewKey: previewKey, FileReadonly: true},
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
