package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	"github.com/iamsalnikov/nastene/internal/session"
)

type GraffitiStoreRef = wall.GraffitiStore

type GraffitiService interface {
	CreateGraffitiPost(ctx context.Context, authorID, ownerID int64, pngData io.Reader, store wall.GraffitiStore) (domain.WallPost, error)
}

type Graffiti struct {
	Service  GraffitiService
	Store    wall.GraffitiStore
	Users    UserLookup
	Renderer *render.Renderer
}

type graffitiPage struct {
	OwnerID     int64
	DisplayName string
}

func (h *Graffiti) GetEditor(w http.ResponseWriter, r *http.Request) {
	if _, ok := session.FromContext(r.Context()); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	ownerID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	owner, err := h.Users.ByID(r.Context(), ownerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, fmt.Sprintf("load owner: %v", err), http.StatusInternalServerError)
		return
	}
	h.Renderer.Page(w, r, "graffiti", render.PageData{
		Title: "Граффити",
		Data:  graffitiPage{OwnerID: owner.ID, DisplayName: owner.DisplayName},
	})
}

const maxGraffitiUpload = 2 << 20 // 2 MiB raw upload (base64 is ~1.33x PNG)

func (h *Graffiti) PostCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	ownerID, ok := parseInt64Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxGraffitiUpload)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "форма слишком большая", http.StatusBadRequest)
		return
	}
	dataURL := r.PostFormValue("image")
	if dataURL == "" {
		http.Error(w, "нет изображения", http.StatusBadRequest)
		return
	}

	pngReader, err := decodeDataURL(dataURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("decode image: %v", err), http.StatusBadRequest)
		return
	}

	if _, err := h.Service.CreateGraffitiPost(r.Context(), user.ID, ownerID, pngReader, h.Store); err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "вам нельзя писать на этой стене", http.StatusForbidden)
		case errors.Is(err, domain.ErrInvalidInput):
			http.Error(w, "картинка невалидна или слишком большая", http.StatusBadRequest)
		default:
			http.Error(w, fmt.Sprintf("create graffiti: %v", err), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/id%d", ownerID), http.StatusFound)
}

func decodeDataURL(v string) (io.Reader, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(v, prefix) {
		return nil, errors.New("expected data:image/png;base64, URL")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v, prefix))
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	return strings.NewReader(string(raw)), nil
}
