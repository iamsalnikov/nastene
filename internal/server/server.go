package server

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-pkgz/routegroup"

	"github.com/iamsalnikov/nastene/internal/config"
	"github.com/iamsalnikov/nastene/internal/handler"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/session"
)

type Deps struct {
	Log             *slog.Logger
	Cfg             config.Config
	DB              handler.Pinger
	Sessions        *session.Manager
	Renderer        *render.Renderer
	AuthService     handler.AuthService
	WallService     handler.WallService
	CommentService  handler.CommentService
	FriendsService  handler.FriendsService
	PrivacyService  handler.PrivacyService
	BanService      handler.BanService
	UserLookup      handler.UserLookup
	PostOwner       handler.PostRefResolver
	GraffitiService handler.GraffitiService
	GraffitiStore   handler.GraffitiStoreRef
	Privacy         handler.PrivacyInitializer
	ProfileService  handler.ProfileService
	ProfilePrivacy  handler.PrivacyInitializer
	NewsService     handler.NewsService
	IncomingCounter render.IncomingCounter
	StaticFS        fs.FS
}

type Server struct {
	cfg    config.Config
	log    *slog.Logger
	server *http.Server
}

func New(deps Deps) *Server {
	root := http.NewServeMux()
	mux := routegroup.New(root)
	mux.Use(requestLogger(deps.Log))
	mux.Use(deps.Sessions.Middleware)
	mux.Use(render.CSRFMiddleware)
	mux.Use(render.TimezoneMiddleware)
	if deps.IncomingCounter != nil {
		mux.Use(render.HeaderStatsMiddleware(deps.IncomingCounter))
	}

	health := &handler.Health{DB: deps.DB}
	mux.HandleFunc("GET /healthz", health.Handle)

	landing := &handler.Landing{Renderer: deps.Renderer}
	mux.HandleFunc("GET /{$}", landing.Handle)

	auth := &handler.Auth{
		Service:        deps.AuthService,
		Sessions:       deps.Sessions,
		Privacy:        deps.Privacy,
		ProfilePrivacy: deps.ProfilePrivacy,
		Renderer:       deps.Renderer,
	}
	mux.HandleFunc("GET /register", auth.GetRegister)
	mux.HandleFunc("POST /register", auth.PostRegister)
	mux.HandleFunc("GET /login", auth.GetLogin)
	mux.HandleFunc("POST /login", auth.PostLogin)
	mux.HandleFunc("POST /logout", auth.PostLogout)

	authed := mux.With(deps.Sessions.RequireAuth)

	wall := &handler.Wall{
		Service:  deps.WallService,
		Profile:  deps.ProfileService,
		Renderer: deps.Renderer,
	}
	authed.HandleFunc("GET /id/{id}", wall.Get)
	authed.HandleFunc("POST /id/{id}/post", wall.PostText)
	authed.HandleFunc("GET /post/{id}", wall.GetPost)

	news := &handler.News{
		Service:  deps.NewsService,
		Renderer: deps.Renderer,
	}
	authed.HandleFunc("GET /news", news.Get)

	comment := &handler.Comment{
		Service: deps.CommentService,
		Posts:   deps.PostOwner,
	}
	authed.HandleFunc("POST /post/{id}/comment", comment.PostComment)
	authed.HandleFunc("POST /post/{id}/delete", comment.DeletePost)
	authed.HandleFunc("POST /comment/{id}/delete", comment.DeleteComment)

	friends := &handler.Friends{
		Service:  deps.FriendsService,
		Renderer: deps.Renderer,
	}
	authed.HandleFunc("GET /friends", friends.GetOverview)
	authed.HandleFunc("POST /friends/request/{id}", friends.Request)
	authed.HandleFunc("POST /friends/accept/{id}", friends.Accept)
	authed.HandleFunc("POST /friends/reject/{id}", friends.Reject)
	authed.HandleFunc("POST /friends/cancel/{id}", friends.Cancel)
	authed.HandleFunc("POST /friends/remove/{id}", friends.Remove)

	settings := &handler.Settings{
		Privacy:  deps.PrivacyService,
		Bans:     deps.BanService,
		Users:    deps.UserLookup,
		Renderer: deps.Renderer,
	}
	settingsHub := &handler.SettingsHub{Renderer: deps.Renderer}
	authed.HandleFunc("GET /settings", settingsHub.Get)
	authed.HandleFunc("GET /settings/privacy", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/settings", http.StatusFound)
	})
	authed.HandleFunc("GET /settings/privacy/wall", settings.GetPrivacy)
	authed.HandleFunc("POST /settings/privacy/wall", settings.PostPrivacy)
	authed.HandleFunc("GET /settings/bans", settings.GetBans)
	authed.HandleFunc("POST /settings/bans/remove/{id}", settings.PostBanRemove)
	authed.HandleFunc("POST /id/{id}/ban", settings.PostBanByID)
	authed.HandleFunc("POST /id/{id}/unban", settings.PostUnbanByID)

	profile := &handler.Profile{
		Service:  deps.ProfileService,
		Users:    deps.UserLookup,
		Renderer: deps.Renderer,
	}
	authed.HandleFunc("GET /settings/profile", profile.GetSettings)
	authed.HandleFunc("POST /settings/profile", profile.PostSettings)
	authed.HandleFunc("GET /settings/privacy/profile", profile.GetPrivacy)
	authed.HandleFunc("POST /settings/privacy/profile", profile.PostPrivacy)
	authed.HandleFunc("GET /id/{id}/friends", profile.GetUserFriends)

	graffiti := &handler.Graffiti{
		Service:  deps.GraffitiService,
		Store:    deps.GraffitiStore,
		Users:    deps.UserLookup,
		Renderer: deps.Renderer,
	}
	authed.HandleFunc("GET /graffiti/{id}", graffiti.GetEditor)
	authed.HandleFunc("POST /graffiti/{id}", graffiti.PostCreate)

	staticHandler := http.StripPrefix("/static/", http.FileServer(http.FS(deps.StaticFS)))
	root.Handle("/static/", staticHandler)

	return &Server{
		cfg: deps.Cfg,
		log: deps.Log,
		server: &http.Server{
			Addr:              deps.Cfg.HTTPAddr,
			Handler:           idPathRewrite(mux),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// idPathRewrite превращает «короткий» путь /idN(/...) в /id/N(/...) до роутинга.
// Снаружи URL остаётся /id1, внутренние роуты регистрируются как валидные /id/{id}.
func idPathRewrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > 3 && path[0] == '/' && path[1] == 'i' && path[2] == 'd' && path[3] >= '0' && path[3] <= '9' {
			i := 3
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				i++
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/id/" + path[3:i] + path[i:]
			next.ServeHTTP(w, r2)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http listen", "addr", s.cfg.HTTPAddr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			cookieNames := make([]string, 0, len(r.Cookies()))
			for _, c := range r.Cookies() {
				cookieNames = append(cookieNames, c.Name)
			}
			next.ServeHTTP(w, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"cookies", cookieNames,
				"duration", time.Since(start).String(),
			)
		})
	}
}
