package wall

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/iamsalnikov/nastene/internal/domain"
)

const (
	maxCommentLen = 1000
	minCommentLen = 1
)

type CommentRepo interface {
	Create(ctx context.Context, postID, authorID int64, body string) (domain.Comment, error)
	ByID(ctx context.Context, id int64) (domain.Comment, error)
	ListByPosts(ctx context.Context, postIDs []int64) ([]domain.Comment, error)
	Delete(ctx context.Context, id int64) error
}

// CreateComment — посетитель оставляет комментарий на посту.
// Автор поста или хозяин стены и так могут комментировать, иначе проверяется CanComment (== CanView + не забанен).
func (s *Service) CreateComment(ctx context.Context, authorID, postID int64, body string) (domain.Comment, error) {
	body = strings.TrimSpace(body)
	l := utf8.RuneCountInString(body)
	if l < minCommentLen || l > maxCommentLen {
		return domain.Comment{}, fmt.Errorf("create comment: length %d out of range: %w", l, domain.ErrInvalidInput)
	}

	post, err := s.posts.ByID(ctx, postID)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("create comment: %w", err)
	}

	ok, err := s.authorizer.CanComment(ctx, authorID, post.WallOwnerID)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	if !ok {
		return domain.Comment{}, fmt.Errorf("create comment: %w", domain.ErrForbidden)
	}

	c, err := s.comments.Create(ctx, postID, authorID, body)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return c, nil
}

// DeleteComment — автор комментария или хозяин стены может удалить.
func (s *Service) DeleteComment(ctx context.Context, actorID, commentID int64) error {
	c, err := s.comments.ByID(ctx, commentID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	post, err := s.posts.ByID(ctx, c.PostID)
	if err != nil {
		return fmt.Errorf("delete comment: post: %w", err)
	}
	if actorID != c.AuthorID && actorID != post.WallOwnerID {
		return fmt.Errorf("delete comment: %w", domain.ErrForbidden)
	}
	if err := s.comments.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

// DeletePost — автор поста или хозяин стены может удалить.
func (s *Service) DeletePost(ctx context.Context, actorID, postID int64) error {
	p, err := s.posts.ByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if actorID != p.AuthorID && actorID != p.WallOwnerID {
		return fmt.Errorf("delete post: %w", domain.ErrForbidden)
	}
	if err := s.posts.Delete(ctx, postID); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}
