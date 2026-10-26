package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/chaitin/panda-wiki/config"
)

// MinioClient is kept as the public type name for compatibility with existing
// usecases. It now represents any S3-compatible store, including Inspur OSS.
type MinioClient struct {
	client       *minio.Client
	bucket       string
	objectPrefix string
	downloadURL  string
	storageType  string
}

func NewMinioClient(appConfig *config.Config) (*MinioClient, error) {
	storage := appConfig.S3
	endpoint, secure, err := normalizeEndpoint(storage.Endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(storage.AccessKey) == "" || strings.TrimSpace(storage.SecretKey) == "" {
		return nil, fmt.Errorf("object storage credentials are incomplete")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(storage.AccessKey, storage.SecretKey, ""),
		Secure:       secure,
		Region:       storage.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}

	// Local legacy deployments use a physical static-file bucket. OSS uses its
	// configured bucket and keeps static-file as a prefix, preserving stored keys.
	bucket := strings.TrimSpace(storage.Bucket)
	prefix := ""
	if bucket == "" {
		bucket = "static-file"
	} else {
		prefix = "static-file"
	}

	exists, err := client.BucketExists(context.Background(), bucket)
	if err != nil {
		return nil, fmt.Errorf("check object storage bucket %q: %w", bucket, err)
	}
	if !exists {
		if storage.Bucket != "" {
			return nil, fmt.Errorf("configured object storage bucket %q does not exist", bucket)
		}
		if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: storage.Region}); err != nil {
			return nil, fmt.Errorf("make local object storage bucket: %w", err)
		}
	}

	storageType := strings.ToLower(strings.TrimSpace(storage.Type))
	if storageType != "oss" && storageType != "minio" {
		if strings.Contains(strings.ToLower(storage.Endpoint), "minio") {
			storageType = "minio"
		} else {
			storageType = "oss"
		}
	}
	result := &MinioClient{
		client:       client,
		bucket:       bucket,
		objectPrefix: prefix,
		downloadURL:  strings.TrimRight(storage.DownloadURL, "/"),
		storageType:  storageType,
	}
	return result, nil
}

func normalizeEndpoint(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("object storage endpoint is required")
	}
	if !strings.Contains(raw, "://") {
		return strings.TrimRight(raw, "/"), false, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", false, fmt.Errorf("invalid object storage endpoint %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false, fmt.Errorf("object storage endpoint must use http or https")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false, fmt.Errorf("object storage endpoint must not contain a path")
	}
	return parsed.Host, parsed.Scheme == "https", nil
}

func (c *MinioClient) physicalObject(object string) string {
	object = strings.TrimLeft(object, "/")
	if c.objectPrefix == "" {
		return object
	}
	return path.Join(c.objectPrefix, object)
}

func (c *MinioClient) PutObject(ctx context.Context, _ string, object string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	if err := c.ensurePrivateMinioBucket(ctx); err != nil {
		return minio.UploadInfo{}, err
	}
	info, err := c.client.PutObject(ctx, c.bucket, c.physicalObject(object), reader, size, privateObjectOptions(opts))
	if err == nil {
		info.Bucket = c.bucket
		info.Key = strings.TrimLeft(object, "/")
	}
	return info, err
}

// MinIO does not implement per-object ACLs. Refuse private-document writes to
// an anonymously readable bucket instead of silently relying on x-amz-acl.
// The application's image/display-asset path is intentionally separate.
func (c *MinioClient) ensurePrivateMinioBucket(ctx context.Context) error {
	if c.storageType != "minio" {
		return nil
	}
	policy, err := c.client.GetBucketPolicy(ctx, c.bucket)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchBucketPolicy" {
			return nil
		}
		return fmt.Errorf("cannot verify private MinIO bucket policy: %w", err)
	}
	if strings.TrimSpace(policy) == "" {
		return nil
	}
	var document struct {
		Statement []struct {
			Effect       string
			Principal    json.RawMessage
			NotPrincipal json.RawMessage
			Action       json.RawMessage
		}
	}
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		return fmt.Errorf("cannot verify MinIO bucket policy: %w", err)
	}
	for _, statement := range document.Statement {
		if !strings.EqualFold(statement.Effect, "Allow") {
			continue
		}
		// Reject any public-read statement conservatively, even if its resource
		// is narrower. It is unsafe to claim an object ACL makes such a bucket private.
		var principal any
		if err := json.Unmarshal(statement.Principal, &principal); err != nil && len(statement.NotPrincipal) == 0 {
			return fmt.Errorf("cannot verify MinIO principal")
		}
		public := len(statement.NotPrincipal) > 0
		switch value := principal.(type) {
		case string:
			public = public || value == "*"
		case map[string]any:
			for _, entry := range value {
				if entry == "*" {
					public = true
				}
				if list, ok := entry.([]any); ok {
					for _, member := range list {
						if member == "*" {
							public = true
						}
					}
				}
			}
		}
		if !public {
			continue
		}
		var actions any
		if err := json.Unmarshal(statement.Action, &actions); err != nil {
			return fmt.Errorf("cannot verify MinIO actions")
		}
		entries, ok := actions.([]any)
		if !ok {
			entries = []any{actions}
		}
		for _, entry := range entries {
			action, _ := entry.(string)
			matches, _ := path.Match(strings.ToLower(action), "s3:getobject")
			if action == "*" || matches {
				return fmt.Errorf("MinIO bucket permits anonymous reads; configure a private document bucket before uploading")
			}
		}
	}
	return nil
}

// privateObjectOptions applies to documents, imports, previews and generated ZIPs.
// S3 ACL/grant headers from a caller cannot broaden a managed document's ACL.
func privateObjectOptions(opts minio.PutObjectOptions) minio.PutObjectOptions {
	metadata := make(map[string]string, len(opts.UserMetadata)+1)
	for key, value := range opts.UserMetadata {
		lower := strings.ToLower(key)
		if lower == "x-amz-acl" || strings.HasPrefix(lower, "x-amz-grant-") {
			continue
		}
		metadata[key] = value
	}
	metadata["X-Amz-Acl"] = "private"
	opts.UserMetadata = metadata
	return opts
}

// Display assets stay private in storage. Admin previews use signed downloads;
// login-page branding has a separate endpoint limited to configured image fields.
func (c *MinioClient) PutDisplayImage(ctx context.Context, object string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return c.PutObject(ctx, c.bucket, object, reader, size, opts)
}

// PresignedPutURL returns a short-lived URL that allows the browser to upload
// an object directly to the configured S3-compatible storage.
func (c *MinioClient) PresignedPutURL(ctx context.Context, object string, expires time.Duration) (string, error) {
	if err := c.ensurePrivateMinioBucket(ctx); err != nil {
		return "", err
	}
	// Bind the download-only MIME type into the upload signature. A client
	// cannot turn a staged archive into an inline HTML object by spoofing MIME.
	headers := http.Header{"Content-Type": []string{"application/octet-stream"}, "X-Amz-Acl": []string{"private"}}
	result, err := c.client.PresignHeader(ctx, http.MethodPut, c.bucket, c.physicalObject(object), expires, nil, headers)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// PresignedUploadPartURL returns a URL for one S3 multipart part. The browser
// uploads bytes directly to object storage; the API only signs the request.
func (c *MinioClient) PresignedUploadPartURL(ctx context.Context, object, uploadID string, partNumber int, expires time.Duration) (string, error) {
	params := url.Values{}
	params.Set("uploadId", uploadID)
	params.Set("partNumber", fmt.Sprintf("%d", partNumber))
	result, err := c.client.PresignHeader(ctx, http.MethodPut, c.bucket, c.physicalObject(object), expires, params, nil)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// UploadMultipartPart streams one multipart part through the API to object
// storage. This is used as a same-origin fallback for browsers whose OSS CORS
// policy does not allow direct PUT requests.
func (c *MinioClient) UploadMultipartPart(ctx context.Context, object, uploadID string, partNumber int, reader io.Reader, size int64) (minio.ObjectPart, error) {
	core := minio.Core{Client: c.client}
	return core.PutObjectPart(ctx, c.bucket, c.physicalObject(object), uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
}

// MultipartUpload starts an S3 multipart upload and returns its opaque ID.
func (c *MinioClient) MultipartUpload(ctx context.Context, object string) (string, error) {
	if err := c.ensurePrivateMinioBucket(ctx); err != nil {
		return "", err
	}
	core := minio.Core{Client: c.client}
	return core.NewMultipartUpload(ctx, c.bucket, c.physicalObject(object), privateObjectOptions(minio.PutObjectOptions{ContentType: "application/octet-stream", ContentDisposition: "attachment"}))
}

func (c *MinioClient) CompleteMultipartUpload(ctx context.Context, object, uploadID string, parts []minio.CompletePart) error {
	if err := c.ensurePrivateMinioBucket(ctx); err != nil {
		return err
	}
	core := minio.Core{Client: c.client}
	_, err := core.CompleteMultipartUpload(ctx, c.bucket, c.physicalObject(object), uploadID, parts, minio.PutObjectOptions{})
	return err
}

func (c *MinioClient) AbortMultipartUpload(ctx context.Context, object, uploadID string) error {
	core := minio.Core{Client: c.client}
	return core.AbortMultipartUpload(ctx, c.bucket, c.physicalObject(object), uploadID)
}

func (c *MinioClient) ListMultipartParts(ctx context.Context, object, uploadID string) ([]minio.ObjectPart, error) {
	core := minio.Core{Client: c.client}
	result, err := core.ListObjectParts(ctx, c.bucket, c.physicalObject(object), uploadID, 0, 10000)
	if err != nil {
		return nil, err
	}
	return result.ObjectParts, nil
}

func (c *MinioClient) StatObject(ctx context.Context, object string) (minio.ObjectInfo, error) {
	return c.client.StatObject(ctx, c.bucket, c.physicalObject(object), minio.StatObjectOptions{})
}

func (c *MinioClient) GetObject(ctx context.Context, _ string, object string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return c.client.GetObject(ctx, c.bucket, c.physicalObject(object), opts)
}

func (c *MinioClient) SignURL(ctx context.Context, _ string, object string, expires time.Duration) (string, error) {
	url, _, err := c.SignURLWithStorageType(ctx, object, expires)
	return url, err
}

// SignDownloadURL returns a signed object URL that asks the browser to
// download the file instead of rendering it inline.
func (c *MinioClient) SignDownloadURL(ctx context.Context, object, filename string, expires time.Duration) (string, error) {
	filename = path.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "/" || filename == "" {
		filename = path.Base(object)
	}
	params := url.Values{}
	params.Set("response-content-disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	result, err := c.client.PresignedGetObject(ctx, c.bucket, c.physicalObject(object), expires, params)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (c *MinioClient) SignURLWithStorageType(ctx context.Context, object string, expires time.Duration) (string, string, error) {
	result, err := c.client.PresignedGetObject(ctx, c.bucket, c.physicalObject(object), expires, nil)
	if err != nil {
		return "", "", err
	}
	return result.String(), c.Type(), nil
}

func (c *MinioClient) Type() string {
	if c == nil || c.storageType == "" {
		return "minio"
	}
	return c.storageType
}

func (c *MinioClient) ObjectExists(ctx context.Context, object string) (bool, error) {
	_, err := c.client.StatObject(ctx, c.bucket, c.physicalObject(object), minio.StatObjectOptions{})
	if err != nil && isObjectNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func isObjectNotFound(err error) bool {
	response := minio.ToErrorResponse(err)
	return response.StatusCode == http.StatusNotFound || response.Code == "NoSuchKey" || response.Code == "NoSuchObject"
}

func (c *MinioClient) RemoveObject(ctx context.Context, object string) error {
	return c.client.RemoveObject(ctx, c.bucket, c.physicalObject(object), minio.RemoveObjectOptions{})
}

func (c *MinioClient) PublicURL(object string) string {
	if c.downloadURL == "" {
		return ""
	}
	return c.downloadURL + "/" + c.physicalObject(object)
}

// ObjectLocation returns the bucket and physical object key used by the
// configured storage backend. It does not include credentials or a signature.
func (c *MinioClient) ObjectLocation(object string) string {
	return c.bucket + "/" + c.physicalObject(object)
}

func (c *MinioClient) IsPublicURL(rawURL string) bool {
	return c.downloadURL != "" && strings.HasPrefix(rawURL, c.downloadURL+"/")
}
