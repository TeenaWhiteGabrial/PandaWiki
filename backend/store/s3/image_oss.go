package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/chaitin/panda-wiki/config"
)

type ImageOSSClient struct {
	client      *minio.Client
	bucket      string
	downloadURL string
}

type ImageUploadResult struct {
	URL          string
	Bucket       string
	Key          string
	ETag         string
	Size         int64
	VersionID    string
	LastModified string
}

func NewImageOSSClient(appConfig *config.Config) (*ImageOSSClient, error) {
	ossConfig := appConfig.ImageOSS
	if !ossConfig.IsConfigured() {
		return &ImageOSSClient{}, nil
	}

	missing := make([]string, 0, 6)
	for name, value := range map[string]string{
		"IMAGE_OSS_ENDPOINT":     ossConfig.Endpoint,
		"IMAGE_OSS_ACCESS_KEY":   ossConfig.AccessKey,
		"IMAGE_OSS_SECRET_KEY":   ossConfig.SecretKey,
		"IMAGE_OSS_BUCKET":       ossConfig.Bucket,
		"IMAGE_OSS_DOWNLOAD_URL": ossConfig.DownloadURL,
		"IMAGE_OSS_REGION":       ossConfig.Region,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("image OSS configuration is incomplete, missing: %s", strings.Join(missing, ", "))
	}

	endpointURL, err := url.Parse(ossConfig.Endpoint)
	if err != nil || endpointURL.Host == "" {
		return nil, fmt.Errorf("invalid IMAGE_OSS_ENDPOINT: %q", ossConfig.Endpoint)
	}
	if endpointURL.Scheme != "http" && endpointURL.Scheme != "https" {
		return nil, fmt.Errorf("IMAGE_OSS_ENDPOINT must use http or https: %q", ossConfig.Endpoint)
	}
	if endpointURL.Path != "" && endpointURL.Path != "/" {
		return nil, fmt.Errorf("IMAGE_OSS_ENDPOINT must not contain a path: %q", ossConfig.Endpoint)
	}

	downloadURL, err := url.Parse(ossConfig.DownloadURL)
	if err != nil || downloadURL.Host == "" {
		return nil, fmt.Errorf("invalid IMAGE_OSS_DOWNLOAD_URL: %q", ossConfig.DownloadURL)
	}
	if downloadURL.Scheme != "http" && downloadURL.Scheme != "https" {
		return nil, fmt.Errorf("IMAGE_OSS_DOWNLOAD_URL must use http or https: %q", ossConfig.DownloadURL)
	}
	downloadPath := strings.TrimRight(downloadURL.Path, "/")
	if path.Base(downloadPath) == ossConfig.Bucket {
		downloadPath = strings.TrimSuffix(downloadPath, "/"+ossConfig.Bucket)
	}
	downloadURL.Path = downloadPath
	downloadURL.RawPath = ""
	downloadURL.RawQuery = ""
	downloadURL.Fragment = ""

	client, err := minio.New(endpointURL.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(ossConfig.AccessKey, ossConfig.SecretKey, ""),
		Secure:       endpointURL.Scheme == "https",
		Region:       ossConfig.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("create image OSS client: %w", err)
	}

	return &ImageOSSClient{
		client:      client,
		bucket:      ossConfig.Bucket,
		downloadURL: strings.TrimRight(downloadURL.String(), "/"),
	}, nil
}

func (c *ImageOSSClient) IsManagedURL(rawURL string) bool {
	if !c.Enabled() {
		return false
	}
	imageURL, err := url.Parse(rawURL)
	if err != nil || imageURL.Scheme == "" || imageURL.Host == "" {
		return false
	}
	baseURL, err := url.Parse(c.downloadURL)
	if err != nil {
		return false
	}
	basePath := strings.TrimRight(baseURL.Path, "/") + "/" + c.bucket + "/"
	return imageURL.Scheme == baseURL.Scheme && imageURL.Host == baseURL.Host && strings.HasPrefix(imageURL.Path, basePath)
}

func (c *ImageOSSClient) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *ImageOSSClient) PutImage(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (*ImageUploadResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("image OSS is not configured")
	}

	objectName = strings.TrimLeft(objectName, "/")
	if objectName != c.bucket && !strings.HasPrefix(objectName, c.bucket+"/") {
		objectName = path.Join(c.bucket, objectName)
	}

	uploadInfo, err := c.client.PutObject(ctx, c.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"X-Amz-Acl": "public-read",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("upload image to OSS: %w", err)
	}

	objectInfo, err := c.client.StatObject(ctx, c.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("verify uploaded image in OSS: %w", err)
	}
	if objectInfo.Size != size {
		return nil, fmt.Errorf("verify uploaded image size in OSS: expected %d bytes, got %d bytes", size, objectInfo.Size)
	}

	return &ImageUploadResult{
		URL:          c.downloadURL + "/" + strings.TrimLeft(objectName, "/"),
		Bucket:       uploadInfo.Bucket,
		Key:          uploadInfo.Key,
		ETag:         uploadInfo.ETag,
		Size:         objectInfo.Size,
		VersionID:    uploadInfo.VersionID,
		LastModified: objectInfo.LastModified.Format(time.RFC3339),
	}, nil
}
