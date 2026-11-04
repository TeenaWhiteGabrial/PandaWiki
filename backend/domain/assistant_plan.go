package domain

type AssistantGenerationPart struct {
	Position  int    `json:"position,omitempty"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview,omitempty"`
	Completed bool   `json:"completed"`
}

type AssistantGenerationProgress struct {
	Completed    int                       `json:"completed"`
	Total        int                       `json:"total"`
	Unit         string                    `json:"unit"`
	CurrentTitle string                    `json:"current_title,omitempty"`
	Parts        []AssistantGenerationPart `json:"parts"`
}

type AssistantPlanQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Default  string   `json:"default,omitempty"`
}

type AssistantOutlineItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Purpose   string   `json:"purpose"`
	SourceIDs []string `json:"source_ids"`
}

type AssistantMaterialReview struct {
	TotalCharacters int    `json:"total_characters,omitempty"`
	ReadMode        string `json:"read_mode,omitempty"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Characters      int    `json:"characters"`
}

type AssistantPlan struct {
	Revision      int64                     `json:"revision"`
	Format        string                    `json:"format"`
	Title         string                    `json:"title"`
	Summary       string                    `json:"summary"`
	Brief         map[string]string         `json:"brief"`
	Questions     []AssistantPlanQuestion   `json:"questions"`
	Sufficient    bool                      `json:"sufficient"`
	Gaps          []string                  `json:"gaps"`
	Outline       []AssistantOutlineItem    `json:"outline"`
	Materials     []AssistantMaterialReview `json:"materials"`
	FrameworkOnly bool                      `json:"framework_only"`
}

type AssistantContinue struct {
	ClientRequestID string                 `json:"client_request_id"`
	Revision        int64                  `json:"revision"`
	Action          string                 `json:"action"` // answer, confirm_outline, revise, supplement, framework
	Answers         map[string]string      `json:"answers"`
	UseDefaults     bool                   `json:"use_defaults"`
	Message         string                 `json:"message"`
	Outline         []AssistantOutlineItem `json:"outline"`
	AttachmentIDs   []string               `json:"attachment_ids"`
	NodeIDs         []string               `json:"node_ids"`
}
