package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type WallPostRepo struct {
	pool *pgxpool.Pool
}

func NewWallPostRepo(pool *pgxpool.Pool) *WallPostRepo {
	return &WallPostRepo{pool: pool}
}

func (r *WallPostRepo) Create(ctx context.Context, p domain.WallPost) (domain.WallPost, error) {
	const q = `
		INSERT INTO wall_posts (wall_owner_id, author_id, kind, body_text, graffiti_path)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, wall_owner_id, author_id, kind, body_text, graffiti_path, created_at
	`
	var out domain.WallPost
	err := r.pool.QueryRow(ctx, q, p.WallOwnerID, p.AuthorID, p.Kind, p.BodyText, p.GraffitiPath).
		Scan(&out.ID, &out.WallOwnerID, &out.AuthorID, &out.Kind, &out.BodyText, &out.GraffitiPath, &out.CreatedAt)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create wall post: %w", err)
	}
	return out, nil
}

func (r *WallPostRepo) ByID(ctx context.Context, id int64) (domain.WallPost, error) {
	const q = `SELECT id, wall_owner_id, author_id, kind, body_text, graffiti_path, created_at FROM wall_posts WHERE id = $1`
	var p domain.WallPost
	err := r.pool.QueryRow(ctx, q, id).
		Scan(&p.ID, &p.WallOwnerID, &p.AuthorID, &p.Kind, &p.BodyText, &p.GraffitiPath, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WallPost{}, fmt.Errorf("post by id: %w", domain.ErrNotFound)
		}
		return domain.WallPost{}, fmt.Errorf("post by id: %w", err)
	}
	return p, nil
}

func (r *WallPostRepo) ListByWall(ctx context.Context, ownerID int64, limit, offset int) ([]domain.WallPost, error) {
	const q = `
		SELECT id, wall_owner_id, author_id, kind, body_text, graffiti_path, created_at
		FROM wall_posts
		WHERE wall_owner_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, ownerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list wall posts: %w", err)
	}
	defer rows.Close()

	var out []domain.WallPost
	for rows.Next() {
		var p domain.WallPost
		if err := rows.Scan(&p.ID, &p.WallOwnerID, &p.AuthorID, &p.Kind, &p.BodyText, &p.GraffitiPath, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan wall post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (r *WallPostRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM wall_posts WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("delete wall post: %w", err)
	}
	return nil
}
