package usecase

import (
	"context"
	"errors"
	"os"
	"testing"

	userV1 "github.com/chaitin/panda-wiki/api/user/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAdministratorLoginPostgres(t *testing.T) {
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
	cfg.Auth.JWT.Secret = "administrator-login-test"
	u := &UserUsecase{repo: pg.NewUserRepository(&store.DB{DB: tx}, log.NewLogger(cfg)), config: cfg}
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(&store.DB{DB: tx}, log.NewLogger(cfg))}
	hash, _ := bcrypt.GenerateFromPassword([]byte("temporary-test-password"), bcrypt.MinCost)
	for _, role := range []consts.UserRole{consts.UserRoleAdmin, consts.UserRoleManager, consts.UserRoleUser, "unknown"} {
		t.Run(string(role), func(t *testing.T) {
			id := uuid.NewString()
			user := &domain.User{ID: id, Account: "login-test-" + id, Name: "temporary login test", Password: string(hash), Role: role}
			if err := tx.Create(user).Error; err != nil {
				t.Fatal(err)
			}
			grants := []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{id}}}
			grantErr := enterprise.NormalizeManagementGrants(context.Background(), grants)
			if (grantErr == nil) != consts.IsAdministrator(role) {
				t.Fatalf("named management grant violates administrator restriction: %v", grantErr)
			}
			kbID := uuid.NewString()
			tokenCtx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: id, KBId: kbID, IsToken: true, Permission: consts.UserKBPermissionFullControl})
			managementAccess, accessErr := enterprise.ManagementAccess(tokenCtx, kbID)
			if accessErr != nil || managementAccess.Global != consts.IsAdministrator(role) {
				t.Fatalf("API token management access violates owner role: %v", accessErr)
			}
			token, err := u.VerifyUserAndGenerateToken(context.Background(), userV1.LoginReq{Account: user.Account, Password: "temporary-test-password"})
			if !consts.IsAdministrator(role) {
				if token != "" || !errors.Is(err, ErrAdministratorOnly) {
					t.Fatalf("non-admin issued token: %v", err)
				}
			} else {
				parsed, parseErr := jwt.Parse(token, func(token *jwt.Token) (any, error) { return []byte(cfg.Auth.JWT.Secret), nil })
				if err != nil || parseErr != nil || !parsed.Valid || parsed.Claims.(jwt.MapClaims)["id"] != id {
					t.Fatalf("administrator login failed: %v %v", err, parseErr)
				}
			}
			token, err = u.VerifyUserAndGenerateToken(context.Background(), userV1.LoginReq{Account: user.Account, Password: "wrong-password"})
			if token != "" || err == nil || errors.Is(err, ErrAdministratorOnly) {
				t.Fatal("credentials must be verified before role")
			}
			token, err = u.VerifyReaderAndGenerateToken(context.Background(), userV1.LoginReq{Account: user.Account, Password: "temporary-test-password"})
			if role == "unknown" {
				if token != "" || err == nil {
					t.Fatal("unknown role must not log in")
				}
			} else {
				parsed, parseErr := jwt.Parse(token, func(token *jwt.Token) (any, error) { return []byte(cfg.Auth.JWT.Secret), nil })
				if err != nil || parseErr != nil || !parsed.Valid || parsed.Claims.(jwt.MapClaims)["id"] != id {
					t.Fatalf("reader login failed: %v %v", err, parseErr)
				}
			}
			token, err = u.VerifyReaderAndGenerateToken(context.Background(), userV1.LoginReq{Account: user.Account, Password: "wrong-password"})
			if token != "" || err == nil {
				t.Fatal("reader credentials must still be verified")
			}
		})
	}
}
