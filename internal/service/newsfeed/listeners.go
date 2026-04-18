package newsfeed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"golang.org/x/sync/errgroup"

	"github.com/iamsalnikov/nastene/internal/events"
	"github.com/iamsalnikov/nastene/pkg/q"
)

// Run starts one goroutine per event topic and blocks until ctx is done or any
// subscribe call errors. Each handler is idempotent (ON CONFLICT DO NOTHING /
// ID-based DELETE), so message redelivery on restart is safe.
func (s *Service) Run(ctx context.Context, sub message.Subscriber, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return runSubscribe(gctx, sub, events.TopicWallPostCreated, log, s.HandleWallPostCreated)
	})
	g.Go(func() error {
		return runSubscribe(gctx, sub, events.TopicCommentCreated, log, s.HandleCommentCreated)
	})
	g.Go(func() error {
		return runSubscribe(gctx, sub, events.TopicWallPrivacyChanged, log, s.HandleWallPrivacyChanged)
	})
	g.Go(func() error {
		return runSubscribe(gctx, sub, events.TopicBanCreated, log, s.HandleBanCreated)
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("newsfeed run: %w", err)
	}
	return nil
}

func runSubscribe[T any](
	ctx context.Context,
	sub message.Subscriber,
	topic string,
	log *slog.Logger,
	handler func(context.Context, T) error,
) error {
	if err := q.Subscribe(ctx, sub, topic, log, handler); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}
