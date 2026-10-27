package usecase

import (
	"context"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

func TestReadingAudienceUsesTheFullPermissionIntersection(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	u := &EnterpriseUsecase{}
	policy := domain.NodePermissions{
		ScopeType: "custom", AllowedDepartmentIDs: []string{"engineering"}, AllowedPermissionLevelIDs: []string{"senior"},
		AllowedUserIDs: []string{"guest", "denied"}, DeniedUserIDs: []string{"denied"},
		Constraints: []domain.NodePermissions{
			{ScopeType: "user", AllowedUserIDs: []string{"a", "b", "guest", "denied", "wrong-level", "admin"}},
			{ScopeType: "custom", AllowAllUsers: true, DeniedDepartmentIDs: []string{"blocked"}},
		},
	}
	users := []domain.UserSummary{
		{ID: "a", Account: "Alpha", Name: "张三", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "senior"},
		{ID: "b", Account: "Beta", Name: "李四", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "senior"},
		{ID: "guest", Account: "Guest", DepartmentIDs: []string{"sales"}, PermissionLevelID: "junior"},
		{ID: "denied", Account: "Denied"},
		{ID: "outside", Account: "Outside", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "senior"},
		{ID: "wrong-level", Account: "Wrong", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "junior"},
		{ID: "admin", Account: "Admin", Role: consts.UserRoleAdmin, DepartmentIDs: []string{"sales"}, PermissionLevelID: "senior"},
		{ID: "blocked", Account: "Blocked", DepartmentIDs: []string{"engineering", "blocked"}, PermissionLevelID: "senior"},
	}
	result := u.filterReadingAudience(policy, users, map[string]int{"senior": 10, "junior": 1}, "", 1, 2, true)
	if result.Total != 4 || result.FilteredTotal != 4 || len(result.Users) != 2 || result.Users[0].ID != "a" || result.Users[1].ID != "b" {
		t.Fatalf("wrong final audience: %#v", result)
	}
	result = u.filterReadingAudience(policy, users, nil, "", 2, 2, true)
	if result.Total != 4 || len(result.Users) != 2 || result.Users[0].ID != "guest" || result.Users[1].ID != "admin" {
		t.Fatalf("pagination lost members: %#v", result)
	}
	result = u.filterReadingAudience(policy, users, nil, "  ALP  ", 1, 20, true)
	if result.Total != 4 || result.FilteredTotal != 1 || len(result.Users) != 1 || result.Users[0].ID != "a" {
		t.Fatalf("search must filter the authorized set while keeping total: %#v", result)
	}
	result = u.filterReadingAudience(policy, users, nil, "李四", 1, 20, false)
	if result.Total != 4 || result.FilteredTotal != 1 || len(result.Users) != 0 {
		t.Fatalf("count-only preview leaked a full list or lost name search: %#v", result)
	}
}

func TestReadingPreviewSanitizesClientRulesAndPreservesHiddenSettings(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "true")
	u := &EnterpriseUsecase{}
	current := domain.NodePermissions{
		AllowedPermissionLevelIDs: []string{"old-level"}, MinPermissionLevelID: "old-level", MinPermissionLevelRank: 5,
		DeniedDepartmentIDs: []string{"blocked"}, DeniedUserIDs: []string{"blocked-user"},
		ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{"manager"}}},
	}
	draft := domain.NodePermissions{ScopeType: "custom", AllowAllUsers: true, Constraints: []domain.NodePermissions{{ScopeType: "user", AllowedUserIDs: []string{"forged"}}}, ParentPermissionSnapshot: []domain.NodePermissions{{AllowAllUsers: true}}, ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{"forged"}}}}
	prepared, err := u.prepareReadingPermissions(context.Background(), current, draft, true)
	if err != nil || len(prepared.Constraints) != 0 || len(prepared.ParentPermissionSnapshot) != 0 || prepared.ManagementGrants[0].SubjectIDs[0] != "manager" || prepared.AllowedPermissionLevelIDs[0] != "old-level" || prepared.DeniedDepartmentIDs[0] != "blocked" || prepared.DeniedUserIDs[0] != "blocked-user" {
		t.Fatalf("preview and save must share sanitization/preservation: %#v %v", prepared, err)
	}
	inherit := true
	prepared, err = u.prepareReadingPermissions(context.Background(), current, domain.NodePermissions{Inherit: &inherit, AllowedUserIDs: []string{"forged"}}, true)
	if err != nil || prepared.Inherit == nil || !*prepared.Inherit || len(prepared.AllowedUserIDs) != 0 || len(prepared.DeniedUserIDs) != 0 || len(prepared.ManagementGrants) != 1 {
		t.Fatalf("inherit reset must match save: %#v %v", prepared, err)
	}
}

func TestReadingBatchPreviewUsesDraftParentsAndSavedSnapshots(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	u := &EnterpriseUsecase{}
	root := &domain.Node{ID: "root", KBID: "kb", NavId: "nav", ParentID: "nav", Permissions: domain.NodePermissions{ScopeType: "department", AllowedDepartmentIDs: []string{"engineering"}}}
	child := &domain.Node{ID: "child", KBID: "kb", NavId: "nav", ParentID: "root"}
	nodes := map[string]*domain.Node{"root": root, "child": child}
	inherit := true
	draft := domain.NodePermissions{Inherit: &inherit}
	policy, err := u.resolveDraftReadingPolicy(context.Background(), nodes, child, draft, map[string]domain.NodePermissions{"root": draft, "child": draft})
	sales := &domain.EnterpriseAccessContext{PrincipalID: "sales", DepartmentIDs: []string{"sales"}}
	if err != nil || !u.CanVisitable(policy, sales) {
		t.Fatalf("batch preview accidentally kept the old parent being replaced: %#v %v", policy, err)
	}
	policy, err = u.resolveDraftReadingPolicy(context.Background(), nodes, child, draft, map[string]domain.NodePermissions{"child": draft})
	if err != nil || u.CanVisitable(policy, sales) {
		t.Fatalf("an unselected parent must remain restrictive: %#v %v", policy, err)
	}
	local := domain.NodePermissions{ScopeType: "custom", AllowAllUsers: true, ParentPermissionSnapshot: []domain.NodePermissions{root.Permissions}}
	policy, err = u.resolveDraftReadingPolicy(context.Background(), nodes, child, local, map[string]domain.NodePermissions{"root": draft})
	if err != nil || u.CanVisitable(policy, sales) {
		t.Fatalf("saved parent snapshot must prevent silent widening: %#v %v", policy, err)
	}
}

func TestUnrestrictedReadingPolicyDoesNotHideAncestorsOrDenyLists(t *testing.T) {
	for _, disabled := range []string{"true", "false"} {
		t.Setenv("DISABLE_PERMISSION_LEVELS", disabled)
		for _, tc := range []struct {
			policy domain.NodePermissions
			want   bool
		}{
			{domain.NodePermissions{}, true},
			{domain.NodePermissions{AllowAllUsers: true, Constraints: []domain.NodePermissions{{ScopeType: "department", AllowedDepartmentIDs: []string{"engineering"}}}}, false},
			{domain.NodePermissions{AllowAllUsers: true, DeniedUserIDs: []string{"denied"}}, false},
			{domain.NodePermissions{ScopeType: "user"}, false},
			{domain.NodePermissions{ScopeType: "department"}, false},
			{domain.NodePermissions{ScopeType: "permission_level"}, config.PermissionLevelsDisabled()},
		} {
			if got := unrestrictedReadingPolicy(tc.policy); got != tc.want {
				t.Fatalf("wrong unrestricted flag for %#v: %v want %v", tc.policy, got, tc.want)
			}
		}
	}
}
