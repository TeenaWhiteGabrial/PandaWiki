package usecase

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

func TestSuperAdminDocumentFallbackPrecedesEveryReadingConstraint(t *testing.T) {
	u := &EnterpriseUsecase{}
	policy := domain.NodePermissions{
		ScopeType: "custom", AllowedDepartmentIDs: []string{"restricted"},
		AllowedPermissionLevelIDs: []string{"secret"}, MinPermissionLevelRank: 100,
		AllowedUserIDs: []string{"another-user"}, DeniedUserIDs: []string{"root"},
		DeniedDepartmentIDs: []string{"root-department"},
		Constraints: []domain.NodePermissions{
			{ScopeType: "user", AllowedUserIDs: []string{"another-user"}},
			{ScopeType: "custom", AllowAllUsers: true, DeniedUserIDs: []string{"root"}},
		},
	}
	before, _ := json.Marshal(policy)
	for _, disabled := range []string{"false", "true"} {
		t.Setenv("DISABLE_PERMISSION_LEVELS", disabled)
		for name, check := range map[string]func(domain.NodePermissions, *domain.EnterpriseAccessContext) bool{
			"directory": u.CanVisible, "document and download": u.CanVisitable, "question answering": u.CanAnswerable,
		} {
			t.Run(disabled+"/"+name, func(t *testing.T) {
				root := &domain.EnterpriseAccessContext{PrincipalID: "root", IsSuperAdmin: true, DepartmentIDs: []string{"root-department"}}
				if !check(policy, root) {
					t.Fatal("super-admin must bypass ancestor, department, level, whitelist and deny rules")
				}
				for _, other := range []*domain.EnterpriseAccessContext{
					{PrincipalID: "root"}, // A matching account name grants nothing.
					{PrincipalID: "manager"}, {PrincipalID: "reader"}, nil,
					{IsSuperAdmin: true}, {PrincipalID: "visitor", IsSuperAdmin: true},
				} {
					if check(policy, other) {
						t.Fatal("fallback must not authorize ordinary or unauthenticated principals")
					}
				}
			})
		}
	}
	after, _ := json.Marshal(policy)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("fallback must not modify saved document permissions")
	}
	var supplied domain.EnterpriseAccessContext
	if err := json.Unmarshal([]byte(`{"principal_id":"reader","IsSuperAdmin":true,"is_super_admin":true}`), &supplied); err != nil || supplied.IsSuperAdmin {
		t.Fatal("client JSON must not create a super-admin principal")
	}
}

func TestSuperAdminIsAlreadyWithinEveryManagementParentScope(t *testing.T) {
	users := []domain.UserSummary{
		{ID: "root", Role: consts.UserRoleAdmin},
		{ID: "manager", Role: consts.UserRoleManager},
	}
	grant := func(id string) []domain.DocumentManagementGrant {
		return []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{id}, DirectoryScope: domain.ManagementScopeTree}}
	}
	parent := [][]domain.DocumentManagementGrant{grant("another-manager")}
	if !managementGrantAudienceWithinParents(users, grant("root"), parent, nil) {
		t.Fatal("global super-admin access cannot be restricted by a parent whitelist")
	}
	if managementGrantAudienceWithinParents(users, grant("manager"), parent, nil) {
		t.Fatal("ordinary managers must still be constrained by the parent scope")
	}
}
