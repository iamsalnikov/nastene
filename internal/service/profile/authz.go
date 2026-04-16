package profile

import (
	"context"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type PrivacyRepo interface {
	Get(ctx context.Context, userID int64) (domain.ProfilePrivacy, error)
}

type FriendRepo interface {
	AreFriends(ctx context.Context, a, b int64) (bool, error)
}

type BanRepo interface {
	IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error)
}

// Authorizer — единая точка правды по приватности профиля и банам внутри домена профиля.
// Не дублируй его логику в хендлерах или других сервисах.
type Authorizer struct {
	privacy PrivacyRepo
	friends FriendRepo
	bans    BanRepo
}

func NewAuthorizer(privacy PrivacyRepo, friends FriendRepo, bans BanRepo) *Authorizer {
	return &Authorizer{privacy: privacy, friends: friends, bans: bans}
}

// Visibility — что viewer вправе увидеть на профиле owner.
// Banned=true означает, что viewer в бане у owner — все остальные флаги тогда false.
type Visibility struct {
	Self        bool
	Banned      bool
	Online      bool
	Basic       bool
	FriendsList bool
	Bio         bool
}

// Visibility — батч-расчёт всех 4 групп за один проход (1 IsBanned + 1 Get + ≤1 AreFriends).
func (a *Authorizer) Visibility(ctx context.Context, viewerID, ownerID int64) (Visibility, error) {
	if viewerID == ownerID && viewerID != 0 {
		return Visibility{Self: true, Online: true, Basic: true, FriendsList: true, Bio: true}, nil
	}

	if viewerID != 0 {
		banned, err := a.bans.IsBanned(ctx, ownerID, viewerID)
		if err != nil {
			return Visibility{}, fmt.Errorf("profile visibility: check ban: %w", err)
		}
		if banned {
			return Visibility{Banned: true}, nil
		}
	}

	p, err := a.privacy.Get(ctx, ownerID)
	if err != nil {
		return Visibility{}, fmt.Errorf("profile visibility: get privacy: %w", err)
	}

	friends, friendsResolved := false, false
	resolveFriends := func() (bool, error) {
		if friendsResolved || viewerID == 0 {
			return friends, nil
		}
		v, err := a.friends.AreFriends(ctx, viewerID, ownerID)
		if err != nil {
			return false, fmt.Errorf("profile visibility: are friends: %w", err)
		}
		friends = v
		friendsResolved = true
		return v, nil
	}

	scope := func(s domain.ProfileScope) (bool, error) {
		switch s {
		case domain.ProfileScopeEveryone:
			return true, nil
		case domain.ProfileScopeFriends:
			if viewerID == 0 {
				return false, nil
			}
			return resolveFriends()
		case domain.ProfileScopeNobody:
			return false, nil
		default:
			return false, fmt.Errorf("profile visibility: unknown scope %q", s)
		}
	}

	online, err := scope(p.OnlineScope)
	if err != nil {
		return Visibility{}, err
	}
	basic, err := scope(p.BasicScope)
	if err != nil {
		return Visibility{}, err
	}
	flist, err := scope(p.FriendsScope)
	if err != nil {
		return Visibility{}, err
	}
	bio, err := scope(p.BioScope)
	if err != nil {
		return Visibility{}, err
	}

	return Visibility{
		Online:      online,
		Basic:       basic,
		FriendsList: flist,
		Bio:         bio,
	}, nil
}

// CanSee — точечная проверка одного поля. Использует Visibility под капотом.
func (a *Authorizer) CanSee(ctx context.Context, viewerID, ownerID int64, f domain.ProfileField) (bool, error) {
	v, err := a.Visibility(ctx, viewerID, ownerID)
	if err != nil {
		return false, err
	}
	switch f {
	case domain.ProfileFieldOnline:
		return v.Online, nil
	case domain.ProfileFieldBasic:
		return v.Basic, nil
	case domain.ProfileFieldFriendsList:
		return v.FriendsList, nil
	case domain.ProfileFieldBio:
		return v.Bio, nil
	default:
		return false, fmt.Errorf("can see: unknown field %d", f)
	}
}
