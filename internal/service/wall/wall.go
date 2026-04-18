package wall

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/events"
	"github.com/iamsalnikov/nastene/pkg/q"
)

const (
	maxTextLen = 4000
	minTextLen = 1
)

type PostRepo interface {
	Create(ctx context.Context, p domain.WallPost) (domain.WallPost, error)
	ByID(ctx context.Context, id int64) (domain.WallPost, error)
	ListByWall(ctx context.Context, ownerID int64, limit, offset int) ([]domain.WallPost, error)
	CountByAuthorSince(ctx context.Context, authorID int64, since time.Time) (int, error)
	Delete(ctx context.Context, id int64) error
}

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
}

// AvatarAuthorizer решает, видит ли viewer аватар конкретного пользователя.
// Аватар прячется правилами приватности профиля (BasicScope) и взаимной невидимостью банов.
type AvatarAuthorizer interface {
	CanSeeAvatar(ctx context.Context, viewerID, ownerID int64) (bool, error)
}

type Service struct {
	posts           PostRepo
	users           UserRepo
	comments        CommentRepo
	friendship      FriendshipQuery
	authorizer      *Authorizer
	avatars         AvatarAuthorizer
	publisher       events.Publisher
	log             *slog.Logger
	postsPerHour    int
	commentsPerHour int
	now             func() time.Time
}

func NewService(posts PostRepo, users UserRepo, comments CommentRepo, friendship FriendshipQuery, authorizer *Authorizer, avatars AvatarAuthorizer) *Service {
	return &Service{posts: posts, users: users, comments: comments, friendship: friendship, authorizer: authorizer, avatars: avatars, log: slog.Default(), now: time.Now}
}

// SetPublisher wires the event publisher. Publishing is best-effort: we log
// but don't fail the user's request if the message broker is unreachable.
func (s *Service) SetPublisher(p events.Publisher) { s.publisher = p }

// SetRateLimits sets per-author hourly caps for posts and comments.
// Zero or negative disables the corresponding limit.
func (s *Service) SetRateLimits(postsPerHour, commentsPerHour int) {
	s.postsPerHour = postsPerHour
	s.commentsPerHour = commentsPerHour
}

func (s *Service) checkPostLimit(ctx context.Context, authorID int64) error {
	if s.postsPerHour <= 0 {
		return nil
	}
	since := s.now().Add(-time.Hour)
	n, err := s.posts.CountByAuthorSince(ctx, authorID, since)
	if err != nil {
		return fmt.Errorf("count author posts: %w", err)
	}
	if n >= s.postsPerHour {
		return fmt.Errorf("posts per hour limit %d reached: %w", s.postsPerHour, domain.ErrRateLimited)
	}
	return nil
}

func (s *Service) checkCommentLimit(ctx context.Context, authorID int64) error {
	if s.commentsPerHour <= 0 {
		return nil
	}
	since := s.now().Add(-time.Hour)
	n, err := s.comments.CountByAuthorSince(ctx, authorID, since)
	if err != nil {
		return fmt.Errorf("count author comments: %w", err)
	}
	if n >= s.commentsPerHour {
		return fmt.Errorf("comments per hour limit %d reached: %w", s.commentsPerHour, domain.ErrRateLimited)
	}
	return nil
}

func (s *Service) CreateTextPost(ctx context.Context, authorID, ownerID int64, body string) (domain.WallPost, error) {
	body = strings.TrimSpace(body)
	l := utf8.RuneCountInString(body)
	if l < minTextLen || l > maxTextLen {
		return domain.WallPost{}, fmt.Errorf("create text post: length %d out of range: %w", l, domain.ErrInvalidInput)
	}

	ok, err := s.authorizer.CanPost(ctx, authorID, ownerID)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create text post: %w", err)
	}
	if !ok {
		return domain.WallPost{}, fmt.Errorf("create text post: %w", domain.ErrForbidden)
	}

	if err := s.checkPostLimit(ctx, authorID); err != nil {
		return domain.WallPost{}, fmt.Errorf("create text post: %w", err)
	}

	post, err := s.posts.Create(ctx, domain.WallPost{
		WallOwnerID: ownerID,
		AuthorID:    authorID,
		Kind:        domain.PostText,
		BodyText:    body,
	})
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create text post: %w", err)
	}

	s.publishWallPost(post.ID)
	return post, nil
}

func (s *Service) publishWallPost(postID int64) {
	if s.publisher == nil {
		return
	}
	if err := q.Publish(s.publisher, events.TopicWallPostCreated, events.WallPostCreated{PostID: postID}); err != nil {
		s.log.Warn("publish wall post created failed", "err", err, "post_id", postID)
	}
}

type FriendshipState string

const (
	FriendshipSelf            FriendshipState = "self"
	FriendshipNone            FriendshipState = "none"
	FriendshipRequestSent     FriendshipState = "request_sent"
	FriendshipRequestReceived FriendshipState = "request_received"
	FriendshipFriend          FriendshipState = "friend"
	FriendshipGuest           FriendshipState = "guest"
)

type FriendshipQuery interface {
	AreFriends(ctx context.Context, a, b int64) (bool, error)
	HasRequest(ctx context.Context, fromID, toID int64) (bool, error)
}

type WallView struct {
	Owner       domain.User
	Visible     bool
	Banned      bool // owner забанил viewer
	IBannedThem bool // viewer забанил owner
	CanPost     bool
	CanComment  bool
	Posts       []PostView
	HasPrev     bool
	PrevPage    int
	NextPage    int
	Friendship  FriendshipState
}

type PostView struct {
	Post     domain.WallPost
	Author   domain.User
	Comments []CommentView
}

type CommentView struct {
	Comment domain.Comment
	Author  domain.User
}

// LoadWall loads owner, visibility flag, friendship state, and a page of posts.
// If the viewer cannot see the wall, Visible=false and Posts is nil — the caller decides how to render.
// Only true errors (DB, missing owner) bubble up.
func (s *Service) LoadWall(ctx context.Context, viewerID, ownerID int64, limit, offset int) (*WallView, error) {
	owner, err := s.users.ByID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: owner: %w", err)
	}

	banned := false
	iBannedThem := false
	if viewerID != 0 && viewerID != ownerID {
		banned, err = s.authorizer.IsBanned(ctx, ownerID, viewerID)
		if err != nil {
			return nil, fmt.Errorf("load wall: banned: %w", err)
		}
		iBannedThem, err = s.authorizer.IsBanned(ctx, viewerID, ownerID)
		if err != nil {
			return nil, fmt.Errorf("load wall: i banned them: %w", err)
		}
	}

	fs, err := s.friendshipState(ctx, viewerID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: friendship: %w", err)
	}

	visible, err := s.authorizer.CanView(ctx, viewerID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: can view: %w", err)
	}
	if !visible {
		return &WallView{Owner: owner, Visible: false, Banned: banned, IBannedThem: iBannedThem, Friendship: fs}, nil
	}

	canPost, err := s.authorizer.CanPost(ctx, viewerID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: can post: %w", err)
	}

	canComment, err := s.authorizer.CanComment(ctx, viewerID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: can comment: %w", err)
	}

	posts, err := s.posts.ListByWall(ctx, ownerID, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("load wall: list posts: %w", err)
	}

	nextPage := 0
	if len(posts) > limit {
		posts = posts[:limit]
		nextPage = offset + limit
	}

	hasPrev := offset > 0
	prevPage := max(offset-limit, 0)

	postIDs := make([]int64, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	comments, err := s.comments.ListByPosts(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("load wall: comments: %w", err)
	}
	commentsByPost := make(map[int64][]domain.Comment, len(postIDs))
	for _, c := range comments {
		commentsByPost[c.PostID] = append(commentsByPost[c.PostID], c)
	}

	authors := map[int64]domain.User{ownerID: owner}
	resolveAuthor := func(id int64) (domain.User, error) {
		if u, ok := authors[id]; ok {
			return u, nil
		}
		u, err := s.users.ByID(ctx, id)
		if err != nil {
			return domain.User{}, err
		}
		authors[id] = u
		return u, nil
	}

	mask, err := s.avatarMasker(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("load wall: avatar masker: %w", err)
	}

	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		author, err := resolveAuthor(p.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("load wall: author %d: %w", p.AuthorID, err)
		}
		maskedAuthor, err := mask(author)
		if err != nil {
			return nil, fmt.Errorf("load wall: mask author %d: %w", p.AuthorID, err)
		}
		cvs := make([]CommentView, 0, len(commentsByPost[p.ID]))
		for _, c := range commentsByPost[p.ID] {
			ca, err := resolveAuthor(c.AuthorID)
			if err != nil {
				return nil, fmt.Errorf("load wall: comment author %d: %w", c.AuthorID, err)
			}
			maskedCA, err := mask(ca)
			if err != nil {
				return nil, fmt.Errorf("load wall: mask comment author %d: %w", c.AuthorID, err)
			}
			cvs = append(cvs, CommentView{Comment: c, Author: maskedCA})
		}
		views = append(views, PostView{Post: p, Author: maskedAuthor, Comments: cvs})
	}

	return &WallView{
		Owner:       owner,
		Visible:     true,
		Banned:      false,
		IBannedThem: iBannedThem,
		CanPost:     canPost,
		CanComment:  canComment,
		Posts:       views,
		HasPrev:     hasPrev,
		PrevPage:    prevPage,
		NextPage:    nextPage,
		Friendship:  fs,
	}, nil
}

// avatarMasker возвращает функцию, которая зануляет AvatarPath у пользователей,
// для которых viewer не вправе видеть аватар (BasicScope приватности профиля).
// Результаты кэшируются по userID, чтобы не спрашивать авторизатор повторно.
func (s *Service) avatarMasker(ctx context.Context, viewerID int64) (func(domain.User) (domain.User, error), error) {
	cache := map[int64]bool{}
	return func(u domain.User) (domain.User, error) {
		if u.AvatarPath == "" {
			return u, nil
		}
		canSee, ok := cache[u.ID]
		if !ok {
			v, err := s.avatars.CanSeeAvatar(ctx, viewerID, u.ID)
			if err != nil {
				return domain.User{}, fmt.Errorf("can see avatar: %w", err)
			}
			cache[u.ID] = v
			canSee = v
		}
		if !canSee {
			u.AvatarPath = ""
		}
		return u, nil
	}, nil
}

func (s *Service) friendshipState(ctx context.Context, viewerID, ownerID int64) (FriendshipState, error) {
	if viewerID == 0 {
		return FriendshipGuest, nil
	}
	if viewerID == ownerID {
		return FriendshipSelf, nil
	}
	friends, err := s.friendship.AreFriends(ctx, viewerID, ownerID)
	if err != nil {
		return "", err
	}
	if friends {
		return FriendshipFriend, nil
	}
	sent, err := s.friendship.HasRequest(ctx, viewerID, ownerID)
	if err != nil {
		return "", err
	}
	if sent {
		return FriendshipRequestSent, nil
	}
	received, err := s.friendship.HasRequest(ctx, ownerID, viewerID)
	if err != nil {
		return "", err
	}
	if received {
		return FriendshipRequestReceived, nil
	}
	return FriendshipNone, nil
}
