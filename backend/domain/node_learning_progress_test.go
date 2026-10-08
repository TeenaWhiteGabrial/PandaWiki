package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLearningProgressKeepsWaitDurationAcrossPolls(t *testing.T) {
	start := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	p := &LearningProgress{}
	p.Advance("rag_pending", "等待索引", start)
	p.Advance("rag_pending", "等待索引", start.Add(time.Hour))
	if len(p.Stages) != 1 || !p.Stages[0].StartedAt.Equal(start) || !p.LastProgressAt.Equal(start) {
		t.Fatal("a status check must not reset duration or masquerade as progress")
	}
	p.Advance("rag_running", "生成向量", start.Add(2*time.Hour))
	if p.Stages[0].FinishedAt == nil || !p.Stages[0].FinishedAt.Equal(start.Add(2*time.Hour)) {
		t.Fatal("previous queue duration must end when processing starts")
	}
	p.Advance("rag_pending", "迟到的排队通知", start.Add(3*time.Hour))
	if len(p.Stages) != 2 || p.Stages[1].Key != "rag_running" {
		t.Fatal("a delayed pending event must not regress the timeline")
	}
	p.Finish(start.Add(4 * time.Hour))
	p.Advance("rag_running", "迟到的运行通知", start.Add(5*time.Hour))
	p.Finish(start.Add(6 * time.Hour))
	if len(p.Stages) != 2 || !p.FinishedAt.Equal(start.Add(4*time.Hour)) {
		t.Fatal("finished timelines must not restart or extend")
	}
}

func TestLearningProgressSurvivesRagInfoSerialization(t *testing.T) {
	p := &LearningProgress{}
	p.Advance("metadata", "mp4 仅索引名称", time.Now())
	original := RagInfo{TaskID: "attempt-1", LearningProgress: p}
	data, err := original.Value()
	if err != nil {
		t.Fatal(err)
	}
	var restored RagInfo
	if err := restored.Scan(data); err != nil {
		t.Fatal(err)
	}
	if restored.LearningProgress == nil || len(restored.LearningProgress.Stages) != 1 {
		t.Fatal("timeline lost from database JSON")
	}
	encoded, err := json.Marshal(restored)
	if err != nil || !json.Valid(encoded) {
		t.Fatal("invalid API response")
	}
}
