package usecase

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/chaitin/panda-wiki/store/rag"
)

func chatContextDB(t *testing.T) (*store.DB, *log.Logger) {
	t.Helper()
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for conversation integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	return &store.DB{DB: tx}, log.NewLogger(&config.Config{})
}

// Opt-in: sends only synthetic conversation content to the locally configured model.
func TestLiveFollowupAnswer(t *testing.T) {
	if os.Getenv("PANDA_WIKI_TEST_LIVE_LLM") != "1" {
		t.Skip("set PANDA_WIKI_TEST_LIVE_LLM=1 for configured-model validation")
	}
	db, logger := chatContextDB(t)
	var modelInfo domain.Model
	var setting domain.SystemSetting
	var mode domain.ModelModeSetting
	if err := db.Where("key = ?", consts.SystemSettingModelMode).First(&setting).Error; err == nil {
		if err := json.Unmarshal(setting.Value, &mode); err != nil {
			t.Fatal("invalid local model settings")
		}
	}
	if mode.Mode == consts.ModelSettingModeAuto {
		provider, err := (&ModelUsecase{}).getAutoModeProvider(mode)
		if err != nil {
			t.Skip("no valid local automatic model configured")
		}
		modelInfo = domain.Model{Provider: provider.Provider, Model: provider.Models.Chat, Type: domain.ModelTypeChat, BaseURL: getAutoModeModelBaseURL(provider, domain.ModelTypeChat), APIKey: provider.APIKey}
	} else if err := db.Where("type = ? AND is_active = true", domain.ModelTypeChat).First(&modelInfo).Error; err != nil {
		t.Skip("no local inference model configured")
	}
	metadata, err := modelInfo.ToModelkitModel()
	if err != nil {
		t.Fatal("invalid local model metadata")
	}
	chatModel, err := modelkit.NewModelKit(logger.Logger).GetChatModel(context.Background(), metadata)
	if err != nil {
		t.Fatal("unable to initialize configured model")
	}
	kb := &domain.KnowledgeBase{ID: uuid.NewString(), Name: "temporary live follow-up test", DatasetID: uuid.NewString()}
	if err := db.Create(kb).Error; err != nil {
		t.Fatal(err)
	}
	repo := pg.NewConversationRepository(db, logger)
	ragService := &contextRAG{}
	u := &LLMUsecase{conversationRepo: repo, kbRepo: pg.NewKnowledgeBaseRepository(db, &config.Config{}, logger, ragService), nodeRepo: pg.NewNodeRepository(db, logger), rag: ragService, logger: logger, promptRepo: pg.NewPromptRepo(db, logger)}
	for _, question := range []string{"上述第二条是什么？只回答条目名称。", "把刚才列出的三个步骤整理成表格。"} {
		source := seedChatHistory(t, db, kb.ID)
		if err := repo.CreateConversationMessage(context.Background(), &domain.ConversationMessage{ID: uuid.NewString(), ConversationID: source.ID, KBID: kb.ID, AppID: source.AppID, Role: schema.User, Content: question}, nil); err != nil {
			t.Fatal(err)
		}
		messages, _, err := u.BuildConversationMessageWithRAG(context.Background(), source.ID, kb.ID, nil, nil, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		var answer strings.Builder
		err = u.ChatWithAgent(ctx, chatModel, messages, &schema.TokenUsage{}, func(_ context.Context, kind, chunk string) error {
			if kind == "data" {
				answer.WriteString(chunk)
			}
			return nil
		})
		cancel()
		if err != nil {
			detail := err.Error()
			if modelInfo.APIKey != "" {
				detail = strings.ReplaceAll(detail, modelInfo.APIKey, "[redacted]")
			}
			t.Fatalf("configured model request failed: %s", detail)
		}
		if !strings.Contains(answer.String(), "测试") {
			t.Fatalf("model did not resolve preceding second item: %s", answer.String())
		}
		if strings.Contains(question, "表格") && (!strings.Contains(answer.String(), "|") || !strings.Contains(answer.String(), "备份") || !strings.Contains(answer.String(), "恢复")) {
			t.Fatalf("model did not transform previous answer: %s", answer.String())
		}
		t.Logf("%s -> %s", question, answer.String())
	}
}

func seedChatHistory(t *testing.T, db *store.DB, kbID string) *domain.Conversation {
	t.Helper()
	conversation := &domain.Conversation{ID: uuid.NewString(), Nonce: uuid.NewString(), KBID: kbID, AppID: uuid.NewString(), Subject: "备份方案", Info: domain.ConversationInfo{UserInfo: domain.UserInfo{UserID: uuid.NewString()}}}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	firstID := uuid.NewString()
	for i, msg := range []*domain.ConversationMessage{
		{ID: firstID, Role: schema.User, Content: "列出三个步骤"},
		{ID: uuid.NewString(), Role: schema.Assistant, Content: "1. 备份\n2. 测试\n3. 恢复", ParentID: firstID},
	} {
		msg.ConversationID, msg.KBID, msg.AppID = conversation.ID, kbID, conversation.AppID
		msg.CreatedAt = time.Now().Add(time.Duration(i-10) * time.Second)
		if err := db.Create(msg).Error; err != nil {
			t.Fatal(err)
		}
	}
	return conversation
}

func TestResumeConversationPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	repo := pg.NewConversationRepository(db, logger)
	u := &ConversationUsecase{repo: repo}
	ctx := context.Background()
	source := seedChatHistory(t, db, uuid.NewString())
	for _, creds := range [][2]string{{source.Nonce, ""}, {"", source.Info.UserInfo.UserID}} {
		session, err := u.ResumeShareConversation(ctx, source.KBID, source.ID, creds[0], creds[1], "test")
		if err != nil || session.Forked || session.ConversationID != source.ID || session.Nonce != source.Nonce {
			t.Fatalf("owner resume failed: %#v %v", session, err)
		}
	}
	if err := u.ValidateChatConversation(ctx, uuid.NewString(), source.ID, source.Nonce); err == nil {
		t.Fatal("cross-KB nonce accepted")
	}
	if err := u.ValidateChatConversation(ctx, source.KBID, source.ID, "wrong"); err == nil {
		t.Fatal("wrong nonce accepted")
	}
	if _, err := u.ResumeShareConversation(ctx, uuid.NewString(), source.ID, source.Nonce, "", "test"); err == nil {
		t.Fatal("cross-KB history copied")
	}
	for _, viewer := range []string{"", uuid.NewString()} {
		session, err := u.ResumeShareConversation(ctx, source.KBID, source.ID, "wrong", viewer, "test")
		if err != nil || !session.Forked || session.ConversationID == source.ID || session.Nonce == source.Nonce {
			t.Fatalf("shared conversation not isolated: %#v %v", session, err)
		}
		messages, err := repo.GetConversationMessagesByID(ctx, session.ConversationID)
		if err != nil || len(messages) != 2 || messages[1].ParentID != messages[0].ID || messages[1].Content != "1. 备份\n2. 测试\n3. 恢复" {
			t.Fatal("fork lost ordered history or parent links")
		}
		if err := u.ValidateChatConversation(ctx, source.KBID, session.ConversationID, session.Nonce); err != nil {
			t.Fatal(err)
		}
		followup := &domain.ConversationMessage{ID: uuid.NewString(), ConversationID: session.ConversationID, KBID: source.KBID, AppID: source.AppID, Role: schema.User, Content: "解释第二条"}
		if err := repo.CreateConversationMessage(ctx, followup, nil); err != nil {
			t.Fatal(err)
		}
		original, _ := repo.GetConversationMessagesByID(ctx, source.ID)
		if len(original) != 2 {
			t.Fatal("shared follow-up modified original")
		}
	}
}

type contextRAG struct {
	rag.RAGService
	rewritten string
	request   *rag.QueryRecordsRequest
}

func (r *contextRAG) QueryRecords(_ context.Context, req *rag.QueryRecordsRequest) (string, []*domain.NodeContentChunk, error) {
	r.request = req
	return r.rewritten, nil, nil
}

func (r *contextRAG) ListKnowledgeBases(context.Context) ([]rag.Dataset, error) {
	return []rag.Dataset{{ID: "test-dataset-1"}, {ID: "test-dataset-2"}}, nil
}

func TestFollowupContextReachesRetrievalAndModelPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	cfg := &config.Config{}
	kb := &domain.KnowledgeBase{ID: uuid.NewString(), Name: "temporary context test", DatasetID: uuid.NewString()}
	if err := db.Create(kb).Error; err != nil {
		t.Fatal(err)
	}
	repo := pg.NewConversationRepository(db, logger)
	for _, compatible := range []bool{false, true} {
		for _, question := range []string{"解释第二条", "把刚才的回答整理成表格", "上述方案适用于哪个阶段"} {
			t.Run(question+map[bool]string{false: "/web", true: "/compatible"}[compatible], func(t *testing.T) {
				source := seedChatHistory(t, db, kb.ID)
				var history []*schema.Message
				if compatible {
					history = []*schema.Message{schema.SystemMessage("用简洁中文回答"), schema.UserMessage("列出三个步骤"), schema.AssistantMessage("1. 备份\n2. 测试\n3. 恢复", nil)}
					if err := db.Where("conversation_id = ?", source.ID).Delete(&domain.ConversationMessage{}).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := repo.CreateConversationMessage(context.Background(), &domain.ConversationMessage{ID: uuid.NewString(), ConversationID: source.ID, KBID: kb.ID, AppID: source.AppID, Role: schema.User, Content: question}, nil); err != nil {
					t.Fatal(err)
				}
				ragService := &contextRAG{rewritten: "安全测试数据备份与恢复流程"}
				u := &LLMUsecase{conversationRepo: repo, kbRepo: pg.NewKnowledgeBaseRepository(db, cfg, logger, ragService), nodeRepo: pg.NewNodeRepository(db, logger), rag: ragService, logger: logger, promptRepo: pg.NewPromptRepo(db, logger)}
				messages, _, err := u.BuildConversationMessageWithRAG(context.Background(), source.ID, kb.ID, nil, nil, "自定义知识库规则", history)
				if err != nil {
					t.Fatal(err)
				}
				if len(messages) != 4 || messages[1].Role != schema.User || messages[2].Role != schema.Assistant {
					t.Fatalf("model lost history: %#v", messages)
				}
				if ragService.request.Query != question || len(ragService.request.HistoryMsgs) != 2 || ragService.request.HistoryMsgs[1].Content != messages[2].Content {
					t.Fatal("retrieval lost follow-up history")
				}
				current := messages[3].Content
				if !strings.Contains(current, "<question>\n"+question+"\n</question>") || !strings.Contains(current, "<retrieval_query>\n"+ragService.rewritten) {
					t.Fatal("rewritten query replaced user intent")
				}
				if !strings.Contains(messages[0].Content, domain.ConversationContextPrompt) || !strings.Contains(messages[0].Content, "自定义知识库规则") {
					t.Fatal("custom prompt lost context rules")
				}
				if compatible && !strings.Contains(messages[0].Content, "用简洁中文回答") {
					t.Fatal("caller system instructions discarded")
				}
				ragService.rewritten = ""
				messages, _, err = u.BuildConversationMessageWithRAG(context.Background(), source.ID, kb.ID, nil, nil, "自定义知识库规则", history)
				if err != nil || !strings.Contains(messages[len(messages)-1].Content, "<retrieval_query>\n"+question) {
					t.Fatal("empty rewrite erased question")
				}
			})
		}
	}
}
