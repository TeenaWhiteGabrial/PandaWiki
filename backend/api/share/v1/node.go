package v1

import (
	"time"

	"github.com/chaitin/panda-wiki/domain"
)

type ShareNodeDetailResp struct {
	ID               string                        `json:"id"`
	KbID             string                        `json:"kb_id"`
	NavID            string                        `json:"nav_id"`
	Type             domain.NodeType               `json:"type"`
	PathOnly         bool                          `json:"path_only,omitempty"`
	Status           domain.NodeStatus             `json:"status"`
	Name             string                        `json:"name"`
	Content          string                        `json:"content"`
	Meta             domain.NodeMeta               `json:"meta"`
	ParentID         string                        `json:"parent_id"`
	CreatedAt        time.Time                     `json:"created_at"`
	UpdatedAt        time.Time                     `json:"updated_at"`
	Permissions      domain.NodePermissions        `json:"permissions"`
	CreatorId        string                        `json:"creator_id"`
	EditorId         string                        `json:"editor_id"`
	PublisherId      string                        `json:"publisher_id"`
	CreatorAccount   string                        `json:"creator_account"`
	EditorAccount    string                        `json:"editor_account"`
	PublisherAccount string                        `json:"publisher_account"`
	List             []*domain.ShareNodeDetailItem `json:"list" gorm:"-"`
	PV               int64                         `json:"pv" gorm:"-"`
}

type NodeListGroupNavResp struct {
	ViewerID      string                         `json:"viewer_id,omitempty"`
	TreeRevision  string                         `json:"tree_revision,omitempty"`
	HasChildren   bool                           `json:"has_children"`
	LoadedParents []string                       `json:"loaded_parents,omitempty"`
	EntryNodeID   string                         `json:"entry_node_id,omitempty"`
	NavName       string                         `json:"nav_name"`
	NavID         string                         `json:"nav_id"`
	Position      float64                        `json:"position"`
	Count         int64                          `json:"count"`
	List          []domain.ShareNodeListItemResp `json:"list"`
}
