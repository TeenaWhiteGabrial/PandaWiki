package v1

import (
	navV1 "github.com/chaitin/panda-wiki/api/nav/v1"
	"github.com/labstack/echo/v4"
)

func (h *NavHandler) GetNavManagement(c echo.Context) error {
	var req navV1.NavPermissionReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	result, err := h.usecase.GetManagement(c.Request().Context(), req.KbId, req.ID)
	if err != nil {
		return h.NewResponseWithError(c, "获取文档管理权限失败", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NavHandler) EditNavManagement(c echo.Context) error {
	var req navV1.NavManagementEditReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := h.usecase.EditManagement(c.Request().Context(), &req); err != nil {
		return h.NewResponseWithError(c, "更新文档管理权限失败", err)
	}
	return h.NewResponseWithData(c, nil)
}
