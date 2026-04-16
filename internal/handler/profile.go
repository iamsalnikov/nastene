package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/service/profile"
	"github.com/iamsalnikov/nastene/internal/session"
)

type ProfileService interface {
	Get(ctx context.Context, viewerID, ownerID int64) (profile.View, error)
	UpdateProfile(ctx context.Context, userID int64, in profile.UpdateInput) error
	UpdateAvatar(ctx context.Context, userID int64, file io.Reader) (string, error)
	GetPrivacy(ctx context.Context, userID int64) (domain.ProfilePrivacy, error)
	UpdatePrivacy(ctx context.Context, userID int64, p domain.ProfilePrivacy) error
}

type Profile struct {
	Service  ProfileService
	Users    UserLookup
	Renderer *render.Renderer
}

const maxProfileFormBytes = 2 << 20 // 2 MiB — на форму с аватаром

type profileFormView struct {
	User      domain.User
	BirthDate string
}

func (h *Profile) GetSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	u, err := h.Users.ByID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("profile: %v", err), http.StatusInternalServerError)
		return
	}
	h.Renderer.Page(w, r, "profile_settings", render.PageData{
		Title: "Моя страница",
		Data:  profileFormView{User: u, BirthDate: formatBirthInput(u.BirthDate)},
	})
}

func (h *Profile) PostSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseMultipartForm(maxProfileFormBytes); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	bd, err := parseBirthDate(strings.TrimSpace(r.PostFormValue("birth_date")))
	if err != nil {
		http.Error(w, "дата рождения должна быть в формате ГГГГ-ММ-ДД", http.StatusBadRequest)
		return
	}

	in := profile.UpdateInput{
		DisplayName: r.PostFormValue("display_name"),
		Gender:      r.PostFormValue("gender"),
		BirthDate:   bd,
		City:        r.PostFormValue("city"),
		Website:     r.PostFormValue("website"),
		Activity:    r.PostFormValue("activity"),
		Quote:       r.PostFormValue("quote"),
		Bio:         r.PostFormValue("bio"),
	}
	if err := h.Service.UpdateProfile(r.Context(), user.ID, in); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, fmt.Sprintf("проверьте поля: %v", err), http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("update profile: %v", err), http.StatusInternalServerError)
		return
	}

	if file, _, ferr := r.FormFile("avatar"); ferr == nil {
		defer file.Close()
		if _, err := h.Service.UpdateAvatar(r.Context(), user.ID, file); err != nil {
			if errors.Is(err, domain.ErrInvalidInput) {
				http.Error(w, "аватар: PNG или JPEG до 1 MiB", http.StatusBadRequest)
				return
			}
			http.Error(w, fmt.Sprintf("update avatar: %v", err), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/settings/profile", http.StatusFound)
}

type profilePrivacyView struct {
	Privacy domain.ProfilePrivacy
}

func (h *Profile) GetPrivacy(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	p, err := h.Service.GetPrivacy(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("profile privacy: %v", err), http.StatusInternalServerError)
		return
	}
	h.Renderer.Page(w, r, "profile_privacy", render.PageData{
		Title: "Приватность профиля",
		Data:  profilePrivacyView{Privacy: p},
	})
}

func (h *Profile) PostPrivacy(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	p := domain.ProfilePrivacy{
		OnlineScope:  domain.ProfileScope(r.PostFormValue("online_scope")),
		BasicScope:   domain.ProfileScope(r.PostFormValue("basic_scope")),
		FriendsScope: domain.ProfileScope(r.PostFormValue("friends_scope")),
		BioScope:     domain.ProfileScope(r.PostFormValue("bio_scope")),
	}
	if err := h.Service.UpdatePrivacy(r.Context(), user.ID, p); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, "неизвестный уровень видимости", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("update profile privacy: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/privacy/profile", http.StatusFound)
}

type userFriendsView struct {
	Owner   domain.User
	Friends []domain.User
}

// GetUserFriends — страница друзей конкретного пользователя /id{id}/friends.
// Список показываем только если приватность профиля разрешает viewer'у его видеть.
func (h *Profile) GetUserFriends(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	viewerID := int64(0)
	if u, ok := session.FromContext(r.Context()); ok {
		viewerID = u.ID
	}
	pview, err := h.Service.Get(r.Context(), viewerID, ownerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("profile: %v", err), http.StatusInternalServerError)
		return
	}
	if !pview.Visibility.FriendsList {
		h.Renderer.Page(w, r, "friends_of", render.PageData{
			Title: "Друзья " + pview.User.DisplayName,
			Data:  userFriendsView{Owner: pview.User},
		})
		return
	}
	h.Renderer.Page(w, r, "friends_of", render.PageData{
		Title: "Друзья " + pview.User.DisplayName,
		Data:  userFriendsView{Owner: pview.User, Friends: pview.Friends},
	})
}

// SettingsHub — общая страница настроек со ссылками на разделы.
type SettingsHub struct {
	Renderer *render.Renderer
}

func (h *SettingsHub) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	h.Renderer.Page(w, r, "settings", render.PageData{Title: "Настройки"})
}

func parseBirthDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("parse birth date: %w", err)
	}
	return &t, nil
}

func formatBirthInput(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
