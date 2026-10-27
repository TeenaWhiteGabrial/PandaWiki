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

func documentManagementAccess(ctx context.Context, db *store.DB, kbID, userID string, policyOnly ...bool) (*domain.DocumentManagementAccess, error) {
	var user domain.User
	if err := db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	access := &domain.DocumentManagementAccess{Principal: &domain.EnterpriseAccessContext{PrincipalID: userID}, Nodes: map[string]*domain.Node{}, Navs: map[string]*domain.Nav{}}
	// Legacy KB membership and management grants never elevate an ordinary user
	// into an administrator. Only the super-admin has global document access.
	if !consts.IsAdministrator(user.Role) {
		return access, nil
	}
	access.Global = consts.IsSuperAdmin(user.Role)
	access.Principal.IsSuperAdmin = access.Global
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
	columns := "id, kb_id, nav_id, parent_id, type, name, position, permissions, meta"
	if len(policyOnly) > 0 && policyOnly[0] {
		columns = "id, kb_id, nav_id, parent_id, type, position, permissions"
	}
	if err := db.WithContext(ctx).Select(columns).Where("kb_id = ?", kbID).Find(&nodes).Error; err != nil {
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

// DocumentManagementTreeAccess omits names and file metadata from the policy graph.
func (r *EnterpriseRepository) DocumentManagementTreeAccess(ctx context.Context, kbID, userID string) (*domain.DocumentManagementAccess, error) {
	return documentManagementAccess(ctx, r.db, kbID, userID, true)
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
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Select("id, account, name, role, COALESCE(permission_level_id, '') AS permission_level_id").Order("account ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	var defaultLevel domain.PermissionLevel
	if err := r.db.WithContext(ctx).Where("is_default = ? AND status = ?", true, domain.PermissionLevelStatusActive).First(&defaultLevel).Error; err == nil {
		for i := range users {
			if users[i].PermissionLevelID == "" {
				users[i].PermissionLevelID = defaultLevel.ID
			}
		}
	}
	var assignments []struct {
		UserID       string `gorm:"column:user_id"`
		DepartmentID string `gorm:"column:department_id"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT assigned.user_id, assigned.department_id
		FROM user_departments assigned
		INNER JOIN departments department ON department.id = assigned.department_id
		WHERE assigned.is_primary = TRUE
		OR (
			NOT EXISTS (
				SELECT 1 FROM user_departments primary_assignment
				WHERE primary_assignment.user_id = assigned.user_id
				AND primary_assignment.is_primary = TRUE
			)
			AND NOT EXISTS (
				SELECT 1
				FROM user_departments descendant_assignment
				INNER JOIN departments descendant ON descendant.id = descendant_assignment.department_id
				WHERE descendant_assignment.user_id = assigned.user_id
				AND descendant.path LIKE department.path || '/%'
			)
		)
		ORDER BY department.path ASC`).Scan(&assignments).Error
	if err != nil {
		return nil, err
	}
	usersByID := make(map[string]int, len(users))
	for i := range users {
		usersByID[users[i].ID] = i
	}
	for _, assignment := range assignments {
		if i, ok := usersByID[assignment.UserID]; ok {
			users[i].DepartmentIDs = append(users[i].DepartmentIDs, assignment.DepartmentID)
		}
	}
	return users, nil
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
