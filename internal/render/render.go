package render

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/iamsalnikov/nastene/internal/domain"
	"github.com/iamsalnikov/nastene/internal/session"
)

// URLPresigner отдаёт ссылку для GET-доступа к объекту в S3-хранилище.
// Подписи в minio-go локальные, без сетевых вызовов — безопасно дёргать на каждый рендер.
type URLPresigner interface {
	PresignURL(ctx context.Context, key string) (string, error)
}

type Renderer struct {
	templates map[string]*template.Template
}

type PageData struct {
	Title            string
	User             *domain.User
	CSRFToken        string
	Flash            string
	Error            string
	IncomingRequests int
	Data             any
}

func New(fsys fs.FS, presigner URLPresigner) (*Renderer, error) {
	avatarURL := func(p string) string {
		if p == "" {
			return defaultAvatarURL
		}
		u, err := presigner.PresignURL(context.Background(), p)
		if err != nil {
			slog.Default().Error("presign avatar", "key", p, "err", err)
			return defaultAvatarURL
		}
		return u
	}
	graffitiURL := func(p string) string {
		if p == "" {
			return ""
		}
		u, err := presigner.PresignURL(context.Background(), p)
		if err != nil {
			slog.Default().Error("presign graffiti", "key", p, "err", err)
			return ""
		}
		return u
	}

	funcs := template.FuncMap{
		"formatDate":  formatDate,
		"trim":        strings.TrimSpace,
		"abbrev":      AbbrevCount,
		"presence":    Presence,
		"avatarURL":   avatarURL,
		"graffitiURL": graffitiURL,
		"birthDate":   FormatBirthDate,
		"genderLabel": GenderLabel,
		"minInt":      minInt,
	}

	layouts, err := fs.Glob(fsys, "templates/layouts/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob layouts: %w", err)
	}
	partials, err := fs.Glob(fsys, "templates/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}
	pages, err := fs.Glob(fsys, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob pages: %w", err)
	}

	renderer := &Renderer{templates: make(map[string]*template.Template, len(pages))}
	for _, p := range pages {
		name := strings.TrimSuffix(path.Base(p), ".html")
		files := append([]string{}, layouts...)
		files = append(files, partials...)
		files = append(files, p)

		t, err := template.New("base.html").Funcs(funcs).ParseFS(fsys, files...)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", p, err)
		}
		renderer.templates[name] = t
	}
	return renderer, nil
}

func (r *Renderer) Page(w http.ResponseWriter, req *http.Request, name string, pd PageData) {
	t, ok := r.templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template %s not found", name), http.StatusInternalServerError)
		return
	}
	if u, ok := session.FromContext(req.Context()); ok {
		pd.User = &u
	}
	if pd.CSRFToken == "" {
		pd.CSRFToken = CSRFFromContext(req.Context())
	}
	if pd.IncomingRequests == 0 {
		pd.IncomingRequests = IncomingFromContext(req.Context())
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", pd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (r *Renderer) Fragment(w http.ResponseWriter, req *http.Request, page, fragment string, data any) {
	t, ok := r.templates[page]
	if !ok {
		http.Error(w, fmt.Sprintf("template %s not found", page), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pd := PageData{Data: data, CSRFToken: CSRFFromContext(req.Context())}
	if u, ok := session.FromContext(req.Context()); ok {
		pd.User = &u
	}
	if err := t.ExecuteTemplate(w, fragment, pd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func formatDate(t time.Time) string {
	return t.Local().Format("2 Jan 2006, 15:04")
}

type csrfKey struct{}

func ContextWithCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey{}, token)
}

func CSRFFromContext(ctx context.Context) string {
	v, _ := ctx.Value(csrfKey{}).(string)
	return v
}
