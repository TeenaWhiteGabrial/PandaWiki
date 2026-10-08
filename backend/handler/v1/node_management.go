package v1

import (
	nodeV1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/labstack/echo/v4"
)

func (h *NodeHandler) GetNodeManagement(c echo.Context) error {
	var req nodeV1.NodePermissionReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	result, err := h.usecase.GetNodeManagement(c.Request().Context(), req.KbId, req.ID)
	if err != nil {
		return h.NewResponseWithError(c, "获取文档管理权限失败", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NodeHandler) EditNodeManagement(c echo.Context) error {
	var req nodeV1.NodeManagementEditReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := h.usecase.EditNodeManagement(c.Request().Context(), &req); err != nil {
		return h.NewResponseWithError(c, "更新文档管理权限失败", err)
	}
	return h.NewResponseWithData(c, nil)
}

func (h *NodeHandler) NodePermissionOptions(c echo.Context) error {
	result, err := h.usecase.NodePermissionOptions(c.Request().Context())
	if err != nil {
		return h.NewResponseWithError(c, "获取授权对象失败", err)
	}
	return h.NewResponseWithData(c, result)
}
