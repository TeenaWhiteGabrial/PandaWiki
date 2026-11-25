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
	m := NewJWTMiddleware(cfg, logger, access)
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
		sessionID := uuid.NewString()
		var exists int64
		tx.Model(&domain.User{}).Where("id=?", id).Count(&exists)
		if exists == 1 {
			if err := access.AccountSessions().Create(context.Background(), sessionID, id, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": id, "jti": sessionID, "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(cfg.Auth.JWT.Secret))
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
			check("retired-api-token", http.StatusUnauthorized)
			if consts.IsAdministrator(role) {
				if err := tx.Model(user).Update("role", consts.UserRoleUser).Error; err != nil {
					t.Fatal(err)
				}
				check(jwtToken, http.StatusForbidden)
			}
		})
	}
	check(sign(uuid.NewString()), http.StatusUnauthorized)
	missingID, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(cfg.Auth.JWT.Secret))
	check(missingID, http.StatusUnauthorized)
}
