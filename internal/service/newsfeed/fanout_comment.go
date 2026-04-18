package newsfeed

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
)

// HandleCommentCreated доставляет запись о комменте автору поста, владельцу стены
// и всем пользователям, которые раньше комментировали этот пост. Автора коммента
// не уведомляем; дубликаты получателей отсекают дедуп и UNIQUE в feed.
func (s *Service) HandleCommentCreated(ctx context.Context, ev events.CommentCreated) error {
	c, err := s.comments.ByID(ctx, ev.CommentID)
	if err != nil {
		return fmt.Errorf("handle comment %d: %w", ev.CommentID, err)
	}
	post, err := s.posts.ByID(ctx, c.PostID)
	if err != nil {
		return fmt.Errorf("handle comment %d: post: %w", ev.CommentID, err)
	}

	recipients := make([]int64, 0, 2)
	seen := make(map[int64]struct{}, 4)
	add := func(id int64) {
		if id == c.AuthorID {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		recipients = append(recipients, id)
	}

	add(post.AuthorID)
	add(post.WallOwnerID)

	priors, err := s.comments.DistinctAuthorsByPost(ctx, post.ID, c.AuthorID)
	if err != nil {
		return fmt.Errorf("handle comment %d: prior authors: %w", ev.CommentID, err)
	}
	for _, rid := range priors {
		add(rid)
	}

	for _, rid := range recipients {
		banned, err := s.mutuallyBanned(ctx, rid, c.AuthorID)
		if err != nil {
			return fmt.Errorf("handle comment %d: ban check %d: %w", ev.CommentID, rid, err)
		}
		if banned {
			continue
		}
		canView, err := s.authorizer.CanView(ctx, rid, post.WallOwnerID)
		if err != nil {
			return fmt.Errorf("handle comment %d: can view %d: %w", ev.CommentID, rid, err)
		}
		row := FeedRow{
			UserID:    rid,
			Kind:      string(domain.NewsEventComment),
			PostID:    post.ID,
			CommentID: c.ID,
			CreatedAt: c.CreatedAt,
			Hidden:    !canView,
		}
		if err := s.feed.Insert(ctx, row); err != nil {
			return fmt.Errorf("handle comment %d: insert for %d: %w", ev.CommentID, rid, err)
		}
	}
	return nil
}
