package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/session"
)

type PrivacyService interface {
	Get(ctx context.Context, userID int64) (domain.WallPrivacy, error)
	Update(ctx context.Context, userID int64, view, post, comment domain.WallScope) error
}

type BanService interface {
	Add(ctx context.Context, ownerID, bannedID int64) error
	Remove(ctx context.Context, ownerID, bannedID int64) error
	ListByOwner(ctx context.Context, ownerID int64) ([]domain.Ban, error)
}

type UserLookup interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
}

type Settings struct {
	Privacy  PrivacyService
	Bans     BanService
	Users    UserLookup
	Renderer *render.Renderer
}

type privacyForm struct {
	ViewScope    domain.WallScope
	PostScope    domain.WallScope
	CommentScope domain.WallScope
}

func (h *Settings) GetPrivacy(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	p, err := h.Privacy.Get(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("privacy: %v", err), http.StatusInternalServerError)
		return
	}
	h.Renderer.Page(w, r, "privacy", render.PageData{
		Title: "Приватность",
		Data:  privacyForm{ViewScope: p.ViewScope, PostScope: p.PostScope, CommentScope: p.CommentScope},
	})
}

func (h *Settings) PostPrivacy(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	view := domain.WallScope(r.PostFormValue("view_scope"))
	post := domain.WallScope(r.PostFormValue("post_scope"))
	comment := domain.WallScope(r.PostFormValue("comment_scope"))
	if view == domain.ScopeNobody || !view.Valid() || !post.Valid() || !comment.Valid() {
		http.Error(w, "invalid scope", http.StatusBadRequest)
		return
	}
	if err := h.Privacy.Update(r.Context(), user.ID, view, post, comment); err != nil {
		http.Error(w, fmt.Sprintf("update privacy: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/privacy/wall", http.StatusFound)
}

type banRow struct {
	BannedID  int64
	User      domain.User
	CreatedAt string
}

func (h *Settings) GetBans(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	bans, err := h.Bans.ListByOwner(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("bans: %v", err), http.StatusInternalServerError)
		return
	}
	rows := make([]banRow, 0, len(bans))
	for _, b := range bans {
		u, err := h.Users.ByID(r.Context(), b.BannedID)
		if err != nil {
			continue
		}
		rows = append(rows, banRow{BannedID: b.BannedID, User: u, CreatedAt: b.CreatedAt.Format("2 Jan 2006")})
	}
	h.Renderer.Page(w, r, "bans", render.PageData{Title: "Бан-лист", Data: rows})
}

func (h *Settings) PostBanRemove(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Bans.Remove(r.Context(), user.ID, targetID); err != nil {
		http.Error(w, fmt.Sprintf("ban remove: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/bans", http.StatusFound)
}

// PostBanByID — бан с страницы /id{id}, возвращает на referer.
func (h *Settings) PostBanByID(w http.ResponseWriter, r *http.Request) {
	h.banAction(w, r, h.Bans.Add, "ban add by id")
}

// PostUnbanByID — разбан с страницы /id{id}, возвращает на referer.
func (h *Settings) PostUnbanByID(w http.ResponseWriter, r *http.Request) {
	h.banAction(w, r, h.Bans.Remove, "ban remove by id")
}

func (h *Settings) banAction(w http.ResponseWriter, r *http.Request, op func(context.Context, int64, int64) error, label string) {
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
	if targetID == user.ID {
		http.Error(w, "себя нельзя", http.StatusBadRequest)
		return
	}
	if err := op(r.Context(), user.ID, targetID); err != nil {
		http.Error(w, fmt.Sprintf("%s: %v", label, err), http.StatusInternalServerError)
		return
	}
	ref := r.Referer()
	if ref == "" {
		ref = fmt.Sprintf("/id%d", targetID)
	}
	http.Redirect(w, r, ref, http.StatusFound)
}
