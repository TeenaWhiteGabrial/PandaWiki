package v1

import (
	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
)

type ContributeHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	auth    middleware.AuthMiddleware
	usecase *usecase.ContributeUsecase
}

func NewContributeHandler(e *echo.Echo, baseHandler *handler.BaseHandler, logger *log.Logger, auth middleware.AuthMiddleware, usecase *usecase.ContributeUsecase) *ContributeHandler {
	h := &ContributeHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.contribute"),
		auth:        auth,
		usecase:     usecase,
	}

	group := e.Group("/api/pro/v1/contribute", auth.Authorize, auth.ValidateKBUserPerm(consts.UserKBPermissionDataOperate))
	group.GET("/list", h.GetList)
	group.GET("/detail", h.GetDetail)
	group.POST("/audit", h.Audit)

	return h
}

func (h *ContributeHandler) GetList(c echo.Context) error {
	var req domain.ContributeListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "bind request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	resp, err := h.usecase.GetList(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get contribute list", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *ContributeHandler) GetDetail(c echo.Context) error {
	kbID := c.QueryParam("kb_id")
	id := c.QueryParam("id")
	if kbID == "" || id == "" {
		return h.NewResponseWithError(c, "kb_id and id are required", nil)
	}

	resp, err := h.usecase.GetDetail(c.Request().Context(), kbID, id)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get contribute detail", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *ContributeHandler) Audit(c echo.Context) error {
	var req domain.ContributeAuditReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "bind request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil || authInfo.UserId == "" {
		return h.NewResponseWithError(c, "user is required", nil)
	}

	if err := h.usecase.Audit(c.Request().Context(), &req, authInfo.UserId); err != nil {
		return h.NewResponseWithError(c, "failed to audit contribute", err)
	}
	return h.NewResponseWithData(c, &domain.ContributeAuditResp{Message: "success"})
}
