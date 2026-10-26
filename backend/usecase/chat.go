package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/utils"
)

type ChatUsecase struct {
	llmUsecase          *LLMUsecase
	conversationUsecase *ConversationUsecase
	modelUsecase        *ModelUsecase
	appRepo             *pg.AppRepository
	blockWordRepo       *pg.BlockWordRepo
	kbRepo              *pg.KnowledgeBaseRepository
	nodeRepo            *pg.NodeRepository
	AuthRepo            *pg.AuthRepo
	enterpriseUsecase   *EnterpriseUsecase
	logger              *log.Logger
	modelkit            *modelkit.ModelKit
}

func NewChatUsecase(llmUsecase *LLMUsecase, kbRepo *pg.KnowledgeBaseRepository, conversationUsecase *ConversationUsecase, modelUsecase *ModelUsecase, appRepo *pg.AppRepository,
	blockWordRepo *pg.BlockWordRepo, nodeRepo *pg.NodeRepository, authRepo *pg.AuthRepo, enterpriseUsecase *EnterpriseUsecase, logger *log.Logger) (*ChatUsecase, error) {
	modelkit := modelkit.NewModelKit(logger.Logger)
	u := &ChatUsecase{
		llmUsecase:          llmUsecase,
		conversationUsecase: conversationUsecase,
		modelUsecase:        modelUsecase,
		appRepo:             appRepo,
		blockWordRepo:       blockWordRepo,
		kbRepo:              kbRepo,
		nodeRepo:            nodeRepo,
		AuthRepo:            authRepo,
		enterpriseUsecase:   enterpriseUsecase,
		logger:              logger.WithModule("usecase.chat"),
		modelkit:            modelkit,
	}
	if err := u.initDFA(); err != nil {
		u.logger.Error("failed to init dfa", log.Error(err))
		return nil, err
	}
	return u, nil
}

func (u *ChatUsecase) initDFA() error {
	ctx := context.Background()
	kbList, err := u.kbRepo.GetKnowledgeBaseList(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get kb list: %w", err)
	}
	for _, kb := range kbList {
		if kb != nil {
			words, err := u.blockWordRepo.GetBlockWords(ctx, kb.ID)
			if err != nil {
				u.logger.Error("failed to get words", log.Error(err), log.String("kb_id", kb.ID))
				return fmt.Errorf("failed to get words for kb: %w", err)
			}
			if len(words) > 0 {
				utils.InitDFA(kb.ID, words)
			}
		}
	}
	return nil
}

func (u *ChatUsecase) Chat(ctx context.Context, req *domain.ChatRequest) (<-chan domain.SSEEvent, error) {
	eventCh := make(chan domain.SSEEvent, 100)
	go func() {
		defer close(eventCh)
		requestStarted := time.Now()
		// 1. get app detail and validate app
		app, err := u.appRepo.GetOrCreateAppByKBIDAndType(ctx, req.KBID, req.AppType)
		if err != nil {
			eventCh <- domain.SSEEvent{Type: "error", Content: "app not found"}
			return
		}
		req.KBID = app.KBID
		req.AppID = app.ID
		req.AppType = app.Type
		// 2. conversation management
		if req.AppType == domain.AppTypeWechatServiceBot || req.AppType == domain.AppTypeWechatBot || req.AppType == domain.AppTypeWecomAIBot { // wechat service has its own id
			nonce := uuid.New().String()
			eventCh <- domain.SSEEvent{Type: "conversation_id", Content: req.ConversationID}
			eventCh <- domain.SSEEvent{Type: "nonce", Content: nonce}
			err = u.conversationUsecase.CreateConversation(ctx, &domain.Conversation{
				ID:        req.ConversationID,
				Nonce:     nonce,
				AppID:     req.AppID,
				KBID:      req.KBID,
				Subject:   req.Message,
				RemoteIP:  req.RemoteIP,
				Info:      req.Info,
				CreatedAt: time.Now(),
			})
			if err != nil {
				u.logger.Error("failed to create chat conversation", log.Error(err))
				eventCh <- domain.SSEEvent{Type: "error", Content: "failed to create chat conversation"}
				return
			}
		} else if req.ConversationID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				u.logger.Error("failed to generate conversation uuid", log.Error(err))
				id = uuid.New()
			}
			conversationID := id.String()
			req.ConversationID = conversationID
			nonce := uuid.New().String()
			eventCh <- domain.SSEEvent{Type: "conversation_id", Content: conversationID}
			eventCh <- domain.SSEEvent{Type: "nonce", Content: nonce}
			err = u.conversationUsecase.CreateConversation(ctx, &domain.Conversation{
				ID:        conversationID,
				Nonce:     nonce,
				AppID:     req.AppID,
				KBID:      req.KBID,
				Subject:   req.Message,
				RemoteIP:  req.RemoteIP,
				Info:      req.Info,
				CreatedAt: time.Now(),
			})
			if err != nil {
				u.logger.Error("failed to create chat conversation", log.Error(err))
				eventCh <- domain.SSEEvent{Type: "error", Content: "failed to create chat conversation"}
				return
			}
		} else {
			if req.Nonce == "" {
				eventCh <- domain.SSEEvent{Type: "error", Content: "nonce is required"}
				return
			}
			err := u.conversationUsecase.ValidateChatConversation(ctx, req.KBID, req.ConversationID, req.Nonce)
			if err != nil {
				u.logger.Error("failed to validate chat conversation nonce", log.Error(err))
				eventCh <- domain.SSEEvent{Type: "error", Content: "validate chat conversation nonce failed"}
				return
			}
		}

		messageId := uuid.New().String()
		eventCh <- domain.SSEEvent{Type: "message_id", Content: messageId}
		userMessageId := uuid.New().String()
		traceID := uuid.NewString()
		trace := newChatTrace(traceID, req.KBID, req.ConversationID, userMessageId, messageId, req.Message, requestStarted)
		eventCh <- domain.SSEEvent{Type: "trace_id", Content: traceID}
		trace.add("request_received", "completed", map[string]any{"question": req.Message, "conversation_id": req.ConversationID, "kb_id": req.KBID, "app_type": req.AppType, "user_id": req.Info.UserInfo.UserID, "auth_user_id": req.Info.UserInfo.AuthUserID, "image_count": len(req.ImagePaths)}, map[string]any{"user_message_id": userMessageId, "assistant_message_id": messageId}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].StartedAt = requestStarted
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(requestStarted).Milliseconds()
		tracePersisted := false
		persistTrace := func() {
			if tracePersisted {
				return
			}
			if trace.trace.Status == "in_progress" {
				if ctx.Err() != nil {
					trace.finish("interrupted")
				} else {
					trace.finish("failed")
				}
			}
			persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if saveErr := u.conversationUsecase.repo.SaveChatTrace(persistCtx, trace.trace); saveErr != nil {
				u.logger.Error("save chat trace failed", log.Error(saveErr), log.String("trace_id", traceID), log.String("conversation_id", req.ConversationID))
			} else {
				tracePersisted = true
			}
		}
		defer persistTrace()
		// save user question to conversation message
		questionSaveStarted := time.Now()
		if err := u.conversationUsecase.CreateChatConversationMessage(ctx, req.KBID, &domain.ConversationMessage{
			ID:             userMessageId,
			ConversationID: req.ConversationID,
			KBID:           req.KBID,
			AppID:          req.AppID,
			Role:           schema.User,
			Content:        req.Message,
			ImagePaths:     req.ImagePaths,
			RemoteIP:       req.RemoteIP,
		}); err != nil {
			trace.add("save_user_question", "failed", nil, nil, err)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(questionSaveStarted).Milliseconds()
			trace.finish("failed")
			persistTrace()
			u.logger.Error("failed to save user question to conversation message", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to save user question to conversation message"}
			return
		}
		trace.add("save_user_question", "completed", nil, map[string]any{"user_message_id": userMessageId}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(questionSaveStarted).Milliseconds()
		modelLoadStarted := time.Now()
		model, modelErr := u.modelUsecase.GetChatModel(ctx)
		if modelErr != nil {
			trace.add("load_chat_model", "failed", nil, nil, modelErr)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(modelLoadStarted).Milliseconds()
			trace.finish("failed")
			persistTrace()
			if modelErr == gorm.ErrRecordNotFound {
				eventCh <- domain.SSEEvent{Type: "error", Content: "请前往管理后台，点击右上角的“系统设置”配置推理大模型。"}
			} else {
				eventCh <- domain.SSEEvent{Type: "error", Content: "模型获取失败"}
			}
			return
		}
		req.ModelInfo = model
		trace.trace.Model = string(model.Model)
		trace.add("load_chat_model", "completed", nil, map[string]any{"provider": model.Provider, "model": model.Model}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(modelLoadStarted).Milliseconds()
		// extra1. if user set question block words then check it
		blockWordsLoadStarted := time.Now()
		blockWords, err := u.blockWordRepo.GetBlockWords(ctx, req.KBID)
		if err != nil {
			trace.add("load_question_filter", "failed", nil, nil, err)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(blockWordsLoadStarted).Milliseconds()
			u.logger.Error("failed to get question block words", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get question block words"}
			return
		}
		trace.add("load_question_filter", "completed", nil, map[string]any{"configured_block_word_count": len(blockWords)}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(blockWordsLoadStarted).Milliseconds()
		if len(blockWords) > 0 { // check --> filter
			questionFilter := utils.GetDFA(req.KBID)
			if err := questionFilter.DFA.Check(req.Message); err != nil { // exist then return err
				answer := "**您的问题包含敏感词, AI 无法回答您的问题。**"
				trace.add("input_filter", "blocked", nil, map[string]any{"reason": "命中问题屏蔽词"}, nil)
				trace.add("final_answer", "completed", nil, map[string]any{"source": "question_block_word", "answer": answer}, nil)
				eventCh <- domain.SSEEvent{Type: "error", Content: answer}
				// save ai answer and set it err
				blockedAnswerSaveStarted := time.Now()
				if err := u.conversationUsecase.CreateChatConversationMessage(context.Background(), req.KBID, &domain.ConversationMessage{
					ID:             messageId,
					ConversationID: req.ConversationID,
					KBID:           req.KBID,
					AppID:          req.AppID,
					Role:           schema.Assistant,
					Content:        answer,
					Provider:       req.ModelInfo.Provider,
					Model:          string(req.ModelInfo.Model),
					RemoteIP:       req.RemoteIP,
					ParentID:       userMessageId,
				}); err != nil {
					trace.add("save_answer", "failed", nil, nil, err)
					trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(blockedAnswerSaveStarted).Milliseconds()
					u.logger.Error("failed to save assistant answer to conversation message", log.Error(err))
					eventCh <- domain.SSEEvent{Type: "error", Content: "failed to save assistant answer to conversation message"}
					return
				}
				trace.add("save_answer", "completed", nil, map[string]any{"assistant_message_id": messageId}, nil)
				trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(blockedAnswerSaveStarted).Milliseconds()
				trace.finish("blocked")
				return
			}
		}
		trace.add("input_filter", "completed", nil, map[string]any{"passed": true}, nil)

		if req.Info.UserInfo.AuthUserID == 0 {
			auth, _ := u.AuthRepo.GetAuthBySourceType(ctx, req.AppType.ToSourceType())
			if auth != nil {
				req.Info.UserInfo.AuthUserID = auth.ID
			}
		}

		kb, err := u.kbRepo.GetKnowledgeBaseByID(ctx, req.KBID)
		if err != nil {
			trace.add("load_knowledge_base", "failed", nil, nil, err)
			u.logger.Error("failed to get kb", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get kb"}
			return
		}
		trace.add("load_knowledge_base", "completed", nil, map[string]any{"kb_id": kb.ID, "dataset_id": kb.DatasetID, "enterprise_auth_enabled": kb.AccessSettings.EnterpriseAuth.Enabled}, nil)

		groupIds := []int(nil)
		if !kb.AccessSettings.EnterpriseAuth.Enabled {
			groupLookupStarted := time.Now()
			groupIds, err = u.AuthRepo.GetAuthGroupIdsWithParentsByAuthId(ctx, req.Info.UserInfo.AuthUserID)
			if err != nil {
				trace.add("load_user_permissions", "failed", nil, nil, err)
				u.logger.Error("failed to get auth groupIds", log.Error(err))
				eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get auth groupIds"}
				return
			}
			step := map[string]any{"user_group_ids": groupIds}
			trace.add("load_user_permissions", "completed", nil, step, nil)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(groupLookupStarted).Milliseconds()
		} else {
			trace.add("load_user_permissions", "skipped", nil, map[string]any{"reason": "知识库启用了企业身份访问控制"}, nil)
		}

		accessStarted := time.Now()
		shareAccess, err := u.enterpriseUsecase.BuildShareAccessContext(ctx, req.Info.UserInfo.UserID)
		if err != nil {
			trace.add("build_access_context", "failed", nil, nil, err)
			u.logger.Error("failed to build enterprise access context", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to build enterprise access context"}
			return
		}
		trace.add("build_access_context", "completed", nil, map[string]any{"authenticated_user": req.Info.UserInfo.UserID != ""}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(accessStarted).Milliseconds()

		messageBuildStarted := time.Now()
		messages, rankedNodes, err := u.llmUsecase.BuildConversationMessageWithRAGTraced(ctx, req.ConversationID, req.KBID, groupIds, shareAccess, req.Prompt, req.HistoryMessages, trace)
		if err != nil {
			trace.add("prepare_model_context", "failed", nil, nil, err)
			u.logger.Error("build messages failed", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: err.Error()}
			return
		}
		trace.add("prepare_model_context", "completed", nil, map[string]any{"message_count": len(messages), "document_count": len(rankedNodes)}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(messageBuildStarted).Milliseconds()

		u.logger.Debug("message:", log.Any("schema", messages))
		// 5. LLM inference (streaming callback), message storage, token statistics
		answer := ""
		usage := schema.TokenUsage{}

		modelConfigStarted := time.Now()
		modelkitModel, err := req.ModelInfo.ToModelkitModel()
		if err != nil {
			trace.add("model_configuration", "failed", nil, nil, err)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(modelConfigStarted).Milliseconds()
			u.logger.Error("failed to convert model to modelkit model", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to convert model to modelkit model"}
			return
		}
		chatModel, err := u.modelkit.GetChatModel(ctx, modelkitModel)

		if err != nil {
			trace.add("model_configuration", "failed", map[string]any{"provider": req.ModelInfo.Provider, "model": req.ModelInfo.Model}, nil, err)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(modelConfigStarted).Milliseconds()
			u.logger.Error("failed to get chat model", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get chat model"}
			return
		}
		trace.add("model_configuration", "completed", nil, map[string]any{"provider": req.ModelInfo.Provider, "model": req.ModelInfo.Model}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(modelConfigStarted).Milliseconds()
		// get words
		onChunkAC, flushBuffer := u.CreateAcOnChunk(ctx, req.KBID, &answer, eventCh, blockWords)

		inferenceStarted := time.Now()
		chatErr := u.llmUsecase.ChatWithAgent(ctx, chatModel, messages, &usage, onChunkAC)

		// 处理缓冲区中剩余的内容
		if flushBuffer != nil {
			flushBuffer(ctx, "data")
		}

		if chatErr != nil {
			trace.add("model_inference", "failed", map[string]any{"provider": req.ModelInfo.Provider, "model": req.ModelInfo.Model}, map[string]any{"partial_answer": answer, "prompt_tokens": usage.PromptTokens, "completion_tokens": usage.CompletionTokens}, chatErr)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(inferenceStarted).Milliseconds()
			u.logger.Error("对话失败", log.Error(chatErr))
			eventCh <- domain.SSEEvent{Type: "error", Content: "对话失败，请稍后再试"}
			return
		}
		trace.add("model_inference", "completed", map[string]any{"provider": req.ModelInfo.Provider, "model": req.ModelInfo.Model}, map[string]any{"prompt_tokens": usage.PromptTokens, "completion_tokens": usage.CompletionTokens, "total_tokens": usage.TotalTokens, "answer_empty": strings.TrimSpace(answer) == ""}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(inferenceStarted).Milliseconds()

		answerSource := "model"
		if strings.TrimSpace(answer) == "" {
			u.logger.Error("chat model returned empty answer", log.String("provider", string(req.ModelInfo.Provider)), log.String("model", string(req.ModelInfo.Model)), log.String("conversation_id", req.ConversationID))
			if len(rankedNodes) == 0 {
				answer = "抱歉，我当前的知识不足以回答这个问题。"
				answerSource = "server_fallback_no_retrieved_documents"
			} else {
				answer = buildFallbackAnswerFromRankedNodes(rankedNodes)
				answerSource = "server_fallback_from_retrieved_documents"
			}
			eventCh <- domain.SSEEvent{Type: "data", Content: answer}
		}
		trace.add("final_answer", "completed", nil, map[string]any{"source": answerSource, "answer": answer}, nil)

		persistAnswerStarted := time.Now()
		if err := u.conversationUsecase.CreateChatConversationMessage(ctx, req.KBID, &domain.ConversationMessage{
			ID:               messageId,
			ConversationID:   req.ConversationID,
			KBID:             req.KBID,
			AppID:            req.AppID,
			Role:             schema.Assistant,
			Content:          answer,
			Provider:         req.ModelInfo.Provider,
			Model:            string(req.ModelInfo.Model),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			RemoteIP:         req.RemoteIP,
			ParentID:         userMessageId,
		}); err != nil {
			trace.add("save_answer", "failed", nil, nil, err)
			u.logger.Error("failed to save assistant answer to conversation message", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to save assistant answer to conversation message"}
			return
		}
		trace.add("save_answer", "completed", nil, map[string]any{"assistant_message_id": messageId}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(persistAnswerStarted).Milliseconds()
		// update model usage
		usageUpdateStarted := time.Now()
		if err := u.modelUsecase.UpdateUsage(ctx, req.ModelInfo.ID, &usage); err != nil {
			trace.add("update_model_usage", "failed", nil, nil, err)
			trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(usageUpdateStarted).Milliseconds()
			trace.finish("failed")
			u.logger.Error("failed to update model usage", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to update model usage"}
			return
		}
		trace.add("update_model_usage", "completed", nil, map[string]any{"total_tokens": usage.TotalTokens}, nil)
		trace.trace.Steps[len(trace.trace.Steps)-1].DurationMS = time.Since(usageUpdateStarted).Milliseconds()

		for _, node := range rankedNodesCitedByAnswer(answer, rankedNodes) {
			chunkResult := domain.NodeContentChunkSSE{
				NodeID:        node.NodeID,
				Name:          node.NodeName,
				Summary:       node.NodeSummary,
				NodePathNames: node.NodePathNames,
			}
			eventCh <- domain.SSEEvent{Type: "chunk_result", ChunkResult: &chunkResult}
		}

		trace.finish("completed")
		persistTrace()
		eventCh <- domain.SSEEvent{Type: "done"}
	}()
	return eventCh, nil
}

func rankedNodesCitedByAnswer(answer string, nodes []*domain.RankedNodeChunks) []*domain.RankedNodeChunks {
	answer = strings.TrimSpace(answerAfterReasoning(answer))
	if isKnowledgeInsufficientAnswer(answer) {
		return nil
	}

	cited := make([]*domain.RankedNodeChunks, 0, len(nodes))
	seen := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if node == nil || node.NodeID == "" || !strings.Contains(answer, "node/"+node.NodeID) {
			continue
		}
		if _, exists := seen[node.NodeID]; exists {
			continue
		}
		seen[node.NodeID] = struct{}{}
		cited = append(cited, node)
	}
	return cited
}

func answerAfterReasoning(answer string) string {
	start := strings.LastIndex(answer, "<think>")
	end := strings.LastIndex(answer, "</think>")
	if start >= 0 && end < start {
		return ""
	}
	if end >= 0 {
		return answer[end+len("</think>"):]
	}
	return answer
}

func isKnowledgeInsufficientAnswer(answer string) bool {
	for _, phrase := range []string{
		"抱歉，我当前的知识不足以回答这个问题",
		"抱歉，我的知识范围不足以回答这个问题",
	} {
		if strings.Contains(answer, phrase) {
			return true
		}
	}
	return false
}

func buildFallbackAnswerFromRankedNodes(rankedNodes []*domain.RankedNodeChunks) string {
	const maxNodes = 3
	const maxChunkRunes = 1000

	var answer strings.Builder
	answer.WriteString("根据已检索到的文档，相关内容如下：\n\n")
	used := 0
	for _, node := range rankedNodes {
		if used >= maxNodes {
			break
		}
		answer.WriteString(fmt.Sprintf("%d. %s\n", used+1, node.NodeName))
		chunkCount := 0
		for _, chunk := range node.Chunks {
			content := strings.TrimSpace(chunk.Content)
			if content == "" {
				continue
			}
			runes := []rune(content)
			if len(runes) > maxChunkRunes {
				content = string(runes[:maxChunkRunes]) + "……"
			}
			if chunkCount > 0 {
				answer.WriteString("\n")
			}
			answer.WriteString(content)
			answer.WriteString("\n")
			chunkCount++
			if chunkCount >= 2 {
				break
			}
		}
		answer.WriteString("\n")
		used++
	}

	if used > 0 {
		answer.WriteString("---\n### 引用列表\n")
		for i := 0; i < used; i++ {
			node := rankedNodes[i]
			answer.WriteString(fmt.Sprintf("> [%d]. [%s](/node/%s)\n", i+1, node.NodeName, node.NodeID))
		}
	}
	return strings.TrimSpace(answer.String())
}

func (u *ChatUsecase) ChatRagOnly(ctx context.Context, req *domain.ChatRagOnlyRequest) (<-chan domain.SSEEvent, error) {
	eventCh := make(chan domain.SSEEvent, 100)
	go func() {
		defer close(eventCh)

		// extra1. if user set question block words then check it
		blockWords, err := u.blockWordRepo.GetBlockWords(ctx, req.KBID)
		if err != nil {
			u.logger.Error("failed to get question block words", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get question block words"}
			return
		}
		if len(blockWords) > 0 { // check --> filter
			questionFilter := utils.GetDFA(req.KBID)
			if err := questionFilter.DFA.Check(req.Message); err != nil { // exist then return err
				answer := "**您的问题包含敏感词, AI 无法回答您的问题。**"
				eventCh <- domain.SSEEvent{Type: "error", Content: answer}
				return
			}
		}

		if req.UserInfo.AuthUserID == 0 {
			auth, _ := u.AuthRepo.GetAuthBySourceType(ctx, req.AppType.ToSourceType())
			if auth != nil {
				req.UserInfo.AuthUserID = auth.ID
			}
		}

		// retrieve documents
		kb, err := u.kbRepo.GetKnowledgeBaseByID(ctx, req.KBID)
		if err != nil {
			u.logger.Error("failed to get kb", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get kb"}
			return
		}
		groupIds := []int(nil)
		if !kb.AccessSettings.EnterpriseAuth.Enabled {
			groupIds, err = u.AuthRepo.GetAuthGroupIdsWithParentsByAuthId(ctx, req.UserInfo.AuthUserID)
			if err != nil {
				u.logger.Error("failed to get auth groupIds", log.Error(err))
				eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get auth groupIds"}
				return
			}
		}
		_, rankedNodes, err := u.llmUsecase.GetRankNodes(ctx, GetRankNodesRequest{
			KBID:                req.KBID,
			DatasetID:           kb.DatasetID,
			Question:            req.Message,
			GroupIDs:            groupIds,
			HistoryMessages:     nil,
			SimilarityThreshold: 0,
			MaxChunksPerDoc:     1,
		})
		if err != nil {
			u.logger.Error("failed to get rank nodes", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to get rank nodes"}
			return
		}
		shareAccess, err := u.enterpriseUsecase.BuildShareAccessContext(ctx, req.UserInfo.UserID)
		if err != nil {
			u.logger.Error("failed to build enterprise access context", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to build enterprise access context"}
			return
		}
		rankedNodes, err = u.llmUsecase.filterRankedNodesByEnterpriseAccess(ctx, rankedNodes, shareAccess)
		if err != nil {
			u.logger.Error("failed to filter rank nodes", log.Error(err))
			eventCh <- domain.SSEEvent{Type: "error", Content: "failed to filter rank nodes"}
			return
		}
		documents := domain.FormatNodeChunks(rankedNodes, kb.AccessSettings.BaseURL)
		u.logger.Debug("documents", log.String("documents", documents))

		// send only the documents part
		eventCh <- domain.SSEEvent{Type: "data", Content: documents}
		eventCh <- domain.SSEEvent{Type: "done"}
	}()
	return eventCh, nil
}

func (u *ChatUsecase) CreateAcOnChunk(ctx context.Context, kbID string, answer *string, eventCh chan<- domain.SSEEvent, blockWords []string) (func(ctx context.Context, dataType, chunk string) error,
	func(ctx context.Context, dataType string)) {
	var buffer strings.Builder
	// 如果用户没有设置敏感词，不需要处理
	if len(blockWords) == 0 {
		onChunk := func(ctx context.Context, dataType, chunk string) error {
			*answer += chunk
			eventCh <- domain.SSEEvent{Type: dataType, Content: chunk}
			return nil
		}
		return onChunk, nil
	}

	// get filter --> exist
	filter := utils.GetDFA(kbID)

	onChunk := func(ctx context.Context, dataType, chunk string) error {
		buffer.WriteString(chunk)

		// 将缓冲区内容转换为 rune 切片，以便正确处理多字节字符
		bufferRunes := []rune(buffer.String())

		// 基于 rune 长度与 bufferSize 进行比较，确保正确处理多字节字符
		if len(bufferRunes) >= filter.BuffSize {
			fullContent := buffer.String() // get buffer string

			// 直接处理完整内容
			processedContent := u.replaceWithSimpleString(fullContent, filter.DFA)
			processedRunes := []rune(processedContent)

			// 输出前面的部分，保留后面bufferSize - 1个rune
			outputPart := string(processedRunes[:len(processedRunes)-filter.BuffSize+1])
			*answer += outputPart
			eventCh <- domain.SSEEvent{Type: dataType, Content: outputPart}

			// 清空缓冲区
			newBufferContent := string(processedRunes[len(processedRunes)-filter.BuffSize+1:])
			buffer.Reset()
			buffer.WriteString(newBufferContent)
		}
		return nil
	}

	flushBuffer := func(ctx context.Context, dataType string) { //小于bufferSize的内容
		bufferRunes := []rune(buffer.String())
		if len(bufferRunes) > 0 {
			fullContent := buffer.String()
			processedContent := u.replaceWithSimpleString(fullContent, filter.DFA)
			*answer += processedContent
			eventCh <- domain.SSEEvent{Type: dataType, Content: processedContent}
		}
	}

	return onChunk, flushBuffer
}

// replaceWithSimpleString
func (u *ChatUsecase) replaceWithSimpleString(content string, filter *utils.DFA) string {
	r1 := filter.Filter(content)
	return r1
}

func (u *ChatUsecase) Search(ctx context.Context, req *domain.ChatSearchReq) (*domain.ChatSearchResp, error) {
	kb, err := u.kbRepo.GetKnowledgeBaseByID(ctx, req.KBID)
	if err != nil {
		return nil, err
	}
	groupIds := []int(nil)
	shareAccess, err := u.enterpriseUsecase.BuildShareAccessContext(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	_, rankedNodes, err := u.llmUsecase.GetRankNodes(ctx, GetRankNodesRequest{
		KBID:                req.KBID,
		DatasetID:           kb.DatasetID,
		Question:            req.Message,
		GroupIDs:            groupIds,
		SimilarityThreshold: 0.2,
		HistoryMessages:     nil,
	})
	if err != nil {
		return nil, err
	}

	nodeIDs := lo.Map(rankedNodes, func(node *domain.RankedNodeChunks, _ int) string {
		return node.NodeID
	})

	nodesMap, err := u.nodeRepo.GetNodesByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	for _, nodeInfo := range nodesMap {
		resolvedPermissions, resolveErr := u.enterpriseUsecase.ResolveNodePermissions(ctx, nodeInfo)
		if resolveErr != nil {
			delete(nodesMap, nodeInfo.ID)
			continue
		}
		if nodeInfo.NavId != "" {
			if u.enterpriseUsecase.navRepo == nil {
				delete(nodesMap, nodeInfo.ID)
				continue
			}
			nav, navErr := u.enterpriseUsecase.navRepo.GetById(ctx, nodeInfo.NavId)
			if navErr != nil || nav.KbID != req.KBID {
				delete(nodesMap, nodeInfo.ID)
				continue
			}
			resolvedPermissions = u.enterpriseUsecase.MergeNodeAndNavPermissions(resolvedPermissions, nav.Permissions)
		}
		nodeInfo.Permissions = resolvedPermissions
	}

	resp := domain.ChatSearchResp{}
	for _, node := range rankedNodes {
		nodeInfo, ok := nodesMap[node.NodeID]
		if !ok || !u.enterpriseUsecase.CanVisitable(nodeInfo.Permissions, shareAccess) {
			continue
		}

		chunkResult := domain.NodeContentChunkSSE{
			NodeID:        node.NodeID,
			Name:          node.NodeName,
			Summary:       node.NodeSummary,
			Emoji:         node.NodeEmoji,
			NodePathNames: node.NodePathNames,
		}
		resp.NodeResult = append(resp.NodeResult, chunkResult)
	}
	return &resp, nil
}
