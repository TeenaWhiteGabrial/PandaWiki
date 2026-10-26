package pg

import (
	"context"
	"os"
	"testing"

	v1 "github.com/chaitin/panda-wiki/api/user/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestListUsersManagerOnlySeesTheirDepartment(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN to run PostgreSQL integration tests")
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

	rootID, ownDepartmentID, siblingDepartmentID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	departments := []domain.Department{
		{ID: rootID, Name: "root", Path: rootID, Status: domain.DepartmentStatusActive},
		{ID: ownDepartmentID, Name: "own", ParentID: &rootID, Path: rootID + "/" + ownDepartmentID, Status: domain.DepartmentStatusActive},
		{ID: siblingDepartmentID, Name: "sibling", ParentID: &rootID, Path: rootID + "/" + siblingDepartmentID, Status: domain.DepartmentStatusActive},
	}
	for _, department := range departments {
		if err := tx.Create(&department).Error; err != nil {
			t.Fatal(err)
		}
	}

	type testUser struct {
		userID string
		role   consts.UserRole
	}
	managerID := uuid.NewString()
	rootManagerID := uuid.NewString()
	users := []testUser{
		{userID: managerID, role: consts.UserRoleManager},     // legacy manager rows without a primary marker
		{userID: uuid.NewString(), role: consts.UserRoleUser}, // current department, new primary marker
		{userID: uuid.NewString(), role: consts.UserRoleUser}, // current department, legacy rows without marker
		{userID: uuid.NewString(), role: consts.UserRoleUser}, // sibling department under the same ancestor
		{userID: uuid.NewString(), role: consts.UserRoleUser}, // root department only
		{userID: rootManagerID, role: consts.UserRoleManager}, // primary root plus a non-primary child
	}
	for index, item := range users {
		user := domain.User{
			ID:       item.userID,
			Account:  "user-scope-test-" + item.userID,
			Name:     "scope test",
			Password: "unused",
			Role:     item.role,
		}
		if err := tx.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		assignments := []domain.UserDepartment{{UserID: item.userID, DepartmentID: rootID}}
		switch index {
		case 0:
			assignments = append(assignments, domain.UserDepartment{UserID: item.userID, DepartmentID: ownDepartmentID})
		case 1:
			assignments = append(assignments, domain.UserDepartment{UserID: item.userID, DepartmentID: ownDepartmentID, IsPrimary: true})
		case 2:
			assignments = append(assignments, domain.UserDepartment{UserID: item.userID, DepartmentID: ownDepartmentID})
		case 3:
			assignments = append(assignments, domain.UserDepartment{UserID: item.userID, DepartmentID: siblingDepartmentID, IsPrimary: true})
		}
		if index == 4 {
			assignments[0].IsPrimary = true
		}
		if index == 5 {
			assignments[0].IsPrimary = true
			assignments = append(assignments, domain.UserDepartment{UserID: item.userID, DepartmentID: ownDepartmentID})
		}
		if err := tx.Create(&assignments).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := &UserRepository{db: &store.DB{DB: tx}}
	listed, total, err := repo.ListUsersWithEnterpriseAccess(context.Background(), managerID, &v1.UserListReq{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(listed) != 3 {
		t.Fatalf("expected manager and two users in their department, got total=%d users=%d", total, len(listed))
	}
	allowedIDs := map[string]bool{
		managerID:       true,
		users[1].userID: true,
		users[2].userID: true,
	}
	for _, user := range listed {
		if !allowedIDs[user.ID] {
			t.Errorf("manager saw user %s outside their department", user.ID)
		}
	}

	effectiveManagerDepartments, err := (&EnterpriseRepository{db: &store.DB{DB: tx}}).GetEffectiveDepartmentIDs(context.Background(), managerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(effectiveManagerDepartments) != 1 || effectiveManagerDepartments[0] != ownDepartmentID {
		t.Fatalf("expected legacy manager's effective department %s, got %v", ownDepartmentID, effectiveManagerDepartments)
	}
	rootManagerDepartments, err := (&EnterpriseRepository{db: &store.DB{DB: tx}}).GetEffectiveDepartmentIDs(context.Background(), rootManagerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rootManagerDepartments) != 1 || rootManagerDepartments[0] != rootID {
		t.Fatalf("expected primary department %s to override inherited child rows, got %v", rootID, rootManagerDepartments)
	}
	resolved, err := (&EnterpriseRepository{db: &store.DB{DB: tx}}).ResolveEffectiveDepartmentIDs(context.Background(), []string{rootID, siblingDepartmentID})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved[0] != siblingDepartmentID {
		t.Fatalf("expected incoming ancestor chain to resolve to sibling department %s, got %v", siblingDepartmentID, resolved)
	}

	rootListed, rootTotal, err := repo.ListUsersWithEnterpriseAccess(context.Background(), rootManagerID, &v1.UserListReq{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if rootTotal != 2 || len(rootListed) != 2 {
		t.Fatalf("expected primary root manager to see only themselves and root-only users, got total=%d users=%d", rootTotal, len(rootListed))
	}
	for _, user := range rootListed {
		if user.ID != rootManagerID && user.ID != users[4].userID {
			t.Errorf("root manager saw user %s assigned to a child department", user.ID)
		}
	}
}
