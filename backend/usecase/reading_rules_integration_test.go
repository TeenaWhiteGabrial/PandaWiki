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

func TestReadingRulesSavedTreeAndDirectAccessPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set isolated PostgreSQL test database")
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
	kb, nav, root, child, doc, hidden := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	super, manager, reader, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	grant := []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{manager}, DirectoryScope: domain.ManagementScopeTree}}
	fixtures := []any{
		&domain.User{ID: super, Account: super, Password: "unused", Role: consts.UserRoleAdmin},
		&domain.User{ID: manager, Account: manager, Password: "unused", Role: consts.UserRoleManager},
		&domain.User{ID: reader, Account: reader, Password: "unused", Role: consts.UserRoleUser},
		&domain.User{ID: outsider, Account: outsider, Password: "unused", Role: consts.UserRoleUser},
		&domain.KnowledgeBase{ID: kb, Name: "isolated reading rules"},
		&domain.Nav{ID: nav, KbID: kb, Name: "restricted nav", Permissions: domain.NodePermissions{ReadingVersion: 2}},
		&domain.Node{ID: root, KBID: kb, NavId: nav, ParentID: nav, Type: domain.NodeTypeFolder, Name: "path root", Permissions: domain.NodePermissions{}},
		&domain.Node{ID: child, KBID: kb, NavId: nav, ParentID: root, Type: domain.NodeTypeFolder, Name: "independent child", Permissions: domain.NodePermissions{ManagementGrants: grant}},
		&domain.Node{ID: doc, KBID: kb, NavId: nav, ParentID: child, Type: domain.NodeTypeDocument, Name: "readable file"},
		&domain.Node{ID: hidden, KBID: kb, NavId: nav, ParentID: root, Type: domain.NodeTypeDocument, Name: "hidden sibling", Meta: domain.NodeMeta{FileKey: "do-not-expose"}},
	}
	for _, f := range fixtures {
		if err := tx.Create(f).Error; err != nil {
			t.Fatal(err)
		}
	}
	stored := &store.DB{DB: tx}
	l := log.NewLogger(&config.Config{})
	nodes, navs := pg.NewNodeRepository(stored, l), pg.NewNavRepository(stored, l)
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(stored, l), nodeRepo: nodes, navRepo: navs}
	u := &NodeUsecase{nodeRepo: nodes, navRepo: navs, enterprise: enterprise}
	assistant := &AssistantUsecase{chat: &ChatUsecase{nodeRepo: nodes, enterpriseUsecase: enterprise}}
	ctx := func(id string) context.Context {
		return context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: id})
	}
	save := func(actor, id string, p domain.NodePermissions) error {
		return u.NodePermissionsEdit(ctx(actor), v1.NodePermissionEditReq{KbId: kb, IDs: []string{id}, Permissions: &p})
	}
	p := readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{reader}}, true)
	// A scoped manager may grant an outsider reading access to the managed child,
	// even though the parent and the nav are not readable to either of them.
	if err := save(manager, child, p); err != nil {
		t.Fatal(err)
	}
	current, err := nodes.GetNodeByID(ctx(super), child)
	if err != nil || len(current.Permissions.ManagementGrants) != 1 || current.Permissions.ReadingVersion != 2 || len(current.Permissions.ParentPermissionSnapshot) != 0 {
		t.Fatalf("save altered management or captured old pyramid: %#v %v", current, err)
	}
	if err := save(reader, child, p); err == nil {
		t.Fatal("ordinary reader changed ACL")
	}
	if err := save(manager, root, p); err == nil {
		t.Fatal("child manager edited parent")
	}
	invalid := readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{uuid.NewString()}}, true)
	if err := save(manager, child, invalid); err == nil {
		t.Fatal("unknown user accepted")
	}
	release := uuid.NewString()
	for _, f := range []any{&domain.KBRelease{ID: release, KBID: kb}, &domain.NavRelease{ID: uuid.NewString(), NavID: nav, KbID: kb, ReleaseID: release, Name: "restricted nav"}} {
		if err := tx.Create(f).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{root, child, doc, hidden} {
		n, err := nodes.GetNodeByID(ctx(super), id)
		if err != nil {
			t.Fatal(err)
		}
		version := uuid.NewString()
		for _, f := range []any{&domain.NodeRelease{ID: version, NodeID: id, KBID: kb, ParentID: n.ParentID, Name: n.Name, Type: n.Type}, &domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: kb, NodeID: id, NodeReleaseID: version, ReleaseID: release, NavID: nav}} {
			if err := tx.Create(f).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		user, id string
		allowed  bool
	}{{reader, doc, true}, {outsider, doc, false}, {manager, doc, false}, {reader, hidden, false}, {reader, root, false}, {super, hidden, true}} {
		if got := u.ValidateNodePerm(ctx(tc.user), kb, tc.id, tc.user); (got == nil) != tc.allowed {
			t.Fatalf("direct/preview/download guard mismatch user=%s node=%s error=%v", tc.user, tc.id, got)
		}
		if _, err := assistant.authorizedNode(ctx(tc.user), kb, tc.user, tc.id); (err == nil) != tc.allowed {
			t.Fatalf("assistant source guard differs: %v", err)
		}
	}
	groups, err := u.GetShareNodeList(ctx(reader), kb, reader)
	if err != nil || len(groups) != 1 || len(groups[0].List) != 3 {
		t.Fatalf("full tree dropped independent child or leaked sibling: %#v %v", groups, err)
	}
	for _, n := range groups[0].List {
		if n.ID == root {
			if !n.PathOnly || n.Meta.FileKey != "" || n.Permissions.HasEnterpriseRule() {
				t.Fatal("path-only node disclosed parent content or ACL")
			}
		} else if n.PathOnly {
			t.Fatal("authorized child marked path-only")
		}
	}
	lazy, err := u.GetShareNodeTree(ctx(reader), kb, reader, v1.NodeListGroupNavReq{NavID: nav})
	if err != nil || len(lazy) != 1 || len(lazy[0].List) != 1 || lazy[0].List[0].ID != root || !lazy[0].List[0].PathOnly {
		t.Fatalf("lazy root path failed: %#v %v", lazy, err)
	}
	lazy, err = u.GetShareNodeTree(ctx(reader), kb, reader, v1.NodeListGroupNavReq{NavID: nav, ParentID: root})
	if err != nil || len(lazy) != 1 || len(lazy[0].List) != 1 || lazy[0].List[0].ID != child {
		t.Fatalf("lazy child leaked sibling or disappeared: %#v %v", lazy, err)
	}
	readerNavs := &NavUsecase{navRepo: navs, nodeRepo: nodes, enterprise: enterprise}
	visibleNavs, err := readerNavs.GetShareReleaseList(ctx(reader), kb, reader)
	if err != nil || len(visibleNavs) != 1 || visibleNavs[0].Permissions.HasEnterpriseRule() {
		t.Fatalf("navigation path disappeared or disclosed ACL: %#v %v", visibleNavs, err)
	}
	visibleNavs, err = readerNavs.GetShareReleaseList(ctx(outsider), kb, outsider)
	if err != nil || len(visibleNavs) != 0 {
		t.Fatal("unauthorized navigation root was exposed")
	}

	result, err := u.ReadingAudience(ctx(manager), &v1.ReadingAudienceReq{KBID: kb, IDs: []string{child}, TargetType: "node", IncludeUsers: true})
	if err != nil || result.Total != 2 {
		t.Fatalf("preview differs from actual reader/super access: %#v %v", result, err)
	}
	// Editing the nav must not revoke the independent child.
	nu := &NavUsecase{navRepo: navs, nodeRepo: nodes, enterprise: enterprise}
	navPolicy := readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{outsider}}, true)
	if err := nu.UpdatePermission(ctx(super), &navV1.NavPermissionEditReq{KbId: kb, ID: nav, Permissions: &navPolicy}); err != nil {
		t.Fatal(err)
	}
	if u.ValidateNodePerm(ctx(reader), kb, doc, reader) != nil {
		t.Fatal("nav edit revoked independent child")
	}
	// Current-only grants cover direct files but not a grandchild folder.
	p.ReadingRules[0].IncludeDescendants = false
	if err := save(manager, child, p); err != nil {
		t.Fatal(err)
	}
	if u.ValidateNodePerm(ctx(reader), kb, doc, reader) != nil {
		t.Fatal("current-only rule lost direct document")
	}
	// Restoring inheritance clears the reader and follows the changed nav.
	inherit := true
	if err := save(manager, child, domain.NodePermissions{ReadingVersion: 2, Inherit: &inherit}); err != nil {
		t.Fatal(err)
	}
	if u.ValidateNodePerm(ctx(reader), kb, doc, reader) == nil || u.ValidateNodePerm(ctx(outsider), kb, doc, outsider) != nil {
		t.Fatal("restoring parent inheritance failed")
	}
}
