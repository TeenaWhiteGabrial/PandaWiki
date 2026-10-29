package domain

import "time"

// LearningProgress is an observed timeline, not an estimated percentage.
// Old tasks begin at the first observation after this feature is deployed.
type LearningProgress struct {
	SubmissionStartedAt *time.Time      `json:"submission_started_at,omitempty"`
	Extraction          *FileExtraction `json:"extraction,omitempty"`
	StartedAt           time.Time       `json:"started_at"`
	FinishedAt          *time.Time      `json:"finished_at,omitempty"`
	LastProgressAt      time.Time       `json:"last_progress_at"`
	LastCheckedAt       *time.Time      `json:"last_checked_at,omitempty"`
	CheckError          string          `json:"check_error,omitempty"`
	RAGStatus           string          `json:"rag_status,omitempty"`
	Source              string          `json:"source,omitempty"`
	DocID               string          `json:"doc_id,omitempty"`
	DatasetID           string          `json:"dataset_id,omitempty"`
	Stages              []LearningStage `json:"stages"`
}

// FileExtraction describes what is actually indexed, independently of RAG status.
type FileExtraction struct {
	Scope          string   `json:"scope"`
	Parser         string   `json:"parser,omitempty"`
	PageCount      int      `json:"page_count,omitempty"`
	ExtractedPages int      `json:"extracted_pages,omitempty"`
	OCRPages       int      `json:"ocr_pages,omitempty"`
	Repaired       bool     `json:"repaired,omitempty"`
	Truncated      bool     `json:"truncated,omitempty"`
	Message        string   `json:"message,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type LearningStage struct {
	Key        string     `json:"key"`
	Message    string     `json:"message"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (p *LearningProgress) Advance(key, message string, now time.Time) {
	if p.FinishedAt != nil {
		return
	}
	if p.StartedAt.IsZero() {
		p.StartedAt = now
	}
	if len(p.Stages) > 0 {
		last := &p.Stages[len(p.Stages)-1]
		// Repeated polling must not reset the duration or look like new progress.
		if last.Key == key {
			if last.Message != message {
				last.Message = message
				p.LastProgressAt = now
			}
			return
		}
		// Delayed PENDING events cannot move an indexing document back to the queue.
		if key == "rag_pending" && last.Key == "rag_running" {
			return
		}
		last.FinishedAt = &now
	}
	p.Stages = append(p.Stages, LearningStage{Key: key, Message: message, StartedAt: now})
	// Bound storage even if a task is repeatedly delivered or reindexed.
	if len(p.Stages) > 40 {
		p.Stages = p.Stages[len(p.Stages)-40:]
	}
	p.LastProgressAt = now
}

func (p *LearningProgress) Finish(now time.Time) {
	if p.FinishedAt != nil {
		return
	}
	p.FinishedAt = &now
	p.LastProgressAt = now
	if len(p.Stages) > 0 {
		p.Stages[len(p.Stages)-1].FinishedAt = &now
	}
}
