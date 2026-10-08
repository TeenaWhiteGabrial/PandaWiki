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
