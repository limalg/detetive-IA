package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
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

	var caseID int64
	if caseIDStr := r.URL.Query().Get("case_id"); caseIDStr != "" {
		if id, err := strconv.ParseInt(caseIDStr, 10, 64); err == nil && id > 0 {
			caseID = id
		}
	}

	if caseID == 0 {
		if roomCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("room"))); roomCode != "" && h.cfg.RoomManager != nil {
			if roomObj, err := h.cfg.RoomManager.GetRoom(roomCode); err == nil && roomObj.CaseID != nil {
				caseID = *roomObj.CaseID
			}
		}
	}

	if caseID == 0 {
		var err error
		caseID, err = h.cfg.GameService.SeedInitialCase()
		if err != nil {
			h.error(w, http.StatusInternalServerError, "failed to load case")
			return
		}
	}

	details, err := h.cfg.GameService.GetCaseDetails(caseID)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to load case details: "+err.Error())
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

	timeRemaining := 420
	warrantsRemaining := 2
	if h.cfg.GameService != nil {
		var userID int64
		if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
			userID = user.ID
		}
		if inv, err := h.cfg.GameService.GetOrCreateInvestigation(userID, caseID); err == nil && inv != nil {
			timeRemaining = inv.TimeRemainingMinutes
			warrantsRemaining = inv.WarrantsRemaining
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"case":                   publicCase,
		"suspects":               publicSuspects,
		"clues":                  details.Clues,
		"time_remaining_minutes": timeRemaining,
		"warrants_remaining":     warrantsRemaining,
	})
}

// HandleStartSoloGame generates a new procedural case and clean investigation for solo play.
func (h *Handler) HandleStartSoloGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.cfg.GameService == nil {
		h.error(w, http.StatusInternalServerError, "game service not available")
		return
	}

	var req struct {
		ThemeID      string `json:"theme_id"`
		SuspectCount int    `json:"suspect_count"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	details, inv, err := h.cfg.GameService.StartSoloGame(userID, req.ThemeID, req.SuspectCount)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to start solo game: "+err.Error())
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
		"case":                   publicCase,
		"suspects":               publicSuspects,
		"clues":                  details.Clues,
		"time_remaining_minutes": inv.TimeRemainingMinutes,
		"warrants_remaining":     inv.WarrantsRemaining,
	})
}


// HandleInterrogate processes interrogation questions with clue confrontation and suspect stress calculation.
func (h *Handler) HandleInterrogate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		SuspectID      int64  `json:"suspect_id"`
		Question       string `json:"question"`
		ConfrontClueID int64  `json:"confront_clue_id"`
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

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	var caseID int64
	if h.cfg.DB != nil {
		_ = h.cfg.DB.QueryRow(`SELECT case_id FROM suspects WHERE id = ?`, req.SuspectID).Scan(&caseID)
	}
	if caseID == 0 {
		caseID = 1
	}

	var inv *models.Investigation
	if h.cfg.GameService != nil {
		inv, _ = h.cfg.GameService.GetLatestInvestigation(userID, caseID)
		if inv == nil {
			inv, _ = h.cfg.GameService.GetOrCreateInvestigation(userID, caseID)
		}
	}


	if inv != nil && (inv.Status == "failed" || inv.TimeRemainingMinutes <= 0 || inv.WarrantsRemaining <= 0) {
		h.json(w, http.StatusBadRequest, map[string]interface{}{
			"error":                  "investigação encerrada: tempo esgotado ou mandados revogados",
			"game_over":              true,
			"time_remaining_minutes": inv.TimeRemainingMinutes,
			"warrants_remaining":     inv.WarrantsRemaining,
		})
		return
	}

	stressLevel := 15
	confronted := false
	var reply string

	// Handle clue confrontation mechanics if confront_clue_id is provided
	if req.ConfrontClueID > 0 && h.cfg.DB != nil {
		var clueName, clueDesc string
		err := h.cfg.DB.QueryRow(`SELECT name, description FROM clues WHERE id = ?`, req.ConfrontClueID).Scan(&clueName, &clueDesc)
		if err == nil {
			confronted = true
			var suspName, suspAlibi string
			var isCulprit bool
			_ = h.cfg.DB.QueryRow(`SELECT name, alibi, is_culprit FROM suspects WHERE id = ?`, req.SuspectID).Scan(&suspName, &suspAlibi, &isCulprit)

			clueLower := strings.ToLower(clueName + " " + clueDesc)
			// Check if this clue contradicts the suspect or catches them red-handed
			if isCulprit && (strings.Contains(clueLower, "álibi") || strings.Contains(clueLower, "alibi") ||
				strings.Contains(clueLower, "falso") || strings.Contains(clueLower, "contradição") ||
				strings.Contains(clueLower, "registro") || strings.Contains(clueLower, "adaga") ||
				strings.Contains(clueLower, "veneno") || strings.Contains(clueLower, "cálice") ||
				strings.Contains(clueLower, "sangue") || strings.Contains(clueLower, strings.ToLower(suspName))) {
				stressLevel = 85
				reply = fmt.Sprintf("O suspeito empalidece em choque absoluto, os olhos cravados na evidência (%s) e a voz desmorona em pânico: 'Como... onde você conseguiu isso?! Isso... isso não podia estar com a polícia!'", clueName)
			} else if isCulprit {
				stressLevel = 50
				reply = fmt.Sprintf("O suspeito semicerra os olhos, visivelmente tenso ao encarar a evidência (%s): 'Você está blefando, detetive. Essa pista não me coloca no local do crime!'", clueName)
			} else {
				stressLevel = 35
				reply = fmt.Sprintf("O suspeito analisa a evidência (%s) com sobrancelhas erguidas e desdém frio: 'Uma peça intrigante, detetive, mas absolutamente inútil contra mim. Meu álibi é de ferro.'", clueName)
			}

			// Persist dialogue exchange
			_, _ = h.cfg.DB.Exec(`
				INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
				VALUES (?, ?, 'detective', ?)
			`, sessionID, req.SuspectID, fmt.Sprintf("[CONFRONTANDO COM: %s] %s", clueName, req.Question))

			_, _ = h.cfg.DB.Exec(`
				INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
				VALUES (?, ?, 'suspect', ?)
			`, sessionID, req.SuspectID, reply)
		}
	}

	if !confronted {
		var err error
		reply, err = h.cfg.GameService.InterrogateWithStress(sessionID, req.SuspectID, req.Question, stressLevel)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				h.error(w, http.StatusBadRequest, "suspect not found: "+err.Error())
				return
			}
			h.error(w, http.StatusInternalServerError, "interrogation failed: "+err.Error())
			return
		}
	}

	// Deduct time: 15 min for normal question, 20 min for confrontation
	deduction := 15
	if confronted {
		deduction = 20
	}

	timeRemaining := 420
	warrantsRemaining := 2
	gameOver := false
	if inv != nil && h.cfg.GameService != nil {
		rem, err := h.cfg.GameService.DeductTime(inv.ID, deduction)
		if err == nil {
			timeRemaining = rem
		}
		warrantsRemaining = inv.WarrantsRemaining
		if timeRemaining <= 0 {
			gameOver = true
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"reply":                  reply,
		"message":                reply,
		"stress_level":           stressLevel,
		"confronted":             confronted,
		"time_remaining_minutes": timeRemaining,
		"warrants_remaining":     warrantsRemaining,
		"game_over":              gameOver,
	})
}

// HandleAccuse evaluates player accusation against the solution.
func (h *Handler) HandleAccuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		CaseID    int64   `json:"case_id"`
		SuspectID int64   `json:"suspect_id"`
		Weapon    string  `json:"weapon"`
		Motive    string  `json:"motive"`
		ClueIDs   []int64 `json:"clue_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if req.SuspectID <= 0 || strings.TrimSpace(req.Weapon) == "" || strings.TrimSpace(req.Motive) == "" {
		h.error(w, http.StatusBadRequest, "suspect_id, weapon and motive are required")
		return
	}

	if req.CaseID <= 0 {
		req.CaseID = 1
	}

	if len(req.ClueIDs) != 2 {
		h.error(w, http.StatusBadRequest, "anexe duas pistas para fundamentar o mandado")
		return
	}

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	var inv *models.Investigation
	if h.cfg.GameService != nil {
		inv, _ = h.cfg.GameService.GetLatestInvestigation(userID, req.CaseID)
		if inv == nil {
			inv, _ = h.cfg.GameService.GetOrCreateInvestigation(userID, req.CaseID)
		}
	}


	if inv != nil && (inv.Status == "failed" || inv.TimeRemainingMinutes <= 0 || inv.WarrantsRemaining <= 0) {
		h.json(w, http.StatusBadRequest, map[string]interface{}{
			"error":                  "investigação encerrada: mandados esgotados ou tempo expirado",
			"game_over":              true,
			"time_remaining_minutes": inv.TimeRemainingMinutes,
			"warrants_remaining":     inv.WarrantsRemaining,
		})
		return
	}

	verdict, err := h.cfg.GameService.SubmitAccusation(userID, req.CaseID, req.SuspectID, req.Weapon, req.Motive)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			h.error(w, http.StatusBadRequest, "suspect not found: "+err.Error())
			return
		}
		h.error(w, http.StatusInternalServerError, "failed to submit accusation: "+err.Error())
		return
	}

	timeRemaining := 420
	warrantsLeft := 2
	gameOver := false
	if inv != nil {
		timeRemaining = inv.TimeRemainingMinutes
		warrantsLeft = inv.WarrantsRemaining
	}

	if verdict.Correct {
		if inv != nil && h.cfg.DB != nil {
			_, _ = h.cfg.DB.Exec(`
				UPDATE investigations
				SET status = 'solved', accused_suspect_id = ?, accused_weapon = ?, accused_motive = ?, completed_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, req.SuspectID, req.Weapon, req.Motive, inv.ID)
		}
	} else if inv != nil && h.cfg.GameService != nil {
		// Deduct 30 min on wrong guess
		rem, err := h.cfg.GameService.DeductTime(inv.ID, 30)
		if err == nil {
			timeRemaining = rem
		}
		// Deduct 1 warrant strike
		wLeft, isOver, err := h.cfg.GameService.RecordWarrantStrike(inv.ID)
		if err == nil {
			warrantsLeft = wLeft
			if isOver {
				gameOver = true
			}
		}
		if timeRemaining <= 0 {
			gameOver = true
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"is_correct":             verdict.Correct,
		"correct":                verdict.Correct,
		"score":                  verdict.Score,
		"message":                verdict.Feedback,
		"verdict":                verdict.Feedback,
		"details":                verdict,
		"time_remaining_minutes": timeRemaining,
		"warrants_remaining":     warrantsLeft,
		"game_over":              gameOver,
	})
}

// HandleRooms handles POST /api/rooms (create room) or redirects.
func (h *Handler) HandleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.HandleCreateRoom(w, r)
		return
	}
	h.error(w, http.StatusMethodNotAllowed, "method not allowed")
}

// HandleCreateRoom handles room creation (POST /api/room/create or POST /api/rooms).
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.cfg.RoomManager == nil {
		h.error(w, http.StatusInternalServerError, "room manager not available")
		return
	}

	var req struct {
		PersonaName string `json:"persona_name"`
		Role        string `json:"role"`
		Secret      string `json:"secret"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	user, _ := h.getAuthenticatedUser(r)
	hostID := int64(1)
	hostUsername := "Detetive Host"
	if user != nil {
		hostID = user.ID
		hostUsername = user.Username
	}

	personaName := strings.TrimSpace(req.PersonaName)
	if personaName == "" {
		personaName = hostUsername
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Detetive Líder"
	}

	newRoom, err := h.cfg.RoomManager.CreateRoom(hostID, hostUsername, personaName, role)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to create room: "+err.Error())
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"code":    newRoom.Code,
		"room":    newRoom,
		"players": newRoom.Players,
	})
}

// HandleJoinRoom handles player joining an existing room (POST /api/rooms/join or /api/room/join).
func (h *Handler) HandleJoinRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.cfg.RoomManager == nil {
		h.error(w, http.StatusInternalServerError, "room manager not available")
		return
	}

	var req struct {
		Code        string `json:"code"`
		PersonaName string `json:"persona_name"`
		Role        string `json:"role"`
		Secret      string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if len(code) != 4 {
		h.error(w, http.StatusBadRequest, "invalid room code")
		return
	}

	user, _ := h.getAuthenticatedUser(r)
	userID := int64(2)
	username := "Investigador"
	if user != nil {
		userID = user.ID
		username = user.Username
	}

	personaName := strings.TrimSpace(req.PersonaName)
	if personaName == "" {
		personaName = username
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Investigador"
	}

	joinedRoom, err := h.cfg.RoomManager.JoinRoom(code, userID, username, personaName, role)
	if err != nil {
		h.error(w, http.StatusBadRequest, "failed to join room: "+err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"code":    joinedRoom.Code,
		"room":    joinedRoom,
		"players": joinedRoom.Players,
	})
}

// HandleRoomOperations handles GET /api/rooms/{code} and POST /api/rooms/{code}/start (also supports /api/room/).
func (h *Handler) HandleRoomOperations(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	// e.g. api, rooms, NOIR or api, rooms, NOIR, start
	if len(parts) < 3 {
		h.error(w, http.StatusBadRequest, "missing room code")
		return
	}

	if parts[len(parts)-1] == "start" && r.Method == http.MethodPost {
		code := strings.ToUpper(parts[len(parts)-2])
		h.handleStartRoom(w, r, code)
		return
	}

	if r.Method == http.MethodGet {
		code := strings.ToUpper(parts[len(parts)-1])
		if h.cfg.RoomManager == nil {
			h.error(w, http.StatusInternalServerError, "room manager not available")
			return
		}
		roomObj, err := h.cfg.RoomManager.GetRoom(code)
		if err != nil {
			h.error(w, http.StatusNotFound, "room not found")
			return
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"code":    roomObj.Code,
			"status":  roomObj.Status,
			"room":    roomObj,
			"players": roomObj.Players,
		})
		return
	}

	h.error(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) handleStartRoom(w http.ResponseWriter, r *http.Request, code string) {
	if h.cfg.RoomManager == nil {
		h.error(w, http.StatusInternalServerError, "room manager not available")
		return
	}

	roomObj, err := h.cfg.RoomManager.GetRoom(code)
	if err != nil {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	if roomObj.Status != "waiting" && roomObj.Status != "lobby" {
		h.error(w, http.StatusBadRequest, "room is already active or finished")
		return
	}

	var caseID int64
	if h.cfg.Weaver != nil {
		wResult, err := h.cfg.Weaver.WeaveCase(roomObj.Players)
		if err != nil {
			h.error(w, http.StatusInternalServerError, "failed to weave mystery case: "+err.Error())
			return
		}
		caseID = wResult.CaseID
	} else if h.cfg.GameService != nil {
		caseID, _ = h.cfg.GameService.SeedInitialCase()
	}

	startedRoom, err := h.cfg.RoomManager.StartRoom(code, roomObj.HostID)
	if err != nil {
		h.error(w, http.StatusBadRequest, "failed to start room: "+err.Error())
		return
	}

	if caseID > 0 {
		startedRoom.CaseID = &caseID
		if h.cfg.DB != nil {
			_, _ = h.cfg.DB.Exec("UPDATE rooms SET case_id = ? WHERE code = ?", caseID, code)
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"code":    startedRoom.Code,
		"room":    startedRoom,
		"case_id": caseID,
		"players": startedRoom.Players,
	})
}
