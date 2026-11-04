package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	dbstore "github.com/chaitin/panda-wiki/store/pg"
	"github.com/chaitin/panda-wiki/store/s3"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAssistantAccess = errors.New("无权访问该会话或文件，或资源已失效")

const assistantMaxFile = 20 << 20

type AssistantUsecase struct {
	db      *gorm.DB
	cfg     *config.Config
	storage *s3.MinioClient
	chat    *ChatUsecase
	logger  *log.Logger
	client  *http.Client
}

func NewAssistantUsecase(db *dbstore.DB, cfg *config.Config, storage *s3.MinioClient, chat *ChatUsecase, logger *log.Logger) *AssistantUsecase {
	return &AssistantUsecase{db: db.DB, cfg: cfg, storage: storage, chat: chat, logger: logger.WithModule("usecase.assistant"), client: &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func assistantJSON(v any) domain.JSONB {
	b, _ := json.Marshal(v)
	var m domain.JSONB
	_ = json.Unmarshal(b, &m)
	return m
}
func assistantDecode(v domain.JSONB, out any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (u *AssistantUsecase) expiry() time.Time {
	return time.Now().Add(time.Duration(u.cfg.Assistant.RetentionDays) * 24 * time.Hour)
}

func (u *AssistantUsecase) OwnConversation(ctx context.Context, kb, user, id string) error {
	if user == "" || id == "" {
		return ErrAssistantAccess
	}
	var n int64
	e := u.db.WithContext(ctx).Model(&domain.Conversation{}).Where("id=? AND kb_id=? AND info->'user_info'->>'user_id'=?", id, kb, user).Count(&n).Error
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrAssistantAccess
	}
	return nil
}
func (u *AssistantUsecase) Upload(ctx context.Context, kb, user, name string, data []byte) (*domain.AssistantUpload, error) {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if user == "" || kb == "" || len(data) == 0 || len(data) > assistantMaxFile || !slices.Contains([]string{"pdf", "docx", "pptx", "xlsx", "txt", "csv", "png", "jpg", "jpeg"}, ext) {
		return nil, fmt.Errorf("支持 PDF、DOCX、PPTX、XLSX、TXT、CSV及PNG/JPEG排版素材，单文件不超过20 MB")
	}
	id := uuid.NewString()
	key := kb + "/assistant/uploads/" + id + "/source." + ext
	v := &domain.AssistantUpload{ID: id, KBID: kb, UserID: user, Name: name, Format: ext, Size: int64(len(data)), Hash: digest(data), ObjectKey: key, Status: "uploading", Report: domain.JSONB{}, ExpiresAt: time.Now().Add(24 * time.Hour)}
	e := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":"+user+":uploads").Error; e != nil {
			return e
		}
		var bytesUsed int64
		if e := tx.Model(&domain.AssistantUpload{}).Select("COALESCE(SUM(size),0)").Where("kb_id=? AND user_id=? AND status <> 'deleted' AND expires_at>?", kb, user, time.Now()).Scan(&bytesUsed).Error; e != nil {
			return e
		}
		if bytesUsed+int64(len(data)) > 1<<30 {
			return fmt.Errorf("个人临时资料已超过1 GB，请先删除不需要的文件")
		}
		return tx.Create(v).Error
	})
	if e != nil {
		return nil, e
	}
	if e = u.managedPut(ctx, key, data, "application/octet-stream", nil); e != nil {
		clean, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		u.db.WithContext(clean).Model(v).Updates(map[string]any{"status": "failed", "error": "上传未完成，请重新上传"})
		return nil, e
	}
	v.Status = "queued"
	if e = u.db.WithContext(ctx).Model(v).Updates(map[string]any{"status": "queued", "updated_at": time.Now()}).Error; e != nil {
		return nil, e
	}

	return v, nil
}
func (u *AssistantUsecase) GetUpload(ctx context.Context, kb, user, id string) (*domain.AssistantUpload, error) {
	var v domain.AssistantUpload
	e := u.db.WithContext(ctx).Where("id=? AND kb_id=? AND user_id=? AND status <> 'deleted' AND expires_at>?", id, kb, user, time.Now()).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, ErrAssistantAccess
	}
	return &v, e
}
func (u *AssistantUsecase) DeleteUpload(ctx context.Context, kb, user, id string) error {
	v, e := u.GetUpload(ctx, kb, user, id)
	if e != nil {
		return e
	}
	return u.db.WithContext(ctx).Model(v).Updates(map[string]any{"status": "deleted", "lease_epoch": gorm.Expr("lease_epoch+1"), "updated_at": time.Now()}).Error
}
func (u *AssistantUsecase) generator(ctx context.Context, route string, req any, out any) error {
	if u.cfg.Assistant.GeneratorToken == "" {
		return fmt.Errorf("文件处理服务未配置认证，请配置 DOCUMENT_GENERATOR_TOKEN")
	}
	b, e := json.Marshal(req)
	if e != nil {
		return e
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.Assistant.GeneratorURL+route, bytes.NewReader(b))
	if e != nil {
		return e
	}
	r.Header.Set("Authorization", "Bearer "+u.cfg.Assistant.GeneratorToken)
	r.Header.Set("Content-Type", "application/json")
	resp, e := u.client.Do(r)
	if e != nil {
		return fmt.Errorf("文件处理服务暂不可用: %w", e)
	}
	defer resp.Body.Close()
	b, e = io.ReadAll(io.LimitReader(resp.Body, 45<<20))
	if e != nil {
		return e
	}
	if resp.StatusCode != 200 {
		var result struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(b, &result)
		if result.Error != "" {
			return errors.New(result.Error)
		}
		return fmt.Errorf("文件处理服务返回%d", resp.StatusCode)
	}
	return json.Unmarshal(b, out)
}
func (u *AssistantUsecase) objectBytes(ctx context.Context, key string, limit int64) ([]byte, error) {
	r, e := u.storage.GetObject(ctx, "", key, minio.GetObjectOptions{})
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e == nil && int64(len(b)) > limit {
		return nil, fmt.Errorf("文件体积超限")
	}
	return b, e
}
func (u *AssistantUsecase) Templates(ctx context.Context, kb, user string, admin bool) ([]domain.AssistantTemplate, error) {
	var all []domain.AssistantTemplate
	q := u.db.WithContext(ctx).Where("kb_id=?", kb).Order("created_at DESC")
	if !admin {
		q = q.Where("state='published'")
	}
	if e := q.Limit(200).Find(&all).Error; e != nil {
		return nil, e
	}
	access, e := u.chat.enterpriseUsecase.BuildShareAccessContext(ctx, user)
	if e != nil {
		return nil, e
	}
	visible := make([]domain.AssistantTemplate, 0, len(all))
	for _, t := range all {
		if admin || u.chat.enterpriseUsecase.CanVisitable(t.Permissions, access) {
			visible = append(visible, t)
		}
	}
	return visible, nil
}
func (u *AssistantUsecase) template(ctx context.Context, kb, user string, ref domain.AssistantTemplateRef) (*domain.AssistantUpload, error) {
	if ref.Kind == "upload" {
		v, e := u.GetUpload(ctx, kb, user, ref.ID)
		if e != nil {
			return nil, e
		}
		if v.Status != "ready" || v.Format != "pptx" || v.TemplateKey == "" {
			return nil, fmt.Errorf("模板未准备好或不兼容")
		}
		return v, nil
	}
	if ref.Kind != "enterprise" || ref.Version < 1 {
		return nil, fmt.Errorf("模板引用无效")
	}
	var t domain.AssistantTemplate
	if e := u.db.WithContext(ctx).Where("id=? AND version=? AND kb_id=?", ref.ID, ref.Version, kb).First(&t).Error; e != nil {
		return nil, ErrAssistantAccess
	}
	// Stopping a template prevents selection, not use of an already-pinned version.
	access, e := u.chat.enterpriseUsecase.BuildShareAccessContext(ctx, user)
	if e != nil {
		return nil, e
	}
	if !u.chat.enterpriseUsecase.CanVisitable(t.Permissions, access) {
		return nil, ErrAssistantAccess
	}
	var v domain.AssistantUpload
	if e := u.db.WithContext(ctx).Where("id=? AND kb_id=? AND status='ready'", t.UploadID, kb).First(&v).Error; e != nil {
		return nil, ErrAssistantAccess
	}
	if v.TemplateKey == "" {
		return nil, fmt.Errorf("模板不兼容")
	}
	return &v, nil
}
func (u *AssistantUsecase) SaveTemplate(ctx context.Context, kb, user string, t domain.AssistantTemplate) (*domain.AssistantTemplate, error) {
	if !t.Confirmed {
		return nil, fmt.Errorf("请先确认模板预览和装饰素材")
	}
	normalized, e := u.chat.enterpriseUsecase.NormalizeNodePermissions(ctx, t.Permissions)
	if e != nil {
		return nil, e
	}
	t.Permissions = normalized
	name := strings.TrimSpace(t.Name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, fmt.Errorf("模板名称需为1～80字")
	}
	v, e := u.GetUpload(ctx, kb, user, t.UploadID)
	if e != nil {
		return nil, e
	}
	if v.Status != "ready" || v.TemplateKey == "" {
		return nil, fmt.Errorf("请先上传通过兼容性检查的PPTX模板")
	}
	if t.ID != "" {
		var count int64
		u.db.WithContext(ctx).Model(&domain.AssistantTemplate{}).Where("id=? AND kb_id=?", t.ID, kb).Count(&count)
		if count == 0 {
			return nil, ErrAssistantAccess
		}
	} else {
		t.ID = uuid.NewString()
	}
	t.KBID = kb
	t.Name = name
	t.State = "draft"
	t.IsDefault = false
	t.Version = 1
	e = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":templates").Error; e != nil {
			return e
		}
		var version int
		if e := tx.Model(&domain.AssistantTemplate{}).Where("id=?", t.ID).Select("COALESCE(MAX(version),0)").Scan(&version).Error; e != nil {
			return e
		}
		t.Version = version + 1
		if e := tx.Model(v).Update("expires_at", time.Now().AddDate(100, 0, 0)).Error; e != nil {
			return e
		}
		return tx.Create(&t).Error
	})
	return &t, e
}
func (u *AssistantUsecase) TemplateState(ctx context.Context, kb, id string, version int, state string, isDefault bool) error {
	if !slices.Contains([]string{"published", "stopped", "draft"}, state) {
		return fmt.Errorf("模板状态无效")
	}
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":templates").Error; e != nil {
			return e
		}
		var t domain.AssistantTemplate
		if e := tx.Where("id=? AND version=? AND kb_id=?", id, version, kb).First(&t).Error; e != nil {
			return ErrAssistantAccess
		}
		if state == "published" {
			var n int64
			if e := tx.Model(&domain.AssistantUpload{}).Where("id=? AND status='ready' AND template_key<>''", t.UploadID).Count(&n).Error; e != nil {
				return e
			}
			if n != 1 {
				return fmt.Errorf("模板未通过检测")
			}
			if e := tx.Model(&domain.AssistantTemplate{}).Where("id=? AND version<>?", id, version).Updates(map[string]any{"state": "stopped", "is_default": false}).Error; e != nil {
				return e
			}
		}
		if isDefault && state == "published" {
			if e := tx.Model(&domain.AssistantTemplate{}).Where("kb_id=?", kb).Update("is_default", false).Error; e != nil {
				return e
			}
		}
		return tx.Model(&domain.AssistantTemplate{}).Where("id=? AND version=? AND kb_id=?", id, version, kb).Updates(map[string]any{"state": state, "is_default": isDefault && state == "published", "updated_at": time.Now()}).Error
	})
}
func (u *AssistantUsecase) Submit(ctx context.Context, kb, user, remote string, req domain.AssistantSubmit) (*domain.AssistantRun, error) {
	if user == "" || kb == "" {
		return nil, ErrAssistantAccess
	}
	if _, e := uuid.Parse(req.ClientRequestID); e != nil {
		return nil, fmt.Errorf("缺少有效请求ID")
	}
	preferenceTemplate := req.TemplateRef
	req.Message = strings.TrimSpace(req.Message)
	if len([]rune(req.Message)) == 0 || len([]rune(req.Message)) > 12000 || len(req.AttachmentIDs) > 10 {
		return nil, fmt.Errorf("问题需为1～12000字，附件不超过10份")
	}
	if req.WebSearchEnabled && (u.cfg.Assistant.SearchKey == "" || strings.TrimSpace(req.PublicSearchQuery) == "") {
		return nil, fmt.Errorf("联网搜索未配置，或尚未提供确认可公开的检索词")
	}
	raw, _ := json.Marshal(req)
	hash := digest(raw)
	var existing domain.AssistantRun
	e := u.db.WithContext(ctx).Where("kb_id=? AND user_id=? AND request_id=?", kb, user, req.ClientRequestID).First(&existing).Error
	if e == nil {
		if existing.RequestHash != hash {
			return nil, fmt.Errorf("同一请求ID不能提交不同内容")
		}
		if e := u.checkSources(ctx, kb, user, existing.Sources); e != nil {
			return nil, e
		}
		return u.acceptedRun(ctx, &existing)
	}
	if !errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, e
	}
	if req.ConversationID != "" {
		if e := u.OwnConversation(ctx, kb, user, req.ConversationID); e != nil {
			return nil, e
		}
	}
	var total int64
	for _, id := range req.AttachmentIDs {
		v, e := u.GetUpload(ctx, kb, user, id)
		if e != nil {
			return nil, e
		}
		if slices.Contains([]string{"failed", "deleted"}, v.Status) {
			return nil, fmt.Errorf("附件%s无法使用", v.Name)
		}
		total += v.Size
	}
	if total > 100<<20 {
		return nil, fmt.Errorf("本轮附件总计不能超过100 MB")
	}
	var baseArtifact *domain.AssistantArtifact
	if req.BaseArtifactID != "" {
		baseArtifact, e = u.Artifact(ctx, kb, user, req.BaseArtifactID)
		if e != nil {
			return nil, e
		}
	}
	format := assistantFileFormat(req, baseArtifact)
	saveTemplatePreference := format == "pptx" || (format == "" && req.TemplateRef != nil)
	if format != "" && format != "pptx" {
		req.TemplateRef = nil
	}
	if req.TemplateRef != nil {
		if _, e := u.template(ctx, kb, user, *req.TemplateRef); e != nil {
			return nil, e
		}
		if req.TemplateRef.Kind == "enterprise" {
			var t domain.AssistantTemplate
			if e := u.db.WithContext(ctx).Where("id=? AND version=? AND state='published'", req.TemplateRef.ID, req.TemplateRef.Version).First(&t).Error; e != nil {
				return nil, fmt.Errorf("模板已停用，请重新选择")
			}
		}
	}
	if req.SourceScope.Type == "" {
		req.SourceScope.Type = "all"
	}
	if req.SourceScope.Type != "all" && req.SourceScope.Type != "documents" {
		return nil, fmt.Errorf("资料范围无效")
	}
	if len(req.SourceScope.NodeIDs) > 200 {
		return nil, fmt.Errorf("指定资料超过200份，请缩小范围")
	}
	req.SourceScope.ReleaseIDs = map[string]string{}
	expanded := []string{}
	for _, id := range req.SourceScope.NodeIDs {
		n, e := u.authorizedNode(ctx, kb, user, id)
		if e != nil {
			return nil, e
		}
		if n.Type == domain.NodeTypeFolder {
			var subtree []string
			if e = u.db.WithContext(ctx).Raw(`WITH RECURSIVE subtree AS (SELECT id,type,ARRAY[id] AS visited FROM nodes WHERE id=? AND kb_id=? UNION ALL SELECT n.id,n.type,s.visited||n.id FROM nodes n JOIN subtree s ON n.parent_id=s.id WHERE n.kb_id=? AND NOT n.id=ANY(s.visited)) SELECT id FROM subtree WHERE type=2 LIMIT 201`, id, kb, kb).Scan(&subtree).Error; e != nil {
				return nil, e
			}
			if len(subtree) > 200 {
				return nil, fmt.Errorf("目录资料过多，请缩小范围；未静默截断处理")
			}
			for _, child := range subtree {
				if _, e := u.authorizedNode(ctx, kb, user, child); e == nil {
					expanded = append(expanded, child)
				}
			}
		} else {
			expanded = append(expanded, id)
		}
	}
	for _, id := range expanded {
		if _, ok := req.SourceScope.ReleaseIDs[id]; ok {
			continue
		}
		release, e := u.chat.nodeRepo.GetLatestNodeReleaseByNodeID(ctx, id)
		if e != nil {
			return nil, fmt.Errorf("资料尚未发布，请选择已发布文档")
		}
		req.SourceScope.ReleaseIDs[id] = release.ID
	}
	if len(req.SourceScope.ReleaseIDs) > 200 {
		return nil, fmt.Errorf("资料超过200份，请缩小目录范围")
	}
	req.SourceScope.NodeIDs = nil
	for id := range req.SourceScope.ReleaseIDs {
		req.SourceScope.NodeIDs = append(req.SourceScope.NodeIDs, id)
	}
	if req.TemplateRef == nil && req.BaseArtifactID == "" && format == "pptx" {
		list, e := u.Templates(ctx, kb, user, false)
		if e != nil {
			return nil, e
		}
		for _, t := range list {
			if t.IsDefault {
				req.TemplateRef = &domain.AssistantTemplateRef{Kind: "enterprise", ID: t.ID, Version: t.Version}
				break
			}
		}
	}

	if req.BaseArtifactID != "" {
		a, e := u.Artifact(ctx, kb, user, req.BaseArtifactID)
		if e != nil {
			return nil, e
		}
		if req.ConversationID != a.ConversationID {
			return nil, ErrAssistantAccess
		}
		if len(req.TargetIDs) > 19 {
			return nil, fmt.Errorf("本次修改的部分过多")
		}
		allowed := map[string]bool{}
		for _, part := range assistantArray(a.Spec[assistantArtifactPartKey(a.Format)]) {
			id := assistantString(part["id"])
			if id != "system-sources" {
				allowed[id] = true
			}
		}
		for _, id := range req.TargetIDs {
			if !allowed[id] {
				return nil, fmt.Errorf("所选修改范围已变化，请重新选择")
			}
		}
	}
	app, e := u.chat.appRepo.GetOrCreateAppByKBIDAndType(ctx, kb, domain.AppTypeWeb)
	if e != nil {
		return nil, e
	}
	run := &domain.AssistantRun{ID: uuid.NewString(), KBID: kb, UserID: user, RequestID: req.ClientRequestID, RequestHash: hash, UserMessageID: uuid.NewString(), AssistantMessageID: uuid.NewString(), State: "queued", Stage: "等待处理", StageStartedAt: time.Now(), Checkpoint: domain.JSONB{}, Sources: domain.JSONB{}, Steps: domain.JSONB{}}
	e = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":"+user).Error; e != nil {
			return e
		}
		var old domain.AssistantRun
		if e := tx.Where("kb_id=? AND user_id=? AND request_id=?", kb, user, req.ClientRequestID).First(&old).Error; e == nil {
			if old.RequestHash != hash {
				return fmt.Errorf("同一请求ID不能提交不同内容")
			}
			*run = old
			return nil
		}
		var active int64
		if e := tx.Model(&domain.AssistantRun{}).Where("user_id=?", user).Where(assistantPendingStates).Count(&active).Error; e != nil {
			return e
		}
		if active >= 2 {
			return fmt.Errorf("当前账号已有两个任务，请等待或取消后再提交")
		}
		if req.ConversationID != "" {
			var pending int64
			if e := tx.Model(&domain.AssistantRun{}).Where("conversation_id=?", req.ConversationID).Where(assistantPendingStates).Count(&pending).Error; e != nil {
				return e
			}
			if pending > 0 {
				return fmt.Errorf("当前会话已有任务或待确认大纲，请先继续或取消该任务")
			}
		}
		if req.ConversationID == "" {
			req.ConversationID = uuid.NewString()
			c := domain.Conversation{ID: req.ConversationID, Nonce: uuid.NewString(), KBID: kb, AppID: app.ID, Subject: string([]rune(req.Message)[:min(len([]rune(req.Message)), 100)]), RemoteIP: remote, Info: domain.ConversationInfo{UserInfo: domain.UserInfo{UserID: user}}}
			if e := tx.Create(&c).Error; e != nil {
				return e
			}
		}
		retainIDs := append([]string{}, req.AttachmentIDs...)
		if req.TemplateRef != nil && req.TemplateRef.Kind == "upload" {
			retainIDs = append(retainIDs, req.TemplateRef.ID)
		}
		if len(retainIDs) > 0 {
			if e := tx.Model(&domain.AssistantUpload{}).Where("id IN ? AND user_id=? AND kb_id=? AND status<>'deleted' AND expires_at>now()", retainIDs, user, kb).Update("expires_at", u.expiry()).Error; e != nil {
				return e
			}
		}
		settings := map[string]any{"web_search_enabled": req.WebSearchEnabled}
		if saveTemplatePreference {
			settings["template_ref"] = preferenceTemplate
		}
		pref := domain.AssistantPreference{ConversationID: req.ConversationID, KBID: kb, UserID: user, Settings: assistantJSON(settings), UpdatedAt: time.Now()}
		if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}}, DoUpdates: clause.Assignments(map[string]any{"settings": gorm.Expr("assistant_preferences.settings || EXCLUDED.settings"), "updated_at": time.Now()})}).Create(&pref).Error; e != nil {
			return e
		}
		run.ConversationID = req.ConversationID
		run.Request = assistantJSON(req)
		for _, m := range []domain.ConversationMessage{{ID: run.UserMessageID, KBID: kb, AppID: app.ID, ConversationID: req.ConversationID, Role: schema.User, Content: req.Message, RemoteIP: remote}, {ID: run.AssistantMessageID, KBID: kb, AppID: app.ID, ConversationID: req.ConversationID, Role: schema.Assistant, Content: "", ParentID: run.UserMessageID, RemoteIP: remote}} {
			if e := tx.Create(&m).Error; e != nil {
				return e
			}
		}
		if e := tx.Create(run).Error; e != nil {
			if errors.Is(e, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("当前会话已有任务，请等待完成或取消后再提交")
			}
			return e
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	u.notifyRun(run.ID)
	return u.acceptedRun(ctx, run)
}
func (u *AssistantUsecase) Run(ctx context.Context, kb, user, id string) (*domain.AssistantRun, error) {
	var v domain.AssistantRun
	e := u.db.WithContext(ctx).Where("id=? AND kb_id=? AND user_id=?", id, kb, user).First(&v).Error
	if e != nil {
		return nil, ErrAssistantAccess
	}
	if e = u.checkSources(ctx, kb, user, v.Sources); e != nil {
		v.Answer = ""
		v.ArtifactID = ""
		v.Sources = nil
		v.Steps = domain.JSONB{}
		v.Error = "来源权限已变化，当前无法查看回答或产物。请按当前权限重新生成。"
		v.Checkpoint = nil
	}
	v.Planning = assistantPlan(&v)
	v.Generation = assistantGenerationProgress(&v)
	v.References = sourceList(v.Sources)
	var request domain.AssistantSubmit
	_ = assistantDecode(v.Request, &request)
	v.Question = request.Message
	u.decorateUploads(ctx, &v, request)
	if e = u.decorateAssistantArtifacts(ctx, kb, user, []*domain.AssistantRun{&v}); e != nil {
		return nil, e
	}
	return &v, nil
}
func (u *AssistantUsecase) Runs(ctx context.Context, kb, user, cid string) ([]domain.AssistantRun, error) {
	if e := u.OwnConversation(ctx, kb, user, cid); e != nil {
		return nil, e
	}
	var runs []domain.AssistantRun
	e := u.db.WithContext(ctx).Where("kb_id=? AND user_id=? AND conversation_id=?", kb, user, cid).Order("created_at ASC").Limit(100).Find(&runs).Error
	if e != nil {
		return nil, e
	}
	for i := range runs {
		var req domain.AssistantSubmit
		_ = assistantDecode(runs[i].Request, &req)
		runs[i].Question = req.Message
		u.decorateUploads(ctx, &runs[i], req)
		runs[i].References = sourceList(runs[i].Sources)
		if u.checkSources(ctx, kb, user, runs[i].Sources) != nil {
			runs[i].Answer = ""
			runs[i].Error = "来源权限已变更，暂不可查看"
			runs[i].ArtifactID = ""
			runs[i].Steps = domain.JSONB{}
			runs[i].References = nil
			runs[i].Checkpoint = nil
		}
		runs[i].Planning = assistantPlan(&runs[i])
		runs[i].Generation = assistantGenerationProgress(&runs[i])
	}
	refs := make([]*domain.AssistantRun, len(runs))
	for i := range runs {
		refs[i] = &runs[i]
	}
	if e = u.decorateAssistantArtifacts(ctx, kb, user, refs); e != nil {
		return nil, e
	}
	return runs, nil
}

func (u *AssistantUsecase) decorateAssistantArtifacts(ctx context.Context, kb, user string, runs []*domain.AssistantRun) error {
	ids := []string{}
	for _, r := range runs {
		if r.ArtifactID != "" {
			ids = append(ids, r.ArtifactID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var artifacts []domain.AssistantArtifact
	if e := u.db.WithContext(ctx).Select("id,run_id,conversation_id,family_id,revision,base_id,name,format,size,preview_state,expires_at,created_at").Where("id IN ? AND kb_id=? AND user_id=? AND expires_at>now()", ids, kb, user).Find(&artifacts).Error; e != nil {
		return e
	}
	byID := map[string]*domain.AssistantArtifact{}
	for i := range artifacts {
		byID[artifacts[i].ID] = &artifacts[i]
	}
	for _, r := range runs {
		if a := byID[r.ArtifactID]; a != nil && a.RunID == r.ID && a.ConversationID == r.ConversationID {
			r.Artifact = a
		}
	}
	return nil
}
func (u *AssistantUsecase) CancelRun(ctx context.Context, kb, user, id string) error {
	var r domain.AssistantRun
	if e := u.db.WithContext(ctx).Where("id=? AND kb_id=? AND user_id=?", id, kb, user).First(&r).Error; e != nil {
		return ErrAssistantAccess
	}
	// A paused user-facing task may have an internal failed state. Ending it
	// preserves its diagnostics and prevents a late retry from resurrecting it.
	res := u.db.WithContext(ctx).Model(&domain.AssistantRun{}).Where("id=?", r.ID).Where("(" + assistantPendingStates + " OR state='failed')").Updates(map[string]any{"state": "canceled", "stage": "已取消", "lease_epoch": gorm.Expr("lease_epoch+1"), "lease_until": nil, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("任务已结束，无法取消")
	}
	return nil
}
func (u *AssistantUsecase) RetryRun(ctx context.Context, kb, user, id string) error {
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", kb+":"+user).Error; err != nil {
			return err
		}
		var r domain.AssistantRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND kb_id=? AND user_id=?", id, kb, user).First(&r).Error; err != nil {
			return ErrAssistantAccess
		}
		if r.State != "failed" {
			return fmt.Errorf("仅失败任务可重试")
		}
		if err := u.checkSources(ctx, kb, user, r.Sources); err != nil {
			return err
		}
		var pending, active int64
		if err := tx.Model(&domain.AssistantRun{}).Where("conversation_id=? AND id<>?", r.ConversationID, id).Where(assistantPendingStates).Count(&pending).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.AssistantRun{}).Where("user_id=?", user).Where(assistantPendingStates).Count(&active).Error; err != nil {
			return err
		}
		if pending > 0 || active >= 2 {
			return fmt.Errorf("当前已有待处理任务，请先继续或取消该任务")
		}
		return tx.Model(&r).Updates(map[string]any{"state": "queued", "stage": "等待处理", "error": "", "attempt": 0, "lease_until": nil, "updated_at": time.Now()}).Error
	})
	if err == nil {
		u.notifyRun(id)
	}
	return err
}
func (u *AssistantUsecase) Artifact(ctx context.Context, kb, user, id string) (*domain.AssistantArtifact, error) {
	var a domain.AssistantArtifact
	if e := u.db.WithContext(ctx).Where("id=? AND kb_id=? AND user_id=? AND expires_at>?", id, kb, user, time.Now()).First(&a).Error; e != nil {
		return nil, ErrAssistantAccess
	}
	if e := u.checkSources(ctx, kb, user, a.Sources); e != nil {
		return nil, e
	}
	a.Outline = assistantArtifactOutline(&a)
	return &a, nil
}
func (u *AssistantUsecase) Content(ctx context.Context, kb, user, kind, id string, preview bool) (io.ReadCloser, string, string, error) {
	var key, name string
	switch kind {
	case "artifacts":
		a, e := u.Artifact(ctx, kb, user, id)
		if e != nil {
			return nil, "", "", e
		}
		key, name = a.ObjectKey, a.Name
		if preview {
			key = a.PreviewKey
			name += ".pdf"
		}
	case "uploads":
		v, e := u.GetUpload(ctx, kb, user, id)
		if e != nil {
			return nil, "", "", e
		}
		key, name = v.ObjectKey, v.Name
		if preview {
			key = v.PreviewKey
			name += ".pdf"
		}
	default:
		return nil, "", "", ErrAssistantAccess
	}
	if key == "" {
		return nil, "", "", fmt.Errorf("预览尚不可用，可下载原文件")
	}
	r, e := u.storage.GetObject(ctx, "", key, minio.GetObjectOptions{})
	if e != nil {
		return nil, "", "", e
	}
	if _, e = r.Stat(); e != nil {
		r.Close()
		return nil, "", "", e
	}
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	return r, name, ct, nil
}
func (u *AssistantUsecase) Preferences(ctx context.Context, kb, user, cid string, settings *domain.JSONB) (domain.JSONB, error) {
	if e := u.OwnConversation(ctx, kb, user, cid); e != nil {
		return nil, e
	}
	if settings != nil {
		for key := range *settings {
			if key != "web_search_enabled" && key != "template_ref" {
				return nil, fmt.Errorf("不支持的会话偏好")
			}
		}
		if v, ok := (*settings)["web_search_enabled"]; ok {
			enabled, valid := v.(bool)
			if !valid {
				return nil, fmt.Errorf("联网偏好必须是开关值")
			}
			if enabled && u.cfg.Assistant.SearchKey == "" {
				return nil, fmt.Errorf("联网搜索尚未配置")
			}
		}
		if v := (*settings)["template_ref"]; v != nil {
			b, _ := json.Marshal(v)
			var ref domain.AssistantTemplateRef
			if json.Unmarshal(b, &ref) != nil {
				return nil, fmt.Errorf("模板偏好无效")
			}
			if _, e := u.template(ctx, kb, user, ref); e != nil {
				return nil, e
			}
		}
	}
	if settings != nil {
		p := domain.AssistantPreference{ConversationID: cid, KBID: kb, UserID: user, Settings: *settings, UpdatedAt: time.Now()}
		e := u.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}}, DoUpdates: clause.AssignmentColumns([]string{"settings", "updated_at"})}).Create(&p).Error
		return p.Settings, e
	}
	var p domain.AssistantPreference
	e := u.db.WithContext(ctx).Where("conversation_id=?", cid).First(&p).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return domain.JSONB{}, nil
	}
	return p.Settings, e
}

func (u *AssistantUsecase) Documents(ctx context.Context, kb, user, query string) ([]map[string]any, error) {
	if user == "" || len([]rune(query)) > 100 {
		return nil, ErrAssistantAccess
	}
	var nodes []domain.Node
	q := u.db.WithContext(ctx).Where("kb_id=?", kb)
	if query != "" {
		q = q.Where("strpos(lower(name),lower(?))>0", query)
	}
	if e := q.Order("updated_at DESC").Limit(80).Find(&nodes).Error; e != nil {
		return nil, e
	}
	result := []map[string]any{}
	for _, n := range nodes {
		if _, e := u.authorizedNode(ctx, kb, user, n.ID); e == nil {
			result = append(result, map[string]any{"id": n.ID, "name": n.Name, "type": n.Type})
		}
	}
	return result, nil
}
func (u *AssistantUsecase) TemplateContent(ctx context.Context, kb, user, id string, version int) (io.ReadCloser, error) {
	v, e := u.template(ctx, kb, user, domain.AssistantTemplateRef{Kind: "enterprise", ID: id, Version: version})
	if e != nil {
		return nil, e
	}
	if v.PreviewKey == "" {
		return nil, fmt.Errorf("模板预览暂不可用")
	}
	r, e := u.storage.GetObject(ctx, "", v.PreviewKey, minio.GetObjectOptions{})
	if e != nil {
		return nil, e
	}
	if _, e = r.Stat(); e != nil {
		r.Close()
		return nil, e
	}
	return r, nil
}

func (u *AssistantUsecase) acceptedRun(ctx context.Context, r *domain.AssistantRun) (*domain.AssistantRun, error) {
	c, e := u.chat.conversationUsecase.repo.GetChatConversation(ctx, r.KBID, r.ConversationID)
	if e != nil {
		return nil, e
	}
	if c.Info.UserInfo.UserID != r.UserID {
		return nil, ErrAssistantAccess
	}
	visible, err := u.Run(ctx, r.KBID, r.UserID, r.ID)
	if err != nil {
		return nil, err
	}
	visible.SessionNonce = c.Nonce
	return visible, nil
}

func (u *AssistantUsecase) notifyRun(id string) {
	if u.cfg == nil || u.cfg.MQ.NATS.Server == "" {
		return
	}
	go func() {
		opts := []nats.Option{nats.Timeout(2 * time.Second), nats.NoReconnect()}
		if u.cfg.MQ.NATS.User != "" {
			opts = append(opts, nats.UserInfo(u.cfg.MQ.NATS.User, u.cfg.MQ.NATS.Password))
		}
		nc, e := nats.Connect(u.cfg.MQ.NATS.Server, opts...)
		if e != nil {
			return
		}
		defer nc.Close()
		js, e := nc.JetStream(nats.MaxWait(2 * time.Second))
		if e == nil {
			_, _ = js.Publish("pandawiki.assistant.wake", []byte(id), nats.MsgId(id))
		}
	}()
}

func (u *AssistantUsecase) TemplatePermissions(ctx context.Context, kb, id string, permissions domain.NodePermissions) error {
	normalized, e := u.chat.enterpriseUsecase.NormalizeNodePermissions(ctx, permissions)
	if e != nil {
		return e
	}
	result := u.db.WithContext(ctx).Model(&domain.AssistantTemplate{}).Where("id=? AND kb_id=?", id, kb).Updates(map[string]any{"permissions": normalized, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAssistantAccess
	}
	return nil
}

func (u *AssistantUsecase) SpreadsheetPreview(ctx context.Context, kb, user, id string) (domain.JSONB, error) {
	a, e := u.Artifact(ctx, kb, user, id)
	if e != nil {
		return nil, e
	}
	if a.Format != "xlsx" {
		return nil, fmt.Errorf("该文件不是Excel")
	}
	return domain.JSONB{"sheets": a.Spec["sheets"]}, nil
}
func (u *AssistantUsecase) derivedExpiry(ctx context.Context, sources domain.JSONB) time.Time {
	result := u.expiry()
	for _, s := range sourceList(sources) {
		id := ""
		if s.Kind == "upload" {
			id = s.ID
		}
		if s.Kind == "template" {
			var ref domain.AssistantTemplateRef
			if json.Unmarshal([]byte(s.Version), &ref) == nil && ref.Kind == "upload" {
				id = ref.ID
			}
		}
		if id != "" {
			var v domain.AssistantUpload
			if u.db.WithContext(ctx).Where("id=?", id).First(&v).Error == nil && v.ExpiresAt.Before(result) {
				result = v.ExpiresAt
			}
		}
	}
	return result
}

func (u *AssistantUsecase) decorateUploads(ctx context.Context, r *domain.AssistantRun, req domain.AssistantSubmit) {
	ids := append([]string{}, req.AttachmentIDs...)
	if req.TemplateRef != nil && req.TemplateRef.Kind == "upload" {
		ids = append(ids, req.TemplateRef.ID)
	}
	for _, id := range ids {
		v, e := u.GetUpload(ctx, r.KBID, r.UserID, id)
		if e == nil {
			r.Uploads = append(r.Uploads, *v)
		} else {
			r.Uploads = append(r.Uploads, domain.AssistantUpload{ID: id, Name: "资料已过期或被删除", Status: "unavailable"})
		}
	}
}
func AssistantErrorMessage(e error) string { return safeAssistantError(e) }

func (u *AssistantUsecase) RetryUpload(ctx context.Context, kb, user, id string) error {
	v, e := u.GetUpload(ctx, kb, user, id)
	if e != nil {
		return e
	}
	if v.Status != "failed" {
		return fmt.Errorf("仅处理失败的附件可重试")
	}
	return u.db.WithContext(ctx).Model(&domain.AssistantUpload{}).Where("id=? AND user_id=? AND status='failed'", id, user).Updates(map[string]any{"status": "queued", "error": "", "lease_epoch": gorm.Expr("lease_epoch+1"), "lease_until": nil, "updated_at": time.Now()}).Error
}
