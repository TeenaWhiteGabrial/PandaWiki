package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	v1 "github.com/chaitin/panda-wiki/api/user/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/pkg/ratelimit"
	"github.com/chaitin/panda-wiki/store/cache"
	"github.com/chaitin/panda-wiki/usecase"
)

type UserHandler struct {
	*handler.BaseHandler
	usecase     *usecase.UserUsecase
	logger      *log.Logger
	config      *config.Config
	auth        middleware.AuthMiddleware
	rateLimiter *ratelimit.RateLimiter
}

func (h *UserHandler) validateManagerDepartmentScope(ctx context.Context, operator *domain.User, targetDepartmentIDs []string) error {
	if operator == nil || operator.Role != consts.UserRoleManager {
		return nil
	}
	managerDepartments, _, err := h.usecase.GetUserDepartments(ctx, operator.ID)
	if err != nil {
		return err
	}
	if len(managerDepartments) == 0 {
		return fmt.Errorf("manager has no department")
	}
	for _, departmentID := range targetDepartmentIDs {
		if !slices.Contains(managerDepartments, departmentID) {
			return fmt.Errorf("department out of manager scope")
		}
	}
	return nil
}

func NewUserHandler(e *echo.Echo, baseHandler *handler.BaseHandler, logger *log.Logger, usecase *usecase.UserUsecase, auth middleware.AuthMiddleware, config *config.Config, cache *cache.Cache) *UserHandler {
	handlerLogger := logger.WithModule("handler.v1.user")
	h := &UserHandler{
		BaseHandler: baseHandler,
		logger:      handlerLogger,
		usecase:     usecase,
		auth:        auth,
		config:      config,
		rateLimiter: ratelimit.NewRateLimiter(handlerLogger, cache),
	}
	group := e.Group("/api/v1/user")
	group.POST("/login", h.Login)
	// User-facing account login remains available to ordinary readers.
	e.POST("/share/v1/auth/login/account", h.ReaderLogin, h.ShareAuthMiddleware.CheckForbidden)
	e.GET("/share/v1/auth/user", h.ReaderUserInfo, h.ShareAuthMiddleware.Authorize)

	group.GET("", h.GetUserInfo, h.auth.Authorize)
	group.GET("/list", h.ListUsers, h.auth.Authorize)
	group.POST("/create", h.CreateUser, h.auth.Authorize, h.auth.ValidateUserRole(consts.UserRoleManager))
	group.PUT("/reset_password", h.ResetPassword, h.auth.Authorize, h.auth.ValidateUserRole(consts.UserRoleManager))
	group.DELETE("/delete", h.DeleteUser, h.auth.Authorize, h.auth.ValidateUserRole(consts.UserRoleManager))

	return h
}

// CreateUser
//
//	@Summary		CreateUser
//	@Description	CreateUser
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Param			body	body		v1.CreateUserReq	true	"CreateUser Request"
//	@Success		200		{object}	domain.Response{data=v1.CreateUserResp}
//	@Router			/api/v1/user/create [post]
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req v1.CreateUserReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}
	operator, err := h.usecase.GetUser(c.Request().Context(), authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get user", err)
	}
	if err := h.validateManagerDepartmentScope(c.Request().Context(), operator, req.DepartmentIDs); err != nil {
		return h.NewResponseWithError(c, "cannot create user out of your department scope", err)
	}

	uid := uuid.New().String()
	err = h.usecase.CreateUser(c.Request().Context(), &domain.User{
		ID:       uid,
		Account:  req.Account,
		Name:     req.Name,
		Password: req.Password,
		Role:     req.Role,
	}, req.DepartmentIDs, consts.GetLicenseEdition(c))
	if err != nil {
		return h.NewResponseWithError(c, "failed to create user", err)
	}

	return h.NewResponseWithData(c, v1.CreateUserResp{ID: uid})
}

// Login
//
//	@Summary		Login
//	@Description	Login
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Param			body	body		v1.LoginReq	true	"Login Request"
//	@Success		200		{object}	v1.LoginResp
//	@Router			/api/v1/user/login [post]
func (h *UserHandler) Login(c echo.Context) error {
	return h.login(c, true)
}

func (h *UserHandler) ReaderLogin(c echo.Context) error {
	return h.login(c, false)
}

func (h *UserHandler) login(c echo.Context, administratorOnly bool) error {
	var req v1.LoginReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	ctx := c.Request().Context()
	ip := c.RealIP()
	locked, remaining := h.rateLimiter.CheckIPLocked(ctx, ip)
	if locked {
		h.logger.Warn("IP is locked", "ip", ip, "remaining", remaining)
		return h.NewResponseWithError(c, fmt.Sprintf("账号已被锁定，请 %s 后重试", remaining.String()), nil)
	}

	var token string
	var err error
	if administratorOnly {
		token, err = h.usecase.VerifyUserAndGenerateToken(ctx, req)
	} else {
		token, err = h.usecase.VerifyReaderAndGenerateToken(ctx, req)
	}
	if err != nil {
		if errors.Is(err, usecase.ErrAdministratorOnly) {
			return c.JSON(http.StatusForbidden, domain.PWResponse{Success: false, Message: err.Error(), Code: domain.AdministratorOnlyCode})
		}
		h.rateLimiter.LockAttempt(ctx, ip)
		return h.NewResponseWithError(c, "用户名或密码错误", err)
	}

	go func() {
		if err := h.rateLimiter.ResetLoginAttempts(context.Background(), ip); err != nil {
			h.logger.Error("failed to reset login attempts", "error", err, "ip", ip)
		}
	}()

	return h.NewResponseWithData(c, v1.LoginResp{Token: token})
}

func (h *UserHandler) ReaderUserInfo(c echo.Context) error {
	if domain.GetAuthInfoFromCtx(c.Request().Context()) == nil {
		return c.JSON(http.StatusUnauthorized, domain.PWResponse{Success: false, Message: "请先登录"})
	}
	return h.GetUserInfo(c)
}

// GetUserInfo
//
//	@Summary		GetUser
//	@Description	GetUser
//	@Tags			user
//	@Accept			json
//	@Success		200	{object}	v1.UserInfoResp
//	@Router			/api/v1/user [get]
func (h *UserHandler) GetUserInfo(c echo.Context) error {
	ctx := c.Request().Context()
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	user, err := h.usecase.GetUser(c.Request().Context(), authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get user", err)
	}

	userInfo := &v1.UserInfoResp{
		PermissionLevelsDisabled: config.PermissionLevelsDisabled(),
		ID:                       user.ID,
		Account:                  user.Account,
		Name:                     user.Name,
		Role:                     user.Role,
		PermissionLevelID:        user.PermissionLevelID,
		IsToken:                  authInfo.IsToken,
		LastAccess:               &user.LastAccess,
		CreatedAt:                user.CreatedAt,
	}
	if departments, _, err := h.usecase.GetUserDepartments(ctx, user.ID); err == nil {
		userInfo.DepartmentIDs = departments
	}

	return h.NewResponseWithData(c, userInfo)
}

// ListUsers
//
//	@Summary		ListUsers
//	@Description	ListUsers
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	domain.PWResponse{data=v1.UserListResp}
//	@Router			/api/v1/user/list [get]
func (h *UserHandler) ListUsers(c echo.Context) error {
	var req v1.UserListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	users, err := h.usecase.ListUsers(c.Request().Context(), authInfo.UserId, &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to list users", err)
	}
	return h.NewResponseWithData(c, users)
}

// ResetPassword
//
//	@Summary		ResetPassword
//	@Description	ResetPassword
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Param			body	body		v1.ResetPasswordReq	true	"ResetPassword Request"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/user/reset_password [put]
func (h *UserHandler) ResetPassword(c echo.Context) error {
	ctx := c.Request().Context()
	var req v1.ResetPasswordReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	if authInfo.IsToken {
		return h.NewResponseWithError(c, "this api not support token call", nil)
	}

	user, err := h.usecase.GetUser(ctx, authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get user", err)
	}

	if user.Account == "admin" {
		// admin 改不了自己的密码
		if authInfo.UserId == req.ID {
			return h.NewResponseWithError(c, "请修改安装目录下 .env 文件中的 ADMIN_PASSWORD，并重启 panda-wiki-api 容器使更改生效。", nil)
		}
	} else {
		targetUser, err := h.usecase.GetUser(ctx, req.ID)
		if err != nil {
			return h.NewResponseWithError(c, "failed to get target user", err)
		}

		// 超级管理员不能改其他超级管理员密码；管理员不能修改超级管理员密码
		if targetUser.Role == consts.UserRoleAdmin && targetUser.ID != authInfo.UserId {
			return h.NewResponseWithError(c, "无法修改其他超级管理员密码", nil)
		}
		if user.Role == consts.UserRoleManager && targetUser.Role != consts.UserRoleUser && targetUser.ID != authInfo.UserId {
			return h.NewResponseWithError(c, "管理员无法修改该用户密码", nil)
		}
		targetDepartments, _, err := h.usecase.GetUserDepartments(ctx, targetUser.ID)
		if err != nil {
			return h.NewResponseWithError(c, "failed to get target user departments", err)
		}
		if err := h.validateManagerDepartmentScope(ctx, user, targetDepartments); err != nil {
			return h.NewResponseWithError(c, "管理员无法修改其他部门用户密码", err)
		}
	}

	err = h.usecase.ResetPassword(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to reset password", err)
	}

	return h.NewResponseWithData(c, nil)
}

// DeleteUser
//
//	@Summary		DeleteUser
//	@Description	DeleteUser
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Param			params	query		v1.DeleteUserReq	true	"DeleteUser Request"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/user/delete [delete]
func (h *UserHandler) DeleteUser(c echo.Context) error {
	ctx := c.Request().Context()

	var req v1.DeleteUserReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	if authInfo.IsToken {
		return h.NewResponseWithError(c, "this api not support token call", nil)
	}

	if authInfo.UserId == req.UserID {
		return h.NewResponseWithError(c, "cannot delete yourself", nil)
	}

	user, err := h.usecase.GetUser(ctx, authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get user", err)
	}

	targetUser, err := h.usecase.GetUser(ctx, req.UserID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get target user", err)
	}

	if targetUser.Account == "admin" {
		return h.NewResponseWithError(c, "cannot delete admin user", nil)
	}

	// 非admin账号的管理员只能删除普通用户的账户
	if user.Account != "admin" {
		if targetUser.Role != consts.UserRoleUser {
			return h.NewResponseWithError(c, "管理员无法删除该用户", nil)
		}
	}
	targetDepartments, _, err := h.usecase.GetUserDepartments(ctx, targetUser.ID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to get target user departments", err)
	}
	if err := h.validateManagerDepartmentScope(ctx, user, targetDepartments); err != nil {
		return h.NewResponseWithError(c, "管理员无法删除其他部门用户", err)
	}

	err = h.usecase.DeleteUser(ctx, req.UserID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to delete user", err)
	}

	return h.NewResponseWithData(c, nil)
}
