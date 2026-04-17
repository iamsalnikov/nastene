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

const defaultInvitesPerUser = 2

func TestService_Register(t *testing.T) {
	t.Parallel()

	type args struct {
		email       string
		password    string
		displayName string
		invite      string
	}
	tests := map[string]struct {
		args       args
		inviteOnly bool
		setupMock  func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester)
		wantErr    error
		wantUserID int64
	}{
		"invalid email is rejected before touching repo": {
			args:      args{email: "not-an-email", password: "secret123", displayName: "Bob"},
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"short password is rejected": {
			args:      args{email: "bob@example.com", password: "123", displayName: "Bob"},
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"short display name is rejected": {
			args:      args{email: "bob@example.com", password: "secret123", displayName: "B"},
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {},
			wantErr:   domain.ErrInvalidInput,
		},
		"invite-only without token → ErrInvalidInvite": {
			args:       args{email: "bob@example.com", password: "secret123", displayName: "Bob"},
			inviteOnly: true,
			setupMock:  func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {},
			wantErr:    domain.ErrInvalidInvite,
		},
		"open mode without token creates user, no friend request": {
			args:       args{email: "bob@example.com", password: "secret123", displayName: "Bob"},
			inviteOnly: false,
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {
				users.EXPECT().
					Create(mock.Anything, "bob@example.com", mock.Anything, "Bob", defaultInvitesPerUser, (*int64)(nil)).
					Return(domain.User{ID: 42, Email: "bob@example.com", DisplayName: "Bob"}, nil).Once()
			},
			wantUserID: 42,
		},
		"duplicate email (open mode) bubbles up": {
			args:       args{email: "bob@example.com", password: "secret123", displayName: "Bob"},
			inviteOnly: false,
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {
				users.EXPECT().
					Create(mock.Anything, "bob@example.com", mock.Anything, "Bob", defaultInvitesPerUser, (*int64)(nil)).
					Return(domain.User{}, domain.ErrEmailAlreadyInUse).Once()
			},
			wantErr: domain.ErrEmailAlreadyInUse,
		},
		"valid invite creates user and sends friend request to inviter": {
			args:       args{email: "newbie@example.com", password: "secret123", displayName: "Newbie", invite: "tok-1"},
			inviteOnly: true,
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {
				users.EXPECT().
					CreateWithInvite(mock.Anything, "newbie@example.com", mock.Anything, "Newbie", "tok-1", defaultInvitesPerUser).
					Return(domain.User{ID: 99, Email: "newbie@example.com", DisplayName: "Newbie"}, int64(7), nil).Once()
				friends.EXPECT().
					SendRequest(mock.Anything, int64(99), int64(7)).
					Return(nil).Once()
			},
			wantUserID: 99,
		},
		"invalid invite never touches friends": {
			args:       args{email: "newbie@example.com", password: "secret123", displayName: "Newbie", invite: "bad"},
			inviteOnly: true,
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {
				users.EXPECT().
					CreateWithInvite(mock.Anything, "newbie@example.com", mock.Anything, "Newbie", "bad", defaultInvitesPerUser).
					Return(domain.User{}, int64(0), domain.ErrInvalidInvite).Once()
			},
			wantErr: domain.ErrInvalidInvite,
		},
		"friend request failure is swallowed (registration still succeeds)": {
			args:       args{email: "newbie@example.com", password: "secret123", displayName: "Newbie", invite: "tok-1"},
			inviteOnly: true,
			setupMock: func(t *testing.T, users *authmocks.UserRepo, friends *authmocks.FriendRequester) {
				users.EXPECT().
					CreateWithInvite(mock.Anything, "newbie@example.com", mock.Anything, "Newbie", "tok-1", defaultInvitesPerUser).
					Return(domain.User{ID: 123, Email: "newbie@example.com", DisplayName: "Newbie"}, int64(5), nil).Once()
				friends.EXPECT().
					SendRequest(mock.Anything, int64(123), int64(5)).
					Return(errors.New("flaky")).Once()
			},
			wantUserID: 123,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			users := authmocks.NewUserRepo(t)
			friends := authmocks.NewFriendRequester(t)
			tc.setupMock(t, users, friends)

			svc := auth.NewService(users, friends, defaultInvitesPerUser, tc.inviteOnly)
			u, err := svc.Register(context.Background(), tc.args.email, tc.args.password, tc.args.displayName, tc.args.invite)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantUserID, u.ID)
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

			svc := auth.NewService(repo, authmocks.NewFriendRequester(t), defaultInvitesPerUser, false)
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
