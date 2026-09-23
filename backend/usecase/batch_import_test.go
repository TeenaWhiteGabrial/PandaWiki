package usecase

import (
	"archive/zip"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDecodeBatchImportEntryNameGB18030(t *testing.T) {
	encoded, err := simplifiedchinese.GB18030.NewEncoder().String("可行性研究报告/文档.txt")
	if err != nil {
		t.Fatalf("encode GB18030 name: %v", err)
	}

	name, err := decodeBatchImportEntryName(&zip.File{
		FileHeader: zip.FileHeader{Name: encoded, NonUTF8: true},
	})
	if err != nil {
		t.Fatalf("decode entry name: %v", err)
	}
	if name != "可行性研究报告/文档.txt" {
		t.Fatalf("decoded name = %q, want %q", name, "可行性研究报告/文档.txt")
	}
}

func TestDecodeBatchImportEntryNameKeepsUTF8(t *testing.T) {
	name, err := decodeBatchImportEntryName(&zip.File{
		FileHeader: zip.FileHeader{Name: "行业级/报告.txt"},
	})
	if err != nil {
		t.Fatalf("decode entry name: %v", err)
	}
	if name != "行业级/报告.txt" {
		t.Fatalf("decoded name = %q, want %q", name, "行业级/报告.txt")
	}
}
