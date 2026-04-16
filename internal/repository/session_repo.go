package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

func (r *SessionRepo) Create(ctx context.Context, token string, userID int64, expiresAt time.Time) (domain.Session, error) {
	const q = `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING token, user_id, expires_at, created_at
	`
	var s domain.Session
	err := r.pool.QueryRow(ctx, q, token, userID, expiresAt).
		Scan(&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

func (r *SessionRepo) ByToken(ctx context.Context, token string) (domain.Session, error) {
	const q = `SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = $1 AND expires_at > NOW()`
	var s domain.Session
	err := r.pool.QueryRow(ctx, q, token).
		Scan(&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, fmt.Errorf("session by token: %w", domain.ErrNotFound)
		}
		return domain.Session{}, fmt.Errorf("session by token: %w", err)
	}
	return s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	if _, err := r.pool.Exec(ctx, q, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) error {
	const q = `DELETE FROM sessions WHERE expires_at <= NOW()`
	if _, err := r.pool.Exec(ctx, q); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
