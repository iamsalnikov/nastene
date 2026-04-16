package domain

import "time"

type ProfileScope string

const (
	ProfileScopeEveryone ProfileScope = "everyone"
	ProfileScopeFriends  ProfileScope = "friends"
	ProfileScopeNobody   ProfileScope = "nobody"
)

func (s ProfileScope) Valid() bool {
	switch s {
	case ProfileScopeEveryone, ProfileScopeFriends, ProfileScopeNobody:
		return true
	}
	return false
}

type ProfileField int

const (
	ProfileFieldOnline ProfileField = iota
	ProfileFieldBasic
	ProfileFieldFriendsList
	ProfileFieldBio
)

type ProfilePrivacy struct {
	UserID        int64
	OnlineScope   ProfileScope
	BasicScope    ProfileScope
	FriendsScope  ProfileScope
	BioScope      ProfileScope
	UpdatedAt     time.Time
}

func DefaultProfilePrivacy(userID int64) ProfilePrivacy {
	return ProfilePrivacy{
		UserID:       userID,
		OnlineScope:  ProfileScopeFriends,
		BasicScope:   ProfileScopeFriends,
		FriendsScope: ProfileScopeFriends,
		BioScope:     ProfileScopeFriends,
	}
}
