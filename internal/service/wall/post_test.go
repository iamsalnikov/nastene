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
	avatars  *wallmocks.AvatarAuthorizer
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
		avatars:  wallmocks.NewAvatarAuthorizer(t),
	}
	authz := wall.NewAuthorizer(f.privacy, f.friend, f.bans)
	svc := wall.NewService(f.posts, f.users, f.comments, f.friends, authz, f.avatars)
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

func TestService_LoadWall_masksAvatarByPrivacy(t *testing.T) {
	t.Parallel()

	const viewer int64 = 2
	const owner int64 = 3
	const commenter int64 = 4
	createdAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

	post := domain.WallPost{
		ID: 10, WallOwnerID: owner, AuthorID: owner,
		Kind: domain.PostText, BodyText: "hello", CreatedAt: createdAt,
	}
	publicPrivacy := domain.WallPrivacy{
		UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic, CommentScope: domain.ScopePublic,
	}

	svc, f := newLoadPostService(t)

	f.users.EXPECT().ByID(mock.Anything, owner).
		Return(domain.User{ID: owner, DisplayName: "O", AvatarPath: "avatars/o.png"}, nil).Once()
	// ban checks (banned?/iBanned? inside LoadWall)
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	// friendship state (viewer != 0, viewer != owner)
	f.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.friends.EXPECT().HasRequest(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.friends.EXPECT().HasRequest(mock.Anything, owner, viewer).Return(false, nil).Once()
	// CanView → mutuallyBanned viewer/owner, owner/viewer, then privacy
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
	// CanPost
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
	// CanComment
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()

	f.posts.EXPECT().ListByWall(mock.Anything, owner, 21, 0).Return([]domain.WallPost{post}, nil).Once()
	f.comments.EXPECT().ListByPosts(mock.Anything, []int64{post.ID}).Return([]domain.Comment{
		{ID: 1, PostID: post.ID, AuthorID: commenter, Body: "hi", CreatedAt: createdAt},
	}, nil).Once()
	f.users.EXPECT().ByID(mock.Anything, commenter).
		Return(domain.User{ID: commenter, DisplayName: "C", AvatarPath: "avatars/c.png"}, nil).Once()

	f.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, commenter).Return(true, nil).Once()

	view, err := svc.LoadWall(context.Background(), viewer, owner, 20, 0)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.True(t, view.Visible)
	require.Len(t, view.Posts, 1)
	require.Empty(t, view.Posts[0].Author.AvatarPath, "post author avatar hidden when privacy denies viewer")
	require.Len(t, view.Posts[0].Comments, 1)
	require.Equal(t, "avatars/c.png", view.Posts[0].Comments[0].Author.AvatarPath, "commenter avatar remains when viewer can see it")
}

func TestService_LoadWall_pagination(t *testing.T) {
	t.Parallel()

	const viewer int64 = 2
	const owner int64 = 3
	createdAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

	newPost := func(id int64) domain.WallPost {
		return domain.WallPost{
			ID: id, WallOwnerID: owner, AuthorID: owner,
			Kind: domain.PostText, BodyText: "x", CreatedAt: createdAt,
		}
	}
	publicPrivacy := domain.WallPrivacy{
		UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic, CommentScope: domain.ScopePublic,
	}

	tests := map[string]struct {
		limit    int
		offset   int
		dbReturn []domain.WallPost
		wantHas  bool
		wantPrev int
		wantNext int
		wantLen  int
	}{
		"first page, no more": {
			limit: 2, offset: 0,
			dbReturn: []domain.WallPost{newPost(1), newPost(2)},
			wantHas:  false, wantPrev: 0, wantNext: 0, wantLen: 2,
		},
		"first page, has next": {
			limit: 2, offset: 0,
			dbReturn: []domain.WallPost{newPost(1), newPost(2), newPost(3)},
			wantHas:  false, wantPrev: 0, wantNext: 2, wantLen: 2,
		},
		"middle page has both prev and next": {
			limit: 2, offset: 4,
			dbReturn: []domain.WallPost{newPost(5), newPost(6), newPost(7)},
			wantHas:  true, wantPrev: 2, wantNext: 6, wantLen: 2,
		},
		"prev clamps to zero when offset < limit": {
			limit: 20, offset: 5,
			dbReturn: nil,
			wantHas:  true, wantPrev: 0, wantNext: 0, wantLen: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, f := newLoadPostService(t)

			f.users.EXPECT().ByID(mock.Anything, owner).
				Return(domain.User{ID: owner, DisplayName: "O"}, nil).Once()
			// banned/iBannedThem
			f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
			f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
			// friendshipState → AreFriends + HasRequest×2
			f.friends.EXPECT().AreFriends(mock.Anything, viewer, owner).Return(false, nil).Once()
			f.friends.EXPECT().HasRequest(mock.Anything, viewer, owner).Return(false, nil).Once()
			f.friends.EXPECT().HasRequest(mock.Anything, owner, viewer).Return(false, nil).Once()
			// CanView
			f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
			f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
			f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
			// CanPost
			f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
			f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
			f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
			// CanComment
			f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
			f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
			f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()

			f.posts.EXPECT().ListByWall(mock.Anything, owner, tc.limit+1, tc.offset).
				Return(tc.dbReturn, nil).Once()

			ids := make([]int64, 0, tc.wantLen)
			for i := 0; i < tc.wantLen; i++ {
				ids = append(ids, tc.dbReturn[i].ID)
			}
			f.comments.EXPECT().ListByPosts(mock.Anything, ids).Return(nil, nil).Once()

			view, err := svc.LoadWall(context.Background(), viewer, owner, tc.limit, tc.offset)
			require.NoError(t, err)
			require.NotNil(t, view)
			require.Len(t, view.Posts, tc.wantLen)
			require.Equal(t, tc.wantHas, view.HasPrev)
			require.Equal(t, tc.wantPrev, view.PrevPage)
			require.Equal(t, tc.wantNext, view.NextPage)
		})
	}
}

func TestService_LoadPost_masksAvatarByPrivacy(t *testing.T) {
	t.Parallel()

	const viewer int64 = 2
	const owner int64 = 3
	const commenter int64 = 4
	const postID int64 = 100
	createdAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

	samplePost := domain.WallPost{
		ID: postID, WallOwnerID: owner, AuthorID: owner,
		Kind: domain.PostText, BodyText: "hello", CreatedAt: createdAt,
	}
	publicPrivacy := domain.WallPrivacy{
		UserID: owner, ViewScope: domain.ScopePublic, PostScope: domain.ScopePublic, CommentScope: domain.ScopePublic,
	}

	svc, f := newLoadPostService(t)

	f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.users.EXPECT().ByID(mock.Anything, owner).
		Return(domain.User{ID: owner, DisplayName: "O", AvatarPath: "avatars/o.png"}, nil).Once()
	f.comments.EXPECT().ListByPosts(mock.Anything, []int64{postID}).Return([]domain.Comment{
		{ID: 1, PostID: postID, AuthorID: commenter, Body: "hi", CreatedAt: createdAt},
	}, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, commenter).Return(false, nil).Once()
	f.users.EXPECT().ByID(mock.Anything, commenter).
		Return(domain.User{ID: commenter, DisplayName: "C", AvatarPath: "avatars/c.png"}, nil).Once()
	f.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.avatars.EXPECT().CanSeeAvatar(mock.Anything, viewer, commenter).Return(true, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, viewer, owner).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, owner, viewer).Return(false, nil).Once()
	f.privacy.EXPECT().Get(mock.Anything, owner).Return(publicPrivacy, nil).Once()

	got, err := svc.LoadPost(context.Background(), viewer, postID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got.Author.AvatarPath, "author avatar must be stripped when privacy denies viewer")
	require.Empty(t, got.Owner.AvatarPath, "owner avatar must be stripped when privacy denies viewer")
	require.Len(t, got.Comments, 1)
	require.Equal(t, "avatars/c.png", got.Comments[0].Author.AvatarPath, "commenter avatar remains when viewer can see it")
}
