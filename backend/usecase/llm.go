package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/pkoukk/tiktoken-go"
	"github.com/samber/lo"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/rag"
	"github.com/chaitin/panda-wiki/utils"
)

type LLMUsecase struct {
	rag              rag.RAGService
	conversationRepo *pg.ConversationRepository
	kbRepo           *pg.KnowledgeBaseRepository
	nodeRepo         *pg.NodeRepository
	enterprise       *EnterpriseUsecase
	modelRepo        *pg.ModelRepository
	promptRepo       *pg.PromptRepo
	config           *config.Config
	logger           *log.Logger
	modelkit         *modelkit.ModelKit
}

const (
	summaryChunkTokenLimit = 30720 // 30KB tokens per chunk
	summaryMaxChunks       = 4     // max chunks to process for summary
)

func NewLLMUsecase(config *config.Config, rag rag.RAGService, conversationRepo *pg.ConversationRepository, kbRepo *pg.KnowledgeBaseRepository, nodeRepo *pg.NodeRepository, enterprise *EnterpriseUsecase, modelRepo *pg.ModelRepository, promptRepo *pg.PromptRepo, logger *log.Logger) *LLMUsecase {
	tiktoken.SetBpeLoader(&utils.Localloader{})
	modelkit := modelkit.NewModelKit(logger.Logger)
	return &LLMUsecase{
		config:           config,
		rag:              rag,
		conversationRepo: conversationRepo,
		kbRepo:           kbRepo,
		nodeRepo:         nodeRepo,
		enterprise:       enterprise,
		modelRepo:        modelRepo,
		promptRepo:       promptRepo,
		logger:           logger.WithModule("usecase.llm"),
		modelkit:         modelkit,
	}
}

func (u *LLMUsecase) BuildConversationMessageWithRAG(
	ctx context.Context,
	conversationID string,
	kbID string,
	groupIDs []int,
	accessCtx *domain.EnterpriseAccessContext,
	systemPrompt string,
	requestHistory []*schema.Message,
) ([]*schema.Message, []*domain.RankedNodeChunks, error) {
	messages := make([]*schema.Message, 0)
	rankedNodes := make([]*domain.RankedNodeChunks, 0)

	msgs, err := u.conversationRepo.GetConversationMessagesByID(ctx, conversationID)
	if err != nil {
		u.logger.Error("get conversation messages failed", log.Error(err))
		return nil, nil, errors.New("get conversation messages failed")
	}
	if len(msgs) > 0 {
		historyMessages := make([]*schema.Message, 0, len(requestHistory)+len(msgs))
		for _, msg := range requestHistory {
			if msg.Role == schema.System {
				continue
			}
			historyMessages = append(historyMessages, msg)
		}
		for _, msg := range msgs {
			switch msg.Role {
			case schema.Assistant:
				if strings.TrimSpace(msg.Content) == "" {
					continue
				}
				historyMessages = append(historyMessages, schema.AssistantMessage(msg.Content, nil))
			case schema.User:
				content := u.formatMessageWithImages(msg.Content, msg.ImagePaths)
				if strings.TrimSpace(content) == "" {
					continue
				}
				historyMessages = append(historyMessages, schema.UserMessage(content))
			default:
				continue
			}
		}
		if len(historyMessages) > 0 {
			question := historyMessages[len(historyMessages)-1].Content
			var rewrittenQuery string
			if systemPrompt == "" {
				if settingPrompt, err := u.promptRepo.GetPromptContent(ctx, kbID); err != nil {
					u.logger.Error("get prompt from settings failed", log.Error(err))
				} else {
					if settingPrompt != "" {
						systemPrompt = settingPrompt
					} else {
						systemPrompt = domain.SystemDefaultPrompt
					}
				}
			}

			if systemPrompt == "" {
				systemPrompt = domain.SystemDefaultPrompt
			}
			for _, msg := range requestHistory {
				if msg.Role == schema.System {
					systemPrompt += "\n调用方补充要求：\n" + msg.Content
				}
			}
			systemPrompt += "\n" + domain.ConversationContextPrompt
			template := prompt.FromMessages(schema.GoTemplate,
				schema.SystemMessage(systemPrompt),
				schema.UserMessage(domain.UserQuestionFormatter),
			)
			kb, err := u.kbRepo.GetKnowledgeBaseByID(ctx, kbID)
			if err != nil {
				u.logger.Error("get kb failed", log.Error(err))
				return nil, nil, errors.New("get kb failed")
			}
			rewrittenQuery, rankedNodes, err = u.GetRankNodes(ctx, GetRankNodesRequest{
				KBID:                kbID,
				DatasetID:           kb.DatasetID,
				Question:            question,
				GroupIDs:            groupIDs,
				SimilarityThreshold: 0.2,
				HistoryMessages:     historyMessages[:len(historyMessages)-1],
			})
			if err != nil {
				u.logger.Error("get rank nodes failed", log.Error(err))
				return nil, nil, errors.New("get rank nodes failed")
			}
			rankedNodes, err = u.filterRankedNodesByEnterpriseAccess(ctx, rankedNodes, accessCtx)
			if err != nil {
				u.logger.Error("filter rank nodes by enterprise access failed", log.Error(err))
				return nil, nil, errors.New("filter rank nodes failed")
			}
			documents := domain.FormatNodeChunks(rankedNodes, kb.AccessSettings.BaseURL)
			if strings.TrimSpace(rewrittenQuery) == "" {
				rewrittenQuery = question
			}
			u.logger.Debug("documents", log.String("documents", documents))

			formattedMessages, err := template.Format(ctx, map[string]any{
				"CurrentDate":    time.Now().Format("2006-01-02"),
				"Question":       question,
				"RetrievalQuery": rewrittenQuery,
				"Documents":      documents,
			})
			if err != nil {
				u.logger.Error("format messages failed", log.Error(err))
				return nil, nil, errors.New("format messages failed")
			}
			messages = slices.Insert(formattedMessages, 1, historyMessages[:len(historyMessages)-1]...)
		}
	}
	return messages, rankedNodes, nil
}

func (u *LLMUsecase) filterRankedNodesByEnterpriseAccess(ctx context.Context, rankedNodes []*domain.RankedNodeChunks, accessCtx *domain.EnterpriseAccessContext) ([]*domain.RankedNodeChunks, error) {
	if u.enterprise == nil || accessCtx == nil || len(rankedNodes) == 0 {
		return rankedNodes, nil
	}

	nodeIDs := lo.Map(rankedNodes, func(node *domain.RankedNodeChunks, _ int) string {
		return node.NodeID
	})
	nodesMap, err := u.nodeRepo.GetNodesByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	for _, nodeInfo := range nodesMap {
		resolvedPermissions, resolveErr := u.enterprise.ResolveNodePermissions(ctx, nodeInfo)
		if resolveErr == nil {
			nodeInfo.Permissions = resolvedPermissions
		}
		if nodeInfo.NavId != "" && u.enterprise != nil && u.enterprise.navRepo != nil {
			if nav, navErr := u.enterprise.navRepo.GetById(ctx, nodeInfo.NavId); navErr == nil {
				nodeInfo.Permissions = u.enterprise.MergeNodeAndNavPermissions(nodeInfo.Permissions, nav.Permissions)
			}
		}
	}

	filtered := make([]*domain.RankedNodeChunks, 0, len(rankedNodes))
	for _, rankedNode := range rankedNodes {
		nodeInfo, ok := nodesMap[rankedNode.NodeID]
		if !ok {
			continue
		}
		if !u.enterprise.CanAnswerable(nodeInfo.Permissions, accessCtx) {
			continue
		}
		filtered = append(filtered, rankedNode)
	}
	return filtered, nil
}

func (u *LLMUsecase) ChatWithAgent(
	ctx context.Context,
	chatModel model.BaseChatModel,
	messages []*schema.Message,
	usage *schema.TokenUsage,
	onChunk func(ctx context.Context, dataType, chunk string) error,
) error {
	resp, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return fmt.Errorf("stream failed: %w", err)
	}
	firstReasoning := false
	firstData := false

	for {
		msg, err := resp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("recv failed: %w", err)
		}
		reasoning, ok := deepseek.GetReasoningContent(msg)
		if ok && reasoning != "" {
			if !firstReasoning {
				firstReasoning = true
				reasoning = "<think>" + reasoning
			}
			if err := onChunk(ctx, "data", reasoning); err != nil {
				return fmt.Errorf("on chunk reasoning: %w", err)
			}
			continue
		}
		if firstReasoning && !firstData {
			firstData = true
			msg.Content = "</think>\n" + msg.Content
			if err := onChunk(ctx, "data", msg.Content); err != nil {
				return fmt.Errorf("on chunk data: %w", err)
			}
			continue
		}
		if msg.Content == "" {
			if msg.ResponseMeta.Usage != nil {
				*usage = *msg.ResponseMeta.Usage
			}
			continue
		}
		if err := onChunk(ctx, "data", msg.Content); err != nil {
			return fmt.Errorf("on chunk data: %w", err)
		}

		// set to usage
		if msg.ResponseMeta.Usage != nil {
			*usage = *msg.ResponseMeta.Usage
		}
	}

	return nil
}

func (u *LLMUsecase) Generate(
	ctx context.Context,
	chatModel model.BaseChatModel,
	messages []*schema.Message,
) (string, error) {
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate failed: %w", err)
	}
	return resp.Content, nil
}

func (u *LLMUsecase) SummaryNode(ctx context.Context, kbID string, model *domain.Model, name, content string) (string, error) {
	modelkitModel, err := model.ToModelkitModel()
	if err != nil {
		return "", err
	}
	chatModel, err := u.modelkit.GetChatModel(ctx, modelkitModel)
	if err != nil {
		return "", err
	}

	chunks, err := u.SplitByTokenLimit(content, summaryChunkTokenLimit)
	if err != nil {
		return "", err
	}
	if len(chunks) > summaryMaxChunks {
		u.logger.Debug("trim summary chunks for large document", log.String("node", name), log.Int("original_chunks", len(chunks)), log.Int("used_chunks", summaryMaxChunks))
		chunks = chunks[:summaryMaxChunks]
	}

	summaries := make([]string, 0, len(chunks))
	for idx, chunk := range chunks {
		summary, err := u.requestSummary(ctx, kbID, chatModel, name, chunk)
		if err != nil {
			u.logger.Error("Failed to generate summary for chunk", log.Int("chunk_index", idx), log.Error(err))
			continue
		}
		if summary == "" {
			u.logger.Warn("Empty summary returned for chunk", log.Int("chunk_index", idx))
			continue
		}
		summaries = append(summaries, summary)
	}

	if len(summaries) == 0 {
		return "", fmt.Errorf("failed to generate summary for document %s", name)
	}
	if len(summaries) == 1 {
		return summaries[0], nil
	}

	// Join all summaries and generate final summary
	joined := strings.Join(summaries, "\n\n")
	finalSummary, err := u.requestSummary(ctx, kbID, chatModel, name, joined)
	if err != nil {
		u.logger.Error("Failed to generate final summary, using aggregated summaries", log.Error(err))
		// Fallback: return the joined summaries directly
		if len(joined) > 500 {
			return joined[:500] + "...", nil
		}
		return joined, nil
	}
	return finalSummary, nil
}

func (u *LLMUsecase) StreamSummaryNode(
	ctx context.Context,
	kbID string,
	model *domain.Model,
	name, content string,
	onChunk func(ctx context.Context, dataType, chunk string) error,
) error {
	modelkitModel, err := model.ToModelkitModel()
	if err != nil {
		return err
	}
	chatModel, err := u.modelkit.GetChatModel(ctx, modelkitModel)
	if err != nil {
		return err
	}

	chunks, err := u.SplitByTokenLimit(content, summaryChunkTokenLimit)
	if err != nil {
		return err
	}
	if len(chunks) > summaryMaxChunks {
		u.logger.Debug("trim summary chunks for large document", log.String("node", name), log.Int("original_chunks", len(chunks)), log.Int("used_chunks", summaryMaxChunks))
		chunks = chunks[:summaryMaxChunks]
	}

	if len(chunks) == 1 {
		return u.streamSummary(ctx, kbID, chatModel, name, chunks[0], onChunk)
	}

	summaries := make([]string, 0, len(chunks))
	for idx, chunk := range chunks {
		summary, summaryErr := u.requestSummary(ctx, kbID, chatModel, name, chunk)
		if summaryErr != nil {
			u.logger.Error("Failed to generate summary for chunk", log.Int("chunk_index", idx), log.Error(summaryErr))
			continue
		}
		if summary == "" {
			u.logger.Warn("Empty summary returned for chunk", log.Int("chunk_index", idx))
			continue
		}
		summaries = append(summaries, summary)
	}

	if len(summaries) == 0 {
		return fmt.Errorf("failed to generate summary for document %s", name)
	}
	if len(summaries) == 1 {
		if err := onChunk(ctx, "data", summaries[0]); err != nil {
			return fmt.Errorf("on chunk data: %w", err)
		}
		return nil
	}

	joined := strings.Join(summaries, "\n\n")
	if err := u.streamSummary(ctx, kbID, chatModel, name, joined, onChunk); err != nil {
		u.logger.Error("Failed to generate final summary, using aggregated summaries", log.Error(err))
		if len(joined) > 500 {
			joined = joined[:500] + "..."
		}
		if chunkErr := onChunk(ctx, "data", joined); chunkErr != nil {
			return fmt.Errorf("on chunk data: %w", chunkErr)
		}
	}
	return nil
}

func (u *LLMUsecase) trimThinking(summary string) string {
	if !strings.HasPrefix(summary, "<think>") {
		return summary
	}
	endIndex := strings.Index(summary, "</think>")
	if endIndex == -1 {
		return summary
	}
	return strings.TrimSpace(summary[endIndex+len("</think>"):])
}

func (u *LLMUsecase) requestSummary(ctx context.Context, kbID string, chatModel model.BaseChatModel, name, content string) (string, error) {
	summaryPrompt, err := u.promptRepo.GetSummaryPrompt(ctx, kbID)
	if err != nil {
		return "", err
	}

	summary, err := u.Generate(ctx, chatModel, []*schema.Message{
		{
			Role:    "system",
			Content: summaryPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("文档名称：%s\n文档内容：%s", name, content),
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(u.trimThinking(summary)), nil
}

func (u *LLMUsecase) streamSummary(
	ctx context.Context,
	kbID string,
	chatModel model.BaseChatModel,
	name, content string,
	onChunk func(ctx context.Context, dataType, chunk string) error,
) error {
	summaryPrompt, err := u.promptRepo.GetSummaryPrompt(ctx, kbID)
	if err != nil {
		return err
	}

	usage := schema.TokenUsage{}
	filter := newThinkingStreamFilter()
	return u.ChatWithAgent(ctx, chatModel, []*schema.Message{
		{
			Role:    "system",
			Content: summaryPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("文档名称：%s\n文档内容：%s", name, content),
		},
	}, &usage, func(ctx context.Context, dataType, chunk string) error {
		if dataType != "data" {
			return onChunk(ctx, dataType, chunk)
		}
		cleaned := filter.Append(chunk)
		if cleaned == "" {
			return nil
		}
		return onChunk(ctx, dataType, cleaned)
	})
}

type thinkingStreamFilter struct {
	buffer strings.Builder
	done   bool
}

func newThinkingStreamFilter() *thinkingStreamFilter {
	return &thinkingStreamFilter{}
}

func (f *thinkingStreamFilter) Append(chunk string) string {
	if f.done {
		return chunk
	}
	f.buffer.WriteString(chunk)
	content := f.buffer.String()
	if !strings.HasPrefix(content, "<think>") {
		f.done = true
		f.buffer.Reset()
		return content
	}
	endIndex := strings.Index(content, "</think>")
	if endIndex == -1 {
		return ""
	}
	cleaned := strings.TrimSpace(content[endIndex+len("</think>"):])
	f.done = true
	f.buffer.Reset()
	return cleaned
}

func (u *LLMUsecase) SplitByTokenLimit(text string, maxTokens int) ([]string, error) {
	if maxTokens <= 0 {
		return nil, fmt.Errorf("maxTokens must be greater than 0")
	}
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get encoding: %w", err)
	}
	tokens := encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return []string{text}, nil
	}

	// 预先计算需要的片段数量并分配空间
	numChunks := (len(tokens) + maxTokens - 1) / maxTokens // 向上取整
	result := make([]string, 0, numChunks)

	for i := 0; i < len(tokens); i += maxTokens {
		end := i + maxTokens
		if end > len(tokens) {
			end = len(tokens)
		}

		chunk := tokens[i:end]
		decodedChunk := encoding.Decode(chunk)
		result = append(result, decodedChunk)
	}

	return result, nil
}

type GetRankNodesRequest struct {
	KBID                string
	DatasetID           string
	Question            string
	GroupIDs            []int
	SimilarityThreshold float64
	HistoryMessages     []*schema.Message
	MaxChunksPerDoc     int
}

func (u *LLMUsecase) GetRankNodes(ctx context.Context, req GetRankNodesRequest) (string, []*domain.RankedNodeChunks, error) {
	var rankedNodes []*domain.RankedNodeChunks
	// get related documents from raglite
	rewrittenQuery, records, err := u.rag.QueryRecords(ctx, &rag.QueryRecordsRequest{
		DatasetID:           req.DatasetID,
		Query:               req.Question,
		GroupIDs:            req.GroupIDs,
		SimilarityThreshold: req.SimilarityThreshold,
		HistoryMsgs:         req.HistoryMessages,
		MaxChunksPerDoc:     req.MaxChunksPerDoc,
	})
	if err != nil {
		return "", nil, fmt.Errorf("get records from raglite failed: %w", err)
	}
	u.logger.Info("get related documents from raglite", log.Any("record_count", len(records)))
	if len(records) == 0 && req.KBID != "" {
		fallbackQuery := rewrittenQuery
		if strings.TrimSpace(fallbackQuery) == "" {
			fallbackQuery = req.Question
		}
		fallbackNodes, fallbackErr := u.getRankNodesByLocalFallback(ctx, req.KBID, fallbackQuery, 5)
		if fallbackErr != nil {
			u.logger.Warn("local fallback retrieval failed", log.Error(fallbackErr))
		} else if len(fallbackNodes) > 0 {
			u.logger.Info("get related documents from local fallback", log.Any("record_count", len(fallbackNodes)))
			return rewrittenQuery, fallbackNodes, nil
		}
	}
	rankedNodesMap := make(map[string]*domain.RankedNodeChunks)
	// get raw node by doc_id
	if len(records) > 0 {
		docIDs := lo.Uniq(lo.Map(records, func(item *domain.NodeContentChunk, _ int) string {
			return item.DocID
		}))
		u.logger.Info("node chunk doc ids", log.Any("docIDs", docIDs))
		docIDNode, err := u.nodeRepo.GetNodeReleasesWithPathsByDocIDs(ctx, docIDs)
		if err != nil {
			return "", nil, fmt.Errorf("get nodes by ids failed: %w", err)
		}
		u.logger.Info("get node release by doc ids", log.Any("docIDNode", lo.Keys(docIDNode)))
		for _, record := range records {
			if nodeChunk, ok := rankedNodesMap[record.DocID]; !ok {
				if docNode, ok := docIDNode[record.DocID]; ok {
					rankNodeChunk := &domain.RankedNodeChunks{
						NodeID:        docNode.NodeID,
						NodeName:      docNode.Name,
						NodeSummary:   docNode.Meta.Summary,
						NodeEmoji:     docNode.Meta.Emoji,
						NodePathNames: docNode.PathNames,
						Chunks:        []*domain.NodeContentChunk{record},
					}
					rankedNodes = append(rankedNodes, rankNodeChunk)
					rankedNodesMap[record.DocID] = rankNodeChunk
				}
			} else {
				nodeChunk.Chunks = append(nodeChunk.Chunks, record)
			}
		}
	}
	return rewrittenQuery, rankedNodes, nil
}

func (u *LLMUsecase) getRankNodesByLocalFallback(ctx context.Context, kbID, question string, limit int) ([]*domain.RankedNodeChunks, error) {
	releases, err := u.nodeRepo.GetLatestDocumentReleasesWithPathsByKBID(ctx, kbID, 500)
	if err != nil {
		return nil, err
	}
	terms := buildLocalSearchTerms(question)
	if len(terms) == 0 {
		return nil, nil
	}

	type scoredRelease struct {
		release *pg.NodeReleaseWithPath
		score   int
	}
	scored := make([]scoredRelease, 0)
	for _, release := range releases {
		score := scoreNodeReleaseForQuestion(release, question, terms)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredRelease{release: release, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].release.UpdatedAt.After(scored[j].release.UpdatedAt)
		}
		return scored[i].score > scored[j].score
	})
	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	rankedNodes := make([]*domain.RankedNodeChunks, 0, limit)
	for _, item := range scored[:limit] {
		release := item.release
		rankedNodes = append(rankedNodes, &domain.RankedNodeChunks{
			NodeID:        release.NodeID,
			NodeName:      release.Name,
			NodeSummary:   release.Meta.Summary,
			NodeEmoji:     release.Meta.Emoji,
			NodePathNames: release.PathNames,
			Chunks: []*domain.NodeContentChunk{
				{
					ID:      release.ID,
					KBID:    release.KBID,
					DocID:   release.DocID,
					Name:    release.Name,
					Content: release.Content,
				},
			},
		})
	}
	return rankedNodes, nil
}

func scoreNodeReleaseForQuestion(release *pg.NodeReleaseWithPath, question string, terms []string) int {
	query := normalizeLocalSearchText(question)
	name := normalizeLocalSearchText(release.Name)
	baseName := normalizeLocalSearchText(strings.TrimSuffix(release.Name, filepath.Ext(release.Name)))
	content := normalizeLocalSearchText(release.Content)
	pathText := normalizeLocalSearchText(strings.Join(release.PathNames, " "))

	score := 0
	if baseName != "" && strings.Contains(query, baseName) {
		score += 300
	}
	if name != "" && strings.Contains(query, name) {
		score += 250
	}
	for _, term := range terms {
		weight := len([]rune(term))
		if weight < 2 {
			continue
		}
		if strings.Contains(name, term) {
			score += 20 * weight
		}
		if strings.Contains(baseName, term) {
			score += 24 * weight
		}
		if strings.Contains(pathText, term) {
			score += 8 * weight
		}
		if strings.Contains(content, term) {
			score += 4 * weight
		}
	}
	return score
}

func buildLocalSearchTerms(question string) []string {
	normalized := normalizeLocalSearchText(question)
	if normalized == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(term string) {
		term = strings.TrimSpace(normalizeLocalSearchText(term))
		if len([]rune(term)) < 2 {
			return
		}
		seen[term] = struct{}{}
	}

	for _, token := range strings.FieldsFunc(normalized, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	}) {
		add(token)
		runes := []rune(token)
		if len(runes) >= 4 {
			maxN := 8
			if len(runes) < maxN {
				maxN = len(runes)
			}
			for n := 2; n <= maxN; n++ {
				for i := 0; i+n <= len(runes); i++ {
					add(string(runes[i : i+n]))
				}
			}
		}
	}

	commonTerms := []string{"账号", "账户", "密码", "地址", "网址", "路径", "链接", "演示", "平台", "系统", "功能", "介绍"}
	for _, term := range commonTerms {
		if strings.Contains(normalized, term) {
			add(term)
		}
	}

	terms := make([]string, 0, len(seen))
	for term := range seen {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(i, j int) bool {
		return len([]rune(terms[i])) > len([]rune(terms[j]))
	})
	return terms
}

func normalizeLocalSearchText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer("　", " ", "\r", "\n", "\t", " ")
	return replacer.Replace(text)
}

// formatMessageWithImages converts image paths to markdown format and appends to message
func (u *LLMUsecase) formatMessageWithImages(message string, imagePaths []string) string {
	if len(imagePaths) == 0 {
		return message
	}
	var builder strings.Builder
	builder.WriteString(message)
	for _, path := range imagePaths {
		builder.WriteString("\n")
		fmt.Fprintf(&builder, "![](%s)", path)
	}
	return builder.String()
}
