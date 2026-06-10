package v1

import (
	"time"

	"github.com/chaitin/panda-wiki/consts"
)

type CreateUserReq struct {
	Account           string          `json:"account" validate:"required,alphanum"`
	Name              string          `json:"name" validate:"required"`
	Password          string          `json:"password" validate:"required,min=8"`
	Role              consts.UserRole `json:"role" validate:"required,oneof=manager user"`
	DepartmentIDs     []string        `json:"department_ids" validate:"required,min=1,dive,required"`
	PrimaryDepartment string          `json:"primary_department" validate:"required"`
}

type CreateUserResp struct {
	ID string `json:"id"`
}

type UserInfoResp struct {
	ID                string          `json:"id"`
	Account           string          `json:"account"`
	Name              string          `json:"name"`
	Role              consts.UserRole `json:"role"`
	PermissionLevelID *string         `json:"permission_level_id,omitempty"`
	DepartmentIDs     []string        `json:"department_ids,omitempty"`
	PrimaryDepartment string          `json:"primary_department,omitempty"`
	IsToken           bool            `json:"is_token"`
	LastAccess        *time.Time      `json:"last_access,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type UserListReq struct {
	Keyword      string          `json:"keyword" query:"keyword"`
	Role         consts.UserRole `json:"role" query:"role" validate:"omitempty,oneof=admin manager user"`
	DepartmentID string          `json:"department_id" query:"department_id"`
}

type UserListItemResp struct {
	ID                string          `json:"id"`
	Account           string          `json:"account"`
	Name              string          `json:"name"`
	Role              consts.UserRole `json:"role"`
	PermissionLevelID *string         `json:"permission_level_id,omitempty"`
	DepartmentIDs     []string        `json:"department_ids,omitempty"`
	PrimaryDepartment string          `json:"primary_department,omitempty"`
	LastAccess        *time.Time      `json:"last_access"`
	CreatedAt         *time.Time      `json:"created_at"`
}

type LoginReq struct {
	Account  string `json:"account" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResp struct {
	Token string `json:"token"`
}

type UserListResp struct {
	Users []UserListItemResp `json:"users"`
}

type ResetPasswordReq struct {
	ID          string `json:"id" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type DeleteUserReq struct {
	UserID string `json:"user_id" query:"user_id" validate:"required"`
}
