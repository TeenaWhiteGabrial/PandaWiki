package usecase

import (
	"context"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
	"testing"
)

func readingPolicyForTest(p domain.NodePermissions, descendants bool) domain.NodePermissions {
	return domain.NodePermissions{ReadingVersion: 2, ReadingRules: []domain.ReadingRule{{Permissions: p, IncludeDescendants: descendants}}}
}
func TestReadingRulesDepartmentAndLevelOrOtherRule(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	u := &EnterpriseUsecase{}
	p := readingPolicyForTest(domain.NodePermissions{ScopeType: "custom", AllowedDepartmentIDs: []string{"dev"}, AllowedPermissionLevelIDs: []string{"secret"}}, true)
	p.ReadingRules = append(p.ReadingRules, domain.ReadingRule{Permissions: domain.NodePermissions{ScopeType: "permission_level", AllowedPermissionLevelIDs: []string{"public"}}, IncludeDescendants: true}, domain.ReadingRule{Permissions: domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"named"}}, IncludeDescendants: true})
	for _, tc := range []struct {
		id, dept, level string
		want            bool
	}{
		{"dev-secret", "dev", "secret", true}, {"dev-top", "dev", "top", false}, {"sales-secret", "sales", "secret", false}, {"sales-public", "sales", "public", true}, {"named", "sales", "top", true}, {"other", "sales", "top", false},
	} {
		a := &domain.EnterpriseAccessContext{PrincipalID: tc.id, DepartmentIDs: []string{tc.dept}, PermissionLevelID: tc.level, PermissionLevelRank: 100}
		for name, check := range map[string]func(domain.NodePermissions, *domain.EnterpriseAccessContext) bool{"tree": u.CanVisible, "document": u.CanVisitable, "qa": u.CanAnswerable} {
			if check(p, a) != tc.want {
				t.Fatalf("%s %s got wrong access", name, tc.id)
			}
		}
	}
	p.DeniedUserIDs = []string{"named"}
	if u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "named"}) {
		t.Fatal("exclude must override named rule")
	}
	p.DeniedDepartmentIDs = []string{"dev"}
	if u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "dev-secret", DepartmentIDs: []string{"dev"}, PermissionLevelID: "secret"}) {
		t.Fatal("department exclusion ignored")
	}
}
func TestReadingRulesRequireLoginAndKeepSuperAdminFallback(t *testing.T) {
	u := &EnterpriseUsecase{}
	p := readingPolicyForTest(domain.NodePermissions{AllowAllUsers: true}, true)
	for _, a := range []*domain.EnterpriseAccessContext{nil, {}, {PrincipalID: "visitor"}} {
		if u.CanVisitable(p, a) {
			t.Fatal("all-users must require login")
		}
	}
	if !u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "user"}) {
		t.Fatal("all-users rejected logged-in user")
	}
	p.ReadingRules = nil
	if u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "user"}) {
		t.Fatal("empty rules widened to public")
	}
	if !u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "super", IsSuperAdmin: true}) {
		t.Fatal("super-admin fallback lost")
	}
	if u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "manager"}) {
		t.Fatal("management identity must not imply reading")
	}
}
func TestReadingRuleDirectoryScopeAndIndependentBoundary(t *testing.T) {
	u := &EnterpriseUsecase{}
	inherit := true
	parent := &domain.Node{ID: "parent", Permissions: readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"a"}}, false)}
	child := &domain.Node{ID: "child", ParentID: "parent"}
	nodes := map[string]*domain.Node{"parent": parent, "child": child}
	lookup := func(id string) (*domain.Node, error) {
		if n := nodes[id]; n != nil {
			return n, nil
		}
		return nil, gorm.ErrRecordNotFound
	}
	a := &domain.EnterpriseAccessContext{PrincipalID: "a"}
	b := &domain.EnterpriseAccessContext{PrincipalID: "b"}
	for _, tc := range []struct {
		name, parent   string
		document, want bool
	}{
		{"parent", "", false, true}, {"direct-file", "parent", true, true}, {"child", "parent", false, false}, {"nested-file", "child", true, false},
	} {
		p := domain.NodePermissions{ReadingVersion: 2, Inherit: &inherit, ReadingTargetDocument: tc.document}
		if tc.name == "parent" {
			p = parent.Permissions
		}
		resolved, err := resolvePermissionHierarchy(tc.name, tc.parent, p, lookup)
		if err != nil || u.CanVisitable(resolved, a) != tc.want {
			t.Fatalf("scope %s: %#v %v", tc.name, resolved, err)
		}
	}
	// An independent child grants outsiders, replacing folder and nav rules.
	child.Permissions = readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"b"}}, true)
	p, err := resolvePermissionHierarchy("file", "child", domain.NodePermissions{ReadingTargetDocument: true}, lookup)
	p = u.MergeNodeAndNavPermissions(p, readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"a"}}, true))
	if err != nil || !u.CanVisitable(p, b) || u.CanVisitable(p, a) {
		t.Fatalf("independent child still intersected ancestor: %#v %v", p, err)
	}
	// Parent changes do not affect independent descendants.
	parent.Permissions = readingPolicyForTest(domain.NodePermissions{}, true)
	p, err = resolvePermissionHierarchy("file", "child", domain.NodePermissions{ReadingTargetDocument: true}, lookup)
	if err != nil || !u.CanVisitable(p, b) {
		t.Fatal("parent edit changed independent subtree")
	}
	// Restoring inheritance follows the parent live and drops the child's grant.
	child.Permissions = domain.NodePermissions{ReadingVersion: 2, Inherit: &inherit}
	parent.Permissions = readingPolicyForTest(domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"a"}}, true)
	p, err = resolvePermissionHierarchy("file", "child", domain.NodePermissions{ReadingTargetDocument: true}, lookup)
	if err != nil || !u.CanVisitable(p, a) || u.CanVisitable(p, b) {
		t.Fatal("restore-inherit failed")
	}
}
func TestReadingNavScopeAndInheritedEnvelope(t *testing.T) {
	u := &EnterpriseUsecase{}
	inherit := true
	nav := readingPolicyForTest(domain.NodePermissions{AllowAllUsers: true}, false)
	for _, document := range []bool{false, true} {
		p, err := resolvePermissionHierarchy("root", "", domain.NodePermissions{ReadingVersion: 2, Inherit: &inherit, ReadingTargetDocument: document}, func(string) (*domain.Node, error) { return nil, gorm.ErrRecordNotFound })
		p = u.MergeNodeAndNavPermissions(p, nav)
		if err != nil || u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "a"}) != document {
			t.Fatalf("nav current-only leaked or denied direct document: %#v", p)
		}
	}
	// Batch preview's AND constraints must be honored even on a v2 primary rule.
	p := readingPolicyForTest(domain.NodePermissions{AllowAllUsers: true}, true)
	p.Constraints = []domain.NodePermissions{{ScopeType: "user", AllowedUserIDs: []string{"a"}}}
	if u.CanVisitable(p, &domain.EnterpriseAccessContext{PrincipalID: "b"}) {
		t.Fatal("batch intersection became union")
	}
	if unrestrictedReadingPolicy(domain.NodePermissions{ReadingVersion: 2}) {
		t.Fatal("empty rules described as unrestricted")
	}
}
func TestReadingRuleValidationRejectsEmptyAndForgedStructures(t *testing.T) {
	u := &EnterpriseUsecase{}
	for _, p := range []domain.NodePermissions{
		{ReadingVersion: 9}, readingPolicyForTest(domain.NodePermissions{}, true), readingPolicyForTest(domain.NodePermissions{ReadingVersion: 2, AllowAllUsers: true}, true),
	} {
		if _, err := u.NormalizeNodePermissions(context.Background(), p); err == nil {
			t.Fatal("invalid rule accepted")
		}
	}
	p := readingPolicyForTest(domain.NodePermissions{AllowAllUsers: true}, true)
	normalized, err := u.NormalizeNodePermissions(context.Background(), p)
	if err != nil || len(normalized.ReadingRules) != 1 {
		t.Fatalf("explicit all-users rule rejected: %v", err)
	}
}

func TestReadingHistoricalCompoundConversionRemainsIntersection(t *testing.T) {
	u := &EnterpriseUsecase{}
	// A legacy child under a converted parent is copied as one compound rule.
	compound := domain.NodePermissions{Constraints: []domain.NodePermissions{readingPolicyForTest(domain.NodePermissions{AllowAllUsers: true}, true)}}
	p := readingPolicyForTest(compound, true)
	normalized, err := u.NormalizeNodePermissions(context.Background(), p)
	if err != nil || !u.CanVisitable(normalized, &domain.EnterpriseAccessContext{PrincipalID: "a"}) {
		t.Fatalf("mixed legacy/new policy cannot be converted: %v", err)
	}
	legacy := readingPolicyForTest(domain.NodePermissions{MinPermissionLevelRank: 5}, true)
	normalized, err = u.NormalizeNodePermissions(context.Background(), legacy)
	if err != nil || normalized.ReadingRules[0].Permissions.MinPermissionLevelRank != 5 {
		t.Fatalf("historical minimum rank silently removed: %v", err)
	}
}

func TestReadingRuleInheritanceCyclesFailClosed(t *testing.T) {
	_, err := resolvePermissionHierarchy("a", "b", domain.NodePermissions{ReadingVersion: 2}, func(string) (*domain.Node, error) { return &domain.Node{ID: "b", ParentID: "a"}, nil })
	if err == nil {
		t.Fatal("cyclic ancestry accepted")
	}
}
