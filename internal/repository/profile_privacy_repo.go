package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type ProfilePrivacyRepo struct {
	pool *pgxpool.Pool
}

func NewProfilePrivacyRepo(pool *pgxpool.Pool) *ProfilePrivacyRepo {
	return &ProfilePrivacyRepo{pool: pool}
}

func (r *ProfilePrivacyRepo) EnsureDefaults(ctx context.Context, userID int64) error {
	const q = `
		INSERT INTO profile_privacy (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("ensure profile privacy defaults: %w", err)
	}
	return nil
}

func (r *ProfilePrivacyRepo) Get(ctx context.Context, userID int64) (domain.ProfilePrivacy, error) {
	const q = `
		SELECT user_id, online_scope, basic_scope, friends_scope, bio_scope, updated_at
		FROM profile_privacy
		WHERE user_id = $1
	`
	var p domain.ProfilePrivacy
	err := r.pool.QueryRow(ctx, q, userID).
		Scan(&p.UserID, &p.OnlineScope, &p.BasicScope, &p.FriendsScope, &p.BioScope, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DefaultProfilePrivacy(userID), nil
		}
		return domain.ProfilePrivacy{}, fmt.Errorf("get profile privacy: %w", err)
	}
	return p, nil
}

func (r *ProfilePrivacyRepo) Update(ctx context.Context, p domain.ProfilePrivacy) error {
	const q = `
		INSERT INTO profile_privacy (user_id, online_scope, basic_scope, friends_scope, bio_scope, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET online_scope  = EXCLUDED.online_scope,
		    basic_scope   = EXCLUDED.basic_scope,
		    friends_scope = EXCLUDED.friends_scope,
		    bio_scope     = EXCLUDED.bio_scope,
		    updated_at    = NOW()
	`
	if _, err := r.pool.Exec(ctx, q, p.UserID, p.OnlineScope, p.BasicScope, p.FriendsScope, p.BioScope); err != nil {
		return fmt.Errorf("update profile privacy: %w", err)
	}
	return nil
}
