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

// IsBanned reports whether ownerID has banned otherID (one-directional, used
// for UI hints like "you've banned this user"). For visibility decisions use
// the two-way check inside CanView/CanPost/CanComment.
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

// mutuallyBanned returns true if either side has banned the other.
func (a *Authorizer) mutuallyBanned(ctx context.Context, x, y int64) (bool, error) {
	if x == y {
		return false, nil
	}
	banned, err := a.bans.IsBanned(ctx, x, y)
	if err != nil {
		return false, fmt.Errorf("mutually banned: %w", err)
	}
	if banned {
		return true, nil
	}
	rev, err := a.bans.IsBanned(ctx, y, x)
	if err != nil {
		return false, fmt.Errorf("mutually banned reverse: %w", err)
	}
	return rev, nil
}

// CanView reports whether viewer can see ownerID's wall.
func (a *Authorizer) CanView(ctx context.Context, viewerID, ownerID int64) (bool, error) {
	if viewerID == ownerID {
		return true, nil
	}
	banned, err := a.mutuallyBanned(ctx, viewerID, ownerID)
	if err != nil {
		return false, fmt.Errorf("can view: %w", err)
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
	banned, err := a.mutuallyBanned(ctx, authorID, ownerID)
	if err != nil {
		return false, fmt.Errorf("can post: %w", err)
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

// CanComment reports whether author can leave a comment on a post on ownerID's wall.
func (a *Authorizer) CanComment(ctx context.Context, authorID, ownerID int64) (bool, error) {
	if authorID == ownerID {
		return true, nil
	}
	banned, err := a.mutuallyBanned(ctx, authorID, ownerID)
	if err != nil {
		return false, fmt.Errorf("can comment: %w", err)
	}
	if banned {
		return false, nil
	}
	p, err := a.privacy.Get(ctx, ownerID)
	if err != nil {
		return false, fmt.Errorf("can comment: get privacy: %w", err)
	}
	return a.scopeSatisfied(ctx, p.CommentScope, authorID, ownerID)
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
	case domain.ScopeNobody:
		return false, nil
	default:
		return false, fmt.Errorf("unknown wall scope: %q", scope)
	}
}
