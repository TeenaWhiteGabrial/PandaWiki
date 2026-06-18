package pg

import (
	"context"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
	"gorm.io/gorm/clause"
)

type SystemSettingRepo struct {
	db     *pg.DB
	logger *log.Logger
}

func NewSystemSettingRepo(db *pg.DB, logger *log.Logger) *SystemSettingRepo {
	return &SystemSettingRepo{
		db:     db,
		logger: logger.WithModule("repo.pg.system_setting"),
	}
}

func (r *SystemSettingRepo) GetSystemSetting(ctx context.Context, key consts.SystemSettingKey) (*domain.SystemSetting, error) {
	var setting domain.SystemSetting
	result := r.db.WithContext(ctx).Where("key = ?", key).First(&setting)
	if result.Error != nil {
		return nil, result.Error
	}

	return &setting, nil
}

func (r *SystemSettingRepo) UpdateSystemSetting(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Model(&domain.SystemSetting{}).Where("key = ?", key).Update("value", value).Error
}

func (r *SystemSettingRepo) UpsertSystemSetting(ctx context.Context, key consts.SystemSettingKey, value, description string) error {
	setting := domain.SystemSetting{
		Key:         key,
		Value:       []byte(value),
		Description: description,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":       []byte(value),
			"description": description,
			"updated_at":  time.Now(),
		}),
	}).Create(&setting).Error
}
