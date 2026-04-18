// Package newsfeed owns the per-user news feed (news_feed table).
// Handlers here are consumed by cmd/feed-builder.
package newsfeed

import (
	"context"
	"time"

	"github.com/iamsalnikov/nastene/internal/domain"
)

// FeedRow describes a single per-user feed entry to insert.
type FeedRow struct {
	UserID    int64
	Kind      string // 'post' | 'comment'
	PostID    int64
	CommentID int64 // zero for 'post' kind
	CreatedAt time.Time
	Hidden    bool
}

// FeedRepo persists news_feed rows.
type FeedRepo interface {
	// Insert adds a row. ON CONFLICT DO NOTHING — safe to retry.
	Insert(ctx context.Context, row FeedRow) error
	// DistinctViewersForOwner lists user_ids that currently have feed rows
	// referencing posts whose wall_owner_id = ownerID.
	DistinctViewersForOwner(ctx context.Context, ownerID int64) ([]int64, error)
	// SetHiddenForOwner marks rows (matching user_ids and wall_owner_id)
	// hidden_at = now (if not already hidden).
	SetHiddenForOwner(ctx context.Context, ownerID int64, userIDs []int64, now time.Time) error
	// SetVisibleForOwner clears hidden_at on rows matching user_ids and wall_owner_id.
	SetVisibleForOwner(ctx context.Context, ownerID int64, userIDs []int64) error
	// DeleteForBan removes every row where one side of the ban pair is user_id
	// and the other appears as post author, wall owner, or comment author.
	DeleteForBan(ctx context.Context, a, b int64) error
}

// PostRepo hydrates a post for the fan-out logic.
type PostRepo interface {
	ByID(ctx context.Context, id int64) (domain.WallPost, error)
}

// CommentRepo hydrates a comment and lists prior commenters on a post.
type CommentRepo interface {
	ByID(ctx context.Context, id int64) (domain.Comment, error)
	DistinctAuthorsByPost(ctx context.Context, postID, excludeAuthorID int64) ([]int64, error)
}

// FriendRepo lists friends of a user.
type FriendRepo interface {
	ListFriendIDs(ctx context.Context, userID int64) ([]int64, error)
}

// BanChecker reports whether ownerID has banned otherID (one-directional).
type BanChecker interface {
	IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error)
}

// Authorizer answers "can viewer see ownerID's wall right now?".
type Authorizer interface {
	CanView(ctx context.Context, viewerID, ownerID int64) (bool, error)
}

// Service bundles dependencies and exposes handlers for each event kind.
type Service struct {
	posts      PostRepo
	comments   CommentRepo
	friends    FriendRepo
	bans       BanChecker
	authorizer Authorizer
	feed       FeedRepo
	now        func() time.Time
}

func NewService(
	posts PostRepo,
	comments CommentRepo,
	friends FriendRepo,
	bans BanChecker,
	authorizer Authorizer,
	feed FeedRepo,
) *Service {
	return &Service{
		posts:      posts,
		comments:   comments,
		friends:    friends,
		bans:       bans,
		authorizer: authorizer,
		feed:       feed,
		now:        time.Now,
	}
}

// mutuallyBanned returns true if there's a ban in either direction between a and b.
func (s *Service) mutuallyBanned(ctx context.Context, a, b int64) (bool, error) {
	if a == b {
		return false, nil
	}
	banned, err := s.bans.IsBanned(ctx, a, b)
	if err != nil {
		return false, err
	}
	if banned {
		return true, nil
	}
	return s.bans.IsBanned(ctx, b, a)
}
