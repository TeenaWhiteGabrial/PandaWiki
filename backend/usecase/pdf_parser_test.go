package usecase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/domain"
)

func TestPDFServiceProgressCoverageAndCleanup(t *testing.T) {
	var deleted atomic.Bool
	var uploaded atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing parser authentication")
		}
		switch {
		case r.Method == http.MethodPost:
			data, _ := io.ReadAll(r.Body)
			if !strings.HasPrefix(string(data), "%PDF-") {
				t.Error("missing uploaded PDF")
			}
			if uploaded.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"fixture"}`)
		case r.Method == http.MethodDelete:
			deleted.Store(true)
		case strings.HasSuffix(r.URL.Path, "/text"):
			io.WriteString(w, "Recovered document text")
		default:
			io.WriteString(w, `{"state":"completed","events":[{"sequence":1,"stage":"pdf_repair","message":"修复对象索引"},{"sequence":2,"stage":"pdf_ocr","message":"第 2 / 3 页"}],"result":{"scope":"partial","parser":"Poppler + Tesseract","page_count":3,"extracted_pages":2,"ocr_pages":1,"repaired":true,"warnings":["第 3 页未识别"],"message":"覆盖 2 / 3 页"}}`)
		}
	}))
	defer server.Close()
	t.Setenv("PDF_PARSER_URL", server.URL)
	t.Setenv("PDF_PARSER_TOKEN", "test-token")
	stages := []string{}
	ctx := WithLearningStageReporter(t.Context(), func(stage, _ string) { stages = append(stages, stage) })
	result := parsePDFWithService(ctx, writeTestPDF(t, false), "test.pdf")
	if !result.OK || !strings.Contains(result.Content, "Recovered document text") || result.Extraction.Scope != "partial" ||
		!result.Extraction.Repaired || result.Extraction.OCRPages != 1 || result.Extraction.ExtractedPages != 2 {
		t.Fatalf("unexpected extraction: %+v", result)
	}
	if !deleted.Load() || uploaded.Load() != 2 || !strings.Contains(strings.Join(stages, ","), "pdf_repair,pdf_ocr") {
		t.Fatalf("missing retry/progress/cleanup: uploaded=%d deleted=%v stages=%v", uploaded.Load(), deleted.Load(), stages)
	}
}

func TestPDFServiceCancellationDeletesRemoteJob(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"cancelled"}`)
		case http.MethodDelete:
			deleted.Store(true)
		default:
			<-r.Context().Done()
		}
	}))
	defer server.Close()
	t.Setenv("PDF_PARSER_URL", server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	result := parsePDFWithService(ctx, writeTestPDF(t, false), "cancel.pdf")
	if result.OK || !deleted.Load() {
		t.Fatalf("cancelled task must fail and clean up: %+v, deleted=%v", result, deleted.Load())
	}
}

func TestPDFServiceFailureIsNotSuccessfulText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"failed"}`)
		} else {
			io.WriteString(w, `{"state":"failed","error":"PDF 已加密，无法读取"}`)
		}
	}))
	defer server.Close()
	t.Setenv("PDF_PARSER_URL", server.URL)
	result := parsePDFWithService(t.Context(), writeTestPDF(t, false), "encrypted.pdf")
	if result.OK || result.Content != "" || !strings.Contains(result.Message, "已加密") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRefreshPDFMetadataAndPartialContent(t *testing.T) {
	release := &domain.NodeReleaseWithDirPath{NodeRelease: &domain.NodeRelease{}}
	release.Name, release.Content, release.Meta.FileKey = "test.pdf", "cached text", "kb/test.pdf"
	if !ShouldRefreshFileContent(release) {
		t.Fatal("legacy PDF needs the new parser")
	}
	for _, scope := range []string{"metadata", "partial", "full"} {
		release.Meta.Extraction = &domain.FileExtraction{Scope: scope}
		if ShouldRefreshFileContent(release) != (scope != "full") {
			t.Fatalf("unexpected refresh decision for %s", scope)
		}
	}
	release.Name = "test.mp4"
	release.Meta.Extraction.Scope = "metadata"
	if ShouldRefreshFileContent(release) {
		t.Fatal("media metadata must never trigger body parsing")
	}
}

func TestLivePDFParserService(t *testing.T) {
	address := os.Getenv("PANDA_WIKI_TEST_PDF_PARSER_URL")
	if address == "" {
		t.Skip("set PANDA_WIKI_TEST_PDF_PARSER_URL for real parser integration")
	}
	t.Setenv("PDF_PARSER_URL", address)
	for _, broken := range []bool{false, true} {
		result := parsePDFWithService(t.Context(), writeTestPDF(t, broken), "synthetic.pdf")
		if !result.OK || result.Extraction.Scope != "full" || !strings.Contains(result.Content, "Healthy PDF") {
			t.Fatalf("real parser failed, broken=%v: %+v", broken, result)
		}
	}
}
