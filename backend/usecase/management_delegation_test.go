package usecase

import (
	"context"
	"os"
	"testing"

	navV1 "github.com/chaitin/panda-wiki/api/nav/v1"
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

func TestManagementLocalDelegationPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set isolated PostgreSQL test database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	defer tx.Rollback()
	kb, nav, root, sub, doc := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	owner, local, leaf, reader := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	grant := func(id string) []domain.DocumentManagementGrant {
		return []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{id}, DirectoryScope: domain.ManagementScopeTree}}
	}
	for _, fixture := range []any{
		&domain.User{ID: owner, Account: owner, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: local, Account: local, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: leaf, Account: leaf, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: reader, Account: reader, Password: "unused", Role: consts.UserRoleUser},
		&domain.KnowledgeBase{ID: kb, Name: "isolated delegation test"},
		&domain.Nav{ID: nav, KbID: kb, Name: "Owner space", Permissions: domain.NodePermissions{ManagementGrants: grant(owner)}},
		&domain.Node{ID: root, KBID: kb, NavId: nav, Type: domain.NodeTypeFolder, Name: "Root", Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{reader}}},
		&domain.Node{ID: sub, KBID: kb, NavId: nav, ParentID: root, Type: domain.NodeTypeFolder, Name: "Sub"},
		&domain.Node{ID: doc, KBID: kb, NavId: nav, ParentID: sub, Type: domain.NodeTypeDocument, Name: "Doc"},
	} {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	stored := &store.DB{DB: tx}
	l := log.NewLogger(&config.Config{})
	nodes, navs := pg.NewNodeRepository(stored, l), pg.NewNavRepository(stored, l)
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(stored, l), nodeRepo: nodes, navRepo: navs}
	u := &NodeUsecase{nodeRepo: nodes, navRepo: navs, enterprise: enterprise}
	nu := &NavUsecase{navRepo: navs, enterprise: enterprise}
	ctx := func(id string) context.Context {
		return context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: id})
	}
	edit := func(actor, node, target string) error {
		return u.EditNodeManagement(ctx(actor), &v1.NodeManagementEditReq{KBID: kb, IDs: []string{node}, Grants: grant(target)})
	}
	if err := edit(owner, root, local); err != nil {
		t.Fatal(err)
	}
	if err := edit(local, sub, leaf); err != nil {
		t.Fatalf("local manager cannot delegate independently: %v", err)
	}
	for _, actor := range []string{owner, local, leaf} {
		a, err := enterprise.ManagementAccess(ctx(actor), kb)
		if err != nil || !a.CanManageNode(doc) {
			t.Fatalf("effective manager lost descendant access: %s %v", actor, err)
		}
	}
	a, _ := enterprise.ManagementAccess(ctx(leaf), kb)
	if a.CanManageNode(root) || a.CanManageNav(nav) {
		t.Fatal("local authorization escaped subtree")
	}
	if err := edit(leaf, root, leaf); err == nil {
		t.Fatal("local manager changed parent authorization")
	}
	if err := nu.EditManagement(ctx(local), &navV1.NavManagementEditReq{KBID: kb, ID: nav, Grants: grant(local)}); err == nil {
		t.Fatal("folder manager changed space owner")
	}
	if err := edit(local, sub, reader); err == nil {
		t.Fatal("ordinary user became document manager")
	}
	if err := edit(local, doc, leaf); err == nil {
		t.Fatal("new document-specific management accepted")
	}
	detail, err := u.GetNodeManagement(ctx(leaf), kb, doc)
	if err != nil || len(detail.InheritedSources) != 3 || detail.NodeType != domain.NodeTypeDocument {
		t.Fatalf("wrong effective provenance: %#v %v", detail, err)
	}
	if err := u.EditNodeManagement(ctx(owner), &v1.NodeManagementEditReq{KBID: kb, IDs: []string{root}, Grants: nil}); err != nil {
		t.Fatal(err)
	}
	a, _ = enterprise.ManagementAccess(ctx(owner), kb)
	if !a.CanManageSubtree(root) {
		t.Fatal("clearing local managers removed owner")
	}
	a, _ = enterprise.ManagementAccess(ctx(local), kb)
	if a.CanManageNode(root) || a.CanManageNode(doc) {
		t.Fatal("removed local grant remained effective")
	}
	a, _ = enterprise.ManagementAccess(ctx(leaf), kb)
	if !a.CanManageNode(doc) {
		t.Fatal("removing another source removed independent local delegation")
	}
	var saved domain.Node
	if err := tx.First(&saved, "id = ?", root).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Permissions.ScopeType != "user" || len(saved.Permissions.AllowedUserIDs) != 1 || saved.Permissions.AllowedUserIDs[0] != reader {
		t.Fatal("management edit changed reading ACL")
	}
}
