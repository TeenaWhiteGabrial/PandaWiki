package usecase

import (
	"context"
	"errors"
	"github.com/chaitin/panda-wiki/config"

	"github.com/google/uuid"

	v1 "github.com/chaitin/panda-wiki/api/nav/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type NavUsecase struct {
	navRepo    *pg.NavRepository
	nodeRepo   *pg.NodeRepository
	ragRepo    *mq.RAGRepository
	enterprise *EnterpriseUsecase
	logger     *log.Logger
}

func NewNavUsecase(
	navRepo *pg.NavRepository,
	nodeRepo *pg.NodeRepository,
	ragRepo *mq.RAGRepository,
	enterprise *EnterpriseUsecase,
	logger *log.Logger,
) *NavUsecase {
	return &NavUsecase{
		navRepo:    navRepo,
		nodeRepo:   nodeRepo,
		ragRepo:    ragRepo,
		enterprise: enterprise,
		logger:     logger.WithModule("usecase.nav"),
	}
}

func (u *NavUsecase) GetList(ctx context.Context, kbID string) ([]v1.NavListResp, error) {
	navs, err := u.navRepo.GetList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil || authInfo.IsToken {
		for i := range navs {
			navs[i].CanManage = authInfo != nil && (authInfo.Permission == consts.UserKBPermissionDocManage || authInfo.Permission == consts.UserKBPermissionFullControl)
			navs[i].CanCreateDocument = navs[i].CanManage
			navs[i].CanCreateFolder = navs[i].CanManage
			navs[i].CanConfigure = navs[i].CanManage
		}
		return navs, nil
	}
	management, err := u.enterprise.ManagementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if management.Global {
		for i := range navs {
			navs[i].CanManage = true
			navs[i].CanCreateDocument = true
			navs[i].CanCreateFolder = true
			navs[i].CanConfigure = true
		}
		return navs, nil
	}
	access, err := u.enterprise.GetUserAccessContext(ctx, authInfo.UserId)
	if err != nil {
		return nil, err
	}
	filtered := make([]v1.NavListResp, 0, len(navs))
	for _, nav := range navs {
		nav.CanManage = management.CanManageNav(nav.ID)
		nav.CanCreateDocument = management.CanCreateInNav("", nav.ID, domain.NodeTypeDocument)
		nav.CanCreateFolder = management.CanCreateInNav("", nav.ID, domain.NodeTypeFolder)
		manageable := nav.CanManage
		for id, node := range management.Nodes {
			if node.NavId == nav.ID && management.CanManageNode(id) {
				manageable = true
				break
			}
		}
		if manageable || u.enterprise.CanVisible(nav.Permissions, access) {
			filtered = append(filtered, nav)
		}
	}
	return filtered, nil
}

func (u *NavUsecase) GetReleaseList(ctx context.Context, kbID string) ([]v1.NavListResp, error) {
	navs, err := u.navRepo.GetReleaseList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	return navs, nil
}

func (u *NavUsecase) GetPermission(ctx context.Context, kbID, id string) (*v1.NavPermissionResp, error) {
	nav, err := u.navRepo.GetByKBID(ctx, kbID, id)
	if err != nil {
		return nil, err
	}
	return &v1.NavPermissionResp{
		ID:          nav.ID,
		Permissions: nav.Permissions,
	}, nil
}

func (u *NavUsecase) Add(ctx context.Context, req *v1.NavAddReq) error {
	if req.Position != nil && (*req.Position > domain.MaxPosition || *req.Position < 0) {
		return errors.New("specified position is out of range")
	}

	nav := &domain.Nav{
		ID:   uuid.New().String(),
		KbID: req.KbId,
		Name: req.Name,
		Permissions: domain.NodePermissions{
			Answerable: consts.NodeAccessPermOpen,
			Visitable:  consts.NodeAccessPermOpen,
			Visible:    consts.NodeAccessPermOpen,
		},
	}

	return u.navRepo.Create(ctx, nav, req.Position)
}

func (u *NavUsecase) Move(ctx context.Context, req *v1.NavMoveReq) error {
	return u.navRepo.Move(ctx, req.KbId, req.ID, req.PrevID, req.NextID)
}

func (u *NavUsecase) Delete(ctx context.Context, req *v1.NavDeleteReq) error {
	nodeIDs, err := u.nodeRepo.GetNodeIDsByNavId(ctx, req.KbId, req.ID)
	if err != nil {
		return err
	}

	if len(nodeIDs) > 0 {
		docIDs, err := u.nodeRepo.Delete(ctx, req.KbId, nodeIDs)
		if err != nil {
			return err
		}
		nodeVectorContentRequests := make([]*domain.NodeReleaseVectorRequest, 0)
		for _, docID := range docIDs {
			nodeVectorContentRequests = append(nodeVectorContentRequests, &domain.NodeReleaseVectorRequest{
				KBID:   req.KbId,
				DocID:  docID,
				Action: "delete",
			})
		}
		if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, nodeVectorContentRequests); err != nil {
			return err
		}
	}

	return u.navRepo.Delete(ctx, req.KbId, req.ID)
}

func (u *NavUsecase) Update(ctx context.Context, req *v1.NavUpdateReq) error {
	return u.navRepo.Update(ctx, req.KbId, req.ID, req.Name)
}

func (u *NavUsecase) UpdatePermission(ctx context.Context, req *v1.NavPermissionEditReq) error {
	if req.Permissions == nil {
		return nil
	}
	nav, err := u.navRepo.GetByKBID(ctx, req.KbId, req.ID)
	if err != nil {
		return err
	}
	req.Permissions.ManagementGrants = nav.Permissions.ManagementGrants
	if config.PermissionLevelsDisabled() {
		req.Permissions.AllowedPermissionLevelIDs = nav.Permissions.AllowedPermissionLevelIDs
		req.Permissions.MinPermissionLevelID = nav.Permissions.MinPermissionLevelID
		req.Permissions.MinPermissionLevelRank = nav.Permissions.MinPermissionLevelRank
	}
	if !config.PermissionLevelsDisabled() {
		permissions, err := u.enterprise.NormalizeNodePermissions(ctx, *req.Permissions)
		if err != nil {
			return err
		}
		req.Permissions = &permissions
	}
	if err := u.navRepo.UpdatePermissions(ctx, req.KbId, req.ID, *req.Permissions); err != nil {
		return err
	}

	return nil
}

func (u *NavUsecase) EnterpriseAccessContext(ctx context.Context, userID string) (*domain.EnterpriseAccessContext, error) {
	return u.enterprise.BuildShareAccessContext(ctx, userID)
}

func (u *NavUsecase) CanVisible(permissions domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	return u.enterprise.CanVisible(permissions, access)
}
