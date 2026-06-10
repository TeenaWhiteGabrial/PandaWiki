package v1

import (
	"encoding/json"

	mcpV1 "github.com/chaitin/panda-wiki/api/mcp/v1"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/labstack/echo/v4"
)

type MCPHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.MCPUsecase
}

func NewMCPHandler(
	baseHandler *handler.BaseHandler,
	echo *echo.Echo,
	usecase *usecase.MCPUsecase,
	auth middleware.AuthMiddleware,
	logger *log.Logger,
) *MCPHandler {
	h := &MCPHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.mcp"),
		usecase:     usecase,
	}

	group := echo.Group("/api/v1/mcp", auth.Authorize)
	group.POST("", h.HandleJSONRPC)
	group.GET("/tools", h.ListTools)
	group.POST("/tools/call", h.CallTool)
	group.GET("/resources", h.ListResources)
	group.POST("/resources/read", h.ReadResource)
	return h
}

func (h *MCPHandler) ListTools(c echo.Context) error {
	var req mcpV1.ToolListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	kbID, err := h.usecase.ResolveKBID(c.Request().Context(), req.KbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to resolve knowledge base", err)
	}
	if err := h.usecase.EnsureKBAccess(c.Request().Context(), kbID); err != nil {
		return h.NewResponseWithError(c, "knowledge base permission denied", err)
	}
	tools, err := h.usecase.ListTools(c.Request().Context(), kbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to list mcp tools", err)
	}
	return h.NewResponseWithData(c, tools)
}

func (h *MCPHandler) CallTool(c echo.Context) error {
	var req mcpV1.ToolCallReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}
	kbID, err := h.usecase.ResolveKBID(c.Request().Context(), req.KbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to resolve knowledge base", err)
	}
	if err := h.usecase.EnsureKBAccess(c.Request().Context(), kbID); err != nil {
		return h.NewResponseWithError(c, "knowledge base permission denied", err)
	}
	req.KbID = kbID
	resp, err := h.usecase.CallTool(c.Request().Context(), kbID, &req, c.RealIP())
	if err != nil {
		return h.NewResponseWithError(c, "mcp tool call failed", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *MCPHandler) ListResources(c echo.Context) error {
	var req mcpV1.ResourceListParams
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	kbID, err := h.usecase.ResolveKBID(c.Request().Context(), req.KbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to resolve knowledge base", err)
	}
	if err := h.usecase.EnsureKBAccess(c.Request().Context(), kbID); err != nil {
		return h.NewResponseWithError(c, "knowledge base permission denied", err)
	}
	resp, err := h.usecase.ListResources(c.Request().Context(), kbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to list resources", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *MCPHandler) ReadResource(c echo.Context) error {
	var req mcpV1.ResourceReadParams
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if req.URI == "" {
		return h.NewResponseWithError(c, "uri is required", nil)
	}
	kbID, err := h.usecase.ResolveKBID(c.Request().Context(), req.KbID)
	if err != nil {
		return h.NewResponseWithError(c, "failed to resolve knowledge base", err)
	}
	if err := h.usecase.EnsureKBAccess(c.Request().Context(), kbID); err != nil {
		return h.NewResponseWithError(c, "knowledge base permission denied", err)
	}
	resp, err := h.usecase.ReadResource(c.Request().Context(), kbID, req.URI)
	if err != nil {
		return h.NewResponseWithError(c, "failed to read resource", err)
	}
	return h.NewResponseWithData(c, resp)
}

func (h *MCPHandler) HandleJSONRPC(c echo.Context) error {
	var req mcpV1.JSONRPCRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(200, mcpV1.JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &mcpV1.JSONRPCError{
				Code:    -32700,
				Message: "parse error",
			},
		})
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.JSONRPC != "2.0" {
		return c.JSON(200, mcpV1.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &mcpV1.JSONRPCError{
				Code:    -32600,
				Message: "invalid jsonrpc version",
			},
		})
	}

	resp := h.dispatchJSONRPC(c, &req)
	return c.JSON(200, resp)
}

func (h *MCPHandler) dispatchJSONRPC(c echo.Context, req *mcpV1.JSONRPCRequest) mcpV1.JSONRPCResponse {
	ctx := c.Request().Context()

	switch req.Method {
	case "initialize":
		var params mcpV1.InitializeParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return jsonRPCInvalidParams(req.ID, err)
			}
		}
		kbID, err := h.usecase.ResolveKBID(ctx, params.KbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to resolve knowledge base", err)
		}
		if err := h.usecase.EnsureKBAccess(ctx, kbID); err != nil {
			return jsonRPCForbidden(req.ID, err)
		}
		result, err := h.usecase.Initialize(ctx, kbID, &params, c.RealIP())
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to initialize mcp session", err)
		}
		return mcpV1.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}

	case "tools/list":
		var params mcpV1.ToolListReq
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return jsonRPCInvalidParams(req.ID, err)
			}
		}
		kbID, err := h.usecase.ResolveKBID(ctx, params.KbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to resolve knowledge base", err)
		}
		if err := h.usecase.EnsureKBAccess(ctx, kbID); err != nil {
			return jsonRPCForbidden(req.ID, err)
		}
		tools, err := h.usecase.ListTools(ctx, kbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to list tools", err)
		}
		return mcpV1.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpV1.ToolListResult{Tools: tools},
		}

	case "tools/call":
		var params mcpV1.ToolCallReq
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return jsonRPCInvalidParams(req.ID, err)
			}
		}
		if params.ToolName == "" {
			return jsonRPCInvalidParams(req.ID, nil)
		}
		kbID, err := h.usecase.ResolveKBID(ctx, params.KbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to resolve knowledge base", err)
		}
		if err := h.usecase.EnsureKBAccess(ctx, kbID); err != nil {
			return jsonRPCForbidden(req.ID, err)
		}
		params.KbID = kbID
		toolResp, err := h.usecase.CallTool(ctx, kbID, &params, c.RealIP())
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to call tool", err)
		}
		return mcpV1.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  buildToolCallResult(toolResp),
		}

	case "resources/list":
		var params mcpV1.ResourceListParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return jsonRPCInvalidParams(req.ID, err)
			}
		}
		kbID, err := h.usecase.ResolveKBID(ctx, params.KbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to resolve knowledge base", err)
		}
		if err := h.usecase.EnsureKBAccess(ctx, kbID); err != nil {
			return jsonRPCForbidden(req.ID, err)
		}
		result, err := h.usecase.ListResources(ctx, kbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to list resources", err)
		}
		return mcpV1.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}

	case "resources/read":
		var params mcpV1.ResourceReadParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return jsonRPCInvalidParams(req.ID, err)
			}
		}
		if params.URI == "" {
			return jsonRPCInvalidParams(req.ID, nil)
		}
		kbID, err := h.usecase.ResolveKBID(ctx, params.KbID)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to resolve knowledge base", err)
		}
		if err := h.usecase.EnsureKBAccess(ctx, kbID); err != nil {
			return jsonRPCForbidden(req.ID, err)
		}
		result, err := h.usecase.ReadResource(ctx, kbID, params.URI)
		if err != nil {
			return jsonRPCInternalError(req.ID, "failed to read resource", err)
		}
		return mcpV1.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}

	default:
		return mcpV1.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &mcpV1.JSONRPCError{
				Code:    -32601,
				Message: "method not found",
			},
		}
	}
}

func buildToolCallResult(resp *mcpV1.ToolCallResp) mcpV1.ToolCallResult {
	if resp == nil {
		return mcpV1.ToolCallResult{
			IsError: true,
			Content: []mcpV1.ToolContent{{Type: "text", Text: "empty tool response"}},
		}
	}
	text := resp.Message
	if text == "" {
		switch resp.Status {
		case mcpV1.ToolCallStatusNeedsInput:
			text = "需要补充输入参数后才能继续执行。"
		case mcpV1.ToolCallStatusSuccess:
			text = "工具执行成功。"
		default:
			text = "工具执行完成。"
		}
	}
	return mcpV1.ToolCallResult{
		Content:           []mcpV1.ToolContent{{Type: "text", Text: text}},
		StructuredContent: resp,
		IsError:           false,
	}
}

func jsonRPCInvalidParams(id interface{}, err error) mcpV1.JSONRPCResponse {
	message := "invalid params"
	if err != nil {
		message = err.Error()
	}
	return mcpV1.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpV1.JSONRPCError{
			Code:    -32602,
			Message: message,
		},
	}
}

func jsonRPCInternalError(id interface{}, message string, err error) mcpV1.JSONRPCResponse {
	respErr := &mcpV1.JSONRPCError{
		Code:    -32603,
		Message: message,
	}
	if err != nil {
		respErr.Data = err.Error()
	}
	return mcpV1.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   respErr,
	}
}

func jsonRPCForbidden(id interface{}, err error) mcpV1.JSONRPCResponse {
	respErr := &mcpV1.JSONRPCError{
		Code:    -32001,
		Message: "forbidden",
	}
	if err != nil {
		respErr.Data = err.Error()
	}
	return mcpV1.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   respErr,
	}
}
