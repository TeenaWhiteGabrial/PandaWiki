package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthenticationAndHeaderFiltering(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if strings.Contains(r.Header.Get("X-Forwarded-For"), "forged") || r.Header.Get("X-Real-IP") != "" {
			t.Error("untrusted forwarding identity escaped")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("proxy credentials leaked upstream")
		}
		w.Header().Set("Server", "nginx/secret-version")
		w.Header().Set("X-Powered-By", "secret-framework")
		w.WriteHeader(418)
		io.Copy(w, r.Body)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	handler := proxyHandler(u, "monitor", "test-password", true)
	for _, auth := range []string{"missing", "wrong", "valid"} {
		r := httptest.NewRequest("POST", "http://monitor/varz?x=1", strings.NewReader("body"))
		if auth != "missing" {
			r.SetBasicAuth("monitor", map[string]string{"wrong": "wrong", "valid": "test-password"}[auth])
		}
		r.Header.Set("X-Forwarded-For", "forged")
		r.Header.Set("X-Real-IP", "forged")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if auth != "valid" {
			if w.Code != 401 || hits != 0 {
				t.Fatalf("%s credentials reached upstream", auth)
			}
		} else if w.Code != 418 || w.Body.String() != "body" || hits != 1 {
			t.Fatalf("proxy failed: %d %q", w.Code, w.Body.String())
		}
		if w.Header().Get("Server") != "" || w.Header().Get("X-Powered-By") != "" {
			t.Fatal("upstream fingerprint escaped")
		}
	}
}

func TestAdminFilteringPreservesSessionCookiesAndBearer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer explicit-key" {
			t.Error("API key protocol changed")
		}
		http.SetCookie(w, &http.Cookie{Name: "pw_admin_session", Value: "opaque", HttpOnly: true})
		w.Header().Set("Server", "nginx")
		w.WriteHeader(401)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	r := httptest.NewRequest("GET", "http://admin/api/v1/user", nil)
	r.Header.Set("Authorization", "Bearer explicit-key")
	w := httptest.NewRecorder()
	proxyHandler(u, "", "", false).ServeHTTP(w, r)
	if w.Code != 401 || w.Header().Get("Server") != "" || !strings.Contains(w.Header().Get("Set-Cookie"), "HttpOnly") {
		t.Fatal("response filtering changed authentication")
	}
}
