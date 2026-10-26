package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"github.com/chaitin/panda-wiki/config"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPDF(catalog string) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R " + catalog + " >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>"}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	off := []int{0}
	for i, obj := range objects {
		off = append(off, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(off))
	for _, n := range off[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", n)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(off), xref)
	return b.Bytes()
}

func TestUploadValidationChecksContentAndParsedPDFActions(t *testing.T) {
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	cases := []struct {
		name              string
		data              []byte
		imageOnly, accept bool
	}{
		{"ok.png", img.Bytes(), true, true}, {"spoof.png", []byte("<?php echo 'test'; ?>"), true, false}, {"wrong.jpg", img.Bytes(), false, false},
		{"payload.html", []byte("<html>test</html>"), false, false}, {"payload.svg", []byte("<svg/>"), true, false}, {"file.php", []byte("test"), false, false},
		{"safe.txt", []byte("<b>Stored as plain text</b>"), false, true}, {"not-image.txt", []byte("text"), true, false}, {"notes.txt", []byte("中文正文"), false, true},
		{"safe.pdf", testPDF(""), false, true},
		{"safe-open-page.pdf", testPDF("/OpenAction [3 0 R /Fit]"), false, true},
		{"safe-open-goto.pdf", testPDF("/OpenAction << /S /GoTo /D [3 0 R /Fit] >>"), false, true}, {"spoof.pdf", []byte("<html>test</html>"), false, false},
		{"active.pdf", testPDF("/OpenAction << /S /JavaScript /JS (void 0) >>"), false, false},
		{"escaped.pdf", testPDF("/Open#41ction << /S /Java#53cript /JS (void 0) >>"), false, false},
		{"link.pdf", testPDF("/A << /S /URI /URI (https://example.test/) >>"), false, true},
		{"bad-link.pdf", testPDF("/A << /S /URI /URI (javascript:void 0) >>"), false, false},
		{"film.mp4", append([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 8)...), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.data)
			mime, err := validateUpload(tc.name, r, int64(len(tc.data)), tc.imageOnly)
			if (err == nil) != tc.accept {
				t.Fatalf("accept=%v, mime=%s err=%v", tc.accept, mime, err)
			}
			pos, _ := r.Seek(0, 1)
			if pos != 0 {
				t.Fatal("source not rewound")
			}
		})
	}
}
func TestUploadValidationRejectsOfficeMacroAndZipSpoof(t *testing.T) {
	for _, macro := range []bool{false, true} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
			w, _ := z.Create(name)
			w.Write([]byte("test"))
		}
		if macro {
			w, _ := z.Create("word/vbaProject.bin")
			w.Write([]byte("test"))
		}
		z.Close()
		_, err := validateUpload("test.docx", bytes.NewReader(b.Bytes()), int64(b.Len()), false)
		if (err != nil) != macro {
			t.Fatalf("macro=%v: %v", macro, err)
		}
	}
	_, err := validateUpload("test.zip", bytes.NewReader([]byte("not zip")), 7, false)
	if err == nil {
		t.Fatal("fake zip accepted")
	}
	_, err = validateUpload("missing", bytes.NewReader([]byte("test")), 4, false)
	if err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatal(err)
	}
}

func TestUploadValidationLocalPDFSamples(t *testing.T) {
	samples := os.Getenv("PANDA_WIKI_TEST_PDF_SAMPLES")
	if samples == "" {
		t.Skip("set local PDF sample paths for compatibility verification")
	}
	for _, name := range filepath.SplitList(samples) {
		t.Run(filepath.Base(name), func(t *testing.T) {
			f, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = validateUpload("sample.pdf", f, info.Size(), false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOfficePreviewRequiresServiceCredentials(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		user, password, ok := r.BasicAuth()
		if !ok || user != "preview-user" || password != "preview-test-secret" {
			t.Error("conversion omitted configured service credentials")
		}
		if r.URL.Path != "/forms/libreoffice/convert" {
			t.Error("conversion route changed")
		}
		w.WriteHeader(400)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.Preview.BaseURL = server.URL
	u := &FileUsecase{config: cfg, httpClient: server.Client()}
	if _, err := u.ConvertOfficeToPDF(context.Background(), "test-kb", "test.docx", []byte("test")); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatal("missing preview credentials not rejected")
	}
	if hits != 0 {
		t.Fatal("anonymous conversion attempted")
	}
	cfg.Preview.Username = "preview-user"
	cfg.Preview.Password = "preview-test-secret"
	if _, err := u.ConvertOfficeToPDF(context.Background(), "test-kb", "test.docx", []byte("test")); err == nil {
		t.Fatal("bad conversion response accepted")
	}
	if hits != 1 {
		t.Fatal("authenticated conversion not requested")
	}
}
