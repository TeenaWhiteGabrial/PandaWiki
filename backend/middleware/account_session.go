package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func checkAccountSession(c echo.Context, claims jwt.MapClaims, repo *pg.AccountSessionRepository, cfg *config.Config) (string, *domain.AccountSessionState, error) {
	id, _ := claims["jti"].(string)
	userID, _ := claims["id"].(string)
	if id == "" || userID == "" || repo == nil {
		return "", nil, fmt.Errorf("session is invalid")
	}
	idle := cfg.Auth.JWT.SessionIdleSeconds
	if idle <= 0 {
		idle = 1800
	}
	state, err := repo.Validate(c.Request().Context(), id, userID, time.Now(), time.Duration(idle)*time.Second)
	if err != nil || state == nil {
		return "", nil, fmt.Errorf("session expired or revoked")
	}
	return id, state, nil
}
func invalidAccountSession(c echo.Context) error {
	ClearBrowserSession(c)
	return c.JSON(http.StatusUnauthorized, domain.PWResponse{Success: false, Code: domain.SessionInvalidCode, Message: "登录已过期或已退出，请重新登录"})
}
func requiresPasswordChange(c echo.Context, required bool) bool {
	if !required {
		return false
	}
	path := c.Request().URL.Path
	switch path {
	case "/api/v1/user", "/share/v1/auth/user", "/api/v1/user/logout", "/share/v1/auth/logout/account", "/api/v1/user/change_password", "/share/v1/auth/change_password":
		return false
	}
	return true
}
func passwordChangeRequired(c echo.Context) error {
	return c.JSON(http.StatusForbidden, domain.PWResponse{Success: false, Code: domain.PasswordChangeRequiredCode, Message: "请先修改初始密码"})
}

// Expose only a non-secret expiry timestamp for UI timers, never the JWT.
func accountSessionExpiresAt(claims jwt.MapClaims) int64 {
	expiry, err := claims.GetExpirationTime()
	if err != nil || expiry == nil {
		return 0
	}
	return expiry.Unix()
}
