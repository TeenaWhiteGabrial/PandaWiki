package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	crawlerV1 "github.com/chaitin/panda-wiki/api/crawler/v1"
	mcpV1 "github.com/chaitin/panda-wiki/api/mcp/v1"
	nodeV1 "github.com/chaitin/panda-wiki/api/node/v1"
	shareV1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/pkg/anydoc"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type MCPUsecase struct {
	nodeRepo       *pg.NodeRepository
	navRepo        *pg.NavRepository
	mcpRepo        *pg.MCPRepository
	userUsecase    *UserUsecase
	nodeUsecase    *NodeUsecase
	kbUsecase      *KnowledgeBaseUsecase
	enterprise     *EnterpriseUsecase
	fileUsecase    *FileUsecase
	crawlerUsecase *CrawlerUsecase
	logger         *log.Logger
}

type mcpResourceNode struct {
	Node    *domain.Node
	Release *shareV1.ShareNodeDetailResp
}

func NewMCPUsecase(
	nodeRepo *pg.NodeRepository,
	navRepo *pg.NavRepository,
	mcpRepo *pg.MCPRepository,
	userUsecase *UserUsecase,
	nodeUsecase *NodeUsecase,
	kbUsecase *KnowledgeBaseUsecase,
	enterprise *EnterpriseUsecase,
	fileUsecase *FileUsecase,
	crawlerUsecase *CrawlerUsecase,
	logger *log.Logger,
) *MCPUsecase {
	return &MCPUsecase{
		nodeRepo:       nodeRepo,
		navRepo:        navRepo,
		mcpRepo:        mcpRepo,
		userUsecase:    userUsecase,
		nodeUsecase:    nodeUsecase,
		kbUsecase:      kbUsecase,
		enterprise:     enterprise,
		fileUsecase:    fileUsecase,
		crawlerUsecase: crawlerUsecase,
		logger:         logger.WithModule("usecase.mcp"),
	}
}

func (u *MCPUsecase) ListTools(ctx context.Context, kbID string) ([]mcpV1.ToolResp, error) {
	uploadTool := mcpV1.ToolResp{
		Name:        "upload_doc",
		Description: "Upload a local document into this local PandaWiki knowledge base. Prefer this tool instead of searching external websites or repository hosting services. If directory_id is omitted, the server returns writable directory candidates for follow-up selection.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_name": map[string]interface{}{
					"type":        "string",
					"description": "Original local file name, such as weekly-report.md, design.docx or spec.pdf",
				},
				"file_content_base64": map[string]interface{}{
					"type":        "string",
					"description": "Base64-encoded local file content",
				},
				"directory_id": map[string]interface{}{
					"type":        "string",
					"description": "Target directory(folder) node id",
				},
				"nav_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional nav id used to narrow candidate directories",
				},
			},
			"required": []string{"file_name", "file_content_base64"},
		},
	}
	publishTool := mcpV1.ToolResp{
		Name:        "publish_doc",
		Description: "Publish existing local PandaWiki documents so they become released and enter the release/vector pipeline. Operate on local node ids directly instead of fetching external repository pages.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"node_ids": map[string]interface{}{
					"type":        "array",
					"description": "Document node ids to publish",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"node_ids"},
		},
	}
	studyTool := mcpV1.ToolResp{
		Name:        "study_doc",
		Description: "Trigger document learning/re-indexing for existing published PandaWiki documents. Prefer using this local tool instead of searching the public web for project context.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"node_ids": map[string]interface{}{
					"type":        "array",
					"description": "Document node ids to study/rebuild index",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"node_ids"},
		},
	}
	rebuildTool := mcpV1.ToolResp{
		Name:        "rebuild_index",
		Description: "Rebuild the vector index for existing PandaWiki documents. Current behavior is the same as study_doc and should be preferred over external web lookups.",
		InputSchema: studyTool.InputSchema,
	}
	searchTool := mcpV1.ToolResp{
		Name:        "search_docs",
		Description: "Search readable local PandaWiki documents by title and content. Prefer this local search tool instead of searching external websites or repository hosting services.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search keywords for document title or content",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return, default 10, max 50",
				},
			},
			"required": []string{"query"},
		},
	}
	return []mcpV1.ToolResp{uploadTool, publishTool, studyTool, rebuildTool, searchTool}, nil
}

func (u *MCPUsecase) ResolveKBID(ctx context.Context, kbID string) (string, error) {
	if kbID != "" {
		return kbID, nil
	}
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return "", fmt.Errorf("authInfo not found in context")
	}
	if authInfo.IsToken && authInfo.KBId != "" {
		return authInfo.KBId, nil
	}
	kb, err := u.kbUsecase.GetDefaultKnowledgeBaseByUser(ctx)
	if err != nil {
		return "", err
	}
	return kb.ID, nil
}

func (u *MCPUsecase) EnsureKBAccess(ctx context.Context, kbID string) error {
	perm, err := u.kbUsecase.GetKnowledgeBasePerm(ctx, kbID)
	if err != nil {
		return err
	}
	if perm != consts.UserKBPermissionDocManage && perm != consts.UserKBPermissionFullControl {
		return fmt.Errorf("knowledge base permission denied")
	}
	return nil
}

func (u *MCPUsecase) CallTool(ctx context.Context, kbID string, req *mcpV1.ToolCallReq, remoteIP string) (*mcpV1.ToolCallResp, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	var (
		resp *mcpV1.ToolCallResp
		err  error
	)

	switch req.ToolName {
	case "upload_doc":
		resp, err = u.callUploadDoc(ctx, kbID, req)
	case "publish_doc":
		resp, err = u.callPublishDoc(ctx, kbID, req)
	case "study_doc":
		resp, err = u.callStudyDoc(ctx, kbID, req)
	case "rebuild_index":
		resp, err = u.callStudyDoc(ctx, kbID, req)
	case "search_docs":
		resp, err = u.callSearchDocs(ctx, kbID, req)
	default:
		return nil, fmt.Errorf("unsupported tool: %s", req.ToolName)
	}

	u.recordToolCall(ctx, kbID, req, resp, remoteIP)
	return resp, err
}

func (u *MCPUsecase) Initialize(ctx context.Context, kbID string, req *mcpV1.InitializeParams, remoteIP string) (*mcpV1.InitializeResult, error) {
	result := &mcpV1.InitializeResult{
		ProtocolVersion: mcpV1.DefaultProtocolVersion,
		ServerInfo: mcpV1.ServerInfo{
			Name:    "pandawiki-mcp",
			Version: "1.0.0",
		},
		Capabilities: mcpV1.MCPServerCaps{
			Tools: map[string]interface{}{
				"listChanged": false,
			},
			Resources: map[string]interface{}{
				"listChanged": false,
			},
		},
		Instructions: "这是一个本地 PandaWiki 知识库 MCP Server。请优先使用本 MCP 提供的 tools 和 resources 完成知识库操作，不要为了理解项目或查找目录而先访问 GitHub、Gitee 或其他外部网站。所有 MCP 调用都会复用 PandaWiki 当前用户权限；无权访问的栏目、目录和文档不会出现在 tools 或 resources 的返回中。当需要目录、文档或发布/学习操作时，应直接调用本地 MCP 能力。",
	}

	if u.mcpRepo != nil {
		reqBytes, _ := json.Marshal(req)
		respBytes, _ := json.Marshal(result)
		if err := u.mcpRepo.RecordToolCall(ctx, &domain.MCPCall{
			MCPSessionID:   req.SessionID,
			KBID:           kbID,
			RemoteIP:       remoteIP,
			InitializeReq:  reqBytes,
			InitializeResp: respBytes,
		}); err != nil {
			u.logger.Error("record mcp initialize failed", log.Error(err))
		}
	}

	return result, nil
}

func (u *MCPUsecase) ListResources(ctx context.Context, kbID string) (*mcpV1.ResourceListResult, error) {
	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return nil, err
	}

	navs, err := u.navRepo.GetList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	nodes, err := u.nodeRepo.GetList(ctx, &domain.GetNodeListReq{KBID: kbID})
	if err != nil {
		return nil, err
	}

	resources := make([]mcpV1.Resource, 0, len(navs)+len(nodes))
	for _, nav := range navs {
		if !u.enterprise.CanVisible(nav.Permissions, access) {
			continue
		}
		resources = append(resources, mcpV1.Resource{
			URI:         fmt.Sprintf("nav://%s", nav.ID),
			Name:        nav.Name,
			Description: "栏目资源",
			MimeType:    "application/json",
		})
	}

	for _, node := range nodes {
		nodeForResolve := &domain.Node{
			ID:          node.ID,
			KBID:        kbID,
			NavId:       node.NavId,
			ParentID:    node.ParentID,
			Name:        node.Name,
			Type:        node.Type,
			Permissions: node.Permissions,
		}
		resolvedPerm, resolveErr := u.enterprise.ResolveNodePermissions(ctx, nodeForResolve)
		if resolveErr == nil {
			node.Permissions = resolvedPerm
		}
		if !u.enterprise.CanVisible(node.Permissions, access) {
			continue
		}

		description := "目录资源"
		mimeType := "application/json"
		if node.Type == domain.NodeTypeDocument {
			description = "文档资源"
			mimeType = "text/markdown"
		}
		resources = append(resources, mcpV1.Resource{
			URI:         fmt.Sprintf("node://%s", node.ID),
			Name:        node.Name,
			Description: description,
			MimeType:    mimeType,
		})
	}

	return &mcpV1.ResourceListResult{Resources: resources}, nil
}

func (u *MCPUsecase) ReadResource(ctx context.Context, kbID, uri string) (*mcpV1.ResourceReadResult, error) {
	scheme, id, err := parseResourceURI(uri)
	if err != nil {
		return nil, err
	}

	switch scheme {
	case "nav":
		return u.readNavResource(ctx, kbID, id)
	case "node":
		return u.readNodeResource(ctx, kbID, id)
	default:
		return nil, fmt.Errorf("unsupported resource uri: %s", uri)
	}
}

func (u *MCPUsecase) callUploadDoc(ctx context.Context, kbID string, req *mcpV1.ToolCallReq) (*mcpV1.ToolCallResp, error) {
	var args mcpV1.UploadDocArgs
	if len(req.Arguments) > 0 {
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if strings.TrimSpace(args.FileName) == "" {
		return nil, fmt.Errorf("file_name is required")
	}
	if strings.TrimSpace(args.FileContentBase64) == "" {
		return nil, fmt.Errorf("file_content_base64 is required")
	}

	if strings.TrimSpace(args.DirectoryID) == "" {
		candidates, recommended, err := u.listWritableDirectories(ctx, kbID, args.NavID)
		if err != nil {
			return nil, err
		}
		return &mcpV1.ToolCallResp{
			Status:                mcpV1.ToolCallStatusNeedsInput,
			ToolName:              req.ToolName,
			Message:               "请选择要创建文档的目录",
			MissingFields:         []string{"directory_id"},
			Candidates:            candidates,
			RecommendedCandidates: recommended,
		}, nil
	}

	directoryNode, err := u.nodeRepo.GetNodeByID(ctx, args.DirectoryID)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %w", err)
	}
	if directoryNode.Type != domain.NodeTypeFolder {
		return nil, fmt.Errorf("directory_id must point to a folder")
	}
	if kbID == "" {
		kbID = directoryNode.KBID
	}
	if directoryNode.KBID != kbID {
		return nil, fmt.Errorf("directory does not belong to current knowledge base")
	}
	if err := u.ensureWritableDirectory(ctx, directoryNode); err != nil {
		return nil, err
	}

	fileBytes, err := base64.StdEncoding.DecodeString(args.FileContentBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid file_content_base64: %w", err)
	}

	title := strings.TrimSuffix(strings.TrimSpace(args.FileName), filepath.Ext(args.FileName))
	if title == "" {
		return nil, fmt.Errorf("file_name does not contain a valid title")
	}

	content := string(fileBytes)
	if !isDirectMarkdownFile(args.FileName) {
		content, err = u.importLocalFileToMarkdown(ctx, kbID, args.FileName, fileBytes)
		if err != nil {
			return nil, err
		}
	}

	reqCreate := &domain.CreateNodeReq{
		KBID:        directoryNode.KBID,
		NavId:       directoryNode.NavId,
		ParentID:    directoryNode.ID,
		Type:        domain.NodeTypeDocument,
		Name:        title,
		Content:     content,
		ContentType: ptrString("md"),
	}
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return nil, fmt.Errorf("authInfo not found in context")
	}
	id, err := u.nodeUsecase.Create(ctx, reqCreate, authInfo.UserId)
	if err != nil {
		return nil, err
	}
	return &mcpV1.ToolCallResp{
		Status:   mcpV1.ToolCallStatusSuccess,
		ToolName: req.ToolName,
		Result: mcpV1.UploadDocResult{
			ID:          id,
			KBID:        directoryNode.KBID,
			NavID:       directoryNode.NavId,
			DirectoryID: directoryNode.ID,
			FileName:    args.FileName,
			Title:       reqCreate.Name,
		},
	}, nil
}

func (u *MCPUsecase) callPublishDoc(ctx context.Context, kbID string, req *mcpV1.ToolCallReq) (*mcpV1.ToolCallResp, error) {
	var args mcpV1.PublishDocArgs
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid publish_doc arguments: %w", err)
	}
	if len(args.NodeIDs) == 0 {
		return nil, fmt.Errorf("node_ids is required")
	}

	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil || authInfo.UserId == "" {
		return nil, fmt.Errorf("auth info is required")
	}

	releaseID, err := u.kbUsecase.CreateKBRelease(ctx, &domain.CreateKBReleaseReq{
		KBID:    kbID,
		NodeIDs: args.NodeIDs,
	}, authInfo.UserId)
	if err != nil {
		return nil, err
	}

	return &mcpV1.ToolCallResp{
		Status:   mcpV1.ToolCallStatusSuccess,
		ToolName: req.ToolName,
		Result: mcpV1.PublishDocResult{
			KBID:      kbID,
			NodeIDs:   args.NodeIDs,
			ReleaseID: releaseID,
		},
	}, nil
}

func (u *MCPUsecase) callStudyDoc(ctx context.Context, kbID string, req *mcpV1.ToolCallReq) (*mcpV1.ToolCallResp, error) {
	var args mcpV1.StudyDocArgs
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid %s arguments: %w", req.ToolName, err)
	}
	if len(args.NodeIDs) == 0 {
		return nil, fmt.Errorf("node_ids is required")
	}

	if err := u.nodeUsecase.NodeRestudy(ctx, &nodeV1.NodeRestudyReq{
		NodeIds: args.NodeIDs,
		KbId:    kbID,
	}); err != nil {
		return nil, err
	}

	return &mcpV1.ToolCallResp{
		Status:   mcpV1.ToolCallStatusSuccess,
		ToolName: req.ToolName,
		Result: mcpV1.StudyDocResult{
			KBID:    kbID,
			NodeIDs: args.NodeIDs,
		},
	}, nil
}

func (u *MCPUsecase) callSearchDocs(ctx context.Context, kbID string, req *mcpV1.ToolCallReq) (*mcpV1.ToolCallResp, error) {
	var args mcpV1.SearchDocsArgs
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid search_docs arguments: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return nil, err
	}

	navs, err := u.navRepo.GetList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	navNameMap := make(map[string]string, len(navs))
	for _, nav := range navs {
		navNameMap[nav.ID] = nav.Name
	}

	nodes, err := u.nodeRepo.GetNodeListByStatus(ctx, kbID, "", strings.TrimSpace(args.Query))
	if err != nil {
		return nil, err
	}

	items := make([]mcpV1.SearchDocItem, 0, min(limit, len(nodes)))
	for _, node := range nodes {
		if node.Type != domain.NodeTypeDocument {
			continue
		}
		nodeForResolve := &domain.Node{
			ID:          node.ID,
			KBID:        kbID,
			NavId:       node.NavId,
			ParentID:    node.ParentID,
			Name:        node.Name,
			Type:        node.Type,
			Permissions: node.Permissions,
		}
		if resolvedPermissions, resolveErr := u.enterprise.ResolveNodePermissions(ctx, nodeForResolve); resolveErr == nil {
			node.Permissions = resolvedPermissions
		}
		if !u.enterprise.CanVisible(node.Permissions, access) {
			continue
		}

		items = append(items, mcpV1.SearchDocItem{
			ID:      node.ID,
			Name:    node.Name,
			NavID:   node.NavId,
			NavName: navNameMap[node.NavId],
			Path:    u.buildCandidatePath(ctx, kbID, node),
			Summary: node.Summary,
			URI:     fmt.Sprintf("node://%s", node.ID),
		})
		if len(items) >= limit {
			break
		}
	}

	return &mcpV1.ToolCallResp{
		Status:   mcpV1.ToolCallStatusSuccess,
		ToolName: req.ToolName,
		Result: mcpV1.SearchDocsResult{
			KBID:  kbID,
			Query: strings.TrimSpace(args.Query),
			Items: items,
		},
	}, nil
}

func (u *MCPUsecase) listWritableDirectories(ctx context.Context, kbID, navID string) ([]mcpV1.DirectoryCandidate, []mcpV1.DirectoryCandidate, error) {
	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return nil, nil, err
	}

	nodes, err := u.nodeRepo.GetList(ctx, &domain.GetNodeListReq{
		KBID:  kbID,
		NavId: navID,
	})
	if err != nil {
		return nil, nil, err
	}

	navs, err := u.navRepo.GetList(ctx, kbID)
	if err != nil {
		return nil, nil, err
	}
	navMap := make(map[string]string, len(navs))
	for _, nav := range navs {
		navMap[nav.ID] = nav.Name
	}

	dirCandidates := make([]mcpV1.DirectoryCandidate, 0)
	for _, node := range nodes {
		if node.Type != domain.NodeTypeFolder {
			continue
		}

		nodeForResolve := &domain.Node{
			ID:          node.ID,
			KBID:        kbID,
			NavId:       node.NavId,
			ParentID:    node.ParentID,
			Name:        node.Name,
			Type:        node.Type,
			Permissions: node.Permissions,
		}
		resolvedPerm, resolveErr := u.enterprise.ResolveNodePermissions(ctx, nodeForResolve)
		if resolveErr == nil {
			node.Permissions = resolvedPerm
		}
		if !u.enterprise.CanVisible(node.Permissions, access) {
			continue
		}

		dirCandidates = append(dirCandidates, mcpV1.DirectoryCandidate{
			ID:      node.ID,
			Name:    node.Name,
			NavID:   node.NavId,
			NavName: navMap[node.NavId],
			Path:    u.buildCandidatePath(ctx, kbID, node),
		})
	}

	recommended := u.recommendDirectories(ctx, dirCandidates)
	return dirCandidates, recommended, nil
}

func (u *MCPUsecase) recommendDirectories(ctx context.Context, candidates []mcpV1.DirectoryCandidate) []mcpV1.DirectoryCandidate {
	if len(candidates) == 0 {
		return nil
	}
	recommended := make([]mcpV1.DirectoryCandidate, 0, 3)
	for _, candidate := range candidates {
		if strings.Contains(candidate.NavName, "公共") {
			candidate.RecommendReason = "公共目录"
		} else {
			candidate.RecommendReason = "推荐可写目录"
		}
		recommended = append(recommended, candidate)
		if len(recommended) >= 3 {
			break
		}
	}
	return recommended
}

func (u *MCPUsecase) ensureWritableDirectory(ctx context.Context, directoryNode *domain.Node) error {
	if directoryNode == nil {
		return fmt.Errorf("directory is required")
	}
	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return err
	}
	resolvedPerm, resolveErr := u.enterprise.ResolveNodePermissions(ctx, directoryNode)
	if resolveErr == nil {
		directoryNode.Permissions = resolvedPerm
	}
	if !u.enterprise.CanVisible(directoryNode.Permissions, access) {
		return fmt.Errorf("directory is not writable for current user")
	}
	return nil
}

func (u *MCPUsecase) importLocalFileToMarkdown(ctx context.Context, kbID, fileName string, fileBytes []byte) (string, error) {
	if u.fileUsecase == nil || u.crawlerUsecase == nil {
		return "", fmt.Errorf("file import pipeline is not available")
	}

	key, err := u.fileUsecase.UploadFileFromBytes(ctx, kbID, fileName, fileBytes)
	if err != nil {
		return "", fmt.Errorf("upload source file failed: %w", err)
	}

	parseResp, err := u.crawlerUsecase.ParseUrl(ctx, &crawlerV1.CrawlerParseReq{
		Key:           key,
		KbID:          kbID,
		CrawlerSource: consts.CrawlerSourceFile,
		Filename:      fileName,
	})
	if err != nil {
		return "", fmt.Errorf("parse source file failed: %w", err)
	}

	doc, ok := findFirstAnydocFile(parseResp.Docs)
	if !ok || doc.ID == "" {
		return "", fmt.Errorf("no parsable document found in source file")
	}

	exportResp, err := u.crawlerUsecase.ExportDoc(ctx, &crawlerV1.CrawlerExportReq{
		KbID:     kbID,
		ID:       parseResp.ID,
		DocID:    doc.ID,
		FileType: doc.FileType,
	})
	if err != nil {
		return "", fmt.Errorf("export parsed document failed: %w", err)
	}

	resultResp, err := u.crawlerUsecase.ScrapeGetResult(ctx, exportResp.TaskId)
	if err != nil {
		return "", fmt.Errorf("fetch parsed document result failed: %w", err)
	}
	if resultResp.Status != consts.CrawlerStatusCompleted {
		return "", fmt.Errorf("document parse not completed, status=%s", resultResp.Status)
	}
	if strings.TrimSpace(resultResp.Content) == "" {
		return "", fmt.Errorf("parsed document content is empty")
	}

	return resultResp.Content, nil
}

func (u *MCPUsecase) currentUserAccess(ctx context.Context) (*domain.EnterpriseAccessContext, error) {
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return nil, fmt.Errorf("authInfo not found in context")
	}
	if authInfo.IsToken {
		return u.enterprise.GetUserAccessContext(ctx, authInfo.UserId)
	}
	return u.enterprise.GetUserAccessContext(ctx, authInfo.UserId)
}

func (u *MCPUsecase) readNavResource(ctx context.Context, kbID, navID string) (*mcpV1.ResourceReadResult, error) {
	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return nil, err
	}

	nav, err := u.navRepo.GetById(ctx, navID)
	if err != nil {
		return nil, fmt.Errorf("nav not found: %w", err)
	}
	if nav.KbID != kbID {
		return nil, fmt.Errorf("nav does not belong to current knowledge base")
	}
	if !u.enterprise.CanVisible(nav.Permissions, access) {
		return nil, fmt.Errorf("resource is not visible for current user")
	}

	payload := map[string]interface{}{
		"id":          nav.ID,
		"name":        nav.Name,
		"kb_id":       nav.KbID,
		"position":    nav.Position,
		"permissions": nav.Permissions,
	}
	text, err := marshalPrettyJSON(payload)
	if err != nil {
		return nil, err
	}

	return &mcpV1.ResourceReadResult{
		Contents: []mcpV1.ResourceContent{
			{
				URI:      fmt.Sprintf("nav://%s", nav.ID),
				MimeType: "application/json",
				Text:     text,
			},
		},
	}, nil
}

func (u *MCPUsecase) readNodeResource(ctx context.Context, kbID, nodeID string) (*mcpV1.ResourceReadResult, error) {
	loaded, err := u.loadReadableNodeResource(ctx, kbID, nodeID)
	if err != nil {
		return nil, err
	}

	if loaded.Node.Type == domain.NodeTypeFolder {
		payload := map[string]interface{}{
			"id":          loaded.Node.ID,
			"name":        loaded.Node.Name,
			"kb_id":       loaded.Node.KBID,
			"nav_id":      loaded.Node.NavId,
			"type":        loaded.Node.Type,
			"parent_id":   loaded.Node.ParentID,
			"permissions": loaded.Node.Permissions,
		}
		text, err := marshalPrettyJSON(payload)
		if err != nil {
			return nil, err
		}
		return &mcpV1.ResourceReadResult{
			Contents: []mcpV1.ResourceContent{
				{
					URI:      fmt.Sprintf("node://%s", loaded.Node.ID),
					MimeType: "application/json",
					Text:     text,
				},
			},
		}, nil
	}

	content := loaded.Node.Content
	if loaded.Release != nil && loaded.Release.Content != "" {
		content = loaded.Release.Content
	}
	return &mcpV1.ResourceReadResult{
		Contents: []mcpV1.ResourceContent{
			{
				URI:      fmt.Sprintf("node://%s", loaded.Node.ID),
				MimeType: "text/markdown",
				Text:     content,
			},
		},
	}, nil
}

func (u *MCPUsecase) loadReadableNodeResource(ctx context.Context, kbID, nodeID string) (*mcpResourceNode, error) {
	access, err := u.currentUserAccess(ctx)
	if err != nil {
		return nil, err
	}

	node, err := u.nodeRepo.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}
	if node.KBID != kbID {
		return nil, fmt.Errorf("node does not belong to current knowledge base")
	}

	resolvedPerm, resolveErr := u.enterprise.ResolveNodePermissions(ctx, node)
	if resolveErr == nil {
		node.Permissions = resolvedPerm
	}
	if !u.enterprise.CanVisible(node.Permissions, access) {
		return nil, fmt.Errorf("resource is not visible for current user")
	}

	var release *shareV1.ShareNodeDetailResp
	if node.Type == domain.NodeTypeDocument {
		release, _ = u.nodeRepo.GetNodeReleaseDetailByKBIDAndID(ctx, kbID, nodeID)
	}
	return &mcpResourceNode{Node: node, Release: release}, nil
}

func (u *MCPUsecase) buildCandidatePath(ctx context.Context, kbID string, node *domain.NodeListItemResp) string {
	if node == nil {
		return "/"
	}
	parts := []string{node.Name}
	parentID := node.ParentID
	for depth := 0; depth < 10 && parentID != ""; depth++ {
		parentNode, err := u.nodeRepo.GetNodeByID(ctx, parentID)
		if err != nil {
			break
		}
		parts = append(parts, parentNode.Name)
		parentID = parentNode.ParentID
	}
	slices.Reverse(parts)
	return "/" + strings.Join(parts, "/")
}

func (u *MCPUsecase) recordToolCall(ctx context.Context, kbID string, req *mcpV1.ToolCallReq, resp *mcpV1.ToolCallResp, remoteIP string) {
	if u.mcpRepo == nil || req == nil {
		return
	}
	reqBytes, _ := json.Marshal(req)
	respBytes, _ := json.Marshal(resp)
	if err := u.mcpRepo.RecordToolCall(ctx, &domain.MCPCall{
		MCPSessionID: req.SessionID,
		KBID:         kbID,
		RemoteIP:     remoteIP,
		ToolCallReq:  reqBytes,
		ToolCallResp: string(respBytes),
	}); err != nil {
		u.logger.Error("record mcp tool call failed", log.Error(err))
	}
}

func parseResourceURI(uri string) (string, string, error) {
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid resource uri: %s", uri)
	}
	return parts[0], parts[1], nil
}

func isDirectMarkdownFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".md" || ext == ".markdown" || ext == ".txt"
}

func findFirstAnydocFile(root anydoc.Child) (anydoc.Value, bool) {
	if root.Value.File && root.Value.ID != "" {
		return root.Value, true
	}
	for _, child := range root.Children {
		if value, ok := findFirstAnydocFile(child); ok {
			return value, true
		}
	}
	return anydoc.Value{}, false
}

func marshalPrettyJSON(v interface{}) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func ptrString(v string) *string {
	return &v
}
