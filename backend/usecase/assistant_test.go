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
	"github.com/google/uuid"
)

func TestAssistantDurableRunIsolationPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	ctx := context.Background()
	kb := domain.KnowledgeBase{ID: uuid.NewString(), Name: "assistant synthetic test", DatasetID: uuid.NewString()}
	if e := db.Create(&kb).Error; e != nil {
		t.Fatal(e)
	}
	owner := domain.User{ID: uuid.NewString(), Account: "assistant_test_" + uuid.NewString(), Role: consts.UserRoleAdmin}
	if e := db.Create(&owner).Error; e != nil {
		t.Fatal(e)
	}
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, logger), nodeRepo: pg.NewNodeRepository(db, logger), navRepo: pg.NewNavRepository(db, logger)}
	conv := &ConversationUsecase{repo: pg.NewConversationRepository(db, logger)}
	chat := &ChatUsecase{appRepo: pg.NewAppRepository(db, logger), conversationUsecase: conv, enterpriseUsecase: enterprise, nodeRepo: pg.NewNodeRepository(db, logger)}
	u := &AssistantUsecase{db: db.DB, cfg: &config.Config{Assistant: config.AssistantConfig{RetentionDays: 30}}, chat: chat, logger: logger}
	req := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "根据上传资料生成PPT", SourceScope: domain.AssistantScope{Type: "documents"}}
	r, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", req)
	if e != nil {
		t.Fatal(e)
	}
	if r.SessionNonce == "" || r.ConversationID == "" {
		t.Fatal("accepted task did not persist recoverable conversation")
	}
	duplicate, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", req)
	if e != nil || duplicate.ID != r.ID {
		t.Fatalf("duplicate request created another task: %+v %v", duplicate, e)
	}
	changed := req
	changed.Message = "不同内容"
	if _, e = u.Submit(ctx, kb.ID, owner.ID, "", changed); e == nil {
		t.Fatal("idempotency key accepted a different request")
	}
	for _, kbUser := range [][2]string{{uuid.NewString(), owner.ID}, {kb.ID, uuid.NewString()}} {
		if _, e = u.Run(ctx, kbUser[0], kbUser[1], r.ID); !errors.Is(e, ErrAssistantAccess) {
			t.Fatal("run accessible across account/KB")
		}
		if e = u.CancelRun(ctx, kbUser[0], kbUser[1], r.ID); !errors.Is(e, ErrAssistantAccess) {
			t.Fatal("another user canceled run")
		}
	}
	claimed, e := u.claimRun(ctx)
	if e != nil || claimed.ID != r.ID || claimed.LeaseEpoch != 1 {
		t.Fatalf("claim failed: %+v %v", claimed, e)
	}
	if e = u.CancelRun(ctx, kb.ID, owner.ID, r.ID); e != nil {
		t.Fatal(e)
	}
	if e = u.publish(ctx, claimed, nil, "late private result"); e == nil {
		t.Fatal("canceled worker published late result")
	}
	final, e := u.Run(ctx, kb.ID, owner.ID, r.ID)
	if e != nil || final.State != "canceled" || final.Answer != "" {
		t.Fatalf("canceled task resurrected: %+v %v", final, e)
	}
	if e = u.RetryRun(ctx, kb.ID, owner.ID, r.ID); e == nil {
		t.Fatal("canceled task retried without explicit new request")
	}
}
func TestAssistantLeaseFencingAndProvenancePostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	ctx := context.Background()
	u := &AssistantUsecase{db: db.DB, logger: logger}
	r := domain.AssistantRun{ID: uuid.NewString(), KBID: uuid.NewString(), UserID: uuid.NewString(), ConversationID: uuid.NewString(), RequestID: uuid.NewString(), RequestHash: "test", Request: domain.JSONB{}, UserMessageID: uuid.NewString(), AssistantMessageID: uuid.NewString(), State: "queued", Stage: "等待处理", StageStartedAt: time.Now(), Checkpoint: domain.JSONB{}, Sources: domain.JSONB{}, Steps: domain.JSONB{}}
	if e := db.Create(&r).Error; e != nil {
		t.Fatal(e)
	}
	a, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Model(&r).Update("lease_until", time.Now().Add(-time.Minute)).Error; e != nil {
		t.Fatal(e)
	}
	b, e := u.claimRun(ctx)
	if e != nil || b.LeaseEpoch <= a.LeaseEpoch {
		t.Fatal("expired lease not replaced")
	}
	if e = u.stage(ctx, a, "迟到结果", nil); e == nil {
		t.Fatal("old worker updated a stage")
	}
	if e = u.publish(ctx, a, nil, "old answer"); e == nil {
		t.Fatal("old worker published answer")
	}
	if e = u.stage(ctx, b, "整理内容", nil); e != nil {
		t.Fatal(e)
	}
	// Reusing a deleted/expired upload cannot be made safe by removing visible citations.
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: r.KBID, UserID: r.UserID, Name: "private.txt", Format: "txt", Size: 3, Hash: "test", ObjectKey: "private", Status: "ready", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e = db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	b.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "upload", ID: upload.ID, Name: upload.Name}})
	if e = u.checkSources(ctx, r.KBID, r.UserID, b.Sources); e != nil {
		t.Fatal(e)
	}
	if e = db.Model(&upload).Update("status", "deleted").Error; e != nil {
		t.Fatal(e)
	}
	if e = u.publish(ctx, b, nil, "copied private text without a visible citation"); !errors.Is(e, ErrAssistantAccess) {
		t.Fatal("removed source was reused in output")
	}
}

func TestAssistantTemplateRightsAndPinnedVersionsPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	ctx := context.Background()
	kb := uuid.NewString()
	admin := domain.User{ID: uuid.NewString(), Account: "admin_" + uuid.NewString(), Role: consts.UserRoleAdmin}
	reader := domain.User{ID: uuid.NewString(), Account: "reader_" + uuid.NewString(), Role: consts.UserRoleUser}
	other := domain.User{ID: uuid.NewString(), Account: "other_" + uuid.NewString(), Role: consts.UserRoleUser}
	for _, v := range []*domain.User{&admin, &reader, &other} {
		if e := db.Create(v).Error; e != nil {
			t.Fatal(e)
		}
	}
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, logger)}
	u := &AssistantUsecase{db: db.DB, chat: &ChatUsecase{enterpriseUsecase: enterprise}}
	up := domain.AssistantUpload{ID: uuid.NewString(), KBID: kb, UserID: admin.ID, Name: "企业模板.pptx", Format: "pptx", Size: 99, Hash: "v1", ObjectKey: "raw", TemplateKey: "clean", Status: "ready", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(time.Hour)}
	if e := db.Create(&up).Error; e != nil {
		t.Fatal(e)
	}
	template, e := u.SaveTemplate(ctx, kb, admin.ID, domain.AssistantTemplate{Name: "企业模板", UploadID: up.ID, Confirmed: true, Permissions: domain.NodePermissions{ScopeType: "custom", AllowedUserIDs: []string{reader.ID}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = u.TemplateState(ctx, kb, template.ID, template.Version, "published", true); e != nil {
		t.Fatal(e)
	}
	list, e := u.Templates(ctx, kb, reader.ID, false)
	if e != nil || len(list) != 1 {
		t.Fatal("authorized reader could not list template", e)
	}
	hidden, e := u.Templates(ctx, kb, other.ID, false)
	if e != nil || len(hidden) != 0 {
		t.Fatal("template visible outside audience", e)
	}
	ref := domain.AssistantTemplateRef{Kind: "enterprise", ID: template.ID, Version: template.Version}
	if _, e = u.template(ctx, kb, reader.ID, ref); e != nil {
		t.Fatal(e)
	}
	if _, e = u.GetUpload(ctx, kb, reader.ID, up.ID); !errors.Is(e, ErrAssistantAccess) {
		t.Fatal("template use granted raw personal-upload access")
	}
	if e = u.TemplateState(ctx, kb, template.ID, template.Version, "stopped", false); e != nil {
		t.Fatal(e)
	}
	if _, e = u.template(ctx, kb, reader.ID, ref); e != nil {
		t.Fatal("stopping invalidated pinned task", e)
	}
	if e = u.TemplatePermissions(ctx, kb, template.ID, domain.NodePermissions{ScopeType: "custom", AllowedUserIDs: []string{other.ID}}); e != nil {
		t.Fatal(e)
	}
	source := sourcesJSON([]domain.AssistantSource{{Kind: "template", ID: template.ID, Version: `{"kind":"enterprise","id":"` + template.ID + `","version":1}`}})
	if e = u.checkSources(ctx, kb, reader.ID, source); !errors.Is(e, ErrAssistantAccess) {
		t.Fatal("revoked template still usable by history/derivatives")
	}
}

func TestAssistantSpreadsheetDataReadProgrammaticallyPostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	u := &AssistantUsecase{db: db.DB, logger: logger}
	ctx := context.Background()
	r := domain.AssistantRun{KBID: uuid.NewString(), UserID: uuid.NewString()}
	upload := domain.AssistantUpload{ID: uuid.NewString(), KBID: r.KBID, UserID: r.UserID, Name: "data.xlsx", Format: "xlsx", Status: "ready", Size: 20, Hash: "test", ObjectKey: "test", Report: domain.JSONB{}, Text: `[{"name":"销售","rows":[["部门","金额"],["研发",3],["研发",5],["市场",7]]}]`, ExpiresAt: time.Now().Add(time.Hour)}
	if e := db.Create(&upload).Error; e != nil {
		t.Fatal(e)
	}
	r.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "upload", ID: upload.ID}})
	spec := domain.JSONB{"sheets": []any{map[string]any{"name": "统计", "source": map[string]any{"upload_id": upload.ID, "sheet": "销售", "operation": "sum", "group_by": "部门", "value_column": "金额"}}}}
	if e := u.resolveWorkbook(ctx, &r, spec); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(spec)
	if !strings.Contains(string(b), `["研发",8]`) || !strings.Contains(string(b), `["市场",7]`) {
		t.Fatalf("numeric aggregation did not use original rows: %s", b)
	}
	r.Sources = domain.JSONB{}
	if e := u.resolveWorkbook(ctx, &r, spec); e == nil {
		t.Fatal("model selected an upload not provided in context")
	}
}

func TestAssistantEndPausedRunPostgres(t *testing.T) {
	u, db, run := planningFixture(t)
	originalError := "model output truncated: preserved for administrators"
	if e := db.Model(&domain.AssistantRun{}).Where("id=?", run.ID).Updates(map[string]any{"state": "failed", "error": originalError}).Error; e != nil {
		t.Fatal(e)
	}
	other := *run
	other.ID = uuid.NewString()
	other.RequestID = uuid.NewString()
	other.State = "failed"
	other.Error = "other task diagnostic"
	if e := db.Create(&other).Error; e != nil {
		t.Fatal(e)
	}
	if e := u.CancelRun(context.Background(), run.KBID, run.UserID, run.ID); e != nil {
		t.Fatal(e)
	}
	final, e := u.Run(context.Background(), run.KBID, run.UserID, run.ID)
	if e != nil || final.State != "canceled" || final.Error != originalError {
		t.Fatalf("paused task did not end with diagnostics retained: %+v %v", final, e)
	}
	unchanged, e := u.Run(context.Background(), run.KBID, run.UserID, other.ID)
	if e != nil || unchanged.State != "failed" {
		t.Fatal("ending one paused task changed another task", e)
	}
	if e = u.RetryRun(context.Background(), run.KBID, run.UserID, run.ID); e == nil {
		t.Fatal("ended task was resurrected")
	}
}
