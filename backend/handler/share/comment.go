package share

import (
	"fmt"
	"net/http"

	"github.com/chaitin/panda-wiki/store/cache"

	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
)

type ShareCommentHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.CommentUsecase
	app     *usecase.AppUsecase
	file    *usecase.FileUsecase
	node    *usecase.NodeUsecase
	cache   *cache.Cache
}

func NewShareCommentHandler(
	e *echo.Echo,
	baseHandler *handler.BaseHandler,
	logger *log.Logger,
	usecase *usecase.CommentUsecase,
	app *usecase.AppUsecase,
	file *usecase.FileUsecase,
	node *usecase.NodeUsecase,
	cache *cache.Cache,
) *ShareCommentHandler {
	h := &ShareCommentHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.share.comment"),
		usecase:     usecase,
		app:         app,
		file:        file,
		node:        node,
		cache:       cache,
	}

	share := e.Group("share/v1/comment",
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if c.Request().Method == "OPTIONS" {
					return c.NoContent(http.StatusOK)
				}
				return next(c)
			}
		}, h.ShareAuthMiddleware.Authorize)

	share.POST("", h.CreateComment)
	share.GET("/list", h.GetCommentList)
	return h
}

// CreateComment
//
//	@Summary		CreateComment
//	@Description	CreateComment
//	@Tags			share_comment
//	@Accept			json
//	@Produce		json
//	@Param			comment	body		domain.CommentReq				true	"Comment"
//	@Success		200		{object}	domain.PWResponse{data=string}	"CommentID"
//	@Router			/share/v1/comment [post]
func (h *ShareCommentHandler) CreateComment(c echo.Context) error {
	ctx := c.Request().Context()
	userID := domain.GetShareUserID(c)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, domain.PWResponse{Message: "请登录后发表评论", Success: false})
	}

	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}

	var req domain.CommentReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "bind comment request failed", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate req failed", err)
	}
	if code := h.node.ValidateNodePerm(ctx, kbID, req.NodeID, userID); code != nil {
		return h.NewResponseWithErrCode(c, *code)
	}
	if len([]rune(req.Content)) > 5000 || len(req.PicUrls) > 9 {
		return h.NewResponseWithError(c, "评论最多 5000 字、9 张图片", nil)
	}
	// Redis provides an atomic cross-process per-account, per-document window.
	count, err := h.cache.Eval(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],60) end; return n`, []string{fmt.Sprintf("comment_rate:%s:%s", userID, req.NodeID)}).Int64()
	if err != nil {
		return h.NewResponseWithError(c, "暂时无法发表评论", err)
	}
	if count > 5 {
		return c.JSON(http.StatusTooManyRequests, domain.PWResponse{Message: "评论过于频繁，请稍后重试", Success: false})
	}
	// 校验是否开启了评论
	appInfo, err := h.app.GetAppDetailByKBIDAndAppType(ctx, kbID, domain.AppType(domain.AppTypeWeb))
	if err != nil {
		return h.NewResponseWithError(c, "app info is not found", err)
	}
	if !appInfo.Settings.WebAppCommentSettings.IsEnable {
		return h.NewResponseWithError(c, "please check comment is open", nil)
	}
	// validate captcha token
	if !h.Captcha.ValidateToken(ctx, req.CaptchaToken) {
		return h.NewResponseWithError(c, "failed to validate captcha token", nil)
	}

	for _, url := range req.PicUrls {
		if !h.file.IsManagedImageURL(url) {
			return h.NewResponseWithError(c, "validate param pic_urls failed", err)
		}
	}

	remoteIP := c.RealIP()

	var status = 1 // no moderate
	// 判断user is moderate comment ---> 默认false
	if appInfo.Settings.WebAppCommentSettings.ModerationEnable {
		status = 0
	}
	commentStatus := domain.CommentStatus(status)

	// 插入到数据库中
	commentID, err := h.usecase.CreateComment(ctx, &req, kbID, remoteIP, commentStatus, userID)
	if err != nil {
		return h.NewResponseWithError(c, "create comment failed", err)
	}

	return h.NewResponseWithData(c, commentID)
}

type ShareCommentLists = *domain.PaginatedResult[[]*domain.ShareCommentListItem]

// GetCommentList
//
//	@Summary		GetCommentList
//	@Description	GetCommentList
//	@Tags			share_comment
//	@Accept			json
//	@Produce		json
//	@Param			id	query		string										true	"nodeID"
//	@Success		200	{object}	domain.PWResponse{data=ShareCommentLists}	"CommentList
//	@Router			/share/v1/comment/list [get]
func (h *ShareCommentHandler) GetCommentList(c echo.Context) error {
	ctx := c.Request().Context()

	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}

	// 拿到node_id即可
	nodeID := c.QueryParam("id")
	if nodeID == "" {
		return h.NewResponseWithError(c, "node id is required", nil)
	}

	if code := h.node.ValidateNodePerm(ctx, kbID, nodeID, domain.GetShareUserID(c)); code != nil {
		return h.NewResponseWithErrCode(c, *code)
	}
	// 校验是否开启了评论
	appInfo, err := h.app.GetAppDetailByKBIDAndAppType(ctx, kbID, domain.AppType(domain.AppTypeWeb))
	if err != nil {
		return h.NewResponseWithError(c, "app info is not found", err)
	}
	if !appInfo.Settings.WebAppCommentSettings.IsEnable {
		return h.NewResponseWithError(c, "please check comment is open", nil)
	}

	// 查询数据库获取所有评论-->0 所有， 1，2 为需要审核的评论
	commentsList, err := h.usecase.GetCommentListByNodeID(ctx, nodeID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get comment list", err)
	}

	return h.NewResponseWithData(c, commentsList)
}
