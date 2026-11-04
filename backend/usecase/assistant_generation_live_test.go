package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/cloudwego/eino/schema"
)

// Opt-in configured-model smoke test. Model inputs contain synthetic facts
// only. All task records roll back; no knowledge retrieval or OSS operations.
func TestAssistantGenerationLiveModel(t *testing.T) {
	if os.Getenv("PANDA_WIKI_TEST_LIVE_LLM") != "1" {
		t.Skip("explicit configured-model opt-in required")
	}
	u, db, r := planningFixture(t)
	var setting domain.SystemSetting
	if e := db.Where("key=?", consts.SystemSettingModelMode).First(&setting).Error; e != nil {
		t.Skip("configured local model unavailable")
	}
	var mode domain.ModelModeSetting
	if json.Unmarshal(setting.Value, &mode) != nil {
		t.Fatal("local model setting invalid")
	}
	var info domain.Model
	if mode.Mode == consts.ModelSettingModeAuto {
		provider, e := (&ModelUsecase{}).getAutoModeProvider(mode)
		if e != nil {
			t.Fatal("model provider unavailable")
		}
		info = domain.Model{Provider: provider.Provider, Model: provider.Models.Chat, Type: domain.ModelTypeChat, BaseURL: getAutoModeModelBaseURL(provider, domain.ModelTypeChat), APIKey: provider.APIKey}
	} else if e := db.Where("type=? AND is_active=true", domain.ModelTypeChat).First(&info).Error; e != nil {
		t.Skip("no active chat model")
	}
	metadata, e := info.ToModelkitModel()
	if e != nil {
		t.Fatal("model metadata invalid")
	}
	cm, e := modelkit.NewModelKit(u.logger.Logger).GetChatModel(context.Background(), metadata)
	if e != nil {
		t.Fatal("model initialization failed")
	}
	// Claim only this transaction-local synthetic run. Never poll the business
	// queue when this opt-in test reads the local model configuration.
	r.State="running";r.LeaseEpoch++;until:=time.Now().Add(5*time.Minute);r.LeaseUntil=&until
	if e=db.Model(&domain.AssistantRun{}).Where("id=?",r.ID).Updates(map[string]any{"state":r.State,"lease_epoch":r.LeaseEpoch,"lease_until":until}).Error;e!=nil{t.Fatal(e)}
	plan := generationPlan()
	r.Checkpoint["planning"] = plan
	facts := "这是合成测试资料，不是真实产品。合成产品A面向设备管理负责人，解决纸质巡检记录分散、异常无法追踪的问题。巡检人员集中填写检查结果，异常生成工单；维修人员接单处理并回填结果，管理者查看未闭环事项。试点从单个车间开始，先统一点检表与责任人，再复盘处理结果。本资料没有性能数据、客户案例、报价或外部市场数据，不得补造。"
	r.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "user", ID: r.ID, Name: "用户提供的合成测试资料", Version: digest([]byte(facts))}})
	messages := []*schema.Message{schema.SystemMessage(`你是企业文件助手。只使用合成测试资料，不编造参数或案例。输出一个完整JSON对象，PPT格式为{"format":"pptx","slides":[{"title":"标题","bullets":["有依据的要点"]}]}。每次仅生成所要求的一页，最多5点、350字。不要返回Markdown围栏。`), schema.UserMessage("制作3页简洁的合成产品介绍，给设备管理负责人看。资料：" + facts)}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	started := time.Now()
	t.Log("configured-model segmented generation started; synthetic input only")
	spec, response, e := u.generateAssistantParts(ctx, r, cm, messages, plan)
	if e != nil {
		t.Fatalf("configured-model generation failed: %s", safeAssistantError(e))
	}
	if len(assistantArray(spec["slides"])) != 3 || assistantGenerationProgress(r).Completed != 3 {
		t.Fatal("live generation lost pages")
	}
	if output := os.Getenv("PANDA_WIKI_TEST_GENERATION_OUTPUT"); output != "" {
		data, _ := json.MarshalIndent(spec, "", "  ")
		if e = os.WriteFile(output+".json", data, 0600); e != nil {
			t.Fatal(e)
		}
		if token := os.Getenv("PANDA_WIKI_TEST_GENERATOR_TOKEN"); token != "" {
			u.cfg = &config.Config{Assistant: config.AssistantConfig{GeneratorURL: "http://127.0.0.1:18091", GeneratorToken: token}}
			u.client = &http.Client{Timeout: 30 * time.Second}
			if e = prepareArtifactSpec(spec, "pptx", r.Sources, nil); e != nil {
				t.Fatal(e)
			}
			var rendered struct {
				Data string `json:"data"`
			}
			if e = u.generator(ctx, "/render", map[string]any{"format": "pptx", "spec": spec}, &rendered); e != nil {
				t.Fatal(e)
			}
			bytes, e := base64.StdEncoding.DecodeString(rendered.Data)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(output+".pptx", bytes, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Logf("configured-model result: pages=3 elapsed=%s prompt_tokens=%d completion_tokens=%d", time.Since(started).Round(time.Second), response.ResponseMeta.Usage.PromptTokens, response.ResponseMeta.Usage.CompletionTokens)
}
