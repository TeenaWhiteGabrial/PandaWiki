package domain

import "time"

// Assistant resources never expose object keys, prompts or intermediate model output.
type AssistantUpload struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	KBID        string     `json:"-"`
	UserID      string     `json:"-"`
	Name        string     `json:"name"`
	Format      string     `json:"format"`
	Size        int64      `json:"size"`
	Hash        string     `json:"-"`
	ObjectKey   string     `json:"-"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	Report      JSONB      `json:"report,omitempty" gorm:"type:jsonb"`
	Text        string     `json:"-"`
	TemplateKey string     `json:"-"`
	PreviewKey  string     `json:"-"`
	LeaseUntil  *time.Time `json:"-"`
	LeaseEpoch  int64      `json:"-"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AssistantTemplate struct {
	Confirmed   bool            `json:"confirmed" gorm:"-"`
	ID          string          `json:"id" gorm:"primaryKey"`
	KBID        string          `json:"-"`
	Name        string          `json:"name"`
	UploadID    string          `json:"upload_id"`
	Version     int             `json:"version" gorm:"primaryKey"`
	State       string          `json:"state"`
	IsDefault   bool            `json:"is_default"`
	Permissions NodePermissions `json:"permissions" gorm:"type:jsonb"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type AssistantTemplateRef struct {
	Kind    string `json:"kind"` // enterprise or upload
	ID      string `json:"id"`
	Version int    `json:"version,omitempty"`
}
type AssistantScope struct {
	Type       string            `json:"type"` // all or documents; folders are expanded before acceptance
	NodeIDs    []string          `json:"node_ids,omitempty"`
	ReleaseIDs map[string]string `json:"release_ids,omitempty"`
}
type AssistantSubmit struct {
	ClientRequestID   string                `json:"client_request_id"`
	ConversationID    string                `json:"conversation_id"`
	Message           string                `json:"message"`
	AttachmentIDs     []string              `json:"attachment_ids"`
	TemplateRef       *AssistantTemplateRef `json:"template_ref,omitempty"`
	BaseArtifactID    string                `json:"base_artifact_id,omitempty"`
	TargetIDs         []string              `json:"target_ids,omitempty"`
	WebSearchEnabled  bool                  `json:"web_search_enabled"`
	PublicSearchQuery string                `json:"public_search_query,omitempty"`
	SourceScope       AssistantScope        `json:"source_scope"`
	Format            string                `json:"format,omitempty"` // optional explicit intent, otherwise planned by model
}

type AssistantSource struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Hash    string `json:"hash,omitempty"`
}
type AssistantRun struct {
	Artifact           *AssistantArtifact           `json:"artifact,omitempty" gorm:"-"`
	Generation         *AssistantGenerationProgress `json:"generation,omitempty" gorm:"-"`
	Uploads            []AssistantUpload            `json:"uploads,omitempty" gorm:"-"`
	SessionNonce       string                       `json:"nonce,omitempty" gorm:"-"`
	Question           string                       `json:"question" gorm:"-"`
	References         []AssistantSource            `json:"sources,omitempty" gorm:"-"`
	Planning           *AssistantPlan               `json:"planning,omitempty" gorm:"-"`
	ID                 string                       `json:"id" gorm:"primaryKey"`
	KBID               string                       `json:"-"`
	UserID             string                       `json:"-"`
	ConversationID     string                       `json:"conversation_id"`
	UserMessageID      string                       `json:"user_message_id"`
	AssistantMessageID string                       `json:"assistant_message_id"`
	RequestID          string                       `json:"-"`
	RequestHash        string                       `json:"-"`
	Request            JSONB                        `json:"-" gorm:"type:jsonb"`
	State              string                       `json:"state"`
	Stage              string                       `json:"stage"`
	Error              string                       `json:"error,omitempty"`
	Answer             string                       `json:"answer,omitempty"`
	Format             string                       `json:"format,omitempty"`
	Checkpoint         JSONB                        `json:"-" gorm:"type:jsonb"`
	Sources            JSONB                        `json:"-" gorm:"type:jsonb"`
	Steps              JSONB                        `json:"steps" gorm:"type:jsonb"`
	Attempt            int                          `json:"attempt"`
	LeaseEpoch         int64                        `json:"-"`
	LeaseUntil         *time.Time                   `json:"-"`
	ArtifactID         string                       `json:"artifact_id,omitempty"`
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
	StageStartedAt     time.Time                    `json:"stage_started_at"`
}

type AssistantArtifactPart struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	Unit     string `json:"unit"`
}

type AssistantArtifact struct {
	Outline        []AssistantArtifactPart `json:"outline,omitempty" gorm:"-"`
	ID             string                  `json:"id" gorm:"primaryKey"`
	RunID          string                  `json:"run_id"`
	KBID           string                  `json:"-"`
	UserID         string                  `json:"-"`
	ConversationID string                  `json:"conversation_id"`
	FamilyID       string                  `json:"family_id"`
	Revision       int                     `json:"revision"`
	BaseID         string                  `json:"base_id,omitempty"`
	Name           string                  `json:"name"`
	Format         string                  `json:"format"`
	Size           int64                   `json:"size"`
	Hash           string                  `json:"-"`
	ObjectKey      string                  `json:"-"`
	PreviewKey     string                  `json:"-"`
	PreviewState   string                  `json:"preview_state"`
	Spec           JSONB                   `json:"-" gorm:"type:jsonb"`
	Sources        JSONB                   `json:"-" gorm:"type:jsonb"`
	ExpiresAt      time.Time               `json:"expires_at"`
	CreatedAt      time.Time               `json:"created_at"`
}

type AssistantPreference struct {
	ConversationID string    `json:"conversation_id" gorm:"primaryKey"`
	UserID         string    `json:"-"`
	KBID           string    `json:"-"`
	Settings       JSONB     `json:"settings" gorm:"type:jsonb"`
	UpdatedAt      time.Time `json:"updated_at"`
}
