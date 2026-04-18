package newsfeed

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
)

// HandleWallPostCreated fans out a post event to interested users.
//
//	author == wall_owner  → все друзья автора (без взаимного бана).
//	author != wall_owner  → владелец стены (без взаимного бана).
//
// Для каждого получателя hidden=true, если CanView сейчас false.
func (s *Service) HandleWallPostCreated(ctx context.Context, ev events.WallPostCreated) error {
	post, err := s.posts.ByID(ctx, ev.PostID)
	if err != nil {
		return fmt.Errorf("handle wall post %d: %w", ev.PostID, err)
	}

	recipients, err := s.recipientsForPost(ctx, post)
	if err != nil {
		return fmt.Errorf("handle wall post %d: recipients: %w", ev.PostID, err)
	}

	for _, rid := range recipients {
		hidden := false
		canView, err := s.authorizer.CanView(ctx, rid, post.WallOwnerID)
		if err != nil {
			return fmt.Errorf("handle wall post %d: can view %d: %w", ev.PostID, rid, err)
		}
		if !canView {
			hidden = true
		}
		row := FeedRow{
			UserID:    rid,
			Kind:      string(domain.NewsEventPost),
			PostID:    post.ID,
			CreatedAt: post.CreatedAt,
			Hidden:    hidden,
		}
		if err := s.feed.Insert(ctx, row); err != nil {
			return fmt.Errorf("handle wall post %d: insert for %d: %w", ev.PostID, rid, err)
		}
	}
	return nil
}

func (s *Service) recipientsForPost(ctx context.Context, post domain.WallPost) ([]int64, error) {
	if post.AuthorID == post.WallOwnerID {
		friends, err := s.friends.ListFriendIDs(ctx, post.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("list friends: %w", err)
		}
		out := make([]int64, 0, len(friends))
		for _, fid := range friends {
			if fid == post.AuthorID {
				continue
			}
			banned, err := s.mutuallyBanned(ctx, fid, post.AuthorID)
			if err != nil {
				return nil, fmt.Errorf("ban check %d: %w", fid, err)
			}
			if banned {
				continue
			}
			out = append(out, fid)
		}
		return out, nil
	}

	banned, err := s.mutuallyBanned(ctx, post.WallOwnerID, post.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("ban check owner-author: %w", err)
	}
	if banned {
		return nil, nil
	}
	return []int64{post.WallOwnerID}, nil
}
