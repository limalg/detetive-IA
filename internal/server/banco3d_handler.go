package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"detetive-ia/internal/banco3d"
)

func (h *Handler) getBanco3DHub() *banco3d.RoomHub {
	if h.cfg.Banco3DHub == nil {
		h.cfg.Banco3DHub = banco3d.NewRoomHub()
	}
	return h.cfg.Banco3DHub
}

func (h *Handler) formatRoomState(room *banco3d.Room, dice *banco3d.DiceResult) map[string]interface{} {
	room.CheckTurnTimeout()
	res := map[string]interface{}{
		"room_id": room.ID,
		"city":    room.City,
		"board":   room.Board,
		"game":    room.Game,
		"voting":  room.Voting,
	}
	if dice != nil {
		res["dice"] = dice
	}
	return res
}

// HandleBanco3DCity returns the 40-tile board and 4 iconic candidates for a city query.
func (h *Handler) HandleBanco3DCity(w http.ResponseWriter, r *http.Request) {
	cityQuery := r.URL.Query().Get("q")
	if cityQuery == "" {
		cityQuery = r.URL.Query().Get("city")
	}

	if r.Method == http.MethodPost {
		var body struct {
			City string `json:"city"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.City != "" {
			cityQuery = body.City
		}
	}

	if strings.TrimSpace(cityQuery) == "" {
		cityQuery = "Curitiba, PR"
	}

	board, candidates, err := banco3d.GenerateCityBoard(cityQuery, "")
	if err != nil {
		h.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"city":       board.City,
		"tiles":      board.Tiles,
		"candidates": candidates,
	})
}

// HandleBanco3DRoomCreate creates a new multiplayer room.
func (h *Handler) HandleBanco3DRoomCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
		City   string `json:"city"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if strings.TrimSpace(req.RoomID) == "" {
		req.RoomID = "sala-1"
	}
	if strings.TrimSpace(req.City) == "" {
		req.City = "São Paulo"
	}

	hub := h.getBanco3DHub()
	room, err := hub.GetOrCreateRoom(req.RoomID, req.City)
	if err != nil {
		h.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"room_id":    room.ID,
		"city":       room.City,
		"candidates": room.Candidates,
	})
}

// HandleBanco3DRoomJoin adds or reconnects a player to the room.
func (h *Handler) HandleBanco3DRoomJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID   string `json:"room_id"`
		PlayerID string `json:"player_id"`
		Name     string `json:"name"`
		Color    string `json:"color"`
		Piece    string `json:"piece"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	if req.PlayerID != "" {
		p, err := room.ReconnectPlayer(req.PlayerID)
		if err == nil {
			h.json(w, http.StatusOK, map[string]interface{}{
				"reconnected": true,
				"player":      p,
			})
			return
		}
	}

	if req.Name == "" {
		req.Name = "Jogador"
	}
	p, err := room.JoinPlayer(req.Name, req.Color, req.Piece)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"reconnected": false,
		"player":      p,
	})
}

// HandleBanco3DRoomVote casts a vote in the 15s iconic location session.
func (h *Handler) HandleBanco3DRoomVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID    string `json:"room_id"`
		PlayerID  string `json:"player_id"`
		Candidate int    `json:"candidate"`
		Action    string `json:"action"` // "start", "vote", "close"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	switch req.Action {
	case "start":
		if err := room.StartVoting(); err != nil {
			h.error(w, http.StatusBadRequest, err.Error())
			return
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"status":     "voting_started",
			"candidates": room.Voting.Candidates,
			"deadline":   room.Voting.Deadline,
		})
	case "close":
		winner, err := room.ConcludeVoting()
		if err != nil {
			h.error(w, http.StatusBadRequest, err.Error())
			return
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"status": "voting_concluded",
			"winner": winner,
		})
	default:
		if err := room.CastVote(req.PlayerID, req.Candidate); err != nil {
			h.error(w, http.StatusBadRequest, err.Error())
			return
		}
		h.json(w, http.StatusOK, map[string]interface{}{
			"status": "vote_recorded",
		})
	}
}

// HandleBanco3DRoomState returns the complete room, voting and game state.
func (h *Handler) HandleBanco3DRoomState(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		h.error(w, http.StatusBadRequest, "room_id required")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(roomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}

// HandleBanco3DRoomStart starts the match.
func (h *Handler) HandleBanco3DRoomStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID   string `json:"room_id"`
		PlayerID string `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	if err := room.StartGame(); err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}

// HandleBanco3DRoomRoll rolls dice for the player.
func (h *Handler) HandleBanco3DRoomRoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID   string `json:"room_id"`
		PlayerID string `json:"player_id"`
		D1       int    `json:"d1"`
		D2       int    `json:"d2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	dice, err := room.Game.RollDice(req.PlayerID, req.D1, req.D2)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, dice))
}

// HandleBanco3DRoomBuy purchases property at player's position.
func (h *Handler) HandleBanco3DRoomBuy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID   string `json:"room_id"`
		PlayerID string `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	if err := room.Game.BuyProperty(req.PlayerID); err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}

// HandleBanco3DRoomPass passes the turn.
func (h *Handler) HandleBanco3DRoomPass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID   string `json:"room_id"`
		PlayerID string `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	if err := room.Game.PassTurn(req.PlayerID); err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}
