package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash, displayName string) (domain.User, error)
	ByEmail(ctx context.Context, email string) (domain.User, error)
}

type Service struct {
	users UserRepo
}

func NewService(users UserRepo) *Service {
	return &Service{users: users}
}

func (s *Service) Register(ctx context.Context, email, password, displayName string) (domain.User, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)

	if _, err := mail.ParseAddress(email); err != nil {
		return domain.User{}, fmt.Errorf("register: invalid email: %w", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(password) < 6 {
		return domain.User{}, fmt.Errorf("register: password too short: %w", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(displayName) < 2 {
		return domain.User{}, fmt.Errorf("register: display name too short: %w", domain.ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("register: hash password: %w", err)
	}

	u, err := s.users.Create(ctx, email, string(hash), displayName)
	if err != nil {
		return domain.User{}, fmt.Errorf("register: %w", err)
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (domain.User, error) {
	email = strings.TrimSpace(email)

	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, fmt.Errorf("login: %w", domain.ErrInvalidCredentials)
		}
		return domain.User{}, fmt.Errorf("login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, fmt.Errorf("login: %w", domain.ErrInvalidCredentials)
	}
	return u, nil
}
