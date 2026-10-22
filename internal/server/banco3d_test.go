package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"detetive-ia/internal/server"
)

func TestBanco3DEndpoints(t *testing.T) {
	router := server.NewRouter(server.Config{})

	// 1. GET /api/banco3d/city
	req := httptest.NewRequest("GET", "/api/banco3d/city?q=Curitiba", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var cityResp struct {
		City       string        `json:"city"`
		Tiles      []interface{} `json:"tiles"`
		Candidates []string      `json:"candidates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &cityResp); err != nil {
		t.Fatalf("failed to decode city response: %v", err)
	}
	if len(cityResp.Tiles) != 40 {
		t.Errorf("expected 40 tiles, got %d", len(cityResp.Tiles))
	}
	if len(cityResp.Candidates) != 4 {
		t.Errorf("expected 4 candidates, got %d", len(cityResp.Candidates))
	}

	// 2. POST /api/banco3d/room/create
	createPayload := []byte(`{"room_id":"sala-curitiba","city":"Curitiba"}`)
	reqCreate := httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createPayload))
	rrCreate := httptest.NewRecorder()
	router.ServeHTTP(rrCreate, reqCreate)

	if rrCreate.Code != http.StatusOK {
		t.Fatalf("expected 200 creating room, got %d", rrCreate.Code)
	}

	// 3. POST /api/banco3d/room/join (p1 e p2)
	joinPayload1 := []byte(`{"room_id":"sala-curitiba","name":"Lucas","color":"#3b82f6","piece":"car"}`)
	reqJoin1 := httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(joinPayload1))
	rrJoin1 := httptest.NewRecorder()
	router.ServeHTTP(rrJoin1, reqJoin1)
	if rrJoin1.Code != http.StatusOK {
		t.Fatalf("expected 200 joining room p1, got %d: %s", rrJoin1.Code, rrJoin1.Body.String())
	}

	var joinResp1 struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rrJoin1.Body.Bytes(), &joinResp1)
	p1ID := joinResp1.Player.ID

	joinPayload2 := []byte(`{"room_id":"sala-curitiba","name":"Ana","color":"#ef4444","piece":"hat"}`)
	reqJoin2 := httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(joinPayload2))
	rrJoin2 := httptest.NewRecorder()
	router.ServeHTTP(rrJoin2, reqJoin2)
	if rrJoin2.Code != http.StatusOK {
		t.Fatalf("expected 200 joining room p2, got %d: %s", rrJoin2.Code, rrJoin2.Body.String())
	}

	var joinResp2 struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rrJoin2.Body.Bytes(), &joinResp2)
	p2ID := joinResp2.Player.ID

	// 4. Iniciar votação e votar
	voteStartPayload := []byte(`{"room_id":"sala-curitiba","action":"start"}`)
	reqVoteStart := httptest.NewRequest("POST", "/api/banco3d/room/vote", bytes.NewReader(voteStartPayload))
	rrVoteStart := httptest.NewRecorder()
	router.ServeHTTP(rrVoteStart, reqVoteStart)
	if rrVoteStart.Code != http.StatusOK {
		t.Fatalf("expected 200 starting vote, got %d", rrVoteStart.Code)
	}

	votePayload, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-curitiba",
		"player_id": p1ID,
		"candidate": 1,
		"action":    "vote",
	})
	reqVote := httptest.NewRequest("POST", "/api/banco3d/room/vote", bytes.NewReader(votePayload))
	rrVote := httptest.NewRecorder()
	router.ServeHTTP(rrVote, reqVote)
	if rrVote.Code != http.StatusOK {
		t.Fatalf("expected 200 casting vote, got %d", rrVote.Code)
	}

	// Concluir votação
	voteClosePayload := []byte(`{"room_id":"sala-curitiba","action":"close"}`)
	reqVoteClose := httptest.NewRequest("POST", "/api/banco3d/room/vote", bytes.NewReader(voteClosePayload))
	rrVoteClose := httptest.NewRecorder()
	router.ServeHTTP(rrVoteClose, reqVoteClose)
	if rrVoteClose.Code != http.StatusOK {
		t.Fatalf("expected 200 closing vote, got %d", rrVoteClose.Code)
	}

	// 5. GET /api/banco3d/room/state (deve conter voting e board atualizado)
	reqState := httptest.NewRequest("GET", "/api/banco3d/room/state?room_id=sala-curitiba", nil)
	rrState := httptest.NewRecorder()
	router.ServeHTTP(rrState, reqState)
	if rrState.Code != http.StatusOK {
		t.Fatalf("expected 200 getting state, got %d", rrState.Code)
	}

	var stateResp struct {
		Voting interface{} `json:"voting"`
		Board  struct {
			Tiles []struct {
				Name    string `json:"name"`
				IsQueen bool   `json:"is_queen"`
			} `json:"tiles"`
		} `json:"board"`
	}
	if err := json.Unmarshal(rrState.Body.Bytes(), &stateResp); err != nil {
		t.Fatalf("failed to decode state: %v", err)
	}
	if stateResp.Voting == nil {
		t.Errorf("voting must be present in room/state")
	}
	if !stateResp.Board.Tiles[39].IsQueen {
		t.Errorf("tile 39 must be Queen")
	}

	// 6. POST /api/banco3d/room/start
	startPayload := []byte(`{"room_id":"sala-curitiba","player_id":"` + p1ID + `"}`)
	reqStart := httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startPayload))
	rrStart := httptest.NewRecorder()
	router.ServeHTTP(rrStart, reqStart)
	if rrStart.Code != http.StatusOK {
		t.Fatalf("expected 200 starting match, got %d: %s", rrStart.Code, rrStart.Body.String())
	}

	// 7. POST /api/banco3d/room/roll
	rollPayload := []byte(`{"room_id":"sala-curitiba","player_id":"` + p1ID + `","d1":0,"d2":1}`)
	reqRoll := httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollPayload))
	rrRoll := httptest.NewRecorder()
	router.ServeHTTP(rrRoll, reqRoll)
	if rrRoll.Code != http.StatusOK {
		t.Fatalf("expected 200 rolling dice, got %d: %s", rrRoll.Code, rrRoll.Body.String())
	}

	var rollResp struct {
		Dice struct {
			Total       int `json:"total"`
			NewPosition int `json:"new_position"`
		} `json:"dice"`
	}
	if err := json.Unmarshal(rrRoll.Body.Bytes(), &rollResp); err != nil {
		t.Fatalf("failed to parse roll response: %v", err)
	}
	if rollResp.Dice.NewPosition != 1 {
		t.Errorf("expected position 1, got %d", rollResp.Dice.NewPosition)
	}

	// 8. POST /api/banco3d/room/buy
	buyPayload := []byte(`{"room_id":"sala-curitiba","player_id":"` + p1ID + `"}`)
	reqBuy := httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyPayload))
	rrBuy := httptest.NewRecorder()
	router.ServeHTTP(rrBuy, reqBuy)
	if rrBuy.Code != http.StatusOK {
		t.Fatalf("expected 200 buying property, got %d: %s", rrBuy.Code, rrBuy.Body.String())
	}

	// 9. POST /api/banco3d/room/pass
	passPayload := []byte(`{"room_id":"sala-curitiba","player_id":"` + p1ID + `"}`)
	reqPass := httptest.NewRequest("POST", "/api/banco3d/room/pass", bytes.NewReader(passPayload))
	rrPass := httptest.NewRecorder()
	router.ServeHTTP(rrPass, reqPass)
	if rrPass.Code != http.StatusOK {
		t.Fatalf("expected 200 passing turn, got %d: %s", rrPass.Code, rrPass.Body.String())
	}

	// 10. POST /api/banco3d/room/trade
	tradePayload, _ := json.Marshal(map[string]interface{}{
		"room_id":     "sala-curitiba",
		"seller_id":   p1ID,
		"buyer_id":    p2ID,
		"tile_index":  1,
		"cash_amount": 100,
	})
	reqTrade := httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(tradePayload))
	rrTrade := httptest.NewRecorder()
	router.ServeHTTP(rrTrade, reqTrade)
	if rrTrade.Code != http.StatusOK {
		t.Fatalf("expected 200 trading, got %d: %s", rrTrade.Code, rrTrade.Body.String())
	}
}

func TestBanco3DCustomConfigAndFullTrade(t *testing.T) {
	router := server.NewRouter(server.Config{})

	// 1. Criar sala com configuração personalizada (R$ 2.500 de saldo, 2 bots)
	createBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-custom-sp",
		"city":    "São Paulo",
		"config": map[string]interface{}{
			"max_players":      4,
			"starting_balance": 2500,
			"turn_seconds":     45,
			"bots": map[string]interface{}{
				"count":      2,
				"difficulty": "hard",
			},
		},
	})
	reqCreate := httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	router.ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusOK {
		t.Fatalf("falha ao criar sala customizada: %d", rrCreate.Code)
	}

	// 2. Entrar jogador 1 e jogador 2
	join1, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-custom-sp",
		"name":    "Roberto",
		"color":   "#2563eb",
	})
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join1)))
	if rr1.Code != http.StatusOK {
		t.Fatalf("falha ao entrar p1: %d", rr1.Code)
	}
	var p1Res struct {
		Player struct {
			ID    string `json:"id"`
			Money int    `json:"money"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rr1.Body.Bytes(), &p1Res)
	if p1Res.Player.Money != 2500 {
		t.Errorf("jogador deveria iniciar com saldo customizado de R$ 2500, recebeu %d", p1Res.Player.Money)
	}

	join2, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-custom-sp",
		"name":    "Mariana",
		"color":   "#dc2626",
	})
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2)))
	var p2Res struct {
		Player struct {
			ID    string `json:"id"`
			Money int    `json:"money"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rr2.Body.Bytes(), &p2Res)

	// 3. Testar troca completa (TradeExchange) entre p1 e p2
	p1ID := p1Res.Player.ID
	p2ID := p2Res.Player.ID

	tradeExchangeBody, _ := json.Marshal(map[string]interface{}{
		"room_id":         "sala-custom-sp",
		"sender_id":       p1ID,
		"receiver_id":     p2ID,
		"offered_tiles":   []int{},
		"offered_cash":    200,
		"requested_tiles": []int{},
		"requested_cash":  50,
	})
	rrTrade := httptest.NewRecorder()
	router.ServeHTTP(rrTrade, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(tradeExchangeBody)))
	if rrTrade.Code != http.StatusOK {
		t.Fatalf("falha no trade exchange completo: %d - %s", rrTrade.Code, rrTrade.Body.String())
	}
}
