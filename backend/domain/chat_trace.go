package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ChatTrace is visible only through the same KB-scoped admin authorization as
// conversation detail. It contains bounded excerpts of model/retrieval inputs.
type ChatTrace struct {
	TraceID            string         `json:"trace_id" gorm:"primaryKey;size:64"`
	KBID               string         `json:"kb_id" gorm:"size:64;index"`
	ConversationID     string         `json:"conversation_id" gorm:"size:64;index"`
	UserMessageID      string         `json:"user_message_id" gorm:"size:64"`
	AssistantMessageID string         `json:"assistant_message_id" gorm:"size:64;index"`
	Question           string         `json:"question" gorm:"type:text"`
	Model              string         `json:"model"`
	Status             string         `json:"status"`
	StartedAt          time.Time      `json:"started_at"`
	FinishedAt         *time.Time     `json:"finished_at,omitempty"`
	Steps              ChatTraceSteps `json:"steps" gorm:"type:jsonb;not null;default:'[]'"`
	ExpiresAt          time.Time      `json:"-" gorm:"not null;index"`
}

func (ChatTrace) TableName() string { return "chat_traces" }

type ChatTraceStep struct {
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	DurationMS int64          `json:"duration_ms"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type ChatTraceSteps []ChatTraceStep

func (s ChatTraceSteps) Value() (driver.Value, error) { return json.Marshal(s) }

func (s *ChatTraceSteps) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	case nil:
		*s = nil
		return nil
	default:
		return errors.New("invalid chat trace steps type")
	}
	return json.Unmarshal(raw, s)
}
