package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestShareContentRequiresCredentialsBeforeResolvingKnowledgeBase(t *testing.T) {
	// Nil dependencies deliberately prove that anonymous requests cannot even
	// resolve a KB or reach a document/search/chat/history handler.
	cfg := &config.Config{}
	m := NewShareAuthMiddleware(cfg, log.NewLogger(cfg), nil, nil)
	for _, path := range []string{
		"/share/v1/node/list", "/share/v1/node/detail", "/share/v1/nav/list",
		"/share/v1/common/file/download/url", "/share/v1/chat/message",
		"/share/v1/chat/search", "/share/v1/chat/widget",
		"/share/v1/conversation/detail", "/share/v1/conversation/history",
	} {
		for _, header := range []string{"", " ", "Bearer ", "Bearer   ", "Basic legacy-password"} {
			t.Run(path+"/"+header, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, path, nil)
				r.Header.Set("Authorization", header)
				r.AddCookie(&http.Cookie{Name: domain.SessionName, Value: "legacy-shared-password-session"})
				w := httptest.NewRecorder()
				c := echo.New().NewContext(r, w)
				if err := m.Authorize(func(echo.Context) error {
					t.Fatal("anonymous caller reached content handler")
					return nil
				})(c); err != nil {
					t.Fatal(err)
				}
				var body domain.PWResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if w.Code != http.StatusUnauthorized || body.Code != domain.SessionInvalidCode || body.Success {
					t.Fatalf("anonymous access accepted: status=%d body=%+v", w.Code, body)
				}
				if w.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("identity-dependent response is cacheable")
				}
			})
		}
	}
}

func TestShareReaderSessionsPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for local transaction test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	cfg := &config.Config{}
	cfg.Auth.JWT.Secret = "reader-session-regression-only"
	logger := log.NewLogger(cfg)
	tokens := pg.NewAPITokenRepo(&store.DB{DB: tx}, logger, nil)
	m := NewShareAuthMiddleware(cfg, logger, nil, tokens)
	userID := uuid.NewString()
	if err := tx.Create(&domain.User{ID: userID, Account: "reader_test_" + uuid.NewString(), Role: consts.UserRoleUser, Password: "unused"}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	newToken := func() (string, string) {
		t.Helper()
		sessionID := uuid.NewString()
		if err := tokens.AccountSessions().Create(ctx, sessionID, userID, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id": userID, "jti": sessionID, "exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte(cfg.Auth.JWT.Secret))
		if err != nil {
			t.Fatal(err)
		}
		return sessionID, token
	}
	check := func(token string, want bool) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/share/v1/node/list", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		c := echo.New().NewContext(r, httptest.NewRecorder())
		if got := m.applyJWTAuth(c, "test-kb"); got != want {
			t.Fatalf("reader session authentication=%v, want %v", got, want)
		}
		if want && domain.GetShareUserID(c) != userID {
			t.Fatal("authenticated identity was not forwarded to document permissions")
		}
	}
	id, token := newToken()
	check(token, true)
	if err := tokens.AccountSessions().Revoke(ctx, id, userID); err != nil {
		t.Fatal(err)
	}
	check(token, false)
	id, token = newToken()
	if err := tx.Model(&domain.AccountSession{}).Where("id=?", id).Update("last_seen_at", time.Now().Add(-31*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	check(token, false)
	_, token = newToken()
	if err := tx.Delete(&domain.User{}, "id=?", userID).Error; err != nil {
		t.Fatal(err)
	}
	check(token, false)
}
