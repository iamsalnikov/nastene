package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type PostOwnerResolver struct {
	posts *WallPostRepo
}

func NewPostOwnerResolver(posts *WallPostRepo) *PostOwnerResolver {
	return &PostOwnerResolver{posts: posts}
}

func (r *PostOwnerResolver) PostOwner(ctx context.Context, postID int64) (int64, error) {
	p, err := r.posts.ByID(ctx, postID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, fmt.Errorf("post owner: %w", domain.ErrNotFound)
		}
		return 0, fmt.Errorf("post owner: %w", err)
	}
	return p.WallOwnerID, nil
}
