package usecase

import (
	"context"
	"fmt"

	navV1 "github.com/chaitin/panda-wiki/api/nav/v1"
	nodeV1 "github.com/chaitin/panda-wiki/api/node/v1"
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
	return &nodeV1.NodeManagementResp{ID: id, Grants: nav.Permissions.ManagementGrants, CanConfigure: access.Global}, nil
}

func (u *NavUsecase) EditManagement(ctx context.Context, req *navV1.NavManagementEditReq) error {
	access, err := u.enterprise.ManagementAccess(ctx, req.KBID)
	if err != nil {
		return err
	}
	if !access.Global {
		return fmt.Errorf("只有知识库管理员可以分配文档管理权限")
	}
	if err := u.enterprise.NormalizeManagementGrants(ctx, req.Grants); err != nil {
		return err
	}
	return u.navRepo.UpdateManagementGrants(ctx, req.KBID, req.ID, req.Grants)
}
