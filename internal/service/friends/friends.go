package friends

import (
	"context"
	"errors"
	"fmt"

	"github.com/iamsalnikov/nastene/internal/domain"
)

type Repo interface {
	AreFriends(ctx context.Context, a, b int64) (bool, error)
	AddFriendship(ctx context.Context, a, b int64) error
	RemoveFriendship(ctx context.Context, a, b int64) error
	AddRequest(ctx context.Context, fromID, toID int64) error
	RemoveRequest(ctx context.Context, fromID, toID int64) error
	HasRequest(ctx context.Context, fromID, toID int64) (bool, error)
	ListIncomingRequests(ctx context.Context, toID int64) ([]domain.FriendRequest, error)
	ListOutgoingRequests(ctx context.Context, fromID int64) ([]domain.FriendRequest, error)
	ListFriendIDs(ctx context.Context, userID int64) ([]int64, error)
}

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
}

type Service struct {
	repo  Repo
	users UserRepo
	bans  BanCheck
}

func NewService(repo Repo, users UserRepo) *Service {
	return &Service{repo: repo, users: users}
}

type BanCheck interface {
	IsBanned(ctx context.Context, ownerID, otherID int64) (bool, error)
}

// SetBanCheck wires the ban repo so SendRequest rejects banned users.
func (s *Service) SetBanCheck(b BanCheck) { s.bans = b }

// SendRequest — fromID шлёт заявку в друзья к toID. Если toID уже прислал заявку — сразу дружба.
func (s *Service) SendRequest(ctx context.Context, fromID, toID int64) error {
	if fromID == toID {
		return fmt.Errorf("send request: %w", domain.ErrSelfAction)
	}
	if s.bans != nil {
		banned, err := s.bans.IsBanned(ctx, toID, fromID)
		if err != nil {
			return fmt.Errorf("send request: ban check: %w", err)
		}
		if banned {
			return fmt.Errorf("send request: %w", domain.ErrForbidden)
		}
	}
	friends, err := s.repo.AreFriends(ctx, fromID, toID)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	if friends {
		return nil
	}
	// если обратная заявка уже есть — сразу дружим
	reverse, err := s.repo.HasRequest(ctx, toID, fromID)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	if reverse {
		if err := s.repo.AddFriendship(ctx, fromID, toID); err != nil {
			return fmt.Errorf("send request: accept reverse: %w", err)
		}
		if err := s.repo.RemoveRequest(ctx, toID, fromID); err != nil {
			return fmt.Errorf("send request: remove reverse: %w", err)
		}
		return nil
	}
	if err := s.repo.AddRequest(ctx, fromID, toID); err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	return nil
}

// Accept — toID принимает заявку от fromID.
func (s *Service) Accept(ctx context.Context, toID, fromID int64) error {
	has, err := s.repo.HasRequest(ctx, fromID, toID)
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	if !has {
		return fmt.Errorf("accept: %w", domain.ErrNotFound)
	}
	if err := s.repo.AddFriendship(ctx, fromID, toID); err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	if err := s.repo.RemoveRequest(ctx, fromID, toID); err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	return nil
}

// CancelRequest — fromID отзывает свою заявку.
func (s *Service) CancelRequest(ctx context.Context, fromID, toID int64) error {
	if err := s.repo.RemoveRequest(ctx, fromID, toID); err != nil {
		return fmt.Errorf("cancel request: %w", err)
	}
	return nil
}

// RejectRequest — toID отклоняет входящую заявку от fromID.
func (s *Service) RejectRequest(ctx context.Context, toID, fromID int64) error {
	if err := s.repo.RemoveRequest(ctx, fromID, toID); err != nil {
		return fmt.Errorf("reject request: %w", err)
	}
	return nil
}

// Remove — удалить из друзей.
func (s *Service) Remove(ctx context.Context, a, b int64) error {
	if err := s.repo.RemoveFriendship(ctx, a, b); err != nil {
		return fmt.Errorf("remove friend: %w", err)
	}
	return nil
}

type OverviewPerson struct {
	User domain.User
}

type Overview struct {
	Friends  []OverviewPerson
	Incoming []OverviewPerson
	Outgoing []OverviewPerson

	InvitesRemaining int
	InviteToken      string
	InviteURL        string
}

// Overview — для страницы /friends.
func (s *Service) Overview(ctx context.Context, userID int64) (*Overview, error) {
	friendIDs, err := s.repo.ListFriendIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("overview friends: %w", err)
	}
	incoming, err := s.repo.ListIncomingRequests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("overview incoming: %w", err)
	}
	outgoing, err := s.repo.ListOutgoingRequests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("overview outgoing: %w", err)
	}

	cache := map[int64]domain.User{}
	resolve := func(id int64) (domain.User, error) {
		if u, ok := cache[id]; ok {
			return u, nil
		}
		u, err := s.users.ByID(ctx, id)
		if err != nil {
			return domain.User{}, err
		}
		cache[id] = u
		return u, nil
	}

	self, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("overview self: %w", err)
	}
	cache[self.ID] = self

	out := &Overview{
		Friends:          make([]OverviewPerson, 0, len(friendIDs)),
		Incoming:         make([]OverviewPerson, 0, len(incoming)),
		Outgoing:         make([]OverviewPerson, 0, len(outgoing)),
		InvitesRemaining: self.InvitesRemaining,
		InviteToken:      self.InviteToken,
	}
	for _, id := range friendIDs {
		u, err := resolve(id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("overview resolve friend: %w", err)
		}
		out.Friends = append(out.Friends, OverviewPerson{User: u})
	}
	for _, req := range incoming {
		u, err := resolve(req.FromID)
		if err != nil {
			continue
		}
		out.Incoming = append(out.Incoming, OverviewPerson{User: u})
	}
	for _, req := range outgoing {
		u, err := resolve(req.ToID)
		if err != nil {
			continue
		}
		out.Outgoing = append(out.Outgoing, OverviewPerson{User: u})
	}
	return out, nil
}
