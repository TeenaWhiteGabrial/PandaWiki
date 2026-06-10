package v1

import (
	"github.com/labstack/echo/v4"

	tokenV1 "github.com/chaitin/panda-wiki/api/token/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
)

type APITokenHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.APITokenUsecase
	auth    middleware.AuthMiddleware
}

func NewAPITokenHandler(
	baseHandler *handler.BaseHandler,
	echo *echo.Echo,
	usecase *usecase.APITokenUsecase,
	auth middleware.AuthMiddleware,
	logger *log.Logger,
) *APITokenHandler {
	h := &APITokenHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.api_token"),
		usecase:     usecase,
		auth:        auth,
	}

	group := echo.Group("/api/pro/v1/token", auth.Authorize, auth.ValidateKBUserPerm(consts.UserKBPermissionNotNull))
	group.POST("/create", h.Create)
	group.GET("/list", h.List)
	group.DELETE("/delete", h.Delete)
	return h
}

func (h *APITokenHandler) Create(c echo.Context) error {
	var req tokenV1.CreateAPITokenReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	resp, err := h.usecase.CreateMCPToken(c.Request().Context(), authInfo.UserId, &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to create api token", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *APITokenHandler) List(c echo.Context) error {
	var req tokenV1.APITokenListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	resp, err := h.usecase.ListMCPToken(c.Request().Context(), authInfo.UserId, req.KbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to list api tokens", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *APITokenHandler) Delete(c echo.Context) error {
	var req tokenV1.DeleteAPITokenReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	if err := h.usecase.DeleteMCPToken(c.Request().Context(), authInfo.UserId, req.KbID, req.ID); err != nil {
		return h.NewResponseWithError(c, "failed to delete api token", err)
	}
	return h.NewResponseWithData(c, nil)
}
