package domain

import "time"

type FriendRequest struct {
	FromID    int64
	ToID      int64
	CreatedAt time.Time
}

type Friendship struct {
	UserA     int64
	UserB     int64
	CreatedAt time.Time
}

// OrderedPair returns (min, max) so it can be stored as a Friendship row.
func OrderedPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}
