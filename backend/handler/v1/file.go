package v1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/chaitin/panda-wiki/utils"
)

type FileHandler struct {
	*handler.BaseHandler
	logger      *log.Logger
	auth        middleware.AuthMiddleware
	config      *config.Config
	fileUsecase *usecase.FileUsecase
	batchImport *usecase.BatchImportUsecase
}

func NewFileHandler(echo *echo.Echo, baseHandler *handler.BaseHandler, logger *log.Logger, auth middleware.AuthMiddleware, minioClient *s3.MinioClient, config *config.Config, fileUsecase *usecase.FileUsecase, batchImport *usecase.BatchImportUsecase) *FileHandler {
	h := &FileHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.file"),
		auth:        auth,
		config:      config,
		fileUsecase: fileUsecase,
		batchImport: batchImport,
	}
	group := echo.Group("/api/v1/file")
	group.POST("/upload", h.Upload, h.auth.Authorize)
	group.POST("/upload/attachment", h.UploadAttachment, h.auth.Authorize)
	group.POST("/upload/image", h.UploadImage, h.auth.Authorize)
	group.POST("/upload/url", h.UploadByUrl, h.auth.Authorize)
	group.POST("/preview/generate", h.GeneratePreview, h.auth.Authorize)
	group.POST("/import/zip", h.ImportZip, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/anydoc", h.UploadAnydoc)
	return h
}

// ImportZip imports a ZIP archive while preserving its relative directory
// structure in both the knowledge base tree and object storage.
func (h *FileHandler) ImportZip(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get zip file", err)
	}
	kbID := c.FormValue("kb_id")
	navID := c.FormValue("nav_id")
	parentID := c.FormValue("parent_id")
	if kbID == "" || navID == "" {
		return h.NewResponseWithError(c, "kb_id and nav_id are required", nil)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	result, err := h.batchImport.ImportZip(c.Request().Context(), kbID, navID, parentID, authInfo.UserId, file)
	if err != nil {
		return h.NewResponseWithError(c, "batch import failed", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *FileHandler) UploadAttachment(c echo.Context) error {
	kbID := c.FormValue("kb_id")
	if kbID == "" {
		kbID = uuid.New().String()
	}
	file, err := c.FormFile("file")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get attachment", err)
	}

	key, err := h.fileUsecase.UploadAttachment(c.Request().Context(), kbID, file)
	if err != nil {
		return h.NewResponseWithError(c, "upload attachment failed", err)
	}
	return h.NewResponseWithData(c, domain.ObjectUploadResp{
		Key:      key,
		Filename: file.Filename,
	})
}

func (h *FileHandler) UploadImage(c echo.Context) error {
	kbID := c.FormValue("kb_id")
	if kbID == "" {
		kbID = uuid.New().String()
	}
	file, err := c.FormFile("file")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get image", err)
	}
	if !utils.IsWebsiteImageFile(file.Filename) {
		return h.NewResponseWithError(c, "unsupported image type", fmt.Errorf("unsupported image type: %s", file.Filename))
	}

	key, imageURL, err := h.fileUsecase.UploadImage(c.Request().Context(), kbID, file)
	if err != nil {
		return h.NewResponseWithError(c, "upload image failed", err)
	}
	return h.NewResponseWithData(c, domain.ObjectUploadResp{
		Key:      key,
		Filename: file.Filename,
		URL:      imageURL,
	})
}

// Upload
//
//	@Summary		Upload File
//	@Description	Upload File
//	@Tags			file
//	@Accept			multipart/form-data
//	@Param			file	formData	file	true	"File"
//	@Param			kb_id	formData	string	false	"Knowledge Base ID"
//	@Success		200		{object}	domain.ObjectUploadResp
//	@Router			/api/v1/file/upload [post]
func (h *FileHandler) Upload(c echo.Context) error {
	cxt := c.Request().Context()
	kbID := c.FormValue("kb_id")
	if kbID == "" {
		kbID = uuid.New().String()
	}
	file, err := c.FormFile("file")
	if err != nil {
		return h.NewResponseWithError(c, "failed to get file", err)
	}
	key, err := h.fileUsecase.UploadFile(cxt, kbID, file)
	if err != nil {
		return h.NewResponseWithError(c, "upload failed", err)
	}

	return h.NewResponseWithData(c, domain.ObjectUploadResp{
		Key:      key,
		Filename: file.Filename,
	})
}

// UploadByUrl
//
//	@Summary		Upload File By Url
//	@Description	Upload File By Url
//	@Tags			file
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.UploadByUrlReq	true	"Request Body"
//	@Success		200		{object}	domain.Response{data=domain.ObjectUploadResp}
//	@Router			/api/v1/file/upload/url [post]
func (h *FileHandler) UploadByUrl(c echo.Context) error {
	ctx := c.Request().Context()

	var req domain.UploadByUrlReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request parameters", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	kbID := req.KbId
	if kbID == "" {
		kbID = uuid.New().String()
	}

	key, err := h.fileUsecase.UploadFileByUrl(ctx, kbID, req.Url)
	if err != nil {
		return h.NewResponseWithError(c, "upload failed", err)
	}

	return h.NewResponseWithData(c, domain.ObjectUploadResp{
		Key: key,
	})
}

func (h *FileHandler) GeneratePreview(c echo.Context) error {
	var req domain.GenerateFilePreviewReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request parameters", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	previewKey, err := h.fileUsecase.GeneratePreviewByKey(
		c.Request().Context(),
		req.KBID,
		req.FileKey,
		req.FileName,
	)
	if err != nil {
		return h.NewResponseWithError(c, "generate preview failed", err)
	}

	return h.NewResponseWithData(c, domain.GenerateFilePreviewResp{
		PreviewKey: previewKey,
	})
}

// UploadAnydoc
//
//	@Summary		Upload Anydoc File
//	@Description	Upload Anydoc File
//	@Tags			file
//	@Accept			multipart/form-data
//	@Param			file	formData	file	true	"File"
//	@Param			path	formData	string	true	"File Path"
//	@Success		200		{object}	domain.AnydocUploadResp
//	@Router			/api/v1/file/upload/anydoc [post]
func (h *FileHandler) UploadAnydoc(c echo.Context) error {
	clientIP := fmt.Sprintf("%s.17", h.config.SubnetPrefix)
	if utils.GetClientIPFromRemoteAddr(c) != clientIP {
		return c.JSON(http.StatusUnauthorized, domain.AnydocUploadResp{
			Code: 1,
			Err:  "invalid required",
		})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, domain.AnydocUploadResp{
			Code: 1,
			Err:  "invalid required",
		})
	}

	path := c.FormValue("path")
	if path == "" {
		return c.JSON(http.StatusBadRequest, domain.AnydocUploadResp{
			Code: 1,
			Err:  "invalid required",
		})
	}

	h.logger.Debug("AnydocUpload file", "path", path)
	_, err = h.fileUsecase.AnyDocUploadFile(c.Request().Context(), file, path)
	if err != nil {
		return h.NewResponseWithError(c, "upload failed", err)
	}
	url := fmt.Sprintf("/static-file/%s", strings.TrimPrefix(path, "/"))
	h.logger.Debug("AnydocUpload file", "path", url)

	return c.JSON(http.StatusOK, domain.AnydocUploadResp{
		Code: 0,
		Data: url,
	})
}
