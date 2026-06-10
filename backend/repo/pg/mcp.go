package pg

import (
	"context"
	"encoding/json"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type MCPRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewMCPRepository(db *pg.DB, logger *log.Logger) *MCPRepository {
	return &MCPRepository{db: db, logger: logger}
}

func (r *MCPRepository) GetMCPCallCount(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("mcp_calls").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MCPRepository) RecordToolCall(ctx context.Context, call *domain.MCPCall) error {
	if call == nil {
		return nil
	}
	record := map[string]interface{}{
		"mcp_session_id": call.MCPSessionID,
		"kb_id":          call.KBID,
		"remote_ip":      call.RemoteIP,
		"tool_call_req":  json.RawMessage(call.ToolCallReq),
		"tool_call_resp": call.ToolCallResp,
	}
	if len(call.InitializeReq) > 0 {
		record["initialize_req"] = json.RawMessage(call.InitializeReq)
	}
	if len(call.InitializeResp) > 0 {
		record["initialize_resp"] = json.RawMessage(call.InitializeResp)
	}
	return r.db.WithContext(ctx).Table("mcp_calls").Create(record).Error
}
