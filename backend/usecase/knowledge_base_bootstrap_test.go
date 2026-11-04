package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/rag"
	"github.com/google/uuid"
)

type bootstrapRAG struct {
	rag.RAGService
	err error
}

func (r bootstrapRAG) CreateKnowledgeBase(context.Context) (string, error) { return "", r.err }

func (r bootstrapRAG) ListKnowledgeBases(context.Context) ([]rag.Dataset, error) {
	return nil, nil
}

func TestKnowledgeBaseBootstrapWithoutModelPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	admin := domain.User{ID: uuid.NewString(), Account: "bootstrap_" + uuid.NewString(), Role: consts.UserRoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: admin.ID})
	cfg := &config.Config{CaddyAPI: t.TempDir() + "/absent.sock"}
	store := bootstrapRAG{err: rag.ErrEmbeddingModelNotConfigured}
	repo := pg.NewKnowledgeBaseRepository(db, cfg, logger, store)
	u := &KnowledgeBaseUsecase{repo: repo, navRepo: pg.NewNavRepository(db, logger), rag: store}
	var count int64
	if err := db.Model(&domain.KnowledgeBase{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	id, err := u.CreateKnowledgeBase(ctx, &domain.CreateKnowledgeBaseReq{Name: "bootstrap fixture", MaxKB: int(count) + 1})
	if err != nil {
		t.Fatal(err)
	}
	list, err := u.GetKnowledgeBaseListByUserId(ctx)
	if err != nil || len(list) != int(count)+1 {
		t.Fatalf("unconfigured model blocked management: %v", err)
	}
	kb, err := repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil || kb.DatasetID != "" {
		t.Fatalf("expected no fabricated dataset: %v", err)
	}
	// Old document backups may reference a dataset absent from a fresh RAGLite.
	// Browsing must remain available without silently replacing that binding.
	if err := repo.UpdateDatasetID(ctx, id, "restored-dataset"); err != nil {
		t.Fatal(err)
	}
	kb, err = repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil || kb.DatasetID != "restored-dataset" {
		t.Fatalf("restored documents must remain readable before configuring models: %v", err)
	}
	var navs int64
	if err := db.Model(&domain.Nav{}).Where("kb_id = ?", id).Count(&navs).Error; err != nil || navs != 1 {
		t.Fatalf("default navigation missing: %v", err)
	}
	u.rag = bootstrapRAG{err: fmt.Errorf("RAG connection failure")}
	if _, err := u.CreateKnowledgeBase(ctx, &domain.CreateKnowledgeBaseReq{Name: "must not be created", MaxKB: int(count) + 2}); err == nil {
		t.Fatal("upstream failures must not be silently ignored")
	}
	var after int64
	if err := db.Model(&domain.KnowledgeBase{}).Count(&after).Error; err != nil || after != count+1 {
		t.Fatalf("unexpected partial initialization: %v", err)
	}
}
