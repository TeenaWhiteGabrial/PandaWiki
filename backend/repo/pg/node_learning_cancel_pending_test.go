package pg

import (
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func TestCancelPendingLearningProtectsUploadsAndSurvivesRestart(t *testing.T) {
	db := securityTestDB(t)
	r := &NodeRepository{db: db}
	ctx := t.Context()
	kbID, otherKB := uuid.NewString(), uuid.NewString()
	makeTask := func(kb string, status consts.NodeLearningTaskStatus, docID string) domain.NodeLearningTask {
		t.Helper()
		task := domain.NodeLearningTask{ID: uuid.NewString(), KBID: kb, NodeID: uuid.NewString(), NodeReleaseID: uuid.NewString(), Status: status, Action: consts.NodeLearningTaskActionRebuild, DocID: docID}
		node := domain.Node{ID: task.NodeID, KBID: kb, RagInfo: domain.RagInfo{TaskID: task.ID, TaskStatus: status, Status: consts.NodeRagStatusSucceeded, ActiveDocID: "existing-index", LearningProgress: &domain.LearningProgress{}}}
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		return task
	}
	waiting := makeTask(kbID, consts.NodeLearningTaskWaiting, "")
	parsing := makeTask(kbID, consts.NodeLearningTaskRunning, "")
	started := makeTask(kbID, consts.NodeLearningTaskRunning, "")
	submitted := makeTask(kbID, consts.NodeLearningTaskRunning, "accepted-doc")
	other := makeTask(otherKB, consts.NodeLearningTaskWaiting, "")
	if ok, err := r.BeginLearningSubmission(ctx, kbID, started.NodeID, started.ID); err != nil || !ok {
		t.Fatal(ok, err)
	}
	stamp := time.Now().UTC().Truncate(time.Microsecond)
	boundary := domain.VectorQueueBoundary{Sequence: 80, PublishedAt: stamp}
	count, protected, err := r.CancelPendingLearning(ctx, kbID, boundary)
	if err != nil || count != 2 || protected != 2 {
		t.Fatalf("count=%d protected=%d err=%v", count, protected, err)
	}
	for _, task := range []domain.NodeLearningTask{waiting, parsing} {
		stored, err := r.GetNodeLearningTaskByID(ctx, task.ID)
		if err != nil || stored.Status != consts.NodeLearningTaskCanceled {
			t.Fatal(stored, err)
		}
		if ok, err := r.BeginLearningSubmission(ctx, kbID, task.NodeID, task.ID); err != nil || ok {
			t.Fatal("canceled task still submitted", ok, err)
		}
		if err := r.UpdateNodeRagInfo(ctx, kbID, task.NodeID, domain.RagInfo{TaskID: task.ID, TaskStatus: consts.NodeLearningTaskRunning}); err != nil {
			t.Fatal(err)
		}
		node, err := r.GetNodeByID(ctx, task.NodeID)
		if err != nil || node.RagInfo.TaskStatus != consts.NodeLearningTaskCanceled || node.RagInfo.ActiveDocID != "existing-index" {
			t.Fatal("cancellation lost to delayed dispatcher or erased existing index", node, err)
		}
	}
	for _, task := range []domain.NodeLearningTask{started, submitted, other} {
		stored, err := r.GetNodeLearningTaskByID(ctx, task.ID)
		if err != nil || stored.Status != task.Status || stored.DocID != task.DocID {
			t.Fatal("accepted submission or other knowledge base was changed", stored, err)
		}
	}
	// A freshly constructed repository represents a restarted API/Consumer.
	restarted := &NodeRepository{db: db}
	for _, check := range []struct {
		kb       string
		seq      uint64
		at       time.Time
		canceled bool
	}{
		{kbID, 79, stamp.Add(-time.Second), true},
		{kbID, 80, stamp, true},
		{kbID, 81, stamp.Add(time.Second), false},
		{kbID, 1, stamp.Add(time.Second), false}, // recreated stream
		{otherKB, 79, stamp, false},
	} {
		got, err := restarted.LegacyLearningCanceled(ctx, check.kb, check.seq, check.at)
		if err != nil || got != check.canceled {
			t.Fatalf("legacy message %+v: got=%v err=%v", check, got, err)
		}
		allowed, err := restarted.BeginLegacyLearningSubmission(ctx, check.kb, check.seq, check.at)
		if err != nil || allowed == check.canceled {
			t.Fatal("pre-upload gate ignored cutoff", allowed, err)
		}
	}
	if _, err := restarted.LegacyLearningCanceled(ctx, kbID, 0, time.Time{}); err == nil {
		t.Fatal("missing queue metadata silently bypassed cancellation")
	}
	count, _, err = restarted.CancelPendingLearning(ctx, kbID, domain.VectorQueueBoundary{Sequence: 70, PublishedAt: stamp.Add(-time.Second)})
	if err != nil || count != 0 {
		t.Fatal("repeat cancellation is not idempotent", count, err)
	}
	if canceled, err := restarted.LegacyLearningCanceled(ctx, kbID, 80, stamp); err != nil || !canceled {
		t.Fatal("repeat request moved cutoff backwards", err)
	}
	future := makeTask(kbID, consts.NodeLearningTaskRunning, "")
	if ok, err := restarted.BeginLearningSubmission(ctx, kbID, future.NodeID, future.ID); err != nil || !ok {
		t.Fatal("new explicit task was blocked by legacy cutoff", ok, err)
	}
	// The existing selected-document cancel path obeys the same upload boundary.
	tasks, err := restarted.CancelActiveNodeLearningTasks(ctx, kbID, []string{started.NodeID, submitted.NodeID, future.NodeID}, "cancel")
	if err != nil || len(tasks) != 0 {
		t.Fatal("selected cancel interrupted a claimed/accepted upload", tasks, err)
	}
}
