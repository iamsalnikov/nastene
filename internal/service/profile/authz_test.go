package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/profile"
	profilemocks "github.com/iamsalnikov/nastene/mocks/profile"
)

type authorizerMocks struct {
	privacy *profilemocks.PrivacyRepo
	friends *profilemocks.FriendRepo
	bans    *profilemocks.BanRepo
}

func newAuthorizer(t *testing.T) (*profile.Authorizer, authorizerMocks) {
	t.Helper()
	m := authorizerMocks{
		privacy: profilemocks.NewPrivacyRepo(t),
		friends: profilemocks.NewFriendRepo(t),
		bans:    profilemocks.NewBanRepo(t),
	}
	return profile.NewAuthorizer(m.privacy, m.friends, m.bans), m
}

func privacyAll(scope domain.ProfileScope) domain.ProfilePrivacy {
	return domain.ProfilePrivacy{
		OnlineScope: scope, BasicScope: scope, FriendsScope: scope, BioScope: scope,
	}
}

func TestAuthorizer_Visibility(t *testing.T) {
	t.Parallel()

	const owner, viewer = int64(10), int64(20)

	tests := map[string]struct {
		viewerID  int64
		setupMock func(m authorizerMocks)
		want      profile.Visibility
	}{
		"self sees everything": {
			viewerID:  owner,
			setupMock: func(m authorizerMocks) {},
			want:      profile.Visibility{Self: true, Online: true, Basic: true, FriendsList: true, Bio: true},
		},
		"banned viewer sees nothing": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(true, nil).Once()
			},
			want: profile.Visibility{Banned: true},
		},
		"everyone scope, non-friend sees all": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeEveryone), nil).Once()
			},
			want: profile.Visibility{Online: true, Basic: true, FriendsList: true, Bio: true},
		},
		"friends scope, non-friend sees nothing": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeFriends), nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(false, nil).Once()
			},
			want: profile.Visibility{},
		},
		"friends scope, friend sees all (one AreFriends call)": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeFriends), nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(true, nil).Once()
			},
			want: profile.Visibility{Online: true, Basic: true, FriendsList: true, Bio: true},
		},
		"nobody scope hides everything from non-self": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeNobody), nil).Once()
			},
			want: profile.Visibility{},
		},
		"guest with everyone scope sees all": {
			viewerID: 0,
			setupMock: func(m authorizerMocks) {
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeEveryone), nil).Once()
			},
			want: profile.Visibility{Online: true, Basic: true, FriendsList: true, Bio: true},
		},
		"guest with friends scope sees nothing, no AreFriends call": {
			viewerID: 0,
			setupMock: func(m authorizerMocks) {
				m.privacy.EXPECT().Get(mock.Anything, owner).
					Return(privacyAll(domain.ProfileScopeFriends), nil).Once()
			},
			want: profile.Visibility{},
		},
		"mixed scopes — friend gets only friend-allowed fields": {
			viewerID: viewer,
			setupMock: func(m authorizerMocks) {
				m.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				m.privacy.EXPECT().Get(mock.Anything, owner).Return(domain.ProfilePrivacy{
					OnlineScope:  domain.ProfileScopeNobody,
					BasicScope:   domain.ProfileScopeEveryone,
					FriendsScope: domain.ProfileScopeFriends,
					BioScope:     domain.ProfileScopeFriends,
				}, nil).Once()
				m.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(true, nil).Once()
			},
			want: profile.Visibility{Basic: true, FriendsList: true, Bio: true},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			authz, mocks := newAuthorizer(t)
			tc.setupMock(mocks)

			got, err := authz.Visibility(context.Background(), tc.viewerID, owner)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestAuthorizer_CanSee(t *testing.T) {
	t.Parallel()

	const owner, viewer = int64(10), int64(20)
	authz, mocks := newAuthorizer(t)

	mocks.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	mocks.privacy.EXPECT().Get(mock.Anything, owner).Return(domain.ProfilePrivacy{
		OnlineScope:  domain.ProfileScopeEveryone,
		BasicScope:   domain.ProfileScopeNobody,
		FriendsScope: domain.ProfileScopeEveryone,
		BioScope:     domain.ProfileScopeEveryone,
	}, nil).Once()

	ok, err := authz.CanSee(context.Background(), viewer, owner, domain.ProfileFieldBasic)
	require.NoError(t, err)
	require.False(t, ok)
}
