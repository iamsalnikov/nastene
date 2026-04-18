package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/service/newsfeed"
)

type FeedRepo struct {
	pool *pgxpool.Pool
}

func NewFeedRepo(pool *pgxpool.Pool) *FeedRepo {
	return &FeedRepo{pool: pool}
}

// Insert appends one feed row. Uniqueness on (user_id, post_id) for 'post'
// rows and (user_id, comment_id) for 'comment' rows makes this idempotent.
func (r *FeedRepo) Insert(ctx context.Context, row newsfeed.FeedRow) error {
	var (
		hidden     any
		commentPtr any
	)
	if row.Hidden {
		hidden = time.Now()
	}
	if row.CommentID != 0 {
		commentPtr = row.CommentID
	}

	const q = `
		INSERT INTO news_feed (user_id, kind, post_id, comment_id, created_at, hidden_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
	`
	if _, err := r.pool.Exec(ctx, q, row.UserID, row.Kind, row.PostID, commentPtr, row.CreatedAt, hidden); err != nil {
		return fmt.Errorf("insert news feed row: %w", err)
	}
	return nil
}

// DistinctViewersForOwner lists user_ids that have feed rows tied to posts on
// ownerID's wall (including via comments on those posts).
func (r *FeedRepo) DistinctViewersForOwner(ctx context.Context, ownerID int64) ([]int64, error) {
	const q = `
		SELECT DISTINCT f.user_id
		FROM news_feed f
		JOIN wall_posts p ON p.id = f.post_id
		WHERE p.wall_owner_id = $1
	`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("distinct viewers: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan viewer: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("viewers rows: %w", err)
	}
	return out, nil
}

// SetHiddenForOwner sets hidden_at=now on still-visible rows where
// (user_id ∈ userIDs) ∧ wall_owner_id = ownerID.
func (r *FeedRepo) SetHiddenForOwner(ctx context.Context, ownerID int64, userIDs []int64, now time.Time) error {
	if len(userIDs) == 0 {
		return nil
	}
	const q = `
		UPDATE news_feed f
		SET hidden_at = $3
		FROM wall_posts p
		WHERE p.id = f.post_id
		  AND p.wall_owner_id = $1
		  AND f.user_id = ANY($2)
		  AND f.hidden_at IS NULL
	`
	if _, err := r.pool.Exec(ctx, q, ownerID, userIDs, now); err != nil {
		return fmt.Errorf("set hidden for owner: %w", err)
	}
	return nil
}

// SetVisibleForOwner clears hidden_at on rows matching (user_id ∈ userIDs, wall_owner=ownerID).
func (r *FeedRepo) SetVisibleForOwner(ctx context.Context, ownerID int64, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	const q = `
		UPDATE news_feed f
		SET hidden_at = NULL
		FROM wall_posts p
		WHERE p.id = f.post_id
		  AND p.wall_owner_id = $1
		  AND f.user_id = ANY($2)
		  AND f.hidden_at IS NOT NULL
	`
	if _, err := r.pool.Exec(ctx, q, ownerID, userIDs); err != nil {
		return fmt.Errorf("set visible for owner: %w", err)
	}
	return nil
}

// DeleteForBan removes rows involving the opposite party from both users' feeds.
func (r *FeedRepo) DeleteForBan(ctx context.Context, a, b int64) error {
	const q = `
		DELETE FROM news_feed f
		USING wall_posts p
		LEFT JOIN comments c ON c.id IS NOT NULL
		WHERE p.id = f.post_id
		  AND (f.comment_id IS NULL OR c.id = f.comment_id)
		  AND (
		    (f.user_id = $1 AND ($2 IN (p.author_id, p.wall_owner_id) OR c.author_id = $2))
		    OR
		    (f.user_id = $2 AND ($1 IN (p.author_id, p.wall_owner_id) OR c.author_id = $1))
		  )
	`
	if _, err := r.pool.Exec(ctx, q, a, b); err != nil {
		return fmt.Errorf("delete for ban: %w", err)
	}
	return nil
}

