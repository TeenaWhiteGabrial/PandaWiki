package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/store/s3"
)

func TestPreviewContentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, key, format, head, mime string
		size                          int64
		want                          error
	}{
		{"pdf", "legacy/a.pdf", "pdf", "%PDF-1.7", "application/pdf", 100, nil},
		{"pdf renamed html", "a.pdf", "pdf", "<html><script>alert(1)</script>", "", 100, ErrFileContentType},
		{"pdf empty", "a.pdf", "pdf", "", "", 0, ErrFileContentType},
		{"pdf size", "a.pdf", "pdf", "%PDF-1.7", "", 100*1024*1024 + 1, ErrFileContentLarge},
		{"text size", "a.txt", "text", "text", "", 5*1024*1024 + 1, ErrFileContentLarge},
		{"binary as text", "a.txt", "text", "a\x00b", "", 3, ErrFileContentType},
		{"binary extension", "a.exe", "text", "text", "", 4, ErrFileContentType},
		{"legacy html inert", "a.html", "text", "<script>alert(1)</script>", "text/plain; charset=utf-8", 100, nil},
		{"download inert", "a.svg", "download", "<svg/>", "application/octet-stream", 100, nil},
		{"unsupported", "a.pdf", "inline", "%PDF-1.7", "", 100, ErrFileContentType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mime, err := previewContentType(tc.key, tc.format, tc.size, []byte(tc.head))
			if !errors.Is(err, tc.want) || mime != tc.mime {
				t.Fatalf("got %q, %v", mime, err)
			}
		})
	}
}

func TestOpenFileContentS3Stream(t *testing.T) {
	data := []byte("%PDF-1.7\nS3 preview fixture")
	var objectGets int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pldms/" || r.URL.Path == "/pldms" {
			w.WriteHeader(200)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/pldms/static-file/legacy/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		switch r.URL.Path {
		case "/pldms/static-file/legacy/missing.pdf":
			w.WriteHeader(404)
			return
		case "/pldms/static-file/legacy/large.pdf":
			if r.Method != "HEAD" {
				t.Error("oversized preview opened")
			}
			w.Header().Set("Content-Length", "104857601")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			return
		}
		if r.Method == "GET" {
			objectGets++
		}
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		http.ServeContent(w, r, "fixture.pdf", time.Unix(1, 0), bytes.NewReader(data))
	}))
	defer server.Close()
	client, err := s3.NewMinioClient(&config.Config{S3: config.S3Config{Endpoint: server.URL, AccessKey: "test-access", SecretKey: "test-secret", Bucket: "pldms", Region: "us-east-1"}})
	if err != nil {
		t.Fatal(err)
	}
	u := &FileUsecase{s3Client: client}
	content, err := u.OpenFileContent(context.Background(), "legacy/source.pdf", "pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer content.Reader.Close()
	actual, err := io.ReadAll(content.Reader)
	if err != nil || !bytes.Equal(actual, data) || content.ContentType != "application/pdf" || content.Size != int64(len(data)) {
		t.Fatalf("stream: %q %+v %v", actual, content, err)
	}
	before := objectGets
	if _, err := u.OpenFileContent(context.Background(), "legacy/large.pdf", "pdf"); !errors.Is(err, ErrFileContentLarge) {
		t.Fatalf("size check: %v", err)
	}
	if before != objectGets {
		t.Fatal("oversized file downloaded")
	}
	if _, err := u.OpenFileContent(context.Background(), "legacy/missing.pdf", "pdf"); !errors.Is(err, ErrFileContentMissing) {
		t.Fatalf("missing: %v", err)
	}
}
