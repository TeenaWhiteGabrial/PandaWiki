package rag

import (
	"context"
	"encoding/json"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetrievalPermissionFilterIsReversible(t *testing.T) {
	for _, mode := range []string{"false", "true", "false"} {
		t.Setenv("DISABLE_PERMISSION_LEVELS", mode)
		var body map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"query":"question","results":[]}}`))
		}))
		cfg := &config.Config{RAG: config.RAGConfig{CTRAG: config.CTRAGConfig{BaseURL: server.URL}}}
		store, err := NewCTRAG(cfg, log.NewLogger(cfg))
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		_, _, err = store.QueryRecords(context.Background(), &QueryRecordsRequest{DatasetID: "dataset-a", Query: "question", GroupIDs: []int{7}})
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if body["dataset_id"] != "dataset-a" {
			t.Fatal("knowledge base scope lost")
		}
		metadata, _ := body["metadata"].(map[string]any)
		_, hasGroups := metadata["group_ids"]
		if !hasGroups {
			t.Fatalf("wrong retrieval filter for mode %s", mode)
		}
	}
}
