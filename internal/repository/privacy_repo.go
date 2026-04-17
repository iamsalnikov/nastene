package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type PrivacyRepo struct {
	pool *pgxpool.Pool
}

func NewPrivacyRepo(pool *pgxpool.Pool) *PrivacyRepo {
	return &PrivacyRepo{pool: pool}
}

func (r *PrivacyRepo) EnsureDefaults(ctx context.Context, userID int64) error {
	const q = `
		INSERT INTO wall_privacy (user_id, view_scope, post_scope, comment_scope)
		VALUES ($1, 'friends', 'friends', 'friends')
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("ensure privacy defaults: %w", err)
	}
	return nil
}

func (r *PrivacyRepo) Get(ctx context.Context, userID int64) (domain.WallPrivacy, error) {
	const q = `SELECT user_id, view_scope, post_scope, comment_scope, updated_at FROM wall_privacy WHERE user_id = $1`
	var p domain.WallPrivacy
	err := r.pool.QueryRow(ctx, q, userID).
		Scan(&p.UserID, &p.ViewScope, &p.PostScope, &p.CommentScope, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WallPrivacy{
				UserID:       userID,
				ViewScope:    domain.ScopeFriends,
				PostScope:    domain.ScopeFriends,
				CommentScope: domain.ScopeFriends,
			}, nil
		}
		return domain.WallPrivacy{}, fmt.Errorf("get privacy: %w", err)
	}
	return p, nil
}

func (r *PrivacyRepo) Update(ctx context.Context, userID int64, view, post, comment domain.WallScope) error {
	const q = `
		INSERT INTO wall_privacy (user_id, view_scope, post_scope, comment_scope, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET view_scope    = EXCLUDED.view_scope,
		    post_scope    = EXCLUDED.post_scope,
		    comment_scope = EXCLUDED.comment_scope,
		    updated_at    = NOW()
	`
	if _, err := r.pool.Exec(ctx, q, userID, view, post, comment); err != nil {
		return fmt.Errorf("update privacy: %w", err)
	}
	return nil
}
