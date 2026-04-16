package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/session"
)

type AuthService interface {
	Register(ctx context.Context, email, password, displayName string) (domain.User, error)
	Login(ctx context.Context, email, password string) (domain.User, error)
}

type SessionManager interface {
	Issue(ctx context.Context, w http.ResponseWriter, userID int64) error
	Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error
}

type PrivacyInitializer interface {
	EnsureDefaults(ctx context.Context, userID int64) error
}

type Auth struct {
	Service        AuthService
	Sessions       SessionManager
	Privacy        PrivacyInitializer
	ProfilePrivacy PrivacyInitializer
	Renderer       *render.Renderer
}

func (h *Auth) GetRegister(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); ok {
		redirectHome(w, r)
		return
	}
	h.Renderer.Page(w, r, "register", render.PageData{Title: "Регистрация"})
}

func (h *Auth) PostRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := r.PostFormValue("email")
	password := r.PostFormValue("password")
	displayName := r.PostFormValue("display_name")

	user, err := h.Service.Register(r.Context(), email, password, displayName)
	if err != nil {
		h.Renderer.Page(w, r, "register", render.PageData{
			Title: "Регистрация",
			Error: registerError(err),
			Data: map[string]string{
				"Email":       email,
				"DisplayName": displayName,
			},
		})
		return
	}

	if h.Privacy != nil {
		if err := h.Privacy.EnsureDefaults(r.Context(), user.ID); err != nil {
			http.Error(w, fmt.Sprintf("init privacy: %v", err), http.StatusInternalServerError)
			return
		}
	}
	if h.ProfilePrivacy != nil {
		if err := h.ProfilePrivacy.EnsureDefaults(r.Context(), user.ID); err != nil {
			http.Error(w, fmt.Sprintf("init profile privacy: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if err := h.Sessions.Issue(r.Context(), w, user.ID); err != nil {
		http.Error(w, fmt.Sprintf("issue session: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", user.ID), http.StatusFound)
}

func (h *Auth) GetLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); ok {
		redirectHome(w, r)
		return
	}
	h.Renderer.Page(w, r, "login", render.PageData{Title: "Вход"})
}

func (h *Auth) PostLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := r.PostFormValue("email")
	password := r.PostFormValue("password")

	user, err := h.Service.Login(r.Context(), email, password)
	if err != nil {
		h.Renderer.Page(w, r, "login", render.PageData{
			Title: "Вход",
			Error: "Неверный e-mail или пароль",
			Data:  map[string]string{"Email": email},
		})
		return
	}

	if err := h.Sessions.Issue(r.Context(), w, user.ID); err != nil {
		http.Error(w, fmt.Sprintf("issue session: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", user.ID), http.StatusFound)
}

func (h *Auth) PostLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.Revoke(r.Context(), w, r); err != nil {
		http.Error(w, fmt.Sprintf("revoke session: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func redirectHome(w http.ResponseWriter, r *http.Request) {
	if u, ok := session.FromContext(r.Context()); ok {
		http.Redirect(w, r, fmt.Sprintf("/id%d", u.ID), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func registerError(err error) string {
	switch {
	case errors.Is(err, domain.ErrEmailAlreadyInUse):
		return "Этот e-mail уже занят"
	case errors.Is(err, domain.ErrInvalidInput):
		return "Проверьте поля: e-mail корректный, пароль ≥ 6 символов, имя ≥ 2 символов"
	default:
		return "Не удалось зарегистрироваться, попробуйте позже"
	}
}
