package s3

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaitin/panda-wiki/config"
)

func TestNewImageOSSClientNormalizesDownloadURL(t *testing.T) {
	tests := []struct {
		name        string
		downloadURL string
	}{
		{name: "base URL", downloadURL: "https://uoc.oss.example.com"},
		{name: "remove duplicated bucket path", downloadURL: "https://uoc.oss.example.com/uoc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewImageOSSClient(&config.Config{
				ImageOSS: config.ImageOSSConfig{
					Endpoint:    "https://oss.example.com",
					AccessKey:   "access-key",
					SecretKey:   "secret-key",
					Bucket:      "uoc",
					DownloadURL: tt.downloadURL,
					Region:      "test-region",
				},
			})
			if err != nil {
				t.Fatalf("NewImageOSSClient() error = %v", err)
			}
			if client.downloadURL != "https://uoc.oss.example.com" {
				t.Fatalf("downloadURL = %q", client.downloadURL)
			}
			if !client.IsManagedURL("https://uoc.oss.example.com/uoc/image.png") {
				t.Fatal("expected image URL to be managed")
			}
			if client.IsManagedURL("https://uoc.oss.example.com/images/image.png") {
				t.Fatal("unexpected URL outside bucket path to be managed")
			}
		})
	}
}

func TestImageOSSClientPutImageMatchesJavaS3Request(t *testing.T) {
	const content = "image-content"
	var putRequestSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uoc/uoc/image.png" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPut:
			putRequestSeen = true
			if acl := r.Header.Get("X-Amz-Acl"); acl != "public-read" {
				t.Errorf("X-Amz-Acl = %q", acl)
			}
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))
			w.Header().Set("ETag", `"test-etag"`)
			w.Header().Set("Last-Modified", "Thu, 16 Jul 2026 01:00:00 GMT")
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewImageOSSClient(&config.Config{
		ImageOSS: config.ImageOSSConfig{
			Endpoint:    server.URL,
			AccessKey:   "access-key",
			SecretKey:   "secret-key",
			Bucket:      "uoc",
			DownloadURL: "https://uoc.oss.example.com",
			Region:      "test-region",
		},
	})
	if err != nil {
		t.Fatalf("NewImageOSSClient() error = %v", err)
	}

	result, err := client.PutImage(context.Background(), "image.png", bytes.NewBufferString(content), int64(len(content)), "image/png")
	if err != nil {
		t.Fatalf("PutImage() error = %v", err)
	}
	if !putRequestSeen {
		t.Fatal("expected PUT request")
	}
	if result.Key != "uoc/image.png" {
		t.Fatalf("key = %q", result.Key)
	}
	if result.URL != "https://uoc.oss.example.com/uoc/image.png" {
		t.Fatalf("url = %q", result.URL)
	}
}
