package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"

	"github.com/chaitin/panda-wiki/consts"
)

const (
	SessionCacheKey = "_session_store"
	SessionName     = "_pw_auth_session"
)

type Auth struct {
	ID            uint              `gorm:"primaryKey;column:id" json:"id,omitempty"` // Unique identifier for the authentication record
	IP            string            `gorm:"column:ip;not null" json:"ip,omitempty"`   // IP address from which the login occurred (nullable)
	KBID          string            `gorm:"column:kb_id;not null"  json:"kb_id,omitempty"`
	UnionID       string            `gorm:"column:union_id;not null" json:"union_id,omitempty"`               // Union ID for the user, used in OAuth scenarios
	SourceType    consts.SourceType `gorm:"column:source_type;not null" json:"source_type,omitempty"`         // Type of authentication source (e.g., "local", "oauth")
	LastLoginTime time.Time         `gorm:"column:last_login_time;not null" json:"last_login_time,omitempty"` // Timestamp of the last successful login (nullable)
	CreatedAt     time.Time         `gorm:"column:created_at;not null;default:now()" json:"created_at"`       // Timestamp when the record was created
	UpdatedAt     time.Time         `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`       // Timestamp when the record was last updated
	UserInfo      AuthUserInfo      `json:"user_info" gorm:"type:jsonb"`
}

func (Auth) TableName() string {
	return "auths"
}

type AuthGroup struct {
	ID           uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string            `json:"name" gorm:"uniqueIndex;size:100;not null"`
	KbID         string            `json:"kb_id,omitempty" gorm:"column:kb_id;not null"`
	ParentID     *uint             `json:"parent_id" gorm:"column:parent_id"`
	Position     float64           `json:"position"`
	AuthIDs      pq.Int64Array     `json:"auth_ids" gorm:"type:int[]"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	SyncId       string            `json:"sync_id"`
	SyncParentId string            `json:"sync_parent_id"`
	SourceType   consts.SourceType `json:"source_type" gorm:"column:source_type;not null"`

	// 关联字段
	Parent   *AuthGroup  `json:"parent,omitempty" gorm:"-"`
	Children []AuthGroup `json:"children,omitempty" gorm:"-"`
}

func (AuthGroup) TableName() string {
	return "auth_groups"
}

type AuthConfig struct {
	ID          uint              `gorm:"primaryKey;column:id"` // Unique identifier for the authentication configuration
	KbID        string            `gorm:"column:kb_id;not null"  json:"kb_id"`
	AuthSetting AuthSetting       `gorm:"type:jsonb" json:"auth_setting"`
	SourceType  consts.SourceType `gorm:"column:source_type;not null;unique"`       // Unique type of authentication source (e.g., "github", "google")
	CreatedAt   time.Time         `gorm:"column:created_at;not null;default:now()"` // Timestamp when the record was created
	UpdatedAt   time.Time         `gorm:"column:updated_at;not null;default:now()"` // Timestamp when the record was last updated
}

func (s *AuthSetting) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid AuthSetting type:", value))
	}
	return json.Unmarshal(bytes, s)
}

func (s AuthSetting) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (AuthConfig) TableName() string {
	return "auth_configs"
}

type AuthSetting struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Proxy        string `json:"proxy,omitempty"`
}

type AuthInfo struct {
	ID           uint         `gorm:"column:id" json:"id,omitempty"`
	AuthUserInfo AuthUserInfo `json:"auth_user_info" gorm:"type:jsonb"`
}

type AuthUserInfo struct {
	Username  string `json:"username,omitempty"`
	AvatarUrl string `json:"avatar_url,omitempty"`
	Email     string `json:"email,omitempty"`
}

func (s *AuthUserInfo) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid user info type:", value))
	}
	return json.Unmarshal(bytes, s)
}

func (s *AuthUserInfo) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func GetShareUserID(c echo.Context) string {
	if authInfo := GetAuthInfoFromCtx(c.Request().Context()); authInfo != nil && authInfo.UserId != "" {
		return authInfo.UserId
	}
	return ""
}
