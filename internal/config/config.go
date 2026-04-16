package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	SessionCookieName string
	SessionTTL        time.Duration
	UploadsDir        string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SessionCookieName: envOr("SESSION_COOKIE_NAME", "nastene_sid"),
		UploadsDir:        envOr("UPLOADS_DIR", "./data/uploads"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("load config: DATABASE_URL is required")
	}

	ttlRaw := envOr("SESSION_TTL", "720h")
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse SESSION_TTL %q: %w", ttlRaw, err)
	}
	cfg.SessionTTL = ttl

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
