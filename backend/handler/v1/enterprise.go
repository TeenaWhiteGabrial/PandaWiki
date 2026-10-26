package v1

import (
	"fmt"
	"slices"

	"github.com/labstack/echo/v4"

	enterpriseV1 "github.com/chaitin/panda-wiki/api/enterprise/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
)

type EnterpriseHandler struct {
	*handler.BaseHandler
	usecase     *usecase.EnterpriseUsecase
	userUsecase *usecase.UserUsecase
	auth        middleware.AuthMiddleware
}

func NewEnterpriseHandler(e *echo.Echo, baseHandler *handler.BaseHandler, usecase *usecase.EnterpriseUsecase, userUsecase *usecase.UserUsecase, auth middleware.AuthMiddleware) *EnterpriseHandler {
	h := &EnterpriseHandler{
		BaseHandler: baseHandler,
		usecase:     usecase,
		userUsecase: userUsecase,
		auth:        auth,
	}
	group := e.Group("/api/v1/enterprise", h.auth.Authorize, h.auth.ValidateUserRole(consts.UserRoleManager))
	group.GET("/departments", h.ListDepartments)
	group.PUT("/users/access", h.UpdateUserEnterpriseAccess)
	group.GET("/permission_levels", h.ListPermissionLevels)
	adminGroup := e.Group("/api/v1/enterprise", h.auth.Authorize, h.auth.ValidateUserRole(consts.UserRoleAdmin))
	adminGroup.POST("/departments", h.UpsertDepartment)
	adminGroup.PUT("/departments", h.UpsertDepartment)
	adminGroup.DELETE("/departments", h.DeleteDepartment)
	adminGroup.POST("/permission_levels", h.UpsertPermissionLevel)
	adminGroup.PUT("/permission_levels", h.UpsertPermissionLevel)
	adminGroup.DELETE("/permission_levels", h.DeletePermissionLevel)
	return h
}

func (h *EnterpriseHandler) validateManagerDepartmentScope(c echo.Context, targetUserID string, targetDepartmentIDs []string) error {
	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return fmt.Errorf("authInfo not found in context")
	}
	operator, err := h.userUsecase.GetUser(c.Request().Context(), authInfo.UserId)
	if err != nil {
		return err
	}
	if operator.Role != consts.UserRoleManager {
		return nil
	}
	targetUser, err := h.userUsecase.GetUser(c.Request().Context(), targetUserID)
	if err != nil {
		return err
	}
	if targetUser.Role != consts.UserRoleUser {
		return fmt.Errorf("manager can only manage normal users")
	}
	managerDepartments, err := h.userUsecase.GetEffectiveDepartmentIDs(c.Request().Context(), operator.ID)
	if err != nil {
		return err
	}
	if len(managerDepartments) == 0 {
		return fmt.Errorf("manager has no department")
	}
	currentDepartments, err := h.userUsecase.GetEffectiveDepartmentIDs(c.Request().Context(), targetUserID)
	if err != nil {
		return err
	}
	requestedDepartments, err := h.userUsecase.ResolveEffectiveDepartmentIDs(c.Request().Context(), targetDepartmentIDs)
	if err != nil {
		return err
	}
	if len(currentDepartments) == 0 || len(requestedDepartments) == 0 {
		return fmt.Errorf("user has no department in manager scope")
	}
	for _, departmentID := range currentDepartments {
		if !slices.Contains(managerDepartments, departmentID) {
			return fmt.Errorf("target user is outside manager scope")
		}
	}
	for _, departmentID := range requestedDepartments {
		if !slices.Contains(managerDepartments, departmentID) {
			return fmt.Errorf("department out of manager scope")
		}
	}
	return nil
}

func (h *EnterpriseHandler) ListDepartments(c echo.Context) error {
	resp, err := h.usecase.ListDepartments(c.Request().Context())
	if err != nil {
		return h.NewResponseWithError(c, "failed to list departments", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *EnterpriseHandler) UpsertDepartment(c echo.Context) error {
	var req enterpriseV1.DepartmentUpsertReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	id, err := h.usecase.UpsertDepartment(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to save department", err)
	}
	return h.NewResponseWithData(c, map[string]string{"id": id})
}

func (h *EnterpriseHandler) DeleteDepartment(c echo.Context) error {
	var req enterpriseV1.DepartmentDeleteReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := h.usecase.DeleteDepartment(c.Request().Context(), req.ID); err != nil {
		return h.NewResponseWithError(c, "failed to delete department", err)
	}
	return h.NewResponseWithData(c, map[string]bool{"success": true})
}

func (h *EnterpriseHandler) ListPermissionLevels(c echo.Context) error {
	resp, err := h.usecase.ListPermissionLevels(c.Request().Context())
	if err != nil {
		return h.NewResponseWithError(c, "failed to list permission levels", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *EnterpriseHandler) UpsertPermissionLevel(c echo.Context) error {
	var req enterpriseV1.PermissionLevelUpsertReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	id, err := h.usecase.UpsertPermissionLevel(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, "failed to save permission level", err)
	}
	return h.NewResponseWithData(c, map[string]string{"id": id})
}

func (h *EnterpriseHandler) DeletePermissionLevel(c echo.Context) error {
	var req enterpriseV1.PermissionLevelDeleteReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := h.usecase.DeletePermissionLevel(c.Request().Context(), req.ID); err != nil {
		return h.NewResponseWithError(c, "failed to delete permission level", err)
	}
	return h.NewResponseWithData(c, map[string]bool{"success": true})
}

func (h *EnterpriseHandler) UpdateUserEnterpriseAccess(c echo.Context) error {
	var req enterpriseV1.UpdateUserEnterpriseAccessReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := h.validateManagerDepartmentScope(c, req.UserID, req.DepartmentIDs); err != nil {
		return h.NewResponseWithError(c, "failed to update user access", err)
	}
	if err := h.usecase.UpdateUserEnterpriseAccess(c.Request().Context(), &req); err != nil {
		return h.NewResponseWithError(c, "failed to update user access", err)
	}
	return h.NewResponseWithData(c, map[string]bool{"success": true})
}
