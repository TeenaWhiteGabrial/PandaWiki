package pg

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// All fixtures and changes are rolled back, including on test failure.
func TestNavigationManagementPostgresRoundTrip(t *testing.T) {
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
	ctx := context.Background()
	kbID, userID, navID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	user := &domain.User{ID: userID, Account: "nav-test-" + userID, Name: "temporary navigation test", Password: "unused", Role: consts.UserRoleUser}
	kb := &domain.KnowledgeBase{ID: kbID, Name: "temporary navigation test"}
	nav := &domain.Nav{ID: navID, KbID: kbID, Name: "temporary navigation test", Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{userID}}}
	direct := &domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: navID, Type: domain.NodeTypeDocument, Name: "direct", CreatorId: userID, EditorId: userID}
	folder := &domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: navID, Type: domain.NodeTypeFolder, Name: "folder", CreatorId: userID, EditorId: userID}
	nested := &domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: navID, ParentID: folder.ID, Type: domain.NodeTypeDocument, Name: "nested", CreatorId: userID, EditorId: userID}
	for _, fixture := range []any{user, kb, nav, direct, folder, nested} {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	storeDB := &store.DB{DB: tx}
	repo := &NavRepository{db: storeDB}
	grants := []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{userID}, DirectoryScope: domain.ManagementScopeTree}}
	if err := repo.UpdateManagementGrants(ctx, kbID, navID, grants); err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetByKBID(ctx, kbID, navID)
	if err != nil || saved.Permissions.ScopeType != "user" || len(saved.Permissions.AllowedUserIDs) != 1 {
		t.Fatalf("management save overwrote reading: %v", err)
	}
	access, err := documentManagementAccess(ctx, storeDB, kbID, userID)
	if err != nil || access.Global || access.HasAnyGrant() || access.CanManageNode(nested.ID) {
		t.Fatalf("ordinary user gained management from a navigation grant: %v", err)
	}
	// Even a legacy full-control membership cannot elevate an ordinary user.
	if err := tx.Create(&domain.KBUsers{KBId: kbID, UserId: userID, Perm: consts.UserKBPermissionFullControl}).Error; err != nil {
		t.Fatal(err)
	}
	userAccess := &UserAccessRepository{db: storeDB}
	if allowed, err := userAccess.ValidateKBPerm(kbID, userID, consts.UserKBPermissionDocManage); err != nil || allowed {
		t.Fatalf("legacy membership bypassed administrator policy: %v", err)
	}
	access, err = documentManagementAccess(ctx, storeDB, kbID, userID)
	if err != nil || access.Global || access.CanManageNode(direct.ID) {
		t.Fatalf("ordinary membership allowed management: %v", err)
	}
	// Reading edits cannot inject or remove management grants.
	if err := repo.UpdatePermissions(ctx, kbID, navID, domain.NodePermissions{ScopeType: "department", AllowedDepartmentIDs: []string{"read-department"}}); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetByKBID(ctx, kbID, navID)
	if err != nil || len(saved.Permissions.ManagementGrants) != 1 {
		t.Fatalf("reading save erased management: %v", err)
	}
	grants[0].DirectoryScope = domain.ManagementScopeCurrent
	if err := repo.UpdateManagementGrants(ctx, kbID, navID, grants); err != nil {
		t.Fatal(err)
	}
	access, err = documentManagementAccess(ctx, storeDB, kbID, userID)
	if err != nil || access.CanManageNode(direct.ID) || access.CanManageNode(nested.ID) {
		t.Fatalf("ordinary user gained management from a current-directory grant: %v", err)
	}
	if err := tx.Model(user).Update("role", consts.UserRoleManager).Error; err != nil {
		t.Fatal(err)
	}
	access, err = documentManagementAccess(ctx, storeDB, kbID, userID)
	if err != nil || !access.Global || !access.CanManageNode(nested.ID) {
		t.Fatalf("administrator lost existing document management: %v", err)
	}
	if err := repo.UpdateManagementGrants(ctx, uuid.NewString(), navID, grants); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-knowledge-base update accepted: %v", err)
	}
	if _, err := repo.GetByKBID(ctx, uuid.NewString(), navID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-knowledge-base read accepted: %v", err)
	}
	// Published names stay stable while authorization uses current permissions.
	release := &domain.KBRelease{ID: uuid.NewString(), KBID: kbID, PublisherId: userID}
	if err := tx.Create(release).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := &domain.NavRelease{ID: uuid.NewString(), NavID: navID, ReleaseID: release.ID, KbID: kbID, Name: "published name", Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"old-user"}}}
	if err := tx.Create(snapshot).Error; err != nil {
		t.Fatal(err)
	}
	list, err := repo.GetReleaseList(ctx, kbID)
	if err != nil || len(list) != 1 || list[0].Name != snapshot.Name || list[0].Permissions.ScopeType != "department" {
		t.Fatalf("published navigation used stale authorization: %v, %#v", err, list)
	}
	if err := repo.UpdateManagementGrants(ctx, kbID, navID, nil); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetByKBID(ctx, kbID, navID)
	if err != nil || len(saved.Permissions.ManagementGrants) != 0 || saved.Permissions.ScopeType != "department" {
		t.Fatalf("removing management grant corrupted reading: %v", err)
	}
}
