package pg

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const permissionSettingKey consts.SystemSettingKey = "permission_levels"

type permissionSetting struct {
	Enabled bool `json:"enabled"`
}

func (r *SystemSettingRepo) PermissionsEnabled(ctx context.Context) (bool, error) {
	r.permissionMu.Lock()
	defer r.permissionMu.Unlock()
	if time.Now().Before(r.permissionExpires) {
		return r.permissionEnabled, nil
	}
	setting, err := r.GetSystemSetting(ctx, permissionSettingKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		value, _ := json.Marshal(permissionSetting{Enabled: !config.DefaultPermissionLevelsDisabled()})
		initial := domain.SystemSetting{Key: permissionSettingKey, Value: value, Description: "等级权限开关"}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&initial).Error; err != nil {
			return true, err
		}
		setting, err = r.GetSystemSetting(ctx, permissionSettingKey)
	}
	if err != nil {
		return true, err
	}
	var value struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.Unmarshal(setting.Value, &value); err != nil {
		return true, err
	}
	if value.Enabled == nil {
		return true, errors.New("invalid permission setting")
	}
	r.permissionEnabled = *value.Enabled
	r.permissionExpires = time.Now().Add(time.Second)
	return r.permissionEnabled, nil
}

func (r *SystemSettingRepo) SetPermissionsEnabled(ctx context.Context, enabled bool) error {
	r.permissionMu.Lock()
	defer r.permissionMu.Unlock()
	value, _ := json.Marshal(permissionSetting{Enabled: enabled})
	if err := r.UpsertSystemSetting(ctx, permissionSettingKey, string(value), "等级权限开关"); err != nil {
		return err
	}
	r.permissionEnabled = enabled
	r.permissionExpires = time.Now().Add(time.Second)
	return nil
}
