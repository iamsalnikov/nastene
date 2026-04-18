package newsfeed

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/events"
)

// HandleBanCreated полностью удаляет из ленты обоих пользователей все записи,
// где фигурирует противоположная сторона (как автор поста/коммента или хозяин стены).
func (s *Service) HandleBanCreated(ctx context.Context, ev events.BanCreated) error {
	if err := s.feed.DeleteForBan(ctx, ev.BannerID, ev.BannedID); err != nil {
		return fmt.Errorf("ban created %d<->%d: %w", ev.BannerID, ev.BannedID, err)
	}
	return nil
}
