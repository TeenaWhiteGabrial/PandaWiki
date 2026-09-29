package rag

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	raglite "github.com/chaitin/raglite-go-sdk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/utils"
)

type CTRAG struct {
	client *raglite.Client
	logger *log.Logger
	mdConv *converter.Converter
}

const (
	listDocumentsBatchSize  = 10
	listDocumentsMaxRetries = 3
	listDocumentsRetryDelay = 500 * time.Millisecond
)

func NewCTRAG(config *config.Config, logger *log.Logger) (*CTRAG, error) {
	client, err := raglite.NewClient(
		config.RAG.CTRAG.BaseURL,
		raglite.WithAPIKey(config.RAG.CTRAG.APIKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create raglite client: %w", err)
	}
	return &CTRAG{
		client: client,
		logger: logger.WithModule("store.vector.ct"),
		mdConv: NewHTML2MDConverter(),
	}, nil
}

func (s *CTRAG) CreateKnowledgeBase(ctx context.Context) (string, error) {
	dataset, err := s.client.Datasets.Create(ctx, &raglite.CreateDatasetRequest{
		Name: uuid.New().String(),
	})
	if err != nil {
		return "", err
	}
	return dataset.ID, nil
}

func (s *CTRAG) ListKnowledgeBases(ctx context.Context) ([]Dataset, error) {
	res, err := s.client.Datasets.List(ctx, &raglite.ListDatasetsRequest{})
	if err != nil {
		return nil, err
	}
	datasets := make([]Dataset, 0, len(res.Datasets))
	for _, item := range res.Datasets {
		datasets = append(datasets, Dataset{
			ID:   item.ID,
			Name: item.Name,
		})
	}
	return datasets, nil
}

func (s *CTRAG) QueryRecords(ctx context.Context, req *QueryRecordsRequest) (string, []*domain.NodeContentChunk, error) {
	var chatMsgs []raglite.ChatMessage
	for _, msg := range req.HistoryMsgs {
		switch msg.Role {
		case schema.User:
			chatMsgs = append(chatMsgs, raglite.ChatMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		case schema.Assistant:
			chatMsgs = append(chatMsgs, raglite.ChatMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		default:
			continue
		}
	}
	s.logger.Debug("retrieving by history msgs", log.Any("history_msgs", req.HistoryMsgs), log.Any("chat_msgs", chatMsgs))
	data := &raglite.RetrieveRequest{
		DatasetID: req.DatasetID,
		Query:     req.Query,
		TopK:      10,
		Metadata: map[string]interface{}{
			"group_ids": req.GroupIDs,
		},
		Tags:                req.Tags,
		SimilarityThreshold: req.SimilarityThreshold,
		ChatHistory:         chatMsgs,
		MaxChunksPerDoc:     req.MaxChunksPerDoc,
	}
	res, err := s.client.Search.Retrieve(ctx, data)
	if err != nil {
		return "", nil, err
	}
	s.logger.Info("retrieve chunks result", log.Int("chunks count", len(res.Results)), log.String("query", res.Query))
	nodeChunks := make([]*domain.NodeContentChunk, len(res.Results))
	for i, chunk := range res.Results {
		nodeChunks[i] = &domain.NodeContentChunk{
			ID:      chunk.ChunkID,
			Content: chunk.Content,
			DocID:   chunk.DocumentID,
		}
	}
	return res.Query, nodeChunks, nil
}

func (s *CTRAG) UpsertRecords(ctx context.Context, req *UpsertRecordsRequest) (string, error) {
	markdown := req.Content
	// if the content is html, convert it to markdown first
	if utils.IsLikelyHTML(req.Content) {
		var err error
		markdown, err = s.mdConv.ConvertString(req.Content)
		if err != nil {
			return "", fmt.Errorf("convert html to markdown failed: %w", err)
		}
	}
	markdown = buildSearchableMarkdown(req.Title, markdown)
	data := &raglite.UploadDocumentRequest{
		DatasetID:  req.DatasetID,
		DocumentID: req.DocID,
		Title:      req.Title,
		File:       strings.NewReader(markdown),
		Filename:   fmt.Sprintf("%s.md", req.ID),
		Metadata:   make(map[string]interface{}),
	}
	if req.GroupIDs != nil {
		data.Metadata["group_ids"] = req.GroupIDs
	}
	if req.Tags != nil {
		data.Tags = req.Tags
	}
	res, err := s.client.Documents.Upload(ctx, data)
	if err != nil {
		return "", fmt.Errorf("upload document text failed: %w", err)
	}
	return res.DocumentID, nil
}

func buildSearchableMarkdown(title, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" {
		return content
	}

	lines := []string{
		fmt.Sprintf("# %s", title),
		"",
		fmt.Sprintf("文档名称：%s", title),
	}

	baseName := strings.TrimSpace(strings.TrimSuffix(title, filepath.Ext(title)))
	if baseName != "" && baseName != title {
		lines = append(lines, fmt.Sprintf("文档关键词：%s", baseName))
	}

	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(title)), "."); ext != "" {
		lines = append(lines, fmt.Sprintf("文件类型：%s", ext))
	}

	if content != "" {
		lines = append(lines, "", content)
	}
	return strings.Join(lines, "\n")
}

func (s *CTRAG) DeleteRecords(ctx context.Context, datasetID string, docIDs []string) error {
	if err := s.client.Documents.BatchDelete(ctx, &raglite.BatchDeleteDocumentsRequest{
		DatasetID:   datasetID,
		DocumentIDs: docIDs,
	}); err != nil {
		return err
	}
	return nil
}

func (s *CTRAG) DeleteKnowledgeBase(ctx context.Context, datasetID string) error {
	if err := s.client.Datasets.Delete(ctx, datasetID); err != nil {
		return err
	}
	return nil
}

func (s *CTRAG) AddModel(ctx context.Context, model *domain.Model) (string, error) {
	maxTokens := model.Parameters.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	modelConfig, err := s.client.Models.Create(ctx, &raglite.CreateModelRequest{
		Name:      model.Model,
		Provider:  string(model.Provider),
		ModelType: string(model.Type),
		ModelName: model.Model,
		Config: raglite.AIModelConfig{
			APIBase:         model.BaseURL,
			APIKey:          model.APIKey,
			APIHeader:       model.APIHeader,
			APIVersion:      model.APIVersion,
			MaxTokens:       raglite.Ptr(maxTokens),
			ExtraParameters: model.Parameters.Map(),
		},
		IsDefault: true,
	})
	if err != nil {
		return "", err
	}
	return modelConfig.ID, nil
}

func (s *CTRAG) UpsertModel(ctx context.Context, model *domain.Model) error {
	maxTokens := model.Parameters.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	data := raglite.UpsertModelRequest{
		Name:      model.Model,
		Provider:  string(model.Provider),
		ModelName: model.Model,
		ModelType: string(model.Type),
		Config: raglite.AIModelConfig{
			APIBase:         model.BaseURL,
			APIKey:          model.APIKey,
			APIHeader:       model.APIHeader,
			APIVersion:      model.APIVersion,
			MaxTokens:       raglite.Ptr(maxTokens),
			ExtraParameters: model.Parameters.Map(),
		},
		IsDefault: true,
		IsActive:  model.IsActive,
	}
	_, err := s.client.Models.Upsert(ctx, &data)
	if err != nil {
		return err
	}
	return nil
}

func (s *CTRAG) UpdateModel(ctx context.Context, model *domain.Model) error {
	maxTokens := model.Parameters.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	data := raglite.UpdateModelRequest{
		Name:      raglite.Ptr(model.Model),
		Provider:  raglite.Ptr(string(model.Provider)),
		ModelName: raglite.Ptr(model.Model),
		Config: &raglite.AIModelConfig{
			APIBase:         model.BaseURL,
			APIKey:          model.APIKey,
			APIHeader:       model.APIHeader,
			APIVersion:      model.APIVersion,
			MaxTokens:       raglite.Ptr(maxTokens),
			ExtraParameters: model.Parameters.Map(),
		},
		IsDefault: raglite.Ptr(true),
		IsActive:  raglite.Ptr(model.IsActive),
	}
	_, err := s.client.Models.Update(ctx, model.ID, &data)
	if err != nil {
		return err
	}
	return nil
}

func (s *CTRAG) DeleteModel(ctx context.Context, model *domain.Model) error {
	err := s.client.Models.Delete(ctx, model.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *CTRAG) GetModelList(ctx context.Context) ([]*domain.Model, error) {
	res, err := s.client.Models.List(ctx, &raglite.ListModelsRequest{})
	if err != nil {
		return nil, err
	}
	models := make([]*domain.Model, len(res.Models))
	for i, model := range res.Models {
		models[i] = &domain.Model{
			ID:      model.ID,
			Model:   model.Name,
			BaseURL: model.Config.APIBase,
			APIKey:  model.Config.APIKey,
			Type:    domain.ModelType(model.ModelType),
		}
	}
	return models, nil
}

func (s *CTRAG) UpdateDocumentGroupIDs(ctx context.Context, datasetID string, docID string, groupIds []int) error {
	req := &raglite.UpdateDocumentRequest{
		DatasetID:  datasetID,
		DocumentID: docID,
		Metadata:   map[string]interface{}{},
	}
	if groupIds != nil {
		req.Metadata["group_ids"] = groupIds
	}
	_, err := s.client.Documents.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("update document group IDs failed: %w", err)
	}
	return nil
}

func (s *CTRAG) ListDocuments(ctx context.Context, datasetID string, documentIDs []string) ([]Document, error) {
	if len(documentIDs) > listDocumentsBatchSize {
		documents := make([]Document, 0, len(documentIDs))
		for start := 0; start < len(documentIDs); start += listDocumentsBatchSize {
			end := start + listDocumentsBatchSize
			if end > len(documentIDs) {
				end = len(documentIDs)
			}
			batchDocuments, err := s.listDocumentsWithRetry(ctx, datasetID, documentIDs[start:end])
			if err != nil {
				return nil, fmt.Errorf("list RAG documents failed for batch %d-%d/%d: %w", start+1, end, len(documentIDs), err)
			}
			documents = append(documents, batchDocuments...)
		}
		return documents, nil
	}
	return s.listDocumentsWithRetry(ctx, datasetID, documentIDs)
}

func (s *CTRAG) listDocumentsWithRetry(ctx context.Context, datasetID string, documentIDs []string) ([]Document, error) {
	var lastErr error
	for attempt := 1; attempt <= listDocumentsMaxRetries; attempt++ {
		documents, err := s.listDocuments(ctx, datasetID, documentIDs)
		if err == nil {
			return documents, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || attempt == listDocumentsMaxRetries {
			return nil, err
		}
		timer := time.NewTimer(time.Duration(attempt) * listDocumentsRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func (s *CTRAG) listDocuments(ctx context.Context, datasetID string, documentIDs []string) ([]Document, error) {
	res, err := s.client.Documents.List(ctx, &raglite.ListDocumentsRequest{
		DocumentIDs: documentIDs,
		DatasetID:   datasetID,
	})
	if err != nil {
		return nil, err
	}
	documents := make([]Document, len(res.Documents))
	for i, document := range res.Documents {
		documents[i] = Document{
			ID:          document.ID,
			Name:        document.Filename,
			DatasetID:   document.DatasetID,
			Status:      document.Status,
			ProgressMsg: document.ProgressMsg,
			Tags:        document.Tags,
			MetaData:    raglite.Decode[DocumentMetadata](document.Metadata),
		}
	}
	return documents, nil
}
