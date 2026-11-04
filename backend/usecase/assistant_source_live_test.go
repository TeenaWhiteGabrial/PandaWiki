package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	modelkit "github.com/chaitin/ModelKit/v2/usecase"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// Explicit opt-in: reads one local published release and sends its body to the
// configured model. Cloned source and task rows live only in a rollback-only
// transaction; no global queue, real object storage, or index operations run.
// This checks content generation, not browser authentication or original ACLs.
func TestAssistantPublishedSourceLiveModel(t *testing.T) {
	releaseID := os.Getenv("PANDA_WIKI_TEST_SOURCE_RELEASE")
	if os.Getenv("PANDA_WIKI_TEST_LIVE_LLM") != "1" || releaseID == "" {
		t.Skip("explicit local published-source/model opt-in required")
	}
	u, db, r := planningFixture(t)
	var original domain.NodeRelease
	if e := db.First(&original, "id=?", releaseID).Error; e != nil {
		t.Fatal("selected local release unavailable")
	}
	var published int64
	if e := db.Raw(`SELECT count(*) FROM kb_release_node_releases kr WHERE kr.node_release_id=? AND kr.release_id=(SELECT id FROM kb_releases WHERE kb_id=? ORDER BY created_at DESC LIMIT 1)`, releaseID, original.KBID).Scan(&published).Error; e != nil || published == 0 {
		t.Fatal("selected release is not currently published")
	}
	if assistantReleaseMetadataOnly(&original) || assistantTextLength(original.Content) < 500 {
		t.Fatal("selected source does not contain sufficient body text")
	}
	cm, info := assistantConfiguredAcceptanceModel(t, u, db)
	var e error
	r.Checkpoint["file_instruct_fields"] = assistantFileInstructFields(&info)
	node := domain.Node{ID: uuid.NewString(), KBID: r.KBID, Type: original.Type, Name: original.Name, Content: original.Content}
	if e = db.Create(&node).Error; e != nil {
		t.Fatal(e)
	}
	release := original
	release.ID, release.NodeID, release.KBID, release.DocID, release.ParentID = uuid.NewString(), node.ID, r.KBID, "", ""
	if e = db.Create(&release).Error; e != nil {
		t.Fatal(e)
	}
	source := domain.AssistantSource{Kind: "node", ID: node.ID, Version: release.ID, Name: original.Name, Hash: digest([]byte(original.Content))}
	r.Sources = sourcesJSON([]domain.AssistantSource{source})
	materials, coverage := prepareAssistantMaterials([]assistantMaterialInput{{Source: source, Header: "\n资料ID：" + assistantSourceKey(source) + "\n", Text: original.Content}}, 30000)
	r.Checkpoint["material_coverage"], r.Checkpoint["material_coverage_version"] = coverage, 1
	r.State, r.LeaseEpoch = "running", r.LeaseEpoch+1
	until := time.Now().Add(9 * time.Minute)
	r.LeaseUntil = &until
	if e = db.Model(&domain.AssistantRun{}).Where("id=?", r.ID).Updates(map[string]any{"state": r.State, "lease_epoch": r.LeaseEpoch, "lease_until": until, "sources": r.Sources, "checkpoint": r.Checkpoint}).Error; e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	req := domain.AssistantSubmit{Message: "根据这份产品清单，生成数字孪生产品介绍PPT，给制造业设备与生产管理负责人看，约5分钟、6页正文。重点比较工厂级、产线级、装备级的适用场景和功能，并交代工业智能数据平台、AI可视化平台的支撑能力。用清晰具体的表达，不要空泛口号。不写报价、客户案例或没有依据的性能数字。直接生成。", Format: "pptx"}
	started := time.Now()
	var spec domain.JSONB
	var plan *domain.AssistantPlan
	reused := os.Getenv("PANDA_WIKI_TEST_EXISTING_SPEC")
	if reused != "" {
		data, err := os.ReadFile(reused)
		if err != nil {
			t.Fatal(err)
		}
		if e = json.Unmarshal(data, &spec); e != nil {
			t.Fatal(e)
		}
		plan = &domain.AssistantPlan{Format: "pptx", Title: assistantString(spec["title"]), Sufficient: true}
		t.Log("revision-only acceptance reuses an explicitly supplied local draft; initial generation is not rerun")
	} else {
		ready, err := u.prepareAssistantPlan(ctx, r, req, "pptx", materials, nil, nil, cm)
		if err != nil {
			t.Fatalf("planning: %s", safeAssistantError(err))
		}
		plan = assistantPlan(r)
		if !ready || plan == nil || !plan.Sufficient || len(plan.Outline) != 6 {
			t.Fatalf("clear source brief did not proceed: ready=%t state=%s", ready, r.State)
		}
		if os.Getenv("PANDA_WIKI_TEST_PLANNING_ONLY") == "1" {
			t.Logf("real source planning acceptance: elapsed=%s outline=%d sufficient=%t", time.Since(started).Round(time.Millisecond), len(plan.Outline), plan.Sufficient)
			return
		}
		planJSON, _ := json.Marshal(plan)
		messages := []*schema.Message{schema.SystemMessage(assistantContentSystemPrompt), schema.SystemMessage("按已确认大纲逐页生成，资料中没有的数值和案例不可补造。已确认计划：" + string(planJSON)), schema.UserMessage("参考资料：\n" + materials + "\n用户要求：" + req.Message)}
		spec, _, e = u.generateAssistantParts(ctx, r, cm, messages, plan)
		if e != nil {
			t.Fatalf("generation: %s", safeAssistantError(e))
		}
		if e = prepareArtifactSpec(spec, "pptx", r.Sources, nil); e != nil {
			t.Fatal(e)
		}
	}
	base := &domain.AssistantArtifact{Format: "pptx", Spec: spec}
	parts := assistantArray(spec["slides"])
	if len(parts) != 7 {
		t.Fatal("body/source page count changed")
	}
	output := os.Getenv("PANDA_WIKI_TEST_GENERATION_OUTPUT")
	if output == "" {
		t.Fatal("local acceptance output path required")
	}
	if e = os.MkdirAll(filepath.Dir(output), 0700); e != nil {
		t.Fatal(e)
	}
	writeJSON := func(name string, value any) {
		t.Helper()
		data, _ := json.MarshalIndent(value, "", "  ")
		if e = os.WriteFile(output+name+".json", data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	writeJSON("-original", spec)
	t.Cleanup(func() {
		writeJSON("-diagnostics", map[string]any{"planning": r.Checkpoint["planning_diagnostics"], "generation": r.Checkpoint["content_generation_diagnostics"], "invalid_response": r.Checkpoint["content_invalid_response"]})
	})
	third := parts[2]
	edit := domain.AssistantSubmit{Message: "只把第三页正文精简为两个要点，每点不超过45个汉字，保留产品名称、版本号与原有事实，不改标题、不新增数字。", TargetIDs: []string{assistantString(third["id"])}}
	targets, positions, e := assistantRevisionTargets(edit, base)
	if e != nil || len(targets) != 1 {
		t.Fatal("revision selection failed", e)
	}
	editPlan := &domain.AssistantPlan{Format: "pptx", Title: plan.Title, Sufficient: true, Revision: r.LeaseEpoch, Outline: []domain.AssistantOutlineItem{{ID: assistantString(third["id"]), Title: assistantString(third["title"]), Purpose: "按用户意见精简本页，其他页面保持原样", SourceIDs: []string{assistantSourceKey(source)}}}}
	r.Checkpoint["planning"], r.Checkpoint["editing_parts"], r.Checkpoint["revision_positions"] = editPlan, domain.JSONB{assistantString(third["id"]): third}, positions
	r.Checkpoint["part_constraints"] = assistantRevisionConstraints(edit.Message)
	partial, _, e := u.generateAssistantParts(ctx, r, cm, assistantRevisionMessages(r, edit, materials), editPlan)
	if e != nil {
		t.Fatalf("revision: %s", safeAssistantError(e))
	}
	revised, e := mergeAssistantRevision(base, assistantArray(partial["slides"]))
	if e != nil {
		t.Fatal(e)
	}
	revisedParts := assistantArray(revised["slides"])
	for i := range parts {
		if i != 2 && !reflect.DeepEqual(parts[i], revisedParts[i]) {
			t.Fatalf("untargeted page %d changed", i+1)
		}
	}
	if reflect.DeepEqual(parts[2], revisedParts[2]) {
		t.Fatal("requested page unchanged")
	}
	var bullets []string
	_ = decodeAssistantValue(revisedParts[2]["bullets"], &bullets)
	if len(bullets) != 2 || assistantString(revisedParts[2]["title"]) != assistantString(third["title"]) {
		t.Fatal("revision did not follow requested scope")
	}
	for _, line := range bullets {
		if len([]rune(line)) > 45 {
			t.Fatal("revision exceeded requested length")
		}
	}
	writeJSON("-report", map[string]any{"source_name": original.Name, "original_release_id": original.ID, "source_hash": source.Hash, "body_characters": assistantTextLength(original.Content), "planning": plan, "elapsed_seconds": time.Since(started).Seconds(), "reused_local_draft": reused != "", "untargeted_pages_unchanged": true, "business_writes_committed": false, "original_acl_browser_tested": false})
	writeJSON("-original", spec)
	writeJSON("-revised", revised)
	token := os.Getenv("PANDA_WIKI_TEST_GENERATOR_TOKEN")
	if token == "" {
		t.Fatal("local generator token required")
	}
	u.cfg = &config.Config{Assistant: config.AssistantConfig{GeneratorURL: "http://127.0.0.1:18091", GeneratorToken: token}}
	u.client = &http.Client{Timeout: 45 * time.Second}
	for name, value := range map[string]domain.JSONB{"-original": spec, "-revised": revised} {
		var rendered struct {
			Data string `json:"data"`
		}
		if e = u.generator(ctx, "/render", map[string]any{"format": "pptx", "spec": value}, &rendered); e != nil {
			t.Fatal(e)
		}
		data, err := base64.StdEncoding.DecodeString(rendered.Data)
		if err != nil {
			t.Fatal(err)
		}
		if e = os.WriteFile(output+name+".pptx", data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("real local source: %d characters, 6 body pages, third-page revision preserved other pages, elapsed=%s", assistantTextLength(original.Content), time.Since(started).Round(time.Second))
	if strings.Contains(assistantString(spec["title"]), "合成") {
		t.Fatal("synthetic test content contaminated real source")
	}
}

func assistantConfiguredAcceptanceModel(t *testing.T, u *AssistantUsecase, db *store.DB) (einomodel.BaseChatModel, domain.Model) {
	t.Helper()
	var setting domain.SystemSetting
	if e := db.Where("key=?", consts.SystemSettingModelMode).First(&setting).Error; e != nil {
		t.Fatal("configured local model unavailable")
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
		t.Fatal("active chat model unavailable")
	}
	metadata, e := info.ToModelkitModel()
	if e != nil {
		t.Fatal("model metadata invalid")
	}
	if os.Getenv("PANDA_WIKI_TEST_MODEL_WIRE") == "1" {
		upstream, err := url.Parse(metadata.BaseURL)
		if err != nil {
			t.Fatal("invalid model URL")
		}
		proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: upstream.Scheme, Host: upstream.Host})
		director := proxy.Director
		proxy.Director = func(request *http.Request) { director(request); request.Host = upstream.Host }
		proxy.ModifyResponse = func(response *http.Response) error {
			data, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			response.Body.Close()
			response.Body = io.NopCloser(bytes.NewReader(data))
			var body map[string]any
			_ = json.Unmarshal(data, &body)
			var choices []struct {
				Finish  string `json:"finish_reason"`
				Message struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning_content"`
					Thinking  string `json:"reasoning"`
				} `json:"message"`
			}
			if raw, err := json.Marshal(body["choices"]); err == nil {
				_ = json.Unmarshal(raw, &choices)
			}
			if len(choices) > 0 {
				t.Logf("wire response: status=%d usage=%v body_bytes=%d reasoning_bytes=%d alternate_reasoning_bytes=%d finish=%s", response.StatusCode, body["usage"], len(choices[0].Message.Content), len(choices[0].Message.Reasoning), len(choices[0].Message.Thinking), choices[0].Finish)
			} else {
				t.Logf("wire response: status=%d bytes=%d choices=0", response.StatusCode, len(data))
			}
			return nil
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			request.Body.Close()
			request.Body = io.NopCloser(bytes.NewReader(data))
			var body map[string]any
			_ = json.Unmarshal(data, &body)
			t.Logf("wire options: enable_thinking=%v chat_template_kwargs=%v reasoning=%v temperature=%v top_p=%v max_tokens=%v input_bytes=%d", body["enable_thinking"], body["chat_template_kwargs"], body["reasoning"], body["temperature"], body["top_p"], body["max_tokens"], len(data))
			proxy.ServeHTTP(w, request)
		}))
		t.Cleanup(server.Close)
		local, _ := url.Parse(server.URL)
		rewritten := *upstream
		rewritten.Scheme, rewritten.Host = local.Scheme, local.Host
		metadata.BaseURL = rewritten.String()
	}
	cm, e := modelkit.NewModelKit(u.logger.Logger).GetChatModel(context.Background(), metadata)
	if e != nil {
		t.Fatal("model initialization failed")
	}

	return cm, info
}

// Local Office + actual local PDF conversion only. No database/model/OSS.
func TestAssistantLiteralOfficeTiming(t *testing.T) {
	input := os.Getenv("PANDA_WIKI_TEST_EXISTING_SPEC")
	token, password := os.Getenv("PANDA_WIKI_TEST_GENERATOR_TOKEN"), os.Getenv("PANDA_WIKI_TEST_PREVIEW_PASSWORD")
	if input == "" || token == "" || password == "" {
		t.Skip("explicit local Office timing inputs required")
	}
	data, e := os.ReadFile(input)
	if e != nil {
		t.Fatal(e)
	}
	var spec domain.JSONB
	if e = json.Unmarshal(data, &spec); e != nil {
		t.Fatal(e)
	}
	u := &AssistantUsecase{cfg: &config.Config{Assistant: config.AssistantConfig{GeneratorURL: "http://127.0.0.1:18091", GeneratorToken: token}, Preview: config.PreviewConfig{BaseURL: "http://127.0.0.1:13000", Username: "panda-wiki", Password: password}}, client: &http.Client{Timeout: 45 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for i, message := range []string{"第三页标题改为“产线数字孪生核心能力”", "第三页删除第2条"} {
		base := &domain.AssistantArtifact{Format: "pptx", Spec: spec}
		req := domain.AssistantSubmit{Message: message}
		started := time.Now()
		targets, _, err := assistantRevisionTargets(req, base)
		if err != nil {
			t.Fatal(err)
		}
		next, handled, err := assistantApplyLiteralRevision(req, base, targets)
		if err != nil || !handled {
			t.Fatal(err)
		}
		var output struct {
			Data string `json:"data"`
		}
		if e = u.generator(ctx, "/render", map[string]any{"format": "pptx", "spec": next}, &output); e != nil {
			t.Fatal(e)
		}
		office, e := base64.StdEncoding.DecodeString(output.Data)
		if e != nil {
			t.Fatal(e)
		}
		pdf, e := u.preview(ctx, office, "local-speed-check.pptx")
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatal("preview not PDF")
		}
		t.Logf("literal edit including actual local PPT/PDF: %s elapsed=%s model_calls=0", message, time.Since(started).Round(time.Millisecond))
		if prefix := os.Getenv("PANDA_WIKI_TEST_GENERATION_OUTPUT"); prefix != "" {
			if e = os.WriteFile(fmt.Sprintf("%s-%d.pptx", prefix, i+1), office, 0600); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(fmt.Sprintf("%s-%d.pdf", prefix, i+1), pdf, 0600); e != nil {
				t.Fatal(e)
			}
		}
		spec = next
	}
}
