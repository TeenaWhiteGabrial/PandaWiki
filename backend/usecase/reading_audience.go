package usecase

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

// prepareReadingPermissions is shared by saving and previewing. Client-provided
// effective constraints/snapshots never replace the server's parent policy.
func (u *EnterpriseUsecase) prepareReadingPermissions(ctx context.Context, current, draft domain.NodePermissions, node bool) (domain.NodePermissions, error) {
	draft.Constraints = nil
	draft.ParentPermissionSnapshot = nil
	draft.ManagementGrants = current.ManagementGrants
	if node && draft.Inherit != nil && *draft.Inherit {
		inherit := true
		return domain.NodePermissions{Inherit: &inherit, ManagementGrants: current.ManagementGrants}, nil
	}
	if draft.DeniedDepartmentIDs == nil {
		draft.DeniedDepartmentIDs = current.DeniedDepartmentIDs
	}
	if draft.DeniedUserIDs == nil {
		draft.DeniedUserIDs = current.DeniedUserIDs
	}
	if config.PermissionLevelsDisabled() {
		draft.MinPermissionLevelID = current.MinPermissionLevelID
		draft.MinPermissionLevelRank = current.MinPermissionLevelRank
		draft.AllowedPermissionLevelIDs = current.AllowedPermissionLevelIDs
		return draft, nil
	}
	return u.NormalizeNodePermissions(ctx, draft)
}

func (u *NodeUsecase) ReadingAudience(ctx context.Context, req *v1.ReadingAudienceReq) (*v1.ReadingAudienceResp, error) {
	access, err := u.managementAccess(ctx, req.KBID)
	if err != nil {
		return nil, err
	}
	// Prepare the complete batch before resolving ancestors, so a parent and
	// child edited together are previewed against the same proposed hierarchy.
	drafts := map[string]domain.NodePermissions{}
	if req.TargetType == "node" && req.Permissions != nil {
		for _, id := range req.IDs {
			if !access.CanManageNode(id) {
				return nil, fmt.Errorf("没有此文档或目录的管理权限")
			}
			node := access.Nodes[id]
			local, err := u.enterprise.prepareReadingPermissions(ctx, node.Permissions, *req.Permissions, true)
			if err != nil {
				return nil, err
			}
			if local.Inherit == nil || !*local.Inherit {
				local.ParentPermissionSnapshot, err = u.enterprise.ParentPermissionSnapshot(ctx, req.KBID, node.ParentID, node.NavId)
				if err != nil {
					return nil, err
				}
			}
			drafts[id] = local
		}
	}
	var policies []domain.NodePermissions
	var source *v1.ReadingAudienceSource
	for _, id := range req.IDs {
		var policy domain.NodePermissions
		if req.TargetType == "nav" {
			if !access.CanManageNav(id) {
				return nil, fmt.Errorf("没有此栏目的管理权限")
			}
			nav, err := u.navRepo.GetByKBID(ctx, req.KBID, id)
			if err != nil {
				return nil, err
			}
			policy = nav.Permissions
			if req.Permissions != nil {
				policy, err = u.enterprise.prepareReadingPermissions(ctx, nav.Permissions, *req.Permissions, false)
				if err != nil {
					return nil, err
				}
			}
			policy, err = u.enterprise.normalizeResolvedNodePermissions(ctx, policy)
			if err != nil {
				return nil, err
			}
		} else {
			if !access.CanManageNode(id) {
				return nil, fmt.Errorf("没有此文档或目录的管理权限")
			}
			node := access.Nodes[id]
			local := node.Permissions
			if draft, exists := drafts[id]; exists {
				local = draft
			}
			if req.Permissions != nil {
				policy, err = u.enterprise.resolveDraftReadingPolicy(ctx, access.Nodes, node, local, drafts)
			} else {
				policy, err = u.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, local)
			}
			if err != nil {
				return nil, err
			}
			if node.NavId != "" {
				nav, err := u.navRepo.GetByKBID(ctx, req.KBID, node.NavId)
				if err != nil {
					return nil, err
				}
				policy = u.enterprise.MergeNodeAndNavPermissions(policy, nav.Permissions)
			}
			if len(req.IDs) == 1 && ((local.Inherit != nil && *local.Inherit) || shouldInheritNodePermissions(local, node.ParentID) || !hasExplicitReadRule(local)) {
				source, err = u.inheritedReadingSource(ctx, access, node)
				if err != nil {
					return nil, err
				}
			}
		}
		policies = append(policies, policy)
	}
	// Batch results are the users who can read ALL selected targets, rather than
	// a union that could imply permission to a target they cannot actually read.
	combined := domain.NodePermissions{Constraints: policies}
	rules := snapshotReadPermissionRules(combined)
	combined = rules[0]
	combined.Constraints = rules[1:]
	users, err := u.enterprise.repo.ReadingAudienceUsers(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := u.enterprise.repo.ListPermissionLevels(ctx)
	if err != nil {
		return nil, err
	}
	ranks := make(map[string]int, len(levels))
	for _, level := range levels {
		ranks[level.ID] = level.Rank
	}
	result := u.enterprise.filterReadingAudience(combined, users, ranks, req.Keyword, req.Page, req.PageSize, req.IncludeUsers)
	result.Permissions = combined
	result.Summary = u.enterprise.summarizeReadingPermissions(combined, users, ranks)
	result.Unrestricted = unrestrictedReadingPolicy(combined)
	result.Source = source
	return result, nil
}

func (u *EnterpriseUsecase) resolveDraftReadingPolicy(ctx context.Context, nodes map[string]*domain.Node, node *domain.Node, local domain.NodePermissions, drafts map[string]domain.NodePermissions) (domain.NodePermissions, error) {
	entries := []nodePermissionHierarchyEntry{{permissions: local, parentID: node.ParentID}}
	seen := map[string]bool{node.ID: true}
	parentID := node.ParentID
	for depth := 0; depth < 20 && parentID != "" && !seen[parentID]; depth++ {
		seen[parentID] = true
		parent := nodes[parentID]
		if parent == nil || parent.KBID != node.KBID || parent.NavId != node.NavId {
			break
		}
		permissions := parent.Permissions
		if draft, exists := drafts[parentID]; exists {
			permissions = draft
		}
		entries = append(entries, nodePermissionHierarchyEntry{permissions: permissions, parentID: parent.ParentID})
		parentID = parent.ParentID
	}
	return u.normalizeResolvedNodePermissions(ctx, composeNodePermissionHierarchy(entries))
}

func (u *NodeUsecase) inheritedReadingSource(ctx context.Context, access *domain.DocumentManagementAccess, node *domain.Node) (*v1.ReadingAudienceSource, error) {
	seen := map[string]bool{node.ID: true}
	for parentID := node.ParentID; parentID != "" && parentID != node.NavId && !seen[parentID]; {
		seen[parentID] = true
		parent := access.Nodes[parentID]
		if parent == nil || parent.KBID != node.KBID || parent.NavId != node.NavId {
			break
		}
		if !shouldInheritNodePermissions(parent.Permissions, parent.ParentID) && hasExplicitReadRule(parent.Permissions) {
			return &v1.ReadingAudienceSource{ID: parent.ID, Name: parent.Name, NavID: parent.NavId, TargetType: "node"}, nil
		}
		parentID = parent.ParentID
	}
	if node.NavId == "" {
		return nil, nil
	}
	nav, err := u.navRepo.GetByKBID(ctx, node.KBID, node.NavId)
	if err != nil {
		return nil, err
	}
	return &v1.ReadingAudienceSource{ID: nav.ID, Name: nav.Name, NavID: nav.ID, TargetType: "nav"}, nil
}

func (u *EnterpriseUsecase) filterReadingAudience(policy domain.NodePermissions, users []domain.UserSummary, ranks map[string]int, keyword string, page, pageSize int, include bool) *v1.ReadingAudienceResp {
	result := &v1.ReadingAudienceResp{Users: []domain.UserSummary{}}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	for _, user := range users {
		principal := &domain.EnterpriseAccessContext{PrincipalID: user.ID, DepartmentIDs: user.DepartmentIDs, PermissionLevelID: user.PermissionLevelID, PermissionLevelRank: ranks[user.PermissionLevelID]}
		if !u.CanVisitable(policy, principal) {
			continue
		}
		result.Total++
		if keyword != "" && !strings.Contains(strings.ToLower(user.Account), keyword) && !strings.Contains(strings.ToLower(user.Name), keyword) {
			continue
		}
		result.FilteredTotal++
		if include && result.FilteredTotal > (page-1)*pageSize && len(result.Users) < pageSize {
			result.Users = append(result.Users, user)
		}
	}
	return result
}

func unrestrictedReadingPolicy(policy domain.NodePermissions) bool {
	for _, constraint := range policy.Constraints {
		if !unrestrictedReadingPolicy(constraint) {
			return false
		}
	}
	if len(policy.DeniedUserIDs) > 0 || len(policy.DeniedDepartmentIDs) > 0 {
		return false
	}
	if policy.AllowAllUsers {
		return true
	}
	if config.PermissionLevelsDisabled() && policy.ScopeType == "permission_level" {
		return true
	}
	if policy.ScopeType == "user" || policy.ScopeType == "department" || policy.ScopeType == "permission_level" || len(policy.AllowedUserIDs) > 0 || len(policy.AllowedDepartmentIDs) > 0 {
		return false
	}
	return config.PermissionLevelsDisabled() || (len(policy.AllowedPermissionLevelIDs) == 0 && policy.MinPermissionLevelID == "" && policy.MinPermissionLevelRank == 0)
}
