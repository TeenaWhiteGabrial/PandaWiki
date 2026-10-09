package share

import (
	"github.com/chaitin/panda-wiki/domain"
	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
)

type ShareConversationHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.ConversationUsecase
}

func NewShareConversationHandler(
	baseHandler *handler.BaseHandler,
	echo *echo.Echo,
	usecase *usecase.ConversationUsecase,
	logger *log.Logger,
) *ShareConversationHandler {
	h := &ShareConversationHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.share.conversation"),
		usecase:     usecase,
	}

	group := echo.Group("share/v1/conversation",
		h.ShareAuthMiddleware.Authorize,
	)
	group.GET("/detail", h.GetConversationDetail)
	group.POST("/resume", h.ResumeConversation)
	group.POST("/history", h.GetConversationHistory)

	return h
}

// GetConversationHistory lists the authenticated user's chats or verified anonymous browser sessions.
func (h *ShareConversationHandler) GetConversationHistory(c echo.Context) error {
	var req domain.ShareConversationHistoryRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "历史记录请求格式错误", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "历史记录查询参数错误", err)
	}
	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	result, err := h.usecase.GetShareConversationHistory(c.Request().Context(), kbID, domain.GetShareUserID(c), &req)
	if err != nil {
		return h.NewResponseWithError(c, "加载历史记录失败，请重试", err)
	}
	return h.NewResponseWithData(c, result)
}

// ResumeConversation restores the owner's session or forks a shared history.
//
// @Summary Resume conversation
// @Tags share_conversation
// @Accept json
// @Produce json
// @Param X-KB-ID header string true "Knowledge base ID"
// @Param request body domain.ResumeConversationRequest true "Conversation to resume"
// @Success 200 {object} domain.PWResponse{data=domain.ChatSession}
// @Router /share/v1/conversation/resume [post]
func (h *ShareConversationHandler) ResumeConversation(c echo.Context) error {
	var req domain.ResumeConversationRequest
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid conversation request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "conversation id is required", err)
	}
	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	session, err := h.usecase.ResumeShareConversation(c.Request().Context(), kbID, req.ID, req.Nonce, domain.GetShareUserID(c), c.RealIP())
	if err != nil {
		return h.NewResponseWithError(c, "恢复会话失败，请重新打开历史对话或新建会话", err)
	}
	return h.NewResponseWithData(c, session)
}

// GetConversationDetail
//
//	@Summary		GetConversationDetail
//	@Description	GetConversationDetail
//	@Tags			share_conversation
//	@Accept			json
//	@Produce		json
//	@Param			X-KB-ID	header		string	true	"kb id"
//	@Param			id		query		string	true	"conversation id"
//	@Success		200		{object}	domain.PWResponse{data=domain.ShareConversationDetailResp}
//	@Router			/share/v1/conversation/detail [get]
func (h *ShareConversationHandler) GetConversationDetail(c echo.Context) error {
	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}
	id := c.QueryParam("id")
	if id == "" {
		return h.NewResponseWithError(c, "id is required", nil)
	}

	node, err := h.usecase.GetShareConversationDetail(c.Request().Context(), kbID, id)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get node detail", err)
	}
	return h.NewResponseWithData(c, node)
}
