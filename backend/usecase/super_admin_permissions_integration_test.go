package usecase

import (
	"context"
	"os"
	"testing"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
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

// Use a rollback-only local fixture, including current account-role changes.
func TestSuperAdminDocumentFallbackPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for PostgreSQL integration")
	}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
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
	ctx := context.Background()
	kbID, navID, folderID, docID, releaseID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	rootID, managerID, readerID, levelID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	policy := domain.NodePermissions{
		ScopeType: "custom", AllowedDepartmentIDs: []string{uuid.NewString()},
		AllowedPermissionLevelIDs: []string{levelID},
		AllowedUserIDs:            []string{uuid.NewString()}, DeniedUserIDs: []string{rootID, managerID, readerID},
	}
	inherit := true
	for _, fixture := range []any{
		&domain.PermissionLevel{ID: levelID, Code: levelID, Name: "restricted fixture", Rank: 100, Status: domain.PermissionLevelStatusActive},
		&domain.User{ID: rootID, Account: "recovery_" + rootID, Password: "unused", Role: consts.UserRoleAdmin},
		&domain.User{ID: managerID, Account: "manager_" + managerID, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: readerID, Account: "reader_" + readerID, Password: "unused", Role: consts.UserRoleUser},
		&domain.KnowledgeBase{ID: kbID, Name: "super-admin fallback fixture"},
		&domain.Nav{ID: navID, KbID: kbID, Name: "restricted column", Permissions: policy},
		&domain.Node{ID: folderID, KBID: kbID, NavId: navID, Type: domain.NodeTypeFolder, Name: "restricted folder", Permissions: policy, CreatorId: rootID, EditorId: rootID},
		&domain.Node{ID: docID, KBID: kbID, NavId: navID, ParentID: folderID, Type: domain.NodeTypeDocument, Name: "restricted document", Permissions: domain.NodePermissions{Inherit: &inherit}, CreatorId: rootID, EditorId: rootID, Meta: domain.NodeMeta{FileKey: "private/fallback.pdf", FileReadonly: true}},
		&domain.KBRelease{ID: releaseID, KBID: kbID},
		&domain.NavRelease{ID: uuid.NewString(), NavID: navID, KbID: kbID, ReleaseID: releaseID, Name: "restricted column", Permissions: policy},
	} {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, nodeID := range []string{folderID, docID} {
		var node domain.Node
		if err := tx.First(&node, "id = ?", nodeID).Error; err != nil {
			t.Fatal(err)
		}
		version := domain.NodeRelease{ID: uuid.NewString(), NodeID: nodeID, KBID: kbID, ParentID: node.ParentID, Type: node.Type, Name: node.Name, Meta: node.Meta}
		if err := tx.Create(&version).Error; err != nil {
			t.Fatal(err)
		}
		link := domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: kbID, NodeID: nodeID, NodeReleaseID: version.ID, ReleaseID: releaseID, NavID: navID}
		if err := tx.Create(&link).Error; err != nil {
			t.Fatal(err)
		}
	}
	storeDB := &store.DB{DB: tx}
	l := log.NewLogger(&config.Config{})
	repo := pg.NewEnterpriseRepository(storeDB, l)
	nodes, navs := pg.NewNodeRepository(storeDB, l), pg.NewNavRepository(storeDB, l)
	enterprise := &EnterpriseUsecase{repo: repo, nodeRepo: nodes, navRepo: navs}
	u := &NodeUsecase{nodeRepo: nodes, navRepo: navs, enterprise: enterprise}
	llm := &LLMUsecase{nodeRepo: nodes, enterprise: enterprise}
	for _, userID := range []string{rootID, managerID, readerID} {
		want := userID == rootID
		principal, err := enterprise.BuildShareAccessContext(ctx, userID)
		if err != nil || principal.IsSuperAdmin != want {
			t.Fatalf("role-derived principal: %+v %v", principal, err)
		}
		// This is also the permission check used by original/preview downloads.
		if code := u.ValidateNodePerm(ctx, kbID, docID, userID); (code == nil) != want {
			t.Fatalf("document/download permission for %s: %+v", userID, code)
		}
		groups, err := u.GetShareNodeTree(ctx, kbID, userID, v1.NodeListGroupNavReq{PathID: docID})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, group := range groups {
			for _, node := range group.List {
				if node.ID == docID && !node.PathOnly {
					found = true
				}
			}
		}
		if found != want {
			t.Fatalf("reader tree visibility for %s: %v", userID, found)
		}
		ranked, err := llm.filterRankedNodesByEnterpriseAccess(ctx, []*domain.RankedNodeChunks{{NodeID: docID, NodeName: "restricted document"}}, principal)
		if err != nil || (len(ranked) == 1) != want {
			t.Fatalf("QA document filter for %s: %+v %v", userID, ranked, err)
		}
		access, err := repo.DocumentManagementAccess(ctx, kbID, userID)
		if err != nil || access.Global != want || access.CanManageNode(docID) != want || access.CanManageNode(folderID) != want || access.CanManageNav(navID) != want {
			t.Fatalf("management fallback for %s: %+v %v", userID, access, err)
		}
	}
	if code := u.ValidateNodePerm(ctx, uuid.NewString(), docID, rootID); code == nil || *code != domain.ErrCodeNotFound {
		t.Fatalf("fallback changed knowledge-base ownership validation: %+v", code)
	}
	if code := u.ValidateNodePerm(ctx, kbID, uuid.NewString(), rootID); code == nil || *code != domain.ErrCodeNotFound {
		t.Fatalf("fallback authorized a nonexistent document: %+v", code)
	}
	// A role change takes effect on the next request without trusting JWT roles.
	if err := tx.Model(&domain.User{}).Where("id = ?", rootID).Update("role", consts.UserRoleManager).Error; err != nil {
		t.Fatal(err)
	}
	if code := u.ValidateNodePerm(ctx, kbID, docID, rootID); code == nil || *code != domain.ErrCodePermissionDenied {
		t.Fatalf("demoted account retained reading fallback: %+v", code)
	}
	access, err := repo.DocumentManagementAccess(ctx, kbID, rootID)
	if err != nil || access.Global || access.CanManageNode(docID) {
		t.Fatalf("demoted account retained global management: %+v %v", access, err)
	}
}
