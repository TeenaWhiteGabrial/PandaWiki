package domain

import "time"

const (
	FolderDownloadWaiting    = "waiting"
	FolderDownloadProcessing = "processing"
	FolderDownloadCompleted  = "completed"
	FolderDownloadFailed     = "failed"
	FolderDownloadExpired    = "expired"
)

type FolderDownloadTask struct {
	ID             string     `json:"id" gorm:"primaryKey"`
	KBID           string     `json:"kb_id" gorm:"index"`
	NodeID         string     `json:"node_id"`
	CreatedBy      string     `json:"created_by"`
	Status         string     `json:"status"`
	TotalFiles     int        `json:"total_files"`
	ProcessedFiles int        `json:"processed_files"`
	TotalBytes     int64      `json:"total_bytes"`
	ProcessedBytes int64      `json:"processed_bytes"`
	RetryCount     int        `json:"retry_count"`
	ObjectKey      string     `json:"object_key"`
	ErrorMessage   string     `json:"error_message"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (FolderDownloadTask) TableName() string { return "folder_download_tasks" }
