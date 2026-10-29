package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
)

type modelIndexNodes interface {
	TraverseNodesByCursor(context.Context, func(*domain.NodeRelease) error) error
}

type modelIndexKnowledgeBases interface {
	GetKnowledgeBaseList(context.Context) ([]*domain.KnowledgeBaseListItem, error)
	UpdateDatasetID(context.Context, string, string) error
}

type modelSettingsStore interface {
	GetSystemSetting(context.Context, consts.SystemSettingKey) (*domain.SystemSetting, error)
	UpdateSystemSetting(context.Context, string, string) error
}

func embeddingIndexConfig(model *domain.Model) domain.EmbeddingIndexConfig {
	if model == nil || strings.TrimSpace(model.Model) == "" {
		return domain.EmbeddingIndexConfig{}
	}
	maxTokens := model.Parameters.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192 // Same default as CTRAG.UpsertModel.
	}
	return domain.EmbeddingIndexConfig{
		Provider:      consts.NormalizeAutoModeProvider(strings.TrimSpace(string(model.Provider))),
		Model:         strings.TrimSpace(model.Model),
		ContextWindow: model.Parameters.ContextWindow,
		MaxTokens:     maxTokens,
	}
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
	value, err := json.Marshal(setting)
	if err != nil {
		return err
	}
	return u.systemSettingRepo.UpdateSystemSetting(ctx, string(consts.SystemSettingModelMode), string(value))
}
