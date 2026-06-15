package pg

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type ContributeRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewContributeRepository(db *pg.DB, logger *log.Logger) *ContributeRepository {
	return &ContributeRepository{db: db, logger: logger.WithModule("repo.pg.contribute")}
}

func (r *ContributeRepository) Create(ctx context.Context, contribute *domain.Contribute) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	now := time.Now()
	contribute.Id = id.String()
	contribute.CreatedAt = now
	contribute.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(contribute).Error; err != nil {
		return "", err
	}
	return contribute.Id, nil
}

func (r *ContributeRepository) GetByID(ctx context.Context, kbID, id string) (*domain.Contribute, error) {
	var contribute domain.Contribute
	if err := r.db.WithContext(ctx).
		Model(&domain.Contribute{}).
		Where("id = ?", id).
		Where("kb_id = ?", kbID).
		First(&contribute).Error; err != nil {
		return nil, err
	}
	return &contribute, nil
}

func (r *ContributeRepository) GetList(ctx context.Context, req *domain.ContributeListReq) ([]*domain.ContributeItem, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Contribute{}).Where("contributes.kb_id = ?", req.KbID)
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.AuthName != "" {
		like := "%" + req.AuthName + "%"
		query = query.Where("name ILIKE ?", like)
	}
	if req.NodeName != "" {
		like := "%" + req.NodeName + "%"
		query = query.Joins("left join nodes on contributes.node_id = nodes.id").Where("nodes.name ILIKE ?", like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*domain.ContributeItem, 0)
	if err := query.
		Joins("left join nodes on contributes.node_id = nodes.id").
		Select("contributes.id, contributes.auth_id, contributes.user_id, contributes.kb_id, contributes.node_id, contributes.status, contributes.type, contributes.name as contribute_name, contributes.reason, contributes.meta, contributes.remote_ip, contributes.audit_user_id, contributes.created_at, contributes.updated_at, contributes.audit_time, nodes.name as node_name").
		Order("contributes.created_at DESC").
		Offset(req.Offset()).
		Limit(req.Limit()).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ContributeRepository) UpdateAuditResult(ctx context.Context, id, kbID, auditUserID, nodeID string, status consts.ContributeStatus) error {
	now := time.Now()
	updates := map[string]any{
		"status":        status,
		"audit_user_id": auditUserID,
		"audit_time":    &now,
		"updated_at":    now,
	}
	if nodeID != "" {
		updates["node_id"] = nodeID
	}
	return r.db.WithContext(ctx).
		Model(&domain.Contribute{}).
		Where("id = ?", id).
		Where("kb_id = ?", kbID).
		Updates(updates).Error
}
