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

type CollabHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	auth    middleware.AuthMiddleware
	usecase *usecase.NodeUsecase
}

func NewCollabHandler(echo *echo.Echo, baseHandler *handler.BaseHandler, logger *log.Logger, auth middleware.AuthMiddleware, usecase *usecase.NodeUsecase) *CollabHandler {
	handler := &CollabHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.collab"),
		auth:        auth,
		usecase:     usecase,
	}

	group := echo.Group("/api/v1/collab", handler.auth.Authorize, handler.auth.ValidateUserRole(consts.UserRoleAdmin))
	group.GET("/setting", handler.GetSetting)
	group.PUT("/setting", handler.UpdateSetting)

	return handler
}

// GetSetting
//
//	@Summary		get collaboration setting
//	@Description	get collaboration setting
//	@Tags			collab
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	domain.Response{data=domain.CollabSetting}
//	@Router			/api/v1/collab/setting [get]
func (h *CollabHandler) GetSetting(c echo.Context) error {
	setting, err := h.usecase.GetCollabSetting(c.Request().Context())
	if err != nil {
		return h.NewResponseWithError(c, "get collab setting failed", err)
	}
	return h.NewResponseWithData(c, setting)
}

// UpdateSetting
//
//	@Summary		update collaboration setting
//	@Description	update collaboration setting
//	@Tags			collab
//	@Accept			json
//	@Produce		json
//	@Param			setting	body		domain.CollabSetting	true	"collaboration setting"
//	@Success		200		{object}	domain.Response{data=domain.CollabSetting}
//	@Router			/api/v1/collab/setting [put]
func (h *CollabHandler) UpdateSetting(c echo.Context) error {
	var req domain.CollabSetting
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	setting, err := h.usecase.UpdateCollabSetting(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "update collab setting failed", err)
	}
	return h.NewResponseWithData(c, setting)
}
