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
	return &v1.NodeManagementResp{ID: nodeID, Grants: node.Permissions.ManagementGrants, CanConfigure: access.Global}, nil
}

func (u *NodeUsecase) EditNodeManagement(ctx context.Context, req *v1.NodeManagementEditReq) error {
	access, err := u.managementAccess(ctx, req.KBID)
	if err != nil {
		return err
	}
	if !access.Global {
		return fmt.Errorf("只有知识库管理员可以分配文档管理权限")
	}
	if err := u.enterprise.NormalizeManagementGrants(ctx, req.Grants); err != nil {
		return err
	}
	return u.nodeRepo.UpdateNodeManagementGrants(ctx, req.KBID, req.IDs, req.Grants)
}

func (u *NodeUsecase) filterReadableManagedNodes(ctx context.Context, kbID string, nodes []*domain.NodeListItemResp) ([]*domain.NodeListItemResp, error) {
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil || auth.IsToken {
		for _, node := range nodes {
			node.CanManage = auth != nil && (auth.Permission == consts.UserKBPermissionDocManage || auth.Permission == consts.UserKBPermissionFullControl)
			node.CanCreateDocument = node.CanManage && node.Type == domain.NodeTypeFolder
			node.CanCreateFolder = node.CanCreateDocument
		}
		return nodes, nil
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if access.Global {
		for _, node := range nodes {
			node.CanManage = true
			node.CanCreateDocument = node.Type == domain.NodeTypeFolder
			node.CanCreateFolder = node.Type == domain.NodeTypeFolder
		}
		return nodes, nil
	}
	readable, err := u.enterprise.GetUserAccessContext(ctx, auth.UserId)
	if err != nil {
		return nil, err
	}
	navs, err := u.navRepo.GetList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	navMap := map[string]domain.NodePermissions{}
	for _, nav := range navs {
		navMap[nav.ID] = nav.Permissions
	}
	keep := map[string]bool{}
	for _, node := range nodes {
		node.CanManage = access.CanManageNode(node.ID)
		node.CanCreateDocument = access.CanCreate(node.ID, domain.NodeTypeDocument)
		node.CanCreateFolder = access.CanCreate(node.ID, domain.NodeTypeFolder)
		perm, err := u.enterprise.ResolveNodePermissions(ctx, &domain.Node{ID: node.ID, KBID: kbID, ParentID: node.ParentID, Permissions: node.Permissions})
		if err != nil {
			return nil, err
		}
		if perm.MinPermissionLevelID != "" {
			perm, err = u.enterprise.NormalizeNodePermissions(ctx, perm)
			if err != nil {
				return nil, err
			}
		}
		allowed := u.enterprise.CanVisible(perm, readable) && u.enterprise.CanVisible(navMap[node.NavId], readable)
		if node.CanManage || allowed {
			keep[node.ID] = true
			current := access.Nodes[node.ParentID]
			seen := map[string]bool{}
			for current != nil && !seen[current.ID] {
				keep[current.ID] = true
				seen[current.ID] = true
				current = access.Nodes[current.ParentID]
			}
		}
	}
	result := make([]*domain.NodeListItemResp, 0, len(nodes))
	for _, node := range nodes {
		if keep[node.ID] {
			result = append(result, node)
		}
	}
	return result, nil
}

func (u *NodeUsecase) canReadOrManageNode(ctx context.Context, kbID, nodeID string) (bool, error) {
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return false, fmt.Errorf("未登录")
	}
	access, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return false, err
	}
	if auth.IsToken {
		return true, nil
	}
	node, ok := access.Nodes[nodeID]
	if !ok {
		return false, fmt.Errorf("文档不存在")
	}
	if access.CanManageNode(nodeID) {
		return true, nil
	}
	readable, err := u.enterprise.GetUserAccessContext(ctx, auth.UserId)
	if err != nil {
		return false, err
	}
	perm, err := u.enterprise.ResolveNodePermissions(ctx, node)
	if err != nil {
		return false, err
	}
	if perm.MinPermissionLevelID != "" {
		perm, err = u.enterprise.NormalizeNodePermissions(ctx, perm)
		if err != nil {
			return false, err
		}
	}
	if !u.enterprise.CanVisitable(perm, readable) {
		return false, nil
	}
	nav, err := u.navRepo.GetById(ctx, node.NavId)
	if err != nil {
		return false, err
	}
	return u.enterprise.CanVisitable(nav.Permissions, readable), nil
}

func (u *NodeUsecase) NodePermissionOptions(ctx context.Context) (map[string]any, error) {
	users, err := u.enterprise.repo.PermissionAudienceUsers(ctx)
	if err != nil {
		return nil, err
	}
	departments, err := u.enterprise.repo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := u.enterprise.repo.ListPermissionLevels(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"users": users, "departments": departments, "levels": levels}, nil
}
