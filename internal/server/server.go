package server

import (
	"database/sql"
	"net/http"
	"time"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/game"
)

// Config holds server dependencies and configuration.
type Config struct {
	Addr        string
	DB          *sql.DB
	AuthService *auth.Service
	GameService *game.Service
	StaticFS    http.Handler
}

// NewServer returns an *http.Server configured with the Detetive IA router and network timeouts.
func NewServer(cfg Config) *http.Server {
	router := NewRouter(cfg)
	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// NewRouter registers API routes and mounts static asset serving.
func NewRouter(cfg Config) http.Handler {
	mux := http.NewServeMux()
	h := NewHandler(cfg)

	// Healthcheck & memory stats
	mux.HandleFunc("/api/health", h.HandleHealth)

	// Authentication endpoints
	mux.HandleFunc("/api/auth/register", h.HandleRegister)
	mux.HandleFunc("/api/auth/login", h.HandleLogin)
	mux.HandleFunc("/api/auth/logout", h.HandleLogout)
	mux.HandleFunc("/api/auth/me", h.HandleMe)

	// Game logic endpoints
	mux.HandleFunc("/api/game/case", h.HandleGetCase)
	mux.HandleFunc("/api/game/interrogate", h.HandleInterrogate)
	mux.HandleFunc("/api/game/accuse", h.HandleAccuse)

	// Static assets & PWA fallback
	if cfg.StaticFS != nil {
		mux.Handle("/", cfg.StaticFS)
	}

	return withCors(mux)
}

func withCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
