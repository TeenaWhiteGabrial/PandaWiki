package s3

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/chaitin/panda-wiki/config"
)

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		input  string
		host   string
		secure bool
	}{
		{"minio:9000", "minio:9000", false},
		{"https://oss.example.com", "oss.example.com", true},
		{"http://127.0.0.1:9000", "127.0.0.1:9000", false},
	}
	for _, tc := range tests {
		host, secure, err := normalizeEndpoint(tc.input)
		if err != nil || host != tc.host || secure != tc.secure {
			t.Fatalf("normalizeEndpoint(%q) = %q, %v, %v", tc.input, host, secure, err)
		}
	}
}

func TestOSSObjectNamespacePreservesLogicalKeys(t *testing.T) {
	client := &MinioClient{bucket: "pldms", objectPrefix: "static-file", downloadURL: "https://download.example.com"}
	logicalKey := "kb-a/file.pdf"
	if got := client.physicalObject(logicalKey); got != "static-file/kb-a/file.pdf" {
		t.Fatalf("unexpected physical key %q", got)
	}
	if got := client.PublicURL(logicalKey); got != "https://download.example.com/static-file/kb-a/file.pdf" {
		t.Fatalf("unexpected public url %q", got)
	}
}

func TestOSSIntegrationPutGetDelete(t *testing.T) {
	if os.Getenv("OSS_INTEGRATION_TEST") != "true" {
		t.Skip("set OSS_INTEGRATION_TEST=true to verify a configured OSS bucket")
	}
	client, err := NewMinioClient(&config.Config{S3: config.S3Config{
		Endpoint:    os.Getenv("OSS_ENDPOINT"),
		AccessKey:   os.Getenv("OSS_ACCESS_KEY"),
		SecretKey:   os.Getenv("OSS_SECRET_KEY"),
		Bucket:      os.Getenv("OSS_BUCKET_NAME"),
		DownloadURL: os.Getenv("OSS_DOWNLOAD_URL"),
		Region:      os.Getenv("OSS_REGION"),
	}})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := "_system/verification/" + uuid.NewString() + ".txt"
	payload := []byte("pandawiki-oss-verification")
	if _, err := client.PutObject(ctx, "static-file", key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.client.RemoveObject(context.Background(), client.bucket, client.physicalObject(key), minio.RemoveObjectOptions{}); err != nil {
			t.Errorf("remove verification object: %v", err)
		}
	}()

	object, err := client.GetObject(ctx, "static-file", key, minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	got, err := io.ReadAll(object)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("unexpected object payload %q", got)
	}
}
