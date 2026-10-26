package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	v1 "github.com/chaitin/panda-wiki/api/user/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type UserUsecase struct {
	repo           *pg.UserRepository
	enterpriseRepo *pg.EnterpriseRepository
	logger         *log.Logger
	config         *config.Config
}

var ErrAdministratorOnly = errors.New("仅管理员可登录管理后台，请使用用户端查阅文档")

func NewUserUsecase(repo *pg.UserRepository, enterpriseRepo *pg.EnterpriseRepository, logger *log.Logger, config *config.Config) (*UserUsecase, error) {
	if config.AdminPassword != "" {
		if err := repo.UpsertDefaultUser(context.Background(), &domain.User{
			ID:       uuid.New().String(),
			Account:  "admin",
			Name:     "超级管理员",
			Password: config.AdminPassword,
			Role:     consts.UserRoleAdmin,
		}); err != nil {
			return nil, fmt.Errorf("failed to create default user: %w", err)
		}
	}
	return &UserUsecase{
		repo:           repo,
		enterpriseRepo: enterpriseRepo,
		logger:         logger.WithModule("usecase.user"),
		config:         config,
	}, nil
}

func (u *UserUsecase) CreateUser(ctx context.Context, user *domain.User, departmentIDs []string, edition consts.LicenseEdition) error {
	if err := domain.ValidatePassword(user.Password, user.Account); err != nil {
		return err
	}
	user.PasswordChangeRequired = true
	if !domain.ValidUserAccount(user.Account) {
		return errors.New("账号仅支持英文字母（大小写）、数字和下划线")
	}
	if user.Role == consts.UserRoleAdmin {
		return errors.New("admin user cannot be created from ui")
	}
	if len(departmentIDs) == 0 {
		return errors.New("department is required")
	}
	if user.PermissionLevelID != nil && *user.PermissionLevelID != "" {
		level, err := u.enterpriseRepo.GetPermissionLevelByID(ctx, *user.PermissionLevelID)
		if err != nil {
			return fmt.Errorf("权限等级不存在：%w", err)
		}
		if level.Status != domain.PermissionLevelStatusActive {
			return errors.New("权限等级已停用")
		}
	}
	return u.repo.CreateUser(ctx, user, departmentIDs, edition, u.enterpriseRepo)
}

func (u *UserUsecase) VerifyUserAndGenerateToken(ctx context.Context, req v1.LoginReq) (string, error) {
	result, err := u.AuthenticateAccount(ctx, req, true)
	if err != nil {
		return "", err
	}
	return result.Token, nil
}
func (u *UserUsecase) VerifyReaderAndGenerateToken(ctx context.Context, req v1.LoginReq) (string, error) {
	result, err := u.AuthenticateAccount(ctx, req, false)
	if err != nil {
		return "", err
	}
	return result.Token, nil
}
func (u *UserUsecase) AuthenticateAccount(ctx context.Context, req v1.LoginReq, administratorOnly bool) (*v1.LoginResp, error) {
	user, err := u.repo.VerifyUser(ctx, req.Account, req.Password)
	if err != nil {
		return nil, err
	}
	if administratorOnly && !consts.IsAdministrator(user.Role) {
		return nil, ErrAdministratorOnly
	}
	if !consts.IsAdministrator(user.Role) && user.Role != consts.UserRoleUser {
		return nil, fmt.Errorf("账号角色无效")
	}
	maxAge := u.config.Auth.JWT.SessionMaxAgeSeconds
	if maxAge <= 0 {
		maxAge = 28800
	}
	sessionID := uuid.NewString()
	now := time.Now()
	expiresAt := now.Add(time.Duration(maxAge) * time.Second)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": user.ID, "jti": sessionID, "iat": now.Unix(), "exp": expiresAt.Unix()})
	signed, err := token.SignedString([]byte(u.config.Auth.JWT.Secret))
	if err != nil {
		return nil, err
	}
	if err := u.repo.AccountSessions().Create(ctx, sessionID, user.ID, expiresAt); err != nil {
		return nil, err
	}
	return &v1.LoginResp{Token: signed, PasswordChangeRequired: user.PasswordChangeRequired}, nil
}
func (u *UserUsecase) Logout(ctx context.Context, sessionID, userID string) error {
	return u.repo.AccountSessions().Revoke(ctx, sessionID, userID)
}
func (u *UserUsecase) ChangeOwnPassword(ctx context.Context, userID string, req *v1.ChangePasswordReq) error {
	user, err := u.repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if _, err := u.repo.VerifyUser(ctx, user.Account, req.CurrentPassword); err != nil {
		return fmt.Errorf("当前密码错误")
	}
	if req.CurrentPassword == req.NewPassword {
		return fmt.Errorf("新密码不能与当前密码相同")
	}
	if err := domain.ValidatePassword(req.NewPassword, user.Account); err != nil {
		return err
	}
	return u.repo.ChangeOwnPassword(ctx, userID, req.NewPassword)
}

func (u *UserUsecase) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return u.repo.GetUser(ctx, userID)
}

func (u *UserUsecase) GetUserDepartments(ctx context.Context, userID string) ([]string, string, error) {
	return u.enterpriseRepo.GetUserDepartments(ctx, userID)
}

func (u *UserUsecase) GetEffectiveDepartmentIDs(ctx context.Context, userID string) ([]string, error) {
	return u.enterpriseRepo.GetEffectiveDepartmentIDs(ctx, userID)
}

func (u *UserUsecase) ResolveEffectiveDepartmentIDs(ctx context.Context, departmentIDs []string) ([]string, error) {
	return u.enterpriseRepo.ResolveEffectiveDepartmentIDs(ctx, departmentIDs)
}

func (u *UserUsecase) ListUsers(ctx context.Context, authUserID string, req *v1.UserListReq) (*v1.UserListResp, error) {
	if req == nil {
		req = &v1.UserListReq{}
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Page > 100000 {
		req.Page = 100000
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	users, total, err := u.repo.ListUsersWithEnterpriseAccess(ctx, authUserID, req)
	if err != nil {
		return nil, err
	}
	return &v1.UserListResp{
		Users:    users,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (u *UserUsecase) ResetPassword(ctx context.Context, req *v1.ResetPasswordReq) error {
	user, err := u.repo.GetUser(ctx, req.ID)
	if err != nil {
		return err
	}
	if err := domain.ValidatePassword(req.NewPassword, user.Account); err != nil {
		return err
	}
	return u.repo.UpdateUserPassword(ctx, req.ID, req.NewPassword)
}

func (u *UserUsecase) DeleteUser(ctx context.Context, userID string) error {
	return u.repo.DeleteUser(ctx, userID)
}
