package pg

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type EnterpriseRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewEnterpriseRepository(db *pg.DB, logger *log.Logger) *EnterpriseRepository {
	return &EnterpriseRepository{
		db:     db,
		logger: logger.WithModule("repo.pg.enterprise"),
	}
}

func (r *EnterpriseRepository) ListDepartments(ctx context.Context) ([]domain.Department, error) {
	var departments []domain.Department
	if err := r.db.WithContext(ctx).Order(`path ASC, sort ASC, created_at ASC`).Find(&departments).Error; err != nil {
		return nil, err
	}
	return departments, nil
}

func (r *EnterpriseRepository) GetDepartmentByID(ctx context.Context, id string) (*domain.Department, error) {
	var department domain.Department
	if err := r.db.WithContext(ctx).First(&department, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &department, nil
}

func (r *EnterpriseRepository) GetDepartmentsByParentPath(ctx context.Context, parentPath string) ([]domain.Department, error) {
	var departments []domain.Department
	if err := r.db.WithContext(ctx).
		Where("path = ? OR path LIKE ?", parentPath, parentPath+"/%").
		Order(`path ASC, sort ASC, created_at ASC`).
		Find(&departments).Error; err != nil {
		return nil, err
	}
	return departments, nil
}

func (r *EnterpriseRepository) UpsertDepartment(ctx context.Context, department *domain.Department) error {
	return r.db.WithContext(ctx).Save(department).Error
}

func (r *EnterpriseRepository) DeleteDepartment(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("department_id = ?", id).Delete(&domain.UserDepartment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Department{}, "id = ?", id).Error
	})
}

func (r *EnterpriseRepository) ListPermissionLevels(ctx context.Context) ([]domain.PermissionLevel, error) {
	var levels []domain.PermissionLevel
	if err := r.db.WithContext(ctx).Order(`rank ASC, created_at ASC`).Find(&levels).Error; err != nil {
		return nil, err
	}
	return levels, nil
}

func (r *EnterpriseRepository) UpsertPermissionLevel(ctx context.Context, level *domain.PermissionLevel) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if level.IsDefault {
			if err := tx.Model(&domain.PermissionLevel{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(level).Error
	})
}

func (r *EnterpriseRepository) DeletePermissionLevel(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.User{}).Where("permission_level_id = ?", id).Update("permission_level_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.PermissionLevel{}, "id = ?", id).Error
	})
}

func (r *EnterpriseRepository) GetDefaultPermissionLevel(ctx context.Context) (*domain.PermissionLevel, error) {
	var level domain.PermissionLevel
	err := r.db.WithContext(ctx).
		Where("is_default = ? AND status = ?", true, domain.PermissionLevelStatusActive).
		First(&level).Error
	if err != nil {
		return nil, err
	}
	return &level, nil
}

func (r *EnterpriseRepository) GetPermissionLevelByID(ctx context.Context, id string) (*domain.PermissionLevel, error) {
	var level domain.PermissionLevel
	if err := r.db.WithContext(ctx).First(&level, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &level, nil
}

func (r *EnterpriseRepository) UpdateUserEnterpriseAccess(ctx context.Context, userID, permissionLevelID string, departmentIDs []string, primaryDepartment string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateMap := map[string]any{
			"permission_level_id": nil,
		}
		if permissionLevelID != "" {
			updateMap["permission_level_id"] = permissionLevelID
		}
		if err := tx.Model(&domain.User{}).Where("id = ?", userID).Updates(updateMap).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&domain.UserDepartment{}).Error; err != nil {
			return err
		}

		if len(departmentIDs) == 0 {
			return nil
		}

		rows := make([]domain.UserDepartment, 0, len(departmentIDs))
		for _, departmentID := range departmentIDs {
			if departmentID == "" {
				continue
			}
			rows = append(rows, domain.UserDepartment{
				UserID:       userID,
				DepartmentID: departmentID,
				IsPrimary:    departmentID == primaryDepartment,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *EnterpriseRepository) GetUserAccessContext(ctx context.Context, userID string) (*domain.EnterpriseAccessContext, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	result := &domain.EnterpriseAccessContext{
		PrincipalID: userID,
	}

	if user.PermissionLevelID != nil && *user.PermissionLevelID != "" {
		level, err := r.GetPermissionLevelByID(ctx, *user.PermissionLevelID)
		if err != nil {
			return nil, err
		}
		result.PermissionLevelID = level.ID
		result.PermissionLevelRank = level.Rank
	} else {
		level, err := r.GetDefaultPermissionLevel(ctx)
		if err == nil {
			result.PermissionLevelID = level.ID
			result.PermissionLevelRank = level.Rank
		}
	}

	ids := make([]string, 0)
	if err := r.db.WithContext(ctx).Model(&domain.UserDepartment{}).Where("user_id = ?", userID).Pluck("department_id", &ids).Error; err != nil {
		return nil, err
	}
	result.DepartmentIDs = ids
	return result, nil
}

func (r *EnterpriseRepository) GetUserDepartments(ctx context.Context, userID string) ([]string, string, error) {
	rows := make([]domain.UserDepartment, 0)
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, "", err
	}
	departmentIDs := make([]string, 0, len(rows))
	primaryDepartment := ""
	for _, row := range rows {
		departmentIDs = append(departmentIDs, row.DepartmentID)
		if row.IsPrimary {
			primaryDepartment = row.DepartmentID
		}
	}
	return departmentIDs, primaryDepartment, nil
}

func (r *EnterpriseRepository) EnsureDefaultPermissionLevel(ctx context.Context) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.PermissionLevel{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaultLevel := &domain.PermissionLevel{
		ID:        "default",
		Name:      "默认级别",
		Code:      "default",
		Rank:      0,
		IsDefault: true,
		Status:    domain.PermissionLevelStatusActive,
	}
	if err := r.UpsertPermissionLevel(ctx, defaultLevel); err != nil {
		return fmt.Errorf("failed to create default permission level: %w", err)
	}
	return nil
}
