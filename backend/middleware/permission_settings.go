package middleware

import (
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/labstack/echo/v4"
	"net/http"
)

// Keep archived rules intact while their settings are unavailable in the UI.
// This wrapper executes only after Authorize has authenticated the caller.
func suspendPermissionWrites(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Path()
		permissionSetting := path == "/api/v1/enterprise/permission_levels"
		if config.PermissionLevelsDisabled() && permissionSetting && c.Request().Method != http.MethodGet {
			return c.JSON(http.StatusForbidden, domain.PWResponse{Success: false, Message: "等级权限已暂停，原授权配置保留"})
		}
		return next(c)
	}
}
