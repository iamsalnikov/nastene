package domain

import "time"

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time

	Gender     string
	BirthDate  *time.Time
	City       string
	Website    string
	Activity   string
	Quote      string
	Bio        string
	AvatarPath string
	LastSeenAt *time.Time

	InvitesRemaining int
	InviteToken      string
	InvitedByUserID  *int64
}
