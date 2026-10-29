package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/labstack/echo/v4"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/usecase"
)

type NodeHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.NodeUsecase
	auth    middleware.AuthMiddleware
}

func NewNodeHandler(
	baseHandler *handler.BaseHandler,
	echo *echo.Echo,
	usecase *usecase.NodeUsecase,
	auth middleware.AuthMiddleware,
	logger *log.Logger,
) *NodeHandler {
	h := &NodeHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.v1.node"),
		usecase:     usecase,
		auth:        auth,
	}

	group := echo.Group("/api/v1/node", h.auth.Authorize)
	readGroup := group.Group("", h.auth.ValidateKBUserPerm(consts.UserKBPermissionNotNull))
	readGroup.GET("/list", h.GetNodeList)
	readGroup.GET("/list/group/nav", h.NodeListGroupNav)
	readGroup.POST("/tree", h.NodeTree)
	readGroup.GET("/stats", h.NodeStats)
	readGroup.POST("/tree/status", h.TreeNodeStatuses)
	readGroup.GET("/detail", h.GetNodeDetail)
	readGroup.GET("/recommend_nodes", h.RecommendNodes)
	readGroup.GET("/permission", h.NodePermission)
	readGroup.GET("/management", h.GetNodeManagement)
	group.GET("/permission/options", h.NodePermissionOptions, h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	readGroup.GET("/release/list", h.GetNodeReleaseList)
	readGroup.GET("/release/detail", h.GetNodeReleaseDetail)

	writeGroup := group.Group("", h.auth.ValidateKBUserPerm(consts.UserKBPermissionDocManage))
	writeGroup.POST("", h.CreateNode)
	writeGroup.PUT("/detail", h.UpdateNodeDetail)
	writeGroup.POST("/summary", h.SummaryNode)
	writeGroup.POST("/summary/stream", h.SummaryNodeStream)
	writeGroup.POST("/action", h.NodeAction)
	writeGroup.POST("/move", h.MoveNode)
	writeGroup.POST("/move/nav", h.NodeMoveNav)
	writeGroup.POST("/batch_move", h.BatchMoveNode)
	writeGroup.POST("/restudy", h.NodeRestudy)
	writeGroup.POST("/learning/cancel", h.NodeLearningCancel)
	writeGroup.POST("/learning/cancel-pending", h.NodeLearningCancelPending)
	writeGroup.PATCH("/permission/edit", h.NodePermissionEdit)
	writeGroup.POST("/permission/audience", h.ReadingAudience)
	writeGroup.PATCH("/management", h.EditNodeManagement)
	writeGroup.POST("/release/restore", h.RestoreNodeRelease)

	return h
}

// CreateNode
//
//	@Summary		Create Node
//	@Description	Create Node
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		domain.CreateNodeReq	true	"Node"
//	@Success		200		{object}	domain.PWResponse{data=map[string]string}
//	@Router			/api/v1/node [post]
func (h *NodeHandler) CreateNode(c echo.Context) error {
	ctx := c.Request().Context()
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	req := &domain.CreateNodeReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	id, err := h.usecase.Create(c.Request().Context(), req, authInfo.UserId)
	if err != nil {
		return h.NewResponseWithError(c, "create node failed", err)
	}
	return h.NewResponseWithData(c, map[string]any{
		"id": id,
	})
}

// NodeStats
//
//	@Summary		Get Node Statistics
//	@Description	Get Node Statistics
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			kb_id	query		v1.NodeStatsReq	true	"Knowledge Base ID"
//	@Success		200		{object}	domain.PWResponse{data=v1.NodeStatsResp}
//	@Router			/api/v1/node/stats [get]
func (h *NodeHandler) NodeStats(c echo.Context) error {
	var req v1.NodeStatsReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}

	ctx := c.Request().Context()
	stats, err := h.usecase.GetNodeStats(ctx, req.KbId)
	if err != nil {
		return h.NewResponseWithError(c, "get node stats failed", err)
	}

	return h.NewResponseWithData(c, stats)
}

// GetNodeList
//
//	@Summary		Get Node List
//	@Description	Get Node List
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			params	query		domain.GetNodeListReq	true	"Params"
//	@Success		200		{object}	domain.PWResponse{data=[]domain.NodeListItemResp}
//	@Router			/api/v1/node/list [get]
func (h *NodeHandler) GetNodeList(c echo.Context) error {
	var req domain.GetNodeListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	ctx := c.Request().Context()
	nodes, err := h.usecase.GetList(ctx, &req)
	if err != nil {
		return h.NewResponseWithError(c, "get node list failed", err)
	}
	return h.NewResponseWithData(c, nodes)
}

// NodeListGroupNav
//
//	@Summary		Get Node List Grouped by Nav
//	@Description	Get unpublished or unstudied document list grouped by nav
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			params	query		v1.NodeListGroupNavReq	true	"Params"
//	@Success		200		{object}	domain.PWResponse{data=[]v1.NodeListGroupNavResp}
//	@Router			/api/v1/node/list/group/nav [get]
func (h *NodeHandler) NodeListGroupNav(c echo.Context) error {
	defer h.TrackTreeLoad(c)()
	var req v1.NodeListGroupNavReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	ctx := c.Request().Context()
	result, err := h.usecase.GetNodeListGroupByNav(ctx, req)
	if err != nil {
		return h.NewResponseWithError(c, "get node list group by nav failed", err)
	}
	return h.NewTreeResponseWithData(c, result)
}

// GetNodeDetail
//
//	@Summary		Get Node Detail
//	@Description	Get Node Detail
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	query		v1.GetNodeDetailReq	true	"conversation id"
//	@Success		200		{object}	domain.PWResponse{data=v1.NodeDetailResp}
//	@Router			/api/v1/node/detail [get]
func (h *NodeHandler) GetNodeDetail(c echo.Context) error {

	var req v1.GetNodeDetailReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}

	node, err := h.usecase.GetNodeByKBID(c.Request().Context(), req.ID, req.KbId, req.Format)
	if err != nil {
		h.logger.Error("get node by kb id failed", log.Error(err))
		return h.NewResponseWithError(c, "get node detail failed", err)
	}
	return h.NewResponseWithData(c, node)
}

// GetNodeReleaseList
//
//	@Summary		Get Node Release List
//	@Description	Get Node Release List
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	query		v1.GetNodeReleaseListReq	true	"params"
//	@Success		200		{object}	domain.PWResponse{data=[]v1.NodeReleaseListItem}
//	@Router			/api/pro/v1/node/release/list [get]
func (h *NodeHandler) GetNodeReleaseList(c echo.Context) error {
	var req v1.GetNodeReleaseListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}

	list, err := h.usecase.GetNodeReleaseList(c.Request().Context(), req.KbID, req.NodeID)
	if err != nil {
		return h.NewResponseWithError(c, "get node release list failed", err)
	}
	return h.NewResponseWithData(c, list)
}

// GetNodeReleaseDetail
//
//	@Summary		Get Node Release Detail
//	@Description	Get Node Release Detail
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	query		v1.GetNodeReleaseDetailReq	true	"params"
//	@Success		200		{object}	domain.PWResponse{data=v1.GetNodeReleaseDetailResp}
//	@Router			/api/pro/v1/node/release/detail [get]
func (h *NodeHandler) GetNodeReleaseDetail(c echo.Context) error {
	var req v1.GetNodeReleaseDetailReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request failed", err)
	}

	detail, err := h.usecase.GetNodeReleaseDetail(c.Request().Context(), req.KbID, req.ID)
	if err != nil {
		return h.NewResponseWithError(c, "get node release detail failed", err)
	}
	return h.NewResponseWithData(c, detail)
}

// RestoreNodeRelease
//
//	@Summary		Restore Node Release
//	@Description	Restore Node Release To Current Draft
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		v1.RestoreNodeReleaseReq	true	"body"
//	@Success		200		{object}	domain.PWResponse
//	@Router			/api/pro/v1/node/release/restore [post]
func (h *NodeHandler) RestoreNodeRelease(c echo.Context) error {
	var req v1.RestoreNodeReleaseReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	if err := h.usecase.RestoreNodeRelease(c.Request().Context(), req.KbID, req.ID, authInfo.UserId); err != nil {
		return h.NewResponseWithError(c, "restore node release failed", err)
	}
	return h.NewResponseWithData(c, map[string]any{})
}

// NodeAction
//
//	@Summary		Node Action
//	@Description	Node Action
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			action	body		domain.NodeActionReq	true	"Action"
//	@Success		200		{object}	domain.PWResponse{data=map[string]string}
//	@Router			/api/v1/node/action [post]
func (h *NodeHandler) NodeAction(c echo.Context) error {
	req := &domain.NodeActionReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	ctx := c.Request().Context()
	if err := h.usecase.NodeAction(ctx, req); err != nil {
		return h.NewResponseWithError(c, "node action failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// UpdateNodeDetail
//
//	@Summary		Update Node Detail
//	@Description	Update Node Detail
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		domain.UpdateNodeReq	true	"Node"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/node/detail [put]
func (h *NodeHandler) UpdateNodeDetail(c echo.Context) error {
	ctx := c.Request().Context()
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return h.NewResponseWithError(c, "authInfo not found in context", nil)
	}

	req := &domain.UpdateNodeReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	if err := h.usecase.Update(ctx, req, authInfo.UserId); err != nil {
		return h.NewResponseWithError(c, "update node detail failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// MoveNode
//
//	@Summary		Move Node
//	@Description	Move Node
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		domain.MoveNodeReq	true	"Move Node"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/node/move [post]
func (h *NodeHandler) MoveNode(c echo.Context) error {
	req := &domain.MoveNodeReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	ctx := c.Request().Context()
	if err := h.usecase.MoveNode(ctx, req); err != nil {
		return h.NewResponseWithError(c, "move node failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// NodeMoveNav
//
//	@Summary		Move Node to Nav
//	@Description	Move node (and all its descendants if folder) to a different nav
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		v1.NodeMoveNavReq	true	"Move Node Nav"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/node/move/nav [post]
func (h *NodeHandler) NodeMoveNav(c echo.Context) error {
	req := &v1.NodeMoveNavReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	ctx := c.Request().Context()
	if err := h.usecase.MoveNodeNav(ctx, req); err != nil {
		return h.NewResponseWithError(c, "move node nav failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// SummaryNode
//
//	@Summary		Summary Node 异步后台生成
//	@Description	Summary Node
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		domain.NodeSummaryReq	true	"Summary Node"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/node/summary [post]
func (h *NodeHandler) SummaryNode(c echo.Context) error {
	req := &domain.NodeSummaryReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	ctx := c.Request().Context()
	err := h.usecase.SummaryNode(ctx, req)
	if err != nil {
		if errors.Is(err, domain.ErrModelNotConfigured) {
			return h.NewResponseWithError(c, "请前往管理后台，点击右上角的“系统设置”配置推理大模型。", err)
		}
		return h.NewResponseWithError(c, "summary node failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// SummaryNodeStream
//
//	@Summary		Stream Summary Node
//	@Description	Stream Summary Node for single document
//	@Tags			node
//	@Accept			json
//	@Produce		text/event-stream
//	@Security		bearerAuth
//	@Param			body	body		domain.NodeSummaryReq	true	"Summary Node"
//	@Success		200		{string}	string					"SSE stream"
//	@Router			/api/v1/node/summary/stream [post]
func (h *NodeHandler) SummaryNodeStream(c echo.Context) error {
	req := &domain.NodeSummaryReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	if len(req.IDs) != 1 {
		return h.NewResponseWithError(c, "stream summary only supports single node", nil)
	}
	ctx := c.Request().Context()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Transfer-Encoding", "chunked")

	err := h.usecase.StreamSummaryNode(ctx, req, func(ctx context.Context, dataType, chunk string) error {
		return h.writeSSEEvent(c, domain.SSEEvent{Type: dataType, Content: chunk})
	})
	if err != nil {
		msg := "summary node failed"
		if errors.Is(err, domain.ErrModelNotConfigured) {
			msg = "请前往管理后台，点击右上角的“系统设置”配置推理大模型。"
		}
		if writeErr := h.writeSSEEvent(c, domain.SSEEvent{Type: "error", Content: msg, Error: err.Error()}); writeErr != nil {
			return writeErr
		}
		return nil
	}
	return h.writeSSEEvent(c, domain.SSEEvent{Type: "done"})
}

func (h *NodeHandler) writeSSEEvent(c echo.Context, data any) error {
	jsonContent, err := json.Marshal(data)
	if err != nil {
		return err
	}

	sseMessage := fmt.Sprintf("data: %s\n\n", string(jsonContent))
	if _, err := c.Response().Write([]byte(sseMessage)); err != nil {
		return err
	}
	c.Response().Flush()
	return nil
}

// RecommendNodes
//
//	@Summary		Recommend Nodes
//	@Description	Recommend Nodes
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			query	query		domain.GetRecommendNodeListReq	true	"Recommend Nodes"
//	@Success		200		{object}	domain.PWResponse{data=[]domain.RecommendNodeListResp}
//	@Router			/api/v1/node/recommend_nodes [get]
func (h *NodeHandler) RecommendNodes(c echo.Context) error {
	var req domain.GetRecommendNodeListReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	if len(req.NodeIDs) == 0 && len(req.NavIds) == 0 {
		return h.NewResponseWithError(c, "node_ids or nav_ids is required", nil)
	}
	ctx := c.Request().Context()
	nodes, err := h.usecase.GetRecommendNodeList(ctx, &req)
	if err != nil {
		return h.NewResponseWithError(c, "get recommend nodes failed", err)
	}
	return h.NewResponseWithData(c, nodes)
}

// BatchMoveNode
//
//	@Summary		Batch Move Node
//	@Description	Batch Move Node
//	@Tags			node
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			body	body		domain.BatchMoveReq	true	"Batch Move Node"
//	@Success		200		{object}	domain.Response
//	@Router			/api/v1/node/batch_move [post]
func (h *NodeHandler) BatchMoveNode(c echo.Context) error {
	req := &domain.BatchMoveReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "request body is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}
	ctx := c.Request().Context()
	if err := h.usecase.BatchMoveNode(ctx, req); err != nil {
		return h.NewResponseWithError(c, "batch move node failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// NodePermission 文档授权信息获取
//
//	@Tags			NodePermission
//	@Summary		文档授权信息获取
//	@Description	文档授权信息获取
//	@ID				v1-NodePermission
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	query		v1.NodePermissionReq	true	"para"
//	@Success		200		{object}	domain.Response{data=v1.NodePermissionResp}
//	@Router			/api/v1/node/permission [get]
func (h *NodeHandler) NodePermission(c echo.Context) error {
	var req v1.NodePermissionReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}

	ctx := c.Request().Context()
	release, err := h.usecase.GetNodePermissionsByID(ctx, req.ID, req.KbId)
	if err != nil {
		return h.NewResponseWithError(c, "get node permission detail failed", err)
	}
	return h.NewResponseWithData(c, release)
}

// NodePermissionEdit 文档授权信息更新
//
//	@Tags			NodePermission
//	@Summary		文档授权信息更新
//	@Description	文档授权信息更新
//	@ID				v1-NodePermissionEdit
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	body		v1.NodePermissionEditReq	true	"para"
//	@Success		200		{object}	domain.Response{data=v1.NodePermissionEditResp}
//	@Router			/api/v1/node/permission/edit [patch]
func (h *NodeHandler) NodePermissionEdit(c echo.Context) error {
	var req v1.NodePermissionEditReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}

	if err := h.usecase.ValidateNodePermissionsEdit(req, consts.GetLicenseEdition(c)); err != nil {
		return h.NewResponseWithError(c, "validate node permission failed", err)
	}

	ctx := c.Request().Context()
	err := h.usecase.NodePermissionsEdit(ctx, req)
	if err != nil {
		return h.NewResponseWithError(c, "update node permission failed", err)
	}
	return h.NewResponseWithData(c, nil)
}

// NodeRestudy 文档重新学习
//
//	@Tags			Node
//	@Summary		文档重新学习
//	@Description	文档重新学习
//	@ID				v1-NodeRestudy
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	body		v1.NodeRestudyReq	true	"para"
//	@Success		200		{object}	domain.Response{data=v1.NodeRestudyResp}
//	@Router			/api/v1/node/restudy [post]
func (h *NodeHandler) NodeRestudy(c echo.Context) error {
	var req v1.NodeRestudyReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	if authInfo := domain.GetAuthInfoFromCtx(c.Request().Context()); authInfo != nil {
		req.UserID = authInfo.UserId
	}

	if err := h.usecase.NodeRestudy(c.Request().Context(), &req); err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}

	return h.NewResponseWithData(c, nil)
}

// NodeLearningCancel 停止文档学习/重建任务
//
//	@Tags			Node
//	@Summary		停止文档学习/重建任务
//	@Description	停止文档学习/重建任务
//	@ID				v1-NodeLearningCancel
//	@Accept			json
//	@Produce		json
//	@Security		bearerAuth
//	@Param			param	body		v1.NodeLearningCancelReq	true	"para"
//	@Success		200		{object}	domain.Response{data=v1.NodeLearningCancelResp}
//	@Router			/api/v1/node/learning/cancel [post]
func (h *NodeHandler) NodeLearningCancel(c echo.Context) error {
	var req v1.NodeLearningCancelReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}

	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	if authInfo := domain.GetAuthInfoFromCtx(c.Request().Context()); authInfo != nil {
		req.UserID = authInfo.UserId
	}

	count, err := h.usecase.NodeLearningCancel(c.Request().Context(), &req)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}

	return h.NewResponseWithData(c, &v1.NodeLearningCancelResp{Count: count})
}

func (h *NodeHandler) NodeLearningCancelPending(c echo.Context) error {
	var req v1.NodeLearningCancelPendingReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "request params is invalid", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request params failed", err)
	}
	auth := domain.GetAuthInfoFromCtx(c.Request().Context())
	if auth == nil || auth.UserId == "" {
		return h.NewResponseWithError(c, "未登录", fmt.Errorf("missing user identity"))
	}
	result, err := h.usecase.CancelPendingLearning(c.Request().Context(), req.KbId, auth.UserId)
	if err != nil {
		return h.NewResponseWithError(c, err.Error(), err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NodeHandler) TreeNodeStatuses(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "private, no-store")
	var req struct {
		KBID string   `json:"kb_id" validate:"required"`
		IDs  []string `json:"ids" validate:"max=1000"`
	}
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid status request", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "invalid status request", err)
	}
	result, err := h.usecase.GetTreeNodeStatuses(c.Request().Context(), req.KBID, req.IDs)
	if err != nil {
		return h.NewResponseWithError(c, "get indexing statuses failed", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NodeHandler) NodeTree(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "private, no-store")
	defer h.TrackTreeLoad(c)()
	var req v1.NodeListGroupNavReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "invalid tree request", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "invalid tree request", err)
	}
	result, err := h.usecase.GetNodeTree(c.Request().Context(), req)
	if err != nil {
		return h.NewResponseWithError(c, "get node tree failed", err)
	}
	return h.NewTreeResponseWithData(c, result)
}
