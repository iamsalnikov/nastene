package handler

import (
	"fmt"
	"net/http"

	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/session"
)

type Landing struct {
	Renderer *render.Renderer
}

func (h *Landing) Handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if u, ok := session.FromContext(r.Context()); ok {
		http.Redirect(w, r, fmt.Sprintf("/id%d", u.ID), http.StatusFound)
		return
	}
	h.Renderer.Page(w, r, "landing", render.PageData{Title: "Стена"})
}
