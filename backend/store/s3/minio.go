package s3

import (
	"context"
	"fmt"
	"io"
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

func (c *MinioClient) PublicURL(object string) string {
	if c.downloadURL == "" {
		return ""
	}
	return c.downloadURL + "/" + c.physicalObject(object)
}

func (c *MinioClient) IsPublicURL(rawURL string) bool {
	return c.downloadURL != "" && strings.HasPrefix(rawURL, c.downloadURL+"/")
}
