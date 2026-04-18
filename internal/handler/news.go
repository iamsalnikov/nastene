package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/service/news"
	"github.com/iamsalnikov/nastene/internal/session"
)

type NewsService interface {
	LoadFeed(ctx context.Context, viewerID int64, limit, offset int) (*news.FeedView, error)
}

type News struct {
	Service  NewsService
	Renderer *render.Renderer
}

const newsPageSize = 20

func (h *News) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	view, err := h.Service.LoadFeed(r.Context(), user.ID, newsPageSize, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("load news: %v", err), http.StatusInternalServerError)
		return
	}

	h.Renderer.Page(w, r, "news", render.PageData{
		Title: "Новости",
		Data:  view,
	})
}
