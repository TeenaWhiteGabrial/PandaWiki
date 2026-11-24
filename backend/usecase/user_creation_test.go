package usecase

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCreateUserUsesAuthorizedPrimaryDepartmentAndRole(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.AutoMigrate(&domain.User{}, &domain.Department{}, &domain.UserDepartment{}); err != nil {
		t.Fatal(err)
	}
	parentID, ownID, siblingID, childID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	departments := []domain.Department{
		{ID: parentID, Path: parentID, Name: "Parent"},
		{ID: ownID, Path: parentID + "/" + ownID, ParentID: &parentID, Name: "Own"},
		{ID: siblingID, Path: parentID + "/" + siblingID, ParentID: &parentID, Name: "Sibling"},
		{ID: childID, Path: parentID + "/" + ownID + "/" + childID, ParentID: &ownID, Name: "Child"},
	}
	if err := tx.Create(&departments).Error; err != nil {
		t.Fatal(err)
	}
	managerID, adminID := uuid.NewString(), uuid.NewString()
	actors := []domain.User{
		{ID: managerID, Role: consts.UserRoleManager, Account: "manager_" + managerID, Name: "Manager"},
		{ID: adminID, Role: consts.UserRoleAdmin, Account: "admin_" + adminID, Name: "Super admin"},
	}
	if err := tx.Create(&actors).Error; err != nil {
		t.Fatal(err)
	}
	assignments := []domain.UserDepartment{
		{UserID: managerID, DepartmentID: parentID},
		{UserID: managerID, DepartmentID: ownID, IsPrimary: true},
	}
	if err := tx.Create(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	fixture := &store.DB{DB: tx}
	logger := log.NewLogger(&config.Config{})
	u := &UserUsecase{repo: pg.NewUserRepository(fixture, logger), enterpriseRepo: pg.NewEnterpriseRepository(fixture, logger)}

	for _, tc := range []struct {
		name, actor, expectedPrimary string
		role                         consts.UserRole
		departments                  []string
		wantErr                      error
	}{
		{"own department", managerID, ownID, consts.UserRoleUser, []string{ownID}, nil},
		{"legacy ancestor ids in reversed order", managerID, ownID, consts.UserRoleUser, []string{ownID, parentID, ownID, parentID}, nil},
		{"parent", managerID, "", consts.UserRoleUser, []string{parentID}, ErrCreateUserDepartment},
		{"sibling", managerID, "", consts.UserRoleUser, []string{parentID, siblingID}, ErrCreateUserDepartment},
		{"child", managerID, "", consts.UserRoleUser, []string{parentID, ownID, childID}, ErrCreateUserDepartment},
		{"mixed assignments", managerID, "", consts.UserRoleUser, []string{ownID, siblingID}, ErrCreateUserDepartmentSelection},
		{"new administrator", managerID, "", consts.UserRoleManager, []string{ownID}, ErrCreateAdministrator},
		{"super admin can create administrator", adminID, siblingID, consts.UserRoleManager, []string{parentID, siblingID}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: tc.actor})
			id := uuid.NewString()
			newUser := &domain.User{ID: id, Account: "test_" + id[:8], Name: "New user", Role: tc.role, Password: "Test!Pass2048"}
			if err := u.CreateUser(ctx, newUser, tc.departments, consts.LicenseEditionFree); !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateUser returned %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if _, err := u.GetUser(ctx, id); !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("denied user exists or lookup failed: %v", err)
				}
				return
			}
			_, primary, err := u.GetUserDepartments(ctx, id)
			if err != nil || primary != tc.expectedPrimary {
				t.Fatalf("persisted primary is %q, want %q: %v", primary, tc.expectedPrimary, err)
			}
		})
	}
}
