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
		"room_id":       room.ID,
		"city":          room.City,
		"config":        room.Config,
		"board":         room.Board,
		"candidates":    room.Candidates,
		"game":          room.Game,
		"voting":        room.Voting,
		"pending_trade": room.PendingTrade,
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
		RoomID string              `json:"room_id"`
		City   string              `json:"city"`
		Config *banco3d.RoomConfig `json:"config,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if strings.TrimSpace(req.RoomID) == "" {
		req.RoomID = "sala-1"
	}
	if strings.TrimSpace(req.City) == "" {
		req.City = "São Paulo"
	}

	hub := h.getBanco3DHub()
	room, err := hub.GetOrCreateRoom(req.RoomID, req.City, req.Config)
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

// HandleBanco3DRoomBuild constructs a house/hotel on an owned property.
func (h *Handler) HandleBanco3DRoomBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID    string `json:"room_id"`
		PlayerID  string `json:"player_id"`
		TileIndex int    `json:"tile_index"`
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

	if err := room.Game.BuildHouse(req.PlayerID, req.TileIndex); err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}

// HandleBanco3DRoomTrade executes or negotiates property transfers between players.
func (h *Handler) HandleBanco3DRoomTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID         string `json:"room_id"`
		Action         string `json:"action"` // "propose", "accept", "decline", or "" (immediate)
		SellerID       string `json:"seller_id"`
		BuyerID        string `json:"buyer_id"`
		TileIndex      int    `json:"tile_index"`
		CashAmount     int    `json:"cash_amount"`
		SenderID       string `json:"sender_id"`
		ReceiverID     string `json:"receiver_id"`
		OfferedTiles   []int  `json:"offered_tiles"`
		OfferedCash    int    `json:"offered_cash"`
		RequestedTiles []int  `json:"requested_tiles"`
		RequestedCash  int    `json:"requested_cash"`

		// Pixel proposed format
		PlayerID   string `json:"player_id"`
		ToPlayerID string `json:"to_player_id"`
		Give       struct {
			Props []int `json:"props"`
			Cash  int   `json:"cash"`
		} `json:"give"`
		Want struct {
			Props []int `json:"props"`
			Cash  int   `json:"cash"`
		} `json:"want"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	// Normalizar campos Pixel -> internal
	if req.PlayerID != "" && req.ToPlayerID != "" {
		req.SenderID = req.PlayerID
		req.ReceiverID = req.ToPlayerID
		req.OfferedTiles = req.Give.Props
		req.OfferedCash = req.Give.Cash
		req.RequestedTiles = req.Want.Props
		req.RequestedCash = req.Want.Cash
	}

	hub := h.getBanco3DHub()
	room, ok := hub.GetRoom(req.RoomID)
	if !ok {
		h.error(w, http.StatusNotFound, "room not found")
		return
	}

	callerID := req.PlayerID
	if callerID == "" {
		callerID = req.SenderID
	}

	var tradeErr error
	switch req.Action {
	case "accept":
		if room.PendingTrade != nil && room.PendingTrade.ReceiverID != callerID {
			h.error(w, http.StatusForbidden, "apenas o destinatário pode aceitar ou recusar a proposta")
			return
		}
		tradeErr = room.AcceptTrade(callerID)
	case "decline":
		if room.PendingTrade != nil && room.PendingTrade.ReceiverID != callerID {
			h.error(w, http.StatusForbidden, "apenas o destinatário pode aceitar ou recusar a proposta")
			return
		}
		tradeErr = room.DeclineTrade(callerID)
	case "propose":
		proposal := banco3d.TradeProposal{
			ID:             "trade-1",
			SenderID:       req.SenderID,
			ReceiverID:     req.ReceiverID,
			OfferedTiles:   req.OfferedTiles,
			OfferedCash:    req.OfferedCash,
			RequestedTiles: req.RequestedTiles,
			RequestedCash:  req.RequestedCash,
		}
		tradeErr = room.ProposeTrade(proposal)
		// Se o destinatário for bot legítimo (IsBot == true), auto-aceita
		if tradeErr == nil {
			var isBot bool
			for _, p := range room.Game.Players {
				if p.ID == req.ReceiverID {
					isBot = p.IsBot
					break
				}
			}
			if isBot {
				_ = room.AcceptTrade(req.ReceiverID)
			}
		}
	default:
		// Execução direta imediata
		if len(req.OfferedTiles) > 0 || len(req.RequestedTiles) > 0 || req.SenderID != "" {
			tradeErr = room.Game.TradeExchange(banco3d.TradeExchangeRequest{
				SenderID:       req.SenderID,
				ReceiverID:     req.ReceiverID,
				OfferedTiles:   req.OfferedTiles,
				OfferedCash:    req.OfferedCash,
				RequestedTiles: req.RequestedTiles,
				RequestedCash:  req.RequestedCash,
			})
		} else {
			tradeErr = room.Game.TradeProperties(req.SellerID, req.BuyerID, req.TileIndex, req.CashAmount)
		}
	}

	if tradeErr != nil {
		h.error(w, http.StatusBadRequest, tradeErr.Error())
		return
	}

	h.json(w, http.StatusOK, h.formatRoomState(room, nil))
}
