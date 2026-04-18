package news_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/news"
	newsmocks "github.com/iamsalnikov/nastene/mocks/news"
)

type serviceMocks struct {
	feed       *newsmocks.FeedRepo
	users      *newsmocks.UserRepo
	authorizer *newsmocks.Authorizer
	avatars    *newsmocks.AvatarAuthorizer
}

func newService(t *testing.T) (*news.Service, serviceMocks) {
	t.Helper()
	m := serviceMocks{
		feed:       newsmocks.NewFeedRepo(t),
		users:      newsmocks.NewUserRepo(t),
		authorizer: newsmocks.NewAuthorizer(t),
		avatars:    newsmocks.NewAvatarAuthorizer(t),
	}
	return news.NewService(m.feed, m.users, m.authorizer, m.avatars), m
}

func TestService_LoadFeed(t *testing.T) {
	t.Parallel()

	const viewer int64 = 1
	const friend int64 = 2
	const other int64 = 3
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	postEvent := domain.NewsEvent{
		Kind: domain.NewsEventPost, EventID: 100, CreatedAt: now, WallOwnerID: friend,
		AuthorID: friend, PostID: 100, PostKind: domain.PostText, BodyText: "hi",
	}
	commentEvent := domain.NewsEvent{
		Kind: domain.NewsEventComment, EventID: 200, CreatedAt: now.Add(-time.Hour),
		WallOwnerID: viewer, AuthorID: other, PostID: 50, CommentID: 200, BodyText: "nice",
	}

	tests := map[string]struct {
		limit     int
		offset    int
		setupMock func(m serviceMocks)
		wantCount int
		wantNext  int
		wantPrev  int
		wantHas   bool
	}{
		"single comment on own wall": {
			limit:  20,
			offset: 0,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).
					Return([]domain.NewsEvent{commentEvent}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, viewer).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, other).Return(domain.User{ID: other, DisplayName: "O"}, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, viewer).Return(domain.User{ID: viewer, DisplayName: "V"}, nil).Once()
			},
			wantCount: 1,
			wantNext:  0,
		},
		"foreign post on viewer's wall": {
			limit:  20,
			offset: 0,
			setupMock: func(m serviceMocks) {
				foreignPost := domain.NewsEvent{
					Kind: domain.NewsEventPost, EventID: 300, CreatedAt: now,
					WallOwnerID: viewer, AuthorID: other, PostID: 300,
					PostKind: domain.PostText, BodyText: "hey",
				}
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).
					Return([]domain.NewsEvent{foreignPost}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, viewer).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, other).Return(domain.User{ID: other, DisplayName: "O"}, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, viewer).Return(domain.User{ID: viewer, DisplayName: "V"}, nil).Once()
			},
			wantCount: 1,
			wantNext:  0,
		},
		"mixed events, all visible": {
			limit:  20,
			offset: 0,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).
					Return([]domain.NewsEvent{postEvent, commentEvent}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, friend).Return(true, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, viewer).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, friend).Return(domain.User{ID: friend, DisplayName: "F"}, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, viewer).Return(domain.User{ID: viewer, DisplayName: "V"}, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, other).Return(domain.User{ID: other, DisplayName: "O"}, nil).Once()
			},
			wantCount: 2,
			wantNext:  0,
		},
		"CanView=false hides friend's post": {
			limit:  20,
			offset: 0,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).
					Return([]domain.NewsEvent{postEvent}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, friend).Return(false, nil).Once()
			},
			wantCount: 0,
			wantNext:  0,
		},
		"hasNext when raw exceeds limit": {
			limit:  1,
			offset: 0,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 2, 0).
					Return([]domain.NewsEvent{postEvent, commentEvent}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, friend).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, friend).Return(domain.User{ID: friend}, nil).Once()
			},
			wantCount: 1,
			wantNext:  1,
		},
		"CanView cached for same owner": {
			limit:  20,
			offset: 0,
			setupMock: func(m serviceMocks) {
				secondPost := postEvent
				secondPost.EventID = 101
				secondPost.PostID = 101
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).
					Return([]domain.NewsEvent{postEvent, secondPost}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, friend).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, friend).Return(domain.User{ID: friend}, nil).Once()
			},
			wantCount: 2,
			wantNext:  0,
		},
		"offset > 0 exposes prev page": {
			limit:  20,
			offset: 40,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 40).
					Return([]domain.NewsEvent{postEvent}, nil).Once()
				m.authorizer.EXPECT().CanView(mock.Anything, viewer, friend).Return(true, nil).Once()
				m.users.EXPECT().ByID(mock.Anything, friend).Return(domain.User{ID: friend}, nil).Once()
			},
			wantCount: 1,
			wantNext:  0,
			wantPrev:  20,
			wantHas:   true,
		},
		"prev page clamps to zero when offset < limit": {
			limit:  20,
			offset: 5,
			setupMock: func(m serviceMocks) {
				m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 5).
					Return(nil, nil).Once()
			},
			wantCount: 0,
			wantNext:  0,
			wantPrev:  0,
			wantHas:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, m := newService(t)
			tc.setupMock(m)

			view, err := svc.LoadFeed(context.Background(), viewer, tc.limit, tc.offset)
			require.NoError(t, err)
			require.NotNil(t, view)
			require.Len(t, view.Events, tc.wantCount)
			require.Equal(t, tc.wantNext, view.NextPage)
			require.Equal(t, tc.wantPrev, view.PrevPage)
			require.Equal(t, tc.wantHas, view.HasPrev)
		})
	}
}

func TestService_LoadFeed_masksAvatarByPrivacy(t *testing.T) {
	t.Parallel()

	const viewer int64 = 1
	const author int64 = 2
	const owner int64 = 3
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	ev := domain.NewsEvent{
		Kind: domain.NewsEventPost, EventID: 1, CreatedAt: now,
		WallOwnerID: owner, AuthorID: author, PostID: 1,
		PostKind: domain.PostText, BodyText: "hi",
	}

	svc, m := newService(t)
	m.feed.EXPECT().ListFeed(mock.Anything, viewer, 21, 0).Return([]domain.NewsEvent{ev}, nil).Once()
	m.authorizer.EXPECT().CanView(mock.Anything, viewer, owner).Return(true, nil).Once()
	m.users.EXPECT().ByID(mock.Anything, author).
		Return(domain.User{ID: author, DisplayName: "A", AvatarPath: "avatars/a.png"}, nil).Once()
	m.users.EXPECT().ByID(mock.Anything, owner).
		Return(domain.User{ID: owner, DisplayName: "O", AvatarPath: "avatars/o.png"}, nil).Once()
	m.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, author).Return(false, nil).Once()
	m.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, owner).Return(true, nil).Once()

	view, err := svc.LoadFeed(context.Background(), viewer, 20, 0)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Len(t, view.Events, 1)
	require.Empty(t, view.Events[0].Author.AvatarPath, "author avatar hidden when privacy denies viewer")
	require.Equal(t, "avatars/o.png", view.Events[0].Owner.AvatarPath, "owner avatar remains when viewer can see it")
}
