package domain

import "time"

type WallScope string

const (
	ScopePublic  WallScope = "public"
	ScopeFriends WallScope = "friends"
	ScopeNobody  WallScope = "nobody"
)

func (s WallScope) Valid() bool {
	return s == ScopePublic || s == ScopeFriends || s == ScopeNobody
}

type WallPrivacy struct {
	UserID       int64
	ViewScope    WallScope
	PostScope    WallScope
	CommentScope WallScope
	UpdatedAt    time.Time
}

type PostKind string

const (
	PostText     PostKind = "text"
	PostGraffiti PostKind = "graffiti"
)

type WallPost struct {
	ID            int64
	WallOwnerID   int64
	AuthorID      int64
	Kind          PostKind
	BodyText      string
	GraffitiPath  string
	CreatedAt     time.Time
}

type Ban struct {
	UserID    int64
	BannedID  int64
	CreatedAt time.Time
}

type Comment struct {
	ID        int64
	PostID    int64
	AuthorID  int64
	Body      string
	CreatedAt time.Time
}
