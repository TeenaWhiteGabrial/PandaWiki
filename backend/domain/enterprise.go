package domain

import "time"

type DepartmentStatus string

const (
	DepartmentStatusActive   DepartmentStatus = "active"
	DepartmentStatusDisabled DepartmentStatus = "disabled"
)

type Department struct {
	ID        string           `json:"id" gorm:"primaryKey"`
	Name      string           `json:"name"`
	ParentID  *string          `json:"parent_id"`
	Path      string           `json:"path"`
	Sort      int              `json:"sort"`
	Status    DepartmentStatus `json:"status" gorm:"default:'active'"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

func (Department) TableName() string {
	return "departments"
}

type UserDepartment struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID       string    `json:"user_id" gorm:"index"`
	DepartmentID string    `json:"department_id" gorm:"index"`
	IsPrimary    bool      `json:"is_primary"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (UserDepartment) TableName() string {
	return "user_departments"
}

type PermissionLevelStatus string

const (
	PermissionLevelStatusActive   PermissionLevelStatus = "active"
	PermissionLevelStatusDisabled PermissionLevelStatus = "disabled"
)

type PermissionLevel struct {
	ID          string                `json:"id" gorm:"primaryKey"`
	Name        string                `json:"name"`
	Code        string                `json:"code"`
	Rank        int                   `json:"rank"`
	Description string                `json:"description"`
	IsDefault   bool                  `json:"is_default"`
	Status      PermissionLevelStatus `json:"status" gorm:"default:'active'"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

func (PermissionLevel) TableName() string {
	return "permission_levels"
}

type EnterpriseAccessContext struct {
	PrincipalID         string   `json:"principal_id"`
	DepartmentIDs       []string `json:"department_ids"`
	PermissionLevelID   string   `json:"permission_level_id"`
	PermissionLevelRank int      `json:"permission_level_rank"`
}
