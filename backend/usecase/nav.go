package usecase

import (
	"context"
	"errors"
	"fmt"

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
		return navs, nil
	}
	access, err := u.enterprise.GetUserAccessContext(ctx, authInfo.UserId)
	if err != nil {
		return navs, nil
	}
	filtered := make([]v1.NavListResp, 0, len(navs))
	for _, nav := range navs {
		if u.enterprise.CanVisible(nav.Permissions, access) {
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
	nav, err := u.navRepo.GetById(ctx, id)
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
	if err := u.navRepo.UpdatePermissions(ctx, req.KbId, req.ID, *req.Permissions); err != nil {
		return err
	}

	nodeIDs, err := u.nodeRepo.GetNodeIDsByNavId(ctx, req.KbId, req.ID)
	if err != nil {
		return fmt.Errorf("get node ids by nav id failed: %w", err)
	}
	if len(nodeIDs) == 0 {
		return nil
	}

	nodesMap, err := u.nodeRepo.GetNodesByIDs(ctx, nodeIDs)
	if err != nil {
		return fmt.Errorf("get nodes by ids failed: %w", err)
	}

	for _, nodeID := range nodeIDs {
		node, ok := nodesMap[nodeID]
		if !ok {
			continue
		}
		permissions := node.Permissions
		permissions.ScopeType = req.Permissions.ScopeType
		updateMap := map[string]interface{}{
			"permissions": permissions,
		}
		if err := u.nodeRepo.UpdateNodesByKbID(ctx, []string{nodeID}, req.KbId, updateMap); err != nil {
			return fmt.Errorf("sync node permission scope type failed: %w", err)
		}
	}

	return nil
}

func (u *NavUsecase) EnterpriseAccessContext(ctx context.Context, userID string) (*domain.EnterpriseAccessContext, error) {
	return u.enterprise.BuildShareAccessContext(ctx, userID)
}

func (u *NavUsecase) CanVisible(permissions domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	return u.enterprise.CanVisible(permissions, access)
}
