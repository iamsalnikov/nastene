package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	SessionCookieName string
	SessionTTL        time.Duration

	S3Endpoint         string
	S3Region           string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3UseSSL           bool
	S3PresignTTL       time.Duration
	S3AutoCreateBucket bool
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SessionCookieName: envOr("SESSION_COOKIE_NAME", "nastene_sid"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3Region:    envOr("S3_REGION", "us-east-1"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("load config: DATABASE_URL is required")
	}
	for k, v := range map[string]string{
		"S3_ENDPOINT":          cfg.S3Endpoint,
		"S3_BUCKET":            cfg.S3Bucket,
		"S3_ACCESS_KEY_ID":     cfg.S3AccessKey,
		"S3_SECRET_ACCESS_KEY": cfg.S3SecretKey,
	} {
		if v == "" {
			return Config{}, fmt.Errorf("load config: %s is required", k)
		}
	}

	ttlRaw := envOr("SESSION_TTL", "720h")
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse SESSION_TTL %q: %w", ttlRaw, err)
	}
	cfg.SessionTTL = ttl

	useSSLRaw := envOr("S3_USE_SSL", "true")
	useSSL, err := strconv.ParseBool(useSSLRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse S3_USE_SSL %q: %w", useSSLRaw, err)
	}
	cfg.S3UseSSL = useSSL

	presignRaw := envOr("S3_PRESIGN_TTL", "1h")
	presign, err := time.ParseDuration(presignRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse S3_PRESIGN_TTL %q: %w", presignRaw, err)
	}
	cfg.S3PresignTTL = presign

	autoCreateRaw := envOr("S3_AUTO_CREATE_BUCKET", "false")
	autoCreate, err := strconv.ParseBool(autoCreateRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse S3_AUTO_CREATE_BUCKET %q: %w", autoCreateRaw, err)
	}
	cfg.S3AutoCreateBucket = autoCreate

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
