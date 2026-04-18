package wall_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	wallmocks "github.com/iamsalnikov/nastene/mocks/wall"
)

type loadPostFixture struct {
	posts    *wallmocks.PostRepo
	users    *wallmocks.UserRepo
	comments *wallmocks.CommentRepo
	friends  *wallmocks.FriendshipQuery
	privacy  *wallmocks.PrivacyRepo
	friend   *wallmocks.FriendRepo
	bans     *wallmocks.BanRepo
}

func newLoadPostService(t *testing.T) (*wall.Service, loadPostFixture) {
	t.Helper()
	f := loadPostFixture{
		posts:    wallmocks.NewPostRepo(t),
		users:    wallmocks.NewUserRepo(t),
		comments: wallmocks.NewCommentRepo(t),
		friends:  wallmocks.NewFriendshipQuery(t),
		privacy:  wallmocks.NewPrivacyRepo(t),
		friend:   wallmocks.NewFriendRepo(t),
		bans:     wallmocks.NewBanRepo(t),
	}
	authz := wall.NewAuthorizer(f.privacy, f.friend, f.bans)
	svc := wall.NewService(f.posts, f.users, f.comments, f.friends, authz)
	return svc, f
}

func TestService_LoadPost(t *testing.T) {
	t.Parallel()

	const viewer int64 = 2
	const owner int64 = 3
	const postID int64 = 100
	createdAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

	samplePost := domain.WallPost{
		ID: postID, WallOwnerID: owner, AuthorID: owner,
		Kind: domain.PostText, BodyText: "hello", CreatedAt: createdAt,
	}
	publicPrivacy := domain.WallPrivacy{
		UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic, CommentScope: domain.ScopePublic,
	}

	tests := map[string]struct {
		viewerID  int64
		setupMock func(f loadPostFixture)
		wantErr   error
		wantCmts  int
		canDel    bool
		canCmt    bool
	}{
		"not found": {
			viewerID: viewer,
			setupMock: func(f loadPostFixture) {
				f.posts.EXPECT().ByID(mock.Anything, postID).Return(domain.WallPost{}, domain.ErrNotFound).Once()
			},
			wantErr: domain.ErrNotFound,
		},
		"forbidden when owner banned viewer": {
			viewerID: viewer,
			setupMock: func(f loadPostFixture) {
				f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()
				// mutuallyBanned in CanView: first viewer→owner false, then owner→viewer true.
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
				f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(true, nil).Once()
			},
			wantErr: domain.ErrForbidden,
		},
		"forbidden when viewer banned owner": {
			viewerID: viewer,
			setupMock: func(f loadPostFixture) {
				f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()
				// mutuallyBanned short-circuits on the first call returning true.
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(true, nil).Once()
			},
			wantErr: domain.ErrForbidden,
		},
		"happy path with 2 comments, one filtered by viewer ban": {
			viewerID: viewer,
			setupMock: func(f loadPostFixture) {
				f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()
				// CanView: mutuallyBanned(viewer, owner) returns false.
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
				f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
				// "viewer banned post author" (author == owner) — one-directional IsBanned.
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
				// owner user lookup
				f.users.EXPECT().ByID(mock.Anything, owner).Return(domain.User{ID: owner, DisplayName: "O"}, nil).Once()
				// comments
				f.comments.EXPECT().ListByPosts(mock.Anything, []int64{postID}).Return([]domain.Comment{
					{ID: 1, PostID: postID, AuthorID: 10, Body: "ok", CreatedAt: createdAt},
					{ID: 2, PostID: postID, AuthorID: 11, Body: "nope", CreatedAt: createdAt},
				}, nil).Once()
				// per-comment ban check (one-directional).
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, int64(10)).Return(false, nil).Once()
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, int64(11)).Return(true, nil).Once()
				// resolve comment10 author
				f.users.EXPECT().ByID(mock.Anything, int64(10)).Return(domain.User{ID: 10, DisplayName: "C10"}, nil).Once()
				// CanComment: mutuallyBanned + privacy again.
				f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
				f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
				f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
			},
			wantCmts: 1,
			canDel:   false,
			canCmt:   true,
		},
		"owner viewing own post can delete and comment": {
			viewerID: owner,
			setupMock: func(f loadPostFixture) {
				f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()
				// CanView: viewer == owner → short-circuit.
				// owner user lookup
				f.users.EXPECT().ByID(mock.Anything, owner).Return(domain.User{ID: owner, DisplayName: "O"}, nil).Once()
				f.comments.EXPECT().ListByPosts(mock.Anything, []int64{postID}).Return(nil, nil).Once()
				// CanComment: viewer == owner → short-circuit.
			},
			wantCmts: 0,
			canDel:   true,
			canCmt:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, f := newLoadPostService(t)
			tc.setupMock(f)

			got, err := svc.LoadPost(context.Background(), tc.viewerID, postID)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Comments, tc.wantCmts)
			require.Equal(t, tc.canDel, got.CanDelete)
			require.Equal(t, tc.canCmt, got.CanComment)
		})
	}
}
