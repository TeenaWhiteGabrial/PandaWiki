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
