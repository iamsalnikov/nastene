package profile_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/repository"
	"github.com/iamsalnikov/nastene/internal/service/profile"
	profilemocks "github.com/iamsalnikov/nastene/mocks/profile"
)

type serviceMocks struct {
	users   *profilemocks.UserRepo
	privacy *profilemocks.ProfilePrivacyRepo
	friends *profilemocks.FriendListRepo
	store   *profilemocks.AvatarStore
	authz   authorizerMocks
}

func newService(t *testing.T) (*profile.Service, serviceMocks) {
	t.Helper()
	authz, am := newAuthorizer(t)
	m := serviceMocks{
		users:   profilemocks.NewUserRepo(t),
		privacy: profilemocks.NewProfilePrivacyRepo(t),
		friends: profilemocks.NewFriendListRepo(t),
		store:   profilemocks.NewAvatarStore(t),
		authz:   am,
	}
	return profile.NewService(m.users, m.privacy, m.friends, authz, m.store), m
}

func ownerUser() domain.User {
	now := time.Now()
	bd := time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC)
	return domain.User{
		ID: 10, Email: "o@x.y", DisplayName: "Owner", CreatedAt: now,
		Gender: "male", BirthDate: &bd, City: "SPb", Website: "https://example.com",
		Activity: "Go", Quote: "be water", Bio: "hello", AvatarPath: "avatars/10-x.png",
		LastSeenAt: &now,
	}
}

func TestService_Get_BannedHidesAll(t *testing.T) {
	t.Parallel()
	const owner, viewer = int64(10), int64(20)

	svc, m := newService(t)
	m.users.EXPECT().ByID(mock.Anything, int64(owner)).Return(ownerUser(), nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(owner), int64(viewer)).Return(true, nil).Once()
	m.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(domain.DefaultProfilePrivacy(owner), nil).Once()

	view, err := svc.Get(context.Background(), viewer, owner)
	require.NoError(t, err)
	require.True(t, view.Visibility.Banned)
	require.Empty(t, view.User.AvatarPath)
	require.Empty(t, view.User.Bio)
	require.Nil(t, view.User.LastSeenAt)
	require.Equal(t, "Owner", view.User.DisplayName)
}

func TestService_Get_MasksHiddenFields(t *testing.T) {
	t.Parallel()
	const owner, viewer = int64(10), int64(20)

	svc, m := newService(t)
	m.users.EXPECT().ByID(mock.Anything, int64(owner)).Return(ownerUser(), nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(owner), int64(viewer)).Return(false, nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(viewer), int64(owner)).Return(false, nil).Once()
	m.authz.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(domain.ProfilePrivacy{
		OnlineScope: domain.ProfileScopeNobody, BasicScope: domain.ProfileScopeNobody,
		FriendsScope: domain.ProfileScopeNobody, BioScope: domain.ProfileScopeNobody,
	}, nil).Once()
	m.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(domain.ProfilePrivacy{
		OnlineScope: domain.ProfileScopeNobody, BasicScope: domain.ProfileScopeNobody,
		FriendsScope: domain.ProfileScopeNobody, BioScope: domain.ProfileScopeNobody,
	}, nil).Once()

	view, err := svc.Get(context.Background(), viewer, owner)
	require.NoError(t, err)
	require.False(t, view.Visibility.Banned)
	require.Empty(t, view.User.AvatarPath)
	require.Empty(t, view.User.City)
	require.Empty(t, view.User.Bio)
	require.Empty(t, view.User.Website)
	require.Nil(t, view.User.LastSeenAt)
	require.Nil(t, view.User.BirthDate)
	require.Equal(t, "Owner", view.User.DisplayName)
}

func TestService_Get_SelfSeesEverything(t *testing.T) {
	t.Parallel()
	const owner = int64(10)

	svc, m := newService(t)
	m.users.EXPECT().ByID(mock.Anything, int64(owner)).Return(ownerUser(), nil).Once()
	m.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(domain.ProfilePrivacy{
		OnlineScope: domain.ProfileScopeNobody, BasicScope: domain.ProfileScopeNobody,
		FriendsScope: domain.ProfileScopeNobody, BioScope: domain.ProfileScopeNobody,
	}, nil).Once()
	m.friends.EXPECT().ListFriendIDs(mock.Anything, int64(owner)).Return(nil, nil).Once()

	view, err := svc.Get(context.Background(), owner, owner)
	require.NoError(t, err)
	require.True(t, view.Visibility.Self)
	require.Equal(t, "avatars/10-x.png", view.User.AvatarPath)
	require.Equal(t, "hello", view.User.Bio)
}

func TestService_Get_MasksFriendAvatars(t *testing.T) {
	t.Parallel()
	const owner, viewer = int64(10), int64(20)
	const friendA, friendB = int64(30), int64(31)

	svc, m := newService(t)
	m.users.EXPECT().ByID(mock.Anything, int64(owner)).Return(ownerUser(), nil).Once()
	// Visibility: not banned in either direction, everything public so friends list visible.
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(owner), int64(viewer)).Return(false, nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(viewer), int64(owner)).Return(false, nil).Once()
	everythingPublic := domain.ProfilePrivacy{
		OnlineScope: domain.ProfileScopeEveryone, BasicScope: domain.ProfileScopeEveryone,
		FriendsScope: domain.ProfileScopeEveryone, BioScope: domain.ProfileScopeEveryone,
	}
	m.authz.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(everythingPublic, nil).Once()
	m.privacy.EXPECT().Get(mock.Anything, int64(owner)).Return(everythingPublic, nil).Once()

	m.friends.EXPECT().ListFriendIDs(mock.Anything, int64(owner)).Return([]int64{friendA, friendB}, nil).Once()
	m.users.EXPECT().ByID(mock.Anything, friendA).
		Return(domain.User{ID: friendA, DisplayName: "A", AvatarPath: "avatars/a.png"}, nil).Once()
	m.users.EXPECT().ByID(mock.Anything, friendB).
		Return(domain.User{ID: friendB, DisplayName: "B", AvatarPath: "avatars/b.png"}, nil).Once()

	// friendA: viewer cannot see avatar. friendB: can.
	// CanSeeAvatar → Visibility(viewer, friendA). Neither banned, privacy Nobody scope → Basic=false.
	m.authz.bans.EXPECT().IsBanned(mock.Anything, friendA, int64(viewer)).Return(false, nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(viewer), friendA).Return(false, nil).Once()
	m.authz.privacy.EXPECT().Get(mock.Anything, friendA).Return(domain.ProfilePrivacy{
		OnlineScope: domain.ProfileScopeNobody, BasicScope: domain.ProfileScopeNobody,
		FriendsScope: domain.ProfileScopeNobody, BioScope: domain.ProfileScopeNobody,
	}, nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, friendB, int64(viewer)).Return(false, nil).Once()
	m.authz.bans.EXPECT().IsBanned(mock.Anything, int64(viewer), friendB).Return(false, nil).Once()
	m.authz.privacy.EXPECT().Get(mock.Anything, friendB).Return(everythingPublic, nil).Once()

	view, err := svc.Get(context.Background(), viewer, owner)
	require.NoError(t, err)
	require.Len(t, view.Friends, 2)
	require.Empty(t, view.Friends[0].AvatarPath, "friend A avatar must be masked by privacy")
	require.Equal(t, "avatars/b.png", view.Friends[1].AvatarPath, "friend B avatar remains visible")
}

func TestService_UpdateProfile_Validation(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(48 * time.Hour)

	tests := map[string]struct {
		in      profile.UpdateInput
		wantErr error
	}{
		"display name too short": {
			in:      profile.UpdateInput{DisplayName: "a"},
			wantErr: domain.ErrInvalidInput,
		},
		"bad gender": {
			in:      profile.UpdateInput{DisplayName: "Ok", Gender: "other"},
			wantErr: domain.ErrInvalidInput,
		},
		"future birthdate": {
			in:      profile.UpdateInput{DisplayName: "Ok", BirthDate: &future},
			wantErr: domain.ErrInvalidInput,
		},
		"website without scheme": {
			in:      profile.UpdateInput{DisplayName: "Ok", Website: "example.com"},
			wantErr: domain.ErrInvalidInput,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, _ := newService(t)
			err := svc.UpdateProfile(context.Background(), 10, tc.in)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestService_UpdateProfile_Success(t *testing.T) {
	t.Parallel()
	svc, m := newService(t)
	m.users.EXPECT().UpdateProfile(mock.Anything, int64(10), mock.MatchedBy(func(p repository.ProfileFields) bool {
		return p.DisplayName == "Owner" && p.Gender == "male" && p.Website == "https://example.com"
	})).Return(nil).Once()
	err := svc.UpdateProfile(context.Background(), 10, profile.UpdateInput{
		DisplayName: "Owner", Gender: "male", Website: "https://example.com",
	})
	require.NoError(t, err)
}

func TestService_UpdateAvatar_RejectsNonImage(t *testing.T) {
	t.Parallel()
	svc, _ := newService(t)
	_, err := svc.UpdateAvatar(context.Background(), 10, bytes.NewBufferString("not an image"))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestService_UpdateAvatar_AcceptsPNG(t *testing.T) {
	t.Parallel()
	svc, m := newService(t)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	m.store.EXPECT().Save(mock.Anything, mock.MatchedBy(func(rel string) bool {
		return len(rel) > len("avatars/10-") && rel[:11] == "avatars/10-" && rel[len(rel)-4:] == ".png"
	}), mock.Anything, mock.AnythingOfType("int64"), "image/png").Return(nil).Once()
	m.users.EXPECT().UpdateAvatar(mock.Anything, int64(10), mock.AnythingOfType("string")).Return(nil).Once()

	path, err := svc.UpdateAvatar(context.Background(), 10, &buf)
	require.NoError(t, err)
	require.Contains(t, path, "avatars/10-")
	require.Contains(t, path, ".png")
}

func TestService_UpdatePrivacy_RejectsBadScope(t *testing.T) {
	t.Parallel()
	svc, _ := newService(t)
	err := svc.UpdatePrivacy(context.Background(), 10, domain.ProfilePrivacy{
		OnlineScope: "bogus", BasicScope: "everyone", FriendsScope: "everyone", BioScope: "everyone",
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestService_EnsureDefaults(t *testing.T) {
	t.Parallel()
	svc, m := newService(t)
	m.privacy.EXPECT().EnsureDefaults(mock.Anything, int64(10)).Return(nil).Once()
	require.NoError(t, svc.EnsureDefaults(context.Background(), 10))
}
