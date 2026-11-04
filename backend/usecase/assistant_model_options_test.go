package usecase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func TestAssistantFileInstructModelRouting(t *testing.T) {
	for _, tc := range []struct {
		provider             domain.ModelProvider
		model, endpoint, key string
	}{
		{domain.ModelProviderCompatibleCloud, "Qwen3.6-27B", "http://127.0.0.1:8001/v1", "chat_template_kwargs"},
		{domain.ModelProviderCompatibleCloud, "Qwen/Qwen3.6-27B", "https://dashscope.aliyuncs.com/compatible-mode/v1", "enable_thinking"},
		{domain.ModelProviderCompatibleCloud, "different-model", "http://127.0.0.1/v1", ""},
		{"OpenAI", "Qwen3.6-27B", "https://example.com/v1", ""},
	} {
		fields := assistantFileInstructFields(&domain.Model{Provider: tc.provider, Model: tc.model, BaseURL: tc.endpoint})
		if tc.key == "" {
			if len(fields) != 0 {
				t.Fatal("unknown model adapted")
			}
		} else if _, ok := fields[tc.key]; !ok {
			t.Fatal("incorrect provider field", fields)
		}
	}
}

func TestAssistantFileInstructCompatibility(t *testing.T) {
	requests := []map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"unknown parameter chat_template_kwargs","type":"invalid_request_error"}}`))
			return
		}
		w.Write([]byte(`{"id":"fixture","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()
	model, e := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{APIKey: "synthetic", BaseURL: server.URL + "/v1", Model: "Qwen3.6-27B"})
	if e != nil {
		t.Fatal(e)
	}
	u := &AssistantUsecase{logger: log.NewLogger(&config.Config{})}
	r := &domain.AssistantRun{ID: "fixture", Checkpoint: domain.JSONB{"file_instruct_fields": map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}}}}
	if _, e = u.assistantFileGenerate(context.Background(), r, model, []*schema.Message{schema.UserMessage("fixture")}, 100, true); e != nil {
		t.Fatal(e)
	}
	if len(requests) != 2 || requests[0]["chat_template_kwargs"] == nil || requests[1]["chat_template_kwargs"] != nil {
		t.Fatal("unsupported option did not fall back exactly once", requests)
	}
	if requests[0]["temperature"] != 0.7 || requests[0]["top_p"] != 0.8 {
		t.Fatal("instruct sampling parameters missing")
	}
	if _, e = u.assistantFileGenerate(context.Background(), r, model, []*schema.Message{schema.UserMessage("fixture")}, 100, true); e != nil {
		t.Fatal(e)
	}
	if len(requests) != 3 || requests[2]["chat_template_kwargs"] != nil {
		t.Fatal("repeated unsupported option")
	}
	delete(r.Checkpoint, "file_instruct_disabled")
	if _, e = u.assistantFileGenerate(context.Background(), r, model, []*schema.Message{schema.UserMessage("ordinary question")}, 100, false); e != nil {
		t.Fatal(e)
	}
	if requests[3]["chat_template_kwargs"] != nil {
		t.Fatal("normal Q&A mode changed")
	}
}
