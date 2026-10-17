package server

import (
	"database/sql"
	"net/http"
	"time"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/game"
	"detetive-ia/internal/room"
)

// Config holds server dependencies and configuration.
type Config struct {
	Addr        string
	DB          *sql.DB
	AuthService *auth.Service
	GameService *game.Service
	RoomManager *room.Manager
	Weaver      *game.Weaver
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
	if cfg.RoomManager == nil && cfg.DB != nil {
		cfg.RoomManager, _ = room.NewManager(cfg.DB)
	}
	if cfg.Weaver == nil && cfg.DB != nil {
		cfg.Weaver = game.NewWeaver(cfg.DB)
	}

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
	mux.HandleFunc("/api/game/solo/start", h.HandleStartSoloGame)
	mux.HandleFunc("/api/game/interrogate", h.HandleInterrogate)
	mux.HandleFunc("/api/game/accuse", h.HandleAccuse)
	mux.HandleFunc("/api/game/personas/saved", h.HandleSavedPersonas)

	// Room & Lobby endpoints (plural REST standard)
	mux.HandleFunc("/api/rooms", h.HandleRooms)
	mux.HandleFunc("/api/rooms/join", h.HandleJoinRoom)
	mux.HandleFunc("/api/rooms/", h.HandleRoomOperations)

	// Room & Lobby endpoints (singular compatibility for frontend)
	mux.HandleFunc("/api/room/create", h.HandleCreateRoom)
	mux.HandleFunc("/api/room/create-with-case", h.HandleCreateRoomWithCase)
	mux.HandleFunc("/api/room/join", h.HandleJoinRoom)
	mux.HandleFunc("/api/room/", h.HandleRoomOperations)

	// Detective Night digital cases & envelopes
	mux.HandleFunc("/api/game/cases/detective-night", h.HandleDetectiveNightCatalog)
	mux.HandleFunc("/api/game/case/", h.HandleDetectiveNightCaseEnvelopes)

	// Admin & OpenTelemetry endpoints
	mux.HandleFunc("/admin", h.HandleAdminPage)
	mux.HandleFunc("/api/admin/settings", h.HandleAdminSettings)
	mux.HandleFunc("/api/admin/logs", h.HandleAdminLogs)
	mux.HandleFunc("/api/admin/telemetry", h.HandleAdminTelemetry)
	mux.HandleFunc("/api/admin/test-ai", h.HandleAdminTestAI)

	// Static assets & PWA fallback
	if cfg.StaticFS != nil {
		mux.Handle("/", cfg.StaticFS)
	}

	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OWASP Defensive Security Headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// CORS Handling
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
