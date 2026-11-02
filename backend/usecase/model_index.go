package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chaitin/panda-wiki/store/rag"
	"github.com/google/uuid"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
)

type modelIndexNodes interface {
	TraverseNodesByCursor(context.Context, func(*domain.NodeRelease) error) error
	GetPendingIndexBuild(context.Context, string) (*domain.KBIndexBuild, error)
	StartIndexBuild(context.Context, *domain.KBIndexBuild) error
	GetLatestIndexBuild(context.Context, string) (*domain.KBIndexBuild, error)
}

func (u *ModelUsecase) syncRAGModel(ctx context.Context, model *domain.Model) error {
	if model.Type == domain.ModelTypeEmbedding {
		service, ok := u.ragStore.(rag.IndexDatasetService)
		if !ok {
			return fmt.Errorf("RAG 存储不支持安全的向量模型隔离")
		}
		return service.SyncEmbeddingConnection(ctx, model)
	}
	return u.ragStore.UpsertModel(ctx, model)
}

func (u *ModelUsecase) stageEmbeddingIndexBuild(ctx context.Context, model *domain.Model) error {
	if model == nil {
		return fmt.Errorf("未配置向量模型")
	}
	kbs, err := u.kbRepo.GetKnowledgeBaseList(ctx)
	if err != nil {
		return err
	}
	for _, kb := range kbs {
		if err := u.stageKBIndexBuild(ctx, kb, model); err != nil {
			return err
		}
	}
	return nil
}

func (u *ModelUsecase) stageKBIndexBuild(ctx context.Context, kb *domain.KnowledgeBaseListItem, model *domain.Model) error {
	keyBytes, _ := json.Marshal(embeddingIndexConfig(model))
	key := string(keyBytes)
	pending, err := u.nodeRepo.GetPendingIndexBuild(ctx, kb.ID)
	if err == nil {
		if pending.ConfigKey == key {
			return nil
		}
		return fmt.Errorf("已有其他向量模型重建正在进行，旧索引继续生效")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	latest, latestErr := u.nodeRepo.GetLatestIndexBuild(ctx, kb.ID)
	if latestErr == nil && latest.Status == domain.IndexActive && latest.ConfigKey == key && latest.DatasetID == kb.DatasetID {
		return nil
	}
	if latestErr != nil && !errors.Is(latestErr, gorm.ErrRecordNotFound) {
		return latestErr
	}
	service, ok := u.ragStore.(rag.IndexDatasetService)
	if !ok {
		return fmt.Errorf("RAG 存储不支持安全的索引隔离")
	}
	dataset, err := service.CreateIndexKnowledgeBase(ctx, model)
	if err != nil {
		return err
	}
	build := &domain.KBIndexBuild{ID: uuid.NewString(), KBID: kb.ID, SourceDatasetID: kb.DatasetID, DatasetID: dataset, ConfigKey: key}
	if err := u.nodeRepo.StartIndexBuild(ctx, build); err != nil {
		// Only the new, unused candidate is eligible for this cleanup.
		_ = u.ragStore.DeleteKnowledgeBase(ctx, dataset)
		return err
	}
	u.logger.Info("staged model index rebuild; serving dataset unchanged")
	return nil
}

type modelIndexKnowledgeBases interface {
	GetKnowledgeBaseList(context.Context) ([]*domain.KnowledgeBaseListItem, error)
	UpdateDatasetID(context.Context, string, string) error
}

type modelSettingsStore interface {
	GetSystemSetting(context.Context, consts.SystemSettingKey) (*domain.SystemSetting, error)
	SaveModelModeSetting(context.Context, domain.ModelModeSetting) error
}

func embeddingIndexConfig(model *domain.Model) domain.EmbeddingIndexConfig {
	return model.EmbeddingIndexIdentity()
}

func (u *ModelUsecase) modelForMode(ctx context.Context, setting domain.ModelModeSetting, modelType domain.ModelType) (*domain.Model, error) {
	if setting.Mode == consts.ModelSettingModeManual {
		return u.modelRepo.GetModelByType(ctx, modelType)
	}
	provider, err := u.getAutoModeProvider(setting)
	if err != nil {
		return nil, err
	}
	name := map[domain.ModelType]string{
		domain.ModelTypeEmbedding:  provider.Models.Embedding,
		domain.ModelTypeChat:       provider.Models.Chat,
		domain.ModelTypeRerank:     provider.Models.Rerank,
		domain.ModelTypeAnalysis:   provider.Models.Analysis,
		domain.ModelTypeAnalysisVL: provider.Models.AnalysisVL,
	}[modelType]
	return &domain.Model{Model: name, Type: modelType, Provider: provider.Provider,
		BaseURL: getAutoModeModelBaseURL(provider, modelType), APIKey: provider.APIKey, IsActive: true}, nil
}

// Bootstrap old installations without rebuilding simply because this field was
// absent. An old pending manual edit has an unknown baseline and must rebuild
// once; otherwise resolve the effective model before any edit is persisted.
func (u *ModelUsecase) ensureEmbeddingIndexConfig(ctx context.Context, setting *domain.ModelModeSetting) error {
	if setting.EmbeddingIndexConfig != nil {
		return nil
	}
	baseline := domain.EmbeddingIndexConfig{}
	if setting.Mode != consts.ModelSettingModeManual || !setting.IsManualEmbeddingUpdated {
		if setting.Mode != consts.ModelSettingModeManual && setting.AutoProvider == nil {
			setting.EmbeddingIndexConfig = &baseline
			return nil
		}
		model, err := u.modelForMode(ctx, *setting, domain.ModelTypeEmbedding)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		baseline = embeddingIndexConfig(model)
	}
	setting.EmbeddingIndexConfig = &baseline
	return nil
}

func (u *ModelUsecase) saveModelModeSetting(ctx context.Context, setting domain.ModelModeSetting) error {
	return u.systemSettingRepo.SaveModelModeSetting(ctx, setting)
}
