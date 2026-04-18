package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type CommentRepo struct {
	pool *pgxpool.Pool
}

func NewCommentRepo(pool *pgxpool.Pool) *CommentRepo {
	return &CommentRepo{pool: pool}
}

func (r *CommentRepo) Create(ctx context.Context, postID, authorID int64, body string) (domain.Comment, error) {
	const q = `
		INSERT INTO comments (post_id, author_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, post_id, author_id, body, created_at
	`
	var c domain.Comment
	err := r.pool.QueryRow(ctx, q, postID, authorID, body).
		Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return c, nil
}

func (r *CommentRepo) ByID(ctx context.Context, id int64) (domain.Comment, error) {
	const q = `SELECT id, post_id, author_id, body, created_at FROM comments WHERE id = $1`
	var c domain.Comment
	err := r.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Comment{}, fmt.Errorf("comment by id: %w", domain.ErrNotFound)
		}
		return domain.Comment{}, fmt.Errorf("comment by id: %w", err)
	}
	return c, nil
}

func (r *CommentRepo) ListByPosts(ctx context.Context, postIDs []int64) ([]domain.Comment, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	const q = `
		SELECT id, post_id, author_id, body, created_at
		FROM comments
		WHERE post_id = ANY($1)
		ORDER BY post_id ASC, created_at ASC, id ASC
	`
	rows, err := r.pool.Query(ctx, q, postIDs)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var out []domain.Comment
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (r *CommentRepo) DistinctAuthorsByPost(ctx context.Context, postID, excludeAuthorID int64) ([]int64, error) {
	const q = `
		SELECT DISTINCT author_id
		FROM comments
		WHERE post_id = $1 AND author_id <> $2
	`
	rows, err := r.pool.Query(ctx, q, postID, excludeAuthorID)
	if err != nil {
		return nil, fmt.Errorf("distinct comment authors: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan author: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authors rows: %w", err)
	}
	return out, nil
}

func (r *CommentRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM comments WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}
