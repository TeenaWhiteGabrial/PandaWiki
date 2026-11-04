package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/rag"
	"github.com/google/uuid"
)

type learningDatasetRAG struct {
	rag.RAGService
	datasets       []rag.Dataset
	listErr        error
	createErr      error
	created        int
	requestedModel string
}

func (r *learningDatasetRAG) ListKnowledgeBases(context.Context) ([]rag.Dataset, error) {
	return r.datasets, r.listErr
}
func (r *learningDatasetRAG) CreateIndexKnowledgeBase(_ context.Context, model *domain.Model) (string, error) {
	r.created++
	r.requestedModel = model.Model
	if r.createErr != nil {
		return "", r.createErr
	}
	id := uuid.NewString()
	r.datasets = append(r.datasets, rag.Dataset{ID: id})
	return id, nil
}
func (r *learningDatasetRAG) SyncEmbeddingConnection(context.Context, *domain.Model) error {
	return nil
}

func TestLearningDatasetRecoveryPostgres(t *testing.T) {
	for _, scenario := range []string{"missing", "existing", "list failed", "create failed", "pending build"} {
		t.Run(scenario, func(t *testing.T) {
			db := securityTestDB(t)
			kb := domain.KnowledgeBase{ID: uuid.NewString(), Name: "dataset recovery fixture", DatasetID: uuid.NewString()}
			if err := db.Create(&kb).Error; err != nil {
				t.Fatal(err)
			}
			store := &learningDatasetRAG{}
			switch scenario {
			case "existing":
				store.datasets = []rag.Dataset{{ID: kb.DatasetID}}
			case "list failed":
				store.listErr = errors.New("offline")
			case "create failed":
				store.createErr = errors.New("invalid embedding credentials")
			case "pending build":
				build := domain.KBIndexBuild{ID: uuid.NewString(), KBID: kb.ID, SourceDatasetID: kb.DatasetID, DatasetID: uuid.NewString(), ConfigKey: "fixture", Status: domain.IndexBuilding}
				if err := db.Create(&build).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := &KnowledgeBaseRepository{db: db, rag: store, logger: log.NewLogger(&config.Config{})}
			resolveCalls := 0
			resolve := func(context.Context) (*domain.Model, error) {
				resolveCalls++
				return &domain.Model{Model: "configured-embedding"}, nil
			}
			id, err := repo.EnsureLearningDataset(t.Context(), kb.ID, resolve)
			failed := scenario != "missing" && scenario != "existing"
			if (err != nil) != failed {
				t.Fatalf("unexpected recovery result: %q %v", id, err)
			}
			var after domain.KnowledgeBase
			if e := db.First(&after, "id=?", kb.ID).Error; e != nil {
				t.Fatal(e)
			}
			if failed || scenario == "existing" {
				if after.DatasetID != kb.DatasetID {
					t.Fatal("failed repair or existing dataset changed serving binding")
				}
			} else {
				if id == "" || id == kb.DatasetID || after.DatasetID != id || store.requestedModel != "configured-embedding" {
					t.Fatal("missing dataset was not explicitly bound to configured model")
				}
				again, err := repo.EnsureLearningDataset(t.Context(), kb.ID, resolve)
				if err != nil || again != id || store.created != 1 || resolveCalls != 1 {
					t.Fatal("repeated learning recreated the dataset")
				}
			}
			if scenario == "existing" && (store.created != 0 || resolveCalls != 0) {
				t.Fatal("valid index must not be rebuilt or rebound")
			}
			var tasks int64
			if err := db.Model(&domain.NodeLearningTask{}).Where("kb_id=?", kb.ID).Count(&tasks).Error; err != nil || tasks != 0 {
				t.Fatal("dataset repair must not submit whole-library tasks")
			}
		})
	}
}
