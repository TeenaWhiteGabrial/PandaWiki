package usecase

import (
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/chaitin/panda-wiki/domain"
)

const chatTraceTextLimit = 2400
const chatTraceCollectionLimit = 50
const chatTraceRetention = 30 * 24 * time.Hour

type chatTraceRecorder struct {
	trace *domain.ChatTrace
}

func newChatTrace(traceID, kbID, conversationID, userMessageID, assistantMessageID, question string, started time.Time) *chatTraceRecorder {
	return &chatTraceRecorder{trace: &domain.ChatTrace{
		TraceID: traceID, KBID: kbID, ConversationID: conversationID,
		UserMessageID: userMessageID, AssistantMessageID: assistantMessageID,
		Question: boundedTraceText(question), Status: "in_progress", StartedAt: started,
		ExpiresAt: started.Add(chatTraceRetention),
		Steps:     make(domain.ChatTraceSteps, 0, 10),
	}}
}

func (r *chatTraceRecorder) add(name, status string, input, output map[string]any, err error) {
	now := time.Now()
	step := domain.ChatTraceStep{Name: name, Status: status, StartedAt: now, FinishedAt: &now, Input: boundTraceMap(input), Output: boundTraceMap(output)}
	if err != nil {
		step.Error = boundedTraceText(err.Error())
	}
	r.trace.Steps = append(r.trace.Steps, step)
}

func (r *chatTraceRecorder) finish(status string) {
	now := time.Now()
	r.trace.Status = status
	r.trace.FinishedAt = &now
}

func traceMessagesForDiagnostics(messages []*schema.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message != nil {
			result = append(result, map[string]any{"role": string(message.Role), "content": message.Content})
		}
	}
	return result
}

func boundedTraceText(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= chatTraceTextLimit {
		return value
	}
	return string(runes[:chatTraceTextLimit]) + "…[已截断]"
}

func boundTraceMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		switch v := item.(type) {
		case string:
			result[key] = boundedTraceText(v)
		case []string:
			limit := min(len(v), chatTraceCollectionLimit)
			items := make([]string, limit)
			for i, text := range v[:limit] {
				items[i] = boundedTraceText(text)
			}
			result[key] = items
		case []map[string]any:
			limit := min(len(v), chatTraceCollectionLimit)
			items := make([]map[string]any, limit)
			for i, entry := range v[:limit] {
				items[i] = boundTraceMap(entry)
			}
			if len(v) > limit {
				result[key+"_truncated"] = true
				result[key+"_total_count"] = len(v)
			}
			result[key] = items
		default:
			result[key] = item
		}
	}
	return result
}
