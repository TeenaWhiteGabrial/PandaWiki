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
	"github.com/chaitin/panda-wiki/store/cache"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAdministratorAccessPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	cfg := &config.Config{}
	cfg.Auth.JWT.Secret = "administrator-access-test"
	storeDB := &store.DB{DB: tx}
	logger := log.NewLogger(cfg)
	access := pg.NewUserAccessRepository(storeDB, logger)
	m := NewJWTMiddleware(cfg, logger, access, nil)
	var tokenCache *cache.Cache
	if addr := os.Getenv("PANDA_WIKI_TEST_REDIS_ADDR"); addr != "" {
		tokenCache = &cache.Cache{Client: redis.NewClient(&redis.Options{Addr: addr})}
		defer tokenCache.Close()
		if err := tokenCache.Ping(context.Background()).Err(); err != nil {
			t.Fatal(err)
		}
		m.apiTokenRepo = pg.NewAPITokenRepo(storeDB, logger, tokenCache)
	}
	check := func(token string, status int) {
		t.Helper()
		called := false
		handler := m.Authorize(func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		c := echo.New().NewContext(req, recorder)
		c.SetPath("/api/v1/user")
		if err := handler(c); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != status || called != (status == http.StatusOK) {
			t.Fatalf("status=%d handler=%v expected=%d", recorder.Code, called, status)
		}
		if status == http.StatusForbidden {
			var response domain.PWResponse
			_ = json.Unmarshal(recorder.Body.Bytes(), &response)
			if response.Code != domain.AdministratorOnlyCode {
				t.Fatal("missing explicit administrator denial")
			}
		}
	}
	sign := func(id string) string {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": id, "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(cfg.Auth.JWT.Secret))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	for _, role := range []consts.UserRole{consts.UserRoleAdmin, consts.UserRoleManager, consts.UserRoleUser, "unknown"} {
		t.Run(string(role), func(t *testing.T) {
			id := uuid.NewString()
			user := &domain.User{ID: id, Account: "access-test-" + id, Name: "temporary access test", Password: "unused", Role: role}
			if err := tx.Create(user).Error; err != nil {
				t.Fatal(err)
			}
			status := http.StatusForbidden
			if consts.IsAdministrator(role) {
				status = http.StatusOK
			}
			jwtToken := sign(id)
			check(jwtToken, status)
			if tokenCache != nil {
				apiToken := &domain.APIToken{ID: uuid.NewString(), Name: "temporary API test", Token: "test-" + uuid.NewString(), UserID: id, KbId: uuid.NewString(), Permission: consts.UserKBPermissionFullControl}
				if err := tx.Create(apiToken).Error; err != nil {
					t.Fatal(err)
				}
				defer tokenCache.Del(context.Background(), "api_token:"+apiToken.Token)
				check(apiToken.Token, status)
				if consts.IsAdministrator(role) {
					if err := tx.Model(user).Update("role", consts.UserRoleUser).Error; err != nil {
						t.Fatal(err)
					}
					check(jwtToken, http.StatusForbidden)
					check(apiToken.Token, http.StatusForbidden)
				}
			}
		})
	}
	check(sign(uuid.NewString()), http.StatusForbidden)
	missingID, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(cfg.Auth.JWT.Secret))
	check(missingID, http.StatusUnauthorized)
}
