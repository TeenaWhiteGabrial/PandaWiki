package rag

import (
	"strings"
	"testing"
)

func TestBuildSearchableMarkdownIndexesTitleWhenContentIsEmpty(t *testing.T) {
	markdown := buildSearchableMarkdown("煤矿演示视频.mp4", "")

	for _, want := range []string{
		"# 煤矿演示视频.mp4",
		"文档名称：煤矿演示视频.mp4",
		"文档关键词：煤矿演示视频",
		"文件类型：mp4",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("searchable markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestBuildSearchableMarkdownKeepsParsedContent(t *testing.T) {
	markdown := buildSearchableMarkdown("数字孪生.md", "数字孪生是一种虚实映射技术。")

	for _, want := range []string{
		"文档名称：数字孪生.md",
		"文档关键词：数字孪生",
		"数字孪生是一种虚实映射技术。",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("searchable markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestBuildSearchableMarkdownWithoutTitleReturnsContent(t *testing.T) {
	content := "只有正文"
	if got := buildSearchableMarkdown("", content); got != content {
		t.Fatalf("expected original content, got %q", got)
	}
}
