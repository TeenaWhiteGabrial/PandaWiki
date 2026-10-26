package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	v1 "github.com/chaitin/panda-wiki/api/user/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type UserRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewUserRepository(db *pg.DB, logger *log.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger.WithModule("repo.pg.user"),
	}
}

func (r *UserRepository) UpsertDefaultUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.User
		err := tx.Where("account = ?", user.Account).First(&existing).Error
		if err == nil {
			return nil
		} // Never reset a changed password on restart.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := domain.ValidatePassword(user.Password, user.Account); err != nil {
			return err
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashed)
		user.PasswordChangeRequired = true
		return tx.Create(user).Error
	})
}

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User, departmentIDs []string, edition consts.LicenseEdition, enterpriseRepo *EnterpriseRepository) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedPassword)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if user.Role == consts.UserRoleAdmin {
			var count int64
			if err := tx.Model(&domain.User{}).Where("role = ?", consts.UserRoleAdmin).Count(&count).Error; err != nil {
				return err
			}
			if count >= domain.GetBaseEditionLimitation(ctx).MaxAdmin {
				return fmt.Errorf("exceed max admin limit, current count: %d, max limit: %d", count, domain.GetBaseEditionLimitation(ctx).MaxAdmin)
			}
		}

		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if enterpriseRepo != nil {
			permissionLevelID := ""
			if user.PermissionLevelID != nil {
				permissionLevelID = *user.PermissionLevelID
			}
			// User, selected grade and department assignments commit together.
			accessRepo := *enterpriseRepo
			accessRepo.db = &pg.DB{DB: tx}
			if err := accessRepo.UpdateUserEnterpriseAccess(ctx, user.ID, permissionLevelID, departmentIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *UserRepository) VerifyUser(ctx context.Context, account string, password string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("account = ?", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid password")
	}
	return &user, nil
}

func (r *UserRepository) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListUsersWithEnterpriseAccess(ctx context.Context, authUserID string, req *v1.UserListReq) ([]v1.UserListItemResp, int64, error) {
	page := 1
	pageSize := 20
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}
	if page > 100000 {
		page = 100000
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var authUser *domain.User
	if authUserID != "" {
		var err error
		authUser, err = r.GetUser(ctx, authUserID)
		if err != nil {
			return nil, 0, fmt.Errorf("get user list scope: %w", err)
		}
	}

	buildQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&domain.User{})
		if req != nil {
			if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
				pattern := "%" + keyword + "%"
				query = query.Where("(users.account ILIKE ? OR users.name ILIKE ?)", pattern, pattern)
			}
			if req.Role != "" {
				query = query.Where("users.role = ?", req.Role)
			}
			if req.DepartmentID != "" {
				query = query.Where(`EXISTS (
					SELECT 1 FROM user_departments filter_departments
					WHERE filter_departments.user_id = users.id
					AND filter_departments.department_id = ?
				)`, req.DepartmentID)
			}
		}
		if authUser != nil && authUser.Role == consts.UserRoleManager {
			// Department links include ancestors for inherited permissions. Scope
			// the list by each user's effective (primary or deepest) department,
			// otherwise managers in sibling departments share an ancestor and
			// would see each other's users.
			query = query.Where("users.role <> ?", consts.UserRoleAdmin).
				Where(`(
					users.id = ? OR EXISTS (
						SELECT 1
						FROM user_departments visible_departments
						INNER JOIN departments visible_department
							ON visible_department.id = visible_departments.department_id
						WHERE visible_departments.user_id = users.id
						AND (
							visible_departments.is_primary = TRUE OR NOT EXISTS (
								SELECT 1 FROM user_departments visible_primary
								WHERE visible_primary.user_id = visible_departments.user_id
								AND visible_primary.is_primary = TRUE
							) AND NOT EXISTS (
								SELECT 1
								FROM user_departments visible_descendants
								INNER JOIN departments visible_descendant_department
									ON visible_descendant_department.id = visible_descendants.department_id
								WHERE visible_descendants.user_id = visible_departments.user_id
								AND visible_descendant_department.path LIKE visible_department.path || '/%'
							)
						)
						AND visible_departments.department_id IN (
							SELECT manager_departments.department_id
							FROM user_departments manager_departments
							INNER JOIN departments manager_department
								ON manager_department.id = manager_departments.department_id
							WHERE manager_departments.user_id = ?
							AND (
								manager_departments.is_primary = TRUE OR NOT EXISTS (
									SELECT 1 FROM user_departments manager_primary
									WHERE manager_primary.user_id = manager_departments.user_id
									AND manager_primary.is_primary = TRUE
								) AND NOT EXISTS (
									SELECT 1
									FROM user_departments manager_descendants
									INNER JOIN departments manager_descendant_department
										ON manager_descendant_department.id = manager_descendants.department_id
									WHERE manager_descendants.user_id = manager_departments.user_id
									AND manager_descendant_department.path LIKE manager_department.path || '/%'
								)
							)
						)
					)
				)`, authUserID, authUserID)
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	users := make([]v1.UserListItemResp, 0, pageSize)
	if err := buildQuery().
		Select("users.id, users.account, users.name, users.role, users.permission_level_id, users.last_access, users.created_at").
		Order("users.created_at DESC, users.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	if len(users) == 0 {
		return users, total, nil
	}
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	departments := make([]domain.UserDepartment, 0)
	if err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Order("id ASC").
		Find(&departments).Error; err != nil {
		return nil, 0, err
	}
	departmentIDsByUser := make(map[string][]string, len(users))
	for _, department := range departments {
		departmentIDsByUser[department.UserID] = append(
			departmentIDsByUser[department.UserID],
			department.DepartmentID,
		)
	}
	for index := range users {
		users[index].DepartmentIDs = departmentIDsByUser[users[index].ID]
	}

	return users, total, nil
}

func (r *UserRepository) GetUsersAccountMap(ctx context.Context) (map[string]string, error) {
	var users []v1.UserListItemResp
	err := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	m := lo.SliceToMap(users, func(user v1.UserListItemResp) (string, string) {
		return user.ID, user.Account
	})

	return m, nil
}

func (r *UserRepository) GetUsersByIDs(ctx context.Context, userIDs []string) (map[string]*domain.UserSummary, error) {
	if len(userIDs) == 0 {
		return map[string]*domain.UserSummary{}, nil
	}

	var users []domain.UserSummary
	if err := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Select("id, account, name").
		Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*domain.UserSummary, len(users))
	for i := range users {
		user := users[i]
		result[user.ID] = &user
	}

	return result, nil
}

func (r *UserRepository) UpdateUserPassword(ctx context.Context, userID string, newPassword string) error {
	return r.setPassword(ctx, userID, newPassword, true)
}
func (r *UserRepository) ChangeOwnPassword(ctx context.Context, userID, password string) error {
	return r.setPassword(ctx, userID, password, false)
}
func (r *UserRepository) setPassword(ctx context.Context, userID, password string, requireChange bool) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.User{}).Where("id = ?", userID).Updates(map[string]any{"password": string(hashed), "password_change_required": requireChange}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&domain.AccountSession{}).Error
	})
}

func (r *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Delete(&domain.User{}).Error; err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Model(&domain.KBUsers{}).Where("user_id = ?", userID).Delete(&domain.KBUsers{}).Error; err != nil {
		return err
	}
	return nil
}
