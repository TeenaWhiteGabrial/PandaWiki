package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type ContributeUsecase struct {
	repo     *pg.ContributeRepository
	nodeRepo *pg.NodeRepository
	userRepo *pg.UserRepository
	appRepo  *pg.AppRepository
	kbRepo   *pg.KnowledgeBaseRepository
	ragRepo  *mq.RAGRepository
	logger   *log.Logger
}

func NewContributeUsecase(
	repo *pg.ContributeRepository,
	nodeRepo *pg.NodeRepository,
	userRepo *pg.UserRepository,
	appRepo *pg.AppRepository,
	kbRepo *pg.KnowledgeBaseRepository,
	ragRepo *mq.RAGRepository,
	logger *log.Logger,
) *ContributeUsecase {
	return &ContributeUsecase{
		repo:     repo,
		nodeRepo: nodeRepo,
		userRepo: userRepo,
		appRepo:  appRepo,
		kbRepo:   kbRepo,
		ragRepo:  ragRepo,
		logger:   logger.WithModule("usecase.contribute"),
	}
}

func (u *ContributeUsecase) Submit(ctx context.Context, req *domain.SubmitContributeReq, kbID, userID, userName, remoteIP string) (string, error) {
	app, err := u.appRepo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
	if err != nil {
		return "", err
	}
	if app == nil || !app.Settings.ContributeSettings.IsEnable {
		return "", errors.New("contribute is disabled")
	}

	contribute := &domain.Contribute{
		KBId:     kbID,
		UserID:   userID,
		Status:   consts.ContributeStatusPending,
		Type:     req.Type,
		NodeId:   req.NodeID,
		Name:     req.Name,
		Content:  req.Content,
		Reason:   req.Reason,
		RemoteIP: remoteIP,
		Meta: domain.NodeMeta{
			Emoji:       req.Emoji,
			ContentType: req.ContentType,
		},
	}

	if req.Type == consts.ContributeTypeEdit {
		if req.NodeID == "" {
			return "", errors.New("node_id is required for edit contribute")
		}
		node, err := u.nodeRepo.GetByID(ctx, req.NodeID, kbID)
		if err != nil {
			return "", err
		}
		if contribute.Name == "" {
			contribute.Name = node.Name
		}
		if contribute.Content == "" {
			return "", errors.New("content is required")
		}
	} else {
		if contribute.Name == "" {
			return "", errors.New("name is required")
		}
		if contribute.Content == "" {
			return "", errors.New("content is required")
		}
	}

	if userName != "" && contribute.Name == "" {
		contribute.Name = userName
	}

	return u.repo.Create(ctx, contribute)
}

func (u *ContributeUsecase) GetList(ctx context.Context, req *domain.ContributeListReq) (*domain.ContributeListResp, error) {
	items, total, err := u.repo.GetList(ctx, req)
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.UserID != "" {
			userIDs = append(userIDs, item.UserID)
		}
	}

	users, err := u.userRepo.GetUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if user, ok := users[item.UserID]; ok {
			if user.Name != "" {
				item.AuthName = user.Name
			} else {
				item.AuthName = user.Account
			}
		} else {
			item.AuthName = "访客"
		}
		if item.RemoteIP != "" {
			item.IPAddress = &domain.IPAddress{IP: item.RemoteIP}
		}
		item.Meta = domain.ContributeNodeMeta{
			ContentType: item.Meta.ContentType,
			Emoji:       item.Meta.Emoji,
		}
	}

	return &domain.ContributeListResp{
		List:  items,
		Total: total,
	}, nil
}

func (u *ContributeUsecase) GetDetail(ctx context.Context, kbID, id string) (*domain.ContributeDetailResp, error) {
	contribute, err := u.repo.GetByID(ctx, kbID, id)
	if err != nil {
		return nil, err
	}

	resp := &domain.ContributeDetailResp{
		ID:          contribute.Id,
		AuthID:      contribute.AuthId,
		UserID:      contribute.UserID,
		KBId:        contribute.KBId,
		NodeID:      contribute.NodeId,
		Status:      contribute.Status,
		Type:        contribute.Type,
		Name:        contribute.Name,
		Content:     contribute.Content,
		Reason:      contribute.Reason,
		AuditUserID: contribute.AuditUserID,
		CreatedAt:   contribute.CreatedAt,
		UpdatedAt:   contribute.UpdatedAt,
		AuditTime:   contribute.AuditTime,
		Meta: domain.ContributeNodeMeta{
			ContentType: contribute.Meta.ContentType,
			Emoji:       contribute.Meta.Emoji,
		},
	}

	if contribute.UserID != "" {
		users, err := u.userRepo.GetUsersByIDs(ctx, []string{contribute.UserID})
		if err != nil {
			return nil, err
		}
		if user, ok := users[contribute.UserID]; ok {
			if user.Name != "" {
				resp.AuthName = user.Name
			} else {
				resp.AuthName = user.Account
			}
		}
	}
	if resp.AuthName == "" {
		resp.AuthName = "访客"
	}

	if contribute.NodeId != "" {
		node, err := u.nodeRepo.GetByID(ctx, contribute.NodeId, kbID)
		if err == nil {
			resp.NodeName = node.Name
			resp.OriginalNode = &domain.OriginalContributeNodeInfo{
				ID:      node.ID,
				Name:    node.Name,
				Content: node.Content,
				Meta: domain.ContributeNodeMeta{
					ContentType: node.Meta.ContentType,
					Emoji:       node.Meta.Emoji,
				},
			}
		}
	}
	return resp, nil
}

func (u *ContributeUsecase) Audit(ctx context.Context, req *domain.ContributeAuditReq, auditUserID string) error {
	contribute, err := u.repo.GetByID(ctx, req.KbID, req.ID)
	if err != nil {
		return err
	}
	if contribute.Status != consts.ContributeStatusPending {
		return fmt.Errorf("contribute already audited")
	}

	if req.Status == consts.ContributeStatusRejected {
		return u.repo.UpdateAuditResult(ctx, req.ID, req.KbID, auditUserID, contribute.NodeId, req.Status)
	}

	if contribute.Type == consts.ContributeTypeAdd {
		createReq := &domain.CreateNodeReq{
			KBID:        req.KbID,
			NavId:       req.NavID,
			ParentID:    derefString(req.ParentID),
			Type:        domain.NodeTypeDocument,
			Name:        contribute.Name,
			Content:     contribute.Content,
			Emoji:       contribute.Meta.Emoji,
			ContentType: stringPtr(defaultString(contribute.Meta.ContentType, "md")),
			Position:    req.Position,
		}
		nodeID, err := u.nodeRepo.Create(ctx, createReq, auditUserID)
		if err != nil {
			return err
		}
		contribute.NodeId = nodeID
	} else {
		updateReq := &domain.UpdateNodeReq{
			ID:          contribute.NodeId,
			KBID:        req.KbID,
			Name:        stringPtr(contribute.Name),
			Content:     stringPtr(contribute.Content),
			Emoji:       stringPtr(contribute.Meta.Emoji),
			ContentType: stringPtr(defaultString(contribute.Meta.ContentType, "md")),
			NavId:       &req.NavID,
		}
		if req.Position != nil {
			updateReq.Position = req.Position
		}
		if err := u.nodeRepo.UpdateNodeContent(ctx, updateReq, auditUserID); err != nil {
			return err
		}
	}

	releaseIDs, err := u.nodeRepo.CreateNodeReleases(ctx, req.KbID, auditUserID, []string{contribute.NodeId})
	if err != nil {
		return err
	}
	if len(releaseIDs) > 0 {
		if err := u.publishAndRestudy(ctx, req.KbID, auditUserID, []string{contribute.NodeId}, releaseIDs); err != nil {
			return err
		}
	}

	if err := u.repo.UpdateAuditResult(ctx, req.ID, req.KbID, auditUserID, contribute.NodeId, req.Status); err != nil {
		return err
	}
	return nil
}

func (u *ContributeUsecase) publishAndRestudy(ctx context.Context, kbID, userID string, nodeIDs, releaseIDs []string) error {
	release := &domain.KBRelease{
		KBID:        kbID,
		Tag:         "文档贡献自动发布",
		Message:     "系统自动发布已审核通过的文档贡献",
		PublisherId: userID,
	}
	if err := u.kbRepo.CreateKBRelease(ctx, release); err != nil {
		return err
	}

	nodeReleases, err := u.nodeRepo.GetLatestNodeReleaseByNodeIDs(ctx, kbID, nodeIDs)
	if err != nil {
		return err
	}
	if len(nodeReleases) == 0 {
		return nil
	}

	for _, nodeRelease := range nodeReleases {
		node, getNodeErr := u.nodeRepo.GetNodeByID(ctx, nodeRelease.NodeID)
		if getNodeErr != nil {
			return getNodeErr
		}

		ragStatus := consts.NodeRagStatusRunning
		if node.RagInfo.Status == consts.NodeRagStatusSucceeded ||
			node.RagInfo.Status == consts.NodeRagStatusFailed ||
			node.RagInfo.Status == consts.NodeRagStatusReindexing ||
			node.RagInfo.Status == consts.NodeRagStatusRunning {
			ragStatus = consts.NodeRagStatusReindexing
		}

		if err := u.nodeRepo.UpdateNodeRagInfo(ctx, nodeRelease.KBID, nodeRelease.NodeID, domain.RagInfo{
			Status:   ragStatus,
			Message:  "",
			SyncedAt: time.Now(),
		}); err != nil {
			return err
		}

		if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, []*domain.NodeReleaseVectorRequest{
			{
				KBID:          nodeRelease.KBID,
				NodeReleaseID: nodeRelease.ID,
				Action:        "upsert",
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func stringPtr(v string) *string {
	return &v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
