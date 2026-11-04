package rag

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/log"
)

func TestCreateKnowledgeBaseWithoutEmbeddingModel(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "not configured", true: "upstream unavailable"}[unavailable], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/models" {
					t.Errorf("unexpected dataset mutation: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if unavailable {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"success":false,"message":"unavailable"}`))
					return
				}
				_, _ = w.Write([]byte(`{"success":true,"data":{"models":[]}}`))
			}))
			defer server.Close()
			cfg := &config.Config{RAG: config.RAGConfig{CTRAG: config.CTRAGConfig{BaseURL: server.URL}}}
			store, err := NewCTRAG(cfg, log.NewLogger(cfg))
			if err != nil {
				t.Fatal(err)
			}
			id, err := store.CreateKnowledgeBase(context.Background())
			if id != "" || err == nil || errors.Is(err, ErrEmbeddingModelNotConfigured) == unavailable {
				t.Fatalf("must distinguish unconfigured models from service failures: id=%q err=%v", id, err)
			}
		})
	}
}

func TestUninitializedDatasetDoesNotSendLearningOrSearchRequests(t *testing.T) {
	store := &CTRAG{}
	if _, err := store.UpsertRecords(context.Background(), &UpsertRecordsRequest{}); !errors.Is(err, ErrEmbeddingModelNotConfigured) {
		t.Fatal(err)
	}
	if _, _, err := store.QueryRecords(context.Background(), &QueryRecordsRequest{}); !errors.Is(err, ErrEmbeddingModelNotConfigured) {
		t.Fatal(err)
	}
}
