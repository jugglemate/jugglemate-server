package oss

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// Client 阿里云 OSS 上传客户端
type Client struct {
	bucket *oss.Bucket
	config Config
}

// Config OSS 配置
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

// UploadResult 上传结果
type UploadResult struct {
	URL string // 公开访问 URL
	Key string // OSS 对象 key
}

// New 创建 OSS 客户端
func New(cfg Config) (*Client, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("oss.New: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("oss.Bucket(%s): %w", cfg.Bucket, err)
	}
	return &Client{bucket: bucket, config: cfg}, nil
}

// Upload 上传文件到 OSS，返回公开访问 URL
// prefix 为对象 key 前缀（如 "avatars/"、"materials/appkey/name/"）
// filename 为最终文件名
func (c *Client) Upload(ctx context.Context, reader io.Reader, prefix, filename string) (*UploadResult, error) {
	key := prefix + filename
	err := c.bucket.PutObject(key, reader)
	if err != nil {
		return nil, fmt.Errorf("PutObject(%s): %w", key, err)
	}
	// OSS 公开访问 URL: https://{bucket}.{endpoint_host}/{key}
	host := strings.TrimPrefix(c.config.Endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")
	publicURL := fmt.Sprintf("https://%s.%s/%s", c.config.Bucket, host, key)
	return &UploadResult{URL: publicURL, Key: key}, nil
}
