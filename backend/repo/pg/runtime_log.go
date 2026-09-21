package pg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/domain"
)

func (r *SystemSettingRepo) WriteRuntimeLog(ctx context.Context, entry domain.RuntimeLog) error {
	return r.db.WithContext(ctx).Create(&entry).Error
}

func (r *SystemSettingRepo) ListRuntimeLogs(ctx context.Context, req domain.RuntimeLogListReq) (*domain.RuntimeLogListResp, error) {
	query := r.db.WithContext(ctx).Model(&domain.RuntimeLog{})
	if req.Level != "" {
		query = query.Where("level = ?", strings.ToLower(req.Level))
	}
	if req.Service != "" {
		query = query.Where("service = ?", req.Service)
	}
	if req.Stage != "" {
		query = query.Where("stage = ?", req.Stage)
	}
	if location := strings.TrimSpace(req.Location); location != "" {
		like := "%" + location + "%"
		query = query.Where("service ILIKE ? OR module ILIKE ? OR source ILIKE ?", like, like, like)
	}
	if from, err := time.Parse(time.RFC3339, req.From); err == nil {
		query = query.Where("event_time >= ?", from)
	}
	if to, err := time.Parse(time.RFC3339, req.To); err == nil {
		query = query.Where("event_time <= ?", to)
	}
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("safe_message ILIKE ? OR event_code ILIKE ? OR module ILIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	items := make([]domain.RuntimeLog, 0, pageSize)
	if err := query.Order("event_time DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return &domain.RuntimeLogListResp{Items: items, Total: total}, nil
}

func runtimeLogFromEntry(entryTime time.Time, level, service, module, source, message string, fields map[string]any, eventCode string) domain.RuntimeLog {
	if entryTime.IsZero() {
		entryTime = time.Now()
	}
	result := domain.RuntimeLog{
		EventTime:   entryTime,
		Level:       level,
		Service:     service,
		Module:      module,
		Source:      source,
		EventCode:   eventCode,
		SafeMessage: message,
		SafeFields:  domain.JSONB(fields),
	}
	result.TraceID = fieldString(fields, "trace_id")
	result.SpanID = fieldString(fields, "span_id")
	result.RequestID = fieldString(fields, "request_id")
	result.Stage = fieldString(fields, "stage")
	result.KBID = optionalField(fields, "kb_id")
	result.DocumentID = optionalField(fields, "document_id", "node_id")
	result.RevisionID = optionalField(fields, "revision_id", "node_release_id")
	result.JobID = optionalField(fields, "job_id")
	result.RunID = optionalField(fields, "run_id", "conversation_id")
	return result
}

func fieldString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			if result := strings.TrimSpace(toString(value)); result != "" {
				return result
			}
		}
	}
	return ""
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if result, ok := value.(string); ok {
		return result
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func optionalField(fields map[string]any, keys ...string) *string {
	value := fieldString(fields, keys...)
	if value == "" {
		return nil
	}
	return &value
}
