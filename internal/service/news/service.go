package news

import (
	"context"
	"fmt"
	"time"

	"github.com/iamsalnikov/nastene/internal/domain"
)

const excerptRunes = 240

// FeedRepo reads a page of materialized feed rows for a user.
type FeedRepo interface {
	ListFeed(ctx context.Context, userID int64, limit, offset int) ([]domain.NewsEvent, error)
}

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
}

// Authorizer is kept as a safety net: ban/friendship state may have changed
// after fan-out, and we want to hide events the viewer can no longer see.
type Authorizer interface {
	CanView(ctx context.Context, viewerID, ownerID int64) (bool, error)
}

// AvatarAuthorizer решает, видит ли viewer аватар конкретного пользователя
// (приватность профиля BasicScope + взаимные баны).
type AvatarAuthorizer interface {
	CanSeeAvatar(ctx context.Context, viewerID, ownerID int64) (bool, error)
}

type Service struct {
	feed       FeedRepo
	users      UserRepo
	authorizer Authorizer
	avatars    AvatarAuthorizer
}

func NewService(feed FeedRepo, users UserRepo, authorizer Authorizer, avatars AvatarAuthorizer) *Service {
	return &Service{feed: feed, users: users, authorizer: authorizer, avatars: avatars}
}

type FeedView struct {
	Events   []EventView
	HasPrev  bool
	PrevPage int
	NextPage int
}

type EventView struct {
	Kind         domain.NewsEventKind
	CreatedAt    time.Time
	Author       domain.User
	Owner        domain.User
	PostID       int64
	CommentID    int64
	PostKind     domain.PostKind
	Excerpt      string
	GraffitiPath string
}

func (s *Service) LoadFeed(ctx context.Context, viewerID int64, limit, offset int) (*FeedView, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	raw, err := s.feed.ListFeed(ctx, viewerID, limit+1, offset)
	if err != nil {
		return nil, fmt.Errorf("load feed: %w", err)
	}

	hasNext := len(raw) > limit
	if hasNext {
		raw = raw[:limit]
	}

	canView := make(map[int64]bool)
	for _, ev := range raw {
		if _, ok := canView[ev.WallOwnerID]; ok {
			continue
		}
		ok, err := s.authorizer.CanView(ctx, viewerID, ev.WallOwnerID)
		if err != nil {
			return nil, fmt.Errorf("load feed: can view %d: %w", ev.WallOwnerID, err)
		}
		canView[ev.WallOwnerID] = ok
	}

	users := make(map[int64]domain.User)
	resolve := func(id int64) (domain.User, error) {
		if u, ok := users[id]; ok {
			return u, nil
		}
		u, err := s.users.ByID(ctx, id)
		if err != nil {
			return domain.User{}, err
		}
		users[id] = u
		return u, nil
	}

	avatarCache := map[int64]bool{}
	mask := func(u domain.User) (domain.User, error) {
		if u.AvatarPath == "" {
			return u, nil
		}
		canSee, ok := avatarCache[u.ID]
		if !ok {
			v, err := s.avatars.CanSeeAvatar(ctx, viewerID, u.ID)
			if err != nil {
				return domain.User{}, fmt.Errorf("can see avatar %d: %w", u.ID, err)
			}
			avatarCache[u.ID] = v
			canSee = v
		}
		if !canSee {
			u.AvatarPath = ""
		}
		return u, nil
	}

	views := make([]EventView, 0, len(raw))
	for _, ev := range raw {
		if !canView[ev.WallOwnerID] {
			continue
		}
		author, err := resolve(ev.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("load feed: author %d: %w", ev.AuthorID, err)
		}
		author, err = mask(author)
		if err != nil {
			return nil, fmt.Errorf("load feed: mask author %d: %w", ev.AuthorID, err)
		}
		owner, err := resolve(ev.WallOwnerID)
		if err != nil {
			return nil, fmt.Errorf("load feed: owner %d: %w", ev.WallOwnerID, err)
		}
		owner, err = mask(owner)
		if err != nil {
			return nil, fmt.Errorf("load feed: mask owner %d: %w", ev.WallOwnerID, err)
		}
		views = append(views, EventView{
			Kind:         ev.Kind,
			CreatedAt:    ev.CreatedAt,
			Author:       author,
			Owner:        owner,
			PostID:       ev.PostID,
			CommentID:    ev.CommentID,
			PostKind:     ev.PostKind,
			Excerpt:      truncate(ev.BodyText, excerptRunes),
			GraffitiPath: ev.GraffitiPath,
		})
	}

	nextPage := 0
	if hasNext {
		nextPage = offset + len(raw)
	}
	return &FeedView{
		Events:   views,
		HasPrev:  offset > 0,
		PrevPage: max(offset-limit, 0),
		NextPage: nextPage,
	}, nil
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
