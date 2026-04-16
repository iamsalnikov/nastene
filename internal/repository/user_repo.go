package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

const userCols = `id, email, password_hash, display_name, created_at,
		gender, birth_date, city, website, activity, quote, bio, avatar_path, last_seen_at`

func scanUser(row pgx.Row, u *domain.User) error {
	return row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt,
		&u.Gender, &u.BirthDate, &u.City, &u.Website, &u.Activity, &u.Quote, &u.Bio, &u.AvatarPath, &u.LastSeenAt,
	)
}

func (r *UserRepo) Create(ctx context.Context, email, passwordHash, displayName string) (domain.User, error) {
	const q = `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING ` + userCols
	var u domain.User
	if err := scanUser(r.pool.QueryRow(ctx, q, email, passwordHash, displayName), &u); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, fmt.Errorf("create user: %w", domain.ErrEmailAlreadyInUse)
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (domain.User, error) {
	const q = `SELECT ` + userCols + ` FROM users WHERE id = $1`
	var u domain.User
	if err := scanUser(r.pool.QueryRow(ctx, q, id), &u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("user by id: %w", domain.ErrNotFound)
		}
		return domain.User{}, fmt.Errorf("user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `SELECT ` + userCols + ` FROM users WHERE LOWER(email) = LOWER($1)`
	var u domain.User
	if err := scanUser(r.pool.QueryRow(ctx, q, email), &u); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("user by email: %w", domain.ErrNotFound)
		}
		return domain.User{}, fmt.Errorf("user by email: %w", err)
	}
	return u, nil
}

// ProfileFields — набор полей, которые правит пользователь через форму профиля.
// Аватар отдельно через UpdateAvatar, чтобы текстовая форма и загрузка файла не зависели друг от друга.
type ProfileFields struct {
	DisplayName string
	Gender      string
	BirthDate   *time.Time
	City        string
	Website     string
	Activity    string
	Quote       string
	Bio         string
}

func (r *UserRepo) UpdateProfile(ctx context.Context, userID int64, p ProfileFields) error {
	const q = `
		UPDATE users SET
			display_name = $2,
			gender       = $3,
			birth_date   = $4,
			city         = $5,
			website      = $6,
			activity     = $7,
			quote        = $8,
			bio          = $9
		WHERE id = $1
	`
	tag, err := r.pool.Exec(ctx, q, userID,
		p.DisplayName, p.Gender, p.BirthDate, p.City, p.Website, p.Activity, p.Quote, p.Bio)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update profile: %w", domain.ErrNotFound)
	}
	return nil
}

func (r *UserRepo) UpdateAvatar(ctx context.Context, userID int64, path string) error {
	const q = `UPDATE users SET avatar_path = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, userID, path)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update avatar: %w", domain.ErrNotFound)
	}
	return nil
}

// TouchLastSeen обновляет last_seen_at не чаще раза в минуту: throttle живёт в WHERE,
// поэтому корректен при многопроцессном деплое и не требует in-memory кэша.
func (r *UserRepo) TouchLastSeen(ctx context.Context, userID int64) error {
	const q = `
		UPDATE users
		SET last_seen_at = NOW()
		WHERE id = $1
		  AND (last_seen_at IS NULL OR last_seen_at < NOW() - INTERVAL '1 minute')
	`
	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("touch last seen: %w", err)
	}
	return nil
}
