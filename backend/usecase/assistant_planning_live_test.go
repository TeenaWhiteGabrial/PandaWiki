package usecase

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

// Opt in only: transaction-local synthetic data, no retrieval, no OSS writes,
// no generated artifacts. Verifies the actual configured model's planning JSON.
func TestAssistantPlanningLiveModel(t *testing.T) {
	if os.Getenv("PANDA_WIKI_TEST_LIVE_LLM") != "1" {
		t.Skip("set PANDA_WIKI_TEST_LIVE_LLM=1 for configured-model planning")
	}
	u, db, run := planningFixture(t)
	var setting domain.SystemSetting
	if e := db.Where("key=?", consts.SystemSettingModelMode).First(&setting).Error; e != nil {
		t.Skip("local model setting unavailable")
	}
	var mode domain.ModelModeSetting
	if json.Unmarshal(setting.Value, &mode) != nil {
		t.Fatal("invalid local model setting")
	}
	var info domain.Model
	if mode.Mode == consts.ModelSettingModeAuto {
		provider, e := (&ModelUsecase{}).getAutoModeProvider(mode)
		if e != nil {
			t.Fatal("local automatic model unavailable")
		}
		info = domain.Model{Provider: provider.Provider, Model: provider.Models.Chat, Type: domain.ModelTypeChat, BaseURL: getAutoModeModelBaseURL(provider, domain.ModelTypeChat), APIKey: provider.APIKey}
	} else if e := db.Where("type=? AND is_active=true", domain.ModelTypeChat).First(&info).Error; e != nil {
		t.Skip("no active local chat model")
	}
	metadata, e := info.ToModelkitModel()
	if e != nil {
		t.Fatal("invalid model metadata")
	}
	cm, e := modelkit.NewModelKit(u.logger.Logger).GetChatModel(context.Background(), metadata)
	if e != nil {
		t.Fatal("model initialization failed")
	}
	for _, tc := range []struct {
		name, message, material string
		wantSufficient          bool
	}{
		{"insufficient real-body evidence", "生成星港与星槎客户演示PPT，给制造业设备管理负责人介绍产品价值。10分钟、6页，不编造数据。", "星槎设备巡检计划、星港库存盘点规范都是权限测试代号，只有文件名称，没有产品介绍正文。", false},
		{"explicit brief and synthetic facts", "生成合成产品A的客户演示PPT。受众：制造业设备管理负责人；目的：介绍巡检到工单闭环的价值并讨论试点；5分钟、3页；简洁务实，不写性能数据、报价或案例。", "合成产品A面向设备管理负责人，解决纸质巡检记录分散、异常无法追踪的问题。巡检人员在手机填写检查结果，异常生成工单；维修人员接单处理并上传结果，管理者查看闭环状态。试点可从单个车间开始，先统一点检表与责任人，再复盘未闭环工单。此资料没有性能指标、客户案例或报价，不可对这些内容作出事实主张。", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := *run
			claim.State = "running"
			claim.LeaseEpoch++
			claim.Checkpoint = domain.JSONB{}
			claim.Sources = sourcesJSON(nil)
			if tc.wantSufficient {
				claim.Checkpoint["user_facts"] = tc.material
				source := domain.AssistantSource{Kind: "user", ID: claim.ID, Version: digest([]byte(tc.material)), Name: "合成测试资料"}
				claim.Sources = sourcesJSON([]domain.AssistantSource{source})
			}
			until := time.Now().Add(3 * time.Minute)
			claim.LeaseUntil = &until
			if e = db.Model(&domain.AssistantRun{}).Where("id=?", run.ID).Updates(map[string]any{"state": "running", "lease_epoch": claim.LeaseEpoch, "lease_until": until, "checkpoint": claim.Checkpoint, "sources": claim.Sources}).Error; e != nil {
				t.Fatal(e)
			}
			request := domain.AssistantSubmit{Message: tc.message + "先给我看大纲，等我确认。", SourceScope: domain.AssistantScope{Type: "documents"}}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			ready, err := u.prepareAssistantPlan(ctx, &claim, request, "pptx", tc.material, nil, nil, cm)
			if err != nil {
				t.Fatalf("live planning failed: %s", safeAssistantError(err))
			}
			plan := assistantPlan(&claim)
			if ready || plan == nil || plan.Sufficient != tc.wantSufficient {
				t.Fatalf("unexpected planning gate: ready=%t sufficient=%v", ready, plan != nil && plan.Sufficient)
			}
			if tc.wantSufficient && (claim.State != "awaiting_outline" || len(plan.Outline) != 3 || len(plan.Questions) > 0) {
				t.Fatalf("explicit brief was not converted into three-part outline: state=%s outline=%d questions=%d", claim.State, len(plan.Outline), len(plan.Questions))
			}
			if !tc.wantSufficient && len(plan.Gaps) == 0 {
				t.Fatal("insufficient material not explained")
			}
			t.Logf("configured model planning: state=%s sufficient=%t questions=%d outline=%d", claim.State, plan.Sufficient, len(plan.Questions), len(plan.Outline))
			if tc.wantSufficient {
				for _, part := range plan.Outline {
					if strings.TrimSpace(part.Purpose) == "" {
						t.Fatal("outline missing core viewpoint")
					}
				}
			}
		})
	}
}

func TestAssistantClarificationLiveModel(t *testing.T) {
	if os.Getenv("PANDA_WIKI_TEST_LIVE_LLM") != "1" {
		t.Skip("set PANDA_WIKI_TEST_LIVE_LLM=1 for configured-model clarification")
	}
	u, db, run := planningFixture(t)
	var setting domain.SystemSetting
	if db.Where("key=?", consts.SystemSettingModelMode).First(&setting).Error != nil {
		t.Skip("no local model setting")
	}
	var mode domain.ModelModeSetting
	if json.Unmarshal(setting.Value, &mode) != nil || mode.Mode != consts.ModelSettingModeAuto {
		t.Skip("automatic local model required")
	}
	provider, e := (&ModelUsecase{}).getAutoModeProvider(mode)
	if e != nil {
		t.Fatal("no local automatic model")
	}
	info := domain.Model{Provider: provider.Provider, Model: provider.Models.Chat, Type: domain.ModelTypeChat, BaseURL: getAutoModeModelBaseURL(provider, domain.ModelTypeChat), APIKey: provider.APIKey}
	metadata, e := info.ToModelkitModel()
	if e != nil {
		t.Fatal("invalid model metadata")
	}
	cm, e := modelkit.NewModelKit(u.logger.Logger).GetChatModel(context.Background(), metadata)
	if e != nil {
		t.Fatal("model initialization failed")
	}
	claim := *run
	claim.State = "running"
	claim.LeaseEpoch = 1
	claim.LeaseUntil = new(time.Time)
	*claim.LeaseUntil = time.Now().Add(3 * time.Minute)
	if e = db.Model(&domain.AssistantRun{}).Where("id=?", run.ID).Updates(map[string]any{"state": claim.State, "lease_epoch": claim.LeaseEpoch, "lease_until": claim.LeaseUntil}).Error; e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Second)
	defer cancel()
	ready, err := u.prepareAssistantPlan(ctx, &claim, domain.AssistantSubmit{Message: "帮我生成有关合成产品A的介绍PPT，我要给客户演示。"}, "pptx", "没有产品正文。", nil, nil, cm)
	if err != nil {
		t.Fatalf("live clarification failed: %s", safeAssistantError(err))
	}
	plan := assistantPlan(&claim)
	if ready || claim.State != "awaiting_input" || plan == nil || len(plan.Questions) == 0 || len(plan.Questions) > 3 {
		t.Fatal("ambiguous customer demo did not ask bounded critical questions")
	}
	t.Logf("configured model clarification: state=%s questions=%d", claim.State, len(plan.Questions))
}
