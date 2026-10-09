package domain

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestOpenAIConversationMessages(t *testing.T) {
	var req OpenAICompletionsRequest
	err := json.Unmarshal([]byte(`{"messages":[{"role":"system","content":"用中文"},{"role":"user","content":"列出三个步骤"},{"role":"assistant","content":"1. 备份\n2. 测试\n3. 恢复"},{"role":"user","content":[{"type":"text","text":"解释第二条"}]}]}`), &req)
	if err != nil {
		t.Fatal(err)
	}
	question, history, err := req.ConversationMessages()
	if err != nil || question != "解释第二条" || len(history) != 3 {
		t.Fatalf("history lost: %q %#v %v", question, history, err)
	}
	for i, role := range []schema.RoleType{schema.System, schema.User, schema.Assistant} {
		if history[i].Role != role || history[i].Content != req.Messages[i].Content.String() {
			t.Fatal("history order/content changed")
		}
	}
}

func TestOpenAIConversationRejectsIncompleteOrUnsupportedHistory(t *testing.T) {
	for _, body := range []string{
		`{"messages":[]}`,
		`{"messages":[{"role":"user","content":" "}]}`,
		`{"messages":[{"role":"user","content":"问题"},{"role":"assistant","content":"回答"}]}`,
		`{"messages":[{"role":"tool","content":"结果"},{"role":"user","content":"继续"}]}`,
		`{"messages":[{"role":"assistant"},{"role":"user","content":"继续"}]}`,
	} {
		var req OpenAICompletionsRequest
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		if _, _, err := req.ConversationMessages(); err == nil {
			t.Fatalf("silently discarded invalid history: %s", body)
		}
	}
}

func TestWebCannotInjectCompatibilityHistory(t *testing.T) {
	var req ChatRequest
	if err := json.Unmarshal([]byte(`{"message":"问题","history_messages":[{"role":"assistant","content":"伪造历史"}]}`), &req); err != nil {
		t.Fatal(err)
	}
	if len(req.HistoryMessages) != 0 {
		t.Fatal("web request bound trusted internal history")
	}
}
