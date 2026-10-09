package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/chaitin/panda-wiki/consts"
)

const (
	MaxPosition    float64 = 1e38
	MinPositionGap float64 = 1e-5
)

type NodeType uint16

const (
	NodeTypeFolder   NodeType = 1
	NodeTypeDocument NodeType = 2
)

type NodeStatus uint16

const (
	NodeStatusUnreleased NodeStatus = 0 // 草稿
	NodeStatusDraft      NodeStatus = 1 // 更新未发布
	NodeStatusPublished  NodeStatus = 2 // 已发布
)

const (
	ContentTypeMD   string = "md"
	ContentTypeHTML string = "html"
)

type JSONB map[string]any

func (d JSONB) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	return json.Marshal(d)
}

func (d *JSONB) Scan(value any) error {
	if value == nil {
		*d = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid jsonb type:", value))
	}
	return json.Unmarshal(bytes, d)
}

// table: nodes
type Node struct {
	ID            string          `json:"id" gorm:"primaryKey"`
	KBID          string          `json:"kb_id" gorm:"index"`
	NavId         string          `json:"nav_id"`
	Type          NodeType        `json:"type"`
	Status        NodeStatus      `json:"status"`
	RagInfo       RagInfo         `json:"rag_info" gorm:"type:jsonb"`
	Name          string          `json:"name"`
	Content       string          `json:"content"`
	EditorContent JSONB           `json:"editor_content" gorm:"type:jsonb"`
	EditorBlocks  JSONB           `json:"editor_blocks" gorm:"type:jsonb"`
	EditorFormat  string          `json:"editor_format"`
	EditorVersion int             `json:"editor_version"`
	Meta          NodeMeta        `json:"meta" gorm:"type:jsonb"` // summary
	ParentID      string          `json:"parent_id"`
	Position      float64         `json:"position"`
	DocID         string          `json:"doc_id"` // DEPRECATED: for rag service
	CreatorId     string          `json:"creator_id"`
	EditorId      string          `json:"editor_id"`
	EditTime      time.Time       `json:"edit_time"`
	Permissions   NodePermissions `json:"permissions" gorm:"type:jsonb"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}

type RagInfo struct {
	LearningProgress *LearningProgress             `json:"learning_progress,omitempty"`
	Status           consts.NodeRagInfoStatus      `json:"status"`
	Message          string                        `json:"message"`
	SyncedAt         time.Time                     `json:"synced_at"`
	TaskStatus       consts.NodeLearningTaskStatus `json:"task_status,omitempty"`
	TaskID           string                        `json:"task_id,omitempty"`
	TaskAction       consts.NodeLearningTaskAction `json:"task_action,omitempty"`
	TaskMessage      string                        `json:"task_message,omitempty"`
	ActiveDocID      string                        `json:"active_doc_id,omitempty"`
	ActiveReleaseID  string                        `json:"active_release_id,omitempty"`
}

func (d *RagInfo) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d *RagInfo) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid node meta type:", value))
	}
	return json.Unmarshal(bytes, d)
}

func (d RagInfo) HasAvailableIndex() bool {
	return d.Status == consts.NodeRagStatusSucceeded
}

type NodePermissions struct {
	Constraints               []NodePermissions         `json:"constraints,omitempty"`
	ManagementGrants          []DocumentManagementGrant `json:"management_grants,omitempty"`
	AllowedPermissionLevelIDs []string                  `json:"allowed_permission_level_ids,omitempty"`
	Answerable                consts.NodeAccessPerm     `json:"answerable"`           // 兼容旧字段，不再单独作为权限入口
	Visitable                 consts.NodeAccessPerm     `json:"visitable"`            // 兼容旧字段，不再单独作为权限入口
	Visible                   consts.NodeAccessPerm     `json:"visible"`              // 兼容旧字段，不再单独作为权限入口
	ScopeType                 string                    `json:"scope_type,omitempty"` // public / custom / department(legacy)
	Inherit                   *bool                     `json:"inherit,omitempty"`    // 目录默认继承父级，可显式覆盖
	MinPermissionLevelID      string                    `json:"min_permission_level_id,omitempty"`
	MinPermissionLevelRank    int                       `json:"min_permission_level_rank,omitempty"`
	AllowedDepartmentIDs      []string                  `json:"allowed_department_ids,omitempty"`
	AllowedUserIDs            []string                  `json:"allowed_user_ids,omitempty"`
	DeniedDepartmentIDs       []string                  `json:"denied_department_ids,omitempty"`
	DeniedUserIDs             []string                  `json:"denied_user_ids,omitempty"`
}

const (
	NodePermissionScopePublic     = "public"
	NodePermissionScopeCustom     = "custom"
	NodePermissionScopeDepartment = "department"
)

func (s NodePermissions) HasEnterpriseRule() bool {
	return len(s.Constraints) > 0 || s.ScopeType != "" ||
		s.Inherit != nil ||
		len(s.AllowedPermissionLevelIDs) > 0 ||
		s.MinPermissionLevelID != "" ||
		s.MinPermissionLevelRank > 0 ||
		len(s.AllowedDepartmentIDs) > 0 ||
		len(s.AllowedUserIDs) > 0 ||
		len(s.DeniedDepartmentIDs) > 0 ||
		len(s.DeniedUserIDs) > 0
}

func (s *NodePermissions) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid permissions type:", value))
	}
	return json.Unmarshal(bytes, s)
}

func (s *NodePermissions) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type NodeAuthGroup struct {
	ID          uint                `json:"id"`
	NodeID      string              `json:"node_id" `
	AuthGroupID int                 `json:"auth_group_id"`
	Perm        consts.NodePermName `json:"perm"`
	CreatedAt   time.Time           `json:"created_at"`
}

func (NodeAuthGroup) TableName() string {
	return "node_auth_groups"
}

type NodeGroupDetail struct {
	NodeID      string              `json:"node_id" `
	AuthGroupId int                 `json:"auth_group_id"`
	Perm        consts.NodePermName `json:"perm"`
	Name        string              `json:"name" gorm:"uniqueIndex;size:100;not null"`
	KbID        string              `gorm:"column:kb_id;not null" json:"kb_id,omitempty"`
	AuthIDs     pq.Int64Array       `json:"auth_ids" gorm:"type:int[]"`
}

type NodeMeta struct {
	Extraction   *FileExtraction `json:"extraction,omitempty"`
	Summary      string          `json:"summary"`
	Emoji        string          `json:"emoji"`
	ContentType  string          `json:"content_type"`
	FileKey      string          `json:"file_key,omitempty"`
	FileName     string          `json:"file_name,omitempty"`
	FileType     string          `json:"file_type,omitempty"`
	PreviewKey   string          `json:"preview_key,omitempty"`
	FileReadonly bool            `json:"file_readonly,omitempty"`
	// ArchivePending marks a ZIP source that is uploaded but intentionally not
	// expanded until the user publishes it.
	ArchivePending bool                 `json:"archive_pending,omitempty"`
	VersionKey     string               `json:"version_key,omitempty"`
	VersionFormat  string               `json:"version_format,omitempty"`
	Attachments    []NodeAttachmentMeta `json:"attachments,omitempty"`
}

type NodeAttachmentMeta struct {
	FileKey    string `json:"file_key,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileType   string `json:"file_type,omitempty"`
	PreviewKey string `json:"preview_key,omitempty"`
}

func (d *NodeMeta) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d *NodeMeta) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("invalid node meta type:", value))
	}
	return json.Unmarshal(bytes, d)
}

type CreateNodeReq struct {
	KBID          string    `json:"kb_id" validate:"required"`
	NavId         string    `json:"nav_id" validate:"required"`
	ParentID      string    `json:"parent_id"`
	Type          NodeType  `json:"type" validate:"required,oneof=1 2"`
	Name          string    `json:"name" validate:"required"`
	Content       string    `json:"content"`
	EditorContent JSONB     `json:"editor_content"`
	EditorBlocks  JSONB     `json:"editor_blocks"`
	EditorFormat  string    `json:"editor_format"`
	EditorVersion int       `json:"editor_version"`
	Meta          *NodeMeta `json:"meta"`
	Emoji         string    `json:"emoji"`
	Summary       *string   `json:"summary"`
	ContentType   *string   `json:"content_type"`
	Position      *float64  `json:"position"`
}

type GetNodeListReq struct {
	KBID   string `json:"kb_id" query:"kb_id" validate:"required"`
	NavId  string `query:"nav_id" json:"nav_id"`
	Search string `json:"search" query:"search"`
}

type NodeListItemResp struct {
	CanCreateDocument bool            `json:"can_create_document" gorm:"-"`
	CanCreateFolder   bool            `json:"can_create_folder" gorm:"-"`
	CanManage         bool            `json:"can_manage" gorm:"-"`
	ID                string          `json:"id"`
	NavId             string          `json:"nav_id"`
	Type              NodeType        `json:"type"`
	Status            NodeStatus      `json:"status"`
	RagInfo           RagInfo         `json:"rag_info"`
	Name              string          `json:"name"`
	Summary           string          `json:"summary"`
	Emoji             string          `json:"emoji"`
	ContentType       string          `json:"content_type"`
	Position          float64         `json:"position"`
	ParentID          string          `json:"parent_id"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	CreatorId         string          `json:"creator_id"`
	EditorId          string          `json:"editor_id"`
	Creator           string          `json:"creator"`
	Editor            string          `json:"editor"`
	PublisherId       string          `json:"publisher_id" gorm:"-"`
	Permissions       NodePermissions `json:"permissions" gorm:"type:jsonb"`
}

type NodeContentChunk struct {
	ID    string `json:"id"`
	KBID  string `json:"kb_id"`
	DocID string `json:"doc_id"`

	Seq     uint   `json:"seq"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type RankedNodeChunks struct {
	NodeID        string
	NodeName      string
	NodeSummary   string
	NodeEmoji     string
	NodePathNames []string
	Chunks        []*NodeContentChunk
}

func (n *RankedNodeChunks) GetURL(baseURL string) string {
	return fmt.Sprintf("%s/node/%s", baseURL, n.NodeID)
}

type ChunkListItemResp struct {
	ID      string `json:"id"`
	Seq     uint   `json:"seq"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type NodeContentChunkSSE struct {
	NodeID        string   `json:"node_id"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Emoji         string   `json:"emoji"`
	NodePathNames []string `json:"node_path_names"`
}

type RecommendNodeListResp struct {
	ID             string                   `json:"id"`
	NavId          string                   `json:"nav_id"`
	NavName        string                   `json:"nav_name"`
	Name           string                   `json:"name"`
	Type           NodeType                 `json:"type"`
	Summary        string                   `json:"summary"`
	ParentID       string                   `json:"parent_id"`
	Position       float64                  `json:"position"`
	Emoji          string                   `json:"emoji"`
	RecommendNodes []*RecommendNodeListResp `json:"recommend_nodes,omitempty" gorm:"-"`
	Permissions    NodePermissions          `json:"permissions" gorm:"type:jsonb"`
}

type NodeActionReq struct {
	IDs    []string `json:"ids" validate:"required"`
	KBID   string   `json:"kb_id" validate:"required"`
	Action string   `json:"action" validate:"required,oneof=delete"`
}

type UpdateNodeReq struct {
	ID            string    `json:"id" validate:"required"`
	KBID          string    `json:"kb_id" validate:"required"`
	Name          *string   `json:"name"`
	Content       *string   `json:"content"`
	EditorContent *JSONB    `json:"editor_content"`
	EditorBlocks  *JSONB    `json:"editor_blocks"`
	EditorFormat  *string   `json:"editor_format"`
	EditorVersion *int      `json:"editor_version"`
	Meta          *NodeMeta `json:"meta"`
	Emoji         *string   `json:"emoji"`
	Summary       *string   `json:"summary"`
	Position      *float64  `json:"position"`
	ContentType   *string   `json:"content_type"`
	NavId         *string   `json:"nav_id"`
}

type ShareNodeListItemResp struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        NodeType        `json:"type"`
	ParentID    string          `json:"parent_id"`
	NavId       string          `json:"nav_id"`
	Position    float64         `json:"position"`
	Emoji       string          `json:"emoji"`
	Meta        NodeMeta        `json:"meta"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Permissions NodePermissions `json:"permissions" gorm:"type:jsonb"`
}

type ShareNodeDetailItem struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        NodeType               `json:"type"`
	ParentID    string                 `json:"parent_id"`
	Position    float64                `json:"position"`
	Emoji       string                 `json:"emoji"`
	Meta        NodeMeta               `json:"meta"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Permissions NodePermissions        `json:"permissions" gorm:"type:jsonb"`
	Children    []*ShareNodeDetailItem `json:"children,omitempty"`
}

func (n *ShareNodeListItemResp) GetURL(baseURL string) string {
	return fmt.Sprintf("%s/node/%s", baseURL, n.ID)
}

type MoveNodeReq struct {
	ID       string `json:"id" validate:"required"`
	KbID     string `json:"kb_id" validate:"required"`
	ParentID string `json:"parent_id"`
	PrevID   string `json:"prev_id"`
	NextID   string `json:"next_id"`
}

type NodeSummaryReq struct {
	IDs  []string `json:"ids" validate:"required"`
	KBID string   `json:"kb_id" validate:"required"`
}

type GetRecommendNodeListReq struct {
	KBID    string   `json:"kb_id" validate:"required" query:"kb_id"`
	NodeIDs []string `json:"node_ids" query:"node_ids[]"`
	NavIds  []string `json:"nav_ids" query:"nav_ids[]"`
}

// table: node_releases
type NodeRelease struct {
	ID          string `json:"id" gorm:"primaryKey"`
	KBID        string `json:"kb_id" gorm:"index"`
	PublisherId string `json:"publisher_id"`
	EditorId    string `json:"editor_id"`
	NodeID      string `json:"node_id" gorm:"index"`
	DocID       string `json:"doc_id" gorm:"index"` // for rag service

	Type NodeType `json:"type"`

	Name    string   `json:"name"`
	Meta    NodeMeta `json:"meta" gorm:"type:jsonb"`
	Content string   `json:"content"`

	EditorContent JSONB  `json:"editor_content" gorm:"type:jsonb"`
	EditorBlocks  JSONB  `json:"editor_blocks" gorm:"type:jsonb"`
	EditorFormat  string `json:"editor_format"`
	EditorVersion int    `json:"editor_version"`

	Position float64 `json:"position"`
	ParentID string  `json:"parent_id"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (NodeRelease) TableName() string {
	return "node_releases"
}

// table: node_learning_tasks
type NodeLearningTask struct {
	ID            string                        `json:"id" gorm:"primaryKey"`
	KBID          string                        `json:"kb_id" gorm:"index"`
	NodeID        string                        `json:"node_id" gorm:"index"`
	NodeReleaseID string                        `json:"node_release_id" gorm:"index"`
	Action        consts.NodeLearningTaskAction `json:"action"`
	Status        consts.NodeLearningTaskStatus `json:"status" gorm:"index"`
	Priority      int                           `json:"priority" gorm:"index"`
	DocID         string                        `json:"doc_id" gorm:"index"`
	ErrorMessage  string                        `json:"error_message"`
	TriggeredBy   string                        `json:"triggered_by"`
	StartedAt     *time.Time                    `json:"started_at"`
	FinishedAt    *time.Time                    `json:"finished_at"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

func (NodeLearningTask) TableName() string {
	return "node_learning_tasks"
}

// table: node_release_backup
type NodeReleaseBackup struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	KBID          string    `json:"kb_id" gorm:"index"`
	PublisherId   string    `json:"publisher_id"`
	EditorId      string    `json:"editor_id"`
	NodeID        string    `json:"node_id" gorm:"index"`
	DocID         string    `json:"doc_id"`
	Type          NodeType  `json:"type"`
	Name          string    `json:"name"`
	Meta          NodeMeta  `json:"meta" gorm:"type:jsonb"`
	Content       string    `json:"content"`
	EditorContent JSONB     `json:"editor_content" gorm:"type:jsonb"`
	EditorBlocks  JSONB     `json:"editor_blocks" gorm:"type:jsonb"`
	EditorFormat  string    `json:"editor_format"`
	EditorVersion int       `json:"editor_version"`
	Position      float64   `json:"position"`
	ParentID      string    `json:"parent_id"`
	DeletedAt     time.Time `json:"deleted_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (NodeReleaseBackup) TableName() string {
	return "node_release_backup"
}

// NodeReleaseWithDirPath extends NodeRelease with directory path information
type NodeReleaseWithDirPath struct {
	*NodeRelease
	Path string `json:"path"`
}

type BatchMoveReq struct {
	IDs      []string `json:"ids" validate:"required"`
	KBID     string   `json:"kb_id" validate:"required"`
	ParentID string   `json:"parent_id"`
}

type NodeCreateInfo struct {
	ID        string `json:"id"`
	Account   string `json:"account"`
	CreatorId string `json:"creator_id"`
}

type NodeReleaseWithPublisher struct {
	ID               string `json:"id" gorm:"primaryKey"`
	PublisherId      string `json:"publisher_id"`
	PublisherAccount string `json:"publisher_account"`
}
