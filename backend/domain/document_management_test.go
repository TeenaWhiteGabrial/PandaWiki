package domain

import (
	"context"
	"reflect"
	"testing"
)

func TestManagementEvaluationMatchesUncachedAndExpiresWithOperation(t *testing.T) {
	for _, scope := range []string{ManagementScopeTree, ManagementScopeCurrent} {
		raw := managementFixture(scope)
		raw.Nodes["sub"].Permissions.ManagementGrants = []DocumentManagementGrant{{SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice"}, DirectoryScope: ManagementScopeTree}}
		raw.Nodes["cycle-a"] = &Node{ID: "cycle-a", ParentID: "cycle-b", Type: NodeTypeFolder, Permissions: raw.Nodes["sub"].Permissions}
		raw.Nodes["cycle-b"] = &Node{ID: "cycle-b", ParentID: "cycle-a", Type: NodeTypeFolder, Permissions: raw.Nodes["sub"].Permissions}
		evaluated := raw.ForEvaluation()
		ids := []string{"subdoc", "sub", "root", "doc", "other", "missing", "cycle-a", "cycle-b"}
		for pass := 0; pass < 2; pass++ {
			for _, id := range ids {
				if got, want := evaluated.CanManageNode(id), raw.CanManageNode(id); got != want {
					t.Fatalf("%s: %s cached=%v uncached=%v", scope, id, got, want)
				}
				if !reflect.DeepEqual(evaluated.ManagementGrantConstraintsForNode(id), raw.ManagementGrantConstraintsForNode(id)) || !reflect.DeepEqual(evaluated.ManagementGrantsForNode(id), raw.ManagementGrantsForNode(id)) {
					t.Fatalf("%s: hierarchy summary changed for %s", scope, id)
				}
				for _, kind := range []NodeType{NodeTypeFolder, NodeTypeDocument} {
					if evaluated.CanCreate(id, kind) != raw.CanCreate(id, kind) {
						t.Fatalf("%s: creation scope changed for %s", scope, id)
					}
				}
			}
		}
		if len(evaluated.evaluation.constraints) == 0 || raw.evaluation != nil {
			t.Fatal("cache must be used only by the operation-local evaluator")
		}
		// A later operation must not retain an earlier permission decision.
		raw.Nodes["root"].Permissions.ManagementGrants = nil
		if raw.CanManageNode("root") || raw.ForEvaluation().CanManageNode("root") {
			t.Fatal("revoked grant survived into a new evaluation")
		}
	}
}

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

func TestManagementGrantsForNodeIncludeApplicableAncestors(t *testing.T) {
	access := managementFixture(ManagementScopeTree)
	access.Nodes["root"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType:    ManagementSubjectDepartment,
		SubjectIDs:     []string{"industrial-security"},
		DirectoryScope: ManagementScopeTree,
	}}
	grants := access.ManagementGrantsForNode("subdoc")
	if len(grants) != 1 || grants[0].SubjectType != ManagementSubjectDepartment || grants[0].SubjectIDs[0] != "industrial-security" {
		t.Fatalf("expected applicable ancestor grant, got %#v", grants)
	}

	access.Nodes["root"].Permissions.ManagementGrants[0].DirectoryScope = ManagementScopeCurrent
	grants = access.ManagementGrantsForNode("doc")
	if len(grants) != 1 {
		t.Fatalf("current-directory grant should apply to direct documents, got %#v", grants)
	}
	grants = access.ManagementGrantsForNode("subdoc")
	if len(grants) != 0 {
		t.Fatalf("current-directory grant must not apply to nested documents, got %#v", grants)
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

func TestDocumentManagementHierarchyOnlyNarrowsParentAudience(t *testing.T) {
	access := managementFixture(ManagementScopeTree)
	access.Nodes["root"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType: ManagementSubjectDepartment, SubjectIDs: []string{"engineering"}, DirectoryScope: ManagementScopeTree,
	}}
	access.Nodes["sub"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice"}, DirectoryScope: ManagementScopeTree,
	}}
	parentConstraints := access.ManagementGrantConstraintsForParentNode("sub")
	if len(parentConstraints) != 1 || parentConstraints[0][0].SubjectIDs[0] != "engineering" {
		t.Fatalf("nested folder delegation should use the parent folder's manager audience, got %#v", parentConstraints)
	}
	access.Principal = &EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"engineering"}}
	if !access.CanManageNode("subdoc") {
		t.Fatal("a manager matching both parent and child rules should manage the descendant")
	}
	access.Principal = &EnterpriseAccessContext{PrincipalID: "bob", DepartmentIDs: []string{"engineering"}}
	if access.CanManageNode("subdoc") {
		t.Fatal("matching only the parent rule must not bypass the child restriction")
	}
	access.Principal = &EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"sales"}}
	if access.CanManageNode("subdoc") {
		t.Fatal("a child grant must not add a manager outside the parent audience")
	}
	constraints := access.ManagementGrantConstraintsForNode("subdoc")
	if len(constraints) != 2 || len(constraints[0]) != 1 || len(constraints[1]) != 1 {
		t.Fatalf("expected separate parent and child management constraints, got %#v", constraints)
	}
}

func TestChildFolderManagementRequiresManagingEveryParentFolder(t *testing.T) {
	access := managementFixture(ManagementScopeTree)
	access.Nodes["root"].Permissions.ManagementGrants = nil
	access.Nodes["sub"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice"}, DirectoryScope: ManagementScopeTree,
	}}
	if access.CanManageNode("sub") {
		t.Fatal("a child folder grant must not bypass an unmanaged parent folder")
	}
	if access.CanCreate("root", NodeTypeFolder) {
		t.Fatal("a manager who cannot manage the parent folder must not create a child folder")
	}

	access.Nodes["root"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType: ManagementSubjectUser, SubjectIDs: []string{"bob"}, DirectoryScope: ManagementScopeTree,
	}}
	if constraints := access.ManagementGrantConstraintsForNode("sub"); len(constraints) != 2 {
		t.Fatalf("expected parent and child grant constraints without duplicates, got %#v", constraints)
	}
	if access.CanManageNode("sub") {
		t.Fatal("a child folder grant must not bypass a parent grant that excludes the principal")
	}

	access.Nodes["root"].Permissions.ManagementGrants = []DocumentManagementGrant{{
		SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice"}, DirectoryScope: ManagementScopeCurrent,
	}}
	if !access.CanManageNode("sub") {
		t.Fatal("a principal who can manage the parent and is explicitly granted the child must manage it")
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
			if got := access.ManagementGrantConstraintsForParentNode("folder"); len(got) != 0 {
				t.Fatalf("top-level folder delegation should not be restricted by the navigation audience: %#v", got)
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

func TestTopLevelFolderGrantDefinesItsManagementAudience(t *testing.T) {
	access := &DocumentManagementAccess{
		Principal: &EnterpriseAccessContext{PrincipalID: "bob"},
		Navs: map[string]*Nav{
			"nav": {ID: "nav", Permissions: NodePermissions{ManagementGrants: []DocumentManagementGrant{{
				SubjectType: ManagementSubjectUser, SubjectIDs: []string{"alice"}, DirectoryScope: ManagementScopeTree,
			}}}},
		},
		Nodes: map[string]*Node{
			"root": {ID: "root", NavId: "nav", Type: NodeTypeFolder, Permissions: NodePermissions{ManagementGrants: []DocumentManagementGrant{{
				SubjectType: ManagementSubjectUser, SubjectIDs: []string{"bob"}, DirectoryScope: ManagementScopeTree,
			}}}},
			"child": {ID: "child", NavId: "nav", ParentID: "root", Type: NodeTypeFolder},
		},
	}
	if !access.CanManageNode("root") || !access.CanManageNode("child") {
		t.Fatal("the top-level folder grant should define its manager audience and inherit to children")
	}
	if access.CanManageNav("nav") {
		t.Fatal("a folder grant must not grant management of the navigation itself")
	}
	if constraints := access.ManagementGrantConstraintsForParentNode("child"); len(constraints) != 1 || constraints[0][0].SubjectIDs[0] != "bob" {
		t.Fatalf("nested delegation should inherit the top-level folder audience, got %#v", constraints)
	}
}
