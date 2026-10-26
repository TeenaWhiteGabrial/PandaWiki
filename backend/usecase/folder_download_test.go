package usecase

import (
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestDescendantsOfNavIncludesVirtualRootChildren(t *testing.T) {
	nodes := []*domain.NodeListItemResp{
		{ID: "root-doc-empty-parent", NavId: "nav-a", Type: domain.NodeTypeDocument},
		{ID: "root-doc-nav-parent", NavId: "nav-a", ParentID: "nav-a", Type: domain.NodeTypeDocument},
		{ID: "folder", NavId: "nav-a", ParentID: "nav-a", Type: domain.NodeTypeFolder},
		{ID: "nested-doc", NavId: "nav-a", ParentID: "folder", Type: domain.NodeTypeDocument},
		{ID: "other-nav-doc", NavId: "nav-b", Type: domain.NodeTypeDocument},
	}

	got := descendantsOfNav(nodes, "nav-a")
	if len(got) != 3 {
		t.Fatalf("expected 3 documents beneath navigation root, got %d", len(got))
	}
	for _, node := range got {
		if node.ID == "other-nav-doc" {
			t.Fatal("document from another navigation was included")
		}
	}
}
