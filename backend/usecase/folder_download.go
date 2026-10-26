package usecase

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/s3"
)

const (
	folderDownloadMaxFiles = 10000
	folderDownloadMaxBytes = int64(20 << 30)
	folderDownloadURLTTL   = time.Hour
	folderDownloadKeepFor  = 24 * time.Hour
)

type FolderDownloadUsecase struct {
	taskRepo *pg.FolderDownloadRepository
	nodeRepo *pg.NodeRepository
	navRepo  *pg.NavRepository
	s3Client *s3.MinioClient
	logger   *log.Logger
}

func NewFolderDownloadUsecase(
	taskRepo *pg.FolderDownloadRepository,
	nodeRepo *pg.NodeRepository,
	navRepo *pg.NavRepository,
	s3Client *s3.MinioClient,
	logger *log.Logger,
) *FolderDownloadUsecase {
	return &FolderDownloadUsecase{
		taskRepo: taskRepo,
		nodeRepo: nodeRepo,
		navRepo:  navRepo,
		s3Client: s3Client,
		logger:   logger.WithModule("usecase.folder_download"),
	}
}

const virtualNavRootPrefix = "nav:"

func VirtualNavRootTarget(navID string) string {
	return virtualNavRootPrefix + navID
}

func (u *FolderDownloadUsecase) Create(ctx context.Context, kbID, targetID, userID string) (*domain.FolderDownloadTask, error) {
	root, docs, err := u.resolveTarget(ctx, kbID, targetID)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("folder has no downloadable files")
	}
	if len(docs) > folderDownloadMaxFiles {
		return nil, fmt.Errorf("folder contains too many files, limit is %d", folderDownloadMaxFiles)
	}
	var totalBytes int64
	for _, item := range docs {
		node, getErr := u.nodeRepo.GetNodeByID(ctx, item.ID)
		if getErr != nil {
			return nil, getErr
		}
		if node.Meta.FileReadonly && node.Meta.FileKey != "" {
			info, statErr := u.s3Client.StatObject(ctx, node.Meta.FileKey)
			if statErr != nil {
				return nil, fmt.Errorf("文件 %q 不在当前启用的 %s 存储中，无法加入压缩包: %w", node.Name, strings.ToUpper(u.s3Client.Type()), statErr)
			}
			totalBytes += info.Size
		} else {
			totalBytes += int64(len(node.Content))
			if node.Meta.ContentType == domain.ContentTypeHTML {
				totalBytes += int64(len(node.EditorContent))
			}
		}
	}
	if totalBytes > folderDownloadMaxBytes {
		return nil, fmt.Errorf("folder contents are too large, limit is %d bytes", folderDownloadMaxBytes)
	}

	task := &domain.FolderDownloadTask{
		ID:         uuid.NewString(),
		KBID:       kbID,
		NodeID:     targetID,
		CreatedBy:  userID,
		Status:     domain.FolderDownloadWaiting,
		TotalFiles: len(docs),
		TotalBytes: totalBytes,
		ObjectKey:  fmt.Sprintf("_exports/%s/%s/%s.zip", kbID, targetID, uuid.NewString()),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := u.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}
	go u.run(task.ID, root, docs)
	return task, nil
}

func (u *FolderDownloadUsecase) Get(ctx context.Context, id, userID string) (*domain.FolderDownloadTask, string, error) {
	task, err := u.taskRepo.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if task.CreatedBy != userID {
		return nil, "", fmt.Errorf("download task not found")
	}
	if task.Status != domain.FolderDownloadCompleted || task.ExpiresAt == nil || time.Now().After(*task.ExpiresAt) {
		if task.Status == domain.FolderDownloadCompleted && task.ExpiresAt != nil && time.Now().After(*task.ExpiresAt) {
			task.Status = domain.FolderDownloadExpired
			_ = u.taskRepo.Update(ctx, task)
			_ = u.s3Client.RemoveObject(ctx, task.ObjectKey)
		}
		return task, "", nil
	}
	url, err := u.s3Client.SignURL(ctx, domain.Bucket, task.ObjectKey, folderDownloadURLTTL)
	return task, url, err
}

func (u *FolderDownloadUsecase) List(ctx context.Context, kbID, userID string) ([]*domain.FolderDownloadTask, error) {
	return u.taskRepo.List(ctx, kbID, userID)
}

func (u *FolderDownloadUsecase) Retry(ctx context.Context, id, userID string) (*domain.FolderDownloadTask, error) {
	task, err := u.taskRepo.Get(ctx, id)
	if err != nil || task.CreatedBy != userID {
		return nil, fmt.Errorf("download task not found")
	}
	if task.Status != domain.FolderDownloadFailed {
		return task, nil
	}
	root, docs, err := u.resolveTarget(ctx, task.KBID, task.NodeID)
	if err != nil {
		return nil, err
	}
	task.Status = domain.FolderDownloadWaiting
	task.ErrorMessage = ""
	task.RetryCount++
	if err := u.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}
	go u.run(task.ID, root, docs)
	return task, nil
}

func (u *FolderDownloadUsecase) resolveTarget(ctx context.Context, kbID, targetID string) (*domain.Node, []*domain.NodeListItemResp, error) {
	if strings.HasPrefix(targetID, virtualNavRootPrefix) {
		navID := strings.TrimPrefix(targetID, virtualNavRootPrefix)
		if navID == "" || u.navRepo == nil {
			return nil, nil, fmt.Errorf("folder not found")
		}
		nav, err := u.navRepo.GetByKBID(ctx, kbID, navID)
		if err != nil {
			return nil, nil, fmt.Errorf("folder not found")
		}
		nodes, err := u.nodeRepo.GetList(ctx, &domain.GetNodeListReq{KBID: kbID, NavId: navID})
		if err != nil {
			return nil, nil, err
		}
		root := &domain.Node{ID: navID, KBID: kbID, NavId: navID, Name: nav.Name, Type: domain.NodeTypeFolder}
		return root, descendantsOfNav(nodes, navID), nil
	}

	root, err := u.nodeRepo.GetNodeByID(ctx, targetID)
	if err != nil {
		return nil, nil, err
	}
	if root.KBID != kbID || root.Type != domain.NodeTypeFolder {
		return nil, nil, fmt.Errorf("folder not found")
	}
	nodes, err := u.nodeRepo.GetList(ctx, &domain.GetNodeListReq{KBID: kbID, NavId: root.NavId})
	if err != nil {
		return nil, nil, err
	}
	return root, descendants(nodes, targetID), nil
}

func descendants(nodes []*domain.NodeListItemResp, rootID string) []*domain.NodeListItemResp {
	children := make(map[string][]*domain.NodeListItemResp)
	for _, node := range nodes {
		children[node.ParentID] = append(children[node.ParentID], node)
	}
	result := make([]*domain.NodeListItemResp, 0)
	var walk func(string)
	walk = func(parent string) {
		for _, node := range children[parent] {
			if node.Type == domain.NodeTypeDocument {
				result = append(result, node)
			} else {
				walk(node.ID)
			}
		}
	}
	walk(rootID)
	return result
}

func descendantsOfNav(nodes []*domain.NodeListItemResp, navID string) []*domain.NodeListItemResp {
	normalized := make([]*domain.NodeListItemResp, 0, len(nodes))
	for _, node := range nodes {
		if node.NavId != navID {
			continue
		}
		if node.ParentID == "" {
			copy := *node
			copy.ParentID = navID
			normalized = append(normalized, &copy)
			continue
		}
		normalized = append(normalized, node)
	}
	return descendants(normalized, navID)
}

func safeZipName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "_")
	value = path.Clean("/" + value)
	return strings.TrimLeft(value, "/")
}

func (u *FolderDownloadUsecase) run(id string, root *domain.Node, list []*domain.NodeListItemResp) {
	ctx := context.Background()
	task, err := u.taskRepo.Get(ctx, id)
	if err != nil {
		return
	}
	task.Status = domain.FolderDownloadProcessing
	_ = u.taskRepo.Update(ctx, task)
	allNodes, err := u.nodeRepo.GetList(ctx, &domain.GetNodeListReq{KBID: root.KBID, NavId: root.NavId})
	if err != nil {
		task.Status = domain.FolderDownloadFailed
		task.ErrorMessage = err.Error()
		_ = u.taskRepo.Update(ctx, task)
		return
	}
	nodeMap := make(map[string]*domain.NodeListItemResp, len(allNodes))
	for _, node := range allNodes {
		nodeMap[node.ID] = node
	}

	pr, pw := io.Pipe()
	uploadErr := make(chan error, 1)
	go func() {
		_, uploadErrValue := u.s3Client.PutObject(ctx, domain.Bucket, task.ObjectKey, pr, -1, minio.PutObjectOptions{ContentType: "application/zip"})
		uploadErr <- uploadErrValue
	}()

	zipWriter := zip.NewWriter(pw)
	rootName := safeZipName(root.Name)
	if rootName == "" {
		rootName = "文件夹"
	}
	failed := func(err error) {
		_ = zipWriter.Close()
		_ = pw.CloseWithError(err)
		task.Status = domain.FolderDownloadFailed
		task.ErrorMessage = err.Error()
		_ = u.taskRepo.Update(ctx, task)
	}

	for _, item := range list {
		node, getErr := u.nodeRepo.GetNodeByID(ctx, item.ID)
		if getErr != nil {
			failed(getErr)
			return
		}
		fileName := safeZipName(node.Name)
		if fileName == "" {
			fileName = node.ID
		}
		var processedBytes int64
		entryName := rootName + "/" + nodeRelativePath(node.ID, root.ID, nodeMap)
		if node.Meta.FileReadonly && node.Meta.FileKey != "" {
			fileName = safeZipName(node.Meta.FileName)
			if fileName == "" {
				fileName = safeZipName(node.Name)
			}
			entryName = rootName + "/" + nodeRelativePath(node.ID, root.ID, nodeMap)
			entryName = path.Join(path.Dir(entryName), fileName)
			object, objectErr := u.s3Client.GetObject(ctx, domain.Bucket, node.Meta.FileKey, minio.GetObjectOptions{})
			if objectErr != nil {
				failed(objectErr)
				return
			}
			writer, writerErr := zipWriter.Create(entryName)
			if writerErr == nil {
				processedBytes, writerErr = io.Copy(writer, object)
			}
			_ = object.Close()
			if writerErr != nil {
				failed(writerErr)
				return
			}
		} else {
			content := []byte(node.Content)
			ext := strings.ToLower(node.Meta.ContentType)
			if ext == "md" || ext == "markdown" || ext == "txt" {
				entryName += ".md"
			} else {
				entryName += ".html"
			}
			writer, writerErr := zipWriter.Create(entryName)
			if writerErr == nil {
				_, writerErr = writer.Write(content)
				processedBytes = int64(len(content))
			}
			if writerErr != nil {
				failed(writerErr)
				return
			}
			if ext == domain.ContentTypeHTML {
				payload, _ := json.Marshal(map[string]any{"version": 1, "name": node.Name, "content": node.Content, "editor_content": node.EditorContent, "editor_format": node.EditorFormat, "editor_version": node.EditorVersion})
				jsonWriter, jsonErr := zipWriter.Create(rootName + "/" + fileName + ".json")
				if jsonErr == nil {
					_, jsonErr = jsonWriter.Write(payload)
				}
				if jsonErr != nil {
					failed(jsonErr)
					return
				}
			}
		}
		task.ProcessedFiles++
		task.ProcessedBytes += processedBytes
		if err := u.taskRepo.Update(ctx, task); err != nil {
			failed(err)
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		failed(err)
		return
	}
	if err := pw.Close(); err != nil {
		failed(err)
		return
	}
	if err := <-uploadErr; err != nil {
		failed(err)
		return
	}
	expires := time.Now().Add(folderDownloadKeepFor)
	task.Status = domain.FolderDownloadCompleted
	task.ExpiresAt = &expires
	_ = u.taskRepo.Update(ctx, task)
}

func nodeRelativePath(nodeID, rootID string, nodes map[string]*domain.NodeListItemResp) string {
	parts := make([]string, 0, 4)
	current := nodeID
	for current != "" && current != rootID {
		node := nodes[current]
		if node == nil {
			break
		}
		parts = append(parts, safeZipName(node.Name))
		current = node.ParentID
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return path.Join(parts...)
}
