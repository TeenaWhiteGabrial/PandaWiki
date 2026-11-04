package pg

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func TestLearningTaskPagesCountsTargetsAndDetails(t *testing.T) {
	db := securityTestDB(t)
	r := &NodeRepository{db: db}
	ctx := context.Background()
	kb, nav, folder := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, item := range []any{
		&domain.KnowledgeBase{ID: kb, Name: "task-page-fixture"},
		&domain.Nav{ID: nav, KbID: kb, Name: "测试栏目"},
		&domain.Node{ID: folder, KBID: kb, NavId: nav, Name: "folder_100%", Type: domain.NodeTypeFolder, ParentID: nav},
	} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	infos := []domain.RagInfo{
		{},
		{Status: consts.NodeRagStatusPending},
		{Status: consts.NodeRagStatusRunning},
		{Status: consts.NodeRagStatusReindexing},
		{Status: consts.NodeRagStatusSucceeded},
		{Status: consts.NodeRagStatusSucceeded, TaskStatus: consts.NodeLearningTaskRunning},
		{Status: consts.NodeRagStatusSucceeded, TaskStatus: consts.NodeLearningTaskFailed},
		{Status: consts.NodeRagStatusSucceeded, TaskStatus: consts.NodeLearningTaskCanceled},
		{Status: consts.NodeRagStatusFailed},
		{Status: consts.NodeRagStatusPending, TaskStatus: consts.NodeLearningTaskCanceled},
		{Status: consts.NodeRagStatusPending, TaskStatus: consts.NodeLearningTaskSucceeded},
		{TaskStatus: consts.NodeLearningTaskWaiting},
		{TaskStatus: consts.NodeLearningTaskCancelRequested},
	}
	var nodes []domain.Node
	for i, info := range infos {
		info.LearningProgress = &domain.LearningProgress{DocID: "private-index-id", Stages: []domain.LearningStage{{Key: "download", Message: "old step", StartedAt: now}, {Key: "rag_running", Message: "current progress", StartedAt: now}}, Extraction: &domain.FileExtraction{Scope: "partial", Warnings: []string{strings.Repeat("large parser warning", 100)}}}
		nodes = append(nodes, domain.Node{ID: uuid.NewString(), KBID: kb, NavId: nav, ParentID: folder, Type: domain.NodeTypeDocument, Status: domain.NodeStatusPublished, Name: fmt.Sprintf("document-%02d", i), EditTime: now, RagInfo: info})
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	// Unpublished documents and folders are never tasks in this panel.
	if err := db.Create(&domain.Node{ID: uuid.NewString(), KBID: kb, NavId: nav, Type: domain.NodeTypeDocument, Status: domain.NodeStatusDraft, Name: "not published"}).Error; err != nil {
		t.Fatal(err)
	}
	counts, err := r.LearningTaskSummary(ctx, kb, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := domain.LearningTaskCounts{Total: 13, Attention: 11, Running: 6, Completed: 1, Failed: 2, Unavailable: 9, Answerable: 4, NeedsLearning: 6}
	if *counts != wantCounts {
		t.Fatalf("wrong status semantics: %+v want %+v", counts, wantCounts)
	}
	for filter, want := range map[string]int64{"all": 13, "attention": 11, "running": 6, "failed": 2, "unavailable": 9, "answerable": 4, "needs_learning": 6} {
		req := domain.LearningTaskQuery{KBID: kb, Filter: filter, Page: 1, PageSize: 2}
		page, err := r.LearningTaskList(ctx, req, nil)
		if err != nil || page.Total != want || len(page.List) > 2 {
			t.Fatalf("%s: %+v %v", filter, page, err)
		}
		ids, err := r.LearningTaskTargets(ctx, req, nil)
		if err != nil || int64(len(ids)) != want {
			t.Fatalf("bulk targets truncated to page: %s %v %v", filter, ids, err)
		}
	}
	first, err := r.LearningTaskList(ctx, domain.LearningTaskQuery{KBID: kb, PageSize: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.LearningTaskList(ctx, domain.LearningTaskQuery{KBID: kb, Page: 2, PageSize: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range first.List {
		for _, b := range second.List {
			if a.ID == b.ID {
				t.Fatal("unstable pagination")
			}
		}
	}
	if first.List[0].FolderPath != "测试栏目 / folder_100%" {
		t.Fatal(first.List[0].FolderPath)
	}
	info := first.List[0].RagInfo
	if len(info.LearningProgress.Stages) != 1 || info.LearningProgress.Stages[0].Key != "rag_running" || info.LearningProgress.DocID != "" || len(info.LearningProgress.Extraction.Warnings) != 0 {
		t.Fatalf("list returned full timeline: %+v", info)
	}
	detail, err := r.LearningTaskDetail(ctx, kb, first.List[0].ID, nil)
	if err != nil || len(detail.RagInfo.LearningProgress.Stages) != 2 || detail.RagInfo.LearningProgress.DocID != "private-index-id" {
		t.Fatalf("detail lost full timeline: %+v %v", detail, err)
	}
	for _, keyword := range []string{"folder_100%", "测试栏目", "current progress"} {
		ids, err := r.LearningTaskTargets(ctx, domain.LearningTaskQuery{KBID: kb, Keyword: keyword}, nil)
		if err != nil || len(ids) != len(nodes) {
			t.Fatalf("path/progress search %s: %d %v", keyword, len(ids), err)
		}
	}
	scope := []string{nodes[4].ID}
	scopedCounts, err := r.LearningTaskSummary(ctx, kb, scope)
	if err != nil || scopedCounts.Answerable != 1 || scopedCounts.Completed != 0 {
		t.Fatalf("old usable index was counted as a completed latest attempt: %+v %v", scopedCounts, err)
	}
	scopedCounts, err = r.LearningTaskSummary(ctx, kb, []string{nodes[10].ID})
	if err != nil || scopedCounts.Completed != 1 {
		t.Fatalf("completed attempt count missing or leaked across scope: %+v %v", scopedCounts, err)
	}
	ids, err := r.LearningTaskTargets(ctx, domain.LearningTaskQuery{KBID: kb}, scope)
	if err != nil || !slices.Equal(ids, scope) {
		t.Fatalf("scope leaked: %v %v", ids, err)
	}
	empty, err := r.LearningTaskSummary(ctx, kb, []string{})
	if err != nil || empty.Total != 0 {
		t.Fatalf("empty scope treated as global: %+v %v", empty, err)
	}
	if _, err := r.LearningTaskDetail(ctx, kb, nodes[0].ID, scope); err == nil {
		t.Fatal("unmanaged detail leaked")
	}
	// Historical rows can have SQL NULL, JSON null and missing timeline arrays.
	if err := db.Exec(`UPDATE nodes SET rag_info = '{"learning_progress":{"stages":null}}'::jsonb WHERE id = ?`, nodes[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE nodes SET rag_info = NULL WHERE id = ?`, nodes[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if detail, err := r.LearningTaskDetail(ctx, kb, nodes[1].ID, nil); err != nil || detail.RagInfo.Status != "" {
		t.Fatalf("legacy NULL detail: %+v %v", detail, err)
	}
	if _, err := r.LearningTaskList(ctx, domain.LearningTaskQuery{KBID: kb, PageSize: 100}, nil); err != nil {
		t.Fatal("legacy null progress:", err)
	}
	page, err := r.LearningTaskList(ctx, domain.LearningTaskQuery{KBID: kb, Page: 100, PageSize: 10}, nil)
	if err != nil || page.Page != 2 || len(page.List) != 3 {
		t.Fatalf("out-of-range page: %+v %v", page, err)
	}
}
