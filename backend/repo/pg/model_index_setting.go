package pg

import (
	"context"
	"encoding/json"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UI saves may race the successful cutover. Only that cutover owns an existing
// index baseline; a stale settings form must never move it backwards.
func (r *SystemSettingRepo) SaveModelModeSetting(ctx context.Context, config domain.ModelModeSetting) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var setting domain.SystemSetting
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&setting, "key = ?", consts.SystemSettingModelMode).Error; err != nil {
			return err
		}
		var current domain.ModelModeSetting
		if err := json.Unmarshal(setting.Value, &current); err != nil {
			return err
		}
		if current.EmbeddingIndexConfig != nil {
			config.EmbeddingIndexConfig = current.EmbeddingIndexConfig
		}
		if config.Mode == consts.ModelSettingModeManual && config.EmbeddingIndexConfig != nil {
			var model domain.Model
			if err := tx.Where("type = ? AND is_active = true", domain.ModelTypeEmbedding).First(&model).Error; err == nil {
				config.IsManualEmbeddingUpdated = *config.EmbeddingIndexConfig != model.EmbeddingIndexIdentity()
			}
		}
		value, err := json.Marshal(config)
		if err != nil {
			return err
		}
		return tx.Model(&setting).UpdateColumn("value", value).Error
	})
}
