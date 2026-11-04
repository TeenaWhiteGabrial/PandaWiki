package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/store/rag"
)

type assistantRetrievalStore struct {
	rag.RAGService
	datasets []rag.Dataset
	err      error
	calls    int
}

func (s *assistantRetrievalStore) ListKnowledgeBases(context.Context) ([]rag.Dataset, error) {
	s.calls++
	return s.datasets, s.err
}

func TestAssistantRetrievalFailureIsActionableWithoutLeakingUpstreamError(t *testing.T) {
	cause := errors.New("HTTP 500 from http://internal-service/?api_key=must-not-leak")
	for _, tc := range []struct {
		name      string
		datasetID string
		store     assistantRetrievalStore
		missing   bool
	}{
		{name: "restored DB missing vector data", datasetID: "old-dataset", missing: true},
		{name: "dataset not bound", missing: true},
		{name: "dataset exists but search failed", datasetID: "serving", store: assistantRetrievalStore{datasets: []rag.Dataset{{ID: "serving"}}}},
		{name: "service unavailable is not proof of missing index", datasetID: "serving", store: assistantRetrievalStore{err: errors.New("offline")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := assistantRetrievalFailure(context.Background(), &tc.store, tc.datasetID, cause)
			message := safeAssistantError(err)
			if !errors.Is(err, cause) || !strings.Contains(err.Error(), "must-not-leak") {
				t.Fatal("internal diagnosis was lost")
			}
			if strings.Contains(message, "must-not-leak") || strings.Contains(message, "internal-service") {
				t.Fatal("upstream details leaked to task card")
			}
			if (message == assistantIndexUnavailable) != tc.missing || !strings.Contains(message, "指定文档 / 目录") {
				t.Fatalf("incorrect recovery guidance: %s", message)
			}
			if tc.datasetID == "" && tc.store.calls != 0 {
				t.Fatal("unbound dataset should not query RAGLite")
			}
		})
	}
}
