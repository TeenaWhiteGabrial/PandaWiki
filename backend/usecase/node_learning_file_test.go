package usecase

import (
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestMediaLearningSkipsExtractionWithGenericMIME(t *testing.T) {
	for _, name := range []string{"培训.mp4", "培训.MP4", "录音.mp3", "图片.png", "归档.zip"} {
		release := &domain.NodeReleaseWithDirPath{NodeRelease: &domain.NodeRelease{}}
		release.Name = name
		release.Meta.FileType = "application/octet-stream"
		if ShouldExtractReleaseFileContent(release) {
			t.Errorf("%s with generic MIME must only index metadata", name)
		}
		release.Name = "培训材料"
		release.Meta.FileName = name
		if ShouldExtractReleaseFileContent(release) {
			t.Errorf("original filename %s must prevent binary extraction", name)
		}
	}
	release := &domain.NodeReleaseWithDirPath{NodeRelease: &domain.NodeRelease{}}
	release.Name = "培训.docx"
	release.Meta.FileType = "application/octet-stream"
	if !ShouldExtractReleaseFileContent(release) {
		t.Fatal("office documents must retain content extraction")
	}
}

func TestModelReindexReusesFullParsedContent(t *testing.T) {
	for _, tc := range []struct {
		name, content, scope string
		reuse, refresh       bool
	}{
		{"legacy PDF cached body", "正文内容", "", true, false},
		{"manual PDF retry", "正文内容", "", false, true},
		{"full cached PDF", "正文内容", "full", true, false},
		{"empty full PDF", "", "full", true, true},
		{"partial PDF", "部分正文", "partial", true, true},
		{"metadata PDF", "只有名称", "metadata", true, true},
		{"legacy metadata placeholder", "这是一个文件型文档。系统当前已索引该文件的基础元信息", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &domain.NodeReleaseWithDirPath{NodeRelease: &domain.NodeRelease{Name: "sample.pdf", Content: tc.content, Meta: domain.NodeMeta{FileKey: "kb/sample.pdf"}}}
			if tc.scope != "" {
				r.Meta.Extraction = &domain.FileExtraction{Scope: tc.scope}
			}
			if got := ShouldRefreshFileContentForIndex(r, tc.reuse); got != tc.refresh {
				t.Fatalf("refresh=%v want %v", got, tc.refresh)
			}
		})
	}
}
