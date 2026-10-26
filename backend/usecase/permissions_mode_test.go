package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
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
	if !u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "owner", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 2}) {
		t.Fatal("explicit user whitelist should be independent of local department and level restrictions")
	}
	if u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "outsider", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 10}) {
		t.Fatal("custom permission should reject users outside the department/level intersection and whitelist")
	}
	if u.CanVisible(customAudience, &domain.EnterpriseAccessContext{PrincipalID: "junior", DepartmentIDs: []string{"engineering"}, PermissionLevelRank: 2}) {
		t.Fatal("department audience must also satisfy the configured minimum level")
	}

	departmentAndLevel := domain.NodePermissions{
		ScopeType:                 domain.NodePermissionScopeCustom,
		AllowedDepartmentIDs:      []string{"engineering"},
		AllowedPermissionLevelIDs: []string{"staff"},
		AllowedUserIDs:            []string{"owner"},
	}
	if !u.CanVisitable(departmentAndLevel, &domain.EnterpriseAccessContext{PrincipalID: "member", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "staff"}) {
		t.Fatal("matching department and level should pass the group route")
	}
	if u.CanVisitable(departmentAndLevel, &domain.EnterpriseAccessContext{PrincipalID: "member", DepartmentIDs: []string{"engineering"}, PermissionLevelID: "executive"}) {
		t.Fatal("matching only the department must not bypass the level restriction")
	}
	if !u.CanVisitable(departmentAndLevel, &domain.EnterpriseAccessContext{PrincipalID: "owner", DepartmentIDs: []string{"sales"}, PermissionLevelID: "executive"}) {
		t.Fatal("whitelist user should be independent of local department and exact level restrictions")
	}
	departmentAndLevel.DeniedUserIDs = []string{"owner"}
	if u.CanVisitable(departmentAndLevel, &domain.EnterpriseAccessContext{PrincipalID: "owner", DepartmentIDs: []string{"sales"}, PermissionLevelID: "executive"}) {
		t.Fatal("explicit denial must take precedence over the whitelist")
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

func TestPermissionAudienceValidationIsOnlyAppliedToWrites(t *testing.T) {
	malformedLegacyRule := domain.NodePermissions{
		ScopeType:            domain.NodePermissionScopeDepartment,
		MinPermissionLevelID: "default",
	}
	if err := validateNodePermissionScope(malformedLegacyRule); err == nil {
		t.Fatal("saving a department rule without a department must be rejected")
	}

	// A read path must not require a database lookup just to decide whether the
	// historical audience fields are structurally complete. An empty department
	// audience remains restrictive in canAccess rather than becoming public.
	readRule := domain.NodePermissions{ScopeType: domain.NodePermissionScopeDepartment}
	if err := validateResolvedRuleForTest(context.Background(), &EnterpriseUsecase{}, readRule); err != nil {
		t.Fatalf("reading an incomplete legacy rule should not fail: %v", err)
	}
	if (&EnterpriseUsecase{}).CanVisible(readRule, &domain.EnterpriseAccessContext{PrincipalID: "user", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("an incomplete department rule must remain fail-closed")
	}
}

func validateResolvedRuleForTest(ctx context.Context, u *EnterpriseUsecase, rule domain.NodePermissions) error {
	_, err := u.normalizeResolvedNodePermissions(ctx, rule)
	return err
}

func TestPermissionHierarchyOnlyNarrowsParentAudience(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	parent := domain.NodePermissions{ScopeType: "department", AllowedDepartmentIDs: []string{"engineering"}}
	child := domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"alice"}}
	effective := composeNodePermissionHierarchy([]nodePermissionHierarchyEntry{
		{permissions: child, parentID: "parent"},
		{permissions: parent, parentID: "nav"},
	})
	u := &EnterpriseUsecase{}
	if !u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a child-selected user inside the parent audience should retain access")
	}
	if u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "bob", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a parent-authorized user outside the child selection should be narrowed out")
	}
	if u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"sales"}}) {
		t.Fatal("a child rule must not add a user outside the parent audience")
	}
}

func TestLegacyDescendantLockDoesNotBlockPyramidPermissions(t *testing.T) {
	var parent domain.NodePermissions
	if err := json.Unmarshal([]byte(`{"scope_type":"user","allowed_user_ids":["alice","bob"],"deny_descendant_overrides":true}`), &parent); err != nil {
		t.Fatal(err)
	}
	child := domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"alice"}}
	effective := composeNodePermissionHierarchy([]nodePermissionHierarchyEntry{
		{permissions: child, parentID: "parent"},
		{permissions: parent, parentID: "nav"},
	})
	u := &EnterpriseUsecase{}
	if !u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "alice"}) {
		t.Fatal("legacy lock data must not prevent a child from applying a narrower rule")
	}
	if u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "bob"}) {
		t.Fatal("the child rule must still narrow the parent's audience")
	}
}

func boolPointer(value bool) *bool { return &value }

func TestReadingAudienceModesAndNavIntersection(t *testing.T) {
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	u := &EnterpriseUsecase{}
	access := &domain.EnterpriseAccessContext{PrincipalID: "alice", PermissionLevelID: "staff", PermissionLevelRank: 5, DepartmentIDs: []string{"engineering"}}
	for _, perm := range []domain.NodePermissions{
		{ScopeType: "permission_level", AllowedPermissionLevelIDs: []string{"staff"}},
		{ScopeType: "department", AllowedDepartmentIDs: []string{"engineering"}},
		{ScopeType: "user", AllowedUserIDs: []string{"alice"}},
	} {
		if !u.CanVisitable(perm, access) {
			t.Fatal("matching audience cannot read")
		}
		outsider := &domain.EnterpriseAccessContext{PrincipalID: "outsider", PermissionLevelID: "executive", PermissionLevelRank: 100, DepartmentIDs: []string{"sales"}}
		if u.CanVisitable(perm, outsider) {
			t.Fatal("outsider bypassed audience")
		}
	}
	node := domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"alice"}}
	nav := domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{"bob"}}
	if u.CanVisitable(u.MergeNodeAndNavPermissions(node, nav), access) {
		t.Fatal("empty audience intersection must not become public")
	}
	legacyWithWhitelist := domain.NodePermissions{
		ScopeType:              "department",
		AllowedDepartmentIDs:   []string{"engineering"},
		AllowedUserIDs:         []string{"alice"},
		MinPermissionLevelRank: 10,
	}
	if !u.CanVisitable(legacyWithWhitelist, &domain.EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"sales"}, PermissionLevelRank: 1}) {
		t.Fatal("an explicitly selected user must remain independent of legacy department and level restrictions")
	}
}

func TestAllUsersBranchAndExcludedUsersRespectHierarchy(t *testing.T) {
	u := &EnterpriseUsecase{}
	allUsers := domain.NodePermissions{
		ScopeType:      domain.NodePermissionScopeCustom,
		AllowAllUsers:  true,
		AllowedUserIDs: []string{"alice"},
		DeniedUserIDs:  []string{"alice"},
	}
	if u.CanVisible(allUsers, &domain.EnterpriseAccessContext{PrincipalID: "alice"}) {
		t.Fatal("an explicit user exclusion must override both all-users and personal grants")
	}
	if !u.CanVisible(allUsers, &domain.EnterpriseAccessContext{PrincipalID: "bob"}) {
		t.Fatal("the all-users branch should allow users who are not excluded")
	}

	parent := domain.NodePermissions{
		ScopeType:            domain.NodePermissionScopeDepartment,
		AllowedDepartmentIDs: []string{"engineering"},
	}
	child := u.MergeNodeAndNavPermissions(domain.NodePermissions{
		ScopeType:     domain.NodePermissionScopeCustom,
		AllowAllUsers: true,
		DeniedUserIDs: []string{"alice"},
	}, parent)
	if u.CanVisible(child, &domain.EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a child exclusion must override inherited department access")
	}
	if !u.CanVisible(child, &domain.EnterpriseAccessContext{PrincipalID: "bob", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a child all-users branch should still be bounded by the inherited department rule")
	}
	if u.CanVisible(child, &domain.EnterpriseAccessContext{PrincipalID: "carol", DepartmentIDs: []string{"sales"}}) {
		t.Fatal("the child must not use all-users to broaden beyond its parent")
	}
	grandchild := domain.NodePermissions{
		ScopeType:      domain.NodePermissionScopeCustom,
		AllowedUserIDs: []string{"alice"},
	}
	grandchild.ParentPermissionSnapshot = snapshotReadPermissionRules(child)
	effectiveGrandchild := composeNodePermissionHierarchy([]nodePermissionHierarchyEntry{
		{permissions: grandchild, parentID: "child"},
		{permissions: child, parentID: "parent"},
		{permissions: parent, parentID: "nav"},
	})
	if u.CanVisible(effectiveGrandchild, &domain.EnterpriseAccessContext{PrincipalID: "alice", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a lower directory must not regrant a user excluded by an ancestor")
	}
}

func TestSavedChildRuleKeepsParentAudienceSnapshot(t *testing.T) {
	u := &EnterpriseUsecase{}
	oldParent := domain.NodePermissions{
		ScopeType:            domain.NodePermissionScopeDepartment,
		AllowedDepartmentIDs: []string{"engineering"},
	}
	child := domain.NodePermissions{
		ScopeType:                domain.NodePermissionScopeCustom,
		AllowAllUsers:            true,
		ParentPermissionSnapshot: snapshotReadPermissionRules(oldParent),
	}
	parentAfterExpansion := domain.NodePermissions{
		ScopeType:            domain.NodePermissionScopeDepartment,
		AllowedDepartmentIDs: []string{"engineering", "sales"},
	}
	effective := composeNodePermissionHierarchy([]nodePermissionHierarchyEntry{
		{permissions: child, parentID: "parent"},
		{permissions: parentAfterExpansion, parentID: "nav"},
	})
	if !u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "eng-user", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("the saved child baseline should keep the original parent audience")
	}
	if u.CanVisible(effective, &domain.EnterpriseAccessContext{PrincipalID: "sales-user", DepartmentIDs: []string{"sales"}}) {
		t.Fatal("a later parent expansion must not widen the saved child audience")
	}

	parentAfterTightening := domain.NodePermissions{
		ScopeType:            domain.NodePermissionScopeDepartment,
		AllowedDepartmentIDs: []string{"engineering"},
	}
	tightened := composeNodePermissionHierarchy([]nodePermissionHierarchyEntry{
		{permissions: child, parentID: "parent"},
		{permissions: parentAfterTightening, parentID: "nav"},
	})
	if !u.CanVisible(tightened, &domain.EnterpriseAccessContext{PrincipalID: "eng-user", DepartmentIDs: []string{"engineering"}}) {
		t.Fatal("a later parent tightening should continue to apply")
	}
}

func TestCustomUserWhitelistDoesNotBecomePublic(t *testing.T) {
	u := &EnterpriseUsecase{}
	permission := domain.NodePermissions{
		ScopeType:      domain.NodePermissionScopeCustom,
		AllowedUserIDs: []string{"alice"},
	}
	if !u.CanVisible(permission, &domain.EnterpriseAccessContext{PrincipalID: "alice"}) {
		t.Fatal("an explicitly selected user should be allowed")
	}
	if u.CanVisible(permission, &domain.EnterpriseAccessContext{PrincipalID: "bob"}) {
		t.Fatal("a custom rule with only a user grant must not become public")
	}
}

func TestParentPermissionSnapshotIsStoredButNotExposedByJSON(t *testing.T) {
	permissions := domain.NodePermissions{
		ScopeType: domain.NodePermissionScopeCustom,
		ParentPermissionSnapshot: []domain.NodePermissions{{
			ScopeType:            domain.NodePermissionScopeDepartment,
			AllowedDepartmentIDs: []string{"engineering"},
		}},
	}
	apiJSON, err := json.Marshal(permissions)
	if err != nil {
		t.Fatal(err)
	}
	var apiObject map[string]json.RawMessage
	if err := json.Unmarshal(apiJSON, &apiObject); err != nil {
		t.Fatal(err)
	}
	if _, exists := apiObject["parent_permission_snapshot"]; exists {
		t.Fatal("internal parent policy snapshots must not be exposed in API JSON")
	}

	value, err := permissions.Value()
	if err != nil {
		t.Fatal(err)
	}
	storedJSON, ok := value.([]byte)
	if !ok {
		t.Fatalf("expected JSONB value to be []byte, got %T", value)
	}
	var storedObject map[string]json.RawMessage
	if err := json.Unmarshal(storedJSON, &storedObject); err != nil {
		t.Fatal(err)
	}
	if _, exists := storedObject["parent_permission_snapshot"]; !exists {
		t.Fatal("database JSONB value must persist the parent policy snapshot")
	}
	var decoded domain.NodePermissions
	if err := decoded.Scan(storedJSON); err != nil {
		t.Fatal(err)
	}
	if decoded.ScopeType != domain.NodePermissionScopeCustom || len(decoded.ParentPermissionSnapshot) != 1 || decoded.ParentPermissionSnapshot[0].AllowedDepartmentIDs[0] != "engineering" {
		t.Fatal("database JSONB scan did not restore the parent policy snapshot")
	}
}
