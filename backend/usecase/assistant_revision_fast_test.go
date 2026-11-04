package usecase

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestAssistantLiteralRevisionSafety(t *testing.T) {
	for _, message := range []string{"请把第三页的标题改为“设备巡检”", "第三页标题改为设备巡检", "在第三页删除第2条", "所选页面删除第二个要点，其他内容保持不变"} {
		if _, ok := assistantLiteralRevision(message); !ok {
			t.Fatal("literal command missed", message)
		}
	}
	for _, message := range []string{"不要删除第2条", "如果没用就删除第2条", "第三页删除第二条并修改第一条", "给第三页想一个更好的标题", "标题改为更简洁一点", "第三页标题改为新标题并精简正文", "删除第二条，并生成一个案例"} {
		if _, ok := assistantLiteralRevision(message); ok {
			t.Fatal("ambiguous or compound command bypassed model", message)
		}
	}
	base := revisionFixture()
	before, _ := json.Marshal(base.Spec)
	parts := assistantArray(base.Spec["slides"])
	req := domain.AssistantSubmit{Message: "第三页标题改为“设备巡检”"}
	result, handled, e := assistantApplyLiteralRevision(req, base, []domain.JSONB{parts[2]})
	if e != nil || !handled {
		t.Fatal(e)
	}
	changed := assistantArray(result["slides"])
	if changed[2]["title"] != "设备巡检" || !reflect.DeepEqual(changed[0], parts[0]) || !reflect.DeepEqual(changed[2]["bullets"], parts[2]["bullets"]) {
		t.Fatal("literal title edit changed unrelated content")
	}
	after, _ := json.Marshal(base.Spec)
	if string(before) != string(after) {
		t.Fatal("original artifact mutated")
	}
	if _, _, e = assistantApplyLiteralRevision(req, base, parts[:2]); e == nil {
		t.Fatal("ambiguous multiple targets accepted")
	}
	req.Message = "第三页标题改为“" + strings.Repeat("长", 51) + "”"
	if _, _, e = assistantApplyLiteralRevision(req, base, []domain.JSONB{parts[2]}); e == nil {
		t.Fatal("oversized title accepted")
	}
}

func TestAssistantLiteralBulletRemoval(t *testing.T) {
	base := revisionFixture()
	parts := assistantArray(base.Spec["slides"])
	parts[2]["bullets"] = []string{"保留一", "删除二", "保留三"}
	base.Spec["slides"] = parts
	req := domain.AssistantSubmit{Message: "第三页删除第2条"}
	result, handled, e := assistantApplyLiteralRevision(req, base, []domain.JSONB{parts[2]})
	if e != nil || !handled {
		t.Fatal(e)
	}
	actual := assistantArray(result["slides"])
	var bullets []string
	_ = decodeAssistantValue(actual[2]["bullets"], &bullets)
	if !reflect.DeepEqual(bullets, []string{"保留一", "保留三"}) || !reflect.DeepEqual(actual[1], parts[1]) {
		t.Fatal("wrong bullet removed")
	}
	for _, message := range []string{"删除第0条", "删除第4条"} {
		req.Message = message
		if _, _, e = assistantApplyLiteralRevision(req, base, []domain.JSONB{parts[2]}); e == nil {
			t.Fatal("invalid bullet accepted")
		}
	}
	parts[2]["bullets"] = []string{"唯一要点"}
	req.Message = "删除第1条"
	if _, _, e = assistantApplyLiteralRevision(req, base, []domain.JSONB{parts[2]}); e == nil {
		t.Fatal("deleted final body point")
	}
}

func TestAssistantRevisionContentOnlyScope(t *testing.T) {
	for _, tc := range []struct {
		message string
		want    bool
	}{
		{"第三页精简为两个要点，不新增数字", true}, {"润色第三页的措辞", true},
		{"第三页精简，并增加一组参数", false}, {"参考知识库纠正第三页", false}, {"增加产品介绍", false},
	} {
		r := &domain.AssistantRun{Checkpoint: domain.JSONB{}}
		messages := assistantRevisionMessages(r, domain.AssistantSubmit{Message: tc.message}, "UNRELATED_SOURCE_BODY")
		if r.Checkpoint["revision_content_only"] != tc.want {
			t.Fatal("unexpected scope", tc)
		}
		contains := strings.Contains(messages[1].Content, "UNRELATED_SOURCE_BODY")
		if contains == tc.want {
			t.Fatal("source body selection incorrect", tc)
		}
	}
	for _, req := range []domain.AssistantSubmit{{Message: "精简第三页", AttachmentIDs: []string{"new"}}, {Message: "精简第三页", WebSearchEnabled: true}, {Message: "精简第三页", SourceScope: domain.AssistantScope{NodeIDs: []string{"source"}}}} {
		if assistantRevisionContentOnly(req) {
			t.Fatal("explicit additional source ignored")
		}
	}
	r := &domain.AssistantRun{Checkpoint: domain.JSONB{}}
	base := &domain.AssistantArtifact{Sources: sourcesJSON([]domain.AssistantSource{{Kind: "node", ID: "old"}})}
	req := domain.AssistantSubmit{Message: "精简第三页", SourceScope: domain.AssistantScope{NodeIDs: []string{"old"}}}
	assistantRevisionMessages(r, req, "source", base)
	if r.Checkpoint["revision_content_only"] != true {
		t.Fatal("unchanged source picker blocked efficient editing")
	}
	req.SourceScope.NodeIDs = []string{"new"}
	r.Checkpoint = domain.JSONB{}
	assistantRevisionMessages(r, req, "source", base)
	if r.Checkpoint["revision_content_only"] != false {
		t.Fatal("new source picker was ignored")
	}
	r.Checkpoint = domain.JSONB{"content_generation": assistantGenerationCheckpoint{Parts: []domain.JSONB{{"title": "已完成部分"}}}}
	assistantRevisionMessages(r, domain.AssistantSubmit{Message: "精简第三页"}, "source", base)
	if r.Checkpoint["revision_content_only"] != false {
		t.Fatal("optimization changed legacy in-flight prompt mode")
	}
}
