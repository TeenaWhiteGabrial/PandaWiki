package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chaitin/panda-wiki/consts"
)

type Contribute struct {
	Id          string                  `json:"id" gorm:"primaryKey;type:text"`
	AuthId      *int64                  `json:"auth_id"`
	UserID      string                  `json:"user_id" gorm:"type:text"`
	KBId        string                  `json:"kb_id" gorm:"type:text;not null"`
	Status      consts.ContributeStatus `json:"status" gorm:"type:text;not null"`
	Type        consts.ContributeType   `json:"type" gorm:"type:text;not null"`
	NodeId      string                  `json:"node_id" gorm:"type:text"`
	Name        string                  `json:"name" gorm:"type:text"`
	Content     string                  `json:"content" gorm:"type:text;not null"`
	Meta        NodeMeta                `json:"meta"`
	Reason      string                  `json:"reason" gorm:"type:text;not null"`
	AuditUserID string                  `json:"audit_user_id" gorm:"type:text;not null"`
	AuditTime   *time.Time              `json:"audit_time"`
	RemoteIP    string                  `json:"remote_ip" gorm:"type:text;not null"`
	CreatedAt   time.Time               `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt   time.Time               `gorm:"column:updated_at;not null;default:now()"`
}

func (Contribute) TableName() string {
	return "contributes"
}

type SubmitContributeReq struct {
	CaptchaToken string                `json:"captcha_token" validate:"required"`
	Content      string                `json:"content"`
	ContentType  string                `json:"content_type" validate:"required,oneof=html md"`
	Emoji        string                `json:"emoji"`
	Name         string                `json:"name"`
	NodeID       string                `json:"node_id"`
	Reason       string                `json:"reason" validate:"required"`
	Type         consts.ContributeType `json:"type" validate:"required,oneof=add edit"`
}

type SubmitContributeResp struct {
	ID string `json:"id"`
}

type ContributeListReq struct {
	KbID     string                   `json:"kb_id" query:"kb_id" validate:"required"`
	AuthName string                   `json:"auth_name" query:"auth_name"`
	NodeName string                   `json:"node_name" query:"node_name"`
	Status   *consts.ContributeStatus `json:"status" query:"status"`
	Pager
}

type ContributeAuditReq struct {
	ID       string                  `json:"id" validate:"required"`
	KbID     string                  `json:"kb_id" validate:"required"`
	NavID    string                  `json:"nav_id" validate:"required"`
	ParentID *string                 `json:"parent_id"`
	Position *float64                `json:"position"`
	Status   consts.ContributeStatus `json:"status" validate:"required,oneof=approved rejected"`
}

type ContributeAuditResp struct {
	Message string `json:"message"`
}

type ContributeNodeMeta struct {
	ContentType string `json:"content_type"`
	Emoji       string `json:"emoji"`
	DocWidth    string `json:"doc_width"`
}

func (d ContributeNodeMeta) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d *ContributeNodeMeta) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid contribute node meta type:", value))
	}
	return json.Unmarshal(bytes, d)
}

type OriginalContributeNodeInfo struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Content string             `json:"content"`
	Meta    ContributeNodeMeta `json:"meta"`
}

type ContributeItem struct {
	ID             string                  `json:"id"`
	AuthID         *int64                  `json:"auth_id"`
	UserID         string                  `json:"user_id"`
	AuthName       string                  `json:"auth_name"`
	ContributeName string                  `json:"contribute_name"`
	KBId           string                  `json:"kb_id"`
	NodeID         string                  `json:"node_id"`
	NodeName       string                  `json:"node_name"`
	Status         consts.ContributeStatus `json:"status"`
	Type           consts.ContributeType   `json:"type"`
	Reason         string                  `json:"reason"`
	Meta           ContributeNodeMeta      `json:"meta"`
	RemoteIP       string                  `json:"remote_ip"`
	IPAddress      *IPAddress              `json:"ip_address" gorm:"-"`
	AuditUserID    string                  `json:"audit_user_id"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
	AuditTime      *time.Time              `json:"audit_time"`
}

type ContributeListResp struct {
	List  []*ContributeItem `json:"list"`
	Total int64             `json:"total"`
}

type ContributeDetailResp struct {
	ID           string                      `json:"id"`
	AuthID       *int64                      `json:"auth_id"`
	UserID       string                      `json:"user_id"`
	AuthName     string                      `json:"auth_name"`
	KBId         string                      `json:"kb_id"`
	NodeID       string                      `json:"node_id"`
	NodeName     string                      `json:"node_name"`
	Status       consts.ContributeStatus     `json:"status"`
	Type         consts.ContributeType       `json:"type"`
	Name         string                      `json:"name"`
	Content      string                      `json:"content"`
	Reason       string                      `json:"reason"`
	Meta         ContributeNodeMeta          `json:"meta"`
	OriginalNode *OriginalContributeNodeInfo `json:"original_node,omitempty"`
	AuditUserID  string                      `json:"audit_user_id"`
	CreatedAt    time.Time                   `json:"created_at"`
	UpdatedAt    time.Time                   `json:"updated_at"`
	AuditTime    *time.Time                  `json:"audit_time"`
}
