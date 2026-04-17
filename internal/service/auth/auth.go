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
	Create(ctx context.Context, email, passwordHash, displayName string, invitesRemaining int, invitedByUserID *int64) (domain.User, error)
	CreateWithInvite(ctx context.Context, email, passwordHash, displayName, inviteToken string, initialInvites int) (domain.User, int64, error)
	ByEmail(ctx context.Context, email string) (domain.User, error)
}

// FriendRequester шлёт friend-request от нового юзера к приглашателю сразу после регистрации.
type FriendRequester interface {
	SendRequest(ctx context.Context, fromID, toID int64) error
}

type Service struct {
	users          UserRepo
	friends        FriendRequester
	invitesPerUser int
	inviteOnly     bool
}

func NewService(users UserRepo, friends FriendRequester, invitesPerUser int, inviteOnly bool) *Service {
	return &Service{
		users:          users,
		friends:        friends,
		invitesPerUser: invitesPerUser,
		inviteOnly:     inviteOnly,
	}
}

func (s *Service) Register(ctx context.Context, email, password, displayName, inviteToken string) (domain.User, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	inviteToken = strings.TrimSpace(inviteToken)

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

	if inviteToken == "" {
		if s.inviteOnly {
			return domain.User{}, fmt.Errorf("register: invite required: %w", domain.ErrInvalidInvite)
		}
		u, err := s.users.Create(ctx, email, string(hash), displayName, s.invitesPerUser, nil)
		if err != nil {
			return domain.User{}, fmt.Errorf("register: %w", err)
		}
		return u, nil
	}

	u, inviterID, err := s.users.CreateWithInvite(ctx, email, string(hash), displayName, inviteToken, s.invitesPerUser)
	if err != nil {
		return domain.User{}, fmt.Errorf("register: %w", err)
	}
	// friend-request — best-effort: если упадёт, регистрация всё равно успешна.
	_ = s.friends.SendRequest(ctx, u.ID, inviterID)
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
