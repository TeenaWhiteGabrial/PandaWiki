package domain

import "time"

const (
	IndexBuilding = "building"
	IndexReady    = "ready"
	IndexActive   = "active"
	IndexFailed   = "failed"
	IndexCanceled = "canceled"
	IndexRetired  = "retired"
	IndexDeleted  = "deleted"
)

// A candidate never overwrites the document currently serving retrieval.
// Its release.doc_id and the active pointer are committed only after SUCCEEDED.
type NodeIndexVersion struct {
	ParsedContent *string
	ParsedMeta    NodeMeta `gorm:"type:jsonb"`
	ID            string   `gorm:"primaryKey"`
	KBID          string
	NodeID        string
	NodeReleaseID string
	DatasetID     string
	DocID         string
	TaskID        string
	BuildID       string
	Status        string
	Message       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (NodeIndexVersion) TableName() string { return "node_index_versions" }

// A model change stages a complete, separately model-bound dataset. The live
// KB keeps its original dataset until every snapshot document has succeeded.
type KBIndexBuild struct {
	ID              string `gorm:"primaryKey"`
	KBID            string
	SourceDatasetID string
	DatasetID       string
	ConfigKey       string
	Status          string
	Message         string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (KBIndexBuild) TableName() string { return "kb_index_builds" }
