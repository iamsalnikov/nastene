// Package privacy wraps the wall_privacy repository and emits
// WallPrivacyChanged events after every successful update so the news feed
// worker can reconcile hidden_at flags.
package privacy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
	"github.com/iamsalnikov/nastene/pkg/q"
)

type Repo interface {
	Get(ctx context.Context, userID int64) (domain.WallPrivacy, error)
	Update(ctx context.Context, userID int64, view, post, comment domain.WallScope) error
	EnsureDefaults(ctx context.Context, userID int64) error
}

type Service struct {
	repo Repo
	pub  events.Publisher
	log  *slog.Logger
}

func NewService(repo Repo, pub events.Publisher, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, pub: pub, log: log}
}

func (s *Service) Get(ctx context.Context, userID int64) (domain.WallPrivacy, error) {
	p, err := s.repo.Get(ctx, userID)
	if err != nil {
		return domain.WallPrivacy{}, fmt.Errorf("get wall privacy: %w", err)
	}
	return p, nil
}

func (s *Service) Update(ctx context.Context, userID int64, view, post, comment domain.WallScope) error {
	if err := s.repo.Update(ctx, userID, view, post, comment); err != nil {
		return fmt.Errorf("update wall privacy: %w", err)
	}
	if s.pub != nil {
		if err := q.Publish(s.pub, events.TopicWallPrivacyChanged, events.WallPrivacyChanged{OwnerID: userID}); err != nil {
			s.log.Warn("publish wall.privacy.changed failed", "err", err, "owner", userID)
		}
	}
	return nil
}

// EnsureDefaults delegates to the repo — used by auth flow on registration.
func (s *Service) EnsureDefaults(ctx context.Context, userID int64) error {
	if err := s.repo.EnsureDefaults(ctx, userID); err != nil {
		return fmt.Errorf("ensure privacy defaults: %w", err)
	}
	return nil
}
