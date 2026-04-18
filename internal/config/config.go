package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	SessionCookieName   string
	SessionCookieSecure bool
	SessionTTL          time.Duration

	S3Endpoint         string
	S3Region           string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3UseSSL           bool
	S3PresignTTL       time.Duration
	S3AutoCreateBucket bool

	InvitesPerUser   int
	RegistrationMode string

	RateLimitPostsPerHour    int
	RateLimitCommentsPerHour int

	RabbitMQDSN   string
	ConsumerGroup string
}

const (
	RegistrationModeInvite = "invite"
	RegistrationModeOpen   = "open"
)

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

	secureRaw := envOr("SESSION_COOKIE_SECURE", "false")
	secure, err := strconv.ParseBool(secureRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse SESSION_COOKIE_SECURE %q: %w", secureRaw, err)
	}
	cfg.SessionCookieSecure = secure

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

	invitesRaw := envOr("INVITES_PER_USER", "2")
	invites, err := strconv.Atoi(invitesRaw)
	if err != nil || invites < 0 {
		return Config{}, fmt.Errorf("parse INVITES_PER_USER %q: must be a non-negative integer", invitesRaw)
	}
	cfg.InvitesPerUser = invites

	mode := envOr("REGISTRATION_MODE", RegistrationModeInvite)
	if mode != RegistrationModeInvite && mode != RegistrationModeOpen {
		return Config{}, fmt.Errorf("parse REGISTRATION_MODE %q: must be %q or %q", mode, RegistrationModeInvite, RegistrationModeOpen)
	}
	cfg.RegistrationMode = mode

	postsLimitRaw := envOr("RATE_LIMIT_POSTS_PER_HOUR", "60")
	postsLimit, err := strconv.Atoi(postsLimitRaw)
	if err != nil || postsLimit < 0 {
		return Config{}, fmt.Errorf("parse RATE_LIMIT_POSTS_PER_HOUR %q: must be a non-negative integer", postsLimitRaw)
	}
	cfg.RateLimitPostsPerHour = postsLimit

	commentsLimitRaw := envOr("RATE_LIMIT_COMMENTS_PER_HOUR", "120")
	commentsLimit, err := strconv.Atoi(commentsLimitRaw)
	if err != nil || commentsLimit < 0 {
		return Config{}, fmt.Errorf("parse RATE_LIMIT_COMMENTS_PER_HOUR %q: must be a non-negative integer", commentsLimitRaw)
	}
	cfg.RateLimitCommentsPerHour = commentsLimit

	cfg.RabbitMQDSN = os.Getenv("RABBITMQ_DSN")
	if cfg.RabbitMQDSN == "" {
		return Config{}, fmt.Errorf("load config: RABBITMQ_DSN is required")
	}
	cfg.ConsumerGroup = envOr("CONSUMER_GROUP", "nastene")

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
