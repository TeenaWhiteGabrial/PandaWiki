package v1

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	logger         *log.Logger
	auth           middleware.AuthMiddleware
	config         *config.Config
	minioClient    *s3.MinioClient
	fileUsecase    *usecase.FileUsecase
	batchImport    *usecase.BatchImportUsecase
	folderDownload *usecase.FolderDownloadUsecase
	zipUpload      *usecase.ZipUploadUsecase
}

func NewFileHandler(echo *echo.Echo, baseHandler *handler.BaseHandler, logger *log.Logger, auth middleware.AuthMiddleware, minioClient *s3.MinioClient, config *config.Config, fileUsecase *usecase.FileUsecase, batchImport *usecase.BatchImportUsecase, folderDownload *usecase.FolderDownloadUsecase, zipUpload *usecase.ZipUploadUsecase) *FileHandler {
	h := &FileHandler{
		BaseHandler:    baseHandler,
		logger:         logger.WithModule("handler.v1.file"),
		auth:           auth,
		config:         config,
		minioClient:    minioClient,
		fileUsecase:    fileUsecase,
		batchImport:    batchImport,
		folderDownload: folderDownload,
		zipUpload:      zipUpload,
	}
	group := echo.Group("/api/v1/file")
	group.POST("/upload", h.Upload, h.auth.Authorize)
	group.GET("/download", h.Download, h.auth.Authorize)
	group.GET("/download/url", h.DownloadURL, h.auth.Authorize)
	group.POST("/upload/attachment", h.UploadAttachment, h.auth.Authorize)
	group.POST("/upload/image", h.UploadImage, h.auth.Authorize)
	group.POST("/upload/url", h.UploadByUrl, h.auth.Authorize)
	group.POST("/preview/generate", h.GeneratePreview, h.auth.Authorize)
	group.POST("/upload/zip/presign", h.PresignZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks", h.CreateZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.GET("/upload/zip/tasks", h.ListZipUploads, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.GET("/upload/zip/tasks/:id", h.GetZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.GET("/upload/zip/tasks/:id/parts/:part/presign", h.PresignZipUploadPart, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.PUT("/upload/zip/tasks/:id/parts/:part", h.UploadZipUploadPart, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks/:id/parts/:part/complete", h.CompleteZipUploadPart, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks/:id/complete", h.CompleteZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks/:id/retry", h.RetryZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks/:id/cancel", h.CancelZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/zip/tasks/:id/fail", h.FailZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/import/zip/commit", h.CommitZipUpload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/import/zip", h.ImportZip, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/folder-download", h.CreateFolderDownload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.GET("/folder-download/:id", h.GetFolderDownload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.GET("/folder-download", h.ListFolderDownloads, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/folder-download/:id/retry", h.RetryFolderDownload, h.auth.Authorize, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	group.POST("/upload/anydoc", h.UploadAnydoc)
	return h
}

// Download redirects the browser to a short-lived signed OSS URL. Static
// files are kept private in OSS; callers must not be forced to make the whole
// bucket publicly readable just to preview or download one document.
func (h *FileHandler) Download(c echo.Context) error {
	key := strings.TrimSpace(c.QueryParam("key"))
	if key == "" || strings.Contains(key, "..") {
		return h.NewResponseWithError(c, "file key is invalid", fmt.Errorf("invalid file key"))
	}
	if h.minioClient == nil {
		return h.NewResponseWithError(c, "object storage is not configured", fmt.Errorf("minio client is nil"))
	}
	url, err := h.minioClient.SignURL(c.Request().Context(), h.config.S3.Bucket, key, 30*time.Minute)
	if err != nil {
		return h.NewResponseWithError(c, "failed to create file download url", err)
	}
	return c.Redirect(http.StatusFound, url)
}

func (h *FileHandler) DownloadURL(c echo.Context) error {
	key := strings.TrimSpace(c.QueryParam("key"))
	if key == "" || strings.Contains(key, "..") {
		return h.NewResponseWithError(c, "file key is invalid", fmt.Errorf("invalid file key"))
	}
	if h.minioClient == nil {
		return h.NewResponseWithError(c, "object storage is not configured", fmt.Errorf("minio client is nil"))
	}
	url, err := h.minioClient.SignURL(c.Request().Context(), h.config.S3.Bucket, key, 30*time.Minute)
	if err != nil {
		return h.NewResponseWithError(c, "failed to create file download url", err)
	}
	return h.NewResponseWithData(c, map[string]string{"url": url})
}

type zipUploadTaskRequest struct {
	KBID     string `json:"kb_id" validate:"required"`
	NavID    string `json:"nav_id" validate:"required"`
	ParentID string `json:"parent_id"`
	Filename string `json:"filename" validate:"required"`
	Size     int64  `json:"size" validate:"required"`
}
type zipUploadPartRequest struct {
	ETag string `json:"etag" validate:"required"`
	Size int64  `json:"size"`
}

func (h *FileHandler) zipUser(c echo.Context) (*domain.CtxAuthInfo, error) {
	a := domain.GetAuthInfoFromCtx(c.Request().Context())
	if a == nil {
		return nil, fmt.Errorf("authInfo not found in context")
	}
	return a, nil
}
func (h *FileHandler) CreateZipUpload(c echo.Context) error {
	var req zipUploadTaskRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "request validation failed", err)
	}
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	t, err := h.zipUpload.Create(c.Request().Context(), usecase.ZipUploadCreate{KBID: req.KBID, NavID: req.NavID, ParentID: req.ParentID, UserID: a.UserId, Filename: req.Filename, Size: req.Size})
	if err != nil {
		return h.NewResponseWithError(c, "create zip upload task failed", err)
	}
	return h.NewResponseWithData(c, t)
}
func (h *FileHandler) ListZipUploads(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	tasks, err := h.zipUpload.List(c.Request().Context(), c.QueryParam("kb_id"), a.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "list zip upload tasks failed", err)
	}
	return h.NewResponseWithData(c, tasks)
}
func (h *FileHandler) GetZipUpload(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	t, err := h.zipUpload.Get(c.Request().Context(), c.Param("id"), a.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "get zip upload task failed", err)
	}
	return h.NewResponseWithData(c, t)
}
func (h *FileHandler) PresignZipUploadPart(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	var part int
	if _, err := fmt.Sscanf(c.Param("part"), "%d", &part); err != nil {
		return h.NewResponseWithError(c, "invalid part", err)
	}
	url, err := h.zipUpload.PresignPart(c.Request().Context(), c.Param("id"), a.UserId, part)
	if err != nil {
		return h.NewResponseWithError(c, "presign upload part failed", err)
	}
	return h.NewResponseWithData(c, map[string]any{"url": url, "part_number": part})
}
func (h *FileHandler) CompleteZipUploadPart(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	var req zipUploadPartRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	var part int
	if _, err := fmt.Sscanf(c.Param("part"), "%d", &part); err != nil {
		return h.NewResponseWithError(c, "invalid part", err)
	}
	t, err := h.zipUpload.CompletePart(c.Request().Context(), c.Param("id"), a.UserId, part, req.ETag, req.Size)
	if err != nil {
		return h.NewResponseWithError(c, "record uploaded part failed", err)
	}
	return h.NewResponseWithData(c, t)
}

func (h *FileHandler) UploadZipUploadPart(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	var part int
	if _, err := fmt.Sscanf(c.Param("part"), "%d", &part); err != nil {
		return h.NewResponseWithError(c, "invalid part", err)
	}
	etag, size, err := h.zipUpload.UploadPart(c.Request().Context(), c.Param("id"), a.UserId, part, c.Request().Body, c.Request().ContentLength)
	if err != nil {
		return h.NewResponseWithError(c, "upload part failed", err)
	}
	return h.NewResponseWithData(c, map[string]any{"etag": etag, "size": size, "part_number": part})
}
func (h *FileHandler) CompleteZipUpload(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	t, err := h.zipUpload.Complete(c.Request().Context(), c.Param("id"), a.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "complete zip upload failed", err)
	}
	// Return as soon as the multipart object has been committed. Extraction can
	// involve many OSS writes and preview conversions, so keeping it inside the
	// HTTP request causes production proxies to time out on medium-sized ZIPs.
	go h.processZipUpload(t.ID, t.KBID, t.NodeID, a.UserId)
	return h.NewResponseWithData(c, t)
}

func (h *FileHandler) processZipUpload(taskID, kbID, nodeID, userID string) {
	ctx := context.Background()
	_, err := h.batchImport.ExpandArchivesWithProgress(
		ctx,
		kbID,
		[]string{nodeID},
		userID,
		false,
		func(progress usecase.ArchiveProgress) {
			if updateErr := h.zipUpload.UpdateProcessingProgress(ctx, taskID, progress.TotalFiles, progress.ProcessedFiles, progress.TotalBytes, progress.ProcessedBytes); updateErr != nil {
				h.logger.Warn("update zip extraction progress failed", log.String("task_id", taskID), log.Error(updateErr))
			}
		},
	)
	if err != nil {
		h.logger.Error("zip extraction failed", log.String("task_id", taskID), log.Error(err))
		if _, failErr := h.zipUpload.Fail(ctx, taskID, userID, err.Error()); failErr != nil {
			h.logger.Error("mark zip extraction failed", log.String("task_id", taskID), log.Error(failErr))
		}
		return
	}
	if err := h.zipUpload.CompleteProcessing(ctx, taskID); err != nil {
		h.logger.Error("complete zip extraction task failed", log.String("task_id", taskID), log.Error(err))
	}
}
func (h *FileHandler) RetryZipUpload(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	t, err := h.zipUpload.Retry(c.Request().Context(), c.Param("id"), a.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "retry zip upload failed", err)
	}
	if t.NodeID != "" {
		go h.processZipUpload(t.ID, t.KBID, t.NodeID, a.UserId)
	}
	return h.NewResponseWithData(c, t)
}

func (h *FileHandler) CancelZipUpload(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	t, err := h.zipUpload.Cancel(c.Request().Context(), c.Param("id"), a.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "cancel zip upload failed", err)
	}
	return h.NewResponseWithData(c, t)
}

func (h *FileHandler) FailZipUpload(c echo.Context) error {
	a, err := h.zipUser(c)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	var req struct {
		Error string `json:"error"`
	}
	_ = c.Bind(&req)
	t, err := h.zipUpload.Fail(c.Request().Context(), c.Param("id"), a.UserId, req.Error)
	if err != nil {
		return h.NewResponseWithError(c, "mark zip upload failed", err)
	}
	return h.NewResponseWithData(c, t)
}

type folderDownloadRequest struct {
	KBID   string `json:"kb_id" validate:"required"`
	NodeID string `json:"node_id" validate:"required"`
}

func (h *FileHandler) CreateFolderDownload(c echo.Context) error {
	var req folderDownloadRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "request validation failed", err)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	task, err := h.folderDownload.Create(c.Request().Context(), req.KBID, req.NodeID, authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "create folder download task failed", err)
	}
	return h.NewResponseWithData(c, task)
}

func (h *FileHandler) GetFolderDownload(c echo.Context) error {
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	task, url, err := h.folderDownload.Get(c.Request().Context(), c.Param("id"), authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "get folder download task failed", err)
	}
	return h.NewResponseWithData(c, map[string]any{"task": task, "download_url": url})
}

func (h *FileHandler) ListFolderDownloads(c echo.Context) error {
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	tasks, err := h.folderDownload.List(c.Request().Context(), c.QueryParam("kb_id"), authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "list folder download tasks failed", err)
	}
	return h.NewResponseWithData(c, tasks)
}

func (h *FileHandler) RetryFolderDownload(c echo.Context) error {
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	task, err := h.folderDownload.Retry(c.Request().Context(), c.Param("id"), authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "retry folder download task failed", err)
	}
	return h.NewResponseWithData(c, task)
}

type zipUploadRequest struct {
	KBID     string `json:"kb_id" validate:"required"`
	Filename string `json:"filename" validate:"required"`
	Size     int64  `json:"size" validate:"required"`
}

type zipUploadCommitRequest struct {
	KBID     string `json:"kb_id" validate:"required"`
	NavID    string `json:"nav_id" validate:"required"`
	ParentID string `json:"parent_id"`
	FileKey  string `json:"file_key" validate:"required"`
	Filename string `json:"filename" validate:"required"`
	Size     int64  `json:"size" validate:"required"`
}

func (h *FileHandler) PresignZipUpload(c echo.Context) error {
	var req zipUploadRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "request validation failed", err)
	}
	ticket, err := h.batchImport.PresignUpload(c.Request().Context(), req.KBID, req.Filename, req.Size)
	if err != nil {
		return h.NewResponseWithError(c, "create zip upload ticket failed", err)
	}
	return h.NewResponseWithData(c, ticket)
}

func (h *FileHandler) CommitZipUpload(c echo.Context) error {
	var req zipUploadCommitRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "request validation failed", err)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	nodeID, err := h.batchImport.CommitUpload(c.Request().Context(), usecase.ZipUploadCommit{
		KBID: req.KBID, NavID: req.NavID, ParentID: req.ParentID, UserID: authInfo.UserId,
		FileKey: req.FileKey, Name: req.Filename, Size: req.Size,
	})
	if err != nil {
		return h.NewResponseWithError(c, "commit zip upload failed", err)
	}
	return h.NewResponseWithData(c, map[string]string{"node_id": nodeID, "file_key": req.FileKey})
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
	result, err := h.batchImport.ImportZip(c.Request().Context(), kbID, navID, parentID, authInfo.UserId, file, true)
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
