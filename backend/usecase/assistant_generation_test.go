package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func TestAssistantAutomaticAndRequestedPlanningPostgres(t *testing.T) {
	for _, tc := range []struct {
		message, feedback string
		automatic         bool
	}{
		{"给管理层制作3页产品PPT，简洁商务风格", "", true},
		{"制作3页PPT，先给我看大纲，等我确认", "", false},
		{"制作3页PPT，不用先看大纲，直接生成", "", true},
		{"先给我看大纲", "现在直接生成", true},
		{"直接生成PPT", "先别生成，先给我看结构", false},
	} {
		t.Run(tc.message+tc.feedback, func(t *testing.T) {
			u, _, _ := planningFixture(t)
			r, e := u.claimRun(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			facts := strings.Repeat("合成资料说明设备巡检、异常工单和闭环复盘，没有客户案例或性能参数。", 3)
			r.Checkpoint["user_facts"], r.Checkpoint["feedback"] = facts, tc.feedback
			s := domain.AssistantSource{Kind: "user", ID: r.ID, Version: digest([]byte(facts)), Name: "合成资料"}
			r.Sources = sourcesJSON([]domain.AssistantSource{s})
			cm := &planningChat{plan: domain.AssistantPlan{Title: "设备巡检介绍", Sufficient: true, Outline: []domain.AssistantOutlineItem{{Title: "巡检与工单闭环", Purpose: "合成产品价值", SourceIDs: []string{assistantSourceKey(s)}}}}}
			ready, e := u.prepareAssistantPlan(context.Background(), r, domain.AssistantSubmit{Message: tc.message}, "pptx", facts, nil, nil, cm)
			if e != nil || ready != tc.automatic {
				t.Fatalf("decision=%t wanted=%t err=%v", ready, tc.automatic, e)
			}
			if !tc.automatic && r.State != "awaiting_outline" {
				t.Fatal("explicit review preference ignored")
			}
			if tc.automatic {
				if confirmed, _ := r.Checkpoint["outline_confirmed"].(bool); !confirmed {
					t.Fatal("automatic plan not durable")
				}
			}
		})
	}
}

func partResponse(title, body string) *schema.Message {
	b, _ := json.Marshal(domain.JSONB{"format": "pptx", "slides": []domain.JSONB{{"title": title, "bullets": []string{body}}}})
	return &schema.Message{Content: string(b), ResponseMeta: &schema.ResponseMeta{FinishReason: "stop", Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}}}
}

func generationPlan() *domain.AssistantPlan {
	return &domain.AssistantPlan{Revision: 1, Format: "pptx", Title: "合成产品介绍", Sufficient: true, Outline: []domain.AssistantOutlineItem{{ID: "part-1", Title: "巡检记录", Purpose: "纸质记录集中管理"}, {ID: "part-2", Title: "异常工单", Purpose: "异常转为可追踪工单"}, {ID: "part-3", Title: "闭环复盘", Purpose: "管理者查看处理进展"}}}
}

func TestAssistantGenerationResumeAndInvalidatePostgres(t *testing.T) {
	u, db, _ := planningFixture(t)
	ctx := context.Background()
	r, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	plan := generationPlan()
	r.Checkpoint["planning"] = plan
	messages := []*schema.Message{schema.UserMessage("合成资料：纸质巡检集中记录，异常创建工单，管理者复盘闭环。")}
	cm := &scriptedPlanningChat{replies: []*schema.Message{partResponse("临时标题", "巡检人员集中登记检查结果。"), {Content: `{"slides":`}, {Content: "not-json"}}}
	if _, _, e = u.generateAssistantParts(ctx, r, cm, messages, plan); e == nil {
		t.Fatal("incomplete draft passed")
	}
	current, e := u.Run(ctx, r.KBID, r.UserID, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	if current.Generation == nil || current.Generation.Completed != 1 || current.Generation.Total != 3 || current.Generation.Parts[0].Title != "巡检记录" || current.Generation.Parts[0].Preview != "巡检人员集中登记检查结果。" {
		t.Fatalf("durable draft missing: %+v", current.Generation)
	}
	// Recreate the worker's run from persisted JSON, just like a new lease after
	// process restart; completed work must not depend on in-memory references.
	if e = db.First(r, "id=?", r.ID).Error; e != nil {
		t.Fatal(e)
	}
	remaining := &scriptedPlanningChat{replies: []*schema.Message{partResponse("异常工单", "异常生成工单，由维修人员接单处理。"), partResponse("闭环复盘", "管理者查看未闭环工单并复盘处理结果。")}}
	spec, response, e := u.generateAssistantParts(ctx, r, remaining, messages, assistantPlan(r))
	if e != nil || remaining.calls != 2 || len(assistantArray(spec["slides"])) != 3 {
		t.Fatalf("did not resume at second page: calls=%d err=%v", remaining.calls, e)
	}
	if assistantArray(spec["slides"])[0]["title"] != "巡检记录" {
		t.Fatal("completed page was replaced")
	}
	if response.ResponseMeta.Usage.TotalTokens != 90 {
		t.Fatalf("generation usage not aggregated: %+v", response.ResponseMeta.Usage)
	}
	// A changed instruction cannot reuse a cached draft made from other inputs.
	newModel := &scriptedPlanningChat{replies: []*schema.Message{partResponse("巡检记录", "新的合成描述。"), partResponse("异常工单", "新的工单描述。"), partResponse("闭环复盘", "新的闭环描述。")}}
	if _, _, e = u.generateAssistantParts(ctx, r, newModel, []*schema.Message{schema.UserMessage("新资料和不同的约束")}, assistantPlan(r)); e != nil || newModel.calls != 3 {
		t.Fatal("stale draft reused after inputs changed", e)
	}
}

func TestAssistantGenerationLegacyCheckpointPostgres(t *testing.T) {
	u, _, _ := planningFixture(t)
	r, e := u.claimRun(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	plan := generationPlan()
	messages := []*schema.Message{schema.UserMessage("合成事实：巡检记录、异常工单、闭环复盘。")}
	planBytes, _ := json.Marshal(plan)
	messageBytes, _ := json.Marshal(messages)
	sourceBytes, _ := json.Marshal(r.Sources)
	// Exact pre-optimization persisted fingerprint, without new optional fields.
	legacy := digest([]byte(`{"Plan":` + string(planBytes) + `,"Messages":` + string(messageBytes) + `,"Sources":` + string(sourceBytes) + `}`))
	r.Checkpoint["planning"] = plan
	r.Checkpoint["content_generation"] = assistantGenerationCheckpoint{Manifest: legacy, PlanHash: assistantPlanHash(plan), Parts: []domain.JSONB{{"id": "part-1", "title": "巡检记录", "bullets": []string{"原先已完成的内容，必须保留。"}}}}
	cm := &scriptedPlanningChat{replies: []*schema.Message{partResponse("异常工单", "异常转为工单。"), partResponse("闭环复盘", "复盘未闭环事项。")}}
	spec, _, e := u.generateAssistantParts(context.Background(), r, cm, messages, plan)
	if e != nil || cm.calls != 2 {
		t.Fatal("legacy completed work was regenerated", e, cm.calls)
	}
	var lines []string
	_ = decodeAssistantValue(assistantArray(spec["slides"])[0]["bullets"], &lines)
	if len(lines) != 1 || lines[0] != "原先已完成的内容，必须保留。" {
		t.Fatal("legacy draft overwritten")
	}
}

func TestAssistantPartialDraftRevocationPostgres(t *testing.T) {
	u, db, run := planningFixture(t)
	ctx := context.Background()
	r, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: r.KBID, UserID: r.UserID, Name: "合成资料.txt", Format: "txt", Status: "ready", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e = db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	r.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "upload", ID: upload.ID, Name: upload.Name}})
	plan := generationPlan()
	r.Checkpoint["planning"] = plan
	cm := &scriptedPlanningChat{replies: []*schema.Message{partResponse("巡检记录", "此部分含已授权的合成资料。"), nil, nil}}
	_, _, _ = u.generateAssistantParts(ctx, r, cm, nil, plan)
	if e = db.Model(&domain.AssistantUpload{}).Where("id=?", upload.ID).Update("status", "deleted").Error; e != nil {
		t.Fatal(e)
	}
	current, e := u.Run(ctx, run.KBID, run.UserID, run.ID)
	if e != nil || current.Generation != nil || current.Planning != nil {
		t.Fatal("revoked partial draft leaked", e)
	}
	if _, _, e = u.generateAssistantParts(ctx, r, &scriptedPlanningChat{}, nil, plan); !errors.Is(e, ErrAssistantAccess) {
		t.Fatal("revoked source was used to resume", e)
	}
}

func TestAssistantGeneratedPartValidation(t *testing.T) {
	good := domain.JSONB{"format": "pptx", "slides": []domain.JSONB{{"title": "真实依据", "bullets": []string{"合成测试事实。"}}}}
	if e := validateAssistantGeneratedPart(good, "pptx"); e != nil {
		t.Fatal(e)
	}
	comparison := domain.JSONB{"format": "pptx", "slides": []domain.JSONB{{"title": "适用范围比较", "table": [][]any{{"类型", "范围"}, {"工厂级", "全厂生产运营"}, {"装备级", "单台装备作业与检修"}}}}}
	if e := validateAssistantGeneratedPart(comparison, "pptx"); e != nil {
		t.Fatal(e)
	}
	comparison["slides"] = []domain.JSONB{{"title": "适用范围比较", "table": [][]any{{"类型", "范围"}, {"工厂级", strings.Repeat("字", 61)}}}}
	if e := validateAssistantGeneratedPart(comparison, "pptx"); e == nil || !strings.Contains(e.Error(), "最多60字") {
		t.Fatal("dense comparison needs an actionable correction", e)
	}
	for _, part := range []domain.JSONB{{"title": "空白"}, {"title": "错误类型", "bullets": "text"}, {"title": "资料不足"}, {"title": "过长", "bullets": []string{strings.Repeat("长", 421)}}, {"title": "忽略字段", "paragraphs": []string{"渲染器不会读取此字段"}}, {"title": "错误图表", "chart": domain.JSONB{"type": "line", "categories": []string{"A", "B"}, "series": []domain.JSONB{{"values": []float64{1}}}}}} {
		if validateAssistantGeneratedPart(domain.JSONB{"format": "pptx", "slides": []domain.JSONB{part}}, "pptx") == nil {
			t.Fatalf("unrenderable draft accepted: %+v", part)
		}
	}
}

func TestAssistantRetrievalTopicIsolation(t *testing.T) {
	history := "此前聊的是无关的甲产品演示"
	if strings.Contains(assistantRetrievalQuestion("制作乙产品介绍PPT", history, "供客户演示"), "甲产品") {
		t.Fatal("fresh task inherited an unrelated retrieval topic")
	}
	if !strings.Contains(assistantRetrievalQuestion("把刚才的介绍整理成PPT", history, ""), "甲产品") {
		t.Fatal("explicit follow-up lost its subject")
	}
}

func TestAssistantTemporaryFailureRecoveryPostgres(t *testing.T) {
	for _, tc := range []struct {
		message   string
		calls     int
		wantError bool
	}{{"status code: 503 temporarily unavailable", 2, false}, {"status code: 401 invalid credentials", 1, true}} {
		t.Run(tc.message, func(t *testing.T) {
			u, _, _ := planningFixture(t)
			r, e := u.claimRun(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			cm := &scriptedPlanningChat{replies: []*schema.Message{nil, partResponse("合成页", "已有事实支撑的测试内容。")}, errors: []error{errors.New(tc.message)}}
			_, _, e = u.generateAssistantSpec(context.Background(), r, cm, nil)
			if (e != nil) != tc.wantError || cm.calls != tc.calls {
				t.Fatalf("unexpected retry policy: calls=%d err=%v", cm.calls, e)
			}
		})
	}
}

func TestAssistantDurableRunAutomaticResumePolicy(t *testing.T) {
	r := &domain.AssistantRun{Attempt: 1, Checkpoint: domain.JSONB{"spec": domain.JSONB{"format": "pptx"}}}
	if !assistantCanResumeAutomatically(r, context.DeadlineExceeded) || !assistantCanResumeAutomatically(r, errors.New("status code: 503")) {
		t.Fatal("recoverable rendering interruption lost durable work")
	}
	for _, err := range []error{context.Canceled, errors.New("status code: 401"), errors.New("invalid content")} {
		if assistantCanResumeAutomatically(r, err) {
			t.Fatal("permanent/canceled failure was retried")
		}
	}
	r.Attempt = 3
	if assistantCanResumeAutomatically(r, context.DeadlineExceeded) {
		t.Fatal("unbounded retries")
	}
}

func TestAssistantPartialDraftCancellationPostgres(t *testing.T) {
	u, _, _ := planningFixture(t)
	ctx := context.Background()
	r, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	plan := generationPlan()
	r.Checkpoint["planning"] = plan
	cm := &scriptedPlanningChat{replies: []*schema.Message{partResponse("巡检记录", "已完成的合成内容。"), nil, nil}}
	_, _, _ = u.generateAssistantParts(ctx, r, cm, nil, plan)
	if e = u.CancelRun(ctx, r.KBID, r.UserID, r.ID); e != nil {
		t.Fatal(e)
	}
	late := &scriptedPlanningChat{replies: []*schema.Message{partResponse("异常工单", "不应再生成")}}
	if _, _, e = u.generateAssistantParts(ctx, r, late, nil, plan); !errors.Is(e, context.Canceled) || late.calls != 0 {
		t.Fatal("canceled draft continued generating", e)
	}
}

func TestAssistantNaturalFileIntentParity(t *testing.T) {
	data, e := os.ReadFile("../../docs/fixtures/assistant-file-intents.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Message string `json:"message"`
		Format  string `json:"format"`
	}
	if e = json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	for _, tc := range cases {
		if format := assistantFileFormat(domain.AssistantSubmit{Message: tc.Message}, nil); format != tc.Format {
			t.Errorf("%s: got %s, expected %s", tc.Message, format, tc.Format)
		}
	}
	if format := assistantFileFormat(domain.AssistantSubmit{Message: "把这个PPT导出成Word"}, &domain.AssistantArtifact{Format: "pptx"}); format != "docx" {
		t.Fatal("conversion target lost to base format")
	}
}
