package wall

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type PrivacyRepo interface {
	Get(ctx context.Context, userID int64) (domain.WallPrivacy, error)
}

type FriendRepo interface {
	AreFriends(ctx context.Context, a, b int64) (bool, error)
}

type BanRepo interface {
	IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error)
}

// Authorizer — единая точка правды по приватности и банам.
// Не дублируй его логику в хендлерах или других сервисах.
type Authorizer struct {
	privacy PrivacyRepo
	friends FriendRepo
	bans    BanRepo
}

func NewAuthorizer(privacy PrivacyRepo, friends FriendRepo, bans BanRepo) *Authorizer {
	return &Authorizer{privacy: privacy, friends: friends, bans: bans}
}

// IsBanned reports whether ownerID has banned otherID.
func (a *Authorizer) IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error) {
	if ownerID == otherID {
		return false, nil
	}
	banned, err := a.bans.IsBanned(ctx, ownerID, otherID)
	if err != nil {
		return false, fmt.Errorf("is banned: %w", err)
	}
	return banned, nil
}

// CanView reports whether viewer can see ownerID's wall.
func (a *Authorizer) CanView(ctx context.Context, viewerID, ownerID int64) (bool, error) {
	if viewerID == ownerID {
		return true, nil
	}
	banned, err := a.bans.IsBanned(ctx, ownerID, viewerID)
	if err != nil {
		return false, fmt.Errorf("can view: check ban: %w", err)
	}
	if banned {
		return false, nil
	}
	p, err := a.privacy.Get(ctx, ownerID)
	if err != nil {
		return false, fmt.Errorf("can view: get privacy: %w", err)
	}
	return a.scopeSatisfied(ctx, p.ViewScope, viewerID, ownerID)
}

// CanPost reports whether author can write a post on ownerID's wall.
func (a *Authorizer) CanPost(ctx context.Context, authorID, ownerID int64) (bool, error) {
	if authorID == ownerID {
		return true, nil
	}
	banned, err := a.bans.IsBanned(ctx, ownerID, authorID)
	if err != nil {
		return false, fmt.Errorf("can post: check ban: %w", err)
	}
	if banned {
		return false, nil
	}
	p, err := a.privacy.Get(ctx, ownerID)
	if err != nil {
		return false, fmt.Errorf("can post: get privacy: %w", err)
	}
	return a.scopeSatisfied(ctx, p.PostScope, authorID, ownerID)
}

// CanComment mirrors CanPost — если нельзя писать на стене, то и комментировать нельзя.
func (a *Authorizer) CanComment(ctx context.Context, authorID, ownerID int64) (bool, error) {
	return a.CanPost(ctx, authorID, ownerID)
}

func (a *Authorizer) scopeSatisfied(ctx context.Context, scope domain.WallScope, actorID, ownerID int64) (bool, error) {
	switch scope {
	case domain.ScopePublic:
		return true, nil
	case domain.ScopeFriends:
		friends, err := a.friends.AreFriends(ctx, actorID, ownerID)
		if err != nil {
			return false, fmt.Errorf("scope friends check: %w", err)
		}
		return friends, nil
	default:
		return false, fmt.Errorf("unknown wall scope: %q", scope)
	}
}
