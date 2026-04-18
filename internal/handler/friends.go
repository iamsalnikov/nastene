package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/service/friends"
	"github.com/iamsalnikov/nastene/internal/session"
)

type FriendsService interface {
	SendRequest(ctx context.Context, fromID, toID int64) error
	Accept(ctx context.Context, toID, fromID int64) error
	CancelRequest(ctx context.Context, fromID, toID int64) error
	RejectRequest(ctx context.Context, toID, fromID int64) error
	Remove(ctx context.Context, a, b int64) error
	Overview(ctx context.Context, userID int64) (*friends.Overview, error)
}

type Friends struct {
	Service  FriendsService
	Renderer *render.Renderer
}

func (h *Friends) GetOverview(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	ov, err := h.Service.Overview(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("friends overview: %v", err), http.StatusInternalServerError)
		return
	}
	if ov.InviteToken != "" {
		ov.InviteURL = requestBaseURL(r) + "/register?invite=" + ov.InviteToken
	}
	h.Renderer.Page(w, r, "friends", render.PageData{Title: "Друзья", Data: ov})
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func (h *Friends) Request(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(user domain.User, targetID int64) error {
		return h.Service.SendRequest(r.Context(), user.ID, targetID)
	})
}

func (h *Friends) Accept(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(user domain.User, targetID int64) error {
		return h.Service.Accept(r.Context(), user.ID, targetID)
	})
}

func (h *Friends) Reject(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(user domain.User, targetID int64) error {
		return h.Service.RejectRequest(r.Context(), user.ID, targetID)
	})
}

func (h *Friends) Cancel(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(user domain.User, targetID int64) error {
		return h.Service.CancelRequest(r.Context(), user.ID, targetID)
	})
}

func (h *Friends) Remove(w http.ResponseWriter, r *http.Request) {
	h.act(w, r, func(user domain.User, targetID int64) error {
		return h.Service.Remove(r.Context(), user.ID, targetID)
	})
}

func (h *Friends) act(w http.ResponseWriter, r *http.Request, do func(user domain.User, targetID int64) error) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	targetID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := do(user, targetID); err != nil {
		switch {
		case errors.Is(err, domain.ErrSelfAction):
			http.Error(w, "с собой нельзя", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, fmt.Sprintf("friends: %v", err), http.StatusInternalServerError)
		}
		return
	}
	ref := r.Referer()
	if ref == "" {
		ref = "/friends"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}
