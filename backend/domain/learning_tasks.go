package domain

import "time"

// LearningTaskQuery applies to published documents, including documents without
// a registered task. Pagination is performed by PostgreSQL, never by the client.
type LearningTaskQuery struct {
	KBID        string `query:"kb_id" json:"kb_id" validate:"required"`
	Filter      string `query:"filter" json:"filter" validate:"omitempty,oneof=attention running failed unavailable answerable all needs_learning"`
	Keyword     string `query:"keyword" json:"keyword" validate:"max=200"`
	Page        int    `query:"page" json:"page" validate:"min=0,max=1000000"`
	PageSize    int    `query:"page_size" json:"page_size" validate:"min=0,max=100"`
	OnlyRunning bool   `query:"only_running" json:"only_running"`
}

type LearningTaskItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       NodeType   `json:"type"`
	Status     NodeStatus `json:"status"`
	ParentID   string     `json:"parent_id"`
	NavID      string     `json:"nav_id"`
	FolderPath string     `json:"folder_path" gorm:"-"`
	UpdatedAt  time.Time  `json:"updated_at"`
	RagInfo    RagInfo    `json:"rag_info" gorm:"type:jsonb"`
}

type LearningTaskCounts struct {
	Total         int64 `json:"total"`
	Attention     int64 `json:"attention"`
	Running       int64 `json:"running"`
	Failed        int64 `json:"failed"`
	Unavailable   int64 `json:"unavailable"`
	Answerable    int64 `json:"answerable"`
	NeedsLearning int64 `json:"needs_learning"`
}

type LearningTaskPage struct {
	List     []LearningTaskItem `json:"list"`
	Total    int64              `json:"total"`
	Running  int64              `json:"running"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}
