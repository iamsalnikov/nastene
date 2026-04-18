// Package events defines typed event payloads and topic names published via
// RabbitMQ (see pkg/q and pkg/rabbit). Producers publish with q.Publish;
// consumers subscribe with q.Subscribe.
package events

import "github.com/ThreeDotsLabs/watermill/message"

// Topic names published in the "application" RabbitMQ topic exchange.
const (
	TopicWallPostCreated    = "wall.post.created"
	TopicCommentCreated     = "wall.comment.created"
	TopicWallPrivacyChanged = "wall.privacy.changed"
	TopicBanCreated         = "wall.ban.created"
)

// WallPostCreated is published when a wall post is successfully created.
// The worker loads the post by ID and fans out to recipients.
type WallPostCreated struct {
	PostID int64 `json:"post_id"`
}

// CommentCreated is published when a comment is successfully created.
type CommentCreated struct {
	CommentID int64 `json:"comment_id"`
}

// WallPrivacyChanged is published after a user changes their wall privacy.
type WallPrivacyChanged struct {
	OwnerID int64 `json:"owner_id"`
}

// BanCreated is published after BannerID bans BannedID. The worker uses it to
// purge feed rows involving the opposite party in both directions.
type BanCreated struct {
	BannerID int64 `json:"banner_id"`
	BannedID int64 `json:"banned_id"`
}

// Publisher is a thin alias for Watermill's message.Publisher — kept here so
// that mockery can generate a mock targeting our own package.
type Publisher interface {
	Publish(topic string, messages ...*message.Message) error
}
