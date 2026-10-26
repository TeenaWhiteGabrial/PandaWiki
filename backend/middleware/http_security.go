package middleware

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// With an explicit list, only those hosts are accepted. The fallback supports
// existing IP-based installations and local development, never arbitrary DNS names.
func AllowedRequestHost(raw string) bool {
	host := strings.ToLower(strings.TrimSuffix(raw, "."))
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = strings.ToLower(h)
	}
	host = strings.Trim(host, "[]")
	if host == "" || strings.ContainsAny(host, "/\\@ \t\r\n") {
		return false
	}
	configured := strings.TrimSpace(os.Getenv("SECURITY_ALLOWED_HOSTS"))
	if configured != "" {
		for _, value := range strings.Split(configured, ",") {
			if host == strings.ToLower(strings.TrimSpace(value)) {
				return true
			}
		}
		return false
	}
	return host == "localhost" || net.ParseIP(host) != nil || host == "api" || host == "panda-wiki-api" || host == "host.docker.internal"
}

func allowedOrigin(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return false
	}
	if strings.EqualFold(parsed.Host, host) {
		return true
	}
	for _, value := range strings.Split(os.Getenv("SECURITY_ALLOWED_ORIGINS"), ",") {
		if origin == strings.TrimSpace(value) {
			return true
		}
	}
	// Development frontends proxy to port 8000.
	return os.Getenv("ENV") == "local" && (origin == "http://localhost:5173" || origin == "http://localhost:3010" || origin == "http://127.0.0.1:5173" || origin == "http://127.0.0.1:3010")
}

func HTTPSecurity() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			h.Del("X-Powered-By")
			h.Del("Server")
			if !AllowedRequestHost(c.Request().Host) {
				return c.NoContent(http.StatusMisdirectedRequest)
			}
			origin := c.Request().Header.Get("Origin")
			if origin != "" {
				if !allowedOrigin(origin, c.Request().Host) {
					return c.NoContent(http.StatusForbidden)
				}
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-KB-ID, X-PW-Session-ID, X-PW-Cache-ID, X-PW-CSRF")
				h.Set("Access-Control-Allow-Credentials", "true")
				if c.Request().Method == http.MethodOptions {
					return c.NoContent(http.StatusNoContent)
				}
			}
			return next(c)
		}
	}
}
