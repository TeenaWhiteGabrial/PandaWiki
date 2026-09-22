package s3

import (
	"context"
	"fmt"
	"io"
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

	return &MinioClient{
		client:       client,
		bucket:       bucket,
		objectPrefix: prefix,
		downloadURL:  strings.TrimRight(storage.DownloadURL, "/"),
	}, nil
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
	info, err := c.client.PutObject(ctx, c.bucket, c.physicalObject(object), reader, size, opts)
	if err == nil {
		info.Bucket = c.bucket
		info.Key = strings.TrimLeft(object, "/")
	}
	return info, err
}

// PresignedPutURL returns a short-lived URL that allows the browser to upload
// an object directly to the configured S3-compatible storage.
func (c *MinioClient) PresignedPutURL(ctx context.Context, object string, expires time.Duration) (string, error) {
	result, err := c.client.PresignedPutObject(ctx, c.bucket, c.physicalObject(object), expires)
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
	core := minio.Core{Client: c.client}
	return core.NewMultipartUpload(ctx, c.bucket, c.physicalObject(object), minio.PutObjectOptions{})
}

func (c *MinioClient) CompleteMultipartUpload(ctx context.Context, object, uploadID string, parts []minio.CompletePart) error {
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
	result, err := c.client.PresignedGetObject(ctx, c.bucket, c.physicalObject(object), expires, nil)
	if err != nil {
		return "", err
	}
	return result.String(), nil
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

func (c *MinioClient) IsPublicURL(rawURL string) bool {
	return c.downloadURL != "" && strings.HasPrefix(rawURL, c.downloadURL+"/")
}
