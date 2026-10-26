package v1

import (
	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/labstack/echo/v4"
)

func (h *NodeHandler) ReadingAudience(c echo.Context) error {
	var req v1.ReadingAudienceReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "参数无效", err)
	}
	result, err := h.usecase.ReadingAudience(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "查阅范围计算失败", err)
	}
	return h.NewResponseWithData(c, result)
}
