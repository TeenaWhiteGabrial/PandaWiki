package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/store/rag"
)

func TestEmbeddingIndexIdentity(t *testing.T) {
	base := &domain.Model{Model: "embed-v1", Provider: "CompatiblePlatform", APIKey: "old", BaseURL: "http://old/v1"}
	want := embeddingIndexConfig(base)
	for _, edit := range []func(*domain.Model){
		func(m *domain.Model) { m.APIKey = "rotated"; m.APIHeader = "auth"; m.APIVersion = "new" },
		func(m *domain.Model) { m.BaseURL = "http://new/v1" },
		func(m *domain.Model) { m.Provider = "Other"; m.Model = " embed-v1 "; m.Parameters.MaxTokens = 8192 },
		func(m *domain.Model) {
			v := float32(.5)
			m.Parameters.Temperature = &v
			m.Parameters.SupportImages = true
		},
	} {
		next := *base
		edit(&next)
		if embeddingIndexConfig(&next) != want {
			t.Fatal("connection/chat-only settings changed embedding identity")
		}
	}
	for _, edit := range []func(*domain.Model){
		func(m *domain.Model) { m.Model = "embed-v2" },
		func(m *domain.Model) { m.Provider = "Xinference" },
		func(m *domain.Model) { m.Parameters.MaxTokens = 4096 },
		func(m *domain.Model) { m.Parameters.ContextWindow = 4096 },
	} {
		next := *base
		edit(&next)
		if embeddingIndexConfig(&next) == want {
			t.Fatal("index-affecting configuration change was ignored")
		}
	}
	encoded, _ := json.Marshal(want)
	if strings.Contains(string(encoded), "old") || strings.Contains(string(encoded), "api_key") {
		t.Fatal("index signature contains connection secrets")
	}
}

type indexSettings struct{ value []byte }

func (s *indexSettings) GetSystemSetting(context.Context, consts.SystemSettingKey) (*domain.SystemSetting, error) {
	return &domain.SystemSetting{Value: append([]byte(nil), s.value...)}, nil
}
func (s *indexSettings) UpdateSystemSetting(_ context.Context, _ string, value string) error {
	s.value = []byte(value)
	return nil
}

type indexKBs struct{ updates int }

func (k *indexKBs) GetKnowledgeBaseList(context.Context) ([]*domain.KnowledgeBaseListItem, error) {
	return []*domain.KnowledgeBaseListItem{{ID: "kb", DatasetID: "old-dataset"}}, nil
}
func (k *indexKBs) UpdateDatasetID(context.Context, string, string) error { k.updates++; return nil }

type indexNodes struct{}

func (indexNodes) TraverseNodesByCursor(_ context.Context, f func(*domain.NodeRelease) error) error {
	return f(&domain.NodeRelease{ID: "release", NodeID: "node", KBID: "kb"})
}

type indexProducer struct {
	messages []domain.NodeReleaseVectorRequest
}

func (p *indexProducer) Produce(_ context.Context, topic, key string, value []byte) error {
	var msg domain.NodeReleaseVectorRequest
	if err := json.Unmarshal(value, &msg); err != nil {
		return err
	}
	p.messages = append(p.messages, msg)
	return nil
}
func (*indexProducer) Close() error { return nil }

type indexRAG struct {
	rag.RAGService
	models               []*domain.Model
	created, deleted     int
	failSync, failCreate bool
}

func (r *indexRAG) UpsertModel(_ context.Context, m *domain.Model) error {
	r.models = append(r.models, m)
	if r.failSync && m.Type == domain.ModelTypeRerank {
		return errors.New("simulated model sync failure")
	}
	return nil
}
func (r *indexRAG) CreateKnowledgeBase(context.Context) (string, error) {
	r.created++
	if r.failCreate {
		return "", errors.New("simulated dataset creation failure")
	}
	return "new-dataset", nil
}
func (r *indexRAG) DeleteKnowledgeBase(context.Context, string) error { r.deleted++; return nil }

func automaticIndexSetting() domain.ModelModeSetting {
	return domain.ModelModeSetting{Mode: consts.ModelSettingModeAuto, AutoProvider: &domain.AutoModeProviderSetting{
		Provider: "CompatiblePlatform", BaseURL: "http://fixture/v1", APIKey: "fixture-secret",
		Models: domain.AutoModeModelSelection{Embedding: "embed-v1", Chat: "chat-v1", Rerank: "rank-v1", Analysis: "analysis-v1", AnalysisVL: "vision-v1"},
	}}
}
func indexUsecase(t *testing.T, initial domain.ModelModeSetting) (*ModelUsecase, *indexRAG, *indexProducer, *indexKBs) {
	t.Helper()
	value, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	r, p, k := &indexRAG{}, &indexProducer{}, &indexKBs{}
	u := &ModelUsecase{ragStore: r, ragRepo: mq.NewRAGRepository(p), kbRepo: k, nodeRepo: indexNodes{}, systemSettingRepo: &indexSettings{value: value}, logger: log.NewLogger(&config.Config{})}
	return u, r, p, k
}

// Exercise the actual settings persistence -> RAG sync -> rebuild decision.
// Validation's provider HTTP calls are outside this deterministic test.
func saveIndexSetting(u *ModelUsecase, next domain.ModelModeSetting) error {
	ctx := context.Background()
	old, err := u.GetModelModeSetting(ctx)
	if err != nil {
		return err
	}
	a := next.AutoProvider
	updated, err := u.updateModeSettingConfig(ctx, &domain.SwitchModeReq{Mode: "auto", AutoProvider: &domain.AutoModeProviderReq{
		Provider: a.Provider, BaseURL: a.BaseURL, BaseURLs: a.BaseURLs, APIKey: a.APIKey,
		Models: domain.AutoModeModels{Chat: a.Models.Chat, Embedding: a.Models.Embedding, Rerank: a.Models.Rerank, Analysis: a.Models.Analysis, AnalysisVL: a.Models.AnalysisVL},
	}}, true)
	if err != nil {
		return err
	}
	return u.updateRAGModelsByMode(ctx, "auto", *updated, old)
}

func TestAutoModelSaveOnlyRebuildsChangedVectorSpace(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(*domain.AutoModeProviderSetting)
		rebuild bool
	}{
		{"unchanged", func(*domain.AutoModeProviderSetting) {}, false},
		{"rotate credentials", func(a *domain.AutoModeProviderSetting) { a.APIKey = "new-key" }, false},
		{"move endpoint", func(a *domain.AutoModeProviderSetting) {
			a.BaseURL = "http://new/v1"
			a.BaseURLs.Embedding = "http://embed/v1"
		}, false},
		{"change chat", func(a *domain.AutoModeProviderSetting) { a.Models.Chat = "chat-v2" }, false},
		{"change rerank", func(a *domain.AutoModeProviderSetting) { a.Models.Rerank = "rank-v2" }, false},
		{"change analysis", func(a *domain.AutoModeProviderSetting) {
			a.Models.Analysis = "analysis-v2"
			a.Models.AnalysisVL = "vision-v2"
		}, false},
		{"provider alias", func(a *domain.AutoModeProviderSetting) { a.Provider = "Other" }, false},
		{"change embedding", func(a *domain.AutoModeProviderSetting) { a.Models.Embedding = "embed-v2" }, true},
		{"change embedding provider", func(a *domain.AutoModeProviderSetting) { a.Provider = "Xinference" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := automaticIndexSetting()
			old.IsManualEmbeddingUpdated = true // legacy automatic saves set this flag indiscriminately.
			u, r, p, k := indexUsecase(t, old)
			next := automaticIndexSetting()
			tc.edit(next.AutoProvider)
			if err := saveIndexSetting(u, next); err != nil {
				t.Fatal(err)
			}
			want := 0
			if tc.rebuild {
				want = 1
			}
			if r.created != want || r.deleted != want || k.updates != want || len(p.messages) != want {
				t.Fatalf("unexpected rebuild: created=%d deleted=%d updated=%d messages=%d", r.created, r.deleted, k.updates, len(p.messages))
			}
			if len(r.models) != 5 {
				t.Fatal("connection changes must still synchronize the RAG models")
			}
			if want == 1 && !p.messages[0].ReuseParsedContent {
				t.Fatal("model rebuild must reuse parsed content")
			}
			// Saving again, including another credential rotation, must not repeat a completed enqueue.
			next.AutoProvider.APIKey = "second-key"
			if err := saveIndexSetting(u, next); err != nil {
				t.Fatal(err)
			}
			if r.created != want || len(p.messages) != want {
				t.Fatal("repeated save rebuilt unchanged vectors")
			}
		})
	}
}

func TestFailedModelSaveKeepsOldIndexBaseline(t *testing.T) {
	for _, failure := range []string{"model sync", "dataset creation"} {
		t.Run(failure, func(t *testing.T) {
			u, r, _, _ := indexUsecase(t, automaticIndexSetting())
			next := automaticIndexSetting()
			next.AutoProvider.Models.Embedding = "embed-v2"
			r.failSync = failure == "model sync"
			r.failCreate = failure == "dataset creation"
			if err := saveIndexSetting(u, next); err == nil {
				t.Fatal("expected failed save")
			}
			stored, err := u.GetModelModeSetting(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if stored.EmbeddingIndexConfig.Model != "embed-v1" {
				t.Fatal("failed save advanced index baseline")
			}
			r.failSync = false
			r.failCreate = false
			before := r.created
			if err := saveIndexSetting(u, next); err != nil {
				t.Fatal(err)
			}
			if r.created != before+1 {
				t.Fatal("retry skipped required index rebuild")
			}
		})
	}
}

func TestLegacyPendingManualIndexHasUnknownBaseline(t *testing.T) {
	u := &ModelUsecase{}
	setting := domain.ModelModeSetting{Mode: consts.ModelSettingModeManual, IsManualEmbeddingUpdated: true}
	if err := u.ensureEmbeddingIndexConfig(context.Background(), &setting); err != nil {
		t.Fatal(err)
	}
	if setting.EmbeddingIndexConfig == nil || setting.EmbeddingIndexConfig.Model != "" {
		t.Fatal("legacy pending edits must not pretend the new model is already indexed")
	}
}
