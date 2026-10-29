package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/mq"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/google/uuid"
)

func TestManualModelEditsUseIndexBaselinePostgres(t *testing.T) {
	db, l := chatContextDB(t) // Opt-in local DB; every write is rolled back.
	ctx := context.Background()
	if err := db.Where("1 = 1").Delete(&domain.Model{}).Error; err != nil {
		t.Fatal(err)
	}
	modelID := uuid.NewString()
	for _, kind := range []domain.ModelType{domain.ModelTypeEmbedding, domain.ModelTypeChat, domain.ModelTypeRerank, domain.ModelTypeAnalysis} {
		id := uuid.NewString()
		name := "fixture-" + string(kind)
		if kind == domain.ModelTypeEmbedding {
			id = modelID
			name = "embed-v1"
		}
		if err := db.Create(&domain.Model{ID: id, Type: kind, Provider: "CompatiblePlatform", Model: name, BaseURL: "http://fixture/v1", APIKey: "fixture-key", IsActive: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	r, p, k := &indexRAG{}, &indexProducer{}, &indexKBs{}
	initial, _ := json.Marshal(domain.ModelModeSetting{Mode: consts.ModelSettingModeManual})
	// Avoid binding the production runtime-log sink to a single rollback
	// transaction: its asynchronous writes cannot share that connection.
	settings := &indexSettings{value: initial}
	u := &ModelUsecase{modelRepo: pg.NewModelRepository(db, l), systemSettingRepo: settings, ragStore: r, ragRepo: mq.NewRAGRepository(p), kbRepo: k, nodeRepo: indexNodes{}, logger: l}
	request := &domain.UpdateModelReq{ID: modelID, BaseModelInfo: domain.BaseModelInfo{Type: domain.ModelTypeEmbedding, Provider: "CompatiblePlatform", Model: "embed-v1", APIKey: "rotated-key", BaseURL: "http://new-endpoint/v1"}}
	if err := u.Update(ctx, request); err != nil {
		t.Fatal(err)
	}
	stored, err := u.GetModelModeSetting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IsManualEmbeddingUpdated || stored.EmbeddingIndexConfig.Model != "embed-v1" {
		t.Fatal("credential rotation marked manual embedding as changed")
	}
	if len(r.models) != 1 || !r.models[0].IsActive || r.models[0].APIKey != "rotated-key" {
		t.Fatal("manual credential update did not reach active RAG model")
	}
	if err := u.SwitchMode(ctx, &domain.SwitchModeReq{Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	if r.created != 0 || len(p.messages) != 0 {
		t.Fatal("manual resave unnecessarily rebuilt index")
	}
	request.Model = "embed-v2"
	if err := u.Update(ctx, request); err != nil {
		t.Fatal(err)
	}
	stored, err = u.GetModelModeSetting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsManualEmbeddingUpdated || stored.EmbeddingIndexConfig.Model != "embed-v1" {
		t.Fatal("manual edit lost original index baseline")
	}
	if err := u.SwitchMode(ctx, &domain.SwitchModeReq{Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	if r.created != 1 || len(p.messages) != 1 {
		t.Fatal("manual model change did not enqueue rebuild")
	}
	if err := u.SwitchMode(ctx, &domain.SwitchModeReq{Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	if r.created != 1 {
		t.Fatal("repeated manual save rebuilt index again")
	}
	// The equivalent automatic configuration must reuse the same vector space.
	next := automaticIndexSetting()
	next.AutoProvider.Models.Embedding = "embed-v2"
	if err := saveIndexSetting(u, next); err != nil {
		t.Fatal(err)
	}
	if r.created != 1 {
		t.Fatal("mode switch with same effective embedding model rebuilt index")
	}
}
