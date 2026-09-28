package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	v1 "github.com/chaitin/panda-wiki/api/kb/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/cache"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/rag"
)

type KnowledgeBaseUsecase struct {
	repo        *pg.KnowledgeBaseRepository
	nodeRepo    *pg.NodeRepository
	navRepo     *pg.NavRepository
	ragRepo     *mq.RAGRepository
	userRepo    *pg.UserRepository
	rag         rag.RAGService
	batchImport *BatchImportUsecase
	kbCache     *cache.KBRepo
	logger      *log.Logger
	config      *config.Config
	fileUsecase *FileUsecase
}

type learnedRelease struct {
	release *domain.NodeReleaseWithDirPath
	docID   string
}

func NewKnowledgeBaseUsecase(repo *pg.KnowledgeBaseRepository, nodeRepo *pg.NodeRepository, navRepo *pg.NavRepository, ragRepo *mq.RAGRepository, userRepo *pg.UserRepository, rag rag.RAGService, kbCache *cache.KBRepo, logger *log.Logger, config *config.Config) (*KnowledgeBaseUsecase, error) {
	u := &KnowledgeBaseUsecase{
		repo:     repo,
		nodeRepo: nodeRepo,
		navRepo:  navRepo,
		ragRepo:  ragRepo,
		userRepo: userRepo,
		rag:      rag,
		logger:   logger.WithModule("usecase.knowledge_base"),
		config:   config,
		kbCache:  kbCache,
	}
	return u, nil
}

func (u *KnowledgeBaseUsecase) SetBatchImportUsecase(batchImport *BatchImportUsecase) {
	u.batchImport = batchImport
}

func (u *KnowledgeBaseUsecase) SetFileUsecase(fileUsecase *FileUsecase) {
	u.fileUsecase = fileUsecase
}

func (u *KnowledgeBaseUsecase) CreateKnowledgeBase(ctx context.Context, req *domain.CreateKnowledgeBaseReq) (string, error) {
	// create kb in vector store
	datasetID, err := u.rag.CreateKnowledgeBase(ctx)
	if err != nil {
		return "", err
	}
	kbID := uuid.New().String()
	kb := &domain.KnowledgeBase{
		ID:        kbID,
		Name:      req.Name,
		DatasetID: datasetID,
		AccessSettings: domain.AccessSettings{
			Ports:      req.Ports,
			SSLPorts:   req.SSLPorts,
			PublicKey:  req.PublicKey,
			PrivateKey: req.PrivateKey,
			Hosts:      req.Hosts,
		},
	}

	if err := u.repo.CreateKnowledgeBase(ctx, req.MaxKB, kb); err != nil {
		return "", err
	}

	nav := &domain.Nav{
		ID:   uuid.New().String(),
		Name: req.Name,
		KbID: kbID,
	}
	if err := u.navRepo.Create(ctx, nav, nil); err != nil {
		return "", err
	}

	return kbID, nil
}

func (u *KnowledgeBaseUsecase) EnsureDefaultKnowledgeBase(ctx context.Context) error {
	kbList, err := u.repo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return err
	}
	if len(kbList) > 0 {
		return nil
	}

	_, err = u.CreateKnowledgeBase(ctx, &domain.CreateKnowledgeBaseReq{
		Name:  "默认知识库",
		MaxKB: max(domain.GetBaseEditionLimitation(ctx).MaxKb, 1),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize default knowledge base: %w", err)
	}

	return nil
}

func (u *KnowledgeBaseUsecase) GetKnowledgeBaseList(ctx context.Context) ([]*domain.KnowledgeBaseListItem, error) {
	if err := u.EnsureDefaultKnowledgeBase(ctx); err != nil {
		return nil, err
	}
	knowledgeBases, err := u.repo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeBases, nil
}

func (u *KnowledgeBaseUsecase) GetDefaultKnowledgeBase(ctx context.Context) (*domain.KnowledgeBaseListItem, error) {
	knowledgeBases, err := u.GetKnowledgeBaseList(ctx)
	if err != nil {
		return nil, err
	}
	if len(knowledgeBases) == 0 {
		return nil, fmt.Errorf("knowledge base not found")
	}
	return knowledgeBases[0], nil
}

func (u *KnowledgeBaseUsecase) GetKnowledgeBaseListByUserId(ctx context.Context) ([]*domain.KnowledgeBaseListItem, error) {
	if err := u.EnsureDefaultKnowledgeBase(ctx); err != nil {
		return nil, err
	}
	knowledgeBases, err := u.repo.GetKnowledgeBaseListByUserId(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeBases, nil
}

func (u *KnowledgeBaseUsecase) GetDefaultKnowledgeBaseByUser(ctx context.Context) (*domain.KnowledgeBaseListItem, error) {
	knowledgeBases, err := u.GetKnowledgeBaseListByUserId(ctx)
	if err != nil {
		return nil, err
	}
	if len(knowledgeBases) == 0 {
		return nil, fmt.Errorf("knowledge base not found")
	}
	return knowledgeBases[0], nil
}

func (u *KnowledgeBaseUsecase) UpdateKnowledgeBase(ctx context.Context, req *domain.UpdateKnowledgeBaseReq) error {
	isChange, err := u.repo.UpdateKnowledgeBase(ctx, req)
	if err != nil {
		return err
	}

	if isChange {
		if err := u.kbCache.ClearSession(ctx); err != nil {
			return err
		}
	}

	if err := u.kbCache.DeleteKB(ctx, req.ID); err != nil {
		return err
	}

	return nil
}

func (u *KnowledgeBaseUsecase) GetKnowledgeBase(ctx context.Context, kbID string) (*domain.KnowledgeBase, error) {
	kb, err := u.kbCache.GetKB(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb != nil {
		return kb, nil
	}
	kb, err = u.repo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if err := u.kbCache.SetKB(ctx, kbID, kb); err != nil {
		return nil, err
	}
	return kb, nil
}

func (u *KnowledgeBaseUsecase) GetKnowledgeBasePerm(ctx context.Context, kbID string) (consts.UserKBPermission, error) {

	perm, err := u.repo.GetKBPermByUserId(ctx, kbID)
	if err != nil {
		return "", err
	}

	return perm, nil
}

func (u *KnowledgeBaseUsecase) DeleteKnowledgeBase(ctx context.Context, kbID string) error {
	if err := u.repo.DeleteKnowledgeBase(ctx, kbID); err != nil {
		return err
	}
	// delete vector store
	if err := u.rag.DeleteKnowledgeBase(ctx, kbID); err != nil {
		return err
	}
	if err := u.kbCache.DeleteKB(ctx, kbID); err != nil {
		return err
	}
	return nil
}

func (u *KnowledgeBaseUsecase) CreateKBRelease(ctx context.Context, req *domain.CreateKBReleaseReq, userId string) (string, error) {
	lockToken := uuid.New().String()
	locked, err := u.kbCache.AcquirePublicationLock(ctx, req.KBID, lockToken, 15*time.Minute)
	if err != nil {
		return "", fmt.Errorf("获取发布锁失败: %w", err)
	}
	if !locked {
		return "", fmt.Errorf("当前已有发布任务正在进行，请稍后再试")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := u.kbCache.ReleasePublicationLock(unlockCtx, req.KBID, lockToken); err != nil {
			u.logger.Error("release publication lock failed", log.String("kb_id", req.KBID), log.Error(err))
		}
	}()

	var releaseIDs []string
	if len(req.NodeIDs) > 0 {
		if u.batchImport == nil {
			return "", fmt.Errorf("ZIP 导入服务未初始化")
		}
		nodes, nodesErr := u.nodeRepo.GetNodesByIDs(ctx, req.NodeIDs)
		if nodesErr != nil {
			return "", fmt.Errorf("读取发布文档失败: %w", nodesErr)
		}
		originalIDs := make([]string, 0, len(req.NodeIDs))
		for _, id := range req.NodeIDs {
			if node := nodes[id]; node == nil || !node.Meta.ArchivePending {
				originalIDs = append(originalIDs, id)
			}
		}
		archiveNodeIDs, expandErr := u.batchImport.ExpandArchives(ctx, req.KBID, req.NodeIDs, userId, true)
		if expandErr != nil {
			return "", fmt.Errorf("解析 ZIP 失败: %w", expandErr)
		}
		if len(archiveNodeIDs) > 0 {
			req.NodeIDs = append(originalIDs, archiveNodeIDs...)
		} else {
			req.NodeIDs = originalIDs
		}
		releaseIDs, err = u.nodeRepo.CreatePendingNodeReleases(ctx, req.KBID, userId, req.NodeIDs)
		if err != nil {
			return "", fmt.Errorf("创建待发布版本失败: %w", err)
		}
		if err := u.snapshotReleasesToOSS(ctx, req.KBID, releaseIDs); err != nil {
			return "", fmt.Errorf("创建版本 OSS 快照失败: %w", err)
		}
	}

	release := &domain.KBRelease{
		ID:          uuid.New().String(),
		KBID:        req.KBID,
		Message:     req.Message,
		Tag:         req.Tag,
		PublisherId: userId,
		CreatedAt:   time.Now(),
	}
	if err := u.repo.PublishKBRelease(ctx, release, req.NodeIDs); err != nil {
		u.cleanupPendingPublication("", releaseIDs, nil, err.Error())
		return "", fmt.Errorf("failed to create kb release: %w", err)
	}
	if len(releaseIDs) > 0 {
		u.startReleaseLearning(req.KBID, release.ID, releaseIDs)
	}
	u.logger.Info("publication committed", log.String("kb_id", req.KBID), log.String("release_id", release.ID), log.String("stage", "publish"), log.Int("document_count", len(releaseIDs)))

	return release.ID, nil
}

func (u *KnowledgeBaseUsecase) startReleaseLearning(kbID, releaseID string, releaseIDs []string) {
	ids := append([]string(nil), releaseIDs...)
	go func() {
		learnCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		u.logger.Info("publication learning queued", log.String("kb_id", kbID), log.String("release_id", releaseID), log.String("stage", "learning"), log.Int("document_count", len(ids)))
		learned, datasetID, err := u.learnPendingReleases(learnCtx, kbID, ids)
		if err != nil {
			u.logger.Error("publication learning async failed", log.String("kb_id", kbID), log.String("release_id", releaseID), log.String("stage", "learning"), log.Error(err))
			return
		}
		u.cleanupOldPublicationRecords(datasetID, learned)
		u.logger.Info("publication learning finished", log.String("kb_id", kbID), log.String("release_id", releaseID), log.String("stage", "learning"), log.Int("document_count", len(learned)))
	}()
}

func (u *KnowledgeBaseUsecase) snapshotReleasesToOSS(ctx context.Context, kbID string, releaseIDs []string) error {
	if u.fileUsecase == nil {
		return fmt.Errorf("文件存储服务未初始化")
	}
	for _, releaseID := range releaseIDs {
		release, err := u.nodeRepo.GetNodeReleaseByID(ctx, releaseID)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"version":        1,
			"release_id":     release.ID,
			"node_id":        release.NodeID,
			"name":           release.Name,
			"content":        release.Content,
			"editor_content": release.EditorContent,
			"editor_blocks":  release.EditorBlocks,
			"editor_format":  release.EditorFormat,
			"editor_version": release.EditorVersion,
			"content_type":   release.Meta.ContentType,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("_versions/%s/%s/%s.json", kbID, release.NodeID, release.ID)
		if _, err := u.fileUsecase.UploadBytesAtKey(ctx, key, release.Name+".json", data); err != nil {
			return err
		}
		release.Meta.VersionKey = key
		release.Meta.VersionFormat = "panda-wiki-v1"
		if err := u.nodeRepo.UpdateNodeReleaseMeta(ctx, release.ID, release.Meta); err != nil {
			return err
		}
	}
	return nil
}

func (u *KnowledgeBaseUsecase) learnPendingReleases(ctx context.Context, kbID string, releaseIDs []string) ([]learnedRelease, string, error) {
	if len(releaseIDs) == 0 {
		return nil, "", nil
	}
	kb, err := u.repo.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, "", fmt.Errorf("读取知识库失败: %w", err)
	}
	learned := make([]learnedRelease, 0, len(releaseIDs))
	for _, releaseID := range releaseIDs {
		release, err := u.nodeRepo.GetNodeReleaseWithDirPathByID(ctx, releaseID)
		if err != nil {
			return learned, kb.DatasetID, fmt.Errorf("读取待发布文档失败: %w", err)
		}
		if release.Type == domain.NodeTypeFolder {
			continue
		}
		if strings.TrimSpace(release.Content) == "" && release.Meta.FileKey != "" {
			content := ""
			contentOK := false
			contentMessage := ""
			if u.batchImport != nil && ShouldExtractReleaseFileContent(release) {
				content, contentOK, contentMessage = u.batchImport.extractFileContent(ctx, kbID, release.Meta.FileKey, release.Name)
			} else {
				contentMessage = "该文件类型不解析正文，仅索引文件元信息"
			}
			if !contentOK || strings.TrimSpace(content) == "" {
				if contentMessage == "" {
					contentMessage = "内容解析未返回有效文本，仅索引文件元信息"
				}
				content = BuildFileMetadataLearningContent(release, contentMessage)
				u.logger.Warn("publication content extraction fallback to file metadata", log.String("kb_id", kbID), log.String("node_id", release.NodeID), log.String("node_release_id", release.ID), log.String("stage", "learning"), log.String("document", release.Name), log.String("reason", contentMessage))
			}
			release.Content = content
			if err := u.nodeRepo.UpdateNodeReleaseContent(ctx, release.ID, content); err != nil {
				return learned, kb.DatasetID, fmt.Errorf("保存文档“%s”解析内容失败: %w", release.Name, err)
			}
		}
		u.logger.Info("publication learning started", log.String("kb_id", kbID), log.String("node_release_id", release.ID), log.String("stage", "learning"))
		node, nodeErr := u.nodeRepo.GetNodeByID(ctx, release.NodeID)
		if nodeErr != nil {
			return learned, kb.DatasetID, fmt.Errorf("读取文档学习状态失败: %w", nodeErr)
		}
		ragInfo := node.RagInfo
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusRunning
			ragInfo.Message = ""
		}
		ragInfo.TaskStatus = consts.NodeLearningTaskRunning
		ragInfo.TaskAction = consts.NodeLearningTaskActionPublish
		ragInfo.TaskMessage = ""
		ragInfo.SyncedAt = time.Now()
		if err := u.nodeRepo.UpdateNodeRagInfo(ctx, kbID, release.NodeID, ragInfo); err != nil {
			return learned, kb.DatasetID, fmt.Errorf("更新学习状态失败: %w", err)
		}
		docID, err := u.rag.UpsertRecords(ctx, &rag.UpsertRecordsRequest{
			ID:        release.ID,
			Title:     release.Name,
			DatasetID: kb.DatasetID,
			Content:   release.Content,
			GroupIDs:  nil,
		})
		if err != nil {
			u.markLearningFailed(kbID, release.NodeID, err.Error())
			u.logger.Error("publication learning submit failed", log.String("kb_id", kbID), log.String("node_id", release.NodeID), log.String("node_release_id", release.ID), log.String("stage", "learning"), log.String("document", release.Name), log.Error(err))
			continue
		}
		learned = append(learned, learnedRelease{release: release, docID: docID})
		if err := u.nodeRepo.UpdateNodeReleaseDocID(ctx, release.ID, docID); err != nil {
			return learned, kb.DatasetID, fmt.Errorf("保存学习任务标识失败: %w", err)
		}
	}
	if len(learned) == 0 {
		return learned, kb.DatasetID, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	pending := make(map[string]learnedRelease, len(learned))
	docIDs := make([]string, 0, len(learned))
	for _, item := range learned {
		pending[item.docID] = item
		docIDs = append(docIDs, item.docID)
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for len(pending) > 0 {
		documents, err := u.rag.ListDocuments(waitCtx, kb.DatasetID, docIDs)
		if err != nil {
			u.markPendingLearningFailed(kbID, pending, err.Error())
			u.logger.Error("publication learning progress query failed", log.String("kb_id", kbID), log.String("stage", "learning"), log.Error(err))
			break
		}
		for _, document := range documents {
			item, exists := pending[document.ID]
			if !exists {
				continue
			}
			switch strings.ToUpper(document.Status) {
			case string(consts.NodeRagStatusSucceeded):
				delete(pending, document.ID)
				_ = u.nodeRepo.UpdateNodeRagInfo(ctx, kbID, item.release.NodeID, domain.RagInfo{
					Status:          consts.NodeRagStatusSucceeded,
					Message:         document.ProgressMsg,
					SyncedAt:        time.Now(),
					TaskStatus:      consts.NodeLearningTaskSucceeded,
					TaskAction:      consts.NodeLearningTaskActionPublish,
					TaskMessage:     document.ProgressMsg,
					ActiveDocID:     document.ID,
					ActiveReleaseID: item.release.ID,
				})
				u.logger.Info("publication learning succeeded", log.String("kb_id", kbID), log.String("node_release_id", item.release.ID), log.String("stage", "learning"))
			case string(consts.NodeRagStatusFailed):
				u.markLearningFailed(kbID, item.release.NodeID, document.ProgressMsg)
				u.logger.Error("publication learning failed", log.String("kb_id", kbID), log.String("node_release_id", item.release.ID), log.String("stage", "learning"), log.String("reason", document.ProgressMsg))
				delete(pending, document.ID)
			}
		}
		if len(pending) == 0 {
			break
		}
		select {
		case <-waitCtx.Done():
			u.markPendingLearningFailed(kbID, pending, "等待学习完成超时")
			u.logger.Error("publication learning timed out", log.String("kb_id", kbID), log.String("stage", "learning"), log.Error(waitCtx.Err()))
			return learned, kb.DatasetID, nil
		case <-ticker.C:
		}
	}
	return learned, kb.DatasetID, nil
}

func ShouldExtractReleaseFileContent(release *domain.NodeReleaseWithDirPath) bool {
	fileType := normalizeReleaseFileType(release)
	switch fileType {
	case "mp4", "webm", "ogv", "mov", "m4v",
		"mp3", "wav", "ogg", "oga", "m4a", "aac", "flac",
		"png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif",
		"zip", "rar", "7z", "tar", "gz":
		return false
	default:
		return true
	}
}

func normalizeReleaseFileType(release *domain.NodeReleaseWithDirPath) string {
	if release == nil {
		return ""
	}
	fileType := strings.ToLower(strings.TrimSpace(release.Meta.FileType))
	if strings.HasPrefix(fileType, ".") {
		fileType = strings.TrimPrefix(fileType, ".")
	}
	if strings.Contains(fileType, "/") {
		parts := strings.Split(fileType, "/")
		fileType = parts[len(parts)-1]
	}
	if fileType != "" {
		return fileType
	}
	fileName := release.Meta.FileName
	if fileName == "" {
		fileName = release.Name
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
}

func BuildFileMetadataLearningContent(release *domain.NodeReleaseWithDirPath, reason string) string {
	fileName := strings.TrimSpace(release.Meta.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(release.Name)
	}
	fileType := normalizeReleaseFileType(release)
	pathText := strings.TrimSpace(release.Path)
	if pathText == "" {
		pathText = "/"
	}
	baseName := strings.TrimSpace(strings.TrimSuffix(fileName, filepath.Ext(fileName)))

	lines := []string{
		fmt.Sprintf("# %s", strings.TrimSpace(release.Name)),
		"",
		"这是一个文件型文档。系统当前已索引该文件的基础元信息，可用于按名称、路径、类型检索。",
		fmt.Sprintf("文档名称：%s", strings.TrimSpace(release.Name)),
		fmt.Sprintf("文件名称：%s", fileName),
	}
	if baseName != "" && baseName != fileName {
		lines = append(lines, fmt.Sprintf("文件关键词：%s", baseName))
	}
	if fileType != "" {
		lines = append(lines, fmt.Sprintf("文件类型：%s", fileType))
	}
	lines = append(lines, fmt.Sprintf("所在路径：%s", pathText))
	if reason != "" {
		lines = append(lines, fmt.Sprintf("正文解析状态：%s。", sanitizeLearningFallbackReason(reason)))
	}
	return strings.Join(lines, "\n")
}

func sanitizeLearningFallbackReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	if len([]rune(reason)) > 120 {
		reason = string([]rune(reason)[:120]) + "..."
	}
	return reason
}

func (u *KnowledgeBaseUsecase) markPendingLearningFailed(kbID string, pending map[string]learnedRelease, message string) {
	for _, item := range pending {
		u.markLearningFailed(kbID, item.release.NodeID, message)
	}
}

func (u *KnowledgeBaseUsecase) cleanupPendingPublication(datasetID string, releaseIDs []string, learned []learnedRelease, reason string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	docIDs := make([]string, 0, len(learned))
	for _, item := range learned {
		u.markLearningFailed(item.release.KBID, item.release.NodeID, reason)
		if item.docID != "" {
			docIDs = append(docIDs, item.docID)
		}
	}
	if datasetID != "" && len(docIDs) > 0 {
		if err := u.rag.DeleteRecords(cleanupCtx, datasetID, docIDs); err != nil {
			u.logger.Error("publication cleanup vectors failed", log.String("dataset_id", datasetID), log.Int("document_count", len(docIDs)), log.Error(err))
		}
	}
	if err := u.nodeRepo.DeleteNodeReleases(cleanupCtx, releaseIDs); err != nil {
		u.logger.Error("publication cleanup snapshots failed", log.Int("release_count", len(releaseIDs)), log.Error(err))
	}
}

func (u *KnowledgeBaseUsecase) cleanupOldPublicationRecords(datasetID string, learned []learnedRelease) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, item := range learned {
		oldDocIDs, err := u.nodeRepo.ListOldNodeDocIDsByNodeID(cleanupCtx, item.release.ID, item.release.NodeID)
		if err != nil {
			u.logger.Error("publication list old vectors failed", log.String("node_id", item.release.NodeID), log.Error(err))
			continue
		}
		if len(oldDocIDs) == 0 {
			continue
		}
		if err := u.rag.DeleteRecords(cleanupCtx, datasetID, oldDocIDs); err != nil {
			u.logger.Error("publication delete old vectors failed", log.String("node_id", item.release.NodeID), log.Int("document_count", len(oldDocIDs)), log.Error(err))
			continue
		}
		if err := u.nodeRepo.ClearNodeReleaseDocIDs(cleanupCtx, oldDocIDs); err != nil {
			u.logger.Error("publication clear old vector references failed", log.String("node_id", item.release.NodeID), log.Int("document_count", len(oldDocIDs)), log.Error(err))
		}
	}
}

func (u *KnowledgeBaseUsecase) markLearningFailed(kbID, nodeID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	node, err := u.nodeRepo.GetNodeByID(ctx, nodeID)
	if err != nil {
		_ = u.nodeRepo.UpdateNodeRagInfo(ctx, kbID, nodeID, domain.RagInfo{Status: consts.NodeRagStatusFailed, Message: message, SyncedAt: time.Now()})
		return
	}
	ragInfo := node.RagInfo
	if !ragInfo.HasAvailableIndex() {
		ragInfo.Status = consts.NodeRagStatusFailed
		ragInfo.Message = message
	}
	ragInfo.TaskStatus = consts.NodeLearningTaskFailed
	ragInfo.TaskAction = consts.NodeLearningTaskActionPublish
	ragInfo.TaskMessage = message
	ragInfo.SyncedAt = time.Now()
	_ = u.nodeRepo.UpdateNodeRagInfo(ctx, kbID, nodeID, ragInfo)
}

func (u *KnowledgeBaseUsecase) GetKBReleaseList(ctx context.Context, req *domain.GetKBReleaseListReq) (*domain.GetKBReleaseListResp, error) {
	total, releases, err := u.repo.GetKBReleaseList(ctx, req.KBID, req.Offset(), req.Limit())
	if err != nil {
		return nil, err
	}

	return domain.NewPaginatedResult(releases, uint64(total)), nil
}

func (u *KnowledgeBaseUsecase) GetKBUserList(ctx context.Context, req v1.KBUserListReq) ([]v1.KBUserListItemResp, error) {
	users, err := u.repo.GetKBUserlist(ctx, req.KBId)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (u *KnowledgeBaseUsecase) KBUserInvite(ctx context.Context, req v1.KBUserInviteReq) error {
	user, err := u.userRepo.GetUser(ctx, req.UserId)
	if err != nil {
		return err
	}
	if user.Role == consts.UserRoleAdmin {
		return fmt.Errorf("knowledge base can not invite to admin user")
	}

	if err := u.repo.CreateKBUser(ctx, &domain.KBUsers{
		KBId:      req.KBId,
		UserId:    req.UserId,
		Perm:      req.Perm,
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}

	return nil
}

func (u *KnowledgeBaseUsecase) UpdateUserKB(ctx context.Context, req v1.KBUserUpdateReq) error {
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return fmt.Errorf("authInfo not found in context")
	}

	kbUser, err := u.repo.GetKBUser(ctx, req.KBId, req.UserId)
	if err != nil {
		return err
	}
	if authInfo.IsToken {
		if authInfo.KBId != req.KBId {
			return fmt.Errorf("invalid knowledge base token")
		}
		if authInfo.Permission != consts.UserKBPermissionFullControl {
			return fmt.Errorf("only admin can update user from knowledge base")
		}
	} else {
		user, err := u.userRepo.GetUser(ctx, authInfo.UserId)
		if err != nil {
			return err
		}
		if user.Role != consts.UserRoleAdmin && kbUser.Perm != consts.UserKBPermissionFullControl {
			return fmt.Errorf("only admin can update user from knowledge base")
		}
	}
	return u.repo.UpdateKBUserPerm(ctx, req.KBId, req.UserId, req.Perm)
}

func (u *KnowledgeBaseUsecase) KBUserDelete(ctx context.Context, req v1.KBUserDeleteReq) error {
	authInfo := domain.GetAuthInfoFromCtx(ctx)
	if authInfo == nil {
		return fmt.Errorf("authInfo not found in context")
	}

	kbUser, err := u.repo.GetKBUser(ctx, req.KBId, req.UserId)
	if err != nil {
		return err
	}
	if authInfo.IsToken {
		if authInfo.KBId != req.KBId {
			return fmt.Errorf("knowledge base can not delete user from knowledge base")
		}
		if authInfo.Permission != consts.UserKBPermissionFullControl {
			return fmt.Errorf("only admin can delete user from knowledge base")
		}
	} else {
		user, err := u.userRepo.GetUser(ctx, authInfo.UserId)
		if err != nil {
			return err
		}
		if user.Role != consts.UserRoleAdmin && kbUser.Perm != consts.UserKBPermissionFullControl {
			return fmt.Errorf("only admin can delete user from knowledge base")
		}
	}
	if err := u.repo.DeleteKBUser(ctx, req.KBId, req.UserId); err != nil {
		return err
	}

	return nil
}
