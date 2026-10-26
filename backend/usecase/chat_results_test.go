package usecase

import (
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestFilterRankedNodesWithUsableContent(t *testing.T) {
	titleOnly := &domain.RankedNodeChunks{
		NodeID:   "title-only",
		NodeName: "产品手册.pdf",
		Chunks: []*domain.NodeContentChunk{{
			Content: "#产品手册.pdf\n# 产品手册.pdf",
		}},
	}
	withBody := &domain.RankedNodeChunks{
		NodeID:   "with-body",
		NodeName: "产品手册.pdf",
		Chunks: []*domain.NodeContentChunk{
			{Content: "#产品手册.pdf\n# 产品手册.pdf"},
			{Content: "门户支持统一登录注册和顶部导航。"},
		},
	}

	filtered, excluded := filterRankedNodesWithUsableContent([]*domain.RankedNodeChunks{titleOnly, withBody})
	if len(filtered) != 1 {
		t.Fatalf("expected only the document with body content, got %d", len(filtered))
	}
	if filtered[0].NodeID != "with-body" {
		t.Fatalf("unexpected retained document: %q", filtered[0].NodeID)
	}
	if len(filtered[0].Chunks) != 1 || filtered[0].Chunks[0].Content != "门户支持统一登录注册和顶部导航。" {
		t.Fatalf("expected title-only chunk to be removed, got %#v", filtered[0].Chunks)
	}
	if len(excluded) != 1 || excluded[0]["node_id"] != "title-only" {
		t.Fatalf("expected title-only document to be reported as excluded, got %#v", excluded)
	}
}

func TestRankedNodesCitedByAnswer(t *testing.T) {
	cited := &domain.RankedNodeChunks{NodeID: "source-1"}
	notCited := &domain.RankedNodeChunks{NodeID: "source-2"}

	got := rankedNodesCitedByAnswer("根据材料可知。\n> [1]. [资料](/node/source-1)", []*domain.RankedNodeChunks{cited, notCited})
	if len(got) != 1 || got[0].NodeID != cited.NodeID {
		t.Fatalf("expected only cited document, got %#v", got)
	}

	got = rankedNodesCitedByAnswer("<think>检查 /node/source-1</think>抱歉，我当前的知识不足以回答这个问题。", []*domain.RankedNodeChunks{cited})
	if len(got) != 0 {
		t.Fatalf("expected no sources for an insufficient answer, got %#v", got)
	}

	got = rankedNodesCitedByAnswer("<think>可能要引用 /node/source-1</think>暂时无法回答。", []*domain.RankedNodeChunks{cited})
	if len(got) != 0 {
		t.Fatalf("expected reasoning-only citations to be hidden, got %#v", got)
	}

	got = rankedNodesCitedByAnswer("<think>可能要引用 /node/source-1", []*domain.RankedNodeChunks{cited})
	if len(got) != 0 {
		t.Fatalf("expected incomplete reasoning to be hidden, got %#v", got)
	}
}
