package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	modelkitDomain "github.com/chaitin/ModelKit/v2/domain"
	modelkit "github.com/chaitin/ModelKit/v2/usecase"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/rag"
)

type ModelUsecase struct {
	modelRepo         *pg.ModelRepository
	logger            *log.Logger
	config            *config.Config
	nodeRepo          modelIndexNodes
	ragRepo           *mq.RAGRepository
	ragStore          rag.RAGService
	kbRepo            modelIndexKnowledgeBases
	systemSettingRepo modelSettingsStore
	modelkit          *modelkit.ModelKit
}

func NewModelUsecase(modelRepo *pg.ModelRepository, nodeRepo *pg.NodeRepository, ragRepo *mq.RAGRepository, ragStore rag.RAGService, logger *log.Logger, config *config.Config, kbRepo *pg.KnowledgeBaseRepository, settingRepo *pg.SystemSettingRepo) *ModelUsecase {
	modelkit := modelkit.NewModelKit(logger.Logger)
	u := &ModelUsecase{
		modelRepo:         modelRepo,
		logger:            logger.WithModule("usecase.model"),
		config:            config,
		nodeRepo:          nodeRepo,
		ragRepo:           ragRepo,
		ragStore:          ragStore,
		kbRepo:            kbRepo,
		systemSettingRepo: settingRepo,
		modelkit:          modelkit,
	}
	return u
}

func (u *ModelUsecase) Create(ctx context.Context, model *domain.Model) error {
	var updatedEmbeddingModel bool
	if model.Type == domain.ModelTypeEmbedding {
		updatedEmbeddingModel = true
		setting, err := u.GetModelModeSetting(ctx)
		if err != nil {
			return err
		}
		// Preserve the baseline before creating the manual model record.
		if err := u.saveModelModeSetting(ctx, setting); err != nil {
			return err
		}
	}
	if err := u.modelRepo.Create(ctx, model); err != nil {
		return err
	}
	// 标记手动向量配置变更，应用模式时再决定是否重建。
	if updatedEmbeddingModel {
		if _, err := u.updateModeSettingConfig(ctx, &domain.SwitchModeReq{}, true); err != nil {
			return err
		}
	}
	return nil
}

func (u *ModelUsecase) normalizeModelModeSetting(config *domain.ModelModeSetting) {
	if config == nil {
		return
	}

	if config.AutoProvider != nil {
		provider := consts.NormalizeAutoModeProvider(string(config.AutoProvider.Provider))
		config.AutoProvider.Provider = domain.ModelProvider(provider)
		if config.AutoProvider.BaseURL == "" {
			if baseURL := consts.GetAutoModeBaseURL(provider); baseURL != "" {
				config.AutoProvider.BaseURL = baseURL
			}
		}
		if strings.TrimSpace(config.AutoProvider.BaseURLs.Rerank) == "" && strings.TrimSpace(config.AutoProvider.RerankBaseURL) != "" {
			config.AutoProvider.BaseURLs.Rerank = strings.TrimSpace(config.AutoProvider.RerankBaseURL)
			config.AutoProvider.RerankBaseURL = ""
		}
		config.AutoProvider.BaseURLs = normalizeAutoModeModelBaseURLs(config.AutoProvider.BaseURLs)
	}

	if config.AutoProvider == nil && len(config.AutoProviderConfigs) > 0 {
		for _, item := range config.AutoProviderConfigs {
			if item.ID != config.ActiveAutoProvider {
				continue
			}
			provider := consts.NormalizeAutoModeProvider(item.Provider)
			config.AutoProvider = &domain.AutoModeProviderSetting{
				Provider: domain.ModelProvider(provider),
				BaseURL:  item.BaseURL,
				BaseURLs: domain.AutoModeModelBaseURLs{},
				APIKey:   item.APIKey,
				Models: domain.AutoModeModelSelection{
					Chat:       consts.GetAutoModeDefaultModel(provider, string(domain.ModelTypeChat)),
					Embedding:  consts.GetAutoModeDefaultModel(provider, string(domain.ModelTypeEmbedding)),
					Rerank:     consts.GetAutoModeDefaultModel(provider, string(domain.ModelTypeRerank)),
					Analysis:   consts.GetAutoModeDefaultModel(provider, string(domain.ModelTypeAnalysis)),
					AnalysisVL: consts.GetAutoModeDefaultModel(provider, string(domain.ModelTypeAnalysisVL)),
				},
			}
			if config.AutoProvider.BaseURL == "" {
				config.AutoProvider.BaseURL = consts.GetAutoModeBaseURL(provider)
			}
			break
		}
	}

	if len(config.AutoProviderConfigs) == 0 && config.AutoModeAPIKey != "" {
		providerID := uuid.New().String()
		config.AutoProviderConfigs = []domain.AutoProviderSettingItem{
			{
				ID:       providerID,
				Provider: string(domain.ModelProviderBrandBaiZhiCloud),
				BaseURL:  consts.AutoModeBaseURL,
				APIKey:   config.AutoModeAPIKey,
			},
		}
		if config.ActiveAutoProvider == "" {
			config.ActiveAutoProvider = providerID
		}
		if config.AutoProvider == nil {
			config.AutoProvider = &domain.AutoModeProviderSetting{
				Provider: domain.ModelProviderBrandBaiZhiCloud,
				BaseURL:  consts.AutoModeBaseURL,
				BaseURLs: domain.AutoModeModelBaseURLs{},
				APIKey:   config.AutoModeAPIKey,
				Models: domain.AutoModeModelSelection{
					Chat:       consts.GetAutoModeDefaultModel(string(domain.ModelProviderBrandBaiZhiCloud), string(domain.ModelTypeChat)),
					Embedding:  consts.GetAutoModeDefaultModel(string(domain.ModelProviderBrandBaiZhiCloud), string(domain.ModelTypeEmbedding)),
					Rerank:     consts.GetAutoModeDefaultModel(string(domain.ModelProviderBrandBaiZhiCloud), string(domain.ModelTypeRerank)),
					Analysis:   consts.GetAutoModeDefaultModel(string(domain.ModelProviderBrandBaiZhiCloud), string(domain.ModelTypeAnalysis)),
					AnalysisVL: consts.GetAutoModeDefaultModel(string(domain.ModelProviderBrandBaiZhiCloud), string(domain.ModelTypeAnalysisVL)),
				},
			}
		}
	}

	for index := range config.AutoProviderConfigs {
		item := &config.AutoProviderConfigs[index]
		item.Provider = consts.NormalizeAutoModeProvider(item.Provider)
		if item.ID == "" {
			item.ID = uuid.New().String()
		}
		if item.BaseURL == "" {
			if baseURL := consts.GetAutoModeBaseURL(item.Provider); baseURL != "" {
				item.BaseURL = baseURL
			}
		}
	}

	if config.ActiveAutoProvider == "" && len(config.AutoProviderConfigs) > 0 {
		config.ActiveAutoProvider = config.AutoProviderConfigs[0].ID
	}
}

func (u *ModelUsecase) getAutoModeProvider(config domain.ModelModeSetting) (*domain.AutoModeProviderSetting, error) {
	u.normalizeModelModeSetting(&config)
	if config.AutoProvider == nil {
		return nil, fmt.Errorf("auto provider config is required")
	}
	return config.AutoProvider, nil
}

func (u *ModelUsecase) validateAutoProviderConfig(ctx context.Context, config *domain.AutoModeProviderReq) error {
	if config == nil {
		return fmt.Errorf("auto provider config is required")
	}
	normalizedProvider := consts.NormalizeAutoModeProvider(string(config.Provider))
	config.Provider = domain.ModelProvider(normalizedProvider)
	if !consts.SupportsAutoModeProvider(string(config.Provider)) {
		return fmt.Errorf("auto mode does not support provider %s", config.Provider)
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		return fmt.Errorf("auto provider base url is required")
	}
	modelsByType := map[domain.ModelType]string{
		domain.ModelTypeChat:       config.Models.Chat,
		domain.ModelTypeEmbedding:  config.Models.Embedding,
		domain.ModelTypeRerank:     config.Models.Rerank,
		domain.ModelTypeAnalysis:   config.Models.Analysis,
		domain.ModelTypeAnalysisVL: config.Models.AnalysisVL,
	}

	for modelType, modelName := range modelsByType {
		if strings.TrimSpace(modelName) == "" {
			return fmt.Errorf("auto mode %s model is required", modelType)
		}
		checkType := modelType
		if modelType == domain.ModelTypeAnalysis || modelType == domain.ModelTypeAnalysisVL {
			checkType = domain.ModelTypeChat
		}
		baseURL := getAutoModeReqModelBaseURL(config, modelType)
		check, err := u.modelkit.CheckModel(ctx, &modelkitDomain.CheckModelReq{
			Provider: normalizedProvider,
			Model:    modelName,
			BaseURL:  baseURL,
			APIKey:   config.APIKey,
			Type:     string(checkType),
		})
		if err != nil {
			return fmt.Errorf("auto provider %s model %s check failed: %w", config.Provider, modelName, err)
		}
		if check.Error != "" {
			return fmt.Errorf("auto provider %s model %s check failed: %s", config.Provider, modelName, check.Error)
		}
	}
	return nil
}

func (u *ModelUsecase) GetList(ctx context.Context) ([]*domain.ModelListItem, error) {
	return u.modelRepo.GetList(ctx)
}

// trigger upsert records after embedding model is updated or created
func (u *ModelUsecase) TriggerUpsertRecords(ctx context.Context) error {
	// update to new dataset
	kbList, err := u.kbRepo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return fmt.Errorf("get knowledge base list failed: %w", err)
	}
	for _, kb := range kbList {
		newDatasetID, err := u.ragStore.CreateKnowledgeBase(ctx)
		if err != nil {
			return fmt.Errorf("create new dataset failed: %w", err)
		}
		if err := u.ragStore.DeleteKnowledgeBase(ctx, kb.DatasetID); err != nil {
			return fmt.Errorf("delete old dataset failed: %w", err)
		}
		if err := u.kbRepo.UpdateDatasetID(ctx, kb.ID, newDatasetID); err != nil {
			return fmt.Errorf("update knowledge base dataset id failed: %w", err)
		}
	}
	// traverse all nodes
	err = u.nodeRepo.TraverseNodesByCursor(ctx, func(nodeRelease *domain.NodeRelease) error {
		// async upsert vector content via mq
		nodeContentVectorRequests := []*domain.NodeReleaseVectorRequest{
			{
				KBID:               nodeRelease.KBID,
				NodeReleaseID:      nodeRelease.ID,
				Action:             "upsert",
				ReuseParsedContent: true,
			},
		}
		if err := u.ragRepo.AsyncUpdateNodeReleaseVector(ctx, nodeContentVectorRequests); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (u *ModelUsecase) Update(ctx context.Context, req *domain.UpdateModelReq) error {
	oldModel, err := u.modelRepo.GetByID(ctx, req.ID)
	if err != nil {
		return err
	}
	setting, err := u.GetModelModeSetting(ctx)
	if err != nil {
		return err
	}
	if req.Type == domain.ModelTypeEmbedding || oldModel.Type == domain.ModelTypeEmbedding {
		// A later retry must still compare against the pre-edit index model.
		if err := u.saveModelModeSetting(ctx, setting); err != nil {
			return err
		}
	}
	if err := u.modelRepo.Update(ctx, req); err != nil {
		return err
	}
	data := &domain.Model{
		Provider:   req.Provider,
		Model:      req.Model,
		Type:       req.Type,
		APIKey:     req.APIKey,
		BaseURL:    req.BaseURL,
		APIHeader:  req.APIHeader,
		APIVersion: req.APIVersion,
		IsActive:   oldModel.IsActive,
	}
	if req.IsActive != nil {
		data.IsActive = *req.IsActive
	}
	if req.Parameters != nil {
		data.Parameters = *req.Parameters
	}
	// Manual model records must not replace the active automatic configuration.
	if setting.Mode == consts.ModelSettingModeManual {
		if err := u.ragStore.UpsertModel(ctx, data); err != nil {
			return err
		}
	}
	// 密钥和地址变更只同步连接配置，不标记为向量空间变更。
	if req.Type == domain.ModelTypeEmbedding {
		changed := *setting.EmbeddingIndexConfig != embeddingIndexConfig(data)
		if _, err := u.updateModeSettingConfig(ctx, &domain.SwitchModeReq{}, changed); err != nil {
			return err
		}
	}
	return nil
}

func (u *ModelUsecase) GetChatModel(ctx context.Context) (*domain.Model, error) {
	var model *domain.Model
	modelModeSetting, err := u.GetModelModeSetting(ctx)
	// 获取不到模型模式时，使用手动模式, 不返回错误
	if err != nil {
		u.logger.Error("get model mode setting failed, use manual mode", log.Error(err))
	}
	if err == nil && modelModeSetting.Mode == consts.ModelSettingModeAuto {
		activeProvider, providerErr := u.getAutoModeProvider(modelModeSetting)
		if providerErr == nil {
			model = &domain.Model{
				Model:    activeProvider.Models.Chat,
				Type:     domain.ModelTypeChat,
				IsActive: true,
				BaseURL:  getAutoModeModelBaseURL(activeProvider, domain.ModelTypeChat),
				APIKey:   activeProvider.APIKey,
				Provider: activeProvider.Provider,
			}
			return model, nil
		}
		u.logger.Warn("active auto provider is invalid, fallback to manual mode", log.Error(providerErr))
	}
	model, err = u.modelRepo.GetChatModel(ctx)
	if err != nil {
		return nil, err
	}

	return model, nil
}

func (u *ModelUsecase) GetModelByType(ctx context.Context, modelType domain.ModelType) (*domain.Model, error) {
	return u.modelRepo.GetModelByType(ctx, modelType)
}

func (u *ModelUsecase) UpdateUsage(ctx context.Context, modelID string, usage *schema.TokenUsage) error {
	return u.modelRepo.UpdateUsage(ctx, modelID, usage)
}

func (u *ModelUsecase) SwitchMode(ctx context.Context, req *domain.SwitchModeReq) error {
	switch consts.ModelSettingMode(req.Mode) {
	case consts.ModelSettingModeAuto:
		if err := u.validateAutoProviderConfig(ctx, req.AutoProvider); err != nil {
			return err
		}
	case consts.ModelSettingModeManual:
		needModelTypes := []domain.ModelType{
			domain.ModelTypeChat,
			domain.ModelTypeEmbedding,
			domain.ModelTypeRerank,
			domain.ModelTypeAnalysis,
		}
		for _, modelType := range needModelTypes {
			model, err := u.modelRepo.GetModelByType(ctx, modelType)
			if err != nil {
				return fmt.Errorf("需要配置 %s 模型", modelType)
			}

			if !model.IsActive {
				if err := u.modelRepo.Updates(ctx, model.ID, map[string]any{
					"is_active": true,
				}); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("invalid req mode: %s", req.Mode)
	}

	oldModelModeSetting, err := u.GetModelModeSetting(ctx)
	if err != nil {
		return err
	}

	var isResetEmbeddingUpdateFlag = true
	// 只有切换手动模式时，重置isManualEmbeddingUpdated为false
	if req.Mode == string(consts.ModelSettingModeManual) {
		isResetEmbeddingUpdateFlag = false
	}

	modelModeSetting, err := u.updateModeSettingConfig(ctx, req, isResetEmbeddingUpdateFlag)
	if err != nil {
		return err
	}

	if err := u.updateRAGModelsByMode(ctx, req.Mode, *modelModeSetting, oldModelModeSetting); err != nil {
		return err
	}

	return nil
}

// updateModeSettingConfig 读取当前设置并更新，然后持久化
func (u *ModelUsecase) updateModeSettingConfig(ctx context.Context, req *domain.SwitchModeReq, isManualEmbeddingUpdated bool) (*domain.ModelModeSetting, error) {
	// 读取当前设置
	config, err := u.GetModelModeSetting(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current model setting: %w", err)
	}

	// 更新设置
	if req.Mode != "" {
		config.Mode = consts.ModelSettingMode(req.Mode)
	}
	if req.Mode == string(consts.ModelSettingModeAuto) {
		if req.AutoProvider != nil {
			config.AutoProvider = &domain.AutoModeProviderSetting{
				Provider: req.AutoProvider.Provider,
				BaseURL:  req.AutoProvider.BaseURL,
				BaseURLs: normalizeAutoModeModelBaseURLs(domain.AutoModeModelBaseURLs{
					Chat:       req.AutoProvider.BaseURLs.Chat,
					Embedding:  req.AutoProvider.BaseURLs.Embedding,
					Rerank:     firstNonBlank(req.AutoProvider.BaseURLs.Rerank, req.AutoProvider.RerankBaseURL),
					Analysis:   req.AutoProvider.BaseURLs.Analysis,
					AnalysisVL: req.AutoProvider.BaseURLs.AnalysisVL,
				}),
				APIKey: req.AutoProvider.APIKey,
				Models: domain.AutoModeModelSelection{
					Chat:       req.AutoProvider.Models.Chat,
					Embedding:  req.AutoProvider.Models.Embedding,
					Rerank:     req.AutoProvider.Models.Rerank,
					Analysis:   req.AutoProvider.Models.Analysis,
					AnalysisVL: req.AutoProvider.Models.AnalysisVL,
				},
			}
		}
	}

	config.IsManualEmbeddingUpdated = isManualEmbeddingUpdated
	u.normalizeModelModeSetting(&config)

	// 持久化设置
	updatedValue, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated model setting: %w", err)
	}
	if err := u.systemSettingRepo.UpdateSystemSetting(ctx, string(consts.SystemSettingModelMode), string(updatedValue)); err != nil {
		return nil, fmt.Errorf("failed to update model setting: %w", err)
	}
	return &config, nil
}

func (u *ModelUsecase) GetModelModeSetting(ctx context.Context) (domain.ModelModeSetting, error) {
	setting, err := u.systemSettingRepo.GetSystemSetting(ctx, consts.SystemSettingModelMode)
	if err != nil {
		return domain.ModelModeSetting{}, fmt.Errorf("failed to get model mode setting: %w", err)
	}
	var config domain.ModelModeSetting
	if err := json.Unmarshal(setting.Value, &config); err != nil {
		return domain.ModelModeSetting{}, fmt.Errorf("failed to parse model mode setting: %w", err)
	}
	u.normalizeModelModeSetting(&config)
	// 无效设置检查
	if config.Mode == "" {
		return domain.ModelModeSetting{}, fmt.Errorf("model mode setting is invalid")
	}
	if err := u.ensureEmbeddingIndexConfig(ctx, &config); err != nil {
		return domain.ModelModeSetting{}, fmt.Errorf("resolve embedding index baseline: %w", err)
	}
	return config, nil
}

// updateRAGModelsByMode 根据模式更新 RAG 模型
func (u *ModelUsecase) updateRAGModelsByMode(ctx context.Context, mode string, modelModeSetting, oldModelModeSetting domain.ModelModeSetting) error {
	if modelModeSetting.EmbeddingIndexConfig == nil {
		if err := u.ensureEmbeddingIndexConfig(ctx, &oldModelModeSetting); err != nil {
			return err
		}
		modelModeSetting.EmbeddingIndexConfig = oldModelModeSetting.EmbeddingIndexConfig
	}
	var nextEmbeddingIndex *domain.EmbeddingIndexConfig

	ragModelTypes := []domain.ModelType{
		domain.ModelTypeEmbedding,
		domain.ModelTypeRerank,
		domain.ModelTypeAnalysis,
		domain.ModelTypeAnalysisVL,
		domain.ModelTypeChat,
	}

	for _, modelType := range ragModelTypes {
		var model *domain.Model

		if mode == string(consts.ModelSettingModeManual) {
			// 获取该类型的活跃模型
			m, err := u.modelRepo.GetModelByType(ctx, modelType)
			if err != nil {
				u.logger.Warn("failed to get model by type", log.String("type", string(modelType)), log.Any("error", err))
				continue
			}
			if m == nil || !m.IsActive {
				u.logger.Warn("no active model found for type", log.String("type", string(modelType)))
				continue
			}
			model = m
		} else {
			activeProvider, err := u.getAutoModeProvider(modelModeSetting)
			if err != nil {
				return err
			}
			modelName := map[domain.ModelType]string{
				domain.ModelTypeChat:       activeProvider.Models.Chat,
				domain.ModelTypeEmbedding:  activeProvider.Models.Embedding,
				domain.ModelTypeRerank:     activeProvider.Models.Rerank,
				domain.ModelTypeAnalysis:   activeProvider.Models.Analysis,
				domain.ModelTypeAnalysisVL: activeProvider.Models.AnalysisVL,
			}[modelType]
			if strings.TrimSpace(modelName) == "" {
				return fmt.Errorf("auto mode %s model is required", modelType)
			}
			model = &domain.Model{
				Model:    modelName,
				Type:     modelType,
				IsActive: true,
				BaseURL:  getAutoModeModelBaseURL(activeProvider, modelType),
				APIKey:   activeProvider.APIKey,
				Provider: activeProvider.Provider,
			}
		}

		// 更新RAG存储中的模型
		if model != nil {
			// 任一模型同步失败时保留原索引基准，供下次保存重试。
			if err := u.ragStore.UpsertModel(ctx, model); err != nil {
				u.logger.Error("failed to update model in RAG store", log.String("model_id", model.ID), log.String("type", string(modelType)), log.Any("error", err))
				return fmt.Errorf("failed to update model in RAG store: %s", model.Type)
			}
			u.logger.Info("successfully updated RAG model", log.String("model name: ", string(model.Model)))
			if modelType == domain.ModelTypeEmbedding {
				value := embeddingIndexConfig(model)
				nextEmbeddingIndex = &value
			}
		}
	}

	// 触发记录更新
	if nextEmbeddingIndex == nil {
		return fmt.Errorf("no active embedding model configured")
	}
	if modelModeSetting.EmbeddingIndexConfig == nil || *modelModeSetting.EmbeddingIndexConfig != *nextEmbeddingIndex {
		u.logger.Info("embedding model updated, triggering upsert records")
		if err := u.TriggerUpsertRecords(ctx); err != nil {
			return err
		}
	} else {
		u.logger.Info("embedding index configuration unchanged; skipped full reindex")
	}
	// Advance only after model synchronization and required enqueueing succeed.
	// Failed saves retain their baseline so the next attempt cannot skip work.
	modelModeSetting.EmbeddingIndexConfig = nextEmbeddingIndex
	modelModeSetting.IsManualEmbeddingUpdated = false
	return u.saveModelModeSetting(ctx, modelModeSetting)
}

func getAutoModeModelBaseURL(activeProvider *domain.AutoModeProviderSetting, modelType domain.ModelType) string {
	if activeProvider == nil {
		return ""
	}
	baseURL := getAutoModeBaseURLByType(activeProvider.BaseURLs, modelType)
	if strings.TrimSpace(baseURL) != "" {
		return strings.TrimSpace(baseURL)
	}
	return activeProvider.BaseURL
}

func getAutoModeReqModelBaseURL(provider *domain.AutoModeProviderReq, modelType domain.ModelType) string {
	if provider == nil {
		return ""
	}
	baseURLs := provider.BaseURLs
	if strings.TrimSpace(baseURLs.Rerank) == "" && strings.TrimSpace(provider.RerankBaseURL) != "" {
		baseURLs.Rerank = provider.RerankBaseURL
	}
	baseURL := getAutoModeBaseURLByType(baseURLs, modelType)
	if strings.TrimSpace(baseURL) != "" {
		return strings.TrimSpace(baseURL)
	}
	return provider.BaseURL
}

func getAutoModeBaseURLByType(baseURLs domain.AutoModeModelBaseURLs, modelType domain.ModelType) string {
	switch modelType {
	case domain.ModelTypeChat:
		return baseURLs.Chat
	case domain.ModelTypeEmbedding:
		return baseURLs.Embedding
	case domain.ModelTypeRerank:
		return baseURLs.Rerank
	case domain.ModelTypeAnalysis:
		return baseURLs.Analysis
	case domain.ModelTypeAnalysisVL:
		return baseURLs.AnalysisVL
	default:
		return ""
	}
}

func normalizeAutoModeModelBaseURLs(baseURLs domain.AutoModeModelBaseURLs) domain.AutoModeModelBaseURLs {
	return domain.AutoModeModelBaseURLs{
		Chat:       strings.TrimSpace(baseURLs.Chat),
		Embedding:  strings.TrimSpace(baseURLs.Embedding),
		Rerank:     strings.TrimSpace(baseURLs.Rerank),
		Analysis:   strings.TrimSpace(baseURLs.Analysis),
		AnalysisVL: strings.TrimSpace(baseURLs.AnalysisVL),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
