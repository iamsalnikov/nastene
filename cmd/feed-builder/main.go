// Package main is the news feed builder worker. It subscribes to the
// application's event topics (wall posts, comments, privacy changes, bans) and
// materializes per-user news_feed rows.
//
// Runs as a separate process alongside cmd/nastene in prod/docker-compose.yml.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/config"
	"github.com/iamsalnikov/nastene/internal/repository"
	"github.com/iamsalnikov/nastene/internal/service/newsfeed"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	"github.com/iamsalnikov/nastene/pkg/rabbit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(log)

	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	log.Info("postgres connected")

	subscriber, err := rabbit.NewSubscriber(cfg.RabbitMQDSN, cfg.ConsumerGroup)
	if err != nil {
		return fmt.Errorf("init rabbit subscriber: %w", err)
	}
	defer func() {
		if err := subscriber.Close(); err != nil {
			log.Warn("close rabbit subscriber", "err", err)
		}
	}()
	log.Info("rabbit subscriber ready", "consumer_group", cfg.ConsumerGroup)

	privacyRepo := repository.NewPrivacyRepo(pool)
	banRepo := repository.NewBanRepo(pool)
	friendRepo := repository.NewFriendRepo(pool)
	postRepo := repository.NewWallPostRepo(pool)
	commentRepo := repository.NewCommentRepo(pool)
	feedRepo := repository.NewFeedRepo(pool)

	authorizer := wall.NewAuthorizer(privacyRepo, friendRepo, banRepo)

	svc := newsfeed.NewService(postRepo, commentRepo, friendRepo, banRepo, authorizer, feedRepo)

	log.Info("feed-builder running")
	if err := svc.Run(ctx, subscriber, log); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run newsfeed: %w", err)
	}
	log.Info("feed-builder stopped cleanly")
	return nil
}
