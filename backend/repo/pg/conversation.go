package pg

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
	"github.com/chaitin/panda-wiki/utils"
)

type ConversationRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewConversationRepository(db *pg.DB, logger *log.Logger) *ConversationRepository {
	return &ConversationRepository{db: db, logger: logger.WithModule("repo.pg.conversation")}
}

func (r *ConversationRepository) CreateConversationMessage(ctx context.Context, conversationMessage *domain.ConversationMessage, references []*domain.ConversationReference) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(conversationMessage).Error; err != nil {
			return err
		}
		if len(references) > 0 {
			return tx.Create(references).Error
		}
		return nil
	})
}

func (r *ConversationRepository) CreateConversation(ctx context.Context, conversation *domain.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

func (r *ConversationRepository) GetConversationList(ctx context.Context, request *domain.ConversationListReq) ([]*domain.ConversationListItem, uint64, error) {
	conversations := []*domain.ConversationListItem{}
	query := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Where("conversations.kb_id = ?", request.KBID)

	if request.AppID != nil && *request.AppID != "" {
		query = query.Where("conversations.app_id = ?", *request.AppID)
	}
	if request.Subject != nil && *request.Subject != "" {
		query = query.Where("conversations.subject like ?", "%"+*request.Subject+"%")
	}
	if request.RemoteIP != nil && *request.RemoteIP != "" {
		query = query.Where("conversations.remote_ip like ?", "%"+*request.RemoteIP+"%")
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Joins("left join apps on conversations.app_id = apps.id").
		Select("conversations.*, apps.name as app_name, apps.type as app_type").
		Offset(request.Offset()).
		Limit(request.Limit()).
		Order("conversations.created_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, 0, err
	}
	return conversations, uint64(count), nil
}

func (r *ConversationRepository) GetConversationDetail(ctx context.Context, kbID, conversationID string) (*domain.ConversationDetailResp, error) {
	conversation := &domain.ConversationDetailResp{}
	query := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Where("id = ?", conversationID)
	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}
	if err := query.
		First(conversation).Error; err != nil {
		return nil, err
	}
	return conversation, nil
}

func (r *ConversationRepository) GetConversationReferences(ctx context.Context, conversationID string) ([]*domain.ConversationReference, error) {
	references := []*domain.ConversationReference{}
	if err := r.db.WithContext(ctx).
		Model(&domain.ConversationReference{}).
		Where("conversation_id = ?", conversationID).
		Find(&references).Error; err != nil {
		return nil, err
	}
	return references, nil
}

func (r *ConversationRepository) SaveChatTrace(ctx context.Context, trace *domain.ChatTrace) error {
	return r.db.WithContext(ctx).Save(trace).Error
}

func (r *ConversationRepository) GetChatTraces(ctx context.Context, kbID, conversationID string) ([]*domain.ChatTrace, error) {
	traces := make([]*domain.ChatTrace, 0)
	err := r.db.WithContext(ctx).
		Where("kb_id = ? AND conversation_id = ?", kbID, conversationID).
		Order("started_at ASC").Find(&traces).Error
	return traces, err
}

func (r *ConversationRepository) DeleteExpiredChatTraces(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&domain.ChatTrace{})
	return result.RowsAffected, result.Error
}

func (r *ConversationRepository) GetConversationMessagesByID(ctx context.Context, conversationID string) ([]*domain.ConversationMessage, error) {
	messages := []*domain.ConversationMessage{}
	if err := r.db.WithContext(ctx).
		Model(&domain.ConversationMessage{}).
		Where("conversation_id = ?", conversationID).
		Order("created_at asc").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *ConversationRepository) ValidateConversationNonce(ctx context.Context, conversationID, nonce string) error {
	conversation := &domain.Conversation{}
	if err := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Where("id = ?", conversationID).
		Where("nonce = ?", nonce).
		First(&conversation).Error; err != nil {
		return err
	}
	return nil
}

func (r *ConversationRepository) GetChatConversation(ctx context.Context, kbID, id string) (*domain.Conversation, error) {
	conversation := &domain.Conversation{}
	err := r.db.WithContext(ctx).Where("kb_id = ? AND id = ?", kbID, id).First(conversation).Error
	return conversation, err
}

func (r *ConversationRepository) GetShareConversationHistory(ctx context.Context, kbID, userID string, req *domain.ShareConversationHistoryRequest) ([]*domain.ShareConversationHistoryItem, uint64, error) {
	items := []*domain.ShareConversationHistoryItem{}
	query := r.db.WithContext(ctx).Model(&domain.Conversation{}).Where("conversations.kb_id = ?", kbID)
	if userID != "" {
		query = query.Where("conversations.info->'user_info'->>'user_id' = ?", userID)
	} else {
		if len(req.Sessions) == 0 {
			return items, 0, nil
		}
		credentials := r.db.Where("1 = 0")
		for _, session := range req.Sessions {
			if session.ID != "" && session.Nonce != "" {
				credentials = credentials.Or("conversations.id = ? AND conversations.nonce = ?", session.ID, session.Nonce)
			}
		}
		query = query.Where("COALESCE(conversations.info->'user_info'->>'user_id', '') = ''").Where(credentials)
	}
	if req.Query != "" {
		// Treat percent and underscore as literal characters, not LIKE wildcards.
		query = query.Where("strpos(lower(conversations.subject), lower(?)) > 0", req.Query)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Select(`conversations.id, conversations.subject, conversations.created_at,
		COALESCE((SELECT MAX(m.created_at) FROM conversation_messages m
		WHERE m.conversation_id = conversations.id AND m.kb_id = conversations.kb_id), conversations.created_at) AS updated_at`).
		Order("updated_at DESC, conversations.id DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&items).Error
	return items, uint64(total), err
}

// Shared histories are copied atomically; the viewer never receives the original nonce.
func (r *ConversationRepository) ForkConversation(ctx context.Context, source *domain.Conversation, userID, remoteIP string) (*domain.Conversation, error) {
	fork := &domain.Conversation{
		ID: uuid.NewString(), Nonce: uuid.NewString(), KBID: source.KBID, AppID: source.AppID,
		Subject: source.Subject, RemoteIP: remoteIP, CreatedAt: time.Now(),
		Info: domain.ConversationInfo{UserInfo: domain.UserInfo{UserID: userID}},
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var messages []domain.ConversationMessage
		if err := tx.Where("conversation_id = ? AND kb_id = ?", source.ID, source.KBID).Order("created_at asc").Find(&messages).Error; err != nil {
			return err
		}
		if err := tx.Create(fork).Error; err != nil {
			return err
		}
		ids := make(map[string]string, len(messages))
		for _, msg := range messages {
			ids[msg.ID] = uuid.NewString()
		}
		for _, msg := range messages {
			if msg.Role != schema.User && msg.Role != schema.Assistant {
				continue
			}
			msg.ID, msg.ParentID = ids[msg.ID], ids[msg.ParentID]
			msg.ConversationID, msg.RemoteIP = fork.ID, remoteIP
			msg.PromptTokens, msg.CompletionTokens, msg.TotalTokens = 0, 0, 0
			msg.Info = domain.FeedBackInfo{}
			if err := tx.Create(&msg).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return fork, err
}

func (r *ConversationRepository) GetConversationDistribution(ctx context.Context, kbID string) ([]domain.ConversationDistribution, error) {
	var distribution []domain.ConversationDistribution
	if err := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Select("app_id", "COUNT(*) AS count").
		Where("kb_id = ?", kbID).
		Where("created_at > now() - interval '24h'").
		Group("app_id").
		Find(&distribution).Error; err != nil {
		return nil, err
	}
	return distribution, nil
}

func (r *ConversationRepository) GetConversationCount(ctx context.Context, kbID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Where("kb_id = ?", kbID).
		Where("created_at > now() - interval '24h'").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ConversationRepository) GetConversationMessagesDetailByID(ctx context.Context, messageId string) (*domain.ConversationMessage, error) {
	message := &domain.ConversationMessage{}
	if err := r.db.WithContext(ctx).
		Model(&domain.ConversationMessage{}).
		Where("id = ?", messageId).
		First(&message).Error; err != nil {
		return nil, err
	}
	return message, nil
}

func (r *ConversationRepository) GetConversationMessagesDetailByKbID(ctx context.Context, kbId, messageId string) (*domain.ConversationMessage, error) {
	message := &domain.ConversationMessage{}
	if err := r.db.WithContext(ctx).
		Model(&domain.ConversationMessage{}).
		Where("id = ?", messageId).
		Where("kb_id = ?", kbId).
		First(&message).Error; err != nil {
		return nil, err
	}
	return message, nil
}

// 更新反馈信息
func (r *ConversationRepository) UpdateMessageFeedback(ctx context.Context, feedback *domain.FeedbackRequest) error {
	// 更新字段
	feedbackInfo := domain.FeedBackInfo{
		Score:           feedback.Score,
		FeedbackType:    feedback.Type,
		FeedbackContent: feedback.FeedbackContent,
	}

	// 更新消息的反馈信息
	if err := r.db.WithContext(ctx).Model(&domain.ConversationMessage{}).
		Where("id = ?", feedback.MessageId).
		Update("info", feedbackInfo).Error; err != nil {
		return err
	}
	return nil
}

func (r *ConversationRepository) GetConversationFeedBackInfoByIDs(ctx context.Context, conversationIDs []string) (map[string]*domain.FeedBackInfo, error) {
	if len(conversationIDs) == 0 {
		return nil, nil
	}

	messages := []domain.ConversationMessage{}
	if err := r.db.WithContext(ctx).Model(&domain.ConversationMessage{}).
		Where("conversation_id IN (?)", conversationIDs).
		Where("info is not null AND info->>'score' != ?", "0").
		Where("role = ?", schema.Assistant).
		Order("created_at ASC").
		Select("conversation_id, info").Find(&messages).Error; err != nil {
		r.logger.Error("GetConversationFeedBackInfoByIDs failed, error:", log.Error(err))
		return nil, err
	}
	result := make(map[string]*domain.FeedBackInfo, 0)
	for _, message := range messages {
		result[message.ConversationID] = &message.Info
	}
	return result, nil
}

func (r *ConversationRepository) GetMessageFeedBackList(ctx context.Context, req *domain.MessageListReq) (int64, []*domain.ConversationMessageListItem, error) {
	// get feedback info -> user must feedback
	query := r.db.WithContext(ctx).Table("conversation_messages as cm").
		Joins("JOIN conversations ON conversations.id = cm.conversation_id").
		Where("conversations.kb_id = ?", req.KBID).
		Where("cm.info is not null AND cm.info->>'score' != ?", "0").
		Where("role = ?", schema.Assistant)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	r.logger.Debug("GetMessageFeedBackList count", log.Int64("count", count))

	query = r.db.WithContext(ctx).Table("conversation_messages as cm").
		Joins("LEFT JOIN LATERAL (SELECT content FROM conversation_messages WHERE conversation_id = cm.conversation_id AND role = 'user' AND created_at < cm.created_at ORDER BY created_at DESC LIMIT 1) u ON true").
		Joins("JOIN conversations ON conversations.id = cm.conversation_id").
		Joins("JOIN apps ON cm.app_id = apps.id").
		Where("conversations.kb_id = ?", req.KBID).
		Where("cm.info is not null AND cm.info->>'score' != ?", "0").
		Where("role = ?", schema.Assistant)

	var messageAnswers []*domain.ConversationMessageListItem

	if err := query.
		Select("cm.id", "cm.app_id", "apps.type as app_type", "u.content as question", "cm.content as answer", "conversations.info as conversation_info", "cm.app_id", "cm.conversation_id", "cm.remote_ip", "cm.info", "cm.created_at").
		Offset(req.Offset()).Limit(req.Limit()).Order("created_at DESC").
		Find(&messageAnswers).Error; err != nil {
		return 0, nil, err
	}

	if len(messageAnswers) == 0 {
		return 0, nil, nil
	}
	return count, messageAnswers, nil
}

func (r *ConversationRepository) GetConversationDistributionByHour(ctx context.Context, kbID string, startHour int64) (map[domain.AppType]int64, error) {
	counts := make(map[domain.AppType]int64)

	distributions := make([]domain.MapStrInt64, 0)
	if err := r.db.WithContext(ctx).Model(&domain.StatPageHour{}).
		Select("conversation_distribution").
		Where("kb_id = ?", kbID).
		Where("hour >= ? and hour < ?", utils.GetTimeHourOffset(-startHour), utils.GetTimeHourOffset(-24)).
		Pluck("conversation_distribution", &distributions).Error; err != nil {
		return nil, err
	}
	for i := range distributions {
		for k, v := range distributions[i] {
			appType, err := strconv.Atoi(k)
			if err != nil {
				continue
			}
			counts[domain.AppType(appType)] += v
		}
	}

	return counts, nil
}

func (r *ConversationRepository) GetConversationCountByAppType(ctx context.Context) (map[domain.AppType]int64, error) {
	type row struct {
		AppType int   `gorm:"column:app_type"`
		Count   int64 `gorm:"column:count"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Joins("JOIN apps ON conversations.app_id = apps.id").
		Select("apps.type as app_type, COUNT(*) as count").
		Group("apps.type").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[domain.AppType]int64)
	for _, t := range domain.AppTypes {
		result[t] = 0
	}
	for _, rrow := range rows {
		result[domain.AppType(rrow.AppType)] = rrow.Count
	}
	return result, nil
}
