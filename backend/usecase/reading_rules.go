package usecase

import (
	"context"
	"fmt"

	"github.com/chaitin/panda-wiki/domain"
)

func readingPolicyAtDistance(policy domain.NodePermissions, distance int, document bool) domain.NodePermissions {
	result := policy
	result.ReadingRules = nil
	result.Constraints = nil
	result.ParentPermissionSnapshot = nil
	result.ReadingBoundary = true
	inherit := false
	result.Inherit = &inherit
	for _, rule := range policy.ReadingRules {
		if distance == 0 || rule.IncludeDescendants || (distance == 1 && document) {
			result.ReadingRules = append(result.ReadingRules, rule)
		}
	}
	return result
}

func (u *EnterpriseUsecase) normalizeReadingRules(ctx context.Context, policy domain.NodePermissions) (domain.NodePermissions, error) {
	// Strip client-supplied computed constraints and legacy top-level allow fields.
	result := domain.NodePermissions{ReadingVersion: 2, Inherit: policy.Inherit, ManagementGrants: policy.ManagementGrants, DeniedUserIDs: policy.DeniedUserIDs, DeniedDepartmentIDs: policy.DeniedDepartmentIDs}
	if policy.Inherit != nil && *policy.Inherit {
		return result, nil
	}
	if len(policy.ReadingRules) > 100 {
		return result, fmt.Errorf("查阅规则最多100条")
	}
	validDepartments := map[string]bool{}
	var validUsers map[string]bool
	validateIDs := func(p domain.NodePermissions) error {
		for _, id := range append(append([]string{}, p.AllowedDepartmentIDs...), p.DeniedDepartmentIDs...) {
			if !validDepartments[id] {
				department, err := u.repo.GetDepartmentByID(ctx, id)
				if err != nil || department.Status != domain.DepartmentStatusActive {
					return fmt.Errorf("查阅部门不存在或已停用：%s", id)
				}
				validDepartments[id] = true
			}
		}
		ids := append(append([]string{}, p.AllowedUserIDs...), p.DeniedUserIDs...)
		if len(ids) > 0 && validUsers == nil {
			users, err := u.repo.PermissionAudienceUsers(ctx)
			if err != nil {
				return err
			}
			validUsers = map[string]bool{}
			for _, user := range users {
				validUsers[user.ID] = true
			}
		}
		for _, id := range ids {
			if !validUsers[id] {
				return fmt.Errorf("查阅用户不存在：%s", id)
			}
		}
		return nil
	}
	if err := validateIDs(result); err != nil {
		return result, err
	}
	visited := 0
	var validate func(domain.NodePermissions, int) (domain.NodePermissions, error)
	validate = func(p domain.NodePermissions, depth int) (domain.NodePermissions, error) {
		visited++
		if depth > 20 || visited > 1000 || (p.ReadingVersion != 0 && (p.ReadingVersion != 2 || depth == 0)) || (p.ReadingVersion == 0 && len(p.ReadingRules) > 0) {
			return p, fmt.Errorf("查阅规则结构无效")
		}
		if err := validateIDs(p); err != nil {
			return p, err
		}
		p.ManagementGrants = nil
		p.Inherit = nil
		p.ParentPermissionSnapshot = nil
		if p.ReadingVersion == 2 {
			if len(p.ReadingRules) > 100 {
				return p, fmt.Errorf("查阅规则最多100条")
			}
			for i, rule := range p.ReadingRules {
				var err error
				p.ReadingRules[i].Permissions, err = validate(rule.Permissions, depth+1)
				if err != nil {
					return p, err
				}
			}
		}
		for i, c := range p.Constraints {
			var err error
			p.Constraints[i], err = validate(c, depth+1)
			if err != nil {
				return p, err
			}
		}
		if p.ReadingVersion == 0 && len(p.Constraints) == 0 {
			if err := validateNodePermissionScope(p); err != nil {
				return p, err
			}
		}
		rank := p.MinPermissionLevelRank
		normalized, err := u.normalizePermissionLevelReferences(ctx, p)
		// Historical snapshots can contain a rank without a level ID. Dropping
		// that rank during conversion would silently widen their audience.
		if p.MinPermissionLevelID == "" {
			normalized.MinPermissionLevelRank = rank
		}
		return normalized, err
	}
	for _, rule := range policy.ReadingRules {
		// Empty conditions would accidentally authorize everybody. Require an
		// explicit public/all choice or at least one actual audience condition.
		p := rule.Permissions
		if !p.AllowAllUsers && p.ScopeType != "public" && len(p.Constraints) == 0 && len(p.AllowedDepartmentIDs) == 0 && len(p.AllowedPermissionLevelIDs) == 0 && p.MinPermissionLevelID == "" && p.MinPermissionLevelRank == 0 && len(p.AllowedUserIDs) == 0 {
			return result, fmt.Errorf("空规则不能保存，请指定对象或明确选择全员")
		}
		normalized, err := validate(p, 0)
		if err != nil {
			return result, err
		}
		result.ReadingRules = append(result.ReadingRules, domain.ReadingRule{Permissions: normalized, IncludeDescendants: rule.IncludeDescendants})
	}
	return result, nil
}
