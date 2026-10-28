package share

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	v1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/labstack/echo/v4"
)

type fileAccessStub struct {
	denied      bool
	meta        domain.NodeMeta
	detailCalls int
}

func (s *fileAccessStub) ValidateNodePerm(context.Context, string, string, string) *domain.PWResponseErrCode {
	if s.denied {
		return &domain.ErrCodePermissionDenied
	}
	return nil
}
func (s *fileAccessStub) GetNodeReleaseDetailByKBIDAndID(context.Context, string, string, string) (*v1.ShareNodeDetailResp, error) {
	s.detailCalls++
	return &v1.ShareNodeDetailResp{Meta: s.meta}, nil
}
func (s *fileAccessStub) FileMetaForAdmin(context.Context, string, string) (domain.NodeMeta, error) {
	if s.denied {
		return domain.NodeMeta{}, usecase.ErrFileContentForbidden
	}
	return s.meta, nil
}

// nil storage deliberately makes these fail loudly if a denied request reaches OSS.
func TestFileContentDeniesBeforeStorage(t *testing.T) {
	for _, reader := range []bool{true, false} {
		for _, denied := range []bool{true, false} {
			e := echo.New()
			cfg := &config.Config{}
			stub := &fileAccessStub{denied: denied, meta: domain.NodeMeta{FileKey: "private/a.pdf"}}
			h := &ShareCommonHandler{BaseHandler: handler.NewBaseHandler(e, log.NewLogger(cfg), cfg, nil, nil, nil), nodeUsecase: stub}
			r := httptest.NewRequest("GET", "/file/content?"+url.Values{"key": {"private/unrelated.pdf"}, "node_id": {"doc"}, "kb_id": {"kb"}, "format": {"pdf"}}.Encode(), nil)
			r.Header.Set("X-KB-ID", "kb")
			w := httptest.NewRecorder()
			if err := h.fileContent(e.NewContext(r, w), reader); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusForbidden {
				t.Fatalf("reader=%v denied=%v: %d %s", reader, denied, w.Code, w.Body)
			}
			if denied && stub.detailCalls != 0 {
				t.Fatal("denied reader loaded document metadata")
			}
		}
	}
}

type previewReadCloser struct{ *bytes.Reader }

func (previewReadCloser) Close() error { return nil }

func TestFileContentRangesHeadAndSecurity(t *testing.T) {
	data := []byte("%PDF-1.7\nfixture bytes")
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/file/content", nil)
		r.Header.Set("Range", "bytes=0-7")
		w := httptest.NewRecorder()
		content := &usecase.FileContent{Reader: previewReadCloser{bytes.NewReader(data)}, ContentType: "application/pdf", Size: int64(len(data))}
		if err := serveFileContent(echo.New().NewContext(r, w), content, "../../测试\n.pdf"); err != nil {
			t.Fatal(err)
		}
		if w.Code != 206 || w.Header().Get("Content-Range") != "bytes 0-7/22" {
			t.Fatalf("range response: %d %v", w.Code, w.Header())
		}
		if method == "GET" && !bytes.Equal(w.Body.Bytes(), data[:8]) {
			t.Fatal("incorrect PDF range")
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD leaked content")
		}
		for key, want := range map[string]string{"Cache-Control": "private, no-store", "X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
			if w.Header().Get(key) != want {
				t.Fatalf("%s: %q", key, w.Header().Get(key))
			}
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatal("missing content CSP")
		}
		disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if err != nil || disposition != "attachment" || params["filename"] != "测试.pdf" {
			t.Fatalf("unsafe disposition: %s", w.Header().Get("Content-Disposition"))
		}
	}
}

func TestFileContentErrorStatuses(t *testing.T) {
	e := echo.New()
	cfg := &config.Config{}
	h := &ShareCommonHandler{BaseHandler: handler.NewBaseHandler(e, log.NewLogger(cfg), cfg, nil, nil, nil)}
	for _, tc := range []struct {
		err    error
		status int
	}{{usecase.ErrFileContentMissing, 404}, {usecase.ErrFileContentLarge, 413}, {usecase.ErrFileContentType, 415}, {context.DeadlineExceeded, 504}, {errors.New("upstream failed"), 502}} {
		w := httptest.NewRecorder()
		if err := h.fileContentError(e.NewContext(httptest.NewRequest("GET", "/file/content", nil), w), tc.err); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status {
			t.Fatalf("%v: %d", tc.err, w.Code)
		}
	}
}
