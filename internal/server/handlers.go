package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"time"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/models"
)

// Handler manages HTTP endpoints for Detetive IA.
type Handler struct {
	cfg Config
}

// NewHandler initializes route handlers with the server configuration.
func NewHandler(cfg Config) *Handler {
	return &Handler{cfg: cfg}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) error(w http.ResponseWriter, status int, message string) {
	h.json(w, status, map[string]string{"error": message})
}

func (h *Handler) getSessionToken(r *http.Request) string {
	if cookie, err := r.Cookie("session_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

func (h *Handler) getAuthenticatedUser(r *http.Request) (*models.User, error) {
	token := h.getSessionToken(r)
	if token == "" {
		return nil, errors.New("no session token provided")
	}
	if h.cfg.AuthService == nil {
		return nil, errors.New("auth service not initialized")
	}
	return h.cfg.AuthService.ValidateSession(token)
}

// HandleHealth provides system health and memory metrics for 1GB RAM environment validation.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := float64(m.Alloc) / (1024 * 1024)
	totalAllocMB := float64(m.TotalAlloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"alloc_mb":        allocMB,
		"total_alloc_mb":  totalAllocMB,
		"sys_mb":          sysMB,
		"num_gc":          m.NumGC,
		"goroutines":      runtime.NumGoroutine(),
		"memory_boundary": "< 60MB RAM Target (<1GB host footprint)",
	})
}

// HandleRegister handles new user creation.
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	user, err := h.cfg.AuthService.Register(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			h.error(w, http.StatusConflict, "username already taken")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			h.error(w, http.StatusBadRequest, err.Error())
			return
		}
		h.error(w, http.StatusInternalServerError, "failed to register user")
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"created_at": user.CreatedAt,
		},
	})
}

// HandleLogin authenticates credentials and assigns an HttpOnly session cookie.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	token, err := h.cfg.AuthService.Login(req.Username, req.Password)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	user, _ := h.cfg.AuthService.ValidateSession(token)

	h.json(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// HandleLogout clears active session.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := h.getSessionToken(r)
	if token != "" && h.cfg.AuthService != nil {
		_ = h.cfg.AuthService.Logout(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	h.json(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// HandleMe validates current session and returns authenticated user info.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.getAuthenticatedUser(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

// HandleGetCase returns case details, suspects, and clues, keeping solution fields hidden.
func (h *Handler) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.cfg.GameService == nil {
		h.error(w, http.StatusInternalServerError, "game service not available")
		return
	}

	caseID, err := h.cfg.GameService.SeedInitialCase()
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to load case")
		return
	}

	details, err := h.cfg.GameService.GetCaseDetails(caseID)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to load case details")
		return
	}

	// Sanitize output so players cannot cheat via inspector
	publicCase := map[string]interface{}{
		"id":          details.Case.ID,
		"title":       details.Case.Title,
		"description": details.Case.Description,
		"victim":      details.Case.Victim,
	}

	var publicSuspects []map[string]interface{}
	for _, s := range details.Suspects {
		publicSuspects = append(publicSuspects, map[string]interface{}{
			"id":          s.ID,
			"name":        s.Name,
			"role":        s.Role,
			"personality": s.Personality,
			"alibi":       s.Alibi,
		})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"case":     publicCase,
		"suspects": publicSuspects,
		"clues":    details.Clues,
	})
}

// HandleInterrogate processes interrogation questions with dynamic LLM or fallback.
func (h *Handler) HandleInterrogate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		SuspectID int64  `json:"suspect_id"`
		Question  string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if req.SuspectID <= 0 || strings.TrimSpace(req.Question) == "" {
		h.error(w, http.StatusBadRequest, "suspect_id and question are required")
		return
	}

	sessionID := h.getSessionToken(r)
	if sessionID == "" {
		sessionID = "guest_session"
	}

	reply, err := h.cfg.GameService.Interrogate(sessionID, req.SuspectID, req.Question)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "interrogation failed: "+err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"reply":   reply,
		"message": reply,
	})
}

// HandleAccuse evaluates player accusation against the solution.
func (h *Handler) HandleAccuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		CaseID    int64  `json:"case_id"`
		SuspectID int64  `json:"suspect_id"`
		Weapon    string `json:"weapon"`
		Motive    string `json:"motive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if req.CaseID <= 0 {
		req.CaseID = 1
	}

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	verdict, err := h.cfg.GameService.SubmitAccusation(userID, req.CaseID, req.SuspectID, req.Weapon, req.Motive)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to submit accusation: "+err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"is_correct": verdict.Correct,
		"correct":    verdict.Correct,
		"score":      verdict.Score,
		"message":    verdict.Feedback,
		"verdict":    verdict.Feedback,
		"details":    verdict,
	})
}
