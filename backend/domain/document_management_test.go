package domain

import (
	"context"
	"testing"
)

func managementFixture(scope string) *DocumentManagementAccess {
	return &DocumentManagementAccess{Principal: &EnterpriseAccessContext{PrincipalID: "alice", PermissionLevelID: "editor", DepartmentIDs: []string{"engineering"}}, Nodes: map[string]*Node{
		"root":   {ID: "root", Type: NodeTypeFolder, Permissions: NodePermissions{ManagementGrants: []DocumentManagementGrant{{SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice", "bob"}, DirectoryScope: scope}}}},
		"doc":    {ID: "doc", Type: NodeTypeDocument, ParentID: "root"},
		"sub":    {ID: "sub", Type: NodeTypeFolder, ParentID: "root"},
		"subdoc": {ID: "subdoc", Type: NodeTypeDocument, ParentID: "sub"},
		"other":  {ID: "other", Type: NodeTypeFolder},
	}}
}

func TestDocumentManagementCurrentDirectory(t *testing.T) {
	access := managementFixture(ManagementScopeCurrent)
	for id, want := range map[string]bool{"root": true, "doc": true, "sub": false, "subdoc": false, "other": false, "missing": false} {
		if got := access.CanManageNode(id); got != want {
			t.Errorf("%s: got %v want %v", id, got, want)
		}
	}
	if !access.CanCreate("root", NodeTypeDocument) || access.CanCreate("root", NodeTypeFolder) || access.CanCreate("sub", NodeTypeDocument) {
		t.Fatal("current directory creation boundary broken")
	}
	if access.CanManageSubtree("root") {
		t.Fatal("must not delete or move an unmanaged subdirectory")
	}
}
func TestDocumentManagementDescendantsAndMultipleUsers(t *testing.T) {
	for _, id := range []string{"alice", "bob"} {
		access := managementFixture(ManagementScopeTree)
		access.Principal.PrincipalID = id
		if !access.CanManageNode("subdoc") || !access.CanManageSubtree("root") || !access.CanCreate("root", NodeTypeFolder) {
			t.Fatal("matching user must manage descendants")
		}
		if access.CanManageNode("other") || access.CanCreate("other", NodeTypeDocument) {
			t.Fatal("grant escaped directory boundary")
		}
	}
}
func TestDocumentManagementAudienceUnionAndExactLevel(t *testing.T) {
	access := managementFixture(ManagementScopeTree)
	access.Nodes["root"].Permissions.ManagementGrants = []DocumentManagementGrant{
		{SubjectType: ManagementSubjectLevel, SubjectIDs: []string{"editor"}, DirectoryScope: ManagementScopeTree},
		{SubjectType: ManagementSubjectDepartment, SubjectIDs: []string{"operations"}, DirectoryScope: ManagementScopeTree},
	}
	if !access.CanManageNode("subdoc") {
		t.Fatal("matching grade must grant access")
	}
	access.Principal.PermissionLevelID = "senior-editor"
	access.Principal.PermissionLevelRank = 100
	if access.CanManageNode("subdoc") {
		t.Fatal("a higher rank cannot bypass exact role assignment")
	}
	access.Principal.DepartmentIDs = []string{"operations"}
	if !access.CanManageNode("subdoc") {
		t.Fatal("department grant must work independently of level grant")
	}
	access.Principal = nil
	if access.CanManageNode("doc") {
		t.Fatal("anonymous management must fail")
	}
}
func TestDocumentManagementCyclesAndContextIsolation(t *testing.T) {
	access := managementFixture(ManagementScopeCurrent)
	access.Nodes["other"].ParentID = "other"
	if access.CanManageNode("other") {
		t.Fatal("cycles must not authorize access")
	}
	ctx := WithDocumentManagementAccess(context.Background(), "kb", "alice", access)
	if DocumentManagementAccessFromContext(ctx, "kb", "alice") != access || DocumentManagementAccessFromContext(ctx, "other-kb", "alice") != nil || DocumentManagementAccessFromContext(ctx, "kb", "bob") != nil {
		t.Fatal("cached authorization must be isolated by knowledge base and principal")
	}
}

func TestNavigationManagementScopeAndRootParentForms(t *testing.T) {
	for _, scope := range []string{ManagementScopeCurrent, ManagementScopeTree} {
		t.Run(scope, func(t *testing.T) {
			access := &DocumentManagementAccess{
				Principal: &EnterpriseAccessContext{PrincipalID: "alice"},
				Navs: map[string]*Nav{
					"nav":   {ID: "nav", Permissions: NodePermissions{ManagementGrants: []DocumentManagementGrant{{SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice", "bob"}, DirectoryScope: scope}}}},
					"other": {ID: "other"},
				},
				Nodes: map[string]*Node{
					"direct":  {ID: "direct", NavId: "nav", Type: NodeTypeDocument},
					"alias":   {ID: "alias", NavId: "nav", ParentID: "nav", Type: NodeTypeDocument},
					"folder":  {ID: "folder", NavId: "nav", Type: NodeTypeFolder},
					"nested":  {ID: "nested", NavId: "nav", ParentID: "folder", Type: NodeTypeDocument},
					"outside": {ID: "outside", NavId: "other", Type: NodeTypeDocument},
					"orphan":  {ID: "orphan", NavId: "nav", ParentID: "missing", Type: NodeTypeDocument},
				},
			}
			if !access.CanManageNav("nav") || !access.HasAnyGrant() || access.CanManageNav("other") {
				t.Fatal("navigation grant boundary broken")
			}
			for _, id := range []string{"direct", "alias"} {
				if !access.CanManageNode(id) {
					t.Fatalf("root parent convention rejected: %s", id)
				}
			}
			tree := scope == ManagementScopeTree
			if access.CanManageNode("folder") != tree || access.CanManageNode("nested") != tree || access.CanManageNode("outside") || access.CanManageNode("orphan") {
				t.Fatal("navigation grant escaped its scope")
			}
			for _, parent := range []string{"", "nav"} {
				if !access.CanCreateInNav(parent, "nav", NodeTypeDocument) || access.CanCreateInNav(parent, "nav", NodeTypeFolder) != tree {
					t.Fatal("root creation scope broken")
				}
			}
			if access.CanCreateInNav("folder", "other", NodeTypeDocument) || access.CanCreateInNav("", "other", NodeTypeDocument) {
				t.Fatal("creation crossed navigation boundary")
			}
			delete(access.Nodes, "orphan")
			if access.CanManageNavSubtree("nav") != tree {
				t.Fatal("navigation deletion must check descendants")
			}
			access.Nodes = nil
			if !access.HasAnyGrant() {
				t.Fatal("an empty authorized navigation must grant access")
			}
		})
	}
}
