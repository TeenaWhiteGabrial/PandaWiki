package middleware

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSecurityRejectsHostAndCrossOrigin(t *testing.T) {
	t.Setenv("SECURITY_ALLOWED_HOSTS", "localhost, wiki.example.test")
	t.Setenv("SECURITY_ALLOWED_ORIGINS", "https://portal.example.test")
	e := echo.New()
	e.Use(HTTPSecurity())
	e.GET("/", func(c echo.Context) error { return c.JSON(200, map[string]bool{"ok": true}) })
	for _, tc := range []struct {
		host, origin string
		status       int
		allow        string
	}{
		{"localhost:8000", "", 200, ""}, {"audit.invalid", "", 421, ""}, {"localhost:8000", "https://audit.invalid", 403, ""}, {"wiki.example.test", "https://wiki.example.test", 200, "https://wiki.example.test"}, {"localhost:8000", "https://portal.example.test", 200, "https://portal.example.test"},
	} {
		r := httptest.NewRequest("GET", "http://localhost/", nil)
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != tc.status || w.Header().Get("Access-Control-Allow-Origin") != tc.allow {
			t.Fatalf("%+v: status=%d, origin=%q", tc, w.Code, w.Header().Get("Access-Control-Allow-Origin"))
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("security headers missing")
		}
	}
	r := httptest.NewRequest(http.MethodOptions, "http://localhost/", nil)
	r.Header.Set("Origin", "https://portal.example.test")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
