package share

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func (h *ShareCommonHandler) AdminFileContent(c echo.Context) error  { return h.fileContent(c, false) }
func (h *ShareCommonHandler) ReaderFileContent(c echo.Context) error { return h.fileContent(c, true) }

func (h *ShareCommonHandler) fileContent(c echo.Context, reader bool) error {
	key := strings.TrimLeft(strings.TrimPrefix(strings.TrimSpace(c.QueryParam("key")), "/static-file/"), "/")
	nodeID, kbID := strings.TrimSpace(c.QueryParam("node_id")), strings.TrimSpace(c.QueryParam("kb_id"))
	if reader {
		kbID = strings.TrimSpace(c.Request().Header.Get("X-KB-ID"))
	}
	format := c.QueryParam("format")
	if key == "" || nodeID == "" || kbID == "" || strings.Contains(key, "..") || strings.ContainsAny(key, "\\\x00\r\n") || strings.Contains(key, "://") || (format != "pdf" && format != "text" && format != "download") {
		return h.NewResponseWithStatusError(c, http.StatusBadRequest, "文件预览参数无效", fmt.Errorf("invalid file content request"))
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()
	var meta domain.NodeMeta
	if reader {
		if denied := h.nodeUsecase.ValidateNodePerm(ctx, kbID, nodeID, domain.GetShareUserID(c)); denied != nil {
			return h.NewResponseWithStatusError(c, http.StatusForbidden, "没有此文档的查阅权限", fmt.Errorf("node access denied: %s", denied.Message))
		}
		node, err := h.nodeUsecase.GetNodeReleaseDetailByKBIDAndID(ctx, kbID, nodeID, "raw")
		if err != nil {
			return h.fileContentError(c, err)
		}
		meta = node.Meta
	} else {
		var err error
		meta, err = h.nodeUsecase.FileMetaForAdmin(ctx, kbID, nodeID)
		if err != nil {
			return h.fileContentError(c, err)
		}
	}
	if !domain.NodeOwnsFileKey(meta, key) {
		return h.NewResponseWithStatusError(c, http.StatusForbidden, "此文件不属于当前文档", fmt.Errorf("file key does not belong to node"))
	}
	content, err := h.fileUsecase.OpenFileContent(ctx, key, format)
	if err != nil {
		return h.fileContentError(c, err)
	}
	defer content.Reader.Close()
	filename := domain.NodeFileName(meta, key)
	if filename == "" {
		filename = path.Base(key)
	}
	return serveFileContent(c, content, filename)
}

func serveFileContent(c echo.Context, content *usecase.FileContent, filename string) error {
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, filename)
	headers := c.Response().Header()
	headers.Set("Cache-Control", "private, no-store")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Referrer-Policy", "no-referrer")
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
	headers.Set("Content-Type", content.ContentType)
	// fetch/PDF.js reads bytes regardless of this header. Direct navigation must
	// always download, never execute a legacy HTML/SVG object on the app origin.
	headers.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	http.ServeContent(c.Response(), c.Request(), filename, content.Modified, content.Reader)
	return nil
}

func (h *ShareCommonHandler) fileContentError(c echo.Context, err error) error {
	status, message := http.StatusBadGateway, "文件读取失败，请稍后重试"
	switch {
	case errors.Is(err, usecase.ErrFileContentForbidden):
		status, message = http.StatusForbidden, err.Error()
	case errors.Is(err, usecase.ErrFileContentMissing), errors.Is(err, gorm.ErrRecordNotFound):
		status, message = http.StatusNotFound, "文件或文档不存在，可能尚未迁移或已被删除"
	case errors.Is(err, usecase.ErrFileContentType):
		status, message = http.StatusUnsupportedMediaType, "文件内容与预览格式不符，请下载原文件检查"
	case errors.Is(err, usecase.ErrFileContentLarge):
		status, message = http.StatusRequestEntityTooLarge, err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		status, message = http.StatusGatewayTimeout, "文件读取超时，请重试"
	}
	return h.NewResponseWithStatusError(c, status, message, err)
}
