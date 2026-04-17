package wall

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/iamsalnikov/nastene/internal/domain"
)

const (
	maxTextLen = 4000
	minTextLen = 1
)

type PostRepo interface {
	Create(ctx context.Context, p domain.WallPost) (domain.WallPost, error)
	ByID(ctx context.Context, id int64) (domain.WallPost, error)
	ListByWall(ctx context.Context, ownerID int64, limit, offset int) ([]domain.WallPost, error)
	Delete(ctx context.Context, id int64) error
}

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
}

type Service struct {
	posts      PostRepo
	users      UserRepo
	comments   CommentRepo
	friendship FriendshipQuery
	authorizer *Authorizer
}

func NewService(posts PostRepo, users UserRepo, comments CommentRepo, friendship FriendshipQuery, authorizer *Authorizer) *Service {
	return &Service{posts: posts, users: users, comments: comments, friendship: friendship, authorizer: authorizer}
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

	post, err := s.posts.Create(ctx, domain.WallPost{
		WallOwnerID: ownerID,
		AuthorID:    authorID,
		Kind:        domain.PostText,
		BodyText:    body,
	})
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create text post: %w", err)
	}
	return post, nil
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

	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		author, err := resolveAuthor(p.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("load wall: author %d: %w", p.AuthorID, err)
		}
		cvs := make([]CommentView, 0, len(commentsByPost[p.ID]))
		for _, c := range commentsByPost[p.ID] {
			ca, err := resolveAuthor(c.AuthorID)
			if err != nil {
				return nil, fmt.Errorf("load wall: comment author %d: %w", c.AuthorID, err)
			}
			cvs = append(cvs, CommentView{Comment: c, Author: ca})
		}
		views = append(views, PostView{Post: p, Author: author, Comments: cvs})
	}

	return &WallView{
		Owner:       owner,
		Visible:     true,
		Banned:      false,
		IBannedThem: iBannedThem,
		CanPost:     canPost,
		CanComment:  canComment,
		Posts:       views,
		NextPage:    nextPage,
		Friendship:  fs,
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
