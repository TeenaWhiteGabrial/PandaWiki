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
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedPassword)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First try to find existing user
		var existingUser domain.User
		err := tx.Where("account = ?", user.Account).First(&existingUser).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			// User doesn't exist, create new user
			if err := tx.Create(user).Error; err != nil {
				return err
			}
			return nil
		}
		// User exists, update default fields
		updates := map[string]any{
			"password": user.Password,
		}
		if user.Name != "" {
			updates["name"] = user.Name
		}
		return tx.Model(&existingUser).Updates(updates).Error
	})
}

func (r *UserRepository) CreateUser(ctx context.Context, user *domain.User, departmentIDs []string, primaryDepartment string, edition consts.LicenseEdition, enterpriseRepo *EnterpriseRepository) error {
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
			if err := enterpriseRepo.UpdateUserEnterpriseAccess(ctx, user.ID, "", departmentIDs, primaryDepartment); err != nil {
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

func (r *UserRepository) ListUsers(ctx context.Context, req *v1.UserListReq) ([]v1.UserListItemResp, error) {
	var users []v1.UserListItemResp
	query := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Select("id, account, name, role, permission_level_id, last_access, created_at").
		Order("created_at DESC")
	if req != nil {
		if req.Keyword != "" {
			keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
			query = query.Where("account LIKE ? OR name LIKE ?", keyword, keyword)
		}
		if req.Role != "" {
			query = query.Where("role = ?", req.Role)
		}
	}
	err := query.Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) ListUsersWithEnterpriseAccess(ctx context.Context, enterpriseRepo *EnterpriseRepository, authUserID string, req *v1.UserListReq) ([]v1.UserListItemResp, error) {
	users, err := r.ListUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	var authUser *domain.User
	if authUserID != "" {
		authUser, _ = r.GetUser(ctx, authUserID)
	}
	managerDepartmentIDs := make(map[string]struct{})
	if authUser != nil && authUser.Role == consts.UserRoleManager {
		departmentIDs, _, err := enterpriseRepo.GetUserDepartments(ctx, authUserID)
		if err != nil {
			return nil, err
		}
		for _, departmentID := range departmentIDs {
			managerDepartmentIDs[departmentID] = struct{}{}
		}
	}
	filtered := make([]v1.UserListItemResp, 0, len(users))
	for i := range users {
		departmentIDs, primaryDepartment, err := enterpriseRepo.GetUserDepartments(ctx, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].DepartmentIDs = departmentIDs
		users[i].PrimaryDepartment = primaryDepartment
		if req != nil && req.DepartmentID != "" {
			matched := false
			for _, departmentID := range departmentIDs {
				if departmentID == req.DepartmentID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if authUser != nil && authUser.Role == consts.UserRoleManager {
			if users[i].Role == consts.UserRoleAdmin {
				continue
			}
			if users[i].ID == authUserID {
				filtered = append(filtered, users[i])
				continue
			}
			matched := false
			for _, departmentID := range departmentIDs {
				if _, ok := managerDepartmentIDs[departmentID]; ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, users[i])
	}
	return filtered, nil
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
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("password", string(hashedPassword)).Error
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
