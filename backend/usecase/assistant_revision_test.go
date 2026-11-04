package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func TestAssistantExplicitRevisionConstraints(t *testing.T) {
	constraints := assistantRevisionConstraints("精简为两个要点，每点不超过45个汉字")
	if constraints.BulletCount != 2 || constraints.BulletMaxCharacters != 45 {
		t.Fatal(constraints)
	}
	if got := assistantRevisionConstraints("解释第三页，产品年产量最多45台"); got != (assistantPartConstraints{}) {
		t.Fatal("ordinary facts became editing constraints", got)
	}
	for _, bullets := range [][]string{{"只有一个要点"}, {strings.Repeat("长", 46), "第二点"}} {
		if validateAssistantPartConstraints(domain.JSONB{"slides": []domain.JSONB{{"bullets": bullets}}}, constraints) == nil {
			t.Fatal("explicit edit requirement ignored")
		}
	}
	if e := validateAssistantPartConstraints(domain.JSONB{"slides": []domain.JSONB{{"bullets": []string{"集中记录检查结果。", "异常工单闭环处理。"}}}}, constraints); e != nil {
		t.Fatal(e)
	}
}

func TestAssistantRevisionConstraintRecoveryPostgres(t *testing.T) {
	u, db, r := planningFixture(t)
	r.State = "running"
	r.LeaseEpoch++
	until := time.Now().Add(time.Minute)
	r.LeaseUntil = &until
	if e := db.Model(&domain.AssistantRun{}).Where("id=?", r.ID).Updates(map[string]any{"state": r.State, "lease_epoch": r.LeaseEpoch, "lease_until": until}).Error; e != nil {
		t.Fatal(e)
	}
	plan := generationPlan()
	plan.Outline = plan.Outline[:1]
	r.Checkpoint["planning"] = plan
	r.Checkpoint["editing_parts"] = domain.JSONB{plan.Outline[0].ID: domain.JSONB{"title": "原始标题", "bullets": []string{"原始测试资料"}}}
	r.Checkpoint["part_constraints"] = assistantRevisionConstraints("精简为两个要点，每点不超过45字")
	response := func(bullets []string) *schema.Message {
		data, _ := json.Marshal(domain.JSONB{"format": "pptx", "slides": []domain.JSONB{{"title": "原始标题", "bullets": bullets}}})
		return &schema.Message{Content: string(data), ResponseMeta: &schema.ResponseMeta{FinishReason: "stop"}}
	}
	model := &scriptedPlanningChat{replies: []*schema.Message{{ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}, response([]string{strings.Repeat("长", 46), "第二点"}), response([]string{"集中记录检查结果。", "异常工单闭环处理。"})}}
	spec, _, e := u.generateAssistantParts(context.Background(), r, model, nil, plan)
	if e != nil || model.calls != 3 {
		t.Fatal("explicit edit was not corrected after truncation", e, model.calls)
	}
	var constraints assistantPartConstraints
	_ = decodeAssistantValue(r.Checkpoint["part_constraints"], &constraints)
	if e = validateAssistantPartConstraints(spec, constraints); e != nil {
		t.Fatal(e)
	}
}

func revisionFixture() *domain.AssistantArtifact {
	return &domain.AssistantArtifact{Format: "pptx", Spec: domain.JSONB{"format": "pptx", "title": "合成介绍", "slides": []domain.JSONB{
		{"id": "first", "title": "巡检记录", "bullets": []string{"原始巡检内容"}},
		{"id": "second", "title": "异常工单", "bullets": []string{"原始工单内容"}},
		{"id": "third", "title": "闭环复盘", "bullets": []string{"原始闭环内容"}},
		{"id": "system-sources", "title": "来源", "bullets": []string{"系统来源"}},
	}}}
}

func TestAssistantRevisionPreservesOtherParts(t *testing.T) {
	base := revisionFixture()
	before, _ := json.Marshal(base.Spec)
	targets, positions, e := assistantRevisionTargets(domain.AssistantSubmit{Message: "修改第三页"}, base)
	if e != nil || len(targets) != 1 || targets[0]["id"] != "third" || positions["third"] != 3 {
		t.Fatal("wrong edit target", e)
	}
	changed := assistantMergePartFields(targets[0], domain.JSONB{"title": "新的复盘标题"})
	result, e := mergeAssistantRevision(base, []domain.JSONB{changed})
	if e != nil {
		t.Fatal(e)
	}
	parts := assistantArray(result["slides"])
	for _, i := range []int{0, 1, 3} {
		a, _ := json.Marshal(parts[i])
		b, _ := json.Marshal(assistantArray(base.Spec["slides"])[i])
		if string(a) != string(b) {
			t.Fatalf("untargeted part %d changed", i)
		}
	}
	if parts[2]["title"] != "新的复盘标题" || len(assistantArray(result["slides"])) != 4 {
		t.Fatal("target title lost or page removed")
	}
	after, _ := json.Marshal(base.Spec)
	if string(before) != string(after) {
		t.Fatal("original artifact mutated")
	}
	if _, _, e = assistantRevisionTargets(domain.AssistantSubmit{Message: "修改第九页"}, base); e == nil {
		t.Fatal("nonexistent page accepted")
	}
	if _, e = mergeAssistantRevision(base, []domain.JSONB{{"id": "not-in-file"}}); e == nil {
		t.Fatal("foreign part inserted")
	}
}

func TestAssistantRevisionRangeAndDelete(t *testing.T) {
	base := revisionFixture()
	for _, tc := range []struct {
		message string
		count   int
	}{
		{"精简第二到第三页", 2}, {"精简第1、3页", 2}, {"精简第1～3页", 3}, {"调整异常工单的表达", 1},
	} {
		parts, _, e := assistantRevisionTargets(domain.AssistantSubmit{Message: tc.message}, base)
		if e != nil || len(parts) != tc.count {
			t.Fatalf("%s: selected=%d err=%v", tc.message, len(parts), e)
		}
	}
	for _, message := range []string{"精简第三到第二页", "修改这一页", "修改第1~8页"} {
		if _, _, e := assistantRevisionTargets(domain.AssistantSubmit{Message: message}, base); e == nil {
			t.Fatalf("ambiguous/invalid target expanded to all: %s", message)
		}
	}
	targets, _, _ := assistantRevisionTargets(domain.AssistantSubmit{Message: "删除第二页"}, base)
	spec, e := assistantRemoveParts(base, targets)
	if e != nil || len(assistantArray(spec["slides"])) != 3 || assistantArray(spec["slides"])[1]["id"] != "third" {
		t.Fatal("delete altered untargeted pages", e)
	}
	if len(assistantArray(base.Spec["slides"])) != 4 {
		t.Fatal("delete mutated the old version")
	}
	all, _, _ := assistantRevisionTargets(domain.AssistantSubmit{Message: "删除全部"}, base)
	if _, e = assistantRemoveParts(base, all); e == nil {
		t.Fatal("deleted every body page")
	}
	for _, message := range []string{"删除第三页的第二条", "删除第三页的图表", "删除第三页标题"} {
		if assistantDeleteWholeParts(message) {
			t.Fatalf("inner content removal interpreted as page removal: %s", message)
		}
	}
	if !assistantRevisionQuestion(domain.AssistantSubmit{Message: "解释第三页为什么这样写"}) || assistantRevisionQuestion(domain.AssistantSubmit{Message: "修改第三页，解释得更清楚"}) {
		t.Fatal("questions and edits confused")
	}
}

func TestAssistantArtifactOutlineOnlyContainsMetadata(t *testing.T) {
	a := revisionFixture()
	outline := assistantArtifactOutline(a)
	if len(outline) != 3 || outline[2].ID != "third" || outline[2].Position != 3 || outline[2].Unit != "页" {
		t.Fatal("outline order or system source isolation wrong")
	}
	a.Outline = outline
	data, _ := json.Marshal(a)
	if string(data) == "" || bytes.Contains(data, []byte("原始工单内容")) {
		t.Fatal("artifact metadata exposed private body")
	}
}

func TestAssistantRevisionGenerationKeepsTitleAndResumePostgres(t *testing.T) {
	u, db, _ := planningFixture(t)
	ctx := context.Background()
	r, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	base := revisionFixture()
	targets, positions, _ := assistantRevisionTargets(domain.AssistantSubmit{Message: "修改第三页"}, base)
	plan := &domain.AssistantPlan{Format: "pptx", Title: "合成介绍", Sufficient: true, Outline: []domain.AssistantOutlineItem{{ID: "third", Title: "闭环复盘"}}}
	r.Checkpoint["planning"] = plan
	r.Checkpoint["editing_parts"] = domain.JSONB{"third": targets[0]}
	r.Checkpoint["revision_positions"] = positions
	cm := &scriptedPlanningChat{replies: []*schema.Message{partResponse("已改为实际处理闭环", "保留工单追踪，精简处理说明。")}}
	partial, _, e := u.generateAssistantParts(ctx, r, cm, nil, plan)
	if e != nil || assistantArray(partial["slides"])[0]["title"] != "已改为实际处理闭环" {
		t.Fatal("model's requested title edit overwritten", e)
	}
	if e = db.First(r, "id=?", r.ID).Error; e != nil {
		t.Fatal(e)
	}
	resume := &scriptedPlanningChat{}
	if _, _, e = u.generateAssistantParts(ctx, r, resume, nil, assistantPlan(r)); e != nil || resume.calls != 0 {
		t.Fatal("saved edit unnecessarily regenerated", e)
	}
	progress := assistantGenerationProgress(r)
	if progress == nil || progress.Parts[0].Position != 3 {
		t.Fatal("edit progress lost original page number")
	}
}

func TestAssistantArtifactQuestionDoesNotGenerateFilePostgres(t *testing.T) {
	u, db, _ := planningFixture(t)
	ctx := context.Background()
	r, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	base := revisionFixture()
	before, _ := json.Marshal(base.Spec)
	cm := &scriptedPlanningChat{replies: []*schema.Message{{Content: `{"format":"text","answer":"第三页说明管理者如何复盘闭环处理。"}`}}}
	if e = u.answerAssistantArtifact(ctx, r, domain.AssistantSubmit{Message: "解释第三页"}, base, cm, &domain.Model{}); e != nil {
		t.Fatal(e)
	}
	var current domain.AssistantRun
	if e = db.First(&current, "id=?", r.ID).Error; e != nil {
		t.Fatal(e)
	}
	if current.State != "completed" || current.ArtifactID != "" || current.Answer != "第三页说明管理者如何复盘闭环处理。" {
		t.Fatal("file explanation was turned into a new artifact")
	}
	after, _ := json.Marshal(base.Spec)
	if string(before) != string(after) {
		t.Fatal("answering changed the file")
	}
}

func TestAssistantArtifactOutlineDeniedAfterRevocationPostgres(t *testing.T) {
	u, db, r := planningFixture(t)
	ctx := context.Background()
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: r.KBID, UserID: r.UserID, Name: "合成正文.txt", Format: "txt", Status: "ready", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e := db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	a := revisionFixture()
	a.ID, a.RunID, a.KBID, a.UserID, a.FamilyID, a.Revision, a.ExpiresAt = uuid.NewString(), r.ID, r.KBID, r.UserID, uuid.NewString(), 1, time.Now().Add(time.Hour)
	a.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "upload", ID: upload.ID, Name: upload.Name}})
	if e := db.Create(a).Error; e != nil {
		t.Fatal(e)
	}
	current, e := u.Artifact(ctx, r.KBID, r.UserID, a.ID)
	if e != nil || len(current.Outline) != 3 {
		t.Fatal("authorized structure not returned", e)
	}
	if _, e = u.Artifact(ctx, r.KBID, uuid.NewString(), a.ID); e == nil {
		t.Fatal("another user read the structure")
	}
	if e = db.Model(&upload).Update("status", "deleted").Error; e != nil {
		t.Fatal(e)
	}
	if _, e = u.Artifact(ctx, r.KBID, r.UserID, a.ID); e == nil {
		t.Fatal("revoked source still exposed artifact structure")
	}
}
