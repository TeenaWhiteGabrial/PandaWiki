package usecase

import (
	"context"
	"testing"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

func TestAttachDepartmentAncestorIDs(t *testing.T) {
	departments := []domain.Department{{ID: "child", Name: "子部门", Path: "root/child"}}
	ancestors := map[string][]string{"child": {"root"}}

	got := attachDepartmentAncestorIDs(departments, ancestors)
	if len(got) != 1 || len(got[0].AncestorIDs) != 1 || got[0].AncestorIDs[0] != "root" {
		t.Fatalf("ancestor context was not attached: %#v", got)
	}
	if len(departments[0].AncestorIDs) != 0 {
		t.Fatalf("helper must not mutate the source department slice: %#v", departments[0])
	}
}

func TestClearSensitiveNodeFieldsForNonManagers(t *testing.T) {
	managed := &domain.NodeListItemResp{
		CanManage:    true,
		FileKey:      "kb/file.pdf",
		FileName:     "file.pdf",
		FileReadonly: true,
		Permissions:  domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"admin"}}}},
	}
	readable := &domain.NodeListItemResp{
		CanManage:                 false,
		FileKey:                   "kb/private.pdf",
		FileName:                  "private.pdf",
		FileReadonly:              true,
		Permissions:               domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"other-admin"}}}},
		EffectiveManagementGrants: []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectDepartment, SubjectIDs: []string{"engineering"}}},
	}

	clearSensitiveNodeFieldsForNonManagers([]*domain.NodeListItemResp{managed, readable})

	if managed.FileKey != "kb/file.pdf" || !managed.FileReadonly {
		t.Fatalf("file metadata for managers should remain available: %#v", managed)
	}
	if readable.FileKey != "" || readable.FileName != "" || readable.FileReadonly {
		t.Fatalf("file metadata should be hidden from non-managers: %#v", readable)
	}
	if len(managed.Permissions.ManagementGrants) != 1 || len(readable.Permissions.ManagementGrants) != 0 {
		t.Fatalf("direct management grants should remain hidden from non-managers: managed=%#v readable=%#v", managed.Permissions.ManagementGrants, readable.Permissions.ManagementGrants)
	}
	if len(readable.EffectiveManagementGrants) != 0 || len(readable.EffectiveManagementConstraints) != 0 {
		t.Fatalf("management grants should be hidden from non-managed path nodes: %#v", readable)
	}
}

func TestAdminDetailRequiresManagementWhileAncestorPermissionMetadataIsAvailable(t *testing.T) {
	access := &domain.DocumentManagementAccess{
		Principal: &domain.EnterpriseAccessContext{PrincipalID: "admin"},
		Nodes: map[string]*domain.Node{
			"root": {
				ID:    "root",
				Type:  domain.NodeTypeFolder,
				NavId: "nav",
				Permissions: domain.NodePermissions{
					ScopeType:      "user",
					AllowedUserIDs: []string{"admin"},
				},
			},
			"child": {
				ID:       "child",
				Type:     domain.NodeTypeDocument,
				NavId:    "nav",
				ParentID: "root",
				Permissions: domain.NodePermissions{
					ManagementGrants: []domain.DocumentManagementGrant{{
						SubjectType:    domain.ManagementSubjectUser,
						SubjectIDs:     []string{"admin"},
						DirectoryScope: domain.ManagementScopeCurrent,
					}},
				},
			},
		},
	}
	ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: "admin"})
	ctx = domain.WithDocumentManagementAccess(ctx, "kb", "admin", access)
	u := &NodeUsecase{enterprise: &EnterpriseUsecase{}}

	if allowed, err := u.canManageNodeInAdmin(ctx, "kb", "root"); err != nil || allowed {
		t.Fatalf("reader ACL alone must not authorize admin detail: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := u.canManageNodeInAdmin(ctx, "kb", "child"); err != nil || !allowed {
		t.Fatalf("explicit management grant should authorize the managed node: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := u.canInspectNodePermissionInAdmin(ctx, "kb", "root"); err != nil || !allowed {
		t.Fatalf("a manager may inspect ancestor ACL metadata needed to configure a managed child: allowed=%v err=%v", allowed, err)
	}
}

func TestManagementTreeIncludesOnlyManagedNodesAndPathFolders(t *testing.T) {
	access := &domain.DocumentManagementAccess{
		Principal: &domain.EnterpriseAccessContext{PrincipalID: "admin"},
		Nodes: map[string]*domain.Node{
			"root":    {ID: "root", Type: domain.NodeTypeFolder, NavId: "nav"},
			"target":  {ID: "target", Type: domain.NodeTypeFolder, NavId: "nav", ParentID: "root"},
			"managed": {ID: "managed", Type: domain.NodeTypeDocument, NavId: "nav", ParentID: "target", Permissions: domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"admin"}, DirectoryScope: domain.ManagementScopeCurrent}}}},
			"sibling": {ID: "sibling", Type: domain.NodeTypeDocument, NavId: "nav", ParentID: "root", Permissions: domain.NodePermissions{ScopeType: "public"}},
		},
	}
	visible := managementVisibleNodeIDs(access)
	if !visible["managed"] || !visible["target"] || !visible["root"] {
		t.Fatalf("management grants should include the node and path folders: %#v", visible)
	}
	if visible["sibling"] {
		t.Fatalf("reader visibility must not add an unmanaged sibling to the admin tree: %#v", visible)
	}
}

func TestManagementGrantChangesOnlyPreserveExistingLegacyGroups(t *testing.T) {
	oldDepartmentGrant := domain.DocumentManagementGrant{
		SubjectType: domain.ManagementSubjectDepartment,
		SubjectIDs:  []string{"engineering", "platform"},
	}
	unchanged := oldDepartmentGrant
	unchanged.DirectoryScope = domain.ManagementScopeTree
	userGrant := domain.DocumentManagementGrant{
		SubjectType:    domain.ManagementSubjectUser,
		SubjectIDs:     []string{"admin"},
		DirectoryScope: domain.ManagementScopeTree,
	}

	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{oldDepartmentGrant}}, []domain.DocumentManagementGrant{unchanged, userGrant}); err != nil {
		t.Fatalf("an unchanged historical group and a new named admin should be allowed: %v", err)
	}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{oldDepartmentGrant}}, []domain.DocumentManagementGrant{userGrant}); err != nil {
		t.Fatalf("removing a historical group should be allowed: %v", err)
	}

	addedDepartment := oldDepartmentGrant
	addedDepartment.SubjectIDs = []string{"engineering", "platform", "finance"}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{oldDepartmentGrant}}, []domain.DocumentManagementGrant{addedDepartment}); err == nil {
		t.Fatal("widening a historical group grant should be rejected")
	}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{nil}, []domain.DocumentManagementGrant{oldDepartmentGrant}); err == nil {
		t.Fatal("new department grants should be rejected")
	}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{oldDepartmentGrant}, nil}, []domain.DocumentManagementGrant{unchanged}); err == nil {
		t.Fatal("a batch edit must not introduce a historical group grant onto a node that did not have it")
	}
}

func TestManagementAdministratorCandidatesAreNotDepartmentScoped(t *testing.T) {
	users := []domain.UserSummary{
		{ID: "admin-a", Role: consts.UserRoleAdmin, DepartmentIDs: []string{"engineering"}},
		{ID: "manager-b", Role: consts.UserRoleManager, DepartmentIDs: []string{"finance"}},
		{ID: "ordinary-user", Role: consts.UserRoleUser, DepartmentIDs: []string{"engineering"}},
	}
	admins := managementAdministratorCandidates(users)
	if len(admins) != 2 || admins[0].ID != "admin-a" || admins[1].ID != "manager-b" {
		t.Fatalf("all admins should be candidates regardless of department, ordinary users excluded: %#v", admins)
	}
}

func TestManagementGrantScopeCurrentIsLegacyOnly(t *testing.T) {
	current := domain.DocumentManagementGrant{
		SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"admin"}, DirectoryScope: domain.ManagementScopeCurrent,
	}
	tree := current
	tree.DirectoryScope = domain.ManagementScopeTree
	if err := validateManagementGrantChanges(nil, []domain.DocumentManagementGrant{current}); err == nil {
		t.Fatal("new management rules must not use current-only scope")
	}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{current}}, []domain.DocumentManagementGrant{current}); err != nil {
		t.Fatalf("an unchanged legacy current-only rule should remain valid: %v", err)
	}
	if err := validateManagementGrantChanges([][]domain.DocumentManagementGrant{{current}}, []domain.DocumentManagementGrant{tree}); err != nil {
		t.Fatalf("an explicit change to current-and-descendants should be allowed for a named admin: %v", err)
	}
}

func TestNestedManagementDelegationUsesParentAudienceNotDepartmentTree(t *testing.T) {
	users := []domain.UserSummary{
		{ID: "admin-a", Role: consts.UserRoleManager, DepartmentIDs: []string{"engineering"}},
		{ID: "admin-b", Role: consts.UserRoleAdmin, DepartmentIDs: []string{"engineering"}},
		{ID: "admin-c", Role: consts.UserRoleManager, DepartmentIDs: []string{"finance"}},
		{ID: "ordinary", Role: consts.UserRoleUser, DepartmentIDs: []string{"engineering"}},
	}
	ancestors := map[string][]string{
		"engineering": {"engineering"},
		"finance":     {"finance"},
	}
	parentScope := [][]domain.DocumentManagementGrant{{{
		SubjectType: domain.ManagementSubjectDepartment, SubjectIDs: []string{"engineering"}, DirectoryScope: domain.ManagementScopeTree,
	}}}
	grantB := []domain.DocumentManagementGrant{{
		SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"admin-b"}, DirectoryScope: domain.ManagementScopeTree,
	}}
	if !managementGrantAudienceWithinParents(users, grantB, parentScope, ancestors) {
		t.Fatal("an administrator matching the parent department scope should be delegable")
	}
	grantC := []domain.DocumentManagementGrant{{
		SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{"admin-c"}, DirectoryScope: domain.ManagementScopeTree,
	}}
	if managementGrantAudienceWithinParents(users, grantC, parentScope, ancestors) {
		t.Fatal("an administrator outside the parent management audience must not be delegable")
	}
	if !managementGrantAudienceWithinParents(users, grantC, nil, ancestors) {
		t.Fatal("a top-level directory has no parent audience restriction")
	}
}

func TestScopedManagerCannotWidenInheritedOrRestrictedAccess(t *testing.T) {
	public := domain.NodePermissions{ScopeType: "public", Inherit: boolPointerManagement(false)}
	private := domain.NodePermissions{ScopeType: "department", AllowedDepartmentIDs: []string{"engineering"}, Inherit: boolPointerManagement(false)}
	inherited := domain.NodePermissions{}

	if isDirectlyUnrestrictedReadAudience(private, public, "parent", false) {
		t.Fatal("a restricted node cannot be changed to public")
	}
	if isDirectlyUnrestrictedReadAudience(inherited, public, "parent", true) {
		t.Fatal("an inherited child cannot be made explicitly public")
	}
	if !isDirectlyUnrestrictedReadAudience(public, public, "parent", false) {
		t.Fatal("preserving an existing explicit public rule should remain possible")
	}

	withDenial := domain.NodePermissions{ScopeType: "public", DeniedUserIDs: []string{"external"}, Inherit: boolPointerManagement(false)}
	if isDirectlyUnrestrictedReadAudience(withDenial, public, "parent", false) {
		t.Fatal("removing an existing denial must not be treated as an unchanged public rule")
	}
}

func boolPointerManagement(value bool) *bool { return &value }
