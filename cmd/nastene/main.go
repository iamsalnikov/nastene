package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iamsalnikov/nastene/internal/config"
	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/internal/repository"
	"github.com/iamsalnikov/nastene/internal/server"
	authsvc "github.com/iamsalnikov/nastene/internal/service/auth"
	"github.com/iamsalnikov/nastene/internal/service/friends"
	"github.com/iamsalnikov/nastene/internal/service/profile"
	"github.com/iamsalnikov/nastene/internal/service/wall"
	"github.com/iamsalnikov/nastene/internal/session"
	"github.com/iamsalnikov/nastene/internal/storage/objectstore"
	"github.com/iamsalnikov/nastene/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(log)

	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	log.Info("postgres connected")

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Info("migrations applied")

	userRepo := repository.NewUserRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	privacyRepo := repository.NewPrivacyRepo(pool)
	profilePrivacyRepo := repository.NewProfilePrivacyRepo(pool)
	banRepo := repository.NewBanRepo(pool)
	friendRepo := repository.NewFriendRepo(pool)
	wallPostRepo := repository.NewWallPostRepo(pool)
	commentRepo := repository.NewCommentRepo(pool)

	sessions := session.NewManager(sessionRepo, userRepo, cfg.SessionCookieName, cfg.SessionCookieSecure, cfg.SessionTTL)
	authorizer := wall.NewAuthorizer(privacyRepo, friendRepo, banRepo)
	wallService := wall.NewService(wallPostRepo, userRepo, commentRepo, friendRepo, authorizer)
	friendsService := friends.NewService(friendRepo, userRepo)
	friendsService.SetBanCheck(banRepo)
	authService := authsvc.NewService(userRepo, friendsService, cfg.InvitesPerUser, cfg.RegistrationMode == config.RegistrationModeInvite)

	objStore, err := objectstore.New(ctx, objectstore.Config{
		Endpoint:         cfg.S3Endpoint,
		Region:           cfg.S3Region,
		AccessKey:        cfg.S3AccessKey,
		SecretKey:        cfg.S3SecretKey,
		Bucket:           cfg.S3Bucket,
		UseSSL:           cfg.S3UseSSL,
		PresignTTL:       cfg.S3PresignTTL,
		AutoCreateBucket: cfg.S3AutoCreateBucket,
	})
	if err != nil {
		return fmt.Errorf("init object store: %w", err)
	}
	log.Info("object store ready", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)

	postResolver := repository.NewPostOwnerResolver(wallPostRepo)

	profileAuthorizer := profile.NewAuthorizer(profilePrivacyRepo, friendRepo, banRepo)
	profileService := profile.NewService(userRepo, profilePrivacyRepo, friendRepo, profileAuthorizer, objStore)

	renderer, err := render.New(web.FS, objStore)
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return fmt.Errorf("sub static fs: %w", err)
	}

	srv := server.New(server.Deps{
		Log:             log,
		Cfg:             cfg,
		DB:              pool,
		Sessions:        sessions,
		Renderer:        renderer,
		AuthService:     authService,
		WallService:     wallService,
		CommentService:  wallService,
		FriendsService:  friendsService,
		PrivacyService:  privacyRepo,
		BanService:      banRepo,
		UserLookup:      userRepo,
		PostOwner:       postResolver,
		GraffitiService: wallService,
		GraffitiStore:   objStore,
		Privacy:         privacyRepo,
		ProfileService:  profileService,
		ProfilePrivacy:  profilePrivacyRepo,
		IncomingCounter: friendRepo,
		StaticFS:        staticFS,
	})

	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	log.Info("server stopped cleanly")
	return nil
}
