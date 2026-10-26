package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

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

func TestSignDownloadURLRequestsAttachment(t *testing.T) {
	client, err := minio.New("oss.example.com", &minio.Options{
		Creds:        credentials.NewStaticV4("access", "secret", ""),
		Secure:       true,
		Region:       "cn-hangzhou",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	storage := &MinioClient{client: client, bucket: "wiki", objectPrefix: "static-file"}
	signedURL, err := storage.SignDownloadURL(context.Background(), "kb-a/manual.pdf", "安全手册.pdf", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatal(err)
	}
	contentDisposition, params, err := mime.ParseMediaType(parsed.Query().Get("response-content-disposition"))
	if err != nil {
		t.Fatal(err)
	}
	if contentDisposition != "attachment" || params["filename"] != "安全手册.pdf" {
		t.Fatalf("unexpected content disposition %q with params %#v", contentDisposition, params)
	}
	if !strings.Contains(parsed.Path, "/wiki/static-file/kb-a/manual.pdf") {
		t.Fatalf("signed URL has unexpected object path %q", parsed.Path)
	}
}

func TestObjectMissingFromActiveOSSCanBeMarkedNonOSS(t *testing.T) {
	key := "kb-a/file.pdf"
	active := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Trim(r.URL.Path, "/") == "oss-bucket" && r.URL.Query().Has("location") {
			_, _ = io.WriteString(w, `<LocationConstraint>us-east-1</LocationConstraint>`)
			return
		}
		if r.Method == http.MethodHead && strings.Trim(r.URL.Path, "/") == "oss-bucket" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`)
	}))
	defer active.Close()

	client, err := NewMinioClient(&config.Config{
		S3: config.S3Config{
			Type:      "oss",
			Endpoint:  active.URL,
			AccessKey: "oss-access",
			SecretKey: "oss-secret",
			Bucket:    "oss-bucket",
			Region:    "us-east-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	exists, err := client.ObjectExists(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("missing OSS object reported as present")
	}
	if client.Type() != "oss" {
		t.Fatalf("unexpected active storage type %q", client.Type())
	}
	signedURL, storageType, err := client.SignURLWithStorageType(context.Background(), key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if storageType != "oss" || signedURL == "" {
		t.Fatalf("expected a signed OSS URL, got type=%q URL=%q", storageType, signedURL)
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

func TestArchiveUploadSignatureBindsContentType(t *testing.T) {
	client, err := minio.New("oss.example.test", &minio.Options{Creds: credentials.NewStaticV4("test-access", "test-secret", ""), Secure: true, Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	storage := &MinioClient{client: client, bucket: "wiki", objectPrefix: "static-file"}
	signed, err := storage.PresignedPutURL(context.Background(), "kb/imports/test.zip", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatal("upload MIME can be changed without invalidating signature")
	}
}

func TestPrivateUploadsCannotInheritPublicACL(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			t.Fatalf("unexpected storage request %s", r.Method)
		}
		if r.URL.Query().Has("uploadId") && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, `<CompleteMultipartUploadResult><Location>http://storage/wiki/file</Location><Bucket>wiki</Bucket><Key>file</Key><ETag>"done"</ETag></CompleteMultipartUploadResult>`)
			calls = append(calls, "complete")
			return
		}
		if r.Header.Get("X-Amz-Acl") != "private" || r.Header.Get("X-Amz-Grant-Read") != "" {
			t.Errorf("document ACL not enforced: acl=%q grant=%q", r.Header.Get("X-Amz-Acl"), r.Header.Get("X-Amz-Grant-Read"))
		}
		if r.URL.Query().Has("uploads") {
			calls = append(calls, "multipart")
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, `<InitiateMultipartUploadResult><Bucket>wiki</Bucket><Key>file</Key><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`)
			return
		}
		calls = append(calls, "put")
		io.Copy(io.Discard, r.Body)
		w.Header().Set("ETag", `"test"`)
	}))
	defer server.Close()
	raw, _ := url.Parse(server.URL)
	sdk, err := minio.New(raw.Host, &minio.Options{Creds: credentials.NewStaticV4("test", "test", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	storage := &MinioClient{client: sdk, bucket: "wiki"}
	metadata := map[string]string{"x-amz-acl": "public-read", "X-Amz-Grant-Read": "uri=public", "originalname": "test.txt"}
	if _, err := storage.PutObject(context.Background(), "ignored", "file", strings.NewReader("hello"), 5, minio.PutObjectOptions{UserMetadata: metadata, DisableMultipart: true}); err != nil {
		t.Fatal(err)
	}
	if metadata["x-amz-acl"] != "public-read" {
		t.Fatal("caller metadata mutated")
	}
	id, err := storage.MultipartUpload(context.Background(), "file")
	if err != nil || id != "upload-1" {
		t.Fatalf("multipart init: %q %v", id, err)
	}
	if err := storage.CompleteMultipartUpload(context.Background(), "file", id, []minio.CompletePart{{PartNumber: 1, ETag: "test"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "put,multipart,complete" {
		t.Fatalf("upload paths not covered: %v", calls)
	}
	signed, err := storage.PresignedPutURL(context.Background(), "file", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(signed)
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "x-amz-acl") {
		t.Fatal("direct PUT can change ACL without breaking signature")
	}
}

func TestLocalMinioPrivateWorkflow(t *testing.T) {
	endpoint := os.Getenv("PW_LOCAL_MINIO_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("isolated local MinIO required")
	}
	if !strings.HasPrefix(endpoint, "127.0.0.1:") {
		t.Fatal("this test only permits localhost")
	}
	sdk, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4("local-security-test", os.Getenv("PW_LOCAL_MINIO_TEST_PASSWORD"), ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bucket := "pw-security-" + uuid.NewString()
	if err = sdk.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	defer sdk.RemoveBucket(ctx, bucket)
	defer sdk.SetBucketPolicy(ctx, bucket, "")
	storage := &MinioClient{client: sdk, bucket: bucket, storageType: "minio"}
	payload := "private knowledge-base document"
	if _, err = storage.PutObject(ctx, "ignored", "test.txt", strings.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}
	defer sdk.RemoveObject(ctx, bucket, "test.txt", minio.RemoveObjectOptions{})
	anonymous, err := http.Get("http://" + endpoint + "/" + bucket + "/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	anonymous.Body.Close()
	if anonymous.StatusCode != 403 {
		t.Fatalf("anonymous GET = %d", anonymous.StatusCode)
	}
	signed, err := storage.SignURL(ctx, "ignored", "test.txt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(signed)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(data) != payload {
		t.Fatal("authorized signed download failed")
	}
	signedPut, err := storage.PresignedPutURL(ctx, "direct.txt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("PUT", signedPut, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Amz-Acl", "private")
	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("signed private PUT = %d", response.StatusCode)
	}
	defer sdk.RemoveObject(ctx, bucket, "direct.txt", minio.RemoveObjectOptions{})
	id, err := storage.MultipartUpload(ctx, "parts.zip")
	if err != nil {
		t.Fatal(err)
	}
	part, err := storage.UploadMultipartPart(ctx, "parts.zip", id, 1, strings.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.CompleteMultipartUpload(ctx, "parts.zip", id, []minio.CompletePart{{PartNumber: 1, ETag: part.ETag}}); err != nil {
		t.Fatal(err)
	}
	defer sdk.RemoveObject(ctx, bucket, "parts.zip", minio.RemoveObjectOptions{})
	anonymous, err = http.Get("http://" + endpoint + "/" + bucket + "/parts.zip")
	if err != nil {
		t.Fatal(err)
	}
	anonymous.Body.Close()
	if anonymous.StatusCode != 403 {
		t.Fatal("multipart archive publicly readable")
	}
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
	if err = sdk.SetBucketPolicy(ctx, bucket, policy); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.PutObject(ctx, "ignored", "must-not-write.txt", strings.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{}); err == nil {
		t.Fatal("private document written to public MinIO bucket")
	}
	if _, err = storage.MultipartUpload(ctx, "must-not-stage.zip"); err == nil {
		t.Fatal("multipart stage permitted in public bucket")
	}
	if _, err = storage.PresignedPutURL(ctx, "must-not-sign.zip", time.Minute); err == nil {
		t.Fatal("direct upload signed for public bucket")
	}
}
