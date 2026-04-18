package wall

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type PostDetail struct {
	Post       domain.WallPost
	Author     domain.User
	Owner      domain.User
	Comments   []CommentView
	CanComment bool
	CanDelete  bool
}

// LoadPost returns a single post with its comment thread and permission flags.
// Returns domain.ErrNotFound if the post does not exist and domain.ErrForbidden
// if the viewer cannot see the wall or has banned the post author.
func (s *Service) LoadPost(ctx context.Context, viewerID, postID int64) (*PostDetail, error) {
	post, err := s.posts.ByID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("load post: %w", err)
	}

	canView, err := s.authorizer.CanView(ctx, viewerID, post.WallOwnerID)
	if err != nil {
		return nil, fmt.Errorf("load post: can view: %w", err)
	}
	if !canView {
		return nil, fmt.Errorf("load post: %w", domain.ErrForbidden)
	}

	iBannedAuthor, err := s.authorizer.IsBanned(ctx, viewerID, post.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("load post: i banned author: %w", err)
	}
	if iBannedAuthor {
		return nil, fmt.Errorf("load post: %w", domain.ErrForbidden)
	}

	owner, err := s.users.ByID(ctx, post.WallOwnerID)
	if err != nil {
		return nil, fmt.Errorf("load post: owner: %w", err)
	}

	users := map[int64]domain.User{post.WallOwnerID: owner}
	resolve := func(id int64) (domain.User, error) {
		if u, ok := users[id]; ok {
			return u, nil
		}
		u, err := s.users.ByID(ctx, id)
		if err != nil {
			return domain.User{}, err
		}
		users[id] = u
		return u, nil
	}

	mask, err := s.avatarMasker(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("load post: avatar masker: %w", err)
	}

	author, err := resolve(post.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("load post: author: %w", err)
	}
	maskedAuthor, err := mask(author)
	if err != nil {
		return nil, fmt.Errorf("load post: mask author: %w", err)
	}

	rawComments, err := s.comments.ListByPosts(ctx, []int64{post.ID})
	if err != nil {
		return nil, fmt.Errorf("load post: comments: %w", err)
	}

	banCache := map[int64]bool{}
	commentViews := make([]CommentView, 0, len(rawComments))
	for _, c := range rawComments {
		banned, ok := banCache[c.AuthorID]
		if !ok {
			banned, err = s.authorizer.IsBanned(ctx, viewerID, c.AuthorID)
			if err != nil {
				return nil, fmt.Errorf("load post: ban check %d: %w", c.AuthorID, err)
			}
			banCache[c.AuthorID] = banned
		}
		if banned {
			continue
		}
		ca, err := resolve(c.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("load post: comment author %d: %w", c.AuthorID, err)
		}
		maskedCA, err := mask(ca)
		if err != nil {
			return nil, fmt.Errorf("load post: mask comment author %d: %w", c.AuthorID, err)
		}
		commentViews = append(commentViews, CommentView{Comment: c, Author: maskedCA})
	}

	canComment, err := s.authorizer.CanComment(ctx, viewerID, post.WallOwnerID)
	if err != nil {
		return nil, fmt.Errorf("load post: can comment: %w", err)
	}

	canDelete := viewerID == post.AuthorID || viewerID == post.WallOwnerID

	maskedOwner, err := mask(owner)
	if err != nil {
		return nil, fmt.Errorf("load post: mask owner: %w", err)
	}

	return &PostDetail{
		Post:       post,
		Author:     maskedAuthor,
		Owner:      maskedOwner,
		Comments:   commentViews,
		CanComment: canComment,
		CanDelete:  canDelete,
	}, nil
}
