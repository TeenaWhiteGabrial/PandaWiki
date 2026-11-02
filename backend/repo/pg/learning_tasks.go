package pg

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
)

// Keep these predicates in one place so counts, pages and bulk targets have
// identical meanings. task_status takes precedence over legacy RAG statuses.
const learningAvailable = `COALESCE(n.rag_info->>'status', '') = 'SUCCEEDED'`
const learningRunning = `(COALESCE(n.rag_info->>'task_status', '') IN ('waiting','running','cancel_requested') OR (COALESCE(n.rag_info->>'task_status','') = '' AND NOT (` + learningAvailable + `) AND COALESCE(n.rag_info->>'status','') IN ('PENDING','RUNNING','REINDEX')))`
const learningFailed = `(COALESCE(n.rag_info->>'task_status','') = 'failed' OR (NOT (` + learningAvailable + `) AND COALESCE(n.rag_info->>'status','') = 'FAILED'))`
const learningAttention = `(NOT (` + learningAvailable + `) OR COALESCE(n.rag_info->>'task_status','') IN ('failed','canceled'))`
const learningNeeds = `(NOT (` + learningRunning + `) AND ` + learningAttention + `)`

// A nil scope means global access. An empty non-nil scope means no access.
func (r *NodeRepository) learningTaskQuery(ctx context.Context, req domain.LearningTaskQuery, scope []string) *gorm.DB {
	q := r.db.WithContext(ctx).Table("nodes AS n").Where("n.kb_id = ? AND n.type = 2 AND n.status = 2", req.KBID)
	if scope != nil {
		q = q.Where("n.id IN ?", scope)
	}
	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		// Treat user input literally, rather than as SQL LIKE wildcards.
		pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(keyword) + "%"
		q = q.Where(`(n.name ILIKE ? OR n.rag_info->>'message' ILIKE ? OR n.rag_info->>'task_message' ILIKE ? OR n.rag_info->'learning_progress'->>'check_error' ILIKE ? OR n.rag_info->'learning_progress'->'stages'->-1->>'message' ILIKE ?
		OR n.nav_id IN (SELECT id FROM navs WHERE kb_id = ? AND name ILIKE ?)
		OR n.id IN (WITH RECURSIVE matches AS (
		 SELECT id FROM nodes WHERE kb_id = ? AND type = 1 AND name ILIKE ?
		 UNION SELECT child.id FROM nodes child JOIN matches parent ON child.parent_id = parent.id WHERE child.kb_id = ?
		) SELECT id FROM matches))`, pattern, pattern, pattern, pattern, pattern, req.KBID, pattern, req.KBID, pattern, req.KBID)
	}
	switch req.Filter {
	case "attention":
		q = q.Where(learningAttention)
	case "running":
		q = q.Where(learningRunning)
	case "failed":
		q = q.Where(learningFailed)
	case "unavailable":
		q = q.Where("NOT (" + learningAvailable + ")")
	case "answerable":
		q = q.Where(learningAvailable)
	case "needs_learning":
		q = q.Where(learningNeeds)
	}
	if req.OnlyRunning {
		q = q.Where(learningRunning)
	}
	return q
}

func (r *NodeRepository) LearningTaskSummary(ctx context.Context, kbID string, scope []string) (*domain.LearningTaskCounts, error) {
	var counts domain.LearningTaskCounts
	selectSQL := `COUNT(*) AS total,
	 COUNT(*) FILTER (WHERE ` + learningAttention + `) AS attention,
	 COUNT(*) FILTER (WHERE ` + learningRunning + `) AS running,
	 COUNT(*) FILTER (WHERE ` + learningFailed + `) AS failed,
	 COUNT(*) FILTER (WHERE NOT (` + learningAvailable + `)) AS unavailable,
	 COUNT(*) FILTER (WHERE ` + learningAvailable + `) AS answerable,
	 COUNT(*) FILTER (WHERE ` + learningNeeds + `) AS needs_learning`
	err := r.learningTaskQuery(ctx, domain.LearningTaskQuery{KBID: kbID}, scope).Select(selectSQL).Scan(&counts).Error
	return &counts, err
}

// Only the latest stage and small extraction summary travel with the list.
// Full timelines, parser warnings and identifiers are fetched on detail open.
const learningRagSummary = `jsonb_strip_nulls(jsonb_build_object(
	'status', n.rag_info->'status', 'message', n.rag_info->'message',
	'task_status', n.rag_info->'task_status', 'task_action', n.rag_info->'task_action', 'task_message', n.rag_info->'task_message',
	'learning_progress', CASE WHEN jsonb_typeof(n.rag_info->'learning_progress') = 'object' THEN jsonb_build_object(
	 'last_progress_at', n.rag_info->'learning_progress'->'last_progress_at',
	 'check_error', n.rag_info->'learning_progress'->'check_error',
	 'extraction', CASE WHEN n.rag_info->'learning_progress'->'extraction'->>'scope' IS NOT NULL THEN jsonb_build_object('scope', n.rag_info->'learning_progress'->'extraction'->'scope') ELSE NULL END,
	 'stages', CASE WHEN jsonb_typeof(n.rag_info->'learning_progress'->'stages') = 'array' AND n.rag_info->'learning_progress'->'stages' <> '[]'::jsonb THEN jsonb_build_array(n.rag_info->'learning_progress'->'stages'->-1) ELSE '[]'::jsonb END
	) ELSE NULL END)) AS rag_info`

func (r *NodeRepository) LearningTaskList(ctx context.Context, req domain.LearningTaskQuery, scope []string) (*domain.LearningTaskPage, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	result := &domain.LearningTaskPage{Page: req.Page, PageSize: req.PageSize, List: []domain.LearningTaskItem{}}
	var totals struct {
		Total   int64
		Running int64
	}
	if err := r.learningTaskQuery(ctx, req, scope).Select(`COUNT(*) AS total, COUNT(*) FILTER (WHERE ` + learningRunning + `) AS running`).Scan(&totals).Error; err != nil {
		return nil, err
	}
	result.Total, result.Running = totals.Total, totals.Running
	pages := max(1, int((result.Total+int64(req.PageSize)-1)/int64(req.PageSize)))
	result.Page = min(req.Page, pages)
	order := `CASE WHEN ` + learningFailed + ` THEN 0 WHEN ` + learningRunning + ` THEN 1 WHEN NOT (` + learningAvailable + `) THEN 2 ELSE 3 END, n.edit_time DESC, n.id ASC`
	if err := r.learningTaskQuery(ctx, req, scope).
		Select("n.id, n.name, n.type, n.status, n.parent_id, n.nav_id, n.edit_time AS updated_at, " + learningRagSummary).
		Order(order).Offset((result.Page - 1) * req.PageSize).Limit(req.PageSize).Scan(&result.List).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.List))
	for _, item := range result.List {
		ids = append(ids, item.ID)
	}
	paths, err := r.learningFolderPaths(ctx, req.KBID, ids)
	if err != nil {
		return nil, err
	}
	for i := range result.List {
		result.List[i].FolderPath = paths[result.List[i].ID]
	}
	return result, nil
}

func (r *NodeRepository) LearningTaskTargets(ctx context.Context, req domain.LearningTaskQuery, scope []string) ([]string, error) {
	ids := []string{}
	err := r.learningTaskQuery(ctx, req, scope).Order("n.id ASC").Pluck("n.id", &ids).Error
	return ids, err
}

func (r *NodeRepository) LearningTaskDetail(ctx context.Context, kbID, id string, scope []string) (*domain.LearningTaskItem, error) {
	var item domain.LearningTaskItem
	err := r.learningTaskQuery(ctx, domain.LearningTaskQuery{KBID: kbID}, scope).
		Select("n.id, n.name, n.type, n.status, n.parent_id, n.nav_id, n.edit_time AS updated_at, COALESCE(n.rag_info, '{}'::jsonb) AS rag_info").Where("n.id = ?", id).Take(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Expand only the page's ancestors. Never fetch full node metadata or build
// permission labels to display a task's location. Cyclic data terminates safely.
func (r *NodeRepository) learningFolderPaths(ctx context.Context, kbID string, ids []string) (map[string]string, error) {
	paths := map[string]string{}
	if len(ids) == 0 {
		return paths, nil
	}
	var rows []struct {
		ID   string
		Path string
	}
	err := r.db.WithContext(ctx).Raw(`WITH RECURSIVE ancestors AS (
	 SELECT n.id AS origin, n.parent_id, n.nav_id, ''::text AS folder, ARRAY[n.id]::text[] AS visited, 0 AS depth
	 FROM nodes n WHERE n.kb_id = ? AND n.id IN ?
	 UNION ALL
	 SELECT a.origin, p.parent_id, a.nav_id, p.name || CASE WHEN a.folder = '' THEN '' ELSE ' / ' || a.folder END, a.visited || p.id::text, a.depth + 1
	 FROM ancestors a JOIN nodes p ON p.id = a.parent_id AND p.kb_id = ? AND p.nav_id = a.nav_id
	 WHERE p.type = 1 AND NOT p.id::text = ANY(a.visited) AND a.depth < 100
	), deepest AS (SELECT DISTINCT ON (origin) origin, nav_id, folder FROM ancestors ORDER BY origin, depth DESC)
	SELECT d.origin AS id, COALESCE(NULLIF(nav.name, ''), '未命名目录') || CASE WHEN d.folder = '' THEN '' ELSE ' / ' || d.folder END AS path
	FROM deepest d LEFT JOIN navs nav ON nav.id = d.nav_id AND nav.kb_id = ?`, kbID, ids, kbID, kbID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load task folder paths: %w", err)
	}
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	return paths, nil
}
