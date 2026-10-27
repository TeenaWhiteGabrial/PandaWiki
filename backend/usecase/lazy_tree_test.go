package usecase

import (
	"context"
	"slices"
	"testing"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/domain"
)

func TestSelectTreeLevel(t *testing.T) {
	edges := map[string]treeEdge{
		"a": {"a", "nav1", "nav1"}, "b": {"b", "a", "nav1"}, "doc": {"doc", "b", "nav1"}, "sibling": {"sibling", "b", "nav1"}, "other": {"other", "", "nav2"},
	}
	ids, loaded, has := selectTreeLevel(edges, v1.NodeListGroupNavReq{}, nil)
	if len(ids) != 0 || len(loaded) != 0 || !has["nav:nav1"] || !has["b"] || has["doc"] {
		t.Fatalf("root should contain only columns: %v %v %v", ids, loaded, has)
	}
	ids, _, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{NavID: "nav1"}, nil)
	if !slices.Equal(ids, []string{"a"}) {
		t.Fatalf("column returned descendants: %v", ids)
	}
	ids, _, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{NavID: "nav1", ParentID: "a"}, nil)
	if !slices.Equal(ids, []string{"b"}) {
		t.Fatalf("folder returned descendants: %v", ids)
	}
	ids, loaded, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{PathID: "doc"}, nil)
	if !slices.Equal(ids, []string{"a", "b", "doc", "sibling"}) || len(loaded["nav1"]) != 3 {
		t.Fatalf("direct link lost path or siblings: %v %v", ids, loaded)
	}
	ids, _, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{}, map[string]bool{"doc": true, "not-visible": true})
	if !slices.Equal(ids, []string{"a", "b", "doc"}) {
		t.Fatalf("global search leaked unrelated siblings: %v", ids)
	}
	ids, _, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{NavID: "nav2", ParentID: "b"}, nil)
	if len(ids) != 0 {
		t.Fatalf("parent from another column leaked: %v", ids)
	}
	ids, loaded, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{Parents: []string{"nav:nav1", "a", "nav:nav2"}}, nil)
	if !slices.Equal(ids, []string{"a", "b", "other"}) || len(loaded) != 2 {
		t.Fatalf("batch refresh returned wrong levels: %v %v", ids, loaded)
	}
	// Corrupt cycles must terminate and cannot import a different column's path.
	edges["cycle"] = treeEdge{"cycle", "cycle", "nav1"}
	ids, _, _ = selectTreeLevel(edges, v1.NodeListGroupNavReq{PathID: "cycle"}, nil)
	if !slices.Equal(ids, []string{"cycle"}) {
		t.Fatal(ids)
	}
}

// Runs inside the rollback-only PostgreSQL fixture in document_tree_performance_test.go.
func assertLazyTrees(t *testing.T, u *NodeUsecase, kbID, navID, adminID, managerID string, nodes []domain.Node) {
	t.Helper()
	ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: adminID})
	graph, err := u.enterprise.ManagementTreeAccess(ctx, kbID)
	if err != nil || graph.Nodes[nodes[4].ID].Name != "" || graph.Nodes[nodes[4].ID].Meta.FileKey != "" || graph.Nodes[nodes[4].ID].Meta.Summary != "" {
		t.Fatalf("policy graph loaded document metadata: %+v %v", graph, err)
	}
	for _, userID := range []string{adminID, managerID} {
		ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: userID})
		roots, err := u.GetNodeTree(ctx, v1.NodeListGroupNavReq{KbId: kbID})
		if err != nil || len(roots) != 1 || len(roots[0].List) != 0 || !roots[0].HasChildren || roots[0].TreeRevision == "" {
			t.Fatalf("admin root is not lazy: %+v %v", roots, err)
		}
		level, err := u.GetNodeTree(ctx, v1.NodeListGroupNavReq{KbId: kbID, NavID: navID})
		if err != nil || len(level) != 1 {
			t.Fatalf("admin level: %+v %v", level, err)
		}
		want := 1
		if userID == adminID {
			want = 2
		}
		if len(level[0].List) != want {
			t.Fatalf("admin level returned descendants/hidden items: got%d want%d", len(level[0].List), want)
		}
		found := false
		for _, n := range level[0].List {
			if n.ID == nodes[0].ID {
				found = n.HasChildren
			}
		}
		if !found {
			t.Fatal("missing expandable folder")
		}
		search, err := u.GetNodeTree(ctx, v1.NodeListGroupNavReq{KbId: kbID, Search: "document-239"})
		if err != nil || len(search) != 1 || len(search[0].List) != 5 {
			t.Fatalf("admin search must include authorized ancestors: %+v %v", search, err)
		}
		branch, err := u.GetNodeTree(ctx, v1.NodeListGroupNavReq{KbId: kbID, NavID: navID, ParentID: nodes[0].ID})
		if err != nil || len(branch) != 1 || len(branch[0].List) != 1 || branch[0].List[0].ID != nodes[1].ID || branch[0].List[0].Name != nodes[1].Name {
			t.Fatalf("admin branch is not shallow: %+v %v", branch, err)
		}
		statuses, err := u.GetTreeNodeStatuses(ctx, kbID, []string{nodes[4].ID, nodes[len(nodes)-1].ID, "foreign-id"})
		if err != nil || len(statuses) != want {
			t.Fatalf("status scope wrong: %+v %v", statuses, err)
		}
	}
	roots, err := u.GetShareNodeTree(context.Background(), kbID, managerID, v1.NodeListGroupNavReq{})
	if err != nil || len(roots) != 1 || len(roots[0].List) != 0 || roots[0].EntryNodeID == "" {
		t.Fatalf("reader root is not lazy: %+v %v", roots, err)
	}
	branch, err := u.GetShareNodeTree(context.Background(), kbID, managerID, v1.NodeListGroupNavReq{NavID: navID, ParentID: nodes[0].ID})
	if err != nil || len(branch) != 1 || len(branch[0].List) != 1 || branch[0].List[0].ID != nodes[1].ID || branch[0].List[0].Name != nodes[1].Name {
		t.Fatalf("reader branch is not shallow: %+v %v", branch, err)
	}
	search, err := u.GetShareNodeTree(context.Background(), kbID, managerID, v1.NodeListGroupNavReq{Search: "document-239"})
	if err != nil || len(search) != 1 || len(search[0].List) != 5 {
		t.Fatalf("reader search missed unloaded content: %+v %v", search, err)
	}
	path, err := u.GetShareNodeTree(context.Background(), kbID, managerID, v1.NodeListGroupNavReq{PathID: nodes[4].ID})
	if err != nil || len(path) != 1 || len(path[0].LoadedParents) != 5 || len(path[0].List) != len(nodes) {
		t.Fatalf("reader direct link lost path levels: %+v %v", path, err)
	}
	for _, n := range path[0].List {
		if n.Meta.FileKey != "" {
			t.Fatal("direct-link tree exposed storage keys")
		}
	}
	for _, n := range search[0].List {
		if n.Permissions.HasEnterpriseRule() || n.Meta.FileKey != "" {
			t.Fatal("reader tree exposed ACL or storage details")
		}
	}
	immediate, err := u.GetNodeReleaseListByParentID(context.Background(), kbID, nodes[0].ID, managerID)
	if err != nil || len(immediate) != 1 || len(immediate[0].Children) != 0 {
		t.Fatalf("folder detail downloaded whole subtree: %+v %v", immediate, err)
	}
}
