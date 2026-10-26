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

// Every fixture is confined to a transaction and rolled back.
func TestReadingAudiencePostgres(t *testing.T) {
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
	kbID, navID, rootID, childID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	adminID, managerID, readerA, readerB, readerC := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	inherit := true
	fixtures := []any{
		&domain.User{ID: adminID, Account: "reading-admin-" + adminID, Password: "unused", Role: consts.UserRoleAdmin},
		&domain.User{ID: managerID, Account: "reading-manager-" + managerID, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: readerA, Account: "reading-a-" + readerA, Name: "名单测试甲", Password: "unused", Role: consts.UserRoleUser},
		&domain.User{ID: readerB, Account: "reading-b-" + readerB, Name: "名单测试乙", Password: "unused", Role: consts.UserRoleUser},
		&domain.User{ID: readerC, Account: "reading-c-" + readerC, Password: "unused", Role: consts.UserRoleUser},
		&domain.KnowledgeBase{ID: kbID, Name: "reading preview test"},
		&domain.Nav{ID: navID, KbID: kbID, Name: "preview nav", Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{readerA, readerB}}},
		&domain.Node{ID: rootID, KBID: kbID, NavId: navID, ParentID: navID, Type: domain.NodeTypeFolder, Name: "真实权限来源", CreatorId: adminID, EditorId: adminID, Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{readerA, readerB, readerC}, DeniedUserIDs: []string{readerB}}},
		&domain.Node{ID: childID, KBID: kbID, NavId: navID, ParentID: rootID, Type: domain.NodeTypeDocument, Name: "reading child", CreatorId: adminID, EditorId: adminID, Permissions: domain.NodePermissions{Inherit: &inherit}},
	}
	for _, fixture := range fixtures {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	storeDB := &store.DB{DB: tx}
	l := log.NewLogger(&config.Config{})
	nodes, navs := pg.NewNodeRepository(storeDB, l), pg.NewNavRepository(storeDB, l)
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(storeDB, l), nodeRepo: nodes, navRepo: navs}
	u := &NodeUsecase{nodeRepo: nodes, navRepo: navs, enterprise: enterprise}
	ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: adminID})
	req := &v1.ReadingAudienceReq{KBID: kbID, IDs: []string{childID}, TargetType: "node", IncludeUsers: true}
	saved, err := u.ReadingAudience(ctx, req)
	if err != nil || saved.Total != 1 || len(saved.Users) != 1 || saved.Users[0].ID != readerA || saved.Source == nil || saved.Source.Name != "真实权限来源" {
		t.Fatalf("saved preview failed: %#v %v", saved, err)
	}
	// Compare the preview with the same user contexts used by live requests.
	for _, userID := range []string{adminID, managerID, readerA, readerB, readerC} {
		access, err := enterprise.BuildShareAccessContext(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		if got := enterprise.CanVisitable(saved.Permissions, access); got != (userID == readerA) {
			t.Fatalf("preview differs from actual reader check for %s", userID)
		}
	}
	req.Permissions = &domain.NodePermissions{ScopeType: "custom", AllowAllUsers: true, DeniedUserIDs: []string{readerA}}
	draft, err := u.ReadingAudience(ctx, req)
	if err != nil || draft.Total != 0 {
		t.Fatalf("unsaved exclusion not applied: %#v %v", draft, err)
	}
	child, err := nodes.GetNodeByID(ctx, childID)
	if err != nil || child.Permissions.Inherit == nil || !*child.Permissions.Inherit || len(child.Permissions.DeniedUserIDs) != 0 {
		t.Fatalf("preview modified stored permissions: %#v %v", child, err)
	}
	req = &v1.ReadingAudienceReq{KBID: kbID, IDs: []string{navID}, TargetType: "nav", Keyword: "测试乙", IncludeUsers: true, Page: 1, PageSize: 1}
	nav, err := u.ReadingAudience(ctx, req)
	if err != nil || nav.Total != 2 || nav.FilteredTotal != 1 || len(nav.Users) != 1 || nav.Users[0].ID != readerB {
		t.Fatalf("nav/search preview failed: %#v %v", nav, err)
	}
	deniedCtx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: managerID})
	if _, err := u.ReadingAudience(deniedCtx, req); err == nil {
		t.Fatal("manager outside target scope read the audience")
	}
	req = &v1.ReadingAudienceReq{KBID: uuid.NewString(), IDs: []string{childID}, TargetType: "node"}
	if _, err := u.ReadingAudience(ctx, req); err == nil {
		t.Fatal("cross-KB preview disclosed an audience")
	}
}
