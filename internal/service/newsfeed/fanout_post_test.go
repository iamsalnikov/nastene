package newsfeed_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
	"github.com/iamsalnikov/nastene/internal/service/newsfeed"
	newsfeedmocks "github.com/iamsalnikov/nastene/mocks/newsfeed"
)

type fanoutFixture struct {
	posts    *newsfeedmocks.PostRepo
	comments *newsfeedmocks.CommentRepo
	friends  *newsfeedmocks.FriendRepo
	bans     *newsfeedmocks.BanChecker
	authz    *newsfeedmocks.Authorizer
	feed     *newsfeedmocks.FeedRepo
}

func newFanoutService(t *testing.T) (*newsfeed.Service, fanoutFixture) {
	t.Helper()
	f := fanoutFixture{
		posts:    newsfeedmocks.NewPostRepo(t),
		comments: newsfeedmocks.NewCommentRepo(t),
		friends:  newsfeedmocks.NewFriendRepo(t),
		bans:     newsfeedmocks.NewBanChecker(t),
		authz:    newsfeedmocks.NewAuthorizer(t),
		feed:     newsfeedmocks.NewFeedRepo(t),
	}
	return newsfeed.NewService(f.posts, f.comments, f.friends, f.bans, f.authz, f.feed), f
}

// noBan wires bans checker so IsBanned returns false in both directions for (a,b).
func noBan(f fanoutFixture, a, b int64) {
	f.bans.EXPECT().IsBanned(mock.Anything, a, b).Return(false, nil).Once()
	f.bans.EXPECT().IsBanned(mock.Anything, b, a).Return(false, nil).Once()
}

func TestService_HandleWallPostCreated(t *testing.T) {
	t.Parallel()

	const author, friendA, friendB, bannedFriend = int64(1), int64(2), int64(3), int64(4)
	createdAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	t.Run("self-wall post fans out to non-banned friends", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		post := domain.WallPost{ID: 100, AuthorID: author, WallOwnerID: author, CreatedAt: createdAt}
		f.posts.EXPECT().ByID(mock.Anything, int64(100)).Return(post, nil).Once()
		f.friends.EXPECT().ListFriendIDs(mock.Anything, author).Return([]int64{friendA, friendB, bannedFriend}, nil).Once()

		// friendA: no ban, can view
		noBan(f, friendA, author)
		f.authz.EXPECT().CanView(mock.Anything, friendA, author).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == friendA && r.PostID == 100 && r.Kind == "post" && !r.Hidden
		})).Return(nil).Once()

		// friendB: no ban, privacy hides → hidden=true
		noBan(f, friendB, author)
		f.authz.EXPECT().CanView(mock.Anything, friendB, author).Return(false, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == friendB && r.Hidden
		})).Return(nil).Once()

		// bannedFriend: author→banned ban exists → skip
		f.bans.EXPECT().IsBanned(mock.Anything, bannedFriend, author).Return(true, nil).Once()

		err := svc.HandleWallPostCreated(context.Background(), events.WallPostCreated{PostID: 100})
		require.NoError(t, err)
	})

	t.Run("post on foreign wall fans out only to owner", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		post := domain.WallPost{ID: 200, AuthorID: author, WallOwnerID: friendA, CreatedAt: createdAt}
		f.posts.EXPECT().ByID(mock.Anything, int64(200)).Return(post, nil).Once()
		noBan(f, friendA, author)
		f.authz.EXPECT().CanView(mock.Anything, friendA, friendA).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == friendA && r.PostID == 200 && r.Kind == "post"
		})).Return(nil).Once()

		err := svc.HandleWallPostCreated(context.Background(), events.WallPostCreated{PostID: 200})
		require.NoError(t, err)
	})

	t.Run("post on foreign wall with mutual ban delivers to nobody", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		post := domain.WallPost{ID: 300, AuthorID: author, WallOwnerID: friendA, CreatedAt: createdAt}
		f.posts.EXPECT().ByID(mock.Anything, int64(300)).Return(post, nil).Once()
		f.bans.EXPECT().IsBanned(mock.Anything, friendA, author).Return(true, nil).Once()

		err := svc.HandleWallPostCreated(context.Background(), events.WallPostCreated{PostID: 300})
		require.NoError(t, err)
	})
}

func TestService_HandleCommentCreated(t *testing.T) {
	t.Parallel()

	const author, postAuthor, wallOwner = int64(1), int64(2), int64(3)
	createdAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	t.Run("deliver to post author and wall owner", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		c := domain.Comment{ID: 500, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: wallOwner, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(500)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return(nil, nil).Once()

		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor && r.CommentID == 500 && r.Kind == "comment"
		})).Return(nil).Once()

		noBan(f, wallOwner, author)
		f.authz.EXPECT().CanView(mock.Anything, wallOwner, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == wallOwner && r.CommentID == 500
		})).Return(nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 500})
		require.NoError(t, err)
	})

	t.Run("skip delivery to banned recipient", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		c := domain.Comment{ID: 600, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: postAuthor, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(600)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return(nil, nil).Once()

		// postAuthor banned author → skip
		f.bans.EXPECT().IsBanned(mock.Anything, postAuthor, author).Return(true, nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 600})
		require.NoError(t, err)
	})

	t.Run("CanView=false stores comment hidden", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		c := domain.Comment{ID: 700, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: postAuthor, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(700)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return(nil, nil).Once()

		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, postAuthor).Return(false, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor && r.Hidden
		})).Return(nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 700})
		require.NoError(t, err)
	})

	t.Run("prior commenter gets the new comment", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		const prior = int64(7)
		c := domain.Comment{ID: 800, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: wallOwner, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(800)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return([]int64{prior}, nil).Once()

		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor && r.CommentID == 800
		})).Return(nil).Once()

		noBan(f, wallOwner, author)
		f.authz.EXPECT().CanView(mock.Anything, wallOwner, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == wallOwner && r.CommentID == 800
		})).Return(nil).Once()

		noBan(f, prior, author)
		f.authz.EXPECT().CanView(mock.Anything, prior, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == prior && r.CommentID == 800 && r.Kind == "comment" && !r.Hidden
		})).Return(nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 800})
		require.NoError(t, err)
	})

	t.Run("prior commenter overlapping with post author is not duplicated", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		c := domain.Comment{ID: 900, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: postAuthor, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(900)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return([]int64{postAuthor}, nil).Once()

		// postAuthor ожидается ровно один раз — повторный .Once() сработал бы дважды при дубле.
		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, postAuthor).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor && r.CommentID == 900
		})).Return(nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 900})
		require.NoError(t, err)
	})

	t.Run("prior commenter is skipped when mutually banned", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		const prior = int64(8)
		c := domain.Comment{ID: 1000, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: postAuthor, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(1000)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return([]int64{prior}, nil).Once()

		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, postAuthor).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor
		})).Return(nil).Once()

		// prior забанил author → Insert не вызывается
		f.bans.EXPECT().IsBanned(mock.Anything, prior, author).Return(true, nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 1000})
		require.NoError(t, err)
	})

	t.Run("prior commenter with CanView=false gets hidden row", func(t *testing.T) {
		t.Parallel()
		svc, f := newFanoutService(t)

		const prior = int64(9)
		c := domain.Comment{ID: 1100, PostID: 10, AuthorID: author, CreatedAt: createdAt}
		post := domain.WallPost{ID: 10, AuthorID: postAuthor, WallOwnerID: wallOwner, CreatedAt: createdAt}
		f.comments.EXPECT().ByID(mock.Anything, int64(1100)).Return(c, nil).Once()
		f.posts.EXPECT().ByID(mock.Anything, int64(10)).Return(post, nil).Once()
		f.comments.EXPECT().DistinctAuthorsByPost(mock.Anything, int64(10), author).Return([]int64{prior}, nil).Once()

		noBan(f, postAuthor, author)
		f.authz.EXPECT().CanView(mock.Anything, postAuthor, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == postAuthor && !r.Hidden
		})).Return(nil).Once()

		noBan(f, wallOwner, author)
		f.authz.EXPECT().CanView(mock.Anything, wallOwner, wallOwner).Return(true, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == wallOwner && !r.Hidden
		})).Return(nil).Once()

		noBan(f, prior, author)
		f.authz.EXPECT().CanView(mock.Anything, prior, wallOwner).Return(false, nil).Once()
		f.feed.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(r newsfeed.FeedRow) bool {
			return r.UserID == prior && r.Hidden
		})).Return(nil).Once()

		err := svc.HandleCommentCreated(context.Background(), events.CommentCreated{CommentID: 1100})
		require.NoError(t, err)
	})
}

func TestService_HandleBanCreated(t *testing.T) {
	t.Parallel()
	svc, f := newFanoutService(t)

	f.feed.EXPECT().DeleteForBan(mock.Anything, int64(1), int64(2)).Return(nil).Once()

	err := svc.HandleBanCreated(context.Background(), events.BanCreated{BannerID: 1, BannedID: 2})
	require.NoError(t, err)
}

func TestService_HandleWallPrivacyChanged(t *testing.T) {
	t.Parallel()
	svc, f := newFanoutService(t)

	f.feed.EXPECT().DistinctViewersForOwner(mock.Anything, int64(10)).Return([]int64{20, 30}, nil).Once()
	f.authz.EXPECT().CanView(mock.Anything, int64(20), int64(10)).Return(true, nil).Once()
	f.authz.EXPECT().CanView(mock.Anything, int64(30), int64(10)).Return(false, nil).Once()
	f.feed.EXPECT().SetHiddenForOwner(mock.Anything, int64(10), []int64{30}, mock.Anything).Return(nil).Once()
	f.feed.EXPECT().SetVisibleForOwner(mock.Anything, int64(10), []int64{20}).Return(nil).Once()

	err := svc.HandleWallPrivacyChanged(context.Background(), events.WallPrivacyChanged{OwnerID: 10})
	require.NoError(t, err)
}
