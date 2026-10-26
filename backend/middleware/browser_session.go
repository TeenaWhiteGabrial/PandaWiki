package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/domain"
)

const (
	AdminSessionCookie  = "pw_admin_session"
	ReaderSessionCookie = "pw_reader_session"
)

func sessionCookieName(c echo.Context) string {
	if strings.HasPrefix(c.Request().URL.Path, "/share/") {
		return ReaderSessionCookie
	}
	return AdminSessionCookie
}

// BrowserSessions translates only the appropriate HttpOnly cookie into the
// existing account authentication pipeline. Explicit API credentials keep their
// protocol and never implicitly fall back to a different browser identity.
func BrowserSessions() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			cookie, err := c.Cookie(sessionCookieName(c))
			cookieAuth := r.Header.Get("Authorization") == "" && err == nil && cookie.Value != ""
			login := r.URL.Path == "/api/v1/user/login" || r.URL.Path == "/share/v1/auth/login/account"
			unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
			if unsafe && (cookieAuth || login) {
				// A cross-site form cannot add this header. Origin validation is
				// also required when supplied, including sibling-site origins.
				origin := r.Header.Get("Origin")
				if r.Header.Get("X-PW-CSRF") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" ||
					(origin != "" && !allowedOrigin(origin, r.Host)) {
					return c.JSON(http.StatusForbidden, domain.PWResponse{Success: false, Message: "请求来源校验失败", Code: 40303})
				}
			}
			if cookieAuth {
				c.Response().Header().Set("Cache-Control", "private, no-store")
				r.Header.Set("Authorization", "Bearer "+cookie.Value)
			}
			return next(c)
		}
	}
}

func SetBrowserSession(c echo.Context, token string, maxAge int) {
	secure := c.IsTLS() || strings.EqualFold(c.Request().Header.Get("X-Forwarded-Proto"), "https") || os.Getenv("AUTH_COOKIE_SECURE") == "true"
	cookie := &http.Cookie{Name: sessionCookieName(c), Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
	if maxAge < 0 {
		cookie.Expires = time.Unix(1, 0).UTC()
	}
	c.SetCookie(cookie)
	c.Response().Header().Set("Cache-Control", "private, no-store")
}

func ClearBrowserSession(c echo.Context) {
	SetBrowserSession(c, "", -1)
	for _, name := range []string{"panda_wiki_token", "token", "_pw_auth_session"} {
		c.SetCookie(&http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
	}
}
