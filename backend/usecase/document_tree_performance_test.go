package usecase

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

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

type treeQueryCounter struct {
	logger.Interface
	count int
}

func (l *treeQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	l.count++
}

// Use a rollback-only local fixture: query volume must not grow with every
// document/ancestor/level combination, and scoped managers must stay scoped.
func TestDocumentTreeQueriesPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for PostgreSQL integration")
	}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	counter := &treeQueryCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: counter})
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
	kbID, navID, adminID, managerID, levelID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	grant := []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{managerID}, DirectoryScope: domain.ManagementScopeTree}}
	for _, fixture := range []any{
		&domain.KnowledgeBase{ID: kbID, Name: "tree query fixture"},
		&domain.PermissionLevel{ID: levelID, Code: levelID, Name: "tree fixture", Rank: 10, Status: domain.PermissionLevelStatusActive},
		&domain.User{ID: adminID, Account: "tree_admin_" + adminID, Password: "unused", Role: consts.UserRoleAdmin, PermissionLevelID: &levelID},
		&domain.User{ID: managerID, Account: "tree_manager_" + managerID, Password: "unused", Role: consts.UserRoleManager, PermissionLevelID: &levelID},
		&domain.Nav{ID: navID, KbID: kbID, Name: "tree", Permissions: domain.NodePermissions{ScopeType: "public"}},
	} {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	var nodes []domain.Node
	parentID := navID
	for depth := 0; depth < 4; depth++ {
		node := domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: navID, ParentID: parentID, Type: domain.NodeTypeFolder, Name: fmt.Sprintf("folder-%d", depth), CreatorId: adminID, EditorId: adminID,
			Permissions: domain.NodePermissions{ScopeType: "permission_level", AllowedPermissionLevelIDs: []string{levelID}, ManagementGrants: grant}}
		nodes = append(nodes, node)
		parentID = node.ID
	}
	for i := 0; i < 240; i++ {
		nodes = append(nodes, domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: navID, ParentID: parentID, Type: domain.NodeTypeDocument, Name: fmt.Sprintf("document-%d", i), CreatorId: adminID, EditorId: adminID,
			Permissions: domain.NodePermissions{ScopeType: "permission_level", AllowedPermissionLevelIDs: []string{levelID}}})
	}
	hiddenID := uuid.NewString()
	nodes = append(nodes, domain.Node{ID: hiddenID, KBID: kbID, NavId: navID, Type: domain.NodeTypeDocument, Name: "unmanaged", CreatorId: adminID, EditorId: adminID})
	nodes[4].Meta = domain.NodeMeta{FileKey: "private/test-object.pdf", Summary: "tree fixture summary"}
	if err := tx.CreateInBatches(nodes, 100).Error; err != nil {
		t.Fatal(err)
	}
	storeDB := &store.DB{DB: tx}
	l := log.NewLogger(&config.Config{})
	nodeRepo, navRepo := pg.NewNodeRepository(storeDB, l), pg.NewNavRepository(storeDB, l)
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(storeDB, l), nodeRepo: nodeRepo, navRepo: navRepo}
	u := &NodeUsecase{nodeRepo: nodeRepo, navRepo: navRepo, enterprise: enterprise}
	for _, userID := range []string{adminID, managerID} {
		ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: userID})
		ctx, metrics := domain.WithTreeLoadMetrics(ctx)
		counter.count = 0
		start := time.Now()
		groups, err := u.GetNodeListGroupByNav(ctx, v1.NodeListGroupNavReq{KbId: kbID})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("global=%v nodes=%d queries=%d duration=%s", userID == adminID, len(nodes), counter.count, time.Since(start))
		if len(groups) != 1 || groups[0].Permissions.ScopeType != "public" {
			t.Fatalf("missing navigation permissions: %+v", groups)
		}
		want := len(nodes)
		if userID == managerID {
			want--
		}
		if len(groups[0].List) != want {
			t.Fatalf("wrong managed audience: got %d, want %d", len(groups[0].List), want)
		}
		if metrics.InputNodes != len(nodes) || metrics.VisibleNodes != want {
			t.Fatalf("incorrect tree metric counts: %+v", metrics)
		}
		for _, phase := range []string{"nodes_query", "navs_query", "released_navs_query", "access_load", "management_permissions", "tree_assembly"} {
			if _, exists := metrics.PhasesMS[phase]; !exists {
				t.Fatalf("missing admin tree timing: %s", phase)
			}
		}
		for _, node := range groups[0].List {
			if !node.CanManage || node.EffectivePermissions == nil {
				t.Fatalf("missing management or read summary for %s", node.ID)
			}
			if node.ID != hiddenID && len(node.EffectivePermissions.AllowedPermissionLevelIDs) != 1 {
				t.Fatalf("lost inherited level restriction for %s", node.ID)
			}
		}
		if counter.count > 15 {
			t.Errorf("tree performed %d queries for %d nodes; expected a bounded number", counter.count, len(nodes))
		}
		// Searching must retain rules on ancestors absent from the result rows.
		groups, err = u.GetNodeListGroupByNav(ctx, v1.NodeListGroupNavReq{KbId: kbID, Search: "document-239"})
		if err != nil || len(groups) != 1 || len(groups[0].List) != 1 || len(groups[0].List[0].EffectivePermissions.AllowedPermissionLevelIDs) != 1 {
			t.Fatalf("search lost ancestor permissions: %+v, %v", groups, err)
		}
	}

	releaseID := uuid.NewString()
	for _, fixture := range []any{
		&domain.KBRelease{ID: releaseID, KBID: kbID},
		&domain.NavRelease{ID: uuid.NewString(), NavID: navID, KbID: kbID, ReleaseID: releaseID, Name: "tree", Permissions: domain.NodePermissions{ScopeType: "public"}},
	} {
		if err := tx.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	var versions []domain.NodeRelease
	var releaseLinks []domain.KBReleaseNodeRelease
	for _, node := range nodes {
		versionID := uuid.NewString()
		versions = append(versions, domain.NodeRelease{ID: versionID, NodeID: node.ID, KBID: kbID, Name: node.Name, Type: node.Type, ParentID: node.ParentID})
		releaseLinks = append(releaseLinks, domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: kbID, NodeID: node.ID, NodeReleaseID: versionID, ReleaseID: releaseID, NavID: navID})
	}
	if err := tx.CreateInBatches(versions, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.CreateInBatches(releaseLinks, 100).Error; err != nil {
		t.Fatal(err)
	}
	assertLazyTrees(t, u, kbID, navID, adminID, managerID, nodes)
	readerNodes := func() []*domain.ShareNodeListItemResp {
		result := make([]*domain.ShareNodeListItemResp, 0, len(nodes))
		for _, node := range nodes {
			result = append(result, &domain.ShareNodeListItemResp{ID: node.ID, NavId: node.NavId, ParentID: node.ParentID, Type: node.Type, Permissions: node.Permissions})
		}
		return result
	}
	counter.count = 0
	start := time.Now()
	readerCtx, metrics := domain.WithTreeLoadMetrics(context.Background())
	readerGroups, err := u.GetShareNodeList(readerCtx, kbID, managerID)
	if err != nil || len(readerGroups) != 1 || len(readerGroups[0].List) != len(nodes) {
		t.Fatalf("reader tree failed: %+v, %v", readerGroups, err)
	}
	t.Logf("reader nodes=%d queries=%d duration=%s", len(nodes), counter.count, time.Since(start))
	if counter.count > 10 {
		t.Errorf("reader tree performed %d queries; expected a bounded number", counter.count)
	}
	for _, phase := range []string{"nodes_query", "navs_query", "access_load", "permission_snapshot", "read_permissions", "tree_assembly"} {
		if _, exists := metrics.PhasesMS[phase]; !exists {
			t.Fatalf("missing reader tree timing: %s", phase)
		}
	}
	// Compare with individual document access, then tighten a draft ancestor.
	// The next tree request must see the change without any cache invalidation.
	leaf := nodes[4]
	individual, err := enterprise.ResolveShareNodePermissions(context.Background(), leaf.ID, leaf.ParentID, leaf.Permissions)
	individual = enterprise.MergeNodeAndNavPermissions(individual, domain.NodePermissions{ScopeType: "public"})
	var treePolicy domain.NodePermissions
	for _, node := range readerGroups[0].List {
		if node.ID == leaf.ID {
			treePolicy = node.Permissions
		}
	}
	if err != nil || !reflect.DeepEqual(individual, treePolicy) {
		t.Fatalf("tree policy differs from document access: %+v vs %+v, %v", treePolicy, individual, err)
	}
	rootPermissions := nodes[0].Permissions
	rootPermissions.DeniedUserIDs = []string{managerID}
	if err := tx.Model(&domain.Node{}).Where("id = ?", nodes[0].ID).Update("permissions", rootPermissions).Error; err != nil {
		t.Fatal(err)
	}
	lazyAfter, err := u.GetShareNodeTree(context.Background(), kbID, managerID, v1.NodeListGroupNavReq{NavID: navID})
	if err != nil || len(lazyAfter) != 1 || len(lazyAfter[0].List) != 1 || lazyAfter[0].List[0].ID != hiddenID || lazyAfter[0].List[0].HasChildren {
		t.Fatalf("lazy tree leaked revoked branch: %+v %v", lazyAfter, err)
	}
	// The ancestor need not be included in the published/search result rows.
	readable, err := u.filterShareNodesWithNavigationPaths(context.Background(), kbID, managerID, readerNodes()[1:])
	if err != nil || len(readable) != 1 || readable[0].ID != hiddenID {
		t.Fatalf("tree ignored current ancestor exclusion: %d nodes, %v", len(readable), err)
	}
	if err := tx.Model(&domain.PermissionLevel{}).Where("id = ?", levelID).Update("status", domain.PermissionLevelStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := u.filterShareNodesWithNavigationPaths(context.Background(), kbID, managerID, readerNodes()); err == nil {
		t.Fatal("disabled level was accepted after a previous tree request")
	}
}
