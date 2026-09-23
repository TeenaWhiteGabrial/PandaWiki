package domain

import "time"

const (
	ZipUploadWaiting    = "waiting"
	ZipUploadUploading  = "uploading"
	ZipUploadProcessing = "processing"
	ZipUploadCompleted  = "completed"
	ZipUploadFailed     = "failed"
)

type ZipUploadTask struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	KBID           string    `json:"kb_id" gorm:"index"`
	NavID          string    `json:"nav_id"`
	ParentID       string    `json:"parent_id"`
	CreatedBy      string    `json:"created_by"`
	Filename       string    `json:"filename"`
	FileKey        string    `json:"file_key"`
	Size           int64     `json:"size"`
	PartSize       int64     `json:"part_size"`
	TotalParts     int       `json:"total_parts"`
	UploadID       string    `json:"-"`
	UploadedParts  string    `json:"uploaded_parts" gorm:"type:jsonb"`
	UploadedBytes  int64     `json:"uploaded_bytes"`
	TotalFiles     int       `json:"total_files"`
	ProcessedFiles int       `json:"processed_files"`
	TotalBytes     int64     `json:"total_bytes"`
	ProcessedBytes int64     `json:"processed_bytes"`
	Status         string    `json:"status"`
	RetryCount     int       `json:"retry_count"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	NodeID         string    `json:"node_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (ZipUploadTask) TableName() string { return "zip_upload_tasks" }
