package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/minio/minio-go/v7"
)

var (
	ErrFileContentForbidden = errors.New("没有此文档的管理权限")
	ErrFileContentMissing   = errors.New("文件不存在，可能尚未迁移或已被删除")
	ErrFileContentType      = errors.New("文件内容与预览格式不符")
	ErrFileContentLarge     = errors.New("文件过大，请下载原文件查看")
)

type FileContent struct {
	Reader      io.ReadSeekCloser
	ContentType string
	Size        int64
	Modified    time.Time
}

func (u *NodeUsecase) FileMetaForAdmin(ctx context.Context, kbID, nodeID string) (domain.NodeMeta, error) {
	if domain.GetAuthInfoFromCtx(ctx) == nil {
		return domain.NodeMeta{}, ErrFileContentForbidden
	}
	node, err := u.nodeRepo.GetByID(ctx, nodeID, kbID)
	if err != nil {
		return domain.NodeMeta{}, err
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return domain.NodeMeta{}, err
	}
	if !access.CanManageNode(nodeID) {
		return domain.NodeMeta{}, ErrFileContentForbidden
	}
	return node.Meta, nil
}

func checkPreviewSize(format string, size int64) error {
	if size < 0 || (format == "pdf" && size == 0) {
		return ErrFileContentType
	}
	if (format == "pdf" && size > 100*1024*1024) || (format == "text" && size > 5*1024*1024) {
		return ErrFileContentLarge
	}
	if format != "pdf" && format != "text" && format != "download" {
		return ErrFileContentType
	}
	return nil
}

func previewContentType(key, format string, size int64, head []byte) (string, error) {
	if err := checkPreviewSize(format, size); err != nil {
		return "", err
	}
	switch format {
	case "pdf":
		if !bytes.Contains(head, []byte("%PDF-")) {
			return "", ErrFileContentType
		}
		return "application/pdf", nil
	case "text":
		switch strings.ToLower(path.Ext(key)) {
		case ".txt", ".md", ".markdown", ".csv", ".json", ".xml", ".log", ".html", ".htm":
		default:
			return "", ErrFileContentType
		}
		if bytes.ContainsRune(head, 0) {
			return "", ErrFileContentType
		}
		return "text/plain; charset=utf-8", nil
	case "download":
		return "application/octet-stream", nil
	default:
		return "", ErrFileContentType
	}
}

// OpenFileContent reads only the configured object store, never a client URL.
// The handler must authorize the node and its exact file association first.
func (u *FileUsecase) OpenFileContent(ctx context.Context, key, format string) (*FileContent, error) {
	key, err := normalizeObjectKey(key)
	if err != nil {
		return nil, err
	}
	if u.s3Client == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	info, err := u.s3Client.StatObject(ctx, key)
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.StatusCode == http.StatusNotFound || response.Code == "NoSuchKey" || response.Code == "NoSuchObject" {
			return nil, ErrFileContentMissing
		}
		return nil, err
	}
	// Reject oversized previews before opening or buffering the object.
	if err := checkPreviewSize(format, info.Size); err != nil {
		return nil, err
	}
	object, err := u.s3Client.GetObject(ctx, domain.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	head := make([]byte, 1024)
	n, err := object.ReadAt(head, 0)
	if err != nil && err != io.EOF {
		object.Close()
		return nil, err
	}
	contentType, err := previewContentType(key, format, info.Size, head[:n])
	if err != nil {
		object.Close()
		return nil, err
	}
	if _, err := object.Seek(0, io.SeekStart); err != nil {
		object.Close()
		return nil, err
	}
	return &FileContent{Reader: object, ContentType: contentType, Size: info.Size, Modified: info.LastModified}, nil
}
