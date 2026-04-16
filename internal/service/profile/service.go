package profile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/repository"
)

const (
	maxAvatarBytes      = 1 << 20 // 1 MiB
	maxDisplayNameRunes = 64
	maxCityRunes        = 80
	maxWebsiteRunes     = 200
	maxActivityRunes    = 200
	maxQuoteRunes       = 1000
	maxBioRunes         = 2000
)

type UserRepo interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
	UpdateProfile(ctx context.Context, userID int64, p repository.ProfileFields) error
	UpdateAvatar(ctx context.Context, userID int64, path string) error
}

type ProfilePrivacyRepo interface {
	EnsureDefaults(ctx context.Context, userID int64) error
	Get(ctx context.Context, userID int64) (domain.ProfilePrivacy, error)
	Update(ctx context.Context, p domain.ProfilePrivacy) error
}

type FriendListRepo interface {
	ListFriendIDs(ctx context.Context, userID int64) ([]int64, error)
}

type AvatarStore interface {
	Save(ctx context.Context, relPath string, data io.Reader) error
}

type Service struct {
	users      UserRepo
	privacy    ProfilePrivacyRepo
	friends    FriendListRepo
	authorizer *Authorizer
	store      AvatarStore
}

func NewService(users UserRepo, privacy ProfilePrivacyRepo, friends FriendListRepo, authorizer *Authorizer, store AvatarStore) *Service {
	return &Service{users: users, privacy: privacy, friends: friends, authorizer: authorizer, store: store}
}

// View — то, что хендлер показывает на /id{n}: User с очищенными по правам полями + флаги видимости.
type View struct {
	User       domain.User
	Privacy    domain.ProfilePrivacy
	Visibility Visibility
	Friends    []domain.User
}

// Get загружает профиль и применяет маски: поля, которые viewer не вправе видеть, обнуляются.
func (s *Service) Get(ctx context.Context, viewerID, ownerID int64) (View, error) {
	owner, err := s.users.ByID(ctx, ownerID)
	if err != nil {
		return View{}, fmt.Errorf("profile get: owner: %w", err)
	}

	vis, err := s.authorizer.Visibility(ctx, viewerID, ownerID)
	if err != nil {
		return View{}, fmt.Errorf("profile get: visibility: %w", err)
	}

	priv, err := s.privacy.Get(ctx, ownerID)
	if err != nil {
		return View{}, fmt.Errorf("profile get: privacy: %w", err)
	}

	if vis.Banned {
		return View{
			User:       domain.User{ID: owner.ID, DisplayName: owner.DisplayName},
			Visibility: vis,
			Privacy:    priv,
		}, nil
	}

	masked := owner
	if !vis.Online {
		masked.LastSeenAt = nil
	}
	if !vis.Basic {
		masked.AvatarPath = ""
		masked.Gender = ""
		masked.BirthDate = nil
		masked.City = ""
	}
	if !vis.Bio {
		masked.Website = ""
		masked.Activity = ""
		masked.Quote = ""
		masked.Bio = ""
	}

	view := View{User: masked, Visibility: vis, Privacy: priv}

	if vis.FriendsList && s.friends != nil {
		ids, err := s.friends.ListFriendIDs(ctx, ownerID)
		if err != nil {
			return View{}, fmt.Errorf("profile get: friends: %w", err)
		}
		view.Friends = make([]domain.User, 0, len(ids))
		for _, id := range ids {
			u, err := s.users.ByID(ctx, id)
			if err != nil {
				continue
			}
			view.Friends = append(view.Friends, u)
		}
	}

	return view, nil
}

// UpdateInput — ввод формы /settings/profile (без аватара).
type UpdateInput struct {
	DisplayName string
	Gender      string
	BirthDate   *time.Time
	City        string
	Website     string
	Activity    string
	Quote       string
	Bio         string
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, in UpdateInput) error {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.City = strings.TrimSpace(in.City)
	in.Website = strings.TrimSpace(in.Website)
	in.Activity = strings.TrimSpace(in.Activity)
	in.Quote = strings.TrimSpace(in.Quote)
	in.Bio = strings.TrimSpace(in.Bio)

	if utf8.RuneCountInString(in.DisplayName) < 2 || utf8.RuneCountInString(in.DisplayName) > maxDisplayNameRunes {
		return fmt.Errorf("update profile: display name: %w", domain.ErrInvalidInput)
	}
	switch in.Gender {
	case "", "male", "female":
	default:
		return fmt.Errorf("update profile: gender %q: %w", in.Gender, domain.ErrInvalidInput)
	}
	if in.BirthDate != nil && in.BirthDate.After(time.Now()) {
		return fmt.Errorf("update profile: birth date in the future: %w", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.City) > maxCityRunes {
		return fmt.Errorf("update profile: city too long: %w", domain.ErrInvalidInput)
	}
	if in.Website != "" {
		if utf8.RuneCountInString(in.Website) > maxWebsiteRunes {
			return fmt.Errorf("update profile: website too long: %w", domain.ErrInvalidInput)
		}
		u, err := url.Parse(in.Website)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("update profile: website must be http(s) URL: %w", domain.ErrInvalidInput)
		}
	}
	if utf8.RuneCountInString(in.Activity) > maxActivityRunes {
		return fmt.Errorf("update profile: activity too long: %w", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Quote) > maxQuoteRunes {
		return fmt.Errorf("update profile: quote too long: %w", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Bio) > maxBioRunes {
		return fmt.Errorf("update profile: bio too long: %w", domain.ErrInvalidInput)
	}

	if err := s.users.UpdateProfile(ctx, userID, repository.ProfileFields{
		DisplayName: in.DisplayName,
		Gender:      in.Gender,
		BirthDate:   in.BirthDate,
		City:        in.City,
		Website:     in.Website,
		Activity:    in.Activity,
		Quote:       in.Quote,
		Bio:         in.Bio,
	}); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

// UpdateAvatar читает картинку (PNG/JPEG, ≤1 MiB), валидирует через image.Decode и сохраняет на диск.
// Возвращает относительный путь, который сохранён в users.avatar_path.
func (s *Service) UpdateAvatar(ctx context.Context, userID int64, file io.Reader) (string, error) {
	limited := io.LimitReader(file, maxAvatarBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("update avatar: read: %w", err)
	}
	if len(raw) > maxAvatarBytes {
		return "", fmt.Errorf("update avatar: too large: %w", domain.ErrInvalidInput)
	}

	_, format, err := image.Decode(bytesReader(raw))
	if err != nil {
		return "", fmt.Errorf("update avatar: decode: %w", domain.ErrInvalidInput)
	}
	ext := ""
	switch format {
	case "png":
		ext = "png"
	case "jpeg":
		ext = "jpg"
	default:
		return "", fmt.Errorf("update avatar: unsupported format %q: %w", format, domain.ErrInvalidInput)
	}

	suffix, err := randomSuffix()
	if err != nil {
		return "", fmt.Errorf("update avatar: name: %w", err)
	}
	rel := filepath.Join("avatars", fmt.Sprintf("%d-%s.%s", userID, suffix, ext))

	if err := s.store.Save(ctx, rel, bytesReader(raw)); err != nil {
		return "", fmt.Errorf("update avatar: save: %w", err)
	}
	if err := s.users.UpdateAvatar(ctx, userID, rel); err != nil {
		return "", fmt.Errorf("update avatar: db: %w", err)
	}
	return rel, nil
}

func (s *Service) GetPrivacy(ctx context.Context, userID int64) (domain.ProfilePrivacy, error) {
	p, err := s.privacy.Get(ctx, userID)
	if err != nil {
		return domain.ProfilePrivacy{}, fmt.Errorf("profile get privacy: %w", err)
	}
	return p, nil
}

func (s *Service) UpdatePrivacy(ctx context.Context, userID int64, p domain.ProfilePrivacy) error {
	if !p.OnlineScope.Valid() || !p.BasicScope.Valid() || !p.FriendsScope.Valid() || !p.BioScope.Valid() {
		return fmt.Errorf("update profile privacy: %w", domain.ErrInvalidInput)
	}
	p.UserID = userID
	if err := s.privacy.Update(ctx, p); err != nil {
		return fmt.Errorf("update profile privacy: %w", err)
	}
	return nil
}

func (s *Service) EnsureDefaults(ctx context.Context, userID int64) error {
	if err := s.privacy.EnsureDefaults(ctx, userID); err != nil {
		return fmt.Errorf("profile ensure defaults: %w", err)
	}
	return nil
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func randomSuffix() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}
