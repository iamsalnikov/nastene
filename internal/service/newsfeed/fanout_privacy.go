package newsfeed

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/events"
)

// HandleWallPrivacyChanged пересобирает hidden_at для всех ленточных строк,
// ссылающихся на посты ownerID. Для каждого уникального viewer-а считаем CanView
// и либо прячем строки, либо возвращаем видимость.
func (s *Service) HandleWallPrivacyChanged(ctx context.Context, ev events.WallPrivacyChanged) error {
	viewers, err := s.feed.DistinctViewersForOwner(ctx, ev.OwnerID)
	if err != nil {
		return fmt.Errorf("privacy changed %d: viewers: %w", ev.OwnerID, err)
	}

	var hide, show []int64
	for _, v := range viewers {
		canView, err := s.authorizer.CanView(ctx, v, ev.OwnerID)
		if err != nil {
			return fmt.Errorf("privacy changed %d: can view %d: %w", ev.OwnerID, v, err)
		}
		if canView {
			show = append(show, v)
		} else {
			hide = append(hide, v)
		}
	}

	if len(hide) > 0 {
		if err := s.feed.SetHiddenForOwner(ctx, ev.OwnerID, hide, s.now()); err != nil {
			return fmt.Errorf("privacy changed %d: set hidden: %w", ev.OwnerID, err)
		}
	}
	if len(show) > 0 {
		if err := s.feed.SetVisibleForOwner(ctx, ev.OwnerID, show); err != nil {
			return fmt.Errorf("privacy changed %d: set visible: %w", ev.OwnerID, err)
		}
	}
	return nil
}
