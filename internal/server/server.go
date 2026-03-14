package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vutran1710/dating-dev/internal/server/handler"
	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	Auth        handler.AuthConfig
}

func LoadConfig() Config {
	return Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:54322/postgres"),
		JWTSecret:   envOr("JWT_SECRET", "dev-secret-change-me"),
		Auth: handler.AuthConfig{
			GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			BackendURL:         envOr("BACKEND_URL", "http://localhost:8080"),
			StateSecret:        envOr("STATE_SECRET", "dev-state-secret"),
		},
	}
}

func Run(ctx context.Context) error {
	cfg := LoadConfig()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initializing store: %w", err)
	}
	defer db.Close()

	authMw := middleware.NewAuth(cfg.JWTSecret)
	rateLimiter := middleware.NewRateLimiter(60, time.Minute)

	authH := handler.NewAuthHandler(db, authMw, cfg.Auth)
	discoverH := handler.NewDiscoverHandler(db)
	likesH := handler.NewLikesHandler(db)
	matchesH := handler.NewMatchesHandler(db)
	messagesH := handler.NewMessagesHandler(db)
	commitmentH := handler.NewCommitmentHandler(db)
	profileH := handler.NewProfileHandler(db)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/{provider}", authH.StartOAuth)
	mux.HandleFunc("GET /auth/callback/{provider}", authH.Callback)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/whoami", authH.Whoami)
	apiMux.HandleFunc("GET /api/discover", discoverH.Discover)
	apiMux.HandleFunc("GET /api/profiles/{public_id}", profileH.View)
	apiMux.HandleFunc("PUT /api/profile", profileH.Update)
	apiMux.HandleFunc("POST /api/likes", likesH.Like)
	apiMux.HandleFunc("GET /api/matches", matchesH.List)
	apiMux.HandleFunc("GET /api/conversations/{conv_id}/messages", messagesH.List)
	apiMux.HandleFunc("POST /api/conversations/{conv_id}/messages", messagesH.Send)
	apiMux.HandleFunc("POST /api/commitments", commitmentH.Propose)
	apiMux.HandleFunc("PUT /api/commitments/{id}/accept", commitmentH.Accept)
	apiMux.HandleFunc("PUT /api/commitments/{id}/decline", commitmentH.Decline)
	apiMux.HandleFunc("GET /api/commitments/status", commitmentH.Status)

	mux.Handle("/api/", authMw.Authenticate(rateLimiter.Limit(apiMux)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("server listening on :%s", cfg.Port)
	return srv.ListenAndServe()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
