package pg

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
	"gorm.io/gorm"
)

func documentManagementAccess(ctx context.Context, db *store.DB, kbID, userID string) (*domain.DocumentManagementAccess, error) {
	var user domain.User
	if err := db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	access := &domain.DocumentManagementAccess{Principal: &domain.EnterpriseAccessContext{PrincipalID: userID}, Nodes: map[string]*domain.Node{}, Navs: map[string]*domain.Nav{}}
	// Reading audiences, legacy KB membership and management grants never elevate
	// an ordinary user into an administrator.
	if !consts.IsAdministrator(user.Role) {
		return access, nil
	}
	access.Global = true
	var membership domain.KBUsers
	err := db.WithContext(ctx).Where("kb_id = ? AND user_id = ?", kbID, userID).First(&membership).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	access.Global = access.Global || membership.Perm == consts.UserKBPermissionFullControl || membership.Perm == consts.UserKBPermissionDocManage
	if user.PermissionLevelID != nil && *user.PermissionLevelID != "" {
		var level domain.PermissionLevel
		if err := db.WithContext(ctx).Where("id = ? AND status = ?", *user.PermissionLevelID, domain.PermissionLevelStatusActive).First(&level).Error; err == nil {
			access.Principal.PermissionLevelID = level.ID
			access.Principal.PermissionLevelRank = level.Rank
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		var level domain.PermissionLevel
		if err := db.WithContext(ctx).Where("is_default = ? AND status = ?", true, domain.PermissionLevelStatusActive).First(&level).Error; err == nil {
			access.Principal.PermissionLevelID = level.ID
			access.Principal.PermissionLevelRank = level.Rank
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if err := db.WithContext(ctx).Model(&domain.UserDepartment{}).Where("user_id = ?", userID).Pluck("department_id", &access.Principal.DepartmentIDs).Error; err != nil {
		return nil, err
	}
	var nodes []*domain.Node
	if err := db.WithContext(ctx).Select("id, kb_id, nav_id, parent_id, type, permissions, meta").Where("kb_id = ?", kbID).Find(&nodes).Error; err != nil {
		return nil, err
	}
	for _, node := range nodes {
		access.Nodes[node.ID] = node
	}
	var navs []*domain.Nav
	if err := db.WithContext(ctx).Select("id, kb_id, permissions").Where("kb_id = ?", kbID).Find(&navs).Error; err != nil {
		return nil, err
	}
	for _, nav := range navs {
		access.Navs[nav.ID] = nav
	}
	return access, nil
}

func (r *NavRepository) GetByKBID(ctx context.Context, kbID, id string) (*domain.Nav, error) {
	var nav domain.Nav
	if err := r.db.WithContext(ctx).Where("kb_id = ? AND id = ?", kbID, id).First(&nav).Error; err != nil {
		return nil, err
	}
	return &nav, nil
}

func (r *NavRepository) UpdateManagementGrants(ctx context.Context, kbID, id string, grants []domain.DocumentManagementGrant) error {
	if grants == nil {
		grants = []domain.DocumentManagementGrant{}
	}
	encoded, err := json.Marshal(grants)
	if err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Model(&domain.Nav{}).Where("kb_id = ? AND id = ?", kbID, id).Update("permissions", gorm.Expr("jsonb_set(COALESCE(permissions, '{}'::jsonb), '{management_grants}', ?::jsonb, true)", string(encoded)))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserAccessRepository) DocumentManagementAccess(ctx context.Context, kbID, userID string) (*domain.DocumentManagementAccess, error) {
	return documentManagementAccess(ctx, r.db, kbID, userID)
}
func (r *EnterpriseRepository) DocumentManagementAccess(ctx context.Context, kbID, userID string) (*domain.DocumentManagementAccess, error) {
	return documentManagementAccess(ctx, r.db, kbID, userID)
}

func (r *UserAccessRepository) GetReleaseNodeForManagement(ctx context.Context, kbID, releaseID string) (string, error) {
	var release domain.NodeRelease
	if err := r.db.WithContext(ctx).Where("kb_id = ? AND id = ?", kbID, releaseID).First(&release).Error; err != nil {
		return "", err
	}
	return release.NodeID, nil
}

func (r *NodeRepository) UpdateNodeManagementGrants(ctx context.Context, kbID string, ids []string, grants []domain.DocumentManagementGrant) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var nodes []domain.Node
		if err := tx.Where("kb_id = ? AND id IN ?", kbID, ids).Find(&nodes).Error; err != nil {
			return err
		}
		found := map[string]bool{}
		for _, node := range nodes {
			found[node.ID] = true
		}
		for _, id := range ids {
			if !found[id] {
				return gorm.ErrRecordNotFound
			}
		}
		encoded, err := json.Marshal(grants)
		if err != nil {
			return err
		}
		if grants == nil {
			encoded = []byte("[]")
		}
		return tx.Model(&domain.Node{}).Where("kb_id = ? AND id IN ?", kbID, ids).Update("permissions", gorm.Expr("jsonb_set(COALESCE(permissions, '{}'::jsonb), '{management_grants}', ?::jsonb, true)", string(encoded))).Error
	})
}

// Permission choices expose only the fields needed to pick an audience.
func (r *EnterpriseRepository) PermissionAudienceUsers(ctx context.Context) ([]domain.UserSummary, error) {
	var users []domain.UserSummary
	err := r.db.WithContext(ctx).Model(&domain.User{}).Select("id, account, name, role").Order("account ASC").Find(&users).Error
	return users, err
}

func (r *UserAccessRepository) ManagementZipTarget(ctx context.Context, id string) (*domain.ZipUploadTask, error) {
	var task domain.ZipUploadTask
	if err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}
func (r *UserAccessRepository) ManagementDownloadTarget(ctx context.Context, id string) (*domain.FolderDownloadTask, error) {
	var task domain.FolderDownloadTask
	if err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}
