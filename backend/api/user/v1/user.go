package v1

import (
	"time"

	"github.com/chaitin/panda-wiki/consts"
)

type CreateUserReq struct {
	Account           string          `json:"account" validate:"required"`
	Name              string          `json:"name" validate:"required"`
	Password          string          `json:"password" validate:"required,min=8"`
	Role              consts.UserRole `json:"role" validate:"required,oneof=manager user"`
	DepartmentIDs     []string        `json:"department_ids" validate:"required,min=1,dive,required"`
	PermissionLevelID string          `json:"permission_level_id,omitempty"`
}

type CreateUserResp struct {
	ID string `json:"id"`
}

type UserInfoResp struct {
	SessionExpiresAt         int64           `json:"session_expires_at,omitempty"`
	PasswordChangeRequired   bool            `json:"password_change_required"`
	PermissionLevelsDisabled bool            `json:"permission_levels_disabled"`
	ID                       string          `json:"id"`
	Account                  string          `json:"account"`
	Name                     string          `json:"name"`
	Role                     consts.UserRole `json:"role"`
	PermissionLevelID        *string         `json:"permission_level_id,omitempty"`
	DepartmentIDs            []string        `json:"department_ids,omitempty"`
	IsToken                  bool            `json:"is_token"`
	LastAccess               *time.Time      `json:"last_access,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
}

type UserListReq struct {
	Keyword      string          `json:"keyword" query:"keyword"`
	Role         consts.UserRole `json:"role" query:"role" validate:"omitempty,oneof=admin manager user"`
	DepartmentID string          `json:"department_id" query:"department_id"`
	Page         int             `json:"page" query:"page" validate:"omitempty,gte=1,lte=100000"`
	PageSize     int             `json:"page_size" query:"page_size" validate:"omitempty,gte=1,lte=100"`
}

type UserListItemResp struct {
	ID                string          `json:"id"`
	Account           string          `json:"account"`
	Name              string          `json:"name"`
	Role              consts.UserRole `json:"role"`
	PermissionLevelID *string         `json:"permission_level_id,omitempty"`
	DepartmentIDs     []string        `json:"department_ids,omitempty"`
	LastAccess        *time.Time      `json:"last_access"`
	CreatedAt         *time.Time      `json:"created_at"`
}

type LoginReq struct {
	Account  string `json:"account" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResp struct {
	PasswordChangeRequired bool   `json:"password_change_required"`
	Token                  string `json:"token,omitempty"`
}

type UserListResp struct {
	Users    []UserListItemResp `json:"users"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type ResetPasswordReq struct {
	ID          string `json:"id" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type DeleteUserReq struct {
	UserID string `json:"user_id" query:"user_id" validate:"required"`
}

// ChangePasswordReq always applies to the authenticated account, never a client ID.
type ChangePasswordReq struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required"`
}
