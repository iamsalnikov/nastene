package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/auth"
	authmocks "github.com/iamsalnikov/nastene/mocks/auth"
)

func TestService_Register(t *testing.T) {
	t.Parallel()

	type args struct {
		email       string
		password    string
		displayName string
	}
	tests := map[string]struct {
		args      args
		setupMock func(m *authmocks.UserRepo)
		wantErr   error
	}{
		"invalid email is rejected before touching repo": {
			args:      args{email: "not-an-email", password: "secret123", displayName: "Bob"},
			setupMock: func(m *authmocks.UserRepo) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"short password is rejected": {
			args:      args{email: "bob@example.com", password: "123", displayName: "Bob"},
			setupMock: func(m *authmocks.UserRepo) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"short display name is rejected": {
			args:      args{email: "bob@example.com", password: "secret123", displayName: "B"},
			setupMock: func(m *authmocks.UserRepo) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"duplicate email bubbles up": {
			args: args{email: "bob@example.com", password: "secret123", displayName: "Bob"},
			setupMock: func(m *authmocks.UserRepo) {
				m.EXPECT().
					Create(mock.Anything, "bob@example.com", mock.Anything, "Bob").
					Return(domain.User{}, domain.ErrEmailAlreadyInUse).Once()
			},
			wantErr: domain.ErrEmailAlreadyInUse,
		},
		"happy path returns user": {
			args: args{email: "bob@example.com", password: "secret123", displayName: "Bob"},
			setupMock: func(m *authmocks.UserRepo) {
				m.EXPECT().
					Create(mock.Anything, "bob@example.com", mock.Anything, "Bob").
					Return(domain.User{ID: 42, Email: "bob@example.com", DisplayName: "Bob"}, nil).Once()
			},
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := authmocks.NewUserRepo(t)
			tc.setupMock(repo)

			svc := auth.NewService(repo)
			u, err := svc.Register(context.Background(), tc.args.email, tc.args.password, tc.args.displayName)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(42), u.ID)
		})
	}
}

func TestService_Login(t *testing.T) {
	t.Parallel()

	hash := mustHash(t, "secret123")

	tests := map[string]struct {
		email     string
		password  string
		setupMock func(m *authmocks.UserRepo)
		wantErr   error
	}{
		"unknown email → invalid credentials": {
			email:    "ghost@example.com",
			password: "whatever",
			setupMock: func(m *authmocks.UserRepo) {
				m.EXPECT().ByEmail(mock.Anything, "ghost@example.com").
					Return(domain.User{}, domain.ErrNotFound).Once()
			},
			wantErr: domain.ErrInvalidCredentials,
		},
		"wrong password → invalid credentials": {
			email:    "bob@example.com",
			password: "wrong-one",
			setupMock: func(m *authmocks.UserRepo) {
				m.EXPECT().ByEmail(mock.Anything, "bob@example.com").
					Return(domain.User{ID: 1, Email: "bob@example.com", PasswordHash: hash}, nil).Once()
			},
			wantErr: domain.ErrInvalidCredentials,
		},
		"happy path": {
			email:    "bob@example.com",
			password: "secret123",
			setupMock: func(m *authmocks.UserRepo) {
				m.EXPECT().ByEmail(mock.Anything, "bob@example.com").
					Return(domain.User{ID: 1, Email: "bob@example.com", PasswordHash: hash}, nil).Once()
			},
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := authmocks.NewUserRepo(t)
			tc.setupMock(repo)

			svc := auth.NewService(repo)
			u, err := svc.Login(context.Background(), tc.email, tc.password)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(1), u.ID)
		})
	}
}
