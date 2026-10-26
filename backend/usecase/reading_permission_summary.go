package usecase

import (
	"slices"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

// (group A OR named A) AND (group B OR named B) consists of the
// intersection of the groups, plus named accounts that pass the full policy.
// Intersecting whitelist IDs alone would incorrectly remove a named account
// that satisfies another ancestor through its department and level.
func (u *EnterpriseUsecase) summarizeReadingPermissions(policy domain.NodePermissions, users []domain.UserSummary, ranks map[string]int) *v1.ReadingPermissionSummary {
	result := &v1.ReadingPermissionSummary{
		GroupAllowed:        true,
		DepartmentGroups:    [][]string{},
		PermissionLevelIDs:  []string{},
		AllowedUserIDs:      []string{},
		DeniedUserIDs:       []string{},
		DeniedDepartmentIDs: []string{},
		UserNames:           map[string]string{},
	}
	named := map[string]bool{}
	appendUnique := func(target *[]string, ids []string) {
		for _, id := range ids {
			if !slices.Contains(*target, id) {
				*target = append(*target, id)
			}
		}
	}
	for _, rule := range snapshotReadPermissionRules(policy) {
		appendUnique(&result.DeniedUserIDs, rule.DeniedUserIDs)
		appendUnique(&result.DeniedDepartmentIDs, rule.DeniedDepartmentIDs)
		for _, id := range rule.AllowedUserIDs {
			named[id] = true
		}
		if rule.AllowAllUsers {
			continue
		}
		departmentRestricted := len(rule.AllowedDepartmentIDs) > 0
		levelRestricted := !config.PermissionLevelsDisabled() && len(rule.AllowedPermissionLevelIDs) > 0
		minimum := rule.MinPermissionLevelRank
		if config.PermissionLevelsDisabled() {
			minimum = 0
		}
		switch rule.ScopeType {
		case "user":
			result.GroupAllowed = false
			continue
		case "permission_level":
			if config.PermissionLevelsDisabled() {
				continue
			}
			departmentRestricted = false
			levelRestricted = true
		case "department":
			departmentRestricted = true
			levelRestricted = false
		default:
			if !departmentRestricted && !levelRestricted && minimum == 0 && len(rule.AllowedUserIDs) > 0 {
				result.GroupAllowed = false
			}
		}
		if departmentRestricted {
			if len(rule.AllowedDepartmentIDs) == 0 {
				result.GroupAllowed = false
			} else {
				group := []string{}
				appendUnique(&group, rule.AllowedDepartmentIDs)
				result.DepartmentGroups = append(result.DepartmentGroups, group)
			}
		}
		if levelRestricted {
			if !result.LevelRestricted {
				appendUnique(&result.PermissionLevelIDs, rule.AllowedPermissionLevelIDs)
			} else {
				intersection := []string{}
				for _, id := range result.PermissionLevelIDs {
					if slices.Contains(rule.AllowedPermissionLevelIDs, id) {
						intersection = append(intersection, id)
					}
				}
				result.PermissionLevelIDs = intersection
			}
			result.LevelRestricted = true
		}
		if minimum > result.MinPermissionLevelRank {
			result.MinPermissionLevelRank = minimum
			result.MinPermissionLevelID = rule.MinPermissionLevelID
		}
	}
	if result.LevelRestricted {
		eligible := []string{}
		for _, id := range result.PermissionLevelIDs {
			if ranks[id] >= result.MinPermissionLevelRank {
				eligible = append(eligible, id)
			}
		}
		result.PermissionLevelIDs = eligible
		if len(eligible) == 0 {
			result.GroupAllowed = false
		}
	}
	// Remove duplicate/broader OR groups. Do not blindly intersect department
	// IDs: a person assigned to multiple departments can satisfy distinct groups.
	for index, group := range result.DepartmentGroups {
		allowed := []string{}
		for _, id := range group {
			if !slices.Contains(result.DeniedDepartmentIDs, id) {
				allowed = append(allowed, id)
			}
		}
		result.DepartmentGroups[index] = allowed
		if len(allowed) == 0 {
			result.GroupAllowed = false
		}
	}
	groups := [][]string{}
	for index, group := range result.DepartmentGroups {
		redundant := false
		for otherIndex, other := range result.DepartmentGroups {
			if index == otherIndex || len(other) > len(group) {
				continue
			}
			subset := true
			for _, id := range other {
				if !slices.Contains(group, id) {
					subset = false
					break
				}
			}
			if subset && (len(other) < len(group) || otherIndex < index) {
				redundant = true
				break
			}
		}
		if !redundant {
			groups = append(groups, group)
		}
	}
	result.DepartmentGroups = groups
	for _, user := range users {
		if named[user.ID] || slices.Contains(result.DeniedUserIDs, user.ID) {
			result.UserNames[user.ID] = user.Account
		}
		if !named[user.ID] {
			continue
		}
		principal := &domain.EnterpriseAccessContext{PrincipalID: user.ID, DepartmentIDs: user.DepartmentIDs, PermissionLevelID: user.PermissionLevelID, PermissionLevelRank: ranks[user.PermissionLevelID]}
		if u.CanVisitable(policy, principal) {
			result.AllowedUserIDs = append(result.AllowedUserIDs, user.ID)
		}
	}
	return result
}
