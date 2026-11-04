package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestAssistantModelJSONWrappersAndRejections(t *testing.T) {
	valid := `{"title":"括号 { 与 }、引号 \\\"","brief":{"audience":"客户"},"questions":[],"sufficient":false,"gaps":["缺资料"],"outline":[]}`
	for _, wrapper := range []string{valid, "\ufeff" + valid, "```json\n" + valid + "\n```", "```JSON\r\n" + valid + "\r\n```", "说明：请确认以下内容。\n" + valid + "\n以上是计划。", "<think>分析中的示例 {不是输出}</think>\n" + valid} {
		var plan domain.AssistantPlan
		if e := decodeAssistantModelJSON(wrapper, &plan); e != nil || plan.Title == "" {
			t.Fatalf("valid wrapped object rejected: %v", e)
		}
	}
	for _, bad := range []string{"", "null", "[]", "[" + valid + "]", valid + valid, `{"title":"未完成","brief":{"audience":"客户"}`, `{"title":"bad",}`, `{"title":"bad","brief":[]}`, "<think>未闭合" + valid, strings.Repeat("x", (128<<10)+1)} {
		var plan domain.AssistantPlan
		if decodeAssistantModelJSON(bad, &plan) == nil {
			t.Fatalf("invalid or ambiguous response accepted: %.80s", bad)
		}
	}
}

type scriptedPlanningChat struct {
	replies []*schema.Message
	errors  []error
	calls   int
}

func TestAssistantFormalContentRecoveryPostgres(t *testing.T) {
	u, _, _ := planningFixture(t)
	r, e := u.claimRun(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	good := `{"format":"xlsx","title":"合成文件","answer":"已整理","sheets":[{"name":"数据","columns":["名称"],"rows":[["合成资料"]]}],"catalog_export":true}`
	cm := &scriptedPlanningChat{replies: []*schema.Message{{Content: `{"format":"xlsx"`, ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}, {Content: "以下是文件：\n```JSON\n" + good + "\n```"}}}
	spec, _, e := u.generateAssistantSpec(context.Background(), r, cm, nil)
	if e != nil || cm.calls != 2 || spec["format"] != "xlsx" {
		t.Fatalf("formal content recovery failed: %v", e)
	}
	if _, exists := spec["catalog_export"]; exists {
		t.Fatal("model enabled catalog capability")
	}
	var diagnostics []domain.JSONB
	if e = decodeAssistantValue(r.Checkpoint["content_generation_diagnostics"], &diagnostics); e != nil || len(diagnostics) != 2 || diagnostics[0]["finish_reason"] != "length" || diagnostics[1]["valid_content"] != true {
		t.Fatal("formal content failure unobservable", e)
	}
}

func (m *scriptedPlanningChat) Generate(context.Context, []*schema.Message, ...einomodel.Option) (*schema.Message, error) {
	i := m.calls
	m.calls++
	if i < len(m.errors) && m.errors[i] != nil {
		return nil, m.errors[i]
	}
	if i >= len(m.replies) {
		return nil, nil
	}
	return m.replies[i], nil
}
func (*scriptedPlanningChat) Stream(context.Context, []*schema.Message, ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	panic("unused")
}

func TestAssistantPlanResponseRecoveryPostgres(t *testing.T) {
	good := `{"title":"合成文件","summary":"先确认目的","brief":{"goal":"方案介绍"},"questions":[{"id":"goal","question":"使用目的？","default":"介绍方案","options":["介绍方案","内部培训"]}],"sufficient":false,"gaps":["需要正文"],"outline":[]}`
	for _, tc := range []struct {
		name    string
		replies []*schema.Message
		calls   int
		failure bool
	}{
		{"prose wrapper without another request", []*schema.Message{{Content: "以下是计划：\n```JSON\n" + good + "\n```"}}, 1, false},
		{"wrong schema repaired once", []*schema.Message{{Content: `{"title":"合成文件","brief":["only-secret-model-response"]}`}, {Content: good}}, 2, false},
		{"truncated even if JSON ends balanced", []*schema.Message{{Content: good, ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}, {Content: good}}, 2, false},
		{"both invalid fail closed", []*schema.Message{{Content: "only-secret-model-response"}, {Content: `{"title":`}}, 2, true},
		{"nil responses fail closed", []*schema.Message{nil, nil}, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, db, run := planningFixture(t)
			ctx := context.Background()
			claim, e := u.claimRun(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var request domain.AssistantSubmit
			_ = assistantDecode(claim.Request, &request)
			model := &scriptedPlanningChat{replies: tc.replies}
			ready, e := u.prepareAssistantPlan(ctx, claim, request, "pptx", "", nil, nil, model)
			if ready || model.calls != tc.calls || (e != nil) != tc.failure {
				t.Fatalf("unexpected recovery: ready=%t calls=%d err=%v", ready, model.calls, e)
			}
			current, readErr := u.Run(ctx, run.KBID, run.UserID, run.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !tc.failure && current.State != "awaiting_input" {
				t.Fatal("recovered plan did not pause for user")
			}
			if tc.calls == 2 && current.Checkpoint["planning_diagnostics"] == nil {
				t.Fatal("missing diagnostic checkpoint")
			}
			data, _ := json.Marshal(current)
			if strings.Contains(string(data), "only-secret-model-response") || strings.Contains(string(data), "planning_invalid_response") {
				t.Fatal("raw model response escaped private checkpoint")
			}
			var artifacts int64
			db.Model(&domain.AssistantArtifact{}).Where("run_id=?", run.ID).Count(&artifacts)
			if artifacts != 0 {
				t.Fatal("malformed response caused a file publication")
			}
		})
	}
}

func TestAssistantPlanWorkbookConstraintRecoveryPostgres(t *testing.T) {
	u, _, run := planningFixture(t)
	ctx := context.Background()
	claim, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	claim.Checkpoint["planning_diagnostics"] = []domain.JSONB{{"attempt": 2, "parse_error": "old failure"}}
	model := &scriptedPlanningChat{replies: []*schema.Message{
		{Content: `{"title":"目录册","brief":{},"questions":[],"sufficient":false,"gaps":["需确认"],"outline":[{"title":"A"},{"title":"B"},{"title":"C"},{"title":"D"},{"title":"E"}]}`},
		{Content: `{"title":"目录册","brief":{},"questions":[{"id":"goal","question":"用途？","default":"目录索引"}],"sufficient":false,"gaps":["需确认"],"outline":[]}`},
	}}
	var request domain.AssistantSubmit
	_ = assistantDecode(claim.Request, &request)
	if ready, e := u.prepareAssistantPlan(ctx, claim, request, "xlsx", "", nil, nil, model); ready || e != nil || model.calls != 2 {
		t.Fatalf("workbook constraint recovery failed: %t %v calls=%d", ready, e, model.calls)
	}
	current, e := u.Run(ctx, run.KBID, run.UserID, run.ID)
	if e != nil || current.State != "awaiting_input" {
		t.Fatal("recovery did not persist", e)
	}
	var diagnostics []domain.JSONB
	if e = decodeAssistantValue(current.Checkpoint["planning_diagnostics"], &diagnostics); e != nil || len(diagnostics) != 2 || diagnostics[0]["valid_json"] != true || diagnostics[0]["valid_plan"] != false || diagnostics[1]["valid_plan"] != true {
		t.Fatalf("incorrect or stale diagnostic: %+v %v", diagnostics, e)
	}
}
