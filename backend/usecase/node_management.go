package usecase

import (
	"context"
	"fmt"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

func (u *NodeUsecase) managementAccess(ctx context.Context, kbID string) (*domain.DocumentManagementAccess, error) {
	return u.enterprise.ManagementAccess(ctx, kbID)
}

func (u *NodeUsecase) GetNodeManagement(ctx context.Context, kbID, nodeID string) (*v1.NodeManagementResp, error) {
	node, err := u.nodeRepo.GetByID(ctx, nodeID, kbID)
	if err != nil {
		return nil, err
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	canConfigure := access.Global || access.CanManageNode(nodeID)
	if !canConfigure {
		return nil, fmt.Errorf("没有此文档或目录的管理权限")
	}
	return &v1.NodeManagementResp{
		ID:                     nodeID,
		Grants:                 node.Permissions.ManagementGrants,
		ParentGrantConstraints: access.ManagementGrantConstraintsForParentNode(nodeID),
		HasParentFolder:        node.ParentID != "" && node.ParentID != node.NavId,
		CanConfigure:           canConfigure,
	}, nil
}

func (u *NodeUsecase) EditNodeManagement(ctx context.Context, req *v1.NodeManagementEditReq) error {
	access, err := u.managementAccess(ctx, req.KBID)
	if err != nil {
		return err
	}
	for _, id := range req.IDs {
		if !access.CanManageNode(id) {
			return fmt.Errorf("没有此文档或目录的管理权限")
		}
	}
	if err := u.enterprise.NormalizeManagementGrants(ctx, req.Grants); err != nil {
		return err
	}
	currentGroups := make([][]domain.DocumentManagementGrant, 0, len(req.IDs))
	for _, id := range req.IDs {
		if node := access.Nodes[id]; node != nil {
			currentGroups = append(currentGroups, node.Permissions.ManagementGrants)
		} else {
			currentGroups = append(currentGroups, nil)
		}
	}
	if err := validateManagementGrantChanges(currentGroups, req.Grants); err != nil {
		return err
	}
	if err := u.enterprise.ValidateManagementGrantNarrowing(ctx, access, req.IDs, req.Grants); err != nil {
		return err
	}
	if err := u.enterprise.ValidateManagementGrantTargets(ctx, access, req.Grants); err != nil {
		return err
	}
	return u.nodeRepo.UpdateNodeManagementGrants(ctx, req.KBID, req.IDs, req.Grants)
}

func (u *NodeUsecase) filterManagedNodes(ctx context.Context, kbID string, nodes []*domain.NodeListItemResp) ([]*domain.NodeListItemResp, error) {
	if domain.GetAuthInfoFromCtx(ctx) == nil {
		return nil, fmt.Errorf("未登录")
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	return u.filterManagedNodesWithAccess(ctx, access, nodes)
}

func (u *NodeUsecase) filterManagedNodesWithAccess(ctx context.Context, access *domain.DocumentManagementAccess, nodes []*domain.NodeListItemResp) ([]*domain.NodeListItemResp, error) {
	done := domain.MeasureTreeLoadPhase(ctx, "management_permissions")
	defer done()
	access = access.ForEvaluation()
	resolve := u.enterprise.newTreePermissionResolver(ctx, access.Nodes)
	var keep map[string]bool
	if !access.Global {
		keep = managementVisibleNodeIDs(access)
	}
	result := make([]*domain.NodeListItemResp, 0, len(nodes))
	for _, node := range nodes {
		if !access.Global && !keep[node.ID] {
			continue
		}
		node.CanManage = access.CanManageNode(node.ID)
		node.CanCreateDocument = node.Type == domain.NodeTypeFolder && access.CanCreate(node.ID, domain.NodeTypeDocument)
		node.CanCreateFolder = node.Type == domain.NodeTypeFolder && access.CanCreate(node.ID, domain.NodeTypeFolder)
		permissions, err := resolve(node.ID, node.ParentID, node.Permissions)
		if err != nil {
			return nil, err
		}
		if nav := access.Navs[node.NavId]; nav != nil {
			permissions = u.enterprise.MergeNodeAndNavPermissions(permissions, nav.Permissions)
		}
		if !access.Global {
			clearManagementGrantsFromPermissions(&permissions)
		}
		node.EffectivePermissions = &permissions
		node.EffectiveManagementGrants = access.ManagementGrantsForNode(node.ID)
		node.EffectiveManagementConstraints = access.ManagementGrantConstraintsForNode(node.ID)
		result = append(result, node)
	}
	if !access.Global {
		clearSensitiveNodeFieldsForNonManagers(result)
	}
	return result, nil
}

// managementVisibleNodeIDs includes managed nodes and only the ancestor
// folders needed to reach them in the admin tree.
func managementVisibleNodeIDs(access *domain.DocumentManagementAccess) map[string]bool {
	visible := map[string]bool{}
	if access == nil {
		return visible
	}
	for managedID, managedNode := range access.Nodes {
		if !access.CanManageNode(managedID) {
			continue
		}
		visible[managedID] = true
		seen := map[string]bool{}
		for current := managedNode; current != nil && current.ParentID != "" && !seen[current.ID]; {
			seen[current.ID] = true
			parent := access.Nodes[current.ParentID]
			if parent == nil {
				break
			}
			visible[parent.ID] = true
			current = parent
		}
	}
	return visible
}

func clearSensitiveNodeFieldsForNonManagers(nodes []*domain.NodeListItemResp) {
	for _, node := range nodes {
		if node.CanManage {
			continue
		}
		node.Permissions.ManagementGrants = nil
		node.EffectiveManagementGrants = nil
		node.EffectiveManagementConstraints = nil
		if node.EffectivePermissions != nil {
			clearManagementGrantsFromPermissions(node.EffectivePermissions)
		}
		node.FileKey = ""
		node.FileName = ""
		node.FileReadonly = false
	}
}

func clearManagementGrantsFromPermissions(permissions *domain.NodePermissions) {
	if permissions == nil {
		return
	}
	permissions.ManagementGrants = nil
	permissions.Constraints = append([]domain.NodePermissions(nil), permissions.Constraints...)
	for index := range permissions.Constraints {
		clearManagementGrantsFromPermissions(&permissions.Constraints[index])
	}
}

func (u *NodeUsecase) canManageNodeInAdmin(ctx context.Context, kbID, nodeID string) (bool, error) {
	if domain.GetAuthInfoFromCtx(ctx) == nil {
		return false, fmt.Errorf("未登录")
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return false, err
	}
	_, ok := access.Nodes[nodeID]
	if !ok {
		return false, fmt.Errorf("文档不存在")
	}
	return access.CanManageNode(nodeID), nil
}

// canInspectNodePermissionInAdmin allows a manager to read ACL metadata for
// ancestors needed to configure a managed descendant, without granting access
// to ancestor document content or admin details.
func (u *NodeUsecase) canInspectNodePermissionInAdmin(ctx context.Context, kbID, nodeID string) (bool, error) {
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return false, err
	}
	if access.Global || access.CanManageNode(nodeID) {
		return true, nil
	}
	for managedID, managedNode := range access.Nodes {
		if !access.CanManageNode(managedID) {
			continue
		}
		seen := map[string]bool{}
		for current := managedNode; current != nil && current.ParentID != "" && !seen[current.ID]; {
			seen[current.ID] = true
			if current.ParentID == nodeID {
				return true, nil
			}
			current = access.Nodes[current.ParentID]
		}
	}
	return false, nil
}

func (u *NodeUsecase) NodePermissionOptions(ctx context.Context, kbID string) (map[string]any, error) {
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return nil, fmt.Errorf("未登录")
	}
	users, err := u.enterprise.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return nil, err
	}
	departments, err := u.enterprise.repo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	departmentAncestors, err := u.enterprise.departmentAncestorIDs(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := u.enterprise.repo.ListPermissionLevels(ctx)
	if err != nil {
		return nil, err
	}
	departments = attachDepartmentAncestorIDs(activeDepartments(departments), departmentAncestors)
	managementUsers := managementAdministratorCandidates(users)
	return map[string]any{
		"users": users, "departments": departments, "levels": levels,
		"management_users": managementUsers, "management_departments": activeDepartments(departments),
		"can_delegate_management_by_level": false,
	}, nil
}

func managementAdministratorCandidates(users []domain.UserSummary) []domain.UserSummary {
	admins := make([]domain.UserSummary, 0, len(users))
	for _, user := range users {
		if consts.IsAdministrator(user.Role) {
			admins = append(admins, user)
		}
	}
	return admins
}

// attachDepartmentAncestorIDs gives the permission editor enough hierarchy
// context to evaluate inherited department rules for any department.
func attachDepartmentAncestorIDs(departments []domain.Department, ancestors map[string][]string) []domain.Department {
	result := make([]domain.Department, len(departments))
	for index, department := range departments {
		department.AncestorIDs = append([]string(nil), ancestors[department.ID]...)
		result[index] = department
	}
	return result
}

func activeDepartments(departments []domain.Department) []domain.Department {
	active := make([]domain.Department, 0, len(departments))
	for _, department := range departments {
		if department.Status == domain.DepartmentStatusActive {
			active = append(active, department)
		}
	}
	return active
}
