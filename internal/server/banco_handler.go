package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"detetive-ia/internal/banco"
)

func (h *Handler) getBancoGame(r *http.Request, explicitID string) *banco.Game {
	if h.cfg.BancoManager == nil {
		return nil
	}

	gameID := strings.TrimSpace(explicitID)
	if gameID == "" {
		gameID = r.URL.Query().Get("game_id")
	}
	if gameID == "" {
		if cookie, err := r.Cookie("banco_game_id"); err == nil {
			gameID = cookie.Value
		}
	}

	if gameID == "" {
		return nil
	}

	return h.cfg.BancoManager.GetGame(gameID)
}

// HandleBancoStart initializes a new game of Banco Imobiliário IA.
func (h *Handler) HandleBancoStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.cfg.BancoManager == nil {
		h.cfg.BancoManager = banco.NewManager()
	}

	var req struct {
		City            string   `json:"city"`
		Age             int      `json:"age"`
		SchoolGrade     string   `json:"school_grade"`
		Difficulty      string   `json:"difficulty"`
		BotsCount       int      `json:"bots_count"`
		StartingBalance int      `json:"starting_balance"`
		HumanName       string   `json:"human_name"`
		Friends         []string `json:"friends"`
	}

	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if req.HumanName == "" {
		if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
			req.HumanName = user.Username
		}
	}

	opts := banco.GameOptions{
		City:            req.City,
		Age:             req.Age,
		SchoolGrade:     req.SchoolGrade,
		Difficulty:      req.Difficulty,
		BotsCount:       req.BotsCount,
		StartingBalance: req.StartingBalance,
		HumanName:       req.HumanName,
		Friends:         req.Friends,
	}

	game := h.cfg.BancoManager.CreateGame(opts)
	state := game.GetState()

	http.SetCookie(w, &http.Cookie{
		Name:     "banco_game_id",
		Value:    state.ID,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"game_id": state.ID,
		"state":   state,
		"message": "Partida de Banco Imobiliário IA iniciada com sucesso!",
	})
}

// HandleBancoState returns the current state of a Banco Imobiliário IA game.
func (h *Handler) HandleBancoState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	game := h.getBancoGame(r, "")
	if game == nil {
		h.error(w, http.StatusNotFound, "partida não encontrada ou game_id ausente")
		return
	}

	h.json(w, http.StatusOK, game.GetState())
}

// HandleBancoRoll handles a dice roll for the current player.
func (h *Handler) HandleBancoRoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		GameID   string `json:"game_id"`
		PlayerID int    `json:"player_id"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	game := h.getBancoGame(r, req.GameID)
	if game == nil {
		h.error(w, http.StatusNotFound, "partida não encontrada")
		return
	}

	state := game.GetState()
	playerID := req.PlayerID
	if playerID <= 0 {
		playerID = state.CurrentTurn
	}

	res, err := game.RollDice(playerID)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"dice":         res.Dice,
		"total":        res.Total,
		"is_double":    res.IsDouble,
		"new_position": res.NewPosition,
		"passed_start": res.PassedStart,
		"can_buy":      res.CanBuy,
		"rent_paid":    res.RentPaid,
		"rent_to":      res.RentTo,
		"event_log":    res.EventLog,
		"message":      res.EventLog,
		"state":        game.GetState(),
	})
}

// HandleBancoBuy allows a player to buy the unowned property at their current tile.
func (h *Handler) HandleBancoBuy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		GameID   string `json:"game_id"`
		PlayerID int    `json:"player_id"`
		TilePos  *int   `json:"tile_pos"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	game := h.getBancoGame(r, req.GameID)
	if game == nil {
		h.error(w, http.StatusNotFound, "partida não encontrada")
		return
	}

	state := game.GetState()
	playerID := req.PlayerID
	if playerID <= 0 {
		playerID = state.CurrentTurn
	}

	tilePos := 0
	if req.TilePos != nil {
		tilePos = *req.TilePos
	} else {
		// Default to player's current position
		p := game.GetPlayer(playerID)
		if p != nil {
			tilePos = p.Position
		}
	}

	err := game.BuyProperty(playerID, tilePos)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"message": "Propriedade comprada com sucesso!",
		"state":   game.GetState(),
	})
}

// HandleBancoBuild constructs a house or hotel on an eligible property.
func (h *Handler) HandleBancoBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		GameID   string `json:"game_id"`
		PlayerID int    `json:"player_id"`
		TilePos  int    `json:"tile_pos"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	game := h.getBancoGame(r, req.GameID)
	if game == nil {
		h.error(w, http.StatusNotFound, "partida não encontrada")
		return
	}

	state := game.GetState()
	playerID := req.PlayerID
	if playerID <= 0 {
		playerID = state.CurrentTurn
	}

	err := game.BuildHouse(playerID, req.TilePos)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"message": "Construção realizada com sucesso!",
		"state":   game.GetState(),
	})
}

// HandleBancoEndTurn finishes the human player's turn and runs autonomous bot turns.
func (h *Handler) HandleBancoEndTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		GameID   string `json:"game_id"`
		PlayerID int    `json:"player_id"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	game := h.getBancoGame(r, req.GameID)
	if game == nil {
		h.error(w, http.StatusNotFound, "partida não encontrada")
		return
	}

	state := game.GetState()
	playerID := req.PlayerID
	if playerID <= 0 {
		playerID = state.CurrentTurn
	}

	_, err := game.EndTurn(playerID)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Autonomous bots play their turns until it's the next human player's turn
	botLogs, nextTurn, _ := game.PlayBotTurns()

	h.json(w, http.StatusOK, map[string]interface{}{
		"next_turn":   nextTurn,
		"bot_actions": botLogs,
		"state":       game.GetState(),
	})
}
