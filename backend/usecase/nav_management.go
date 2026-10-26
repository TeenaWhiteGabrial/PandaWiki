package usecase

import (
	"context"
	"fmt"

	navV1 "github.com/chaitin/panda-wiki/api/nav/v1"
	nodeV1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/domain"
)

func (u *NavUsecase) GetManagement(ctx context.Context, kbID, id string) (*nodeV1.NodeManagementResp, error) {
	nav, err := u.navRepo.GetByKBID(ctx, kbID, id)
	if err != nil {
		return nil, err
	}
	access, err := u.enterprise.ManagementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	canConfigure := access.Global || access.CanManageNav(id)
	if !canConfigure {
		return nil, fmt.Errorf("没有此目录的管理权限")
	}
	return &nodeV1.NodeManagementResp{ID: id, Grants: nav.Permissions.ManagementGrants, CanConfigure: canConfigure}, nil
}

func (u *NavUsecase) EditManagement(ctx context.Context, req *navV1.NavManagementEditReq) error {
	access, err := u.enterprise.ManagementAccess(ctx, req.KBID)
	if err != nil {
		return err
	}
	if !access.CanManageNav(req.ID) {
		return fmt.Errorf("没有此目录的管理权限")
	}
	if err := u.enterprise.NormalizeManagementGrants(ctx, req.Grants); err != nil {
		return err
	}
	current := access.Navs[req.ID]
	var currentGroups [][]domain.DocumentManagementGrant
	if current != nil {
		currentGroups = append(currentGroups, current.Permissions.ManagementGrants)
	}
	if err := validateManagementGrantChanges(currentGroups, req.Grants); err != nil {
		return err
	}
	if err := u.enterprise.ValidateManagementGrantTargets(ctx, access, req.Grants); err != nil {
		return err
	}
	return u.navRepo.UpdateManagementGrants(ctx, req.KBID, req.ID, req.Grants)
}
