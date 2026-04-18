package domain

import "time"

type NewsEventKind string

const (
	NewsEventPost    NewsEventKind = "post"
	NewsEventComment NewsEventKind = "comment"
)

type NewsEvent struct {
	Kind         NewsEventKind
	EventID      int64
	CreatedAt    time.Time
	WallOwnerID  int64
	AuthorID     int64
	PostID       int64
	CommentID    int64
	PostKind     PostKind
	BodyText     string
	GraffitiPath string
}
