package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type FriendRepo struct {
	pool *pgxpool.Pool
}

func NewFriendRepo(pool *pgxpool.Pool) *FriendRepo {
	return &FriendRepo{pool: pool}
}

func (r *FriendRepo) AreFriends(ctx context.Context, a, b int64) (bool, error) {
	ua, ub := domain.OrderedPair(a, b)
	const q = `SELECT EXISTS(SELECT 1 FROM friendships WHERE user_a = $1 AND user_b = $2)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, ua, ub).Scan(&exists); err != nil {
		return false, fmt.Errorf("are friends: %w", err)
	}
	return exists, nil
}

func (r *FriendRepo) ListFriendIDs(ctx context.Context, userID int64) ([]int64, error) {
	const q = `
		SELECT CASE WHEN user_a = $1 THEN user_b ELSE user_a END
		FROM friendships
		WHERE user_a = $1 OR user_b = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list friend ids: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list friend ids scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list friend ids rows: %w", err)
	}
	return out, nil
}

func (r *FriendRepo) AddFriendship(ctx context.Context, a, b int64) error {
	ua, ub := domain.OrderedPair(a, b)
	const q = `INSERT INTO friendships (user_a, user_b) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := r.pool.Exec(ctx, q, ua, ub); err != nil {
		return fmt.Errorf("add friendship: %w", err)
	}
	return nil
}

func (r *FriendRepo) RemoveFriendship(ctx context.Context, a, b int64) error {
	ua, ub := domain.OrderedPair(a, b)
	const q = `DELETE FROM friendships WHERE user_a = $1 AND user_b = $2`
	if _, err := r.pool.Exec(ctx, q, ua, ub); err != nil {
		return fmt.Errorf("remove friendship: %w", err)
	}
	return nil
}

func (r *FriendRepo) AddRequest(ctx context.Context, fromID, toID int64) error {
	const q = `INSERT INTO friend_requests (from_id, to_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := r.pool.Exec(ctx, q, fromID, toID); err != nil {
		return fmt.Errorf("add friend request: %w", err)
	}
	return nil
}

func (r *FriendRepo) RemoveRequest(ctx context.Context, fromID, toID int64) error {
	const q = `DELETE FROM friend_requests WHERE from_id = $1 AND to_id = $2`
	if _, err := r.pool.Exec(ctx, q, fromID, toID); err != nil {
		return fmt.Errorf("remove friend request: %w", err)
	}
	return nil
}

func (r *FriendRepo) HasRequest(ctx context.Context, fromID, toID int64) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM friend_requests WHERE from_id = $1 AND to_id = $2)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, fromID, toID).Scan(&exists); err != nil {
		return false, fmt.Errorf("has friend request: %w", err)
	}
	return exists, nil
}

func (r *FriendRepo) CountIncomingRequests(ctx context.Context, toID int64) (int, error) {
	const q = `SELECT COUNT(*) FROM friend_requests WHERE to_id = $1`
	var n int
	if err := r.pool.QueryRow(ctx, q, toID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count incoming requests: %w", err)
	}
	return n, nil
}

func (r *FriendRepo) ListIncomingRequests(ctx context.Context, toID int64) ([]domain.FriendRequest, error) {
	const q = `SELECT from_id, to_id, created_at FROM friend_requests WHERE to_id = $1 ORDER BY created_at DESC`
	return scanRequests(ctx, r.pool, q, toID)
}

func (r *FriendRepo) ListOutgoingRequests(ctx context.Context, fromID int64) ([]domain.FriendRequest, error) {
	const q = `SELECT from_id, to_id, created_at FROM friend_requests WHERE from_id = $1 ORDER BY created_at DESC`
	return scanRequests(ctx, r.pool, q, fromID)
}

func scanRequests(ctx context.Context, pool *pgxpool.Pool, q string, arg int64) ([]domain.FriendRequest, error) {
	rows, err := pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()
	var out []domain.FriendRequest
	for rows.Next() {
		var fr domain.FriendRequest
		if err := rows.Scan(&fr.FromID, &fr.ToID, &fr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		out = append(out, fr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}
