package share

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	v1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/chaitin/panda-wiki/utils"
)

type ShareCommonHandler struct {
	*handler.BaseHandler
	logger      *log.Logger
	fileUsecase *usecase.FileUsecase
	nodeUsecase *usecase.NodeUsecase
}

func NewShareCommonHandler(
	e *echo.Echo,
	baseHandler *handler.BaseHandler,
	logger *log.Logger,
	fileUsecase *usecase.FileUsecase,
	nodeUsecase *usecase.NodeUsecase,
) *ShareCommonHandler {
	h := &ShareCommonHandler{
		BaseHandler: baseHandler,
		logger:      logger,
		fileUsecase: fileUsecase,
		nodeUsecase: nodeUsecase,
	}

	share := e.Group("share/v1/common",
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if c.Request().Method == "OPTIONS" {
					return c.NoContent(http.StatusOK)
				}
				return next(c)
			}
		})
	share.POST("/file/upload", h.FileUpload, h.ShareAuthMiddleware.Authorize)
	share.POST("/file/upload/url", h.FileUploadByUrl, h.ShareAuthMiddleware.Authorize)
	share.GET("/file/download/url", h.FileDownloadURL, h.ShareAuthMiddleware.Authorize)
	return h
}

// FileUpload 文件上传
//
//	@Tags			ShareFile
//	@Summary		文件上传
//	@Description	前台用户上传文件,目前只支持图片文件上传
//	@ID				share-FileUpload
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			X-KB-ID			header		string	true	"kb id"
//	@Param			file			formData	file	true	"File"
//	@Param			captcha_token	formData	string	true	"captcha_token"
//	@Success		200				{object}	domain.Response{data=v1.FileUploadResp}
//	@Router			/share/v1/common/file/upload [post]
func (h *ShareCommonHandler) FileUpload(c echo.Context) error {
	ctx := c.Request().Context()

	var req v1.ShareFileUploadReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request parameters", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	req.KbId = kbID

	file, err := c.FormFile("file")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get file", err)
	}

	if !utils.IsImageFile(file.Filename) {
		return h.NewResponseWithError(c, "只支持图片文件上传", fmt.Errorf("unsupported file type: %s", file.Filename))
	}

	// validate captcha token
	if !h.Captcha.ValidateToken(ctx, req.CaptchaToken) {
		return h.NewResponseWithError(c, "failed to validate captcha token", nil)
	}

	key, imageURL, err := h.fileUsecase.UploadImage(ctx, req.KbId, file)
	if err != nil {
		return h.NewResponseWithError(c, "upload failed", err)
	}

	return h.NewResponseWithData(c, v1.FileUploadResp{
		Key: key,
		URL: imageURL,
	})
}

// FileUploadByUrl 通过url上传文件
//
//	@Tags			ShareFile
//	@Summary		文件上传
//	@Description	前台用户上传文件,目前只支持图片文件上传
//	@ID				share-FileUploadByUrl
//	@Accept			json
//	@Produce		json
//	@Param			body	body		v1.ShareFileUploadUrlReq	true	"body"
//	@Success		200		{object}	domain.Response{data=v1.ShareFileUploadUrlResp}
//	@Router			/share/v1/common/file/upload/url [post]
func (h *ShareCommonHandler) FileUploadByUrl(c echo.Context) error {
	ctx := c.Request().Context()

	var req v1.ShareFileUploadUrlReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request parameters", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	req.KbId = kbID

	parsedURL, err := url.Parse(req.Url)
	if err != nil {
		return h.NewResponseWithError(c, "invalid URL format", err)
	}
	if !utils.IsImageFile(parsedURL.Path) {
		return h.NewResponseWithError(c, "只支持图片文件上传", fmt.Errorf("unsupported file type: %s", req.Url))
	}

	// validate captcha token
	if !h.Captcha.ValidateToken(ctx, req.CaptchaToken) {
		return h.NewResponseWithError(c, "failed to validate captcha token", nil)
	}

	key, err := h.fileUsecase.UploadFileByUrl(ctx, req.KbId, req.Url)
	if err != nil {
		return h.NewResponseWithError(c, "upload failed", err)
	}

	return h.NewResponseWithData(c, v1.ShareFileUploadUrlResp{
		Key: key,
	})
}

// FileDownloadURL 获取前台文件下载/预览签名地址
//
//	@Tags		ShareFile
//	@Summary	文件下载地址
//	@Produce	json
//	@Param		key	query		string	true	"file key"
//	@Param		node_id	query	string	true	"node id"
//	@Success	200	{object}	domain.Response{data=v1.ShareFileDownloadUrlResp}
//	@Router		/share/v1/common/file/download/url [get]
func (h *ShareCommonHandler) FileDownloadURL(c echo.Context) error {
	key := strings.TrimSpace(c.QueryParam("key"))
	key = strings.TrimSpace(strings.TrimPrefix(key, "/static-file/"))
	key = strings.TrimLeft(key, "/")
	if key == "" {
		return h.NewResponseWithError(c, "file key is required", fmt.Errorf("file key is empty"))
	}
	kbID := strings.TrimSpace(c.Request().Header.Get("X-KB-ID"))
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	// Storage keys can retain an older KB prefix after data migration. The
	// current KB still scopes node permission checks, and the exact key must
	// still be present in that node's published metadata below.
	nodeID := strings.TrimSpace(c.QueryParam("node_id"))
	if nodeID == "" {
		return h.NewResponseWithError(c, "node_id is required", fmt.Errorf("node_id is empty"))
	}
	if errCode := h.nodeUsecase.ValidateNodePerm(c.Request().Context(), kbID, nodeID, domain.GetShareUserID(c)); errCode != nil {
		return h.NewResponseWithErrCode(c, *errCode)
	}
	node, err := h.nodeUsecase.GetNodeReleaseDetailByKBIDAndID(c.Request().Context(), kbID, nodeID, "raw")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get node detail", err)
	}
	if !shareNodeOwnsFileKey(node.Meta, key) {
		return h.NewResponseWithError(c, "file key is invalid", fmt.Errorf("file key does not belong to current node"))
	}
	url, err := h.fileUsecase.SignFileURL(c.Request().Context(), key, 30*time.Minute)
	if err != nil {
		return h.NewResponseWithError(c, "failed to create file download url", err)
	}
	return h.NewResponseWithData(c, v1.ShareFileDownloadUrlResp{URL: url})
}

func shareNodeOwnsFileKey(meta domain.NodeMeta, key string) bool {
	if key == "" {
		return false
	}
	if meta.FileKey == key || meta.PreviewKey == key {
		return true
	}
	for _, attachment := range meta.Attachments {
		if attachment.FileKey == key || attachment.PreviewKey == key {
			return true
		}
	}
	return false
}
