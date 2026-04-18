package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/service/profile"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	"github.com/iamsalnikov/nastene/internal/session"
)

type WallService interface {
	LoadWall(ctx context.Context, viewerID, ownerID int64, limit, offset int) (*wall.WallView, error)
	LoadPost(ctx context.Context, viewerID, postID int64) (*wall.PostDetail, error)
	CreateTextPost(ctx context.Context, authorID, ownerID int64, body string) (domain.WallPost, error)
}

type Wall struct {
	Service  WallService
	Profile  ProfileService
	Renderer *render.Renderer
}

const wallPageSize = 20

// wallPageData — связка стены и профиля владельца, передаётся в шаблон wall.html.
type wallPageData struct {
	Wall    *wall.WallView
	Profile profile.View
}

func (h *Wall) Get(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := parseOwnerID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	viewerID := viewerIDFromCtx(r.Context())

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	view, err := h.Service.LoadWall(r.Context(), viewerID, ownerID, wallPageSize, offset)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("load wall: %v", err), http.StatusInternalServerError)
		return
	}

	pview, err := h.Profile.Get(r.Context(), viewerID, ownerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("load profile: %v", err), http.StatusInternalServerError)
		return
	}

	h.Renderer.Page(w, r, "wall", render.PageData{
		Title: "Стена " + view.Owner.DisplayName,
		Data:  wallPageData{Wall: view, Profile: pview},
	})
}

func (h *Wall) PostText(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := parseOwnerID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, ok := session.FromContext(r.Context())
	if !ok {
		slog.Default().Warn("wall: post without session",
			"path", r.URL.Path,
			"cookies", len(r.Cookies()),
		)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	body := r.PostFormValue("body")

	_, err := h.Service.CreateTextPost(r.Context(), user.ID, ownerID, body)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "вам нельзя писать на этой стене", http.StatusForbidden)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "пост должен быть от 1 до 4000 символов", http.StatusBadRequest)
		case errors.Is(err, domain.ErrRateLimited):
			http.Error(w, "слишком часто: лимит постов в час исчерпан, попробуйте позже", http.StatusTooManyRequests)
		default:
			http.Error(w, fmt.Sprintf("create post: %v", err), http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/id%d", ownerID), http.StatusFound)
}

func (h *Wall) GetPost(w http.ResponseWriter, r *http.Request) {
	postID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	viewerID := viewerIDFromCtx(r.Context())

	detail, err := h.Service.LoadPost(r.Context(), viewerID, postID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "пост скрыт", http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("load post: %v", err), http.StatusInternalServerError)
		}
		return
	}

	h.Renderer.Page(w, r, "post", render.PageData{
		Title: "Пост",
		Data:  detail,
	})
}

func parseOwnerID(r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func viewerIDFromCtx(ctx context.Context) int64 {
	if u, ok := session.FromContext(ctx); ok {
		return u.ID
	}
	return 0
}
