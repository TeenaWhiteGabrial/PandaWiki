package rag

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	raglite "github.com/chaitin/raglite-go-sdk"
)

func TestIndexDatasetUsesSeparateModelBinding(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/api/v1/models":
			if r.Method != "POST" || body["model_name"] != "embedding-new" || body["is_default"] == true || !strings.HasPrefix(body["name"].(string), "pandawiki-index-") {
				t.Error("serving/default model would be overwritten", body)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"model-new"}}`))
		case "/api/v1/datasets":
			if r.Method != "POST" || body["dense_model_id"] != "model-new" {
				t.Error("new dataset not explicitly bound", body)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"dataset-new"}}`))
		default:
			t.Error("unexpected mutation", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	cfg := &config.Config{RAG: config.RAGConfig{CTRAG: config.CTRAGConfig{BaseURL: server.URL}}}
	store, err := NewCTRAG(cfg, log.NewLogger(cfg))
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := store.CreateIndexKnowledgeBase(context.Background(), &domain.Model{Model: "embedding-new", Provider: "CompatiblePlatform", Type: domain.ModelTypeEmbedding})
	if err != nil || dataset != "dataset-new" {
		t.Fatal(dataset, err)
	}
	if len(paths) != 2 {
		t.Fatal("unexpected writes", paths)
	}
}

func TestIndexCandidateUploadDoesNotOverwriteServingDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/datasets/serving-dataset/documents" {
			t.Error("unexpected endpoint", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("document_id") != "" {
			t.Error("candidate upload reused serving document ID")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if header.Filename != "candidate-version.md" || !strings.Contains(string(body), "pandawiki-index-version:candidate-version") || !strings.Contains(string(body), "new text") {
			t.Error("candidate identity/content missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"document_id":"new-document"}}`))
	}))
	defer server.Close()
	cfg := &config.Config{RAG: config.RAGConfig{CTRAG: config.CTRAGConfig{BaseURL: server.URL}}}
	store, _ := NewCTRAG(cfg, log.NewLogger(cfg))
	id, err := store.UpsertRecords(context.Background(), &UpsertRecordsRequest{ID: "candidate-version", IndexVersionID: "candidate-version", DatasetID: "serving-dataset", Title: "document", Content: "new text"})
	if err != nil || id != "new-document" {
		t.Fatal(id, err)
	}
}

func TestEmbeddingCredentialSyncDoesNotRenameOldModel(t *testing.T) {
	model := &domain.Model{Model: "embedding-new", Provider: "CompatiblePlatform", Type: domain.ModelTypeEmbedding, APIKey: "rotated-key", BaseURL: "http://new-endpoint/v1"}
	oldConfig := indexModelConfig(model)
	oldConfig.APIKey = "previous-key"
	var updated []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"models": []raglite.AIModel{
				{ID: "model-old", ModelType: "embedding", Provider: "CompatiblePlatform", ModelName: "embedding-old", Config: oldConfig},
				{ID: "model-new", ModelType: "embedding", Provider: "CompatiblePlatform", ModelName: "embedding-new", Config: oldConfig},
			}}})
			return
		}
		updated = append(updated, r.URL.Path)
		if r.Method != "PUT" || r.URL.Path != "/api/v1/models/model-new" {
			t.Error("attempted to mutate a different vector space", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["model_name"]; ok {
			t.Error("connection edit attempted model rename")
		}
		if _, ok := body["is_default"]; ok {
			t.Error("connection edit changed default model")
		}
		config := body["config"].(map[string]any)
		if config["api_key"] != "rotated-key" {
			t.Error("credential was not updated")
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"model-new"}}`))
	}))
	defer server.Close()
	cfg := &config.Config{RAG: config.RAGConfig{CTRAG: config.CTRAGConfig{BaseURL: server.URL}}}
	store, _ := NewCTRAG(cfg, log.NewLogger(cfg))
	if err := store.SyncEmbeddingConnection(context.Background(), model); err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 {
		t.Fatal("unexpected credential updates", updated)
	}
}
