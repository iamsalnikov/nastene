// Package bans implements the "mutual-invisibility" ban semantics.
//
// Adding a ban tears down any friendship between the pair and clears pending
// friend requests in both directions. After a ban, neither party can send a
// friend request to the other (enforced by service/friends), and both sides see
// only the target's name (enforced by handler rendering). Removing a ban only
// the banner can do; it does NOT restore prior state (feed entries stay deleted,
// friendship stays broken).
package bans

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
	"github.com/iamsalnikov/nastene/pkg/q"
)

type Repo interface {
	Add(ctx context.Context, ownerID, bannedID int64) error
	Remove(ctx context.Context, ownerID, bannedID int64) error
	IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]domain.Ban, error)
}

// FriendRepo is the subset of the friendships repo we need to tear down the
// relationship when a ban is applied.
type FriendRepo interface {
	RemoveFriendship(ctx context.Context, a, b int64) error
	RemoveRequest(ctx context.Context, fromID, toID int64) error
}

type Service struct {
	repo    Repo
	friends FriendRepo
	pub     events.Publisher
	log     *slog.Logger
}

func NewService(repo Repo, friends FriendRepo, pub events.Publisher, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, friends: friends, pub: pub, log: log}
}

// Add bans bannedID from bannerID's side. Side-effects:
//   - friendship removed (if any)
//   - pending friend requests in both directions removed
//   - ban row inserted
//   - BanCreated event published (best-effort; only the insert failure aborts).
func (s *Service) Add(ctx context.Context, bannerID, bannedID int64) error {
	if bannerID == bannedID {
		return fmt.Errorf("add ban: %w", domain.ErrSelfAction)
	}

	if err := s.friends.RemoveFriendship(ctx, bannerID, bannedID); err != nil {
		return fmt.Errorf("add ban: remove friendship: %w", err)
	}
	if err := s.friends.RemoveRequest(ctx, bannerID, bannedID); err != nil {
		return fmt.Errorf("add ban: remove outgoing request: %w", err)
	}
	if err := s.friends.RemoveRequest(ctx, bannedID, bannerID); err != nil {
		return fmt.Errorf("add ban: remove incoming request: %w", err)
	}
	if err := s.repo.Add(ctx, bannerID, bannedID); err != nil {
		return fmt.Errorf("add ban: %w", err)
	}

	if s.pub != nil {
		if err := q.Publish(s.pub, events.TopicBanCreated, events.BanCreated{
			BannerID: bannerID,
			BannedID: bannedID,
		}); err != nil {
			s.log.Warn("publish ban.created failed", "err", err, "banner", bannerID, "banned", bannedID)
		}
	}
	return nil
}

// Remove lifts a ban. Only the banner may invoke this (enforced by handler).
func (s *Service) Remove(ctx context.Context, bannerID, bannedID int64) error {
	if err := s.repo.Remove(ctx, bannerID, bannedID); err != nil {
		return fmt.Errorf("remove ban: %w", err)
	}
	return nil
}

// ListByOwner returns bans issued by ownerID.
func (s *Service) ListByOwner(ctx context.Context, ownerID int64) ([]domain.Ban, error) {
	bans, err := s.repo.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}
	return bans, nil
}

// IsBanned reports whether ownerID has banned otherID (directional).
func (s *Service) IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error) {
	ok, err := s.repo.IsBanned(ctx, ownerID, otherID)
	if err != nil {
		return false, fmt.Errorf("is banned: %w", err)
	}
	return ok, nil
}

// IsMutuallyInvisible reports whether a ban exists in either direction between
// a and b. Both parties should be hidden from each other in this case.
func (s *Service) IsMutuallyInvisible(ctx context.Context, a, b int64) (bool, error) {
	if a == b || a == 0 || b == 0 {
		return false, nil
	}
	banned, err := s.repo.IsBanned(ctx, a, b)
	if err != nil {
		return false, fmt.Errorf("is mutually invisible: %w", err)
	}
	if banned {
		return true, nil
	}
	reverse, err := s.repo.IsBanned(ctx, b, a)
	if err != nil {
		return false, fmt.Errorf("is mutually invisible reverse: %w", err)
	}
	return reverse, nil
}
