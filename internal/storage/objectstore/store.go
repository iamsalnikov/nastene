// Package objectstore хранит и раздаёт пользовательские файлы (аватарки, граффити)
// в S3-совместимом хранилище. Один Client используется и для записи, и для генерации
// presigned-ссылок, которые рендерятся прямо в шаблоны.
package objectstore

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint         string // host:port или host без схемы; схема берётся из UseSSL
	Region           string
	AccessKey        string
	SecretKey        string
	Bucket           string
	UseSSL           bool
	PresignTTL       time.Duration // дефолт 1h, если <= 0
	AutoCreateBucket bool          // dev-удобство; в проде явно false
}

type Client struct {
	c          *minio.Client
	bucket     string
	presignTTL time.Duration
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("objectstore: endpoint, bucket, access key and secret key are required")
	}
	if cfg.PresignTTL <= 0 {
		cfg.PresignTTL = time.Hour
	}

	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("objectstore: minio client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("objectstore: bucket exists: %w", err)
	}
	if !exists {
		if !cfg.AutoCreateBucket {
			return nil, fmt.Errorf("objectstore: bucket %q does not exist", cfg.Bucket)
		}
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("objectstore: make bucket %q: %w", cfg.Bucket, err)
		}
	}

	return &Client{c: mc, bucket: cfg.Bucket, presignTTL: cfg.PresignTTL}, nil
}

// Save кладёт объект в bucket. size обязателен (S3 PutObject требует знать длину заранее).
func (s *Client) Save(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	_, err := s.c.PutObject(ctx, s.bucket, key, data, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("objectstore: put object %q: %w", key, err)
	}
	return nil
}

// PresignURL возвращает временную ссылку на GET с TTL из конфига.
// Подпись локальная (HMAC), сетевых вызовов не делает — безопасно вызывать на каждый рендер.
func (s *Client) PresignURL(ctx context.Context, key string) (string, error) {
	u, err := s.c.PresignedGetObject(ctx, s.bucket, key, s.presignTTL, nil)
	if err != nil {
		return "", fmt.Errorf("objectstore: presign %q: %w", key, err)
	}
	return u.String(), nil
}
