package domain

import "encoding/json"

type MCPCall struct {
	MCPSessionID   string
	KBID           string
	RemoteIP       string
	ToolCallReq    json.RawMessage
	ToolCallResp   string
	InitializeReq  json.RawMessage
	InitializeResp json.RawMessage
}
