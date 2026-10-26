package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
	"github.com/samber/lo"
	"gorm.io/gorm"

	navV1 "github.com/chaitin/panda-wiki/api/nav/v1"
	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	shareV1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/rag"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/chaitin/panda-wiki/utils"
)

type NodeUsecase struct {
	nodeRepo     *pg.NodeRepository
	navRepo      *pg.NavRepository
	appRepo      *pg.AppRepository
	ragRepo      *mq.RAGRepository
	kbRepo       *pg.KnowledgeBaseRepository
	modelRepo    *pg.ModelRepository
	userRepo     *pg.UserRepository
	authRepo     *pg.AuthRepo
	enterprise   *EnterpriseUsecase
	llmUsecase   *LLMUsecase
	config       *config.Config
	logger       *log.Logger
	s3Client     *s3.MinioClient
	rAGService   rag.RAGService
	modelUsecase *ModelUsecase
}

func NewNodeUsecase(
	nodeRepo *pg.NodeRepository,
	navRepo *pg.NavRepository,
	appRepo *pg.AppRepository,
	ragRepo *mq.RAGRepository,
	userRepo *pg.UserRepository,
	kbRepo *pg.KnowledgeBaseRepository,
	llmUsecase *LLMUsecase,
	ragService rag.RAGService,
	logger *log.Logger,
	s3Client *s3.MinioClient,
	modelRepo *pg.ModelRepository,
	authRepo *pg.AuthRepo,
	modelUsecase *ModelUsecase,
	enterprise *EnterpriseUsecase,
	_ *pg.SystemSettingRepo,
	config *config.Config,
) *NodeUsecase {
	return &NodeUsecase{
		nodeRepo:     nodeRepo,
		navRepo:      navRepo,
		rAGService:   ragService,
		appRepo:      appRepo,
		ragRepo:      ragRepo,
		kbRepo:       kbRepo,
		authRepo:     authRepo,
		enterprise:   enterprise,
		userRepo:     userRepo,
		llmUsecase:   llmUsecase,
		modelRepo:    modelRepo,
		config:       config,
		logger:       logger.WithModule("usecase.node"),
		s3Client:     s3Client,
		modelUsecase: modelUsecase,
	}
}

const (
	ragSyncChunkSize                = 10
	nodeLearningDispatchBatchSize   = 30
	nodeLearningMaxRunningTasks     = 50
	nodeLearningBulkPriority        = 10
	nodeLearningManualPriority      = 100
	nodeLearningMetadataTaskTimeout = 30 * time.Minute
	nodeLearningTaskRunningTimeout  = 4 * time.Hour
)

func (u *NodeUsecase) Create(ctx context.Context, req *domain.CreateNodeReq, userId string) (string, error) {
	nodeID, err := u.nodeRepo.Create(ctx, req, userId)
	if err != nil {
		return "", err
	}
	return nodeID, nil
}

func (u *NodeUsecase) GetList(ctx context.Context, req *domain.GetNodeListReq) ([]*domain.NodeListItemResp, error) {
	nodes, err := u.nodeRepo.GetList(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nodes, nil
	}

	nodes, err = u.filterManagedNodes(ctx, req.KBID, nodes)
	if err != nil {
		return nil, err
	}

	publisherMap, err := u.nodeRepo.GetNodeReleasePublisherMap(ctx, req.KBID)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if publisherID, exists := publisherMap[node.ID]; exists {
			node.PublisherId = publisherID
		}
	}

	return nodes, nil
}

func (u *NodeUsecase) GetNodeByKBID(ctx context.Context, id, kbId, format string) (*v1.NodeDetailResp, error) {
	node, err := u.nodeRepo.GetByID(ctx, id, kbId)
	if err != nil {
		return nil, err
	}

	allowed, err := u.canManageNodeInAdmin(ctx, kbId, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("没有此文档的管理权限")
	}
	access, err := u.managementAccess(ctx, kbId)
	if err != nil {
		return nil, err
	}
	node.CanManage = access.Global || access.CanManageNode(id)
	if !node.CanManage {
		node.Permissions.ManagementGrants = nil
	}

	nodeRelease, err := u.nodeRepo.GetLatestNodeReleaseWithPublishAccount(ctx, node.ID)
	if err != nil {
		return nil, err
	}
	if nodeRelease != nil {
		node.PublisherId = nodeRelease.PublisherId
		node.PublisherAccount = nodeRelease.PublisherAccount
	}

	nodeStat, err := u.nodeRepo.GetNodeStatsByNodeId(ctx, node.ID)
	if err != nil {
		return nil, err
	}
	node.PV = nodeStat.PV

	if node.Meta.ContentType == domain.ContentTypeMD {
		return node, nil
	}
	if format != "raw" {
		if !utils.IsLikelyHTML(node.Content) {
			node.Content = u.convertMDToHTML(node.Content)
		}
	}
	return node, nil
}

func (u *NodeUsecase) NodeAction(ctx context.Context, req *domain.NodeActionReq) error {
	switch req.Action {
	case "delete":
		docIDs, err := u.nodeRepo.Delete(ctx, req.KBID, req.IDs)
		if err != nil {
			return err
		}
		nodeVectorContentRequests := make([]*domain.NodeReleaseVectorRequest, 0)
		for _, docID := range docIDs {
			nodeVectorContentRequests = append(nodeVectorContentRequests, &domain.NodeReleaseVectorRequest{
				KBID:   req.KBID,
				DocID:  docID,
				Action: "delete",
			})
		}
		if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, nodeVectorContentRequests); err != nil {
			return err
		}
	}
	return nil
}

func (u *NodeUsecase) Update(ctx context.Context, req *domain.UpdateNodeReq, userId string) error {
	if req.NavId != nil {
		_, err := u.navRepo.GetById(ctx, *req.NavId)
		if err != nil {
			return errors.New("invalid nav_id")
		}
	}
	err := u.nodeRepo.UpdateNodeContent(ctx, req, userId)
	if err != nil {
		return err
	}
	return nil
}

func (u *NodeUsecase) GetNodeReleaseList(ctx context.Context, kbID, nodeID string) ([]*v1.NodeReleaseListItem, error) {
	if allowed, err := u.canManageNodeInAdmin(ctx, kbID, nodeID); err != nil {
		return nil, err
	} else if !allowed {
		return nil, fmt.Errorf("没有此文档的管理权限")
	}
	return u.nodeRepo.GetNodeReleaseList(ctx, kbID, nodeID)
}

func (u *NodeUsecase) GetNodeReleaseDetail(ctx context.Context, kbID, releaseID string) (*v1.GetNodeReleaseDetailResp, error) {
	release, err := u.nodeRepo.GetNodeReleaseByID(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if release.KBID != kbID {
		return nil, fmt.Errorf("文档不属于此知识库")
	}
	if allowed, err := u.canManageNodeInAdmin(ctx, kbID, release.NodeID); err != nil {
		return nil, err
	} else if !allowed {
		return nil, fmt.Errorf("没有此文档的管理权限")
	}
	return u.nodeRepo.GetNodeReleaseDetail(ctx, kbID, releaseID)
}

func (u *NodeUsecase) RestoreNodeRelease(ctx context.Context, kbID, releaseID, userID string) error {
	release, err := u.nodeRepo.GetNodeReleaseByID(ctx, releaseID)
	if err != nil {
		return fmt.Errorf("get node release failed: %w", err)
	}
	if release.KBID != kbID {
		return fmt.Errorf("node release does not belong to current knowledge base")
	}

	contentType := release.Meta.ContentType
	if contentType == "" {
		contentType = domain.ContentTypeMD
	}

	updateReq := &domain.UpdateNodeReq{
		ID:            release.NodeID,
		KBID:          kbID,
		Name:          lo.ToPtr(release.Name),
		Content:       lo.ToPtr(release.Content),
		EditorContent: &release.EditorContent,
		EditorBlocks:  &release.EditorBlocks,
		EditorFormat:  lo.ToPtr(release.EditorFormat),
		EditorVersion: lo.ToPtr(release.EditorVersion),
		Emoji:         lo.ToPtr(release.Meta.Emoji),
		Summary:       lo.ToPtr(release.Meta.Summary),
		ContentType:   lo.ToPtr(contentType),
	}
	if err := u.nodeRepo.UpdateNodeContent(ctx, updateReq, userID); err != nil {
		return fmt.Errorf("restore node release failed: %w", err)
	}

	return nil
}

func (u *NodeUsecase) ValidateNodePerm(ctx context.Context, kbID, nodeId, userID string) *domain.PWResponseErrCode {
	node, err := u.nodeRepo.GetNodeReleaseDetailByKBIDAndID(ctx, kbID, nodeId)
	if err != nil {
		return &domain.ErrCodeNotFound
	}
	resolvedPermissions, err := u.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, node.Permissions)
	if err != nil {
		return &domain.ErrCodeInternalError
	}
	node.Permissions = resolvedPermissions
	if node.NavID != "" {
		nav, navErr := u.navRepo.GetById(ctx, node.NavID)
		if navErr != nil || nav.KbID != kbID {
			return &domain.ErrCodeInternalError
		}
		node.Permissions = u.enterprise.MergeNodeAndNavPermissions(node.Permissions, nav.Permissions)
	}
	shareAccess, err := u.enterprise.BuildShareAccessContext(ctx, userID)
	if err != nil {
		return &domain.ErrCodeInternalError
	}
	if !u.enterprise.CanVisitable(node.Permissions, shareAccess) {
		return &domain.ErrCodePermissionDenied
	}
	return nil
}

func (u *NodeUsecase) GetNodeReleaseDetailByKBIDAndID(ctx context.Context, kbID, nodeId, format string) (*shareV1.ShareNodeDetailResp, error) {
	node, err := u.nodeRepo.GetNodeReleaseDetailByKBIDAndID(ctx, kbID, nodeId)
	if err != nil {
		return nil, err
	}
	resolvedPermissions, resolveErr := u.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, node.Permissions)
	if resolveErr != nil {
		return nil, resolveErr
	}
	node.Permissions = resolvedPermissions
	if node.NavID != "" {
		nav, navErr := u.navRepo.GetById(ctx, node.NavID)
		if navErr != nil {
			return nil, navErr
		}
		if nav.KbID != kbID {
			return nil, fmt.Errorf("node navigation does not belong to current knowledge base")
		}
		node.Permissions = u.enterprise.MergeNodeAndNavPermissions(node.Permissions, nav.Permissions)
	}

	userMap, err := u.userRepo.GetUsersAccountMap(ctx)
	if err != nil {
		return nil, err
	}
	if account, ok := userMap[node.CreatorId]; ok {
		node.CreatorAccount = account
	}
	if account, ok := userMap[node.EditorId]; ok {
		node.EditorAccount = account
	}
	if account, ok := userMap[node.PublisherId]; ok {
		node.PublisherAccount = account
	}

	if domain.GetBaseEditionLimitation(ctx).AllowNodeStats {
		webApp, err := u.appRepo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
		if err != nil {
			return nil, err
		}

		if webApp.Settings.StatsSetting.PVEnable {
			nodeStat, err := u.nodeRepo.GetNodeStatsByNodeId(ctx, nodeId)
			if err != nil {
				return nil, err
			}
			node.PV = nodeStat.PV
		}
	}

	if node.Meta.ContentType == domain.ContentTypeMD {
		return node, nil
	}
	// just for info
	if format != "raw" {
		if !utils.IsLikelyHTML(node.Content) {
			node.Content = u.convertMDToHTML(node.Content)
		}
	}
	return node, nil
}

func (u *NodeUsecase) MoveNode(ctx context.Context, req *domain.MoveNodeReq) error {
	return u.nodeRepo.MoveNodeBetween(ctx, req.ID, req.ParentID, req.PrevID, req.NextID, req.KbID)
}

func (u *NodeUsecase) SummaryNode(ctx context.Context, req *domain.NodeSummaryReq) error {
	_, err := u.modelUsecase.GetChatModel(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrModelNotConfigured
		}
		return err
	}
	// async create node summary
	nodeVectorContentRequests := make([]*domain.NodeReleaseVectorRequest, 0)
	for _, id := range req.IDs {
		nodeVectorContentRequests = append(nodeVectorContentRequests, &domain.NodeReleaseVectorRequest{
			KBID:   req.KBID,
			NodeID: id,
			Action: "summary",
		})
	}
	if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, nodeVectorContentRequests); err != nil {
		return err
	}

	return nil
}

func (u *NodeUsecase) StreamSummaryNode(ctx context.Context, req *domain.NodeSummaryReq, onChunk func(ctx context.Context, dataType, chunk string) error) error {
	model, err := u.modelUsecase.GetChatModel(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrModelNotConfigured
		}
		return err
	}
	if len(req.IDs) != 1 {
		return fmt.Errorf("stream summary only supports single node")
	}

	node, err := u.nodeRepo.GetNodeByID(ctx, req.IDs[0])
	if err != nil {
		return fmt.Errorf("get latest node release failed: %w", err)
	}

	if err := u.llmUsecase.StreamSummaryNode(ctx, req.KBID, model, node.Name, node.Content, onChunk); err != nil {
		return fmt.Errorf("summary node failed: %w", err)
	}
	return nil
}

func (u *NodeUsecase) GetRecommendNodeList(ctx context.Context, req *domain.GetRecommendNodeListReq) ([]*domain.RecommendNodeListResp, error) {
	nodes, err := u.getPublishedRecommendNodeList(ctx, req)
	if err != nil {
		return nil, err
	}
	return u.filterRecommendNodeAudience(ctx, req.KBID, nodes)
}

// Callers must apply their audience policy: management permissions for the
// admin editor, reading permissions for the published reader application.
func (u *NodeUsecase) getPublishedRecommendNodeList(ctx context.Context, req *domain.GetRecommendNodeListReq) ([]*domain.RecommendNodeListResp, error) {
	// get latest kb release
	kbRelease, err := u.kbRepo.GetLatestRelease(ctx, req.KBID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var nodes []*domain.RecommendNodeListResp

	// 优先通过 NavIds 搜索，如果 NavIds 为空则使用 NodeIDs
	if len(req.NavIds) > 0 {
		nodes, err = u.nodeRepo.GetRecommendNodeListByNavIDs(ctx, req.KBID, kbRelease.ID, req.NavIds)
		if err != nil {
			return nil, err
		}
	} else if len(req.NodeIDs) > 0 {
		nodes, err = u.nodeRepo.GetRecommendNodeListByIDs(ctx, req.KBID, kbRelease.ID, req.NodeIDs)
		if err != nil {
			return nil, err
		}
	}

	if len(nodes) > 0 {
		// 如果是通过 NodeIDs 查询，按照 req.NodeIDs 的顺序排序
		if len(req.NodeIDs) > 0 && len(req.NavIds) == 0 {
			nodesMap := lo.SliceToMap(nodes, func(item *domain.RecommendNodeListResp) (string, *domain.RecommendNodeListResp) {
				return item.ID, item
			})
			nodes = make([]*domain.RecommendNodeListResp, 0)
			for _, id := range req.NodeIDs {
				if node, ok := nodesMap[id]; ok {
					nodes = append(nodes, node)
				}
			}
		}

		// get folder nodes
		folderNodeIds := lo.Filter(nodes, func(item *domain.RecommendNodeListResp, _ int) bool {
			return item.Type == domain.NodeTypeFolder
		})
		if len(folderNodeIds) > 0 {
			parentIDNodeMap, err := u.nodeRepo.GetRecommendNodeListByParentIDs(ctx, req.KBID, kbRelease.ID, lo.Map(folderNodeIds, func(item *domain.RecommendNodeListResp, _ int) string {
				return item.ID
			}))
			if err != nil {
				return nil, err
			}
			for _, node := range nodes {
				if parentNodes, ok := parentIDNodeMap[node.ID]; ok {
					node.RecommendNodes = parentNodes
				}
			}
		}
		return nodes, nil
	}
	return nil, nil
}

func (u *NodeUsecase) filterRecommendNodeAudience(ctx context.Context, kbID string, nodes []*domain.RecommendNodeListResp) ([]*domain.RecommendNodeListResp, error) {
	filtered := make([]*domain.RecommendNodeListResp, 0, len(nodes))
	for _, node := range nodes {
		allowed, err := u.canManageNodeInAdmin(ctx, kbID, node.ID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}
		node.Permissions.ManagementGrants = nil
		node.RecommendNodes, err = u.filterRecommendNodeAudience(ctx, kbID, node.RecommendNodes)
		if err != nil {
			return nil, err
		}
		filtered = append(filtered, node)
	}
	return filtered, nil
}

func (u *NodeUsecase) BatchMoveNode(ctx context.Context, req *domain.BatchMoveReq) error {
	return u.nodeRepo.BatchMove(ctx, req)
}

func (u *NodeUsecase) MoveNodeNav(ctx context.Context, req *v1.NodeMoveNavReq) error {
	nav, err := u.navRepo.GetById(ctx, req.NavID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("nav not found: %w", err)
		}
		return err
	}
	if nav.KbID != req.KbID {
		return fmt.Errorf("nav does not belong to kb %s", req.KbID)
	}
	return u.nodeRepo.MoveNodeNav(ctx, req.KbID, req.NavID, req.IDs)
}

func (u *NodeUsecase) convertMDToHTML(mdStr string) string {
	extensions := parser.CommonExtensions & ^parser.Autolink & ^parser.MathJax
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(mdStr))

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	maybeUnsafeHTML := markdown.Render(doc, renderer)
	html := bluemonday.UGCPolicy().SanitizeBytes(maybeUnsafeHTML)
	return string(html)
}

func (u *NodeUsecase) GetShareNodeList(ctx context.Context, kbId, userID string) ([]*shareV1.NodeListGroupNavResp, error) {
	nodes, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbId)
	if err != nil {
		return nil, err
	}

	nodes, err = u.filterShareNodesWithNavigationPaths(ctx, kbId, userID, nodes)
	if err != nil {
		return nil, err
	}

	navs, err := u.navRepo.GetReleaseList(ctx, kbId)
	if err != nil {
		return nil, err
	}
	shareAccess, err := u.enterprise.BuildShareAccessContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*shareV1.NodeListGroupNavResp, 0, len(navs))
	navIndexMap := make(map[string]int, len(navs))
	for _, nav := range navs {
		if nav.Permissions.HasEnterpriseRule() && !u.enterprise.CanVisible(nav.Permissions, shareAccess) {
			continue
		}
		navIndexMap[nav.ID] = len(result)
		result = append(result, &shareV1.NodeListGroupNavResp{
			NavID:    nav.ID,
			NavName:  nav.Name,
			Position: nav.Position,
			List:     []domain.ShareNodeListItemResp{},
		})
	}

	for _, node := range nodes {
		if idx, ok := navIndexMap[node.NavId]; ok {
			result[idx].List = append(result[idx].List, *node)
			result[idx].Count++
		}
	}

	return result, nil
}

func (u *NodeUsecase) GetNodeReleaseListByParentID(ctx context.Context, kbID, parentID, userID string) ([]*domain.ShareNodeDetailItem, error) {
	// 一次性查询所有节点
	allNodes, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbID)
	if err != nil {
		return nil, err
	}

	visibleNodes, err := u.filterShareNodesWithNavigationPaths(ctx, kbID, userID, allNodes)
	if err != nil {
		return nil, err
	}

	// 构建父子关系映射
	childrenMap := make(map[string][]*domain.ShareNodeListItemResp)
	for _, node := range visibleNodes {
		childrenMap[node.ParentID] = append(childrenMap[node.ParentID], node)
	}

	// 构建树结构
	result := u.buildNodeTree(parentID, childrenMap)

	return result, nil
}

func (u *NodeUsecase) filterShareNodesWithNavigationPaths(ctx context.Context, kbID, userID string, nodes []*domain.ShareNodeListItemResp) ([]*domain.ShareNodeListItemResp, error) {
	shareAccess, err := u.enterprise.BuildShareAccessContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	navs, err := u.navRepo.GetReleaseList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	navPermissions := make(map[string]domain.NodePermissions, len(navs))
	for _, nav := range navs {
		navPermissions[nav.ID] = nav.Permissions
	}

	nodeByID := make(map[string]*domain.ShareNodeListItemResp, len(nodes))
	readable := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
		if node.NavId != "" {
			if _, ok := navPermissions[node.NavId]; !ok {
				readable[node.ID] = false
				continue
			}
		}
		resolved, resolveErr := u.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, node.Permissions)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if navPermissions[node.NavId].HasEnterpriseRule() {
			resolved = u.enterprise.MergeNodeAndNavPermissions(resolved, navPermissions[node.NavId])
		}
		node.Permissions = resolved
		readable[node.ID] = u.enterprise.CanVisible(resolved, shareAccess)
	}

	pathOnly := make(map[string]bool)
	for _, node := range nodes {
		if !readable[node.ID] {
			continue
		}
		parentID := node.ParentID
		visited := map[string]bool{node.ID: true}
		for parentID != "" && !visited[parentID] {
			visited[parentID] = true
			parent, exists := nodeByID[parentID]
			if !exists || parent.Type != domain.NodeTypeFolder || parent.NavId != node.NavId {
				break
			}
			if !readable[parent.ID] {
				pathOnly[parent.ID] = true
			}
			parentID = parent.ParentID
		}
	}

	filtered := make([]*domain.ShareNodeListItemResp, 0, len(nodes))
	for _, node := range nodes {
		if readable[node.ID] {
			copyNode := *node
			copyNode.PathOnly = false
			filtered = append(filtered, &copyNode)
			continue
		}
		if !pathOnly[node.ID] {
			continue
		}
		// Path nodes reveal only the folder name and structure needed to reach
		// authorized descendants; metadata and ACL details are kept private.
		copyNode := *node
		copyNode.PathOnly = true
		copyNode.Emoji = ""
		copyNode.Meta = domain.NodeMeta{}
		copyNode.UpdatedAt = time.Time{}
		copyNode.Permissions = domain.NodePermissions{}
		filtered = append(filtered, &copyNode)
	}
	return filtered, nil
}

func (u *NodeUsecase) GetSharePathOnlyNodeDetail(ctx context.Context, kbID, nodeID, userID string) (*shareV1.ShareNodeDetailResp, error) {
	nodes, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	filtered, err := u.filterShareNodesWithNavigationPaths(ctx, kbID, userID, nodes)
	if err != nil {
		return nil, err
	}
	childrenMap := make(map[string][]*domain.ShareNodeListItemResp, len(filtered))
	for _, item := range filtered {
		childrenMap[item.ParentID] = append(childrenMap[item.ParentID], item)
	}
	for _, node := range filtered {
		if node.ID != nodeID || !node.PathOnly {
			continue
		}
		return &shareV1.ShareNodeDetailResp{
			ID:       node.ID,
			KbID:     kbID,
			NavID:    node.NavId,
			Type:     node.Type,
			Name:     node.Name,
			ParentID: node.ParentID,
			PathOnly: true,
			List:     u.buildNodeTree(node.ID, childrenMap),
		}, nil
	}
	return nil, fmt.Errorf("node is not a navigation-only ancestor")
}

// buildNodeTree 递归构建节点树结构
func (u *NodeUsecase) buildNodeTree(parentID string, childrenMap map[string][]*domain.ShareNodeListItemResp) []*domain.ShareNodeDetailItem {
	children := childrenMap[parentID]
	result := make([]*domain.ShareNodeDetailItem, 0, len(children))

	for _, child := range children {
		node := &domain.ShareNodeDetailItem{
			ID:        child.ID,
			Name:      child.Name,
			Type:      child.Type,
			PathOnly:  child.PathOnly,
			ParentID:  child.ParentID,
			Position:  child.Position,
			Meta:      child.Meta,
			Emoji:     child.Emoji,
			UpdatedAt: child.UpdatedAt,
			Children:  make([]*domain.ShareNodeDetailItem, 0),
		}

		// 如果是文件夹，递归构建其子节点
		if child.Type == domain.NodeTypeFolder {
			childNodes := u.buildNodeTree(child.ID, childrenMap)
			if len(childNodes) > 0 {
				node.Children = append(node.Children, childNodes...)
			}
		}

		result = append(result, node)
	}

	return result
}

func (u *NodeUsecase) GetNodeIdsByAuthId(ctx context.Context, authId uint, PermName consts.NodePermName) ([]string, error) {
	authGroups, err := u.authRepo.GetAuthGroupWithParentsByAuthId(ctx, authId)
	if err != nil {
		return nil, err
	}

	authGroupIds := lo.Map(authGroups, func(v domain.AuthGroup, i int) uint {
		return v.ID
	})

	nodeGroupIds := make([]string, 0)
	if len(authGroupIds) != 0 {
		nodeGroups, err := u.nodeRepo.GetNodeGroupsByGroupIdsPerm(ctx, authGroupIds, PermName)
		if err != nil {
			return nil, err
		}

		nodeGroupIds = lo.Map(nodeGroups, func(v domain.NodeAuthGroup, i int) string {
			return v.NodeID
		})
	}

	return nodeGroupIds, nil
}
func (u *NodeUsecase) GetNodePermissionsByID(ctx context.Context, id, kbID string) (*v1.NodePermissionResp, error) {
	node, err := u.nodeRepo.GetByID(ctx, id, kbID)
	if err != nil {
		return nil, err
	}
	allowed, err := u.canInspectNodePermissionInAdmin(ctx, kbID, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("没有此文档的管理权限")
	}
	effectivePermissions, err := u.enterprise.ResolveShareNodePermissions(ctx, node.ID, node.ParentID, node.Permissions)
	if err != nil {
		return nil, err
	}
	if node.NavId != "" {
		nav, err := u.navRepo.GetById(ctx, node.NavId)
		if err != nil {
			return nil, err
		}
		if nav.KbID != kbID {
			return nil, fmt.Errorf("栏目不属于当前知识库")
		}
		effectivePermissions = u.enterprise.MergeNodeAndNavPermissions(effectivePermissions, nav.Permissions)
	}
	management, err := u.managementAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	resp := &v1.NodePermissionResp{
		ID:                   node.ID,
		Permissions:          node.Permissions,
		EffectivePermissions: effectivePermissions,
	}
	if !management.CanManageNode(id) {
		resp.Permissions.ManagementGrants = nil
		resp.EffectivePermissions.ManagementGrants = nil
	}
	return resp, nil
}

func (u *NodeUsecase) ValidateNodePermissionsEdit(req v1.NodePermissionEditReq, edition consts.LicenseEdition) error {
	if req.Permissions == nil {
		return nil
	}
	return nil
}

func (u *NodeUsecase) NodePermissionsEdit(ctx context.Context, req v1.NodePermissionEditReq) error {
	if req.Permissions != nil {
		access, err := u.managementAccess(ctx, req.KbId)
		if err != nil {
			return err
		}
		prepared := make(map[string]domain.NodePermissions, len(req.IDs))
		for _, id := range req.IDs {
			node, err := u.nodeRepo.GetByID(ctx, id, req.KbId)
			if err != nil {
				return err
			}
			if !access.CanManageNode(id) {
				return fmt.Errorf("没有此文档或目录的管理权限")
			}
			permissions, err := u.enterprise.prepareReadingPermissions(ctx, node.Permissions, *req.Permissions, true)
			if err != nil {
				return err
			}
			if err := u.enterprise.ValidateNodePermissionAudience(ctx, req.KbId, permissions, node.Permissions, node.ParentID, node.NavId); err != nil {
				return err
			}
			if permissions.Inherit != nil && *permissions.Inherit {
				permissions.ParentPermissionSnapshot = nil
			} else if (permissions.Inherit != nil && !*permissions.Inherit) || hasExplicitNodePermissionRule(permissions) {
				permissions.ParentPermissionSnapshot, err = u.enterprise.ParentPermissionSnapshot(ctx, req.KbId, node.ParentID, node.NavId)
				if err != nil {
					return err
				}
			}
			prepared[id] = permissions
		}
		for id, permissions := range prepared {
			if err := u.nodeRepo.UpdateNodesByKbID(ctx, []string{id}, req.KbId, map[string]interface{}{"permissions": permissions}); err != nil {
				return err
			}
		}
	}

	nodeReleases, err := u.nodeRepo.GetLatestNodeReleaseByNodeIDs(ctx, req.KbId, req.IDs)
	if err != nil {
		return fmt.Errorf("get latest node release failed: %w", err)
	}

	if req.Permissions != nil && len(nodeReleases) > 0 {
		nodeVectorContentRequests := make([]*domain.NodeReleaseVectorRequest, 0)
		for _, nodeRelease := range nodeReleases {
			if nodeRelease.DocID == "" {
				continue
			}
			nodeVectorContentRequests = append(nodeVectorContentRequests, &domain.NodeReleaseVectorRequest{
				KBID:     req.KbId,
				DocID:    nodeRelease.DocID,
				Action:   "update_group_ids",
				GroupIds: nil,
			})
		}

		if len(nodeVectorContentRequests) != 0 {
			if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, nodeVectorContentRequests); err != nil {
				return err
			}
		}
	}

	return nil
}

func (u *NodeUsecase) SyncRagNodeStatus(ctx context.Context) error {
	kbs, err := u.kbRepo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return err
	}
	for _, kb := range kbs {
		docIds, err := u.nodeRepo.GetNodeIdsWithoutStatusByKbId(ctx, kb.ID)
		if err != nil {
			u.logger.Error("get node ids without status failed",
				log.String("kb_id", kb.ID),
				log.Error(err))
			continue
		}
		if len(docIds) == 0 {
			continue
		}

		chunks := lo.Chunk(docIds, ragSyncChunkSize)
		for _, chunk := range chunks {
			docToReleaseMap, err := u.nodeRepo.GetNodeReleasesByDocIDs(ctx, chunk)
			if err != nil {
				u.logger.Error("get node releases by doc ids failed", log.Error(err))
				continue
			}
			docs, listErr := u.rAGService.ListDocuments(ctx, kb.DatasetID, chunk)
			found := make(map[string]bool, len(docs))
			for _, doc := range docs {
				found[doc.ID] = true
			}
			for docID, release := range docToReleaseMap {
				checkError := ""
				if listErr != nil {
					checkError = "查询 RAGLite 状态失败：" + listErr.Error()
				} else if !found[docID] {
					checkError = "RAGLite 未返回该文档，请检查文档是否存在或已被删除"
				}
				if checkError == "" {
					continue
				}
				task, err := u.nodeRepo.GetLatestNodeLearningTaskByDocID(ctx, docID)
				if err == nil {
					if err := u.nodeRepo.ObserveNodeLearningRAG(ctx, kb.ID, release.NodeID, task.ID, kb.DatasetID, docID, "", "", "poll", checkError); err != nil {
						u.logger.Error("record RAG status check failure failed", log.Error(err))
					}
				}
			}
			if listErr != nil {
				u.logger.Error("list documents from RAG failed", log.String("kb_id", kb.ID), log.Error(listErr))
				continue
			}

			for _, doc := range docs {
				release, exists := docToReleaseMap[doc.ID]
				if !exists {
					u.logger.Warn("doc_id not found in node_releases",
						log.String("doc_id", doc.ID))
					continue
				}
				nodeID := release.NodeID

				var latestTask *domain.NodeLearningTask
				var activeTask *domain.NodeLearningTask
				task, taskErr := u.nodeRepo.GetActiveNodeLearningTaskByDocID(ctx, doc.ID)
				if taskErr == nil {
					if task.NodeReleaseID == release.ID && task.NodeID == release.NodeID {
						activeTask = task
					} else {
						u.logger.Warn("ignore active learning task because task and release mismatch",
							log.String("doc_id", doc.ID),
							log.String("task_id", task.ID),
							log.String("task_release_id", task.NodeReleaseID),
							log.String("release_id", release.ID))
					}
				} else if !errors.Is(taskErr, gorm.ErrRecordNotFound) {
					u.logger.Error("get active node learning task by doc id failed",
						log.String("doc_id", doc.ID),
						log.Error(taskErr))
				}
				if activeTask == nil {
					task, taskErr := u.nodeRepo.GetLatestNodeLearningTaskByDocID(ctx, doc.ID)
					if taskErr == nil {
						if task.NodeReleaseID == release.ID && task.NodeID == release.NodeID {
							latestTask = task
						}
					} else if !errors.Is(taskErr, gorm.ErrRecordNotFound) {
						u.logger.Error("get latest node learning task by doc id failed",
							log.String("doc_id", doc.ID),
							log.Error(taskErr))
					}
				}

				node, err := u.nodeRepo.GetNodeByID(ctx, nodeID)
				if err != nil {
					u.logger.Error("get node failed before sync rag status",
						log.String("kb_id", kb.ID),
						log.String("node_id", nodeID),
						log.Error(err))
					continue
				}

				status := consts.NodeRagInfoStatus(strings.ToUpper(doc.Status))
				observedTask := activeTask
				if observedTask == nil {
					observedTask = latestTask
				}
				if observedTask != nil {
					if err := u.nodeRepo.ObserveNodeLearningRAG(ctx, kb.ID, nodeID, observedTask.ID, kb.DatasetID, doc.ID, string(status), nodeLearningRagStatusMessage(status, doc.ProgressMsg), "poll", ""); err != nil {
						u.logger.Error("record RAG status observation failed", log.Error(err))
					}
				}
				ragInfo := node.RagInfo
				if activeTask != nil {
					ragInfo.TaskID = activeTask.ID
					ragInfo.TaskAction = activeTask.Action
					ragInfo.TaskMessage = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
				}
				switch status {
				case consts.NodeRagStatusSucceeded:
					completionTask := activeTask
					if completionTask == nil &&
						latestTask != nil &&
						latestTask.Status != consts.NodeLearningTaskCanceled &&
						latestTask.Status != consts.NodeLearningTaskCancelRequested {
						completionTask = latestTask
					}
					ragInfo.Status = consts.NodeRagStatusSucceeded
					ragInfo.Message = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
					ragInfo.ActiveDocID = doc.ID
					ragInfo.ActiveReleaseID = release.ID
					if completionTask != nil {
						ragInfo.TaskID = completionTask.ID
						ragInfo.TaskAction = completionTask.Action
						ragInfo.TaskStatus = consts.NodeLearningTaskSucceeded
						ragInfo.TaskMessage = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
						if err := u.nodeRepo.FinishNodeLearningTask(ctx, completionTask.ID, consts.NodeLearningTaskSucceeded, doc.ID, ""); err != nil {
							u.logger.Error("finish node learning task as succeeded failed",
								log.String("task_id", completionTask.ID),
								log.String("doc_id", doc.ID),
								log.Error(err))
						}
					} else if shouldReconcileSucceededRagTaskStatus(ragInfo.TaskStatus) {
						ragInfo.TaskStatus = consts.NodeLearningTaskSucceeded
						ragInfo.TaskMessage = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
					}
				case consts.NodeRagStatusFailed:
					if !ragInfo.HasAvailableIndex() {
						ragInfo.Status = consts.NodeRagStatusFailed
						ragInfo.Message = doc.ProgressMsg
					}
					if activeTask != nil {
						ragInfo.TaskStatus = consts.NodeLearningTaskFailed
						if err := u.nodeRepo.FinishNodeLearningTask(ctx, activeTask.ID, consts.NodeLearningTaskFailed, doc.ID, doc.ProgressMsg); err != nil {
							u.logger.Error("finish node learning task as failed failed",
								log.String("task_id", activeTask.ID),
								log.String("doc_id", doc.ID),
								log.Error(err))
						}
					}
				case consts.NodeRagStatusPending, consts.NodeRagStatusRunning, consts.NodeRagStatusReindexing:
					if !ragInfo.HasAvailableIndex() {
						ragInfo.Status = status
						ragInfo.Message = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
					}
					if activeTask != nil {
						ragInfo.TaskStatus = consts.NodeLearningTaskRunning
						ragInfo.TaskMessage = nodeLearningRagStatusMessage(status, doc.ProgressMsg)
					}
				default:
					continue
				}
				ragInfo.SyncedAt = time.Now()
				if err := u.nodeRepo.UpdateNodeRagInfo(ctx, kb.ID, nodeID, ragInfo); err != nil {
					u.logger.Error("sync node rag status failed",
						log.String("kb_id", kb.ID),
						log.String("node_id", nodeID),
						log.String("status", doc.Status),
						log.Error(err))
				}
			}
		}
	}

	return nil
}

func (u *NodeUsecase) DispatchWaitingNodeLearningTasksForAllKB(ctx context.Context) error {
	kbs, err := u.kbRepo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return err
	}
	for _, kb := range kbs {
		if _, err := u.DispatchWaitingNodeLearningTasks(ctx, kb.ID); err != nil {
			u.logger.Error("dispatch waiting node learning tasks failed",
				log.String("kb_id", kb.ID),
				log.Error(err))
		}
	}
	return nil
}

func (u *NodeUsecase) DispatchWaitingNodeLearningTasks(ctx context.Context, kbID string) (int, error) {
	runningCount, err := u.nodeRepo.CountNodeLearningTasksByStatus(ctx, kbID, consts.NodeLearningTaskRunning)
	if err != nil {
		return 0, err
	}
	available := nodeLearningMaxRunningTasks - int(runningCount)
	if available <= 0 {
		return 0, nil
	}
	limit := nodeLearningDispatchBatchSize
	if available < limit {
		limit = available
	}
	tasks, err := u.nodeRepo.AcquireWaitingNodeLearningTasks(ctx, kbID, limit)
	if err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	dispatched := 0
	for _, task := range tasks {
		if err := u.dispatchNodeLearningTask(ctx, task); err != nil {
			u.logger.Error("dispatch node learning task failed",
				log.String("kb_id", task.KBID),
				log.String("node_id", task.NodeID),
				log.String("task_id", task.ID),
				log.Error(err))
			continue
		}
		dispatched++
	}
	if dispatched > 0 {
		u.logger.Info("dispatched waiting node learning tasks",
			log.String("kb_id", kbID),
			log.Int("count", dispatched),
			log.Int("max_running", nodeLearningMaxRunningTasks))
	}
	return dispatched, nil
}

func (u *NodeUsecase) dispatchNodeLearningTask(ctx context.Context, task *domain.NodeLearningTask) error {
	node, err := u.nodeRepo.GetNodeByID(ctx, task.NodeID)
	if err != nil {
		_ = u.nodeRepo.FinishNodeLearningTask(ctx, task.ID, consts.NodeLearningTaskFailed, task.DocID, err.Error())
		return err
	}
	ragInfo := node.RagInfo
	if !ragInfo.HasAvailableIndex() {
		ragInfo.Status = consts.NodeRagStatusRunning
		ragInfo.Message = "正在提交索引任务"
	}
	ragInfo.TaskID = task.ID
	ragInfo.TaskAction = task.Action
	ragInfo.TaskStatus = consts.NodeLearningTaskRunning
	ragInfo.TaskMessage = "已进入索引队列"
	ragInfo.SyncedAt = time.Now()
	if err := u.nodeRepo.UpdateNodeRagInfo(ctx, task.KBID, task.NodeID, ragInfo); err != nil {
		_ = u.nodeRepo.FinishNodeLearningTask(ctx, task.ID, consts.NodeLearningTaskFailed, task.DocID, err.Error())
		return err
	}
	if err := u.nodeRepo.RecordNodeLearningStage(ctx, task.KBID, task.NodeID, task.ID, "mq_waiting", "任务已调度，提交消息队列并等待后台消费者取走"); err != nil {
		u.logger.Error("record queued learning stage failed", log.Error(err))
	}
	if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, []*domain.NodeReleaseVectorRequest{
		{
			KBID:          task.KBID,
			NodeReleaseID: task.NodeReleaseID,
			NodeID:        task.NodeID,
			TaskID:        task.ID,
			Action:        "upsert",
		},
	}); err != nil {
		ragInfo.TaskStatus = consts.NodeLearningTaskFailed
		ragInfo.TaskMessage = err.Error()
		if !node.RagInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusFailed
			ragInfo.Message = err.Error()
		}
		_ = u.nodeRepo.UpdateNodeRagInfo(ctx, task.KBID, task.NodeID, ragInfo)
		_ = u.nodeRepo.FinishNodeLearningTask(ctx, task.ID, consts.NodeLearningTaskFailed, task.DocID, err.Error())
		return err
	}
	return nil
}

func shouldReconcileSucceededRagTaskStatus(status consts.NodeLearningTaskStatus) bool {
	switch status {
	case consts.NodeLearningTaskFailed:
		return true
	default:
		return false
	}
}

func (u *NodeUsecase) ExpireStaleNodeLearningTasks(ctx context.Context) (int, error) {
	// Scan from the shortest timeout so metadata-only files can be reclaimed
	// promptly; regular documents are filtered by their own timeout below.
	tasks, err := u.nodeRepo.ListStaleRunningNodeLearningTasks(ctx, time.Now().Add(-nodeLearningMetadataTaskTimeout), 200)
	if err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	expiredCount := 0
	for _, task := range tasks {
		timeout := nodeLearningTaskRunningTimeout
		if release, releaseErr := u.nodeRepo.GetNodeReleaseWithDirPathByID(ctx, task.NodeReleaseID); releaseErr == nil && !ShouldExtractReleaseFileContent(release) {
			timeout = nodeLearningMetadataTaskTimeout
		}
		startedAt := task.CreatedAt
		if task.StartedAt != nil {
			startedAt = *task.StartedAt
		}
		if time.Since(startedAt) < timeout {
			continue
		}
		message := fmt.Sprintf("索引重建超时：RAGLite 超过 %s 未返回最终状态，请检查 RAGLite/NATS 队列后重试重建索引", formatNodeLearningTimeout(timeout))
		if err := u.nodeRepo.FinishNodeLearningTask(ctx, task.ID, consts.NodeLearningTaskFailed, task.DocID, message); err != nil {
			u.logger.Error("finish stale node learning task failed",
				log.String("task_id", task.ID),
				log.String("node_id", task.NodeID),
				log.Error(err))
			continue
		}
		node, getNodeErr := u.nodeRepo.GetNodeByID(ctx, task.NodeID)
		if getNodeErr != nil {
			u.logger.Error("get node by id after stale learning task failed",
				log.String("task_id", task.ID),
				log.String("node_id", task.NodeID),
				log.Error(getNodeErr))
			continue
		}
		ragInfo := node.RagInfo
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusFailed
			ragInfo.Message = message
		}
		ragInfo.TaskID = task.ID
		ragInfo.TaskAction = task.Action
		ragInfo.TaskStatus = consts.NodeLearningTaskFailed
		ragInfo.TaskMessage = message
		ragInfo.SyncedAt = time.Now()
		if err := u.nodeRepo.UpdateNodeRagInfo(ctx, task.KBID, task.NodeID, ragInfo); err != nil {
			u.logger.Error("update node rag info after stale learning task failed",
				log.String("task_id", task.ID),
				log.String("node_id", task.NodeID),
				log.Error(err))
			continue
		}
		u.logger.Warn("expired stale node learning task",
			log.String("task_id", task.ID),
			log.String("node_id", task.NodeID),
			log.String("doc_id", task.DocID),
			log.String("timeout", timeout.String()))
		expiredCount++
	}
	return expiredCount, nil
}

func nodeLearningRagStatusMessage(status consts.NodeRagInfoStatus, message string) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	switch status {
	case consts.NodeRagStatusPending:
		return "RAGLite 已接收，等待索引"
	case consts.NodeRagStatusRunning:
		return "RAGLite 正在解析并生成向量"
	case consts.NodeRagStatusReindexing:
		return "RAGLite 正在重建索引"
	case consts.NodeRagStatusSucceeded:
		return "索引已完成"
	case consts.NodeRagStatusFailed:
		return "RAGLite 索引失败"
	default:
		return "等待后台更新状态"
	}
}

func formatNodeLearningTimeout(timeout time.Duration) string {
	if timeout > 0 && timeout%time.Hour == 0 {
		return fmt.Sprintf("%d 小时", int(timeout/time.Hour))
	}
	if timeout > 0 && timeout%time.Minute == 0 {
		return fmt.Sprintf("%d 分钟", int(timeout/time.Minute))
	}
	return timeout.String()
}

func (u *NodeUsecase) NodeLearningCancel(ctx context.Context, req *v1.NodeLearningCancelReq) (int, error) {
	const message = "已手动停止"
	tasks, err := u.nodeRepo.CancelActiveNodeLearningTasks(ctx, req.KbId, req.NodeIds, message)
	if err != nil {
		u.logger.Error("cancel node learning tasks failed", log.Error(err))
		return 0, fmt.Errorf("停止学习任务失败")
	}
	for _, task := range tasks {
		node, getNodeErr := u.nodeRepo.GetNodeByID(ctx, task.NodeID)
		if getNodeErr != nil {
			u.logger.Error("get node by id after cancel learning task failed",
				log.String("node_id", task.NodeID),
				log.String("task_id", task.ID),
				log.Error(getNodeErr))
			continue
		}
		ragInfo := node.RagInfo
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusPending
			ragInfo.Message = message
		}
		ragInfo.TaskStatus = consts.NodeLearningTaskCanceled
		ragInfo.TaskID = task.ID
		ragInfo.TaskAction = task.Action
		ragInfo.TaskMessage = message
		ragInfo.SyncedAt = time.Now()
		if err := u.nodeRepo.UpdateNodeRagInfo(ctx, req.KbId, task.NodeID, ragInfo); err != nil {
			u.logger.Error("update node rag info after cancel learning task failed",
				log.String("node_id", task.NodeID),
				log.String("task_id", task.ID),
				log.Error(err))
		}
	}
	return len(tasks), nil
}

func (u *NodeUsecase) NodeRestudy(ctx context.Context, req *v1.NodeRestudyReq) error {
	nodeReleases, err := u.nodeRepo.GetLatestNodeReleaseByNodeIDs(ctx, req.KbId, req.NodeIds)
	if err != nil {
		u.logger.Error("get latest node release failed", log.Error(err))
		return fmt.Errorf("get latest node release failed")
	}

	if len(nodeReleases) == 0 {
		return fmt.Errorf("文档未首次发布，无法重新学习")
	}

	priority := req.Priority
	if priority <= 0 {
		priority = nodeLearningBulkPriority
	}
	if priority > nodeLearningManualPriority {
		priority = nodeLearningManualPriority
	}

	createdCount := 0
	for _, nodeRelease := range nodeReleases {
		if nodeRelease.Type == domain.NodeTypeFolder {
			continue
		}
		node, getNodeErr := u.nodeRepo.GetNodeByID(ctx, nodeRelease.NodeID)
		if getNodeErr != nil {
			u.logger.Error("get node by id failed",
				log.String("node_id", nodeRelease.NodeID),
				log.Error(getNodeErr))
			continue
		}
		task, created, taskErr := u.nodeRepo.CreateNodeLearningTaskIfNotActive(ctx, &domain.NodeLearningTask{
			KBID:          nodeRelease.KBID,
			NodeID:        nodeRelease.NodeID,
			NodeReleaseID: nodeRelease.ID,
			Action:        consts.NodeLearningTaskActionRebuild,
			Status:        consts.NodeLearningTaskWaiting,
			Priority:      priority,
			TriggeredBy:   req.UserID,
		})
		if taskErr != nil {
			u.logger.Error("create node learning task failed",
				log.String("node_id", nodeRelease.NodeID),
				log.Error(taskErr))
			continue
		}
		if !created {
			u.logger.Info("skip restudy because node learning task already exists",
				log.String("node_id", nodeRelease.NodeID),
				log.String("task_id", task.ID),
				log.String("task_status", string(task.Status)))
			continue
		}
		createdCount++

		ragInfo := node.RagInfo
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusPending
			ragInfo.Message = "等待进入索引队列"
		} else {
			ragInfo.Message = ""
		}
		ragInfo.TaskStatus = consts.NodeLearningTaskWaiting
		ragInfo.TaskID = task.ID
		ragInfo.TaskAction = task.Action
		ragInfo.TaskMessage = "排队中"
		ragInfo.SyncedAt = time.Now()
		if err := u.nodeRepo.UpdateNodeRagInfo(ctx, nodeRelease.KBID, nodeRelease.NodeID, domain.RagInfo{
			Status:          ragInfo.Status,
			Message:         ragInfo.Message,
			SyncedAt:        ragInfo.SyncedAt,
			TaskStatus:      ragInfo.TaskStatus,
			TaskID:          ragInfo.TaskID,
			TaskAction:      ragInfo.TaskAction,
			TaskMessage:     ragInfo.TaskMessage,
			ActiveDocID:     ragInfo.ActiveDocID,
			ActiveReleaseID: ragInfo.ActiveReleaseID,
		}); err != nil {
			u.logger.Error("update node rag info failed",
				log.String("node_id", nodeRelease.NodeID),
				log.Error(err))
			_ = u.nodeRepo.FinishNodeLearningTask(ctx, task.ID, consts.NodeLearningTaskFailed, "", err.Error())
			continue
		}
	}

	if createdCount > 0 {
		if _, err := u.DispatchWaitingNodeLearningTasks(ctx, req.KbId); err != nil {
			u.logger.Error("dispatch waiting node learning tasks after restudy failed",
				log.String("kb_id", req.KbId),
				log.Error(err))
			return fmt.Errorf("提交学习任务失败")
		}
	}

	return nil
}

func (u *NodeUsecase) GetNodeStats(ctx context.Context, kbId string) (*v1.NodeStatsResp, error) {
	resp, err := u.nodeRepo.GetNodeStats(ctx, kbId)
	if err != nil {
		return nil, err
	}

	navs, err := u.navRepo.GetList(ctx, kbId)
	if err != nil {
		return nil, err
	}

	navsReleased, err := u.navRepo.GetReleaseList(ctx, kbId)
	if err != nil {
		return nil, err
	}

	navsReleasedMap := make(map[string]*navV1.NavListResp, len(navsReleased))
	for _, nr := range navsReleased {
		navsReleasedMap[nr.ID] = &nr
	}

	for _, nav := range navs {
		navsRelease, found := navsReleasedMap[nav.ID]
		if !found || navsRelease.Position != nav.Position || navsRelease.Name != nav.Name {
			resp.UnreleasedNavCount++
		}
	}
	return resp, nil
}

func (u *NodeUsecase) GetNodeListGroupByNav(ctx context.Context, req v1.NodeListGroupNavReq) ([]*v1.NodeListGroupNavResp, error) {
	nodes, err := u.nodeRepo.GetNodeListByStatus(ctx, req.KbId, req.Status, req.Search)
	if err != nil {
		return nil, err
	}

	navs, err := u.navRepo.GetListByIds(ctx, req.KbId, req.NavIds)
	if err != nil {
		return nil, err
	}

	navsReleased, err := u.navRepo.GetReleaseList(ctx, req.KbId)
	if err != nil {
		return nil, err
	}

	navsReleasedMap := make(map[string]*navV1.NavListResp, len(navsReleased))
	for _, nr := range navsReleased {
		navsReleasedMap[nr.ID] = &nr
	}

	nodes, err = u.filterManagedNodes(ctx, req.KbId, nodes)
	if err != nil {
		return nil, err
	}
	// Keep navigation roots for readable or manageable descendants.
	visibleNavs := map[string]bool{}
	for _, node := range nodes {
		visibleNavs[node.NavId] = true
	}
	management, err := u.managementAccess(ctx, req.KbId)
	if err != nil {
		return nil, err
	}
	if !management.Global {
		filtered := navs[:0]
		for _, nav := range navs {
			if visibleNavs[nav.ID] || management.CanManageNav(nav.ID) {
				filtered = append(filtered, nav)
			}
		}
		navs = filtered
	}

	// 按 position 顺序预建分组，用 map 做 O(1) 索引
	result := make([]*v1.NodeListGroupNavResp, 0, len(navs))
	navIndexMap := make(map[string]int, len(navs))
	for _, nav := range navs {
		release, found := navsReleasedMap[nav.ID]
		navIndexMap[nav.ID] = len(result)
		result = append(result, &v1.NodeListGroupNavResp{
			NavID:             nav.ID,
			CanManage:         management.Global || management.CanManageNav(nav.ID),
			CanCreateDocument: management.Global || management.CanCreateInNav("", nav.ID, domain.NodeTypeDocument),
			CanCreateFolder:   management.Global || management.CanCreateInNav("", nav.ID, domain.NodeTypeFolder),
			CanConfigure:      management.Global || management.CanManageNav(nav.ID),
			Permissions:       nav.Permissions,
			NavName:           nav.Name,
			Position:          nav.Position,
			IsReleased:        found && release.Position == nav.Position && release.Name == nav.Name,
			List:              []domain.NodeListItemResp{},
		})
	}

	for _, node := range nodes {
		if idx, ok := navIndexMap[node.NavId]; ok {
			result[idx].List = append(result[idx].List, *node)
			result[idx].Count++
		}
	}

	// 搜索时过滤掉空分组
	if req.Search != "" {
		filtered := make([]*v1.NodeListGroupNavResp, 0, len(result))
		for _, group := range result {
			if group.Count > 0 {
				filtered = append(filtered, group)
			}
		}
		return filtered, nil
	}

	return result, nil
}
