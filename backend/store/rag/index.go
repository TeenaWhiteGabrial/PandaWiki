package rag

import (
	"context"
	"fmt"
	"reflect"

	"github.com/chaitin/panda-wiki/domain"
	raglite "github.com/chaitin/raglite-go-sdk"
	"github.com/google/uuid"
)

func indexModelConfig(model *domain.Model) raglite.AIModelConfig {
	max := model.Parameters.MaxTokens
	if max == 0 {
		max = 8192
	}
	return raglite.AIModelConfig{APIBase: model.BaseURL, APIKey: model.APIKey, APIHeader: model.APIHeader, APIVersion: model.APIVersion, MaxTokens: &max, ExtraParameters: model.Parameters.Map()}
}

// Create, never upsert: mutating a model already referenced by the serving
// dataset would make its stored vectors incompatible with query embeddings.
func (s *CTRAG) CreateIndexKnowledgeBase(ctx context.Context, model *domain.Model) (string, error) {
	created, err := s.client.Models.Create(ctx, &raglite.CreateModelRequest{Name: "pandawiki-index-" + uuid.NewString(), ModelType: string(domain.ModelTypeEmbedding), Provider: string(model.Provider), ModelName: model.Model, Config: indexModelConfig(model), IsDefault: false})
	if err != nil {
		return "", fmt.Errorf("create isolated embedding model: %w", err)
	}
	dataset, err := s.client.Datasets.Create(ctx, &raglite.CreateDatasetRequest{Name: uuid.NewString(), DenseModelID: &created.ID})
	if err != nil {
		// This new model has never served a dataset; no existing model is touched.
		_ = s.client.Models.Delete(ctx, created.ID)
		return "", fmt.Errorf("create separately bound index dataset: %w", err)
	}
	return dataset.ID, nil
}

// Connection edits apply to matching vector spaces only. In particular a
// retry while a new model build is pending must not rename the old live model.
func (s *CTRAG) SyncEmbeddingConnection(ctx context.Context, model *domain.Model) error {
	models, err := s.client.Models.List(ctx, &raglite.ListModelsRequest{ModelType: string(domain.ModelTypeEmbedding)})
	if err != nil {
		return err
	}
	desired := indexModelConfig(model)
	for _, existing := range models.Models {
		if existing.ModelName != model.Model || existing.Provider != string(model.Provider) {
			continue
		}
		max := 8192
		if existing.Config.MaxTokens != nil {
			max = *existing.Config.MaxTokens
		}
		if max != *desired.MaxTokens {
			continue
		}
		oldWindow := existing.Config.ExtraParameters["context_window"]
		newWindow := desired.ExtraParameters["context_window"]
		if fmt.Sprint(oldWindow) != fmt.Sprint(newWindow) {
			continue
		}
		config := existing.Config
		config.APIBase = model.BaseURL
		config.APIKey = model.APIKey
		config.APIHeader = model.APIHeader
		config.APIVersion = model.APIVersion
		if reflect.DeepEqual(config, existing.Config) {
			continue
		}
		if _, err := s.client.Models.Update(ctx, existing.ID, &raglite.UpdateModelRequest{Config: &config}); err != nil {
			return err
		}
	}
	return nil
}
