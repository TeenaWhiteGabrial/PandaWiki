package domain

import (
	"time"

	"github.com/chaitin/panda-wiki/consts"
)

type User struct {
	ID                     string          `json:"id" gorm:"primaryKey"`
	Account                string          `json:"account" gorm:"uniqueIndex"`
	Name                   string          `json:"name"`
	Password               string          `json:"-"`
	Role                   consts.UserRole `json:"role" gorm:"default:'user'"`
	PasswordChangeRequired bool            `json:"password_change_required"`
	PermissionLevelID      *string         `json:"permission_level_id"`
	CreatedAt              time.Time       `json:"created_at"`
	LastAccess             time.Time       `json:"last_access" gorm:"default:null"`
}

// ValidUserAccount keeps account creation consistent across entry points.
func ValidUserAccount(account string) bool {
	if account == "" {
		return false
	}
	for _, char := range account {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

type UserSummary struct {
	ID                string          `json:"id"`
	Account           string          `json:"account"`
	Name              string          `json:"name"`
	Role              consts.UserRole `json:"role,omitempty"`
	PermissionLevelID string          `json:"permission_level_id,omitempty"`
	DepartmentIDs     []string        `json:"department_ids,omitempty" gorm:"-"`
}

// KBUsers 知识库用户关联表（多对多关系）
type KBUsers struct {
	ID        int64                   `json:"id" gorm:"primaryKey;autoIncrement"`
	KBId      string                  `json:"kb_id" gorm:"uniqueIndex:idx_uniq_kb_users_kb_id_user_id"`
	UserId    string                  `json:"user_id" gorm:"uniqueIndex:idx_uniq_kb_users_kb_id_user_id"`
	Perm      consts.UserKBPermission `json:"perm"`
	CreatedAt time.Time               `json:"created_at"`
}

func (KBUsers) TableName() string {
	return "kb_users"
}

type UserAccessTime struct {
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}
