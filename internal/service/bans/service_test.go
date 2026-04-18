package bans_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/bans"
	bansmocks "github.com/iamsalnikov/nastene/mocks/bans"
	eventsmocks "github.com/iamsalnikov/nastene/mocks/events"
)

type fixture struct {
	repo    *bansmocks.Repo
	friends *bansmocks.FriendRepo
	pub     *eventsmocks.Publisher
}

func newService(t *testing.T) (*bans.Service, fixture) {
	t.Helper()
	f := fixture{
		repo:    bansmocks.NewRepo(t),
		friends: bansmocks.NewFriendRepo(t),
		pub:     eventsmocks.NewPublisher(t),
	}
	return bans.NewService(f.repo, f.friends, f.pub, nil), f
}

func TestAdd_TearsDownFriendshipRequestsAndPublishes(t *testing.T) {
	t.Parallel()
	const banner, banned = int64(1), int64(2)

	svc, f := newService(t)
	f.friends.EXPECT().RemoveFriendship(mock.Anything, banner, banned).Return(nil).Once()
	f.friends.EXPECT().RemoveRequest(mock.Anything, banner, banned).Return(nil).Once()
	f.friends.EXPECT().RemoveRequest(mock.Anything, banned, banner).Return(nil).Once()
	f.repo.EXPECT().Add(mock.Anything, banner, banned).Return(nil).Once()
	f.pub.EXPECT().Publish("wall.ban.created", mock.Anything).Return(nil).Once()

	require.NoError(t, svc.Add(context.Background(), banner, banned))
}

func TestAdd_SelfActionRejected(t *testing.T) {
	t.Parallel()
	svc, _ := newService(t)
	err := svc.Add(context.Background(), 1, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrSelfAction))
}

func TestIsMutuallyInvisible(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		setup func(f fixture)
		want  bool
	}{
		"forward ban makes invisible": {
			setup: func(f fixture) {
				f.repo.EXPECT().IsBanned(mock.Anything, int64(1), int64(2)).Return(true, nil).Once()
			},
			want: true,
		},
		"reverse ban makes invisible": {
			setup: func(f fixture) {
				f.repo.EXPECT().IsBanned(mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
				f.repo.EXPECT().IsBanned(mock.Anything, int64(2), int64(1)).Return(true, nil).Once()
			},
			want: true,
		},
		"no ban": {
			setup: func(f fixture) {
				f.repo.EXPECT().IsBanned(mock.Anything, int64(1), int64(2)).Return(false, nil).Once()
				f.repo.EXPECT().IsBanned(mock.Anything, int64(2), int64(1)).Return(false, nil).Once()
			},
			want: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, f := newService(t)
			tc.setup(f)
			got, err := svc.IsMutuallyInvisible(context.Background(), 1, 2)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
