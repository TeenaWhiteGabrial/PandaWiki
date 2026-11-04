package rag

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/google/wire"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
)

var ErrEmbeddingModelNotConfigured = errors.New("尚未配置向量模型，请先在模型设置中配置并应用向量模型")

type QueryRecordsRequest struct {
	TopK                int
	DatasetID           string
	Query               string
	GroupIDs            []int
	Tags                []string
	SimilarityThreshold float64
	HistoryMsgs         []*schema.Message
	MaxChunksPerDoc     int
}

// IndexDatasetService builds a separately bound vector space without mutating
// the embedding model or dataset serving existing requests.
type IndexDatasetService interface {
	CreateIndexKnowledgeBase(context.Context, *domain.Model) (string, error)
	SyncEmbeddingConnection(context.Context, *domain.Model) error
}

type UpsertRecordsRequest struct {
	IndexVersionID string
	ID             string
	DatasetID      string
	DocID          string
	Title          string
	Content        string
	GroupIDs       []int
	Tags           []string
}

type DocumentMetadata struct {
	GroupIDs []int `json:"group_ids"`
}

type Document struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	DatasetID   string           `json:"dataset_id"`
	Status      string           `json:"status"`
	ProgressMsg string           `json:"progress_msg"`
	MetaData    DocumentMetadata `json:"meta_data"`
	Tags        []string         `json:"tags"`
}

type Dataset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RAGService interface {
	CreateKnowledgeBase(ctx context.Context) (string, error)
	UpsertRecords(ctx context.Context, req *UpsertRecordsRequest) (string, error)
	QueryRecords(ctx context.Context, req *QueryRecordsRequest) (string, []*domain.NodeContentChunk, error)
	DeleteRecords(ctx context.Context, datasetID string, docIDs []string) error
	DeleteKnowledgeBase(ctx context.Context, datasetID string) error
	UpdateDocumentGroupIDs(ctx context.Context, datasetID string, docID string, groupIds []int) error
	ListDocuments(ctx context.Context, datasetID string, documentIDs []string) ([]Document, error)
	ListKnowledgeBases(ctx context.Context) ([]Dataset, error)

	GetModelList(ctx context.Context) ([]*domain.Model, error)
	AddModel(ctx context.Context, model *domain.Model) (string, error)
	UpdateModel(ctx context.Context, model *domain.Model) error
	UpsertModel(ctx context.Context, model *domain.Model) error
	DeleteModel(ctx context.Context, model *domain.Model) error
}

func NewRAGService(config *config.Config, logger *log.Logger) (RAGService, error) {
	switch config.RAG.Provider {
	case "ct":
		return NewCTRAG(config, logger)
	default:
		return nil, fmt.Errorf("unsupported vector provider: %s", config.RAG.Provider)
	}
}

var ProviderSet = wire.NewSet(NewRAGService)
