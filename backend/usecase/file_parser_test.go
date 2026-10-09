package usecase

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePDFRecoversBrokenObjectReference(t *testing.T) {
	filePath := writeTestPDF(t, true)
	content, err := parseLocalFileToMarkdown(t.Context(), filePath, "broken.pdf")
	if err == nil || !strings.Contains(err.Error(), "PDF 正文解析异常") ||
		!strings.Contains(err.Error(), "loading {2 0}: found {3 0}") {
		t.Fatalf("expected recovered PDF object-reference error, got content=%q err=%v", content, err)
	}
	if content != "" {
		t.Fatalf("broken PDF must not return partial content: %q", content)
	}

	// A bad document must not stop subsequent documents from being parsed.
	content, err = parseLocalFileToMarkdown(t.Context(), writeTestPDF(t, false), "healthy.pdf")
	if err != nil || !strings.Contains(content, "Healthy PDF") {
		t.Fatalf("parse healthy PDF after broken PDF: content=%q err=%v", content, err)
	}
}

func TestParsePDFReadableAndMissingFiles(t *testing.T) {
	content, err := parsePDFFile(writeTestPDF(t, false), "healthy.pdf")
	if err != nil || !strings.Contains(content, "Healthy PDF") {
		t.Fatalf("parse healthy PDF: content=%q err=%v", content, err)
	}
	content, err = parsePDFFile(filepath.Join(t.TempDir(), "missing.pdf"), "missing.pdf")
	if err == nil || !strings.Contains(err.Error(), "PDF 打开失败") || content != "" {
		t.Fatalf("expected normal file-open error: content=%q err=%v", content, err)
	}
}

func writeTestPDF(t *testing.T, brokenPagesReference bool) string {
	t.Helper()
	stream := "BT /F1 12 Tf 72 720 Td (Healthy PDF) Tj ET\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var document strings.Builder
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	// Match the production crash: an xref entry points at a different object.
	if brokenPagesReference {
		offsets[1] = offsets[2]
	}
	xrefOffset := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&document, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	filePath := filepath.Join(t.TempDir(), "fixture.pdf")
	if err := os.WriteFile(filePath, []byte(document.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath
}

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
