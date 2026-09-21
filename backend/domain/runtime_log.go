package domain

import "time"

type RuntimeLog struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	EventTime   time.Time `json:"event_time"`
	Level       string    `json:"level"`
	Service     string    `json:"service"`
	Module      string    `json:"module"`
	Source      string    `json:"source"`
	TraceID     string    `json:"trace_id"`
	SpanID      string    `json:"span_id"`
	RequestID   string    `json:"request_id"`
	KBID        *string   `json:"kb_id,omitempty"`
	DocumentID  *string   `json:"document_id,omitempty"`
	RevisionID  *string   `json:"revision_id,omitempty"`
	JobID       *string   `json:"job_id,omitempty"`
	RunID       *string   `json:"run_id,omitempty"`
	Stage       string    `json:"stage"`
	EventCode   string    `json:"event_code"`
	DurationMS  *int64    `json:"duration_ms,omitempty"`
	SafeMessage string    `json:"safe_message"`
	SafeFields  JSONB     `json:"safe_fields" gorm:"type:jsonb"`
}

func (RuntimeLog) TableName() string { return "runtime_logs" }

type RuntimeLogListReq struct {
	Scope    string `query:"scope"`
	Level    string `query:"level"`
	Service  string `query:"service"`
	Stage    string `query:"stage"`
	Keyword  string `query:"keyword"`
	Location string `query:"location"`
	From     string `query:"from"`
	To       string `query:"to"`
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
}

type RuntimeLogListResp struct {
	Items []RuntimeLog `json:"items"`
	Total int64        `json:"total"`
}

type RawRuntimeLog struct {
	EventTime time.Time         `json:"event_time"`
	Level     string            `json:"level"`
	Service   string            `json:"service"`
	Container string            `json:"container"`
	Stream    string            `json:"stream"`
	Location  string            `json:"location"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels"`
}

type RawRuntimeLogListResp struct {
	Items     []RawRuntimeLog `json:"items"`
	Available bool            `json:"available"`
	HasMore   bool            `json:"has_more"`
}
