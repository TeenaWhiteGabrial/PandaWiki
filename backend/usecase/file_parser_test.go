package usecase

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLocalOfficeFilesToMarkdown(t *testing.T) {
	dir := t.TempDir()

	docxPath := filepath.Join(dir, "数字孪生.docx")
	writeTestZip(t, docxPath, map[string]string{
		"word/document.xml": `<?xml version="1.0"?><w:document xmlns:w="w"><w:body><w:p><w:r><w:t>数字孪生正文解析成功</w:t></w:r></w:p></w:body></w:document>`,
	})
	docx, err := parseLocalFileToMarkdown(t.Context(), docxPath, "数字孪生.docx")
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if !strings.Contains(docx, "数字孪生正文解析成功") {
		t.Fatalf("docx content missing: %q", docx)
	}

	pptxPath := filepath.Join(dir, "演示.pptx")
	writeTestZip(t, pptxPath, map[string]string{
		"ppt/slides/slide1.xml": `<?xml version="1.0"?><p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>第一页重点内容</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	pptx, err := parseLocalFileToMarkdown(t.Context(), pptxPath, "演示.pptx")
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	if !strings.Contains(pptx, "第一页重点内容") || !strings.Contains(pptx, "第 1 页") {
		t.Fatalf("pptx content missing: %q", pptx)
	}

	xlsxPath := filepath.Join(dir, "表格.xlsx")
	writeTestZip(t, xlsxPath, map[string]string{
		"xl/workbook.xml":                `<?xml version="1.0"?><workbook xmlns:r="r"><sheets><sheet name="设备清单" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":     `<?xml version="1.0"?><Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":           `<?xml version="1.0"?><sst><si><t>设备名称</t></si><si><t>状态</t></si><si><t>空压机A</t></si><si><t>正常</t></si></sst>`,
		"xl/worksheets/sheet1.xml":       `<?xml version="1.0"?><worksheet><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row><row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2" t="s"><v>3</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/ignored.xml":      ``,
		"docProps/core.xml":              ``,
		"_rels/.rels":                    ``,
		"[Content_Types].xml":            ``,
		"xl/styles.xml":                  ``,
		"xl/theme/theme1.xml":            ``,
		"xl/_rels/unused-compat.xml":     ``,
		"xl/worksheets/_rels/sheet1.xml": ``,
	})
	xlsx, err := parseLocalFileToMarkdown(t.Context(), xlsxPath, "表格.xlsx")
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if !strings.Contains(xlsx, "设备清单") || !strings.Contains(xlsx, "空压机A") {
		t.Fatalf("xlsx content missing: %q", xlsx)
	}
}

func TestParseLocalCSVToMarkdown(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "设备.csv")
	if err := os.WriteFile(filePath, []byte("名称,状态\n空压机A,正常\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	md, err := parseLocalFileToMarkdown(t.Context(), filePath, "设备.csv")
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if !strings.Contains(md, "| 名称 | 状态 |") || !strings.Contains(md, "| 空压机A | 正常 |") {
		t.Fatalf("csv content missing: %q", md)
	}
}

func writeTestZip(t *testing.T, filePath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
