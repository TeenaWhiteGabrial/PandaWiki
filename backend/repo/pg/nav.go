package pg

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	v1 "github.com/chaitin/panda-wiki/api/nav/v1"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type NavRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewNavRepository(db *pg.DB, logger *log.Logger) *NavRepository {
	return &NavRepository{db: db, logger: logger.WithModule("repo.pg.nav")}
}

func (r *NavRepository) GetById(ctx context.Context, id string) (*domain.Nav, error) {
	var nav domain.Nav
	if err := r.db.WithContext(ctx).Model(&domain.Nav{}).Where("id = ?", id).First(&nav).Error; err != nil {
		return nil, err
	}

	return &nav, nil
}

func (r *NavRepository) GetList(ctx context.Context, kbId string) ([]v1.NavListResp, error) {
	navs := make([]v1.NavListResp, 0)
	query := r.db.WithContext(ctx).
		Model(&domain.Nav{}).
		Where("kb_id = ?", kbId).
		Order("position ASC")

	if err := query.Find(&navs).Error; err != nil {
		return nil, err
	}
	return navs, nil
}

func (r *NavRepository) GetListByIds(ctx context.Context, kbId string, ids []string) ([]v1.NavListResp, error) {
	navs := make([]v1.NavListResp, 0)
	query := r.db.WithContext(ctx).
		Model(&domain.Nav{}).
		Where("kb_id = ?", kbId).
		Order("position ASC")

	if len(ids) > 0 {
		query = query.Where("id IN (?)", ids)
	}

	if err := query.Find(&navs).Error; err != nil {
		return nil, err
	}
	return navs, nil
}

func (r *NavRepository) getMaxPosByKbId(tx *gorm.DB, kbId string) (float64, error) {
	var maxPos float64
	if err := tx.Model(&domain.Nav{}).
		Select("COALESCE(MAX(position::float), 0)").
		Where("kb_id = ?", kbId).
		Scan(&maxPos).Error; err != nil {
		return 0, err
	}
	return maxPos, nil
}

func (r *NavRepository) Create(ctx context.Context, nav *domain.Nav, position *float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if position != nil {
			nav.Position = *position
		} else {
			maxPos, err := r.getMaxPosByKbId(tx, nav.KbID)
			if err != nil {
				return err
			}
			newPos := maxPos + (domain.MaxPosition-maxPos)/2.0
			if newPos-maxPos < domain.MinPositionGap {
				if err := r.reorderPositionsTx(tx, nav.KbID); err != nil {
					return err
				}
				maxPos, err = r.getMaxPosByKbId(tx, nav.KbID)
				if err != nil {
					return err
				}
				newPos = maxPos + (domain.MaxPosition-maxPos)/2.0
			}
			nav.Position = newPos
		}
		return tx.Create(nav).Error
	})
}

func (r *NavRepository) reorderPositionsTx(tx *gorm.DB, kbId string) error {
	var navs []*domain.Nav
	if err := tx.Model(&domain.Nav{}).
		Where("kb_id = ?", kbId).
		Order("position").
		Find(&navs).Error; err != nil {
		return err
	}
	if len(navs) == 0 {
		return nil
	}
	basePosition := int64(1000)
	interval := int64(1000)
	for i, nav := range navs {
		nav.Position = float64(basePosition + int64(i)*interval)
	}
	return tx.Select("position").Save(navs).Error
}

func (r *NavRepository) Move(ctx context.Context, kbId, id, prevID, nextID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prevPos float64
		var maxPos = domain.MaxPosition
		if prevID != "" {
			var prev domain.Nav
			if err := tx.Where("id = ? AND kb_id = ?", prevID, kbId).
				Select("position").First(&prev).Error; err != nil {
				return err
			}
			prevPos = prev.Position
		}
		if nextID != "" {
			var next domain.Nav
			if err := tx.Where("id = ? AND kb_id = ?", nextID, kbId).
				Select("position").First(&next).Error; err != nil {
				return err
			}
			maxPos = next.Position
		}

		newPos := prevPos + (maxPos-prevPos)/2.0
		if newPos-prevPos < domain.MinPositionGap {
			if err := r.reorderPositionsTx(tx, kbId); err != nil {
				return err
			}
			// recalculate after reorder
			if prevID != "" {
				var prev domain.Nav
				if err := tx.Where("id = ? AND kb_id = ?", prevID, kbId).Select("position").First(&prev).Error; err != nil {
					return err
				}
				prevPos = prev.Position
			}
			if nextID != "" {
				var next domain.Nav
				if err := tx.Where("id = ? AND kb_id = ?", nextID, kbId).Select("position").First(&next).Error; err != nil {
					return err
				}
				maxPos = next.Position
			}
			newPos = prevPos + (maxPos-prevPos)/2.0
		}

		return tx.Model(&domain.Nav{}).
			Where("id = ? AND kb_id = ?", id, kbId).
			Update("position", newPos).Error
	})
}

func (r *NavRepository) Delete(ctx context.Context, kbId, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND kb_id = ?", id, kbId).
		Delete(&domain.Nav{}).Error
}

func (r *NavRepository) Update(ctx context.Context, kbId, id, name string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Nav{}).
		Where("id = ? AND kb_id = ?", id, kbId).
		Update("name", name).Error
}

func (r *NavRepository) UpdatePermissions(ctx context.Context, kbId, id string, permissions domain.NodePermissions) error {
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&domain.Nav{}).
		Where("id = ? AND kb_id = ?", id, kbId).
		Update("permissions", gorm.Expr("jsonb_set(?::jsonb - 'management_grants', '{management_grants}', COALESCE(permissions->'management_grants', '[]'::jsonb), true)", string(encoded))).Error
}

func (r *NavRepository) GetReleaseList(ctx context.Context, kbId string) ([]v1.NavListResp, error) {
	// get latest kb release
	var kbRelease *domain.KBRelease
	if err := r.db.WithContext(ctx).
		Model(&domain.KBRelease{}).
		Where("kb_id = ?", kbId).
		Order("created_at DESC").
		First(&kbRelease).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	navs := make([]v1.NavListResp, 0)
	if err := r.db.WithContext(ctx).
		Model(&domain.NavRelease{}).
		Where("release_id = ?", kbRelease.ID).
		Joins("LEFT JOIN navs ON navs.id = nav_releases.nav_id AND navs.kb_id = nav_releases.kb_id").
		Select("nav_releases.nav_id as id, nav_releases.name, nav_releases.position, COALESCE(navs.permissions, nav_releases.permissions) as permissions").
		Order("nav_releases.position ASC").
		Find(&navs).Error; err != nil {
		return nil, err
	}
	return navs, nil
}
