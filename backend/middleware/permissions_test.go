package middleware

import (
	"context"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisabledPermissionsStillRequireAuthentication(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "true")
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{Secret: "test-only"}}}
	m := NewJWTMiddleware(cfg, log.NewLogger(cfg), nil, nil)
	for _, token := range []string{"", "Bearer invalid.jwt.value", "Bearer invalid-token"} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/node/list?kb_id=a", nil)
		r.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		c := echo.New().NewContext(r, w)
		if err := m.Authorize(func(c echo.Context) error { t.Fatal("unauthenticated request reached handler"); return nil })(c); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", w.Code)
		}
	}
}

func TestDisabledPermissionsPreserveTokenScope(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "true")
	cfg := &config.Config{}
	m := NewJWTMiddleware(cfg, log.NewLogger(cfg), nil, nil)
	for _, tc := range []struct {
		kb         string
		permission consts.UserKBPermission
		want       int
	}{
		{"a", consts.UserKBPermissionNotNull, http.StatusNoContent},
		{"b", consts.UserKBPermissionNotNull, http.StatusForbidden},
		{"a", consts.UserKBPermissionDocManage, http.StatusForbidden},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/node/list?kb_id="+tc.kb, nil)
		r = r.WithContext(context.WithValue(r.Context(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{IsToken: true, KBId: "a", Permission: consts.UserKBPermissionNotNull}))
		w := httptest.NewRecorder()
		c := echo.New().NewContext(r, w)
		if err := m.ValidateKBUserPerm(tc.permission)(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })(c); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.want {
			t.Fatalf("got %d, want %d", w.Code, tc.want)
		}
	}
}

func TestPermissionSettingsAreFrozenOnlyWhileDisabled(t *testing.T) {
	for _, mode := range []string{"true", "false"} {
		t.Setenv("DISABLE_PERMISSION_LEVELS", mode)
		for _, path := range []string{"/api/v1/enterprise/permission_levels", "/api/v1/node/permission/edit", "/api/v1/nav/permission", "/api/v1/enterprise/departments", "/api/v1/node/detail"} {
			c := echo.New().NewContext(httptest.NewRequest(http.MethodPatch, path, nil), httptest.NewRecorder())
			c.SetPath(path)
			called := false
			err := suspendPermissionWrites(func(c echo.Context) error { called = true; return nil })(c)
			if err != nil {
				t.Fatal(err)
			}
			if called != (mode == "false" || path != "/api/v1/enterprise/permission_levels") {
				t.Fatalf("unexpected write policy: %s %s", mode, path)
			}
		}
	}
}
