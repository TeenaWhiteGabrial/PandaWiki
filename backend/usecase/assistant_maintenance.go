package usecase

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

func (u *AssistantUsecase) managedPut(ctx context.Context, key string, data []byte, contentType string, r *domain.AssistantRun) error {
	runID := ""
	var epoch int64
	if r != nil {
		runID = r.ID
		epoch = r.LeaseEpoch
	}
	if e := u.db.WithContext(ctx).Exec("INSERT INTO assistant_objects(object_key,run_id,lease_epoch) VALUES(?,?,?) ON CONFLICT DO NOTHING", key, runID, epoch).Error; e != nil {
		return e
	}
	_, e := u.storage.PutObject(ctx, "", key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType, ContentDisposition: "attachment"})
	return e
}
func (u *AssistantUsecase) cleanupObjects(ctx context.Context) error {
	var keys []string
	e := u.db.WithContext(ctx).Raw(`SELECT o.object_key FROM assistant_objects o
 WHERE o.created_at < now()-interval '24 hours'
 AND NOT EXISTS (SELECT 1 FROM assistant_runs r WHERE r.id=o.run_id AND r.state IN ('queued','running','awaiting_input','awaiting_sources','awaiting_outline'))
 AND NOT EXISTS (SELECT 1 FROM assistant_uploads u WHERE u.expires_at>now() AND u.status<>'deleted' AND o.object_key IN (u.object_key,u.template_key,u.preview_key))
 AND NOT EXISTS (SELECT 1 FROM assistant_artifacts a WHERE a.expires_at>now() AND o.object_key IN (a.object_key,a.preview_key))
 ORDER BY o.created_at LIMIT 50`).Scan(&keys).Error
	if e != nil {
		return e
	}
	for _, key := range keys {
		if e = u.storage.RemoveObject(ctx, key); e != nil {
			return e
		}
		if e = u.db.WithContext(ctx).Exec("DELETE FROM assistant_objects WHERE object_key=?", key).Error; e != nil {
			return e
		}
	}
	return nil
}
func (u *AssistantUsecase) saveRunTrace(r *domain.AssistantRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var current domain.AssistantRun
	if e := u.db.WithContext(ctx).First(&current, "id=?", r.ID).Error; e != nil {
		return
	}
	var req domain.AssistantSubmit
	_ = assistantDecode(current.Request, &req)
	trace := &domain.ChatTrace{TraceID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s:%d", r.ID, r.LeaseEpoch))).String(), KBID: r.KBID, ConversationID: r.ConversationID, UserMessageID: r.UserMessageID, AssistantMessageID: r.AssistantMessageID, Question: req.Message, Status: current.State, StartedAt: r.CreatedAt, ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}
	trace.Model = assistantString(current.Checkpoint["model"])
	now := time.Now()
	if current.State != "running" && current.State != "queued" && !assistantWaiting(current.State) {
		trace.FinishedAt = &now
	}
	for name, value := range r.Steps {
		v, ok := value.(map[string]any)
		if !ok {
			continue
		}
		ms, _ := v["duration_ms"].(int64)
		if n, ok := v["duration_ms"].(float64); ok {
			ms = int64(n)
		}
		start, _ := v["started_at"].(time.Time)
		if raw, ok := v["started_at"].(string); ok {
			start, _ = time.Parse(time.RFC3339Nano, raw)
		}
		end := start.Add(time.Duration(ms) * time.Millisecond)
		trace.Steps = append(trace.Steps, domain.ChatTraceStep{Name: name, Status: "completed", StartedAt: start, FinishedAt: &end, DurationMS: ms})
	}
	output := map[string]any{"sources": sourceList(current.Sources), "artifact_id": current.ArtifactID}
	if coverage := current.Checkpoint["material_coverage"]; coverage != nil {
		output["material_coverage"] = coverage
	}
	if progress := assistantGenerationProgress(&current); progress != nil {
		output["generation"] = map[string]any{"completed": progress.Completed, "total": progress.Total, "unit": progress.Unit}
	}
	if plan := assistantPlan(&current); plan != nil {
		output["planning"] = plan
	}
	if diagnostics := current.Checkpoint["planning_diagnostics"]; diagnostics != nil {
		output["planning_diagnostics"] = diagnostics
	}
	if diagnostics := current.Checkpoint["content_generation_diagnostics"]; diagnostics != nil {
		output["content_generation_diagnostics"] = diagnostics
	}
	trace.Steps = append(trace.Steps, domain.ChatTraceStep{Name: current.Stage, Status: current.State, StartedAt: r.StageStartedAt, FinishedAt: &now, DurationMS: now.Sub(r.StageStartedAt).Milliseconds(), Input: map[string]any{"run_id": current.ID, "web_search_enabled": req.WebSearchEnabled, "public_search_query": req.PublicSearchQuery, "attachment_count": len(req.AttachmentIDs), "template": req.TemplateRef, "attempt": current.Attempt}, Output: output, Error: current.Error})
	sort.SliceStable(trace.Steps, func(i, j int) bool { return trace.Steps[i].StartedAt.Before(trace.Steps[j].StartedAt) })
	if e := u.chat.conversationUsecase.repo.SaveChatTrace(ctx, trace); e != nil {
		u.logger.Error("save assistant trace", log.Error(e))
	}
}
