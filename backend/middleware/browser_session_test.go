package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCookieSessionBoundary(t *testing.T) {
	t.Setenv("ENV", "production")
	for _, tc := range []struct {
		name, path, method, cookie, header, origin, csrf, wantAuth string
		wantStatus                                                 int
	}{
		{"reader cookie", "/share/v1/node/list", "GET", ReaderSessionCookie, "", "", "", "Bearer signed", 204},
		{"admin cookie", "/api/v1/user", "GET", AdminSessionCookie, "", "", "", "Bearer signed", 204},
		{"reader cannot become admin", "/api/v1/user", "GET", ReaderSessionCookie, "", "", "", "", 204},
		{"admin cannot become reader", "/share/v1/node/list", "GET", AdminSessionCookie, "", "", "", "", 204},
		{"legacy cookie ignored", "/api/v1/user", "GET", "panda_wiki_token", "", "", "", "", 204},
		{"explicit API credential wins", "/api/v1/user", "POST", AdminSessionCookie, "Bearer api-key", "", "", "Bearer api-key", 204},
		{"form without CSRF", "/api/v1/user/create", "POST", AdminSessionCookie, "", "http://wiki.test", "", "", 403},
		{"sibling origin", "/api/v1/user/create", "POST", AdminSessionCookie, "", "http://evil.wiki.test", "1", "", 403},
		{"same origin write", "/api/v1/user/create", "POST", AdminSessionCookie, "", "http://wiki.test", "1", "Bearer signed", 204},
		{"login CSRF", "/api/v1/user/login", "POST", "", "", "http://wiki.test", "", "", 403},
		{"reader login", "/share/v1/auth/login/account", "POST", "", "", "http://wiki.test", "1", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://wiki.test"+tc.path, nil)
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: tc.cookie, Value: "signed"})
			}
			r.Header.Set("Authorization", tc.header)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-PW-CSRF", tc.csrf)
			w := httptest.NewRecorder()
			c := echo.New().NewContext(r, w)
			err := BrowserSessions()(func(c echo.Context) error {
				if c.Request().Header.Get("Authorization") != tc.wantAuth {
					t.Errorf("wrong identity: %q", c.Request().Header.Get("Authorization"))
				}
				return c.NoContent(204)
			})(c)
			if err != nil || w.Code != tc.wantStatus {
				t.Fatalf("status=%d err=%v", w.Code, err)
			}
		})
	}
}

func TestSessionCookieFlagsAndDeletion(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		r := httptest.NewRequest("POST", scheme+"://wiki.test/share/v1/auth/login/account", nil)
		w := httptest.NewRecorder()
		c := echo.New().NewContext(r, w)
		SetBrowserSession(c, "signed", 28800)
		cookie := w.Result().Cookies()[0]
		if cookie.Name != ReaderSessionCookie || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure != (scheme == "https") || cookie.Path != "/" {
			t.Fatalf("incorrect cookie flags: %+v", cookie)
		}
		if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("session response cached")
		}
	}
	r := httptest.NewRequest("POST", "http://wiki.test/api/v1/user/logout", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	c := echo.New().NewContext(r, w)
	ClearBrowserSession(c)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge != -1 {
			t.Fatal("cookie not expired")
		}
	}
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("HTTPS proxy lost Secure")
	}
}
