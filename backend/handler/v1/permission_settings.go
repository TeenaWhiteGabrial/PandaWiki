package v1

import (
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/labstack/echo/v4"
	"net/http"
)

type PermissionSettingsHandler struct{}

func NewPermissionSettingsHandler(e *echo.Echo, base *handler.BaseHandler, auth middleware.AuthMiddleware, repo *pg.SystemSettingRepo) *PermissionSettingsHandler {
	group := e.Group("/api/v1/system/permissions", auth.Authorize, auth.ValidateUserRole(consts.UserRoleAdmin))
	group.GET("", func(c echo.Context) error {
		enabled, err := repo.PermissionsEnabled(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "message": "读取权限配置失败"})
		}
		return base.NewResponseWithData(c, map[string]bool{"enabled": enabled})
	})
	group.PUT("", func(c echo.Context) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil || req.Enabled == nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"success": false, "message": "enabled 必须为布尔值"})
		}
		if err := repo.SetPermissionsEnabled(c.Request().Context(), *req.Enabled); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"success": false, "message": "保存权限配置失败"})
		}
		return base.NewResponseWithData(c, map[string]bool{"enabled": *req.Enabled})
	})
	return &PermissionSettingsHandler{}
}
