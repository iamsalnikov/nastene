package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type BanRepo struct {
	pool *pgxpool.Pool
}

func NewBanRepo(pool *pgxpool.Pool) *BanRepo {
	return &BanRepo{pool: pool}
}

func (r *BanRepo) IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM bans WHERE user_id = $1 AND banned_id = $2)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, ownerID, otherID).Scan(&exists); err != nil {
		return false, fmt.Errorf("is banned: %w", err)
	}
	return exists, nil
}

func (r *BanRepo) Add(ctx context.Context, ownerID, bannedID int64) error {
	const q = `
		INSERT INTO bans (user_id, banned_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	if _, err := r.pool.Exec(ctx, q, ownerID, bannedID); err != nil {
		return fmt.Errorf("add ban: %w", err)
	}
	return nil
}

func (r *BanRepo) Remove(ctx context.Context, ownerID, bannedID int64) error {
	const q = `DELETE FROM bans WHERE user_id = $1 AND banned_id = $2`
	if _, err := r.pool.Exec(ctx, q, ownerID, bannedID); err != nil {
		return fmt.Errorf("remove ban: %w", err)
	}
	return nil
}

func (r *BanRepo) ListByOwner(ctx context.Context, ownerID int64) ([]domain.Ban, error) {
	const q = `SELECT user_id, banned_id, created_at FROM bans WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list bans: %w", err)
	}
	defer rows.Close()

	var out []domain.Ban
	for rows.Next() {
		var b domain.Ban
		if err := rows.Scan(&b.UserID, &b.BannedID, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("list bans scan: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list bans rows: %w", err)
	}
	return out, nil
}
