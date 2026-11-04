package usecase

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	aclopenai "github.com/cloudwego/eino-ext/libs/acl/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Qwen3.6 documents a hard API switch for short instruct tasks. Limit this
// adaptation to the known model family over the configurable OpenAI transport;
// do not guess controls for arbitrary models or alter normal Q&A settings.
// https://huggingface.co/Qwen/Qwen3.6-27B#instruct-or-non-thinking-mode
func assistantFileInstructFields(model *domain.Model) map[string]any {
	if model == nil || model.Provider != domain.ModelProviderCompatibleCloud || !regexp.MustCompile(`(?i)(^|/)qwen3\.6-`).MatchString(model.Model) {
		return nil
	}
	u, e := url.Parse(model.BaseURL)
	if e != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "dashscope.aliyuncs.com" || strings.HasSuffix(host, ".dashscope.aliyuncs.com") || host == "dashscope-intl.aliyuncs.com" || host == "dashscope-us.aliyuncs.com" {
		return map[string]any{"enable_thinking": false}
	}
	// Compatible gateways may forward either the serving-engine field or the
	// cloud-style field. Both express the same documented switch. Strict
	// gateways still use the explicit unsupported-parameter fallback below.
	// A number of compatible gateways expose the unified reasoning control
	// instead of forwarding native model fields. It disables reasoning; merely
	// excluding reasoning text would still consume time and output tokens.
	// https://openrouter.ai/docs/guides/best-practices/reasoning-tokens
	return map[string]any{"enable_thinking": false, "chat_template_kwargs": map[string]any{"enable_thinking": false}, "reasoning": map[string]any{"enabled": false}}
}

func (u *AssistantUsecase) assistantFileGenerate(ctx context.Context, r *domain.AssistantRun, cm einomodel.BaseChatModel, messages []*schema.Message, budget int, instruct bool) (*schema.Message, error) {
	options := []einomodel.Option{einomodel.WithMaxTokens(budget)}
	var fields map[string]any
	_ = decodeAssistantValue(r.Checkpoint["file_instruct_fields"], &fields)
	disabled, _ := r.Checkpoint["file_instruct_disabled"].(bool)
	if !instruct || disabled || len(fields) == 0 {
		return cm.Generate(ctx, messages, options...)
	}
	response, err := cm.Generate(ctx, messages, append(options, einomodel.WithTemperature(0.7), einomodel.WithTopP(0.8), aclopenai.WithExtraFields(fields))...)
	if err == nil {
		return response, nil
	}
	message := strings.ToLower(err.Error())
	// Some compatible gateways reject extension fields. Fall back once only
	// for that explicit parameter error; auth, quota and network errors retain
	// their normal recovery policy and never trigger a second blind request.
	if (strings.Contains(message, "400") || strings.Contains(message, "422")) && (strings.Contains(message, "chat_template_kwargs") || strings.Contains(message, "enable_thinking") || strings.Contains(message, "reasoning")) {
		r.Checkpoint["file_instruct_disabled"] = true
		u.logger.Warn("assistant instruct parameter unsupported; using configured default", log.String("run_id", r.ID))
		return cm.Generate(ctx, messages, options...)
	}
	return response, err
}
