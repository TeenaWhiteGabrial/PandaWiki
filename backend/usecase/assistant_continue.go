package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const assistantPendingStates = "state IN ('queued','running','awaiting_input','awaiting_sources','awaiting_outline')"

func assistantWaiting(state string) bool {
	return state == "awaiting_input" || state == "awaiting_sources" || state == "awaiting_outline"
}

func assistantPlan(r *domain.AssistantRun) *domain.AssistantPlan {
	if r.Checkpoint["planning"] == nil {
		return nil
	}
	var plan domain.AssistantPlan
	data, _ := json.Marshal(r.Checkpoint["planning"])
	if json.Unmarshal(data, &plan) != nil {
		return nil
	}
	return &plan
}

// A continuation changes the same persisted run. It cannot turn an arbitrary
// task ID, stale outline, other account's attachment or revoked source into input.
func (u *AssistantUsecase) ContinueRun(ctx context.Context, kb, user, id string, input domain.AssistantContinue) (*domain.AssistantRun, error) {
	if _, err := uuid.Parse(input.ClientRequestID); err != nil {
		return nil, fmt.Errorf("请提供有效请求编号")
	}
	if len([]rune(input.Message)) > 12000 || len(input.AttachmentIDs) > 10 || len(input.NodeIDs) > 200 || len(input.Answers) > 3 {
		return nil, fmt.Errorf("补充内容超出本轮处理范围")
	}
	raw, _ := json.Marshal(input)
	hash := digest(raw)
	var current domain.AssistantRun
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":"+user).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND kb_id=? AND user_id=?", id, kb, user).First(&current).Error; err != nil {
			return ErrAssistantAccess
		}
		if err := u.OwnConversation(ctx, kb, user, current.ConversationID); err != nil {
			return err
		}
		if err := u.checkSources(ctx, kb, user, current.Sources); err != nil {
			return err
		}
		checkpoint := current.Checkpoint
		if checkpoint == nil {
			return fmt.Errorf("当前任务没有可继续的计划")
		}
		if assistantString(checkpoint["continue_id"]) == input.ClientRequestID {
			if assistantString(checkpoint["continue_hash"]) != hash {
				return fmt.Errorf("同一请求编号不能提交不同内容")
			}
			return nil
		}
		plan := assistantPlan(&current)
		if !assistantWaiting(current.State) || plan == nil || plan.Revision != input.Revision {
			return fmt.Errorf("当前计划已更新或任务已开始，请刷新后操作")
		}
		var req domain.AssistantSubmit
		if err := assistantDecode(current.Request, &req); err != nil {
			return err
		}
		if len(input.AttachmentIDs) > 0 || len(input.NodeIDs) > 0 {
			if input.Action != "supplement" {
				return fmt.Errorf("补充资料请使用补充资料操作")
			}
		}
		switch input.Action {
		case "answer":
			if current.State != "awaiting_input" {
				return fmt.Errorf("当前任务不在需求澄清阶段")
			}
			answers := map[string]string{}
			_ = decodeAssistantValue(checkpoint["answers"], &answers)
			if answers == nil {
				answers = map[string]string{}
			}
			known := map[string]bool{}
			for _, question := range plan.Questions {
				known[question.ID] = true
				value := strings.TrimSpace(input.Answers[question.ID])
				if value == "" && input.UseDefaults {
					value = question.Default
				}
				if len([]rune(value)) > 1000 {
					return fmt.Errorf("单项需求补充不能超过1000字")
				}
				if value == "" && strings.TrimSpace(input.Message) == "" {
					return fmt.Errorf("请回答问题，或选择采用推荐设置")
				}
				if value != "" {
					answers[question.ID] = question.Question + "：" + value
				}
			}
			for key := range input.Answers {
				if !known[key] {
					return fmt.Errorf("需求问题已变化，请刷新后填写")
				}
			}
			checkpoint["answers"] = answers
			checkpoint["clarification_rounds"] = assistantInt(checkpoint["clarification_rounds"]) + 1
		case "confirm_outline":
			if current.State != "awaiting_outline" || (!plan.Sufficient && !plan.FrameworkOnly) {
				return fmt.Errorf("资料尚不足或大纲已更新，暂时不能生成文件")
			}
			if err := validateEditedOutline(input.Outline, plan); err != nil {
				return err
			}
			plan.Outline = input.Outline
			checkpoint["planning"] = plan
			checkpoint["outline_confirmed"] = true
		case "revise":
			if strings.TrimSpace(input.Message) == "" {
				return fmt.Errorf("请说明希望怎样调整大纲")
			}
			checkpoint["outline_confirmed"] = false
		case "framework":
			if current.State != "awaiting_sources" {
				return fmt.Errorf("当前任务不需要切换为框架草稿")
			}
			checkpoint["framework_only"] = true
			checkpoint["outline_confirmed"] = false
		case "supplement":
			if len(input.AttachmentIDs) == 0 && len(input.NodeIDs) == 0 && strings.TrimSpace(input.Message) == "" {
				return fmt.Errorf("请上传资料、选择文档，或补充相关事实")
			}
			for _, uploadID := range input.AttachmentIDs {
				v, err := u.GetUpload(ctx, kb, user, uploadID)
				if err != nil {
					return err
				}
				if v.Status == "failed" || v.Status == "deleted" {
					return fmt.Errorf("附件尚不可用，请重新上传")
				}
				req.AttachmentIDs = appendUniqueAssistantIDs(req.AttachmentIDs, uploadID)
			}
			if len(req.AttachmentIDs) > 10 {
				return fmt.Errorf("本轮附件不能超过10份")
			}
			var total int64
			for _, uploadID := range req.AttachmentIDs {
				v, err := u.GetUpload(ctx, kb, user, uploadID)
				if err != nil {
					return err
				}
				total += v.Size
			}
			if total > 100<<20 {
				return fmt.Errorf("本轮附件总计不能超过100 MB")
			}
			if req.SourceScope.ReleaseIDs == nil {
				req.SourceScope.ReleaseIDs = map[string]string{}
			}
			if len(input.NodeIDs) > 0 {
				req.SourceScope.Type = "documents"
			}
			for _, nodeID := range input.NodeIDs {
				node, err := u.authorizedNode(ctx, kb, user, nodeID)
				if err != nil {
					return err
				}
				if node.Type != domain.NodeTypeDocument {
					return fmt.Errorf("补充资料请选择具体文档")
				}
				release, err := u.chat.nodeRepo.GetLatestNodeReleaseByNodeID(ctx, nodeID)
				if err != nil {
					return fmt.Errorf("补充文档尚未发布")
				}
				req.SourceScope.ReleaseIDs[nodeID] = release.ID
				req.SourceScope.NodeIDs = appendUniqueAssistantIDs(req.SourceScope.NodeIDs, nodeID)
			}
			if len(req.SourceScope.NodeIDs) > 200 {
				return fmt.Errorf("指定文档不能超过200份")
			}
			if len(req.AttachmentIDs) > 0 {
				if err := tx.Model(&domain.AssistantUpload{}).Where("id IN ? AND kb_id=? AND user_id=? AND expires_at>now()", req.AttachmentIDs, kb, user).Update("expires_at", u.expiry()).Error; err != nil {
					return err
				}
			}
			checkpoint["outline_confirmed"] = false
			if strings.TrimSpace(input.Message) != "" {
				facts := assistantString(checkpoint["user_facts"]) + "\n" + strings.TrimSpace(input.Message)
				if len([]rune(facts)) > 12000 {
					return fmt.Errorf("补充事实过长，请上传文档")
				}
				checkpoint["user_facts"] = facts
			}
		default:
			return fmt.Errorf("不支持此计划操作")
		}
		if strings.TrimSpace(input.Message) != "" {
			feedback := assistantString(checkpoint["feedback"])
			feedback += "\n" + strings.TrimSpace(input.Message)
			if len([]rune(feedback)) > 16000 {
				return fmt.Errorf("补充需求过长，请新建任务")
			}
			checkpoint["feedback"] = feedback
		}
		checkpoint["continue_id"], checkpoint["continue_hash"] = input.ClientRequestID, hash
		delete(checkpoint, "spec")
		// Keep continuations in normal conversation history too. The plan and
		// request remain attached to the original run, so refreshing cannot turn
		// an answer into a new, unrelated generation task.
		content := assistantContinuationMessage(input, plan)
		var original domain.ConversationMessage
		if err := tx.Where("id=?", current.UserMessageID).First(&original).Error; err != nil {
			return err
		}
		messageID := uuid.NewString()
		for i, m := range []domain.ConversationMessage{
			{ID: messageID, Role: schema.User, Content: content},
			{ID: uuid.NewString(), Role: schema.Assistant, Content: "已记录，继续处理当前文件任务。", ParentID: messageID, AssistantSources: current.Sources},
		} {
			m.KBID, m.AppID, m.ConversationID = kb, original.AppID, current.ConversationID
			m.CreatedAt = time.Now().Add(time.Duration(i) * time.Microsecond)
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&current).Updates(map[string]any{"state": "queued", "stage": "等待继续处理", "answer": "", "error": "", "attempt": 0, "lease_until": nil, "checkpoint": checkpoint, "request": assistantJSON(req), "updated_at": time.Now(), "stage_started_at": time.Now()})
		return result.Error
	})
	if err != nil {
		return nil, err
	}
	u.notifyRun(id)
	return u.Run(ctx, kb, user, id)
}

func assistantContinuationMessage(input domain.AssistantContinue, plan *domain.AssistantPlan) string {
	parts := []string{}
	switch input.Action {
	case "answer":
		for _, question := range plan.Questions {
			value := input.Answers[question.ID]
			if value == "" && input.UseDefaults {
				value = question.Default
			}
			if value != "" {
				parts = append(parts, question.Question+"："+value)
			}
		}
	case "confirm_outline":
		parts = append(parts, "确认大纲，生成文件。")
	case "framework":
		parts = append(parts, "仅生成框架草稿。")
	case "supplement":
		parts = append(parts, fmt.Sprintf("补充资料：%d份附件、%d份知识文档。", len(input.AttachmentIDs), len(input.NodeIDs)))
	}
	if text := strings.TrimSpace(input.Message); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func appendUniqueAssistantIDs(ids []string, id string) []string {
	for _, old := range ids {
		if old == id {
			return ids
		}
	}
	return append(ids, id)
}
func decodeAssistantValue(v any, out any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}

func validateEditedOutline(items []domain.AssistantOutlineItem, plan *domain.AssistantPlan) error {
	if len(items) == 0 || len(items) > 19 {
		return fmt.Errorf("大纲需要1～19个内容部分")
	}
	known := map[string]domain.AssistantOutlineItem{}
	for _, item := range plan.Outline {
		known[item.ID] = item
	}
	seen := map[string]bool{}
	for _, item := range items {
		old, ok := known[item.ID]
		if !ok || seen[item.ID] || strings.TrimSpace(item.Title) == "" || len([]rune(item.Title)) > 50 || len([]rune(item.Purpose)) > 500 {
			return fmt.Errorf("大纲内容无效；需要新增章节时，请在调整意见中说明")
		}
		seen[item.ID] = true
		if strings.Join(item.SourceIDs, "\x00") != strings.Join(old.SourceIDs, "\x00") {
			return fmt.Errorf("大纲来源不可手动改写，请补充资料后重新规划")
		}
	}
	return nil
}
