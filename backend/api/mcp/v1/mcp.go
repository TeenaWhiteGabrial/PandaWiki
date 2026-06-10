package v1

import "encoding/json"

const DefaultProtocolVersion = "2025-03-26"

type ToolListReq struct {
	KbID string `json:"kb_id" query:"kb_id"`
}

type ToolCallReq struct {
	KbID      string          `json:"kb_id"`
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name" validate:"required"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResp struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

type ToolCallStatus string

const (
	ToolCallStatusSuccess    ToolCallStatus = "success"
	ToolCallStatusNeedsInput ToolCallStatus = "needs_input"
)

type DirectoryCandidate struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NavID           string `json:"nav_id"`
	NavName         string `json:"nav_name"`
	Path            string `json:"path"`
	RecommendReason string `json:"recommend_reason,omitempty"`
}

type ToolCallResp struct {
	Status                ToolCallStatus       `json:"status"`
	ToolName              string               `json:"tool_name"`
	Message               string               `json:"message,omitempty"`
	MissingFields         []string             `json:"missing_fields,omitempty"`
	Candidates            []DirectoryCandidate `json:"candidates,omitempty"`
	RecommendedCandidates []DirectoryCandidate `json:"recommended_candidates,omitempty"`
	Result                interface{}          `json:"result,omitempty"`
}

type UploadDocArgs struct {
	FileName          string `json:"file_name"`
	FileContentBase64 string `json:"file_content_base64"`
	DirectoryID       string `json:"directory_id"`
	NavID             string `json:"nav_id"`
}

type UploadDocResult struct {
	ID          string `json:"id"`
	KBID        string `json:"kb_id"`
	NavID       string `json:"nav_id"`
	DirectoryID string `json:"directory_id"`
	FileName    string `json:"file_name"`
	Title       string `json:"title"`
}

type PublishDocArgs struct {
	NodeIDs []string `json:"node_ids"`
}

type PublishDocResult struct {
	KBID      string   `json:"kb_id"`
	NodeIDs   []string `json:"node_ids"`
	ReleaseID string   `json:"release_id,omitempty"`
}

type StudyDocArgs struct {
	NodeIDs []string `json:"node_ids"`
}

type StudyDocResult struct {
	KBID    string   `json:"kb_id"`
	NodeIDs []string `json:"node_ids"`
}

type SearchDocsArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type SearchDocItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NavID   string `json:"nav_id"`
	NavName string `json:"nav_name,omitempty"`
	Path    string `json:"path"`
	Summary string `json:"summary,omitempty"`
	URI     string `json:"uri"`
}

type SearchDocsResult struct {
	KBID  string          `json:"kb_id"`
	Query string          `json:"query"`
	Items []SearchDocItem `json:"items"`
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      ClientInfo `json:"clientInfo"`
	KbID            string     `json:"kb_id,omitempty"`
	SessionID       string     `json:"session_id,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	ServerInfo      ServerInfo    `json:"serverInfo"`
	Capabilities    MCPServerCaps `json:"capabilities"`
	Instructions    string        `json:"instructions,omitempty"`
}

type MCPServerCaps struct {
	Tools     map[string]interface{} `json:"tools,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty"`
}

type ToolListResult struct {
	Tools []ToolResp `json:"tools"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content           []ToolContent `json:"content"`
	StructuredContent interface{}   `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

type ResourceListParams struct {
	KbID string `json:"kb_id,omitempty"`
}

type ResourceReadParams struct {
	KbID string `json:"kb_id,omitempty"`
	URI  string `json:"uri"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ResourceListResult struct {
	Resources []Resource `json:"resources"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
}

type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}
