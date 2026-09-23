package usecase

import (
	"github.com/chaitin/panda-wiki/domain"
	"testing"
)

func TestLevelSwitchPreservesDepartmentAndPersonalPermissions(t *testing.T) {
	u := &EnterpriseUsecase{}
	for _, mode := range []string{"true", "false", "true"} {
		t.Setenv("DISABLE_PERMISSION_LEVELS", mode)
		perm := domain.NodePermissions{ScopeType: domain.NodePermissionScopeDepartment, AllowedDepartmentIDs: []string{"engineering"}, MinPermissionLevelRank: 5}
		member := &domain.EnterpriseAccessContext{PrincipalID: "member", DepartmentIDs: []string{"engineering"}, PermissionLevelRank: 1}
		outsider := &domain.EnterpriseAccessContext{PrincipalID: "outsider", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 10}
		for _, check := range []func(domain.NodePermissions, *domain.EnterpriseAccessContext) bool{u.CanVisible, u.CanVisitable, u.CanAnswerable} {
			if check(perm, member) != (mode == "true") {
				t.Fatal("level switch not respected")
			}
			if check(perm, outsider) || check(perm, nil) {
				t.Fatal("department boundary bypassed")
			}
			denied := perm
			denied.DeniedUserIDs = []string{"member"}
			if check(denied, member) {
				t.Fatal("personal deny bypassed")
			}
			denied = perm
			denied.DeniedDepartmentIDs = []string{"engineering"}
			if check(denied, member) {
				t.Fatal("department deny bypassed")
			}
		}
		if perm.MinPermissionLevelRank != 5 {
			t.Fatal("stored level mutated")
		}
	}
}

func TestCustomPermissionsSupportLevelOnlyAndAudienceCombinations(t *testing.T) {
	u := &EnterpriseUsecase{}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")

	levelOnly := domain.NodePermissions{
		ScopeType:              domain.NodePermissionScopeCustom,
		MinPermissionLevelRank: 5,
	}
	if !u.CanVisitable(levelOnly, &domain.EnterpriseAccessContext{PrincipalID: "manager", PermissionLevelRank: 5}) {
		t.Fatal("level-only permission should allow users at the minimum rank")
	}
	if u.CanVisitable(levelOnly, &domain.EnterpriseAccessContext{PrincipalID: "staff", PermissionLevelRank: 4}) {
		t.Fatal("level-only permission should reject users below the minimum rank")
	}

	customAudience := domain.NodePermissions{
		ScopeType:              domain.NodePermissionScopeCustom,
		MinPermissionLevelRank: 3,
		AllowedDepartmentIDs:   []string{"engineering"},
		AllowedUserIDs:         []string{"owner"},
	}
	if !u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "engineer", DepartmentIDs: []string{"engineering"}, PermissionLevelRank: 3}) {
		t.Fatal("custom permission should allow matching department")
	}
	if !u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "owner", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 3}) {
		t.Fatal("custom permission should allow explicitly selected user")
	}
	if u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "owner", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 2}) {
		t.Fatal("explicitly selected user should still satisfy minimum level")
	}
	if u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "outsider", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 10}) {
		t.Fatal("custom permission should reject users outside selected departments and users")
	}

	denyOnly := domain.NodePermissions{
		DeniedUserIDs: []string{"blocked"},
	}
	if !denyOnly.HasEnterpriseRule() {
		t.Fatal("deny-only permission should be treated as an enterprise rule")
	}
	if u.CanVisitable(denyOnly, &domain.EnterpriseAccessContext{PrincipalID: "blocked"}) {
		t.Fatal("deny-only permission should reject blocked user")
	}
	if !u.CanVisitable(denyOnly, &domain.EnterpriseAccessContext{PrincipalID: "allowed"}) {
		t.Fatal("deny-only permission should allow users outside the deny list")
	}
}

func TestPermissionInheritanceDefaultsAndLegacyOverrides(t *testing.T) {
	if !shouldInheritNodePermissions(domain.NodePermissions{}, "parent") {
		t.Fatal("empty node permission should inherit parent by default")
	}
	if shouldInheritNodePermissions(domain.NodePermissions{}, "") {
		t.Fatal("root node should not inherit without parent")
	}
	legacyOverride := domain.NodePermissions{ScopeType: domain.NodePermissionScopeCustom}
	if shouldInheritNodePermissions(legacyOverride, "parent") {
		t.Fatal("legacy explicit permission without inherit flag should remain an override")
	}
	inherit := true
	if !shouldInheritNodePermissions(domain.NodePermissions{Inherit: &inherit, ScopeType: domain.NodePermissionScopeCustom}, "parent") {
		t.Fatal("explicit inherit=true should inherit even when stale permission fields exist")
	}
	inherit = false
	if shouldInheritNodePermissions(domain.NodePermissions{Inherit: &inherit}, "parent") {
		t.Fatal("explicit inherit=false should override parent")
	}
}
