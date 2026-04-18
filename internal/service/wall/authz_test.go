package wall_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	wallmocks "github.com/iamsalnikov/nastene/mocks/wall"
)

type authorizerMocks struct {
	privacy *wallmocks.PrivacyRepo
	friends *wallmocks.FriendRepo
	bans    *wallmocks.BanRepo
}

func newAuthorizer(t *testing.T) (*wall.Authorizer, authorizerMocks) {
	t.Helper()
	m := authorizerMocks{
		privacy: wallmocks.NewPrivacyRepo(t),
		friends: wallmocks.NewFriendRepo(t),
		bans:    wallmocks.NewBanRepo(t),
	}
	return wall.NewAuthorizer(m.privacy, m.friends, m.bans), m
}

// noBanBetween sets up the two IsBanned calls made by Authorizer.mutuallyBanned,
// both returning false. Order: (viewer, owner) then (owner, viewer).
func noBanBetween(m authorizerMocks, viewer, owner int64) {
	m.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
}

func TestAuthorizer_CanView(t *testing.T) {
	t.Parallel()

	const owner, viewer = int64(10), int64(20)

	tests := map[string]struct {
		viewerID  int64
		setupMock func(m authorizerMocks)
		want      bool
	}{
		"owner always sees own wall": {
			viewerID:  owner,
			setupMock: func(m authorizerMocks) {},
			want:      true,
		},
		"owner banned viewer → blocked": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(true, nil).Once()
			},
			want: false,
		},
		"viewer banned owner → blocked": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(true, nil).Once()
			},
			want: false,
		},
		"public scope → anyone sees": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, viewer, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic}, nil).Once()
			},
			want: true,
		},
		"friends scope + not friend → no": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, viewer, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopeFriends, PostScope: domain.ScopeFriends}, nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(false, nil).Once()
			},
			want: false,
		},
		"friends scope + is friend → yes": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, viewer, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopeFriends, PostScope: domain.ScopeFriends}, nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(true, nil).Once()
			},
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			authz, mocks := newAuthorizer(t)
			tc.setupMock(mocks)

			got, err := authz.CanView(context.Background(), tc.viewerID, owner)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestAuthorizer_CanPost(t *testing.T) {
	t.Parallel()

	const owner, author = int64(10), int64(20)

	tests := map[string]struct {
		authorID  int64
		setupMock func(m authorizerMocks)
		want      bool
	}{
		"owner posts on own wall": {
			authorID:  owner,
			setupMock: func(m authorizerMocks) {},
			want:      true,
		},
		"owner banned author → blocked": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, author, owner).Return(false, nil).Once()
				m.bans.EXPECT().IsBanned(mock.Anything, owner, author).Return(true, nil).Once()
			},
			want: false,
		},
		"author banned owner → blocked": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, author, owner).Return(true, nil).Once()
			},
			want: false,
		},
		"public post scope → yes": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic}, nil).Once()
			},
			want: true,
		},
		"friends post scope + non-friend → no": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopeFriends}, nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, author, owner).Return(false, nil).Once()
			},
			want: false,
		},
		"nobody post scope → no for non-owner": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopeNobody}, nil).Once()
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			authz, mocks := newAuthorizer(t)
			tc.setupMock(mocks)

			got, err := authz.CanPost(context.Background(), tc.authorID, owner)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestAuthorizer_CanComment(t *testing.T) {
	t.Parallel()

	const owner, author = int64(10), int64(20)

	tests := map[string]struct {
		authorID  int64
		setupMock func(m authorizerMocks)
		want      bool
	}{
		"owner comments on own wall": {
			authorID:  owner,
			setupMock: func(m authorizerMocks) {},
			want:      true,
		},
		"owner banned author → blocked": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, author, owner).Return(false, nil).Once()
				m.bans.EXPECT().IsBanned(mock.Anything, owner, author).Return(true, nil).Once()
			},
			want: false,
		},
		"public comment scope → yes": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, CommentScope: domain.ScopePublic}, nil).Once()
			},
			want: true,
		},
		"friends comment scope + friend → yes": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, CommentScope: domain.ScopeFriends}, nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, author, owner).Return(true, nil).Once()
			},
			want: true,
		},
		"nobody comment scope → no for non-owner": {
			authorID: author,
			setupMock: func(m authorizerMocks) {
				noBanBetween(m, author, owner)
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(domain.WallPrivacy{UserID: owner, CommentScope: domain.ScopeNobody}, nil).Once()
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			authz, mocks := newAuthorizer(t)
			tc.setupMock(mocks)

			got, err := authz.CanComment(context.Background(), tc.authorID, owner)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
