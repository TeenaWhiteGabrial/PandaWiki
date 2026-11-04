package usecase

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// Uses only an explicitly configured local generator, synthetic documents, a
// rollback-only test DB and an in-memory S3 test server. Never writes real OSS.
func TestAssistantOfficePipelinePostgres(t *testing.T) {
	token := os.Getenv("PANDA_WIKI_TEST_GENERATOR_TOKEN")
	if token == "" {
		t.Skip("set local generator token for real Office pipeline test")
	}
	db, logger := chatContextDB(t)
	ctx := context.Background()
	objects := map[string][]byte{}
	var mu sync.Mutex
	objectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fixture/" || r.URL.Path == "/fixture" {
			w.WriteHeader(200)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/fixture/static-file/") {
			t.Errorf("unexpected object path %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "PUT":
			if r.Header.Get("X-Amz-Acl") != "private" {
				t.Error("assistant object was not written private")
			}
			b, e := io.ReadAll(r.Body)
			if e != nil {
				t.Error(e)
			}
			if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") || bytes.Contains(b[:min(100, len(b))], []byte(";chunk-signature=")) {
				input := bufio.NewReader(bytes.NewReader(b))
				var decoded bytes.Buffer
				for {
					line, er := input.ReadString('\n')
					if er != nil {
						t.Error(er)
						return
					}
					n, er := strconv.ParseInt(strings.Split(strings.TrimSpace(line), ";")[0], 16, 64)
					if er != nil {
						t.Error(er)
						return
					}
					if n == 0 {
						break
					}
					if _, er = io.CopyN(&decoded, input, n); er != nil {
						t.Error(er)
						return
					}
					_, _ = input.ReadString('\n')
				}
				b = decoded.Bytes()
			}
			objects[r.URL.Path] = b
			w.Header().Set("ETag", `"900150983cd24fb0d6963f7d28e17f72"`)
			w.WriteHeader(200)
		case "DELETE":
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		default:
			b, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("ETag", `"900150983cd24fb0d6963f7d28e17f72"`)
			http.ServeContent(w, r, "fixture", time.Unix(1, 0), bytes.NewReader(b))
		}
	}))
	defer objectServer.Close()
	storage, e := s3.NewMinioClient(&config.Config{S3: config.S3Config{Type: "oss", Endpoint: objectServer.URL, Bucket: "fixture", AccessKey: "synthetic", SecretKey: "synthetic", Region: "us-east-1"}})
	if e != nil {
		t.Fatal(e)
	}
	preview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, "%PDF-1.7\nsynthetic preview")
	}))
	defer preview.Close()
	cfg := &config.Config{Assistant: config.AssistantConfig{GeneratorURL: "http://127.0.0.1:18091", GeneratorToken: token, RetentionDays: 30}, Preview: config.PreviewConfig{BaseURL: preview.URL}}
	owner := domain.User{ID: uuid.NewString(), Account: "pipeline_" + uuid.NewString(), Role: consts.UserRoleAdmin}
	if e = db.Create(&owner).Error; e != nil {
		t.Fatal(e)
	}
	kb := domain.KnowledgeBase{ID: uuid.NewString(), Name: "synthetic assistant KB", DatasetID: uuid.NewString()}
	if e = db.Create(&kb).Error; e != nil {
		t.Fatal(e)
	}
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, logger)}
	conversations := &ConversationUsecase{repo: pg.NewConversationRepository(db, logger)}
	chat := &ChatUsecase{appRepo: pg.NewAppRepository(db, logger), enterpriseUsecase: enterprise, conversationUsecase: conversations}
	u := &AssistantUsecase{db: db.DB, cfg: cfg, storage: storage, chat: chat, logger: logger, client: &http.Client{Timeout: 30 * time.Second}}
	spec := map[string]any{"format": "pptx", "title": "企业产品介绍", "answer": "已按模板生成产品介绍。", "slides": []any{map[string]any{"title": "合成测试产品", "bullets": []string{"这是隔离测试资料，不涉及业务信息。", "验证完整生成与下载链路。"}}}}
	var sample struct {
		Data string `json:"data"`
	}
	if e = u.generator(ctx, "/render", map[string]any{"format": "pptx", "spec": spec}, &sample); e != nil {
		t.Fatal(e)
	}
	sampleBytes, e := base64.StdEncoding.DecodeString(sample.Data)
	if e != nil {
		t.Fatal(e)
	}
	upload, e := u.Upload(ctx, kb.ID, owner.ID, "企业模板.pptx", sampleBytes)
	if e != nil {
		t.Fatal(e)
	}
	readBack, readErr := u.objectBytes(ctx, upload.ObjectKey, assistantMaxFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(readBack, sampleBytes) {
		t.Fatalf("mock S3 payload differs: got length=%d expected=%d prefix=%q", len(readBack), len(sampleBytes), readBack[:min(50, len(readBack))])
	}
	var inspected struct {
		Hash string `json:"hash"`
	}
	if er := u.generator(ctx, "/inspect", map[string]any{"format": "pptx", "data": base64.StdEncoding.EncodeToString(sampleBytes)}, &inspected); er != nil {
		t.Fatal(er)
	}
	if inspected.Hash != digest(sampleBytes) {
		t.Fatalf("generator inspection hash differs: received %q", inspected.Hash)
	}
	if e = u.processUpload(ctx); e != nil {
		t.Fatal(e)
	}
	ready, e := u.GetUpload(ctx, kb.ID, owner.ID, upload.ID)
	if e != nil || ready.Status != "ready" || ready.TemplateKey == "" {
		t.Fatalf("template inspection failed: %+v %v", ready, e)
	}
	template, e := u.SaveTemplate(ctx, kb.ID, owner.ID, domain.AssistantTemplate{Name: "测试模板", UploadID: upload.ID, Confirmed: true, Permissions: domain.NodePermissions{ScopeType: "public", AllowAllUsers: true}})
	if e != nil {
		t.Fatal(e)
	}
	if e = u.TemplateState(ctx, kb.ID, template.ID, template.Version, "published", true); e != nil {
		t.Fatal(e)
	}
	req := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "用企业模板生成产品介绍PPT", SourceScope: domain.AssistantScope{Type: "documents"}}
	run, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", req)
	if e != nil {
		t.Fatal(e)
	}
	claim, e := u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	plan := &domain.AssistantPlan{Revision: claim.LeaseEpoch, Format: "pptx", Title: "合成产品介绍", Sufficient: true, Outline: []domain.AssistantOutlineItem{{ID: "part-1", Title: "合成测试产品", Purpose: "验证确认后生成文件的完整流程"}}}
	claim.Checkpoint["planning"], claim.Checkpoint["planning_materials"] = plan, "合成测试资料：验证完整生成与下载链路。"
	if e = u.pauseAssistantPlan(ctx, claim, "awaiting_outline", "请确认生成大纲", plan); e != nil {
		t.Fatal(e)
	}
	confirmed, e := u.ContinueRun(ctx, kb.ID, owner.ID, run.ID, domain.AssistantContinue{ClientRequestID: uuid.NewString(), Revision: plan.Revision, Action: "confirm_outline", Outline: plan.Outline})
	if e != nil {
		t.Fatal(e)
	}
	// Bypass only the external model, not confirmation, rendering, storage or publishing.
	confirmed.Checkpoint["spec"] = spec
	if e = db.Model(&domain.AssistantRun{}).Where("id=?", run.ID).Update("checkpoint", confirmed.Checkpoint).Error; e != nil {
		t.Fatal(e)
	}
	claim, e = u.claimRun(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = u.buildRun(ctx, claim); e != nil {
		t.Fatal(e)
	}
	finished, e := u.Run(ctx, kb.ID, owner.ID, run.ID)
	if e != nil || finished.State != "completed" || finished.ArtifactID == "" {
		t.Fatalf("not completed: %+v %v", finished, e)
	}
	artifact, e := u.Artifact(ctx, kb.ID, owner.ID, finished.ArtifactID)
	if e != nil || artifact.Revision != 1 {
		t.Fatal(e)
	}
	body, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", artifact.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	defer body.Close()
	b, e := io.ReadAll(body)
	if e != nil || digest(b) != artifact.Hash || !bytes.HasPrefix(b, []byte("PK")) {
		t.Fatal("download did not match published Office file", e)
	}
	if _, e = u.Artifact(ctx, kb.ID, uuid.NewString(), artifact.ID); e == nil {
		t.Fatal("another account downloaded private output")
	}
	// Exercise durable per-page generation through actual template rendering,
	// private publication, inspection and download, without any external model.
	partsReq := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "生成三页合成产品PPT", Format: "pptx", SourceScope: domain.AssistantScope{Type: "documents"}}
	partsRun, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", partsReq)
	if e != nil {
		t.Fatal(e)
	}
	partsClaim, e := u.claimRun(ctx)
	if e != nil || partsClaim.ID != partsRun.ID {
		t.Fatal("segmented task claim failed", e)
	}
	partsPlan := generationPlan()
	partsClaim.Checkpoint["planning"] = partsPlan
	ref := domain.AssistantTemplateRef{Kind: "enterprise", ID: template.ID, Version: template.Version}
	refBytes, _ := json.Marshal(ref)
	partsClaim.Sources = sourcesJSON([]domain.AssistantSource{{Kind: "template", ID: template.ID, Version: string(refBytes), Name: template.Name}})
	firstPart, _ := json.Marshal(domain.JSONB{"format": "pptx", "slides": []domain.JSONB{{"title": "巡检记录", "bullets": []string{"巡检人员填写检查结果。", "保留巡检记录以便复盘。"}}}})
	partsModel := &scriptedPlanningChat{replies: []*schema.Message{{Content: string(firstPart)}, partResponse("异常工单", "异常生成工单，维修人员接单并反馈结果。"), partResponse("闭环复盘", "管理者查看未闭环事项并安排复盘。")}}
	partsSpec, _, e := u.generateAssistantParts(ctx, partsClaim, partsModel, nil, partsPlan)
	if e != nil {
		t.Fatal(e)
	}
	partsTemplate, e := u.objectBytes(ctx, ready.TemplateKey, assistantMaxFile)
	if e != nil {
		t.Fatal(e)
	}
	if e = u.renderAssistantArtifact(ctx, partsClaim, partsReq, partsSpec, nil, partsTemplate); e != nil {
		t.Fatal(e)
	}
	partsDone, e := u.Run(ctx, kb.ID, owner.ID, partsRun.ID)
	if e != nil || partsDone.State != "completed" || partsDone.Generation == nil || partsDone.Generation.Completed != 3 {
		t.Fatal("segmented task failed to deliver", e)
	}
	partsBody, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", partsDone.ArtifactID, false)
	if e != nil {
		t.Fatal(e)
	}
	partsBytes, e := io.ReadAll(partsBody)
	partsBody.Close()
	if e != nil {
		t.Fatal(e)
	}
	var partsInspect struct {
		Text   string `json:"text"`
		Report struct {
			Slides int `json:"slides"`
		} `json:"report"`
	}
	if e = u.generator(ctx, "/inspect", map[string]any{"format": "pptx", "data": base64.StdEncoding.EncodeToString(partsBytes)}, &partsInspect); e != nil || partsInspect.Report.Slides != 4 || !strings.Contains(partsInspect.Text, "未闭环事项") {
		t.Fatal("actual segmented presentation is incomplete", e)
	}
	baseParts, e := u.Artifact(ctx, kb.ID, owner.ID, partsDone.ArtifactID)
	if e != nil {
		t.Fatal(e)
	}
	editReq := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: partsDone.ConversationID, Message: "第三页改得更简洁", BaseArtifactID: baseParts.ID, TargetIDs: []string{baseParts.Outline[2].ID}, SourceScope: domain.AssistantScope{Type: "documents"}}
	editRun, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", editReq)
	if e != nil {
		t.Fatal(e)
	}
	editClaim, e := u.claimRun(ctx)
	if e != nil || editClaim.ID != editRun.ID {
		t.Fatal("revision claim failed", e)
	}
	editClaim.Sources = baseParts.Sources
	editModel := &scriptedPlanningChat{replies: []*schema.Message{partResponse("复盘未闭环事项", "管理者复盘未闭环事项。")}}
	if e = u.buildAssistantRevision(ctx, editClaim, editReq, baseParts, partsTemplate, "", editModel, &domain.Model{}); e != nil {
		t.Fatal(e)
	}
	editDone, e := u.Run(ctx, kb.ID, owner.ID, editRun.ID)
	if e != nil {
		t.Fatal(e)
	}
	edited, e := u.Artifact(ctx, kb.ID, owner.ID, editDone.ArtifactID)
	if e != nil || edited.FamilyID != baseParts.FamilyID || edited.Revision != 2 {
		t.Fatal("revision did not preserve file family", e)
	}
	editBody, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", edited.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	editBytes, e := io.ReadAll(editBody)
	editBody.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = u.generator(ctx, "/inspect", map[string]any{"format": "pptx", "data": base64.StdEncoding.EncodeToString(editBytes)}, &partsInspect); e != nil || partsInspect.Report.Slides != 4 || !strings.Contains(partsInspect.Text, "巡检人员填写检查结果") || !strings.Contains(partsInspect.Text, "复盘未闭环事项") {
		t.Fatal("edited Office file lost unselected page or changed page", e)
	}
	// Literal edits traverse the real worker path with no model configured.
	// Any accidental model call would fail instead of hiding behind a stub.
	literalBase := edited
	for _, message := range []string{"第三页标题改为“处理结果复盘”", "第一页删除第2条"} {
		literalReq := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: partsDone.ConversationID, Message: message, BaseArtifactID: literalBase.ID, TemplateRef: &ref, SourceScope: domain.AssistantScope{Type: "documents"}}
		literalRun, err := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", literalReq)
		if err != nil {
			t.Fatal(err)
		}
		literalClaim, err := u.claimRun(ctx)
		if err != nil || literalClaim.ID != literalRun.ID {
			t.Fatal("literal claim", err)
		}
		started := time.Now()
		if err = u.buildRun(ctx, literalClaim); err != nil {
			t.Fatal("literal worker path", err)
		}
		done, err := u.Run(ctx, kb.ID, owner.ID, literalRun.ID)
		if err != nil || done.State != "completed" || done.Checkpoint["revision_method"] != "literal" {
			t.Fatal("literal edit not published", err)
		}
		next, err := u.Artifact(ctx, kb.ID, owner.ID, done.ArtifactID)
		if err != nil || next.FamilyID != literalBase.FamilyID || next.Revision != literalBase.Revision+1 {
			t.Fatal("literal version lost", err)
		}
		var saved domain.AssistantArtifact
		if err = db.First(&saved, "id=?", next.ID).Error; err != nil {
			t.Fatal(err)
		}
		changed := assistantArray(saved.Spec["slides"])
		if strings.Contains(message, "标题") && changed[2]["title"] != "处理结果复盘" {
			t.Fatal("literal title missing in actual rendered spec")
		}
		if strings.Contains(message, "删除") {
			var bullets []string
			_ = decodeAssistantValue(changed[0]["bullets"], &bullets)
			if len(bullets) != 1 || bullets[0] != "巡检人员填写检查结果。" {
				t.Fatal("literal removal lost retained point")
			}
		}
		t.Logf("literal Office worker path: %s elapsed=%s (preview uses fixture, no real OSS)", message, time.Since(started).Round(time.Millisecond))
		literalBase = next
	}
	// Editing an older version forks a file instead of overwriting its newer version.
	deleteReq := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), ConversationID: partsDone.ConversationID, Message: "删除第二页", BaseArtifactID: baseParts.ID, SourceScope: domain.AssistantScope{Type: "documents"}}
	deleteRun, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", deleteReq)
	if e != nil {
		t.Fatal(e)
	}
	deleteClaim, e := u.claimRun(ctx)
	if e != nil || deleteClaim.ID != deleteRun.ID {
		t.Fatal(e)
	}
	deleteClaim.Sources = baseParts.Sources
	if e = u.buildAssistantRevision(ctx, deleteClaim, deleteReq, baseParts, partsTemplate, "", nil, nil); e != nil {
		t.Fatal(e)
	}
	deleteDone, e := u.Run(ctx, kb.ID, owner.ID, deleteRun.ID)
	if e != nil {
		t.Fatal(e)
	}
	fork, e := u.Artifact(ctx, kb.ID, owner.ID, deleteDone.ArtifactID)
	if e != nil || fork.FamilyID == baseParts.FamilyID || fork.Revision != 1 || len(fork.Outline) != 2 {
		t.Fatal("old-version fork replaced a newer revision", e)
	}
	oldBody, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", baseParts.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	oldBytes, e := io.ReadAll(oldBody)
	oldBody.Close()
	if e != nil || !bytes.Equal(partsBytes, oldBytes) {
		t.Fatal("original downloadable artifact changed", e)
	}
	wordReq := domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "把合成资料整理为Word报告", Format: "docx", SourceScope: domain.AssistantScope{Type: "documents"}}
	wordRun, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", wordReq)
	if e != nil {
		t.Fatal(e)
	}
	wordClaim, e := u.claimRun(ctx)
	if e != nil || wordClaim.ID != wordRun.ID {
		t.Fatal("Word claim failed", e)
	}
	wordPlan := &domain.AssistantPlan{Revision: 1, Format: "docx", Title: "巡检闭环报告", Sufficient: true, Outline: []domain.AssistantOutlineItem{{ID: "part-1", Title: "集中记录"}, {ID: "part-2", Title: "异常闭环"}}}
	wordClaim.Checkpoint["planning"] = wordPlan
	wordModel := &scriptedPlanningChat{replies: []*schema.Message{{Content: `{"format":"docx","sections":[{"title":"集中记录","paragraphs":["巡检人员集中填写检查结果，统一保留记录。"]}]}`}, {Content: `{"format":"docx","sections":[{"title":"异常闭环","paragraphs":["异常转为工单，维修人员处理并回填结果，管理者复盘未闭环事项。"]}]}`}}}
	wordSpec, _, e := u.generateAssistantParts(ctx, wordClaim, wordModel, nil, wordPlan)
	if e != nil {
		t.Fatal(e)
	}
	if e = u.renderAssistantArtifact(ctx, wordClaim, wordReq, wordSpec, nil, nil); e != nil {
		t.Fatal(e)
	}
	wordDone, e := u.Run(ctx, kb.ID, owner.ID, wordRun.ID)
	if e != nil || wordDone.State != "completed" || wordDone.Generation.Completed != 2 {
		t.Fatal("segmented Word not completed", e)
	}
	wordBody, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", wordDone.ArtifactID, false)
	if e != nil {
		t.Fatal(e)
	}
	wordBytes, e := io.ReadAll(wordBody)
	wordBody.Close()
	if e != nil {
		t.Fatal(e)
	}
	var wordInspect struct {
		Text string `json:"text"`
	}
	if e = u.generator(ctx, "/inspect", map[string]any{"format": "docx", "data": base64.StdEncoding.EncodeToString(wordBytes)}, &wordInspect); e != nil || !strings.Contains(wordInspect.Text, "集中记录") || !strings.Contains(wordInspect.Text, "复盘未闭环事项") {
		t.Fatal("actual segmented Word missing content", e)
	}
	// A library catalog must enumerate the published snapshot without any
	// model or vector-store client. Render, private storage and download are real.
	nav := domain.Nav{ID: uuid.NewString(), KbID: kb.ID, Name: "测试栏目"}
	kbRelease := domain.KBRelease{ID: uuid.NewString(), KBID: kb.ID}
	if e = db.Create(&nav).Error; e != nil {
		t.Fatal(e)
	}
	if e = db.Create(&kbRelease).Error; e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 23; i++ {
		n := domain.Node{ID: uuid.NewString(), KBID: kb.ID, NavId: nav.ID, Type: domain.NodeTypeDocument, Name: "草稿不可见"}
		nr := domain.NodeRelease{ID: uuid.NewString(), NodeID: n.ID, KBID: kb.ID, Type: domain.NodeTypeDocument, Name: fmt.Sprintf("已发布资料%d.pdf", i)}
		if e = db.Create(&n).Error; e != nil {
			t.Fatal(e)
		}
		if e = db.Create(&nr).Error; e != nil {
			t.Fatal(e)
		}
		if e = db.Create(&domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: kb.ID, ReleaseID: kbRelease.ID, NodeID: n.ID, NodeReleaseID: nr.ID, NavID: nav.ID}).Error; e != nil {
			t.Fatal(e)
		}
	}
	catalog, e := u.Submit(ctx, kb.ID, owner.ID, "127.0.0.1", domain.AssistantSubmit{ClientRequestID: uuid.NewString(), Message: "把目前知识库整理成图书馆目录册，生成Excel", Format: "xlsx", SourceScope: domain.AssistantScope{Type: "all"}})
	if e != nil {
		t.Fatal(e)
	}
	catalogClaim, e := u.claimRun(ctx)
	if e != nil || catalogClaim.ID != catalog.ID {
		t.Fatal("catalog claim failed", e)
	}
	if e = u.buildRun(ctx, catalogClaim); e != nil {
		t.Fatal(e)
	}
	catalog, e = u.Run(ctx, kb.ID, owner.ID, catalog.ID)
	if e != nil || catalog.State != "completed" {
		t.Fatal("catalog was not delivered", e)
	}
	catalogArtifact, e := u.Artifact(ctx, kb.ID, owner.ID, catalog.ArtifactID)
	if e != nil {
		t.Fatal(e)
	}
	if len(sourceList(catalogArtifact.Sources)) != 23 {
		t.Fatal("catalog omitted published files")
	}
	previews := assistantArray(catalogArtifact.Spec["preview_sheets"])
	var data struct {
		Rows [][]any `json:"rows"`
	}
	if len(previews) != 3 {
		t.Fatal("catalog workbook structure incorrect")
	}
	if e = assistantDecode(previews[1], &data); e != nil || len(data.Rows) != 23 {
		t.Fatal("actual rendered workbook lost rows", e)
	}
	for _, row := range data.Rows {
		if strings.Contains(fmt.Sprint(row), "草稿不可见") {
			t.Fatal("draft data leaked")
		}
	}
	catalogBody, _, _, e := u.Content(ctx, kb.ID, owner.ID, "artifacts", catalogArtifact.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	catalogBytes, e := io.ReadAll(catalogBody)
	catalogBody.Close()
	if e != nil || digest(catalogBytes) != catalogArtifact.Hash || !bytes.HasPrefix(catalogBytes, []byte("PK")) {
		t.Fatal("catalog download invalid", e)
	}
	if e = u.DeleteUpload(ctx, kb.ID, owner.ID, upload.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = u.Artifact(ctx, kb.ID, owner.ID, artifact.ID); e == nil {
		t.Fatal("deleted template source did not restrict derived file")
	}
}
