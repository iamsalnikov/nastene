package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/session"
)

type CommentService interface {
	CreateComment(ctx context.Context, authorID, postID int64, body string) (domain.Comment, error)
	DeleteComment(ctx context.Context, actorID, commentID int64) error
	DeletePost(ctx context.Context, actorID, postID int64) error
}

type PostRefResolver interface {
	PostOwner(ctx context.Context, postID int64) (int64, error)
}

type Comment struct {
	Service CommentService
	Posts   PostRefResolver
}

func (h *Comment) PostComment(w http.ResponseWriter, r *http.Request) {
	postID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	body := r.PostFormValue("body")

	if _, err := h.Service.CreateComment(r.Context(), user.ID, postID, body); err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "нельзя комментировать", http.StatusForbidden)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "комментарий от 1 до 1000 символов", http.StatusBadRequest)
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, fmt.Sprintf("create comment: %v", err), http.StatusInternalServerError)
		}
		return
	}
	redirectToPostWall(w, r, h.Posts, postID)
}

func (h *Comment) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := h.Service.DeleteComment(r.Context(), user.ID, commentID); err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "нельзя удалить", http.StatusForbidden)
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, fmt.Sprintf("delete comment: %v", err), http.StatusInternalServerError)
		}
		return
	}
	redirectBackOrHome(w, r, user.ID)
}

func (h *Comment) DeletePost(w http.ResponseWriter, r *http.Request) {
	postID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ownerID, err := h.Posts.PostOwner(r.Context(), postID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		http.Error(w, fmt.Sprintf("resolve post: %v", err), http.StatusInternalServerError)
		return
	}

	if err := h.Service.DeletePost(r.Context(), user.ID, postID); err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "нельзя удалить", http.StatusForbidden)
		case errors.Is(err, domain.ErrNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, fmt.Sprintf("delete post: %v", err), http.StatusInternalServerError)
		}
		return
	}
	if ownerID == 0 {
		ownerID = user.ID
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", ownerID), http.StatusFound)
}

func parseInt64Path(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func redirectToPostWall(w http.ResponseWriter, r *http.Request, posts PostRefResolver, postID int64) {
	ownerID, err := posts.PostOwner(r.Context(), postID)
	if err != nil || ownerID == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", ownerID), http.StatusFound)
}

func redirectBackOrHome(w http.ResponseWriter, r *http.Request, userID int64) {
	ref := r.Referer()
	if ref != "" {
		http.Redirect(w, r, ref, http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", userID), http.StatusFound)
}
