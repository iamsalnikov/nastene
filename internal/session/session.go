package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type ctxKey int

const userCtxKey ctxKey = 1

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
	TouchLastSeen(ctx context.Context, id int64) error
}

type Repo interface {
	Create(ctx context.Context, token string, userID int64, expiresAt time.Time) (domain.Session, error)
	ByToken(ctx context.Context, token string) (domain.Session, error)
	Delete(ctx context.Context, token string) error
}

type Manager struct {
	repo       Repo
	users      UserRepo
	cookieName string
	ttl        time.Duration
}

func NewManager(repo Repo, users UserRepo, cookieName string, ttl time.Duration) *Manager {
	return &Manager{repo: repo, users: users, cookieName: cookieName, ttl: ttl}
}

func (m *Manager) Issue(ctx context.Context, w http.ResponseWriter, userID int64) error {
	token, err := newToken()
	if err != nil {
		return fmt.Errorf("issue session: %w", err)
	}
	expires := time.Now().Add(m.ttl)
	if _, err := m.repo.Create(ctx, token, userID, expires); err != nil {
		return fmt.Errorf("issue session: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	slog.Default().Info("session: issued",
		"user_id", userID,
		"cookie_name", m.cookieName,
		"token_prefix", token[:8],
		"expires", expires.Format(time.RFC3339),
	)
	return nil
}

func (m *Manager) Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(m.cookieName)
	if err != nil {
		return nil
	}
	if err := m.repo.Delete(ctx, c.Value); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(m.cookieName)
		if err != nil || c.Value == "" {
			if isMutating(r.Method) {
				slog.Default().Debug("session: no cookie", "path", r.URL.Path)
			}
			next.ServeHTTP(w, r)
			return
		}
		sess, err := m.repo.ByToken(r.Context(), c.Value)
		if err != nil {
			slog.Default().Warn("session: token lookup failed", "path", r.URL.Path, "err", err)
			next.ServeHTTP(w, r)
			return
		}
		user, err := m.users.ByID(r.Context(), sess.UserID)
		if err != nil {
			slog.Default().Warn("session: user lookup failed", "user_id", sess.UserID, "err", err)
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/healthz" {
			if err := m.users.TouchLastSeen(r.Context(), user.ID); err != nil {
				slog.Default().Warn("session: touch last seen failed", "user_id", user.ID, "err", err)
			}
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func (m *Manager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func FromContext(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userCtxKey).(domain.User)
	return u, ok
}

func MustUser(ctx context.Context) domain.User {
	u, ok := FromContext(ctx)
	if !ok {
		panic(errors.New("session: no user in context"))
	}
	return u
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
