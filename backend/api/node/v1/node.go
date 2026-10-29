package v1

import (
	"time"

	"github.com/chaitin/panda-wiki/domain"
)

type GetNodeDetailReq struct {
	KbId   string `query:"kb_id" json:"kb_id" validate:"required"`
	ID     string `query:"id" json:"id" validate:"required"`
	Format string `query:"format" json:"format"`
}

type NodeDetailResp struct {
	CanManage        bool                   `json:"can_manage" gorm:"-"`
	ID               string                 `json:"id"`
	KbID             string                 `json:"kb_id"`
	NavId            string                 `json:"nav_id"`
	Type             domain.NodeType        `json:"type"`
	Status           domain.NodeStatus      `json:"status"`
	Name             string                 `json:"name"`
	Content          string                 `json:"content"`
	EditorContent    domain.JSONB           `json:"editor_content,omitempty"`
	EditorBlocks     domain.JSONB           `json:"editor_blocks,omitempty"`
	EditorFormat     string                 `json:"editor_format,omitempty"`
	EditorVersion    int                    `json:"editor_version,omitempty"`
	Meta             domain.NodeMeta        `json:"meta"`
	ParentID         string                 `json:"parent_id"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	Permissions      domain.NodePermissions `json:"permissions"`
	CreatorId        string                 `json:"creator_id"`
	EditorId         string                 `json:"editor_id"`
	PublisherId      string                 `json:"publisher_id" gorm:"-"`
	CreatorAccount   string                 `json:"creator_account"`
	EditorAccount    string                 `json:"editor_account"`
	PublisherAccount string                 `json:"publisher_account" gorm:"-"`
	PV               int64                  `json:"pv" gorm:"-"`
}

type NodePermissionReq struct {
	KbId string `query:"kb_id" json:"kb_id" validate:"required"`
	ID   string `query:"id" json:"id" validate:"required"`
}

type NodePermissionResp struct {
	ID                   string                 `json:"id"`
	Permissions          domain.NodePermissions `json:"permissions"`
	EffectivePermissions domain.NodePermissions `json:"effective_permissions"`
}

type NodePermissionEditReq struct {
	KbId        string                  `query:"kb_id" json:"kb_id" validate:"required"`
	IDs         []string                `query:"ids" json:"ids" validate:"required"`
	Permissions *domain.NodePermissions `json:"permissions"`
}

type NodePermissionEditResp struct {
}

type NodeRestudyReq struct {
	NodeIds []string `json:"node_ids" validate:"required,min=1"`
	KbId    string   `json:"kb_id" validate:"required"`
	// Priority controls queue ordering. Interactive requests may use a higher
	// value so a manually selected document is not blocked by a bulk rebuild.
	Priority int    `json:"priority,omitempty"`
	UserID   string `json:"-"`
}

type NodeRestudyResp struct {
}

type NodeLearningCancelReq struct {
	NodeIds []string `json:"node_ids" validate:"required,min=1"`
	KbId    string   `json:"kb_id" validate:"required"`
	UserID  string   `json:"-"`
}

type NodeLearningCancelResp struct {
	Count int `json:"count"`
}

type NodeLearningCancelPendingReq struct {
	KbId string `json:"kb_id" validate:"required"`
}

type NodeLearningCancelPendingResp struct {
	Count          int    `json:"count"`
	ProtectedCount int    `json:"protected_count"`
	LegacyCutoff   uint64 `json:"legacy_cutoff"`
}

type NodeStatsReq struct {
	KbId string `query:"kb_id" json:"kb_id" validate:"required"`
}

type NodeStatsResp struct {
	UnpublishedCount   int64 `json:"unpublished_count"`    // 未发布的文档数
	UnstudiedCount     int64 `json:"unstudied_count"`      // 未学习的文档数
	UnreleasedNavCount int64 `json:"unreleased_nav_count"` // 未发布目录数量
}

type NodeMoveNavReq struct {
	IDs   []string `json:"ids" query:"[]ids" validate:"required,min=1"`
	KbID  string   `json:"kb_id" validate:"required"`
	NavID string   `json:"nav_id" validate:"required"`
}

type NodeListGroupNavReq struct {
	Tree     bool     `query:"tree"`
	NavID    string   `query:"nav_id" json:"nav_id"`
	ParentID string   `query:"parent_id" json:"parent_id"`
	PathID   string   `query:"path_id" json:"path_id"`
	Parents  []string `query:"parents[]" json:"parents" validate:"max=5000"`
	KbId     string   `json:"kb_id" query:"kb_id" validate:"required"`
	NavIds   []string `json:"nav_ids" query:"nav_ids[]"`
	Search   string   `json:"search" query:"search"`
	Status   string   `json:"status" query:"status" validate:"omitempty,oneof=released unpublished unstudied"`
}

type NodeListGroupNavResp struct {
	EntryNodeID       string                    `json:"entry_node_id,omitempty"`
	TreeRevision      string                    `json:"tree_revision,omitempty"`
	HasChildren       bool                      `json:"has_children"`
	LoadedParents     []string                  `json:"loaded_parents,omitempty"`
	CanCreateDocument bool                      `json:"can_create_document"`
	CanCreateFolder   bool                      `json:"can_create_folder"`
	CanConfigure      bool                      `json:"can_configure"`
	CanManage         bool                      `json:"can_manage"`
	Permissions       domain.NodePermissions    `json:"permissions"`
	NavName           string                    `json:"nav_name"`
	NavID             string                    `json:"nav_id"`
	Position          float64                   `json:"position"`
	Count             int64                     `json:"count"`
	IsReleased        bool                      `json:"is_released"`
	List              []domain.NodeListItemResp `json:"list"`
}

type GetNodeReleaseListReq struct {
	KbID   string `query:"kb_id" json:"kb_id" validate:"required"`
	NodeID string `query:"node_id" json:"node_id" validate:"required"`
}

type GetNodeReleaseDetailReq struct {
	KbID string `query:"kb_id" json:"kb_id" validate:"required"`
	ID   string `query:"id" json:"id" validate:"required"`
}

type RestoreNodeReleaseReq struct {
	KbID string `json:"kb_id" validate:"required"`
	ID   string `json:"id" validate:"required"`
}

type NodeReleaseListItem struct {
	ID               string          `json:"id"`
	NodeID           string          `json:"node_id"`
	Name             string          `json:"name"`
	Meta             domain.NodeMeta `json:"meta"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ReleaseID        string          `json:"release_id"`
	ReleaseName      string          `json:"release_name"`
	ReleaseMessage   string          `json:"release_message"`
	PublisherID      string          `json:"publisher_id"`
	PublisherAccount string          `json:"publisher_account"`
	EditorID         string          `json:"editor_id"`
	EditorAccount    string          `json:"editor_account"`
	CreatorID        string          `json:"creator_id"`
	CreatorAccount   string          `json:"creator_account"`
}

type GetNodeReleaseDetailResp struct {
	NodeID           string          `json:"node_id"`
	Name             string          `json:"name"`
	Content          string          `json:"content"`
	EditorContent    domain.JSONB    `json:"editor_content,omitempty"`
	EditorBlocks     domain.JSONB    `json:"editor_blocks,omitempty"`
	EditorFormat     string          `json:"editor_format,omitempty"`
	EditorVersion    int             `json:"editor_version,omitempty"`
	Meta             domain.NodeMeta `json:"meta"`
	PublisherID      string          `json:"publisher_id"`
	PublisherAccount string          `json:"publisher_account"`
	EditorID         string          `json:"editor_id"`
	EditorAccount    string          `json:"editor_account"`
	CreatorID        string          `json:"creator_id"`
	CreatorAccount   string          `json:"creator_account"`
}

// NodeManagementEditReq configures management independently of reading rules.
type NodeManagementEditReq struct {
	KBID   string                           `json:"kb_id" validate:"required"`
	IDs    []string                         `json:"ids" validate:"required,min=1"`
	Grants []domain.DocumentManagementGrant `json:"grants"`
}

type NodeManagementResp struct {
	ID                     string                             `json:"id"`
	Grants                 []domain.DocumentManagementGrant   `json:"grants"`
	ParentGrantConstraints [][]domain.DocumentManagementGrant `json:"parent_grant_constraints,omitempty"`
	HasParentFolder        bool                               `json:"has_parent_folder"`
	CanConfigure           bool                               `json:"can_configure"`
}
