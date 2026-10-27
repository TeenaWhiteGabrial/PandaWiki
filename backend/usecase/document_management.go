package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
)

func (u *EnterpriseUsecase) ManagementAccess(ctx context.Context, kbID string) (*domain.DocumentManagementAccess, error) {
	return u.managementAccess(ctx, kbID, false)
}

func (u *EnterpriseUsecase) ManagementTreeAccess(ctx context.Context, kbID string) (*domain.DocumentManagementAccess, error) {
	return u.managementAccess(ctx, kbID, true)
}

func (u *EnterpriseUsecase) managementAccess(ctx context.Context, kbID string, policyOnly bool) (*domain.DocumentManagementAccess, error) {
	load := u.repo.DocumentManagementAccess
	if policyOnly {
		load = u.repo.DocumentManagementTreeAccess
	}

	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return nil, fmt.Errorf("未登录")
	}
	if auth.IsToken {
		if auth.KBId != kbID {
			return nil, fmt.Errorf("知识库不匹配")
		}
		ownerAccess, err := load(ctx, kbID, auth.UserId)
		if err != nil {
			return nil, err
		}
		if auth.Permission != consts.UserKBPermissionDocManage && auth.Permission != consts.UserKBPermissionFullControl {
			ownerAccess.Global = false
			for _, node := range ownerAccess.Nodes {
				node.Permissions.ManagementGrants = nil
			}
			for _, nav := range ownerAccess.Navs {
				nav.Permissions.ManagementGrants = nil
			}
		}
		return ownerAccess, nil
	}
	if access := domain.DocumentManagementAccessFromContext(ctx, kbID, auth.UserId); access != nil {
		return access, nil
	}
	return load(ctx, kbID, auth.UserId)
}

func (u *EnterpriseUsecase) NormalizeManagementGrants(ctx context.Context, grants []domain.DocumentManagementGrant) error {
	var validUsers map[string]bool
	for i := range grants {
		grant := &grants[i]
		if len(grant.SubjectIDs) == 0 {
			return fmt.Errorf("请选择管理授权对象")
		}
		if grant.DirectoryScope == "" {
			grant.DirectoryScope = domain.ManagementScopeTree
		}
		if !slices.Contains([]string{domain.ManagementScopeTree, domain.ManagementScopeCurrent}, grant.DirectoryScope) {
			return fmt.Errorf("目录权限范围无效")
		}
		for _, id := range grant.SubjectIDs {
			switch grant.SubjectType {
			case domain.ManagementSubjectLevel:
				level, err := u.repo.GetPermissionLevelByID(ctx, id)
				if err != nil || level.Status != domain.PermissionLevelStatusActive {
					return fmt.Errorf("管理授权等级不存在或已停用")
				}
			case domain.ManagementSubjectDepartment:
				department, err := u.repo.GetDepartmentByID(ctx, id)
				if err != nil || department.Status != domain.DepartmentStatusActive {
					return fmt.Errorf("管理授权部门不存在或已停用")
				}
			case domain.ManagementSubjectUser:
				if validUsers == nil {
					users, err := u.repo.PermissionAudienceUsers(ctx)
					if err != nil {
						return err
					}
					validUsers = map[string]bool{}
					for _, user := range users {
						validUsers[user.ID] = consts.IsAdministrator(user.Role)
					}
				}
				if !validUsers[id] {
					return fmt.Errorf("管理授权只能指定管理员账号，请删除普通用户授权")
				}
			default:
				return fmt.Errorf("管理授权方式无效")
			}
		}
	}
	return nil
}

// validateManagementGrantChanges keeps historical department/level grants
// effective until an administrator removes them, but prevents adding or
// widening those group grants. New management access is assigned to named
// administrator accounts only.
func validateManagementGrantChanges(currentGroups [][]domain.DocumentManagementGrant, grants []domain.DocumentManagementGrant) error {
	for _, grant := range grants {
		if grant.DirectoryScope == domain.ManagementScopeCurrent && !containsManagementGrantInEveryGroup(currentGroups, grant) {
			return fmt.Errorf("新管理授权范围只能是当前及下级目录；历史的当前目录规则可以保留或调整为当前及下级目录")
		}
		if grant.SubjectType == domain.ManagementSubjectUser {
			continue
		}
		if !containsManagementGrantInEveryGroup(currentGroups, grant) {
			return fmt.Errorf("新的管理权限只能指定管理员账号；历史部门或等级规则可保留或删除，不能新增或修改")
		}
	}
	return nil
}

func containsManagementGrantInEveryGroup(groups [][]domain.DocumentManagementGrant, candidate domain.DocumentManagementGrant) bool {
	if len(groups) == 0 {
		return false
	}
	for _, group := range groups {
		if !containsManagementGrant(group, candidate) {
			return false
		}
	}
	return true
}

func containsManagementGrant(grants []domain.DocumentManagementGrant, candidate domain.DocumentManagementGrant) bool {
	canonical := func(grant domain.DocumentManagementGrant) domain.DocumentManagementGrant {
		if grant.DirectoryScope == "" {
			grant.DirectoryScope = domain.ManagementScopeTree
		}
		grant.SubjectIDs = append([]string(nil), grant.SubjectIDs...)
		sort.Strings(grant.SubjectIDs)
		return grant
	}
	candidate = canonical(candidate)
	for _, grant := range grants {
		grant = canonical(grant)
		if grant.SubjectType == candidate.SubjectType && grant.DirectoryScope == candidate.DirectoryScope && slices.Equal(grant.SubjectIDs, candidate.SubjectIDs) {
			return true
		}
	}
	return false
}

// ValidateManagementGrantTargets permits delegation to any administrator.
// Nested-directory scope is enforced separately by ValidateManagementGrantNarrowing.
// Historical level/department grants are admitted only when unchanged.
func (u *EnterpriseUsecase) ValidateManagementGrantTargets(ctx context.Context, access *domain.DocumentManagementAccess, grants []domain.DocumentManagementGrant) error {
	if access == nil {
		return fmt.Errorf("没有文档管理权限")
	}
	if access.Global {
		return nil
	}
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return fmt.Errorf("未登录")
	}
	users, err := u.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return err
	}
	usersByID := make(map[string]domain.UserSummary, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}
	for _, grant := range grants {
		switch grant.SubjectType {
		case domain.ManagementSubjectLevel:
			// Existing level-based grants are accepted only when callers prove
			// they are unchanged through validateManagementGrantChanges.
			continue
		case domain.ManagementSubjectDepartment:
			// Existing department-based grants are accepted only when unchanged.
			continue
		case domain.ManagementSubjectUser:
			for _, id := range grant.SubjectIDs {
				user, ok := usersByID[id]
				if !ok || !consts.IsAdministrator(user.Role) {
					return fmt.Errorf("管理权限只能分配给管理员账号")
				}
			}
		default:
			return fmt.Errorf("管理授权方式无效")
		}
	}
	return nil
}

// ValidateManagementGrantNarrowing makes each nested node's manager list a
// subset of the effective manager audience inherited from parent folders.
func (u *EnterpriseUsecase) ValidateManagementGrantNarrowing(ctx context.Context, access *domain.DocumentManagementAccess, nodeIDs []string, grants []domain.DocumentManagementGrant) error {
	if access == nil || len(grants) == 0 {
		return nil
	}
	users, err := u.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return err
	}
	departmentAncestors, err := u.departmentAncestorIDs(ctx)
	if err != nil {
		return err
	}
	for _, nodeID := range nodeIDs {
		inherited := access.ManagementGrantConstraintsForParentNode(nodeID)
		if len(inherited) == 0 {
			if node := access.Nodes[nodeID]; node != nil && node.ParentID != "" && node.ParentID != node.NavId {
				return fmt.Errorf("上级目录尚未配置可委派的管理员范围，请先配置上级目录管理权限")
			}
			continue
		}
		if !managementGrantAudienceWithinParents(users, grants, inherited, departmentAncestors) {
			return fmt.Errorf("下级目录的管理权限只能在上级已授权管理员范围内收紧")
		}
	}
	return nil
}

func managementGrantAudienceWithinParents(users []domain.UserSummary, grants []domain.DocumentManagementGrant, inherited [][]domain.DocumentManagementGrant, departmentAncestors map[string][]string) bool {
	for _, user := range users {
		// Super-admins already manage both parent and child globally.
		if !consts.IsAdministrator(user.Role) || consts.IsSuperAdmin(user.Role) {
			continue
		}
		principal := &domain.EnterpriseAccessContext{
			PrincipalID:       user.ID,
			PermissionLevelID: user.PermissionLevelID,
			DepartmentIDs:     expandAudienceDepartmentIDs(user.DepartmentIDs, departmentAncestors),
		}
		selected := false
		for _, grant := range grants {
			if grant.Matches(principal) {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		for _, group := range inherited {
			matchesGroup := false
			for _, grant := range group {
				if grant.Matches(principal) {
					matchesGroup = true
					break
				}
			}
			if !matchesGroup {
				return false
			}
		}
	}
	return true
}

// ValidateNodePermissionAudience keeps read permissions within inherited
// parent rules while allowing managers to target any organization member.
func (u *EnterpriseUsecase) ValidateNodePermissionAudience(ctx context.Context, kbID string, permissions, current domain.NodePermissions, parentID, navID string) error {
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return fmt.Errorf("未登录")
	}
	if err := u.validateReadAudienceWithinParent(ctx, kbID, permissions, parentID, navID); err != nil {
		return err
	}
	access, err := u.ManagementAccess(ctx, kbID)
	if err != nil {
		return err
	}
	if access.Global {
		return nil
	}
	currentInherits := current.Inherit != nil && *current.Inherit
	if current.Inherit == nil && parentID != "" && !hasExplicitReadRule(current) {
		currentInherits = true
	}
	if permissions.Inherit != nil && *permissions.Inherit {
		// Resetting a node to inherit its parent cannot broaden access beyond
		// the parent rule; runtime checks still apply the saved snapshot and
		// every current ancestor constraint.
		return nil
	}
	if isUnrestrictedReadAudience(permissions) && parentID == "" && navID == "" && !isDirectlyUnrestrictedReadAudience(current, permissions, parentID, currentInherits) {
		return fmt.Errorf("普通管理员不能将文档查阅范围设置为全员公开")
	}

	departments, err := u.repo.ListDepartments(ctx)
	if err != nil {
		return err
	}
	activeDepartments := make(map[string]struct{}, len(departments))
	for _, department := range departments {
		if department.Status == domain.DepartmentStatusActive {
			activeDepartments[department.ID] = struct{}{}
		}
	}
	for _, id := range permissions.AllowedDepartmentIDs {
		if _, ok := activeDepartments[id]; !ok {
			if !slices.Contains(current.AllowedDepartmentIDs, id) {
				return fmt.Errorf("查阅权限部门不存在或已停用")
			}
		}
	}
	for _, id := range permissions.DeniedDepartmentIDs {
		if _, ok := activeDepartments[id]; !ok && !slices.Contains(current.DeniedDepartmentIDs, id) {
			return fmt.Errorf("拒绝查阅的部门不存在或已停用")
		}
	}
	users, err := u.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return err
	}
	usersByID := make(map[string]domain.UserSummary, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}
	for _, id := range permissions.AllowedUserIDs {
		if _, ok := usersByID[id]; !ok && !slices.Contains(current.AllowedUserIDs, id) {
			return fmt.Errorf("查阅权限用户不存在")
		}
	}
	for _, id := range permissions.DeniedUserIDs {
		if _, ok := usersByID[id]; !ok && !slices.Contains(current.DeniedUserIDs, id) {
			return fmt.Errorf("拒绝查阅的用户不存在")
		}
	}
	return nil
}

// ParentPermissionSnapshot returns a flattened copy of the parent's current
// effective reader policy. It is stored on a newly configured child so later
// parent widening does not silently enlarge the child's saved audience.
func (u *EnterpriseUsecase) ParentPermissionSnapshot(ctx context.Context, kbID, parentID, navID string) ([]domain.NodePermissions, error) {
	permissions, err := u.resolveParentReadPermissions(ctx, kbID, parentID, navID)
	if err != nil {
		return nil, err
	}
	return snapshotReadPermissionRules(permissions), nil
}

// validateReadAudienceWithinParent validates the effective child audience after
// intersecting its local rule with the parent. A local all-users branch means
// every user allowed by that parent; it must not be rejected as a public grant.
func (u *EnterpriseUsecase) validateReadAudienceWithinParent(ctx context.Context, kbID string, candidate domain.NodePermissions, parentID, navID string) error {
	if candidate.Inherit != nil && *candidate.Inherit {
		return nil
	}
	parentPermissions, err := u.resolveParentReadPermissions(ctx, kbID, parentID, navID)
	if err != nil {
		return err
	}
	effectiveCandidate := u.MergeNodeAndNavPermissions(candidate, parentPermissions)
	users, err := u.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return err
	}
	departmentAncestors, err := u.departmentAncestorIDs(ctx)
	if err != nil {
		return err
	}
	levels, err := u.repo.ListPermissionLevels(ctx)
	if err != nil {
		return err
	}
	levelRanks := make(map[string]int, len(levels))
	for _, level := range levels {
		levelRanks[level.ID] = level.Rank
	}
	for _, user := range users {
		departmentIDs := expandAudienceDepartmentIDs(user.DepartmentIDs, departmentAncestors)
		userAccess := &domain.EnterpriseAccessContext{
			PrincipalID:         user.ID,
			IsSuperAdmin:        consts.IsSuperAdmin(user.Role),
			DepartmentIDs:       departmentIDs,
			PermissionLevelID:   user.PermissionLevelID,
			PermissionLevelRank: levelRanks[user.PermissionLevelID],
		}
		if u.CanVisible(effectiveCandidate, userAccess) && !u.CanVisible(parentPermissions, userAccess) {
			return fmt.Errorf("下级目录的查阅权限只能在上级已授权范围内收紧")
		}
	}
	visitor, err := u.BuildShareAccessContext(ctx, "")
	if err != nil {
		return err
	}
	if u.CanVisible(effectiveCandidate, visitor) && !u.CanVisible(parentPermissions, visitor) {
		return fmt.Errorf("下级目录的查阅权限只能在上级已授权范围内收紧")
	}
	return nil
}

func (u *EnterpriseUsecase) resolveParentReadPermissions(ctx context.Context, kbID, parentID, navID string) (domain.NodePermissions, error) {
	parentPermissions := domain.NodePermissions{}
	if parentID != "" && parentID != navID {
		lookupID := parentID
		if strings.HasPrefix(lookupID, "nav:") {
			lookupID = strings.TrimPrefix(lookupID, "nav:")
		}
		parentNode, err := u.nodeRepo.GetNodeByID(ctx, lookupID)
		if err == nil {
			if parentNode.KBID != kbID || (navID != "" && parentNode.NavId != navID) {
				return domain.NodePermissions{}, fmt.Errorf("上级目录不属于当前知识库栏目")
			}
			parentPermissions, err = u.ResolveNodePermissions(ctx, parentNode)
			if err != nil {
				return domain.NodePermissions{}, err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.NodePermissions{}, err
		}
	}
	if navID != "" {
		nav, err := u.navRepo.GetById(ctx, navID)
		if err != nil {
			return domain.NodePermissions{}, err
		}
		if nav.KbID != kbID {
			return domain.NodePermissions{}, fmt.Errorf("栏目不属于当前知识库")
		}
		parentPermissions = u.MergeNodeAndNavPermissions(parentPermissions, nav.Permissions)
	}
	return parentPermissions, nil
}

func (u *EnterpriseUsecase) departmentAncestorIDs(ctx context.Context) (map[string][]string, error) {
	departments, err := u.repo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	departmentIDsByPath := make(map[string]string, len(departments))
	for _, department := range departments {
		departmentIDsByPath[department.Path] = department.ID
	}
	ancestors := make(map[string][]string, len(departments))
	for _, department := range departments {
		pathParts := strings.Split(department.Path, "/")
		for index := range pathParts {
			if ancestorID, ok := departmentIDsByPath[strings.Join(pathParts[:index+1], "/")]; ok {
				ancestors[department.ID] = append(ancestors[department.ID], ancestorID)
			}
		}
	}
	return ancestors, nil
}

func expandAudienceDepartmentIDs(ids []string, ancestors map[string][]string) []string {
	var expanded []string
	for _, id := range ids {
		expanded = append(expanded, ancestors[id]...)
	}
	return expanded
}

func hasExplicitReadRule(permissions domain.NodePermissions) bool {
	return permissions.ScopeType != "" || permissions.AllowAllUsers || permissions.MinPermissionLevelID != "" || permissions.MinPermissionLevelRank > 0 ||
		len(permissions.AllowedPermissionLevelIDs) > 0 || len(permissions.AllowedDepartmentIDs) > 0 || len(permissions.AllowedUserIDs) > 0 ||
		len(permissions.DeniedDepartmentIDs) > 0 || len(permissions.DeniedUserIDs) > 0
}

func isUnrestrictedReadAudience(permissions domain.NodePermissions) bool {
	if permissions.AllowAllUsers {
		return true
	}
	if permissions.MinPermissionLevelID != "" || permissions.MinPermissionLevelRank > 0 ||
		permissions.ScopeType == "permission_level" || permissions.ScopeType == "department" || permissions.ScopeType == "user" {
		return false
	}
	return len(permissions.AllowedPermissionLevelIDs) == 0 &&
		len(permissions.AllowedDepartmentIDs) == 0 &&
		len(permissions.AllowedUserIDs) == 0
}

func isDirectlyUnrestrictedReadAudience(current, candidate domain.NodePermissions, parentID string, inherits bool) bool {
	if inherits || !isUnrestrictedReadAudience(current) || !isUnrestrictedReadAudience(candidate) {
		return false
	}
	currentOpenMode := current.ScopeType == "" || current.ScopeType == "public" || current.ScopeType == "custom"
	candidateOpenMode := candidate.ScopeType == "" || candidate.ScopeType == "public" || candidate.ScopeType == "custom"
	if !currentOpenMode || !candidateOpenMode || current.MinPermissionLevelID != candidate.MinPermissionLevelID || current.MinPermissionLevelRank != candidate.MinPermissionLevelRank {
		return false
	}
	if parentID != "" && current.Inherit == nil {
		return false
	}
	return stringSetContains(candidate.DeniedDepartmentIDs, current.DeniedDepartmentIDs) && stringSetContains(candidate.DeniedUserIDs, current.DeniedUserIDs)
}

func stringSetContains(values, required []string) bool {
	for _, item := range required {
		if !slices.Contains(values, item) {
			return false
		}
	}
	return true
}
