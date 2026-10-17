package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/game"
	"detetive-ia/internal/models"
	"detetive-ia/internal/telemetry"
	"detetive-ia/web"
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
		"memory_boundary": "Unrestricted / Dynamic Host Footprint",
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
func sanitizePublicSuspect(s models.Suspect) map[string]interface{} {
	return map[string]interface{}{
		"id":           s.ID,
		"name":         s.Name,
		"role":         s.Role,
		"occupation":   s.Occupation,
		"personality":  s.Personality,
		"alibi":        s.Alibi,
		"routine":      s.Routine,
		"tics":         s.Tics,
		"avatar":       s.Avatar,
		"gender":       s.Gender,
		"repeat_count": s.RepeatCount,
	}
}

// HandleGetCase returns case details, suspects, and clues.
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
		publicSuspects = append(publicSuspects, sanitizePublicSuspect(s))
	}

	timeRemaining := 40
	timerLimit := 40
	warrantsRemaining := 2
	if h.cfg.GameService != nil {
		var userID int64
		if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
			userID = user.ID
		}
		if inv, err := h.cfg.GameService.GetOrCreateInvestigation(userID, caseID); err == nil && inv != nil {
			timeRemaining = inv.TimeRemainingMinutes
			timerLimit = inv.TimerLimitMinutes
			if timerLimit <= 0 {
				timerLimit = 40
			}
			warrantsRemaining = inv.WarrantsRemaining
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"case":                   publicCase,
		"suspects":               publicSuspects,
		"clues":                  details.Clues,
		"time_remaining_minutes": timeRemaining,
		"timer_limit_minutes":    timerLimit,
		"warrants_remaining":     warrantsRemaining,
	})
}

// HandleStartSoloGame generates a new procedural case and clean investigation for solo play with customizable time.
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
		ThemeID          string `json:"theme_id"`
		SuspectCount     int    `json:"suspect_count"`
		TimeLimitMinutes int    `json:"time_limit_minutes"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.TimeLimitMinutes <= 0 {
		req.TimeLimitMinutes = 40
	}

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	details, inv, err := h.cfg.GameService.StartSoloGame(userID, req.ThemeID, req.SuspectCount, req.TimeLimitMinutes)
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
		publicSuspects = append(publicSuspects, sanitizePublicSuspect(s))
	}

	timerLimit := inv.TimerLimitMinutes
	if timerLimit <= 0 {
		timerLimit = req.TimeLimitMinutes
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"case":                   publicCase,
		"suspects":               publicSuspects,
		"clues":                  details.Clues,
		"time_remaining_minutes": inv.TimeRemainingMinutes,
		"timer_limit_minutes":    timerLimit,
		"warrants_remaining":     inv.WarrantsRemaining,
	})
}


// HandleInterrogate processes interrogation questions with clue confrontation, repeat question hostility, and suspect stress calculation.
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

	// Fetch full suspect profile
	var susp models.Suspect
	if h.cfg.DB != nil {
		_ = h.cfg.DB.QueryRow(`
			SELECT id, case_id, name, role, personality, alibi, secret, is_culprit,
			       COALESCE(tier1_alibi, ''), COALESCE(tier2_petty_crime, ''), COALESCE(tier3_truth, ''), COALESCE(cross_suspicion, ''),
			       COALESCE(gender, ''), COALESCE(avatar, ''),
			       COALESCE(occupation, ''), COALESCE(routine, ''), COALESCE(tics, ''), COALESCE(repeat_count, 0)
			FROM suspects WHERE id = ?
		`, req.SuspectID).Scan(
			&susp.ID, &susp.CaseID, &susp.Name, &susp.Role, &susp.Personality, &susp.Alibi, &susp.Secret, &susp.IsCulprit,
			&susp.Tier1Alibi, &susp.Tier2PettyCrime, &susp.Tier3Truth, &susp.CrossSuspicion, &susp.Gender, &susp.Avatar,
			&susp.Occupation, &susp.Routine, &susp.Tics, &susp.RepeatCount,
		)
	}

	caseID := susp.CaseID
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

	// Check repetition: count past questions and check similarity with previous questions
	var pastCount int
	var lastQuestion string
	if h.cfg.DB != nil {
		_ = h.cfg.DB.QueryRow(`
			SELECT COUNT(*), COALESCE((SELECT message FROM interrogation_messages WHERE session_id = ? AND suspect_id = ? AND sender = 'detective' ORDER BY id DESC LIMIT 1), '')
			FROM interrogation_messages
			WHERE session_id = ? AND suspect_id = ? AND sender = 'detective'
		`, sessionID, req.SuspectID, sessionID, req.SuspectID).Scan(&pastCount, &lastQuestion)
	}

	isRepeatQuestion := false
	normCurrent := strings.ToLower(strings.TrimSpace(req.Question))
	normLast := strings.ToLower(strings.TrimSpace(lastQuestion))
	if normLast != "" && (normCurrent == normLast || strings.Contains(normLast, normCurrent) || strings.Contains(normCurrent, normLast)) {
		isRepeatQuestion = true
	}

	newRepeatCount := susp.RepeatCount
	if isRepeatQuestion {
		newRepeatCount += 2
	} else if pastCount >= 2 {
		newRepeatCount++
	}
	if h.cfg.DB != nil {
		_, _ = h.cfg.DB.Exec(`UPDATE suspects SET repeat_count = ? WHERE id = ?`, newRepeatCount, req.SuspectID)
	}

	stressLevel := 15 + (newRepeatCount * 22)
	confronted := false
	var reply string

	// Handle clue confrontation mechanics if confront_clue_id is provided
	if req.ConfrontClueID > 0 && h.cfg.DB != nil {
		var clueName, clueDesc string
		err := h.cfg.DB.QueryRow(`SELECT name, description FROM clues WHERE id = ?`, req.ConfrontClueID).Scan(&clueName, &clueDesc)
		if err == nil {
			confronted = true
			stressLevel += 35
			clueLower := strings.ToLower(clueName + " " + clueDesc)
			ticAction := ""
			if susp.Tics != "" {
				ticAction = fmt.Sprintf("[%s] ", susp.Tics)
			}

			if susp.IsCulprit && (strings.Contains(clueLower, "álibi") || strings.Contains(clueLower, "alibi") ||
				strings.Contains(clueLower, "falso") || strings.Contains(clueLower, "contradição") ||
				strings.Contains(clueLower, "registro") || strings.Contains(clueLower, "adaga") ||
				strings.Contains(clueLower, "veneno") || strings.Contains(clueLower, "cálice") ||
				strings.Contains(clueLower, "sangue") || strings.Contains(clueLower, strings.ToLower(susp.Name))) {
				stressLevel = 90
				reply = fmt.Sprintf("%s%s empalidece em choque absoluto, os olhos cravados na evidência (%s) e a voz desmorona em pânico: 'Como... onde você conseguiu isso?! Isso... isso não podia estar com a polícia!'", ticAction, susp.Name, clueName)
			} else if susp.IsCulprit {
				stressLevel = 60
				reply = fmt.Sprintf("%s%s semicerra os olhos, visivelmente tenso ao encarar a evidência (%s): 'Você está blefando, detetive. Essa pista não me coloca no local do crime!'", ticAction, susp.Name, clueName)
			} else {
				stressLevel = 40
				reply = fmt.Sprintf("%s%s analisa a evidência (%s) com sobrancelhas erguidas e desdém frio: 'Uma peça intrigante, detetive, mas absolutamente inútil contra mim. Meu álibi é de ferro.'", ticAction, susp.Name, clueName)
			}

			// Consult AI client if available
			if h.cfg.GameService != nil {
				confrontPrompt := fmt.Sprintf("[O detetive confronta você com a evidência '%s' (%s)]. Pergunta: %s. Seu perfil: Profissão=%s, Rotina=%s, Tiques=%s. Estresse atual=%d/100.", clueName, clueDesc, req.Question, susp.Occupation, susp.Routine, susp.Tics, stressLevel)
				if aiReply, err := h.cfg.GameService.InterrogateWithStress(sessionID, req.SuspectID, confrontPrompt, stressLevel); err == nil && aiReply != "" {
					reply = aiReply
				}
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
		ticAction := ""
		if susp.Tics != "" {
			ticAction = fmt.Sprintf("[%s] ", susp.Tics)
		}

		// Check if AI is available
		var aiReply string
		if h.cfg.GameService != nil {
			aiContextPrompt := fmt.Sprintf(`Você é %s, atuando como %s (%s). Sua rotina na noite foi: %s. Seus tiques nervosos são: %s.
Seu nível de estresse atual é %d/100.
O detetive fez a seguinte pergunta: "%s"
Histórico: você já foi interrogado/pressionado sobre isso %d vezes.
Instrução importante:
1. Responda diegeticamente em primeira pessoa com estilo Noir de época (anos 1940).
2. Se o detetive estiver repetindo perguntas (perguntas repetidas > 0 ou estresse > 50), mostre irritação crescente, recusa a repetir respostas óbvias e invoque seus tiques nervosos entre colchetes como [ajusta o nó da gravata com dedos trêmulos].
3. Faça alusão à sua profissão e rotina na noite. Se for culpado e o estresse passar de 80, mostre sinais graves de desmoronamento psicológico.`,
				susp.Name, susp.Role, susp.Occupation, susp.Routine, susp.Tics,
				stressLevel, req.Question, newRepeatCount,
			)
			aiReply, _ = h.cfg.GameService.InterrogateWithStress(sessionID, req.SuspectID, aiContextPrompt, stressLevel)
		}

		if aiReply != "" {
			reply = aiReply
		} else {
			// Rich diegetic fallback matching user's request for repeated questions
			if newRepeatCount >= 3 || (isRepeatQuestion && newRepeatCount >= 2) {
				stressLevel = 90
				if susp.IsCulprit {
					reply = fmt.Sprintf("%s%s explode em suor frio e desespero: 'Por que insiste nessa mesma pergunta, detetive?! Já disse que estava %s! Não tente me encurralar com suas perguntas venenosas!'", ticAction, susp.Name, susp.Routine)
				} else {
					reply = fmt.Sprintf("%s%s bate a palma da mão na mesa com cólera: 'Detetive, já repeti isso %d vezes! Minha ocupação como %s exige respeito e meu álibi (%s) é irrefutável. Traga provas reais ou encerre este interrogatório agora!'", ticAction, susp.Name, newRepeatCount, susp.Occupation, susp.Alibi)
				}
			} else if newRepeatCount >= 1 || isRepeatQuestion {
				stressLevel = 55
				reply = fmt.Sprintf("%s%s franze o cenho com visível irritação: 'Detetive, já respondi exatamente a isso há poucos instantes. Como lhe expliquei, durante a noite eu estava %s. O senhor por acaso não anota as respostas em seu bloco de notas?'", ticAction, susp.Name, susp.Routine)
			} else {
				// First time question
				qLower := strings.ToLower(req.Question)
				if strings.Contains(qLower, "rotina") || strings.Contains(qLower, "horas") || strings.Contains(qLower, "onde") || strings.Contains(qLower, "estava") {
					reply = fmt.Sprintf("%s%s ajeita a postura e relata: 'Naquela noite fatídica, minha rotina foi clara: %s. Não tenho nada a esconder.'", ticAction, susp.Name, susp.Routine)
				} else if strings.Contains(qLower, "trabalho") || strings.Contains(qLower, "profissão") || strings.Contains(qLower, "ocupação") {
					reply = fmt.Sprintf("%s%s responde com ar de dignidade: 'Como %s, tenho deveres rigorosos. %s'", ticAction, susp.Name, susp.Occupation, susp.Alibi)
				} else if strings.Contains(qLower, "suspeita") || strings.Contains(qLower, "quem") || strings.Contains(qLower, "acha") {
					if susp.CrossSuspicion != "" {
						reply = fmt.Sprintf("%s%s inclina-se para a frente e sussurra: '%s'", ticAction, susp.Name, susp.CrossSuspicion)
					} else {
						reply = fmt.Sprintf("%s%s murmura com desdém: 'Todos nesta casa têm algo a esconder sob as aparências, detetive.'", ticAction, susp.Name)
					}
				} else {
					reply = fmt.Sprintf("%s%s olha firmemente nos olhos do investigador: '%s'", ticAction, susp.Name, susp.Alibi)
				}
			}
		}

		if h.cfg.DB != nil {
			_, _ = h.cfg.DB.Exec(`
				INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
				VALUES (?, ?, 'detective', ?)
			`, sessionID, req.SuspectID, req.Question)

			_, _ = h.cfg.DB.Exec(`
				INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
				VALUES (?, ?, 'suspect', ?)
			`, sessionID, req.SuspectID, reply)
		}
	}

	if stressLevel > 100 {
		stressLevel = 100
	}

	// Deduct time: 1 min for normal question, 2 min for confrontation or repetitive interrogation
	deduction := 1
	if confronted || isRepeatQuestion || newRepeatCount >= 2 {
		deduction = 2
	}

	timeRemaining := 40
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
		"repeat_count":           newRepeatCount,
		"is_repeated":            isRepeatQuestion,
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

	verdict, err := h.cfg.GameService.SubmitAccusation(userID, req.CaseID, req.SuspectID, req.Weapon, req.Motive, req.ClueIDs...)
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
	var userID int64
	username := "Investigador"
	if user != nil {
		userID = user.ID
		username = user.Username
	} else {
		// Unique negative ID for unauthenticated guests so distinct guests don't collide
		userID = -time.Now().UnixNano()
		if req.PersonaName != "" {
			username = req.PersonaName
		}
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

// HandleAdminSettings retrieves or updates the AI configuration.
func (h *Handler) HandleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if h.cfg.DB == nil {
		h.error(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	user, err := h.getAuthenticatedUser(r)
	if err != nil || user == nil || (!user.IsAdmin && user.Username != "admin") {
		h.error(w, http.StatusForbidden, "acesso restrito apenas para administradores")
		return
	}

	if r.Method == http.MethodGet {
		settings := make(map[string]string)
		rows, err := h.cfg.DB.Query("SELECT key, value FROM system_settings")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, v string
				if err := rows.Scan(&k, &v); err == nil {
					settings[k] = v
				}
			}
		}
		// Mascarar chave para proteção em trânsito
		maskedKey := settings["ai_api_key"]
		if len(maskedKey) > 8 {
			maskedKey = maskedKey[:4] + "••••••••" + maskedKey[len(maskedKey)-4:]
		} else if len(maskedKey) > 0 {
			maskedKey = "••••••••"
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"url":         settings["ai_url"],
			"model":       settings["ai_model"],
			"provider":    settings["ai_provider"],
			"api_key":     maskedKey,
			"masked_key":  maskedKey,
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			URL      string `json:"url"`
			APIKey   string `json:"api_key"`
			Model    string `json:"model"`
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.error(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
			return
		}

		if req.URL != "" {
			_, _ = h.cfg.DB.Exec("INSERT OR REPLACE INTO system_settings (key, value) VALUES ('ai_url', ?)", req.URL)
		}
		if req.APIKey != "" {
			_, _ = h.cfg.DB.Exec("INSERT OR REPLACE INTO system_settings (key, value) VALUES ('ai_api_key', ?)", req.APIKey)
		}
		if req.Model != "" {
			_, _ = h.cfg.DB.Exec("INSERT OR REPLACE INTO system_settings (key, value) VALUES ('ai_model', ?)", req.Model)
		}
		if req.Provider != "" {
			_, _ = h.cfg.DB.Exec("INSERT OR REPLACE INTO system_settings (key, value) VALUES ('ai_provider', ?)", req.Provider)
		}

		// Atualizar cliente de IA ativo em tempo real
		if h.cfg.GameService != nil {
			newClient := game.NewUniversalAIClient(game.AIConfig{
				URL:      req.URL,
				APIKey:   req.APIKey,
				Model:    req.Model,
				Provider: req.Provider,
			})
			h.cfg.GameService.SetAIClient(newClient)
		}

		telemetry.Info("admin", "", "Configurações de IA atualizadas com sucesso", map[string]interface{}{
			"model": req.Model,
			"url":   req.URL,
		})

		h.json(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "Configurações da IA atualizadas com sucesso",
		})
		return
	}

	h.error(w, http.StatusMethodNotAllowed, "method not allowed")
}

// HandleAdminLogs retrieves the live OpenTelemetry structured logs.
func (h *Handler) HandleAdminLogs(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthenticatedUser(r)
	if err != nil || user == nil || (!user.IsAdmin && user.Username != "admin") {
		h.error(w, http.StatusForbidden, "acesso restrito para administradores")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	level := r.URL.Query().Get("level")

	logs := telemetry.GetCollector().GetRecentLogs(limit, level)
	h.json(w, http.StatusOK, map[string]interface{}{
		"total": len(logs),
		"logs":  logs,
	})
}

// HandleAdminTelemetry retrieves system performance metrics and OpenTelemetry traces.
func (h *Handler) HandleAdminTelemetry(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthenticatedUser(r)
	if err != nil || user == nil || (!user.IsAdmin && user.Username != "admin") {
		h.error(w, http.StatusForbidden, "acesso restrito para administradores")
		return
	}

	metrics := telemetry.Metrics()
	spans := telemetry.GetCollector().GetRecentSpans(30)

	h.json(w, http.StatusOK, map[string]interface{}{
		"metrics": metrics,
		"spans":   spans,
	})
}

// HandleAdminTestAI tests sending a prompt through the active AI model.
func (h *Handler) HandleAdminTestAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := h.getAuthenticatedUser(r)
	if err != nil || user == nil || (!user.IsAdmin && user.Username != "admin") {
		h.error(w, http.StatusForbidden, "acesso restrito para administradores")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Prompt == "" {
		req.Prompt = "Olá! Confirme que você é a IA investigativa do Detetive IA em apenas uma frase temática."
	}

	start := time.Now()
	aiClient := h.cfg.GameService.GetAIClient()
	if aiClient == nil {
		h.error(w, http.StatusBadRequest, "nenhum cliente de IA inicializado")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := aiClient.GenerateResponse(ctx, req.Prompt)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		telemetry.Error("ai_client", "", "Falha no teste de conexão da IA: "+err.Error(), map[string]interface{}{
			"duration_ms": duration,
		})
		h.json(w, http.StatusOK, map[string]interface{}{
			"status":      "error",
			"error":       err.Error(),
			"duration_ms": duration,
		})
		return
	}

	telemetry.Info("ai_client", "", "Teste de IA bem-sucedido", map[string]interface{}{
		"duration_ms": duration,
		"response":    resp,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"response":    resp,
		"duration_ms": duration,
	})
}

// HandleDetectiveNightCatalog returns the pre-authored digital Detective Night cases.
func (h *Handler) HandleDetectiveNightCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := game.GetDetectiveNightCatalog()
	type CaseSummary struct {
		ID          int64  `json:"id,omitempty"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Victim      string `json:"victim"`
		Difficulty  string `json:"difficulty"`
		Category    string `json:"category"`
	}

	var summaries []CaseSummary
	for _, c := range catalog {
		var caseID int64
		if h.cfg.DB != nil {
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title = ?", c.Title).Scan(&caseID)
		}
		summaries = append(summaries, CaseSummary{
			ID:          caseID,
			Title:       c.Title,
			Description: c.Description,
			Victim:      c.Victim,
			Difficulty:  "4 de 5 (Investigação Profunda)",
			Category:    "detective_night",
		})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"total": len(summaries),
		"cases": summaries,
	})
}

// HandleAdminPage serves the admin dashboard only to authenticated admin users.
func (h *Handler) HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthenticatedUser(r)
	isAdmin := err == nil && user != nil && (user.IsAdmin || user.Username == "admin")
	if !isAdmin {
		if user == nil {
			http.Redirect(w, r, "/?auth=login_required&redirect=/admin", http.StatusFound)
			return
		}
		h.error(w, http.StatusForbidden, "acesso restrito para administradores")
		return
	}

	data, err := web.Assets.ReadFile("templates/admin.html")
	if err != nil {
		http.Error(w, "admin template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// HandleDetectiveNightCaseEnvelopes returns the 3 envelopes for a specific case.
func (h *Handler) HandleDetectiveNightCaseEnvelopes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/game/case/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		h.error(w, http.StatusBadRequest, "invalid case path")
		return
	}

	caseID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil && h.cfg.DB != nil {
		switch parts[0] {
		case "fazenda_coracao":
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Fazenda Coração%' LIMIT 1").Scan(&caseID)
		case "sherlock_holmes":
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Sherlock Holmes%' LIMIT 1").Scan(&caseID)
		case "carga_479":
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Carga 479%' LIMIT 1").Scan(&caseID)
		case "estrela_fria":
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Estrela Fria%' LIMIT 1").Scan(&caseID)
		default:
			_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE ? LIMIT 1", "%"+parts[0]+"%").Scan(&caseID)
		}
	}
	if caseID == 0 {
		h.error(w, http.StatusBadRequest, "invalid case id")
		return
	}

	envelopes, err := game.GetCaseEnvelopes(h.cfg.DB, caseID)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to get case envelopes: "+err.Error())
		return
	}

	h.json(w, http.StatusOK, envelopes)
}

// HandleCreateRoomWithCase creates a room with custom case selection and custom character casting.
func (h *Handler) HandleCreateRoomWithCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		CaseID           interface{}              `json:"case_id"`
		CustomCharacters []models.CustomCharacter `json:"custom_characters"`
		CustomCast       []models.CustomCharacter `json:"custom_cast"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}

	var caseID int64
	caseSlug := ""
	switch v := req.CaseID.(type) {
	case float64:
		caseID = int64(v)
	case int64:
		caseID = v
	case string:
		caseSlug = v
		caseID, _ = strconv.ParseInt(v, 10, 64)
		if caseID == 0 && h.cfg.DB != nil {
			switch v {
			case "fazenda_coracao":
				_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Fazenda Coração%' LIMIT 1").Scan(&caseID)
			case "sherlock_holmes":
				_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Sherlock Holmes%' LIMIT 1").Scan(&caseID)
			case "carga_479":
				_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Carga 479%' LIMIT 1").Scan(&caseID)
			case "estrela_fria":
				_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE '%Estrela Fria%' LIMIT 1").Scan(&caseID)
			default:
				_ = h.cfg.DB.QueryRow("SELECT id FROM cases WHERE title LIKE ? LIMIT 1", "%"+v+"%").Scan(&caseID)
			}
		}
	}
	if caseID <= 0 {
		caseID = 1
	}

	characters := req.CustomCharacters
	if len(characters) == 0 && len(req.CustomCast) > 0 {
		characters = req.CustomCast
	}

	user, err := h.getAuthenticatedUser(r)
	var hostID int64 = 1
	username := "Detetive Anfitrião"
	if err == nil && user != nil {
		hostID = user.ID
		username = user.Username
	}

	roomObj, err := h.cfg.RoomManager.CreateRoom(hostID, username, username, "Investigador Chefe")
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to create room: "+err.Error())
		return
	}

	// Se o jogador enviou personagens customizados (reais ou fictícios), a IA ou Weaver organiza o elenco
	if len(characters) > 0 && h.cfg.DB != nil {
		var aiClient game.AIClient
		if h.cfg.GameService != nil {
			aiClient = h.cfg.GameService.GetAIClient()
		}
		_ = game.CastCustomCharacters(r.Context(), aiClient, h.cfg.DB, caseID, characters)
	}

	roomObj.CaseID = &caseID
	if h.cfg.DB != nil {
		_, _ = h.cfg.DB.Exec("UPDATE rooms SET case_id = ? WHERE code = ?", caseID, roomObj.Code)
	}

	var suspects []models.Suspect
	if h.cfg.DB != nil {
		rows, err := h.cfg.DB.Query("SELECT id, case_id, name, role, personality, alibi FROM suspects WHERE case_id = ?", caseID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var s models.Suspect
				if err := rows.Scan(&s.ID, &s.CaseID, &s.Name, &s.Role, &s.Personality, &s.Alibi); err == nil {
					suspects = append(suspects, s)
				}
			}
		}
	}

	telemetry.Info("room", "", fmt.Sprintf("Sala %s criada com caso %d e %d personagens customizados", roomObj.Code, caseID, len(characters)), nil)

	returnCaseID := interface{}(caseID)
	if caseSlug != "" {
		returnCaseID = caseSlug
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"code":             roomObj.Code,
		"room":             roomObj,
		"case_id":          returnCaseID,
		"suspects":         suspects,
		"custom_cast":      characters,
		"custom_cast_size": len(characters),
	})
}

// HandleSavedPersonas handles GET, POST, DELETE /api/game/personas/saved.
func (h *Handler) HandleSavedPersonas(w http.ResponseWriter, r *http.Request) {
	if h.cfg.DB == nil {
		h.error(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	user, _ := h.getAuthenticatedUser(r)
	var userID int64
	if user != nil {
		userID = user.ID
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := h.cfg.DB.Query(`
			SELECT id, COALESCE(user_id, 0), name, role, occupation, personality, alibi, secret, routine, tics, avatar, gender, created_at
			FROM saved_personas
			WHERE user_id = ? OR user_id = 0 OR user_id IS NULL
			ORDER BY id DESC
		`, userID)
		if err != nil {
			h.error(w, http.StatusInternalServerError, "failed to query saved personas: "+err.Error())
			return
		}
		defer rows.Close()

		personas := []models.SavedPersona{}
		for rows.Next() {
			var p models.SavedPersona
			if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Role, &p.Occupation, &p.Personality, &p.Alibi, &p.Secret, &p.Routine, &p.Tics, &p.Avatar, &p.Gender, &p.CreatedAt); err == nil {
				personas = append(personas, p)
			}
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"personas": personas,
			"total":    len(personas),
		})

	case http.MethodPost:
		var req models.SavedPersona
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.error(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			h.error(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.Avatar == "" {
			req.Avatar = "/static/assets/char_vincent.webp"
		}
		if req.Gender == "" {
			req.Gender = "male"
		}
		if req.Role == "" {
			req.Role = "Investigador Particular"
		}

		res, err := h.cfg.DB.Exec(`
			INSERT INTO saved_personas (user_id, name, role, occupation, personality, alibi, secret, routine, tics, avatar, gender)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, req.Name, req.Role, req.Occupation, req.Personality, req.Alibi, req.Secret, req.Routine, req.Tics, req.Avatar, req.Gender)
		if err != nil {
			h.error(w, http.StatusInternalServerError, "failed to save persona: "+err.Error())
			return
		}
		id, _ := res.LastInsertId()
		req.ID = id
		req.UserID = userID
		h.json(w, http.StatusCreated, req)

	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			h.error(w, http.StatusBadRequest, "invalid persona id")
			return
		}
		_, err = h.cfg.DB.Exec(`DELETE FROM saved_personas WHERE id = ? AND (user_id = ? OR ? = 0)`, id, userID, userID)
		if err != nil {
			h.error(w, http.StatusInternalServerError, "failed to delete persona: "+err.Error())
			return
		}
		h.json(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}



