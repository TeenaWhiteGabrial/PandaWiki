package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type planningChat struct {
	plan  domain.AssistantPlan
	calls int
}

func (m *planningChat) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	m.calls++
	data, _ := json.Marshal(m.plan)
	return schema.AssistantMessage(string(data), nil), nil
}
func (m *planningChat) Stream(context.Context, []*schema.Message, ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not used")
}

func planningFixture(t *testing.T) (*AssistantUsecase, *store.DB, *domain.AssistantRun) {
	t.Helper()
	db, logger := chatContextDB(t)
	kb := domain.KnowledgeBase{ID: uuid.NewString(), Name: "synthetic planning fixture", DatasetID: uuid.NewString()}
	owner := domain.User{ID: uuid.NewString(), Account: "plan_" + uuid.NewString(), Role: consts.UserRoleAdmin}
	if e := db.Create(&kb).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Create(&owner).Error; e != nil {
		t.Fatal(e)
	}
	chat := &ChatUsecase{appRepo: pg.NewAppRepository(db, logger), nodeRepo: pg.NewNodeRepository(db, logger), enterpriseUsecase: &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, logger), nodeRepo: pg.NewNodeRepository(db, logger), navRepo: pg.NewNavRepository(db, logger)}, conversationUsecase: &ConversationUsecase{repo: pg.NewConversationRepository(db, logger)}}
	u := &AssistantUsecase{db: db.DB, cfg: &config.Config{Assistant: config.AssistantConfig{RetentionDays: 30}}, chat: chat, logger: logger}
	run, e := u.Submit(context.Background(), kb.ID, owner.ID, "127.0.0.1", domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "生成客户演示PPT，先给我看大纲，等我确认", SourceScope: domain.AssistantScope{Type: "documents"}})
	if e != nil {
		t.Fatal(e)
	}
	return u, db, run
}

func TestAssistantPlanningGatesAndClarification(t *testing.T) {
	source := domain.AssistantMaterialReview{ID: "node:test@release", Status: "已发布正文", Characters: 100}
	base := func() domain.AssistantPlan {
		return domain.AssistantPlan{Title: "合成产品介绍", Sufficient: true, Materials: []domain.AssistantMaterialReview{source}, Outline: []domain.AssistantOutlineItem{{Title: "核心价值", SourceIDs: []string{source.ID}}}}
	}
	run := &domain.AssistantRun{Checkpoint: domain.JSONB{}}
	t.Run("metadata cannot qualify a formal deliverable", func(t *testing.T) {
		p := base()
		p.Materials[0].Status = "仅文件元信息"
		p.Materials[0].Characters = 12
		if e := normalizeAssistantPlan(&p, run, false); e != nil || p.Sufficient {
			t.Fatalf("metadata passed: %+v %v", p, e)
		}
	})
	t.Run("unknown citation rejected", func(t *testing.T) {
		p := base()
		p.Outline[0].SourceIDs = []string{"invented"}
		if normalizeAssistantPlan(&p, run, false) == nil {
			t.Fatal("invented evidence accepted")
		}
	})
	t.Run("uncited outline blocks delivery", func(t *testing.T) {
		p := base()
		p.Outline[0].SourceIDs = nil
		if e := normalizeAssistantPlan(&p, run, false); e != nil || p.Sufficient {
			t.Fatalf("uncited outline passed: %+v %v", p, e)
		}
	})
	t.Run("at most two questions and no already answered IDs", func(t *testing.T) {
		p := base()
		for _, id := range []string{"audience", "goal", "length", "style", "budget"} {
			p.Questions = append(p.Questions, domain.AssistantPlanQuestion{ID: id, Question: id, Default: "default"})
		}
		r := &domain.AssistantRun{Checkpoint: domain.JSONB{"answers": map[string]string{"audience": "客户"}}}
		if e := normalizeAssistantPlan(&p, r, false); e != nil || len(p.Questions) != 2 || p.Questions[0].ID == "audience" {
			t.Fatalf("bad questions: %+v %v", p, e)
		}
		r.Checkpoint["clarification_rounds"] = 2
		if e := normalizeAssistantPlan(&p, r, false); e != nil || len(p.Questions) != 0 {
			t.Fatal("clarification loop did not end")
		}
	})
	t.Run("editing cannot replace source provenance", func(t *testing.T) {
		p := base()
		p.Outline[0].ID = "part-1"
		items := []domain.AssistantOutlineItem{{ID: "part-1", Title: "用户修改标题", SourceIDs: []string{"foreign"}}}
		if validateEditedOutline(items, &p) == nil {
			t.Fatal("source replacement accepted")
		}
		items[0].SourceIDs = []string{source.ID}
		if e := validateEditedOutline(items, &p); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("model cannot silently change planned format or page count", func(t *testing.T) {
		p := base()
		p.Format = "pptx"
		for _, spec := range []domain.JSONB{{"format": "text"}, {"format": "pptx", "slides": []any{}}, {"format": "pptx", "title": "产品资料待补充", "slides": []any{map[string]any{"title": "a"}}}} {
			if validateAssistantPlannedSpec(spec, &p) == nil {
				t.Fatalf("invalid spec passed: %+v", spec)
			}
		}
	})
}

func TestAssistantPlanningCompactSourceIDsPostgres(t *testing.T) {
	u, _, r := planningFixture(t)
	ctx := context.Background()
	r.State = "running"
	r.LeaseEpoch++
	until := time.Now().Add(time.Minute)
	r.LeaseUntil = &until
	if e := u.db.Model(&domain.AssistantRun{}).Where("id=?", r.ID).Updates(map[string]any{"state": r.State, "lease_epoch": r.LeaseEpoch, "lease_until": until}).Error; e != nil {
		t.Fatal(e)
	}
	facts := strings.Repeat("有依据的合成产品巡检与工单闭环功能。", 5)
	r.Checkpoint["user_facts"] = facts
	source := domain.AssistantSource{Kind: "user", ID: r.ID, Version: digest([]byte(facts)), Name: "合成事实"}
	r.Sources = sourcesJSON([]domain.AssistantSource{source})
	model := &planningChat{plan: domain.AssistantPlan{Title: "巡检介绍", Sufficient: true, Outline: []domain.AssistantOutlineItem{{Title: "巡检与工单", Purpose: "介绍检查记录如何进入异常处理流程", SourceIDs: []string{"S1"}}}}}
	ready, e := u.prepareAssistantPlan(ctx, r, domain.AssistantSubmit{Message: "直接制作一页介绍PPT"}, "pptx", facts, nil, nil, model)
	if e != nil || !ready {
		t.Fatal("compact source plan rejected", e)
	}
	p := assistantPlan(r)
	if p.Outline[0].SourceIDs[0] != assistantSourceKey(source) || p.Materials[0].ID != assistantSourceKey(source) {
		t.Fatal("model alias escaped into stored provenance")
	}
	compact, material, aliases := assistantPlanningSourceAliases(p.Materials, "资料ID："+assistantSourceKey(source)+"\n正文S1是业务代号")
	if compact[0].ID != "S1" || aliases["S1"] != assistantSourceKey(source) || !strings.Contains(material, "正文S1是业务代号") || p.Materials[0].ID == "S1" {
		t.Fatal("alias mapping changed canonical source or body")
	}
	model.plan.Outline[0].SourceIDs = []string{"S999"}
	delete(r.Checkpoint, "outline_confirmed")
	if _, e = u.prepareAssistantPlan(ctx, r, domain.AssistantSubmit{Message: "直接生成"}, "pptx", facts, nil, nil, model); e == nil {
		t.Fatal("invented source alias accepted")
	}
}

func TestAssistantPlanningContinuationPostgres(t *testing.T) {
	u, db, run := planningFixture(t)
	ctx := context.Background()
	claim, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var req domain.AssistantSubmit
	_ = assistantDecode(claim.Request, &req)
	model := &planningChat{plan: domain.AssistantPlan{Title: "合成客户演示", Questions: []domain.AssistantPlanQuestion{{ID: "audience", Question: "客户对象", Default: "企业业务负责人"}, {ID: "goal", Question: "演示目的", Default: "方案价值介绍"}}}}
	ready, e := u.prepareAssistantPlan(ctx, claim, req, "pptx", "", nil, nil, model)
	if e != nil || ready {
		t.Fatalf("should pause: %v %v", ready, e)
	}
	current, e := u.Run(ctx, run.KBID, run.UserID, run.ID)
	if e != nil || current.State != "awaiting_input" || current.Planning == nil || current.LeaseUntil != nil {
		t.Fatalf("plan not durably paused: %+v %v", current, e)
	}
	if _, e = u.claimRun(ctx); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatalf("waiting run was reclaimed: %v", e)
	}
	other := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: run.ConversationID, Message: "另一个文件", SourceScope: domain.AssistantScope{Type: "documents"}}
	if _, e = u.Submit(ctx, run.KBID, run.UserID, "", other); e == nil {
		t.Fatal("new task bypassed pending clarification")
	}
	answer := domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: current.Planning.Revision, Action: "answer", UseDefaults: true}
	for _, pair := range [][2]string{{uuid.NewString(), run.UserID}, {run.KBID, uuid.NewString()}} {
		if _, e = u.ContinueRun(ctx, pair[0], pair[1], run.ID, answer); !errors.Is(e, ErrAssistantAccess) {
			t.Fatal("continuation crossed ownership")
		}
	}
	current, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, answer)
	if e != nil || current.State != "queued" {
		t.Fatalf("resume: %+v %v", current, e)
	}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, answer); e != nil {
		t.Fatal("idempotent retry failed", e)
	}
	var messages []domain.ConversationMessage
	if e = db.Where("conversation_id=?", run.ConversationID).Find(&messages).Error; e != nil || len(messages) != 4 {
		t.Fatalf("continuation history lost or duplicated: %d %v", len(messages), e)
	}
	answer.Message = "changed"
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, answer); e == nil {
		t.Fatal("idempotent request accepted different body")
	}
	claim, e = u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var answers map[string]string
	_ = decodeAssistantValue(claim.Checkpoint["answers"], &answers)
	if answers["audience"] == "" || answers["goal"] == "" {
		t.Fatal("answers not persisted")
	}
	model.plan.Questions = nil
	model.plan.Gaps = []string{"缺少合成产品的能力与价值说明"}
	model.plan.Sufficient = false
	ready, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", "", nil, nil, model)
	if e != nil || ready {
		t.Fatal("insufficient material passed", e)
	}
	current, _ = u.Run(ctx, run.KBID, run.UserID, run.ID)
	if current.State != "awaiting_sources" {
		t.Fatal(current.State)
	}
	supplement := domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: current.Planning.Revision, Action: "supplement", Message: strings.Repeat("合成产品面向企业设备管理，提供巡检与工单闭环，资料没有性能参数或客户案例。", 3)}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, supplement); e != nil {
		t.Fatal(e)
	}
	claim, e = u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	facts := assistantString(claim.Checkpoint["user_facts"])
	source := domain.AssistantSource{Kind: "user", ID: claim.ID, Version: digest([]byte(facts)), Name: "用户补充说明"}
	claim.Sources = sourcesJSON([]domain.AssistantSource{source})
	model.plan.Sufficient = true
	model.plan.Gaps = nil
	model.plan.Outline = []domain.AssistantOutlineItem{{Title: "巡检工单闭环", Purpose: "展示合成产品的闭环流程", SourceIDs: []string{assistantSourceKey(source)}}}
	ready, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", facts, nil, nil, model)
	if e != nil || ready {
		t.Fatal("outline was not gated", e)
	}
	current, _ = u.Run(ctx, run.KBID, run.UserID, run.ID)
	if current.State != "awaiting_outline" {
		t.Fatalf("bad state %s", current.State)
	}
	var artifacts int64
	db.Model(&domain.AssistantArtifact{}).Where("run_id=?", run.ID).Count(&artifacts)
	if artifacts != 0 {
		t.Fatal("artifact published before confirmation")
	}
	edited := append([]domain.AssistantOutlineItem{}, current.Planning.Outline...)
	edited[0].Title = "用户确认的核心价值"
	confirmation := domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: current.Planning.Revision, Action: "confirm_outline", Outline: edited}
	stale := confirmation
	stale.Revision--
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, stale); e == nil {
		t.Fatal("stale outline accepted")
	}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, confirmation); e != nil {
		t.Fatal(e)
	}
	claim, e = u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	beforeCalls := model.calls
	ready, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", "new search must not replace approved material", nil, nil, model)
	if e != nil || !ready || model.calls != beforeCalls {
		t.Fatal("confirmed outline replanned", e)
	}
	if assistantPlan(claim).Outline[0].Title != edited[0].Title || assistantString(claim.Checkpoint["planning_materials"]) != facts {
		t.Fatal("approved outline or material snapshot lost")
	}
	if e = u.CancelRun(ctx, run.KBID, run.UserID, run.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, domain.AssistantContinue{ClientRequestID: uuid.NewString(), Action: "revise", Revision: confirmation.Revision, Message: "late edit"}); e == nil {
		t.Fatal("canceled task resumed")
	}
}

func TestAssistantPlanningRevokedSourcePostgres(t *testing.T) {
	u, db, run := planningFixture(t)
	ctx := context.Background()
	claim, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: run.KBID, UserID: run.UserID, Name: "synthetic.txt", Format: "txt", Status: "ready", Hash: "test", ObjectKey: "test", Text: strings.Repeat("测试资料", 30), Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e = db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	source := domain.AssistantSource{Kind: "upload", ID: upload.ID, Name: upload.Name}
	claim.Sources = sourcesJSON([]domain.AssistantSource{source})
	model := &planningChat{plan: domain.AssistantPlan{Title: "测试", Sufficient: true, Outline: []domain.AssistantOutlineItem{{Title: "测试能力", SourceIDs: []string{assistantSourceKey(source)}}}}}
	var req domain.AssistantSubmit
	_ = assistantDecode(claim.Request, &req)
	if _, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", upload.Text, nil, nil, model); e != nil {
		t.Fatal(e)
	}
	if e = db.Model(&upload).Update("status", "deleted").Error; e != nil {
		t.Fatal(e)
	}
	current, e := u.Run(ctx, run.KBID, run.UserID, run.ID)
	if e != nil || current.Planning != nil || len(current.References) > 0 {
		t.Fatal("revoked plan was exposed")
	}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: claim.LeaseEpoch, Action: "confirm_outline", Outline: model.plan.Outline}); !errors.Is(e, ErrAssistantAccess) {
		t.Fatal("revoked plan was confirmed", e)
	}
	if e = u.CancelRun(ctx, run.KBID, run.UserID, run.ID); e != nil {
		t.Fatal(e)
	}
}

func TestAssistantPlanningFrameworkAndRetryPostgres(t *testing.T) {
	u, db, run := planningFixture(t)
	ctx := context.Background()
	claim, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var req domain.AssistantSubmit
	_ = assistantDecode(claim.Request, &req)
	model := &planningChat{plan: domain.AssistantPlan{Title: "合成产品演示", Sufficient: false, Gaps: []string{"没有正文"}, Outline: []domain.AssistantOutlineItem{{Title: "产品价值", Purpose: "需用户补充产品价值"}}}}
	if _, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", "", nil, nil, model); e != nil {
		t.Fatal(e)
	}
	current, _ := u.Run(ctx, run.KBID, run.UserID, run.ID)
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: current.Planning.Revision, Action: "confirm_outline", Outline: current.Planning.Outline}); e == nil {
		t.Fatal("formal outline confirmation bypassed material gate")
	}
	if _, e = u.ContinueRun(ctx, run.KBID, run.UserID, run.ID, domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: current.Planning.Revision, Action: "framework"}); e != nil {
		t.Fatal(e)
	}
	claim, e = u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = u.prepareAssistantPlan(ctx, claim, req, "pptx", "", nil, nil, model); e != nil {
		t.Fatal(e)
	}
	current, _ = u.Run(ctx, run.KBID, run.UserID, run.ID)
	if current.State != "awaiting_outline" || !current.Planning.FrameworkOnly || !strings.Contains(current.Planning.Title, "框架草稿") {
		t.Fatal("draft did not require labelled outline confirmation")
	}
	spec := domain.JSONB{"format": "pptx", "title": "合成产品", "answer": "已完成", "slides": []any{map[string]any{"title": "产品价值"}}}
	if e = validateAssistantPlannedSpec(spec, current.Planning); e != nil || !strings.Contains(assistantString(spec["title"]), "框架草稿") || !strings.Contains(assistantString(spec["answer"]), "框架草稿") {
		t.Fatal("draft label not enforced", e)
	}
	// A retry cannot bypass a waiting task in the same conversation.
	old := domain.AssistantRun{ID: uuid.NewString(), KBID: run.KBID, UserID: run.UserID, ConversationID: run.ConversationID, UserMessageID: uuid.NewString(), AssistantMessageID: uuid.NewString(), RequestID: uuid.NewString(), RequestHash: "test", Request: domain.JSONB{}, State: "failed", Stage: "failed", Checkpoint: domain.JSONB{}, Sources: domain.JSONB{}, Steps: domain.JSONB{}, StageStartedAt: time.Now()}
	if e = db.Create(&old).Error; e != nil {
		t.Fatal(e)
	}
	if e = u.RetryRun(ctx, run.KBID, run.UserID, old.ID); e == nil {
		t.Fatal("retry bypassed pending outline")
	}
	if e = u.CancelRun(ctx, run.KBID, run.UserID, run.ID); e != nil {
		t.Fatal(e)
	}
	if e = u.RetryRun(ctx, run.KBID, run.UserID, old.ID); e != nil {
		t.Fatal(e)
	}
}

func TestAssistantPlannedRevisionSourcePage(t *testing.T) {
	plan := &domain.AssistantPlan{Format: "pptx", Outline: []domain.AssistantOutlineItem{{Title: "能力"}}}
	spec := domain.JSONB{"format": "pptx", "title": "合成产品", "slides": []any{map[string]any{"title": "能力", "id": "slide-1"}, map[string]any{"title": "资料来源", "id": "system-sources"}}}
	if e := validateAssistantPlannedSpec(spec, plan); e != nil {
		t.Fatal("existing system source page counted as content", e)
	}
}

func TestAssistantFileFormatIntent(t *testing.T) {
	for _, tc := range []struct{ message, want string }{{"生成客户介绍ppt", "pptx"}, {"帮我做一个word方案", "docx"}, {"导出Excel汇总", "xlsx"}, {"总结上传的PPT内容", ""}, {"解释Word和Excel的区别", ""}} {
		if got := assistantFileFormat(domain.AssistantSubmit{Message: tc.message}, nil); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.message, got, tc.want)
		}
	}
}
