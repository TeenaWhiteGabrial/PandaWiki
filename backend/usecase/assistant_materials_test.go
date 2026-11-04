package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func TestAssistantMaterialCoverageIsActualRead(t *testing.T) {
	small := strings.Repeat("可用的短文正文。", 60)
	large := strings.Repeat("背景说明", 4000) + "目标能力：异常触发工单，维修人员闭环处理。" + strings.Repeat("附录", 4000)
	inputs := []assistantMaterialInput{{Source: domain.AssistantSource{Kind: "node", ID: "small", Version: "v1"}, Text: small, Header: "小文档\n"}, {Source: domain.AssistantSource{Kind: "node", ID: "large", Version: "v2"}, Text: large, Header: "大文档\n", Hints: []string{"目标能力：异常触发工单"}}}
	text, coverage := prepareAssistantMaterials(inputs, 8000)
	if len([]rune(text)) > 8000 || !strings.Contains(text, "目标能力：异常触发工单") {
		t.Fatal("budget or relevant text lost")
	}
	if read := coverage[assistantSourceKey(inputs[0].Source)]; read.Mode != "full" || read.Characters != assistantTextLength(small) {
		t.Fatal("small source not fully read")
	}
	if read := coverage[assistantSourceKey(inputs[1].Source)]; read.Mode != "excerpt" || read.Characters >= read.Total || read.Characters == 0 {
		t.Fatalf("excerpt called full: %+v", read)
	}
	if !strings.Contains(text, "不能宣称完整覆盖") {
		t.Fatal("partial reading was hidden")
	}
}

func TestAssistantLegacyMetadataCannotQualifyAsFullText(t *testing.T) {
	for _, release := range []*domain.NodeRelease{
		{Content: "这是一个文件型文档。系统当前已索引该文件的基础元信息，可用于按名称、路径、类型检索。" + strings.Repeat("资料名称：合成产品介绍。", 30)},
		{Content: "合成演示视频", Meta: domain.NodeMeta{FileKey: "synthetic/video", FileType: "mp4"}},
	} {
		if !assistantReleaseMetadataOnly(release) {
			t.Fatal("legacy filename-only material treated as body facts")
		}
	}
	if assistantReleaseMetadataOnly(&domain.NodeRelease{Content: "正常的正文资料", Meta: domain.NodeMeta{FileKey: "synthetic/manual", FileType: "pdf"}}) {
		t.Fatal("unknown extraction report alone is not proof of missing body")
	}
}

func TestAssistantReadsPinnedChapterEvidencePostgres(t *testing.T) {
	u, db, r := planningFixture(t)
	ctx := context.Background()
	n := domain.Node{ID: uuid.NewString(), KBID: r.KBID, Name: "草稿标题", Type: domain.NodeTypeDocument}
	if e := db.Create(&n).Error; e != nil {
		t.Fatal(e)
	}
	body := strings.Repeat("背景说明。", 50) + "关键事实：设备告警会转成可追踪工单。"
	release := domain.NodeRelease{ID: uuid.NewString(), NodeID: n.ID, KBID: r.KBID, Name: "发布标题", Type: domain.NodeTypeDocument, Content: body}
	if e := db.Create(&release).Error; e != nil {
		t.Fatal(e)
	}
	source := domain.AssistantSource{Kind: "node", ID: n.ID, Version: release.ID, Name: release.Name, Hash: digest([]byte(body))}
	r.Sources = sourcesJSON([]domain.AssistantSource{source})
	part := domain.AssistantOutlineItem{Title: "异常闭环", SourceIDs: []string{assistantSourceKey(source)}}
	text, e := u.assistantPartMaterials(ctx, r, part, 5000)
	if e != nil || !strings.Contains(text, "关键事实：设备告警") || strings.Contains(text, "草稿标题") {
		t.Fatal("pinned published evidence missing", e)
	}
	if duplicate, e := u.assistantPartMaterials(ctx, r, part, 5000, "当前请求中的已授权资料："+body); e != nil || duplicate != "" {
		t.Fatal("complete source was appended twice", e)
	}
	if additional, e := u.assistantPartMaterials(ctx, r, part, 5000, "背景说明。"); e != nil || !strings.Contains(additional, "关键事实：设备告警") {
		t.Fatal("partial input suppressed needed evidence", e)
	}
	if content, e := u.assistantPartMaterials(ctx, r, part, 0); e != nil || content != "" {
		t.Fatal("editing-only path appended source body", e)
	}
	r.Checkpoint["material_coverage_version"] = 1
	r.Checkpoint["material_coverage"] = map[string]assistantMaterialCoverage{assistantSourceKey(source): {Characters: 5, Total: assistantTextLength(body), Mode: "excerpt"}}
	reviews, e := u.materialReviews(ctx, r)
	if e != nil || reviews[0].Characters != 5 || reviews[0].TotalCharacters <= 5 {
		t.Fatal("audit pretended the whole document was provided", e)
	}
	plan := domain.AssistantPlan{Title: "介绍", Sufficient: true, Materials: reviews, Outline: []domain.AssistantOutlineItem{{Title: "闭环", SourceIDs: []string{assistantSourceKey(source)}}}}
	if e = normalizeAssistantPlan(&plan, r, false); e != nil || plan.Sufficient {
		t.Fatal("five supplied characters passed as sufficient facts", e)
	}
	if e = db.Model(&release).Update("content", body+"内容已变化").Error; e != nil {
		t.Fatal(e)
	}
	if _, e = u.assistantPartMaterials(ctx, r, part, 5000); e == nil {
		t.Fatal("mixed changed text into an old pinned draft")
	}
	if _, e = u.assistantPartMaterials(ctx, r, part, 5000, body+"内容已变化"); e == nil {
		t.Fatal("deduplication bypassed pinned hash check")
	}
	if _, e = u.assistantPartMaterials(ctx, r, part, 0); e == nil {
		t.Fatal("editing-only path bypassed pinned source hash")
	}
}

func TestAssistantTemplateOnlyAppliesToPPTPostgres(t *testing.T) {
	u, db, r := planningFixture(t)
	ctx := context.Background()
	if e := u.CancelRun(ctx, r.KBID, r.UserID, r.ID); e != nil {
		t.Fatal(e)
	}
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: r.KBID, UserID: r.UserID, Name: "模板.pptx", Format: "pptx", Status: "ready", Hash: "synthetic", TemplateKey: "never-read", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e := db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	template := domain.AssistantTemplate{ID: uuid.NewString(), KBID: r.KBID, Name: "默认模板", UploadID: upload.ID, Version: 1, State: "published", IsDefault: true, Permissions: domain.NodePermissions{}}
	if e := db.Create(&template).Error; e != nil {
		t.Fatal(e)
	}
	ref := domain.AssistantTemplateRef{Kind: "enterprise", ID: template.ID, Version: 1}
	if _, e := u.Preferences(ctx, r.KBID, r.UserID, r.ConversationID, func() *domain.JSONB { v := assistantJSON(map[string]any{"template_ref": ref}); return &v }()); e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"docx", "xlsx"} {
		item, e := u.Submit(ctx, r.KBID, r.UserID, "", domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: r.ConversationID, Message: "生成文件", Format: format, TemplateRef: &domain.AssistantTemplateRef{Kind: "enterprise", ID: "irrelevant-invalid-template", Version: 1}, SourceScope: domain.AssistantScope{Type: "documents"}})
		if e != nil {
			t.Fatal("unrelated PPT template blocked another format", e)
		}
		var req domain.AssistantSubmit
		_ = assistantDecode(item.Request, &req)
		if req.TemplateRef != nil {
			t.Fatal("PPT template included in a Word/Excel task")
		}
		pref, e := u.Preferences(ctx, r.KBID, r.UserID, r.ConversationID, nil)
		if e != nil {
			t.Fatal(e)
		}
		var current domain.AssistantTemplateRef
		_ = decodeAssistantValue(pref["template_ref"], &current)
		if current.ID != ref.ID {
			t.Fatal("Word/Excel erased the user's PPT preference")
		}
		if e = u.CancelRun(ctx, r.KBID, r.UserID, item.ID); e != nil {
			t.Fatal(e)
		}
	}
	item, e := u.Submit(ctx, r.KBID, r.UserID, "", domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: r.ConversationID, Message: "生成PPT", Format: "pptx", SourceScope: domain.AssistantScope{Type: "documents"}})
	if e != nil {
		t.Fatal(e)
	}
	var req domain.AssistantSubmit
	_ = assistantDecode(item.Request, &req)
	if req.TemplateRef == nil || req.TemplateRef.ID != template.ID {
		t.Fatal("default PPT template stopped working")
	}
}
