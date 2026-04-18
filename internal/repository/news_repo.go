package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type NewsRepo struct {
	pool *pgxpool.Pool
}

func NewNewsRepo(pool *pgxpool.Pool) *NewsRepo {
	return &NewsRepo{pool: pool}
}

const newsFeedQuery = `
SELECT f.kind,
       p.id                                AS post_id,
       COALESCE(f.comment_id, 0)           AS comment_id,
       CASE WHEN f.kind = 'comment' THEN c.created_at ELSE p.created_at END AS event_created_at,
       p.wall_owner_id,
       COALESCE(c.author_id, p.author_id)  AS author_id,
       p.kind::text                        AS post_kind,
       CASE WHEN f.kind = 'comment' THEN c.body ELSE p.body_text END AS body_text,
       COALESCE(p.graffiti_path, '')       AS graffiti_path
FROM news_feed f
JOIN wall_posts p ON p.id = f.post_id
LEFT JOIN comments c ON c.id = f.comment_id
WHERE f.user_id = $1
  AND f.hidden_at IS NULL
ORDER BY f.created_at DESC, f.id DESC
LIMIT $2 OFFSET $3
`

// ListFeed returns a page of visible news entries for a user, hydrated with
// post/comment bodies for rendering.
func (r *NewsRepo) ListFeed(ctx context.Context, userID int64, limit, offset int) ([]domain.NewsEvent, error) {
	rows, err := r.pool.Query(ctx, newsFeedQuery, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	defer rows.Close()

	var out []domain.NewsEvent
	for rows.Next() {
		var (
			ev       domain.NewsEvent
			kind     string
			postKind string
		)
		if err := rows.Scan(
			&kind,
			&ev.PostID,
			&ev.CommentID,
			&ev.CreatedAt,
			&ev.WallOwnerID,
			&ev.AuthorID,
			&postKind,
			&ev.BodyText,
			&ev.GraffitiPath,
		); err != nil {
			return nil, fmt.Errorf("scan feed row: %w", err)
		}
		ev.Kind = domain.NewsEventKind(kind)
		ev.PostKind = domain.PostKind(postKind)
		if ev.Kind == domain.NewsEventComment {
			ev.EventID = ev.CommentID
		} else {
			ev.EventID = ev.PostID
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed rows: %w", err)
	}
	return out, nil
}
