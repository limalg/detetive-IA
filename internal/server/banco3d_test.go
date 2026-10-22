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

func TestTradeHTTPRejectionUnauthorizedAndHumanConsent(t *testing.T) {
	router := server.NewRouter(server.Config{})

	// 1. Criar sala com 2 humanos
	createBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-trade-auth",
		"city":    "Rio de Janeiro",
	})
	rrCreate := httptest.NewRecorder()
	router.ServeHTTP(rrCreate, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody)))
	if rrCreate.Code != http.StatusOK {
		t.Fatalf("falha ao criar sala: %d", rrCreate.Code)
	}

	join1, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-trade-auth",
		"name":    "Alice",
		"color":   "#3b82f6",
	})
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join1)))
	var p1Res struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rr1.Body.Bytes(), &p1Res)
	p1ID := p1Res.Player.ID

	join2, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-trade-auth",
		"name":    "Bob",
		"color":   "#ef4444",
	})
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2)))
	var p2Res struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rr2.Body.Bytes(), &p2Res)
	p2ID := p2Res.Player.ID

	// Iniciar jogo
	startBody, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-auth",
		"player_id": p1ID,
	})
	rrStart := httptest.NewRecorder()
	router.ServeHTTP(rrStart, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startBody)))
	if rrStart.Code != http.StatusOK {
		t.Fatalf("falha ao iniciar jogo: %d", rrStart.Code)
	}

	// 2. Proposta de p1 (Alice) para p2 (Bob)
	proposeBody, _ := json.Marshal(map[string]interface{}{
		"room_id":      "sala-trade-auth",
		"action":       "propose",
		"player_id":    p1ID,
		"to_player_id": p2ID,
		"give": map[string]interface{}{
			"props": []int{},
			"cash":  100,
		},
		"want": map[string]interface{}{
			"props": []int{},
			"cash":  0,
		},
	})
	rrProp := httptest.NewRecorder()
	router.ServeHTTP(rrProp, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(proposeBody)))
	if rrProp.Code != http.StatusOK {
		t.Fatalf("falha ao propor troca: %d - %s", rrProp.Code, rrProp.Body.String())
	}

	// Como Bob é humano, a troca NÃO pode ter sido auto-aceita!
	var stateAfterProp struct {
		PendingTrade *struct {
			Status     string `json:"status"`
			ReceiverID string `json:"receiver_id"`
		} `json:"pending_trade"`
	}
	_ = json.Unmarshal(rrProp.Body.Bytes(), &stateAfterProp)
	if stateAfterProp.PendingTrade == nil || stateAfterProp.PendingTrade.Status != "pending" {
		t.Fatalf("troca para humano não deveria ter sido auto-aceita, pending_trade: %+v", stateAfterProp.PendingTrade)
	}

	// 3. Terceiro estranho tenta aceitar -> DEVE RETORNAR 403 Forbidden
	unauthorizedAccept, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-auth",
		"action":    "accept",
		"player_id": "hacker-999",
	})
	rrUnauthAcc := httptest.NewRecorder()
	router.ServeHTTP(rrUnauthAcc, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(unauthorizedAccept)))
	if rrUnauthAcc.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 Forbidden para terceiro estranho aceitando troca, obteve %d", rrUnauthAcc.Code)
	}

	// 4. Próprio remetente tenta aceitar sua própria proposta -> DEVE RETORNAR 403 Forbidden
	senderSelfAccept, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-auth",
		"action":    "accept",
		"player_id": p1ID,
	})
	rrSelfAcc := httptest.NewRecorder()
	router.ServeHTTP(rrSelfAcc, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(senderSelfAccept)))
	if rrSelfAcc.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 Forbidden para remetente aceitando própria proposta, obteve %d", rrSelfAcc.Code)
	}

	// 5. Terceiro estranho tenta recusar -> DEVE RETORNAR 403 Forbidden
	unauthorizedDecline, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-auth",
		"action":    "decline",
		"player_id": "hacker-999",
	})
	rrUnauthDec := httptest.NewRecorder()
	router.ServeHTTP(rrUnauthDec, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(unauthorizedDecline)))
	if rrUnauthDec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 Forbidden para terceiro estranho recusando troca, obteve %d", rrUnauthDec.Code)
	}

	// 6. Destinatário legítimo (Bob / p2ID) aceita -> DEVE RETORNAR 200 OK
	legitAccept, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-auth",
		"action":    "accept",
		"player_id": p2ID,
	})
	rrLegitAcc := httptest.NewRecorder()
	router.ServeHTTP(rrLegitAcc, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(legitAccept)))
	if rrLegitAcc.Code != http.StatusOK {
		t.Fatalf("destinatário legítimo deveria conseguir aceitar a troca: %d - %s", rrLegitAcc.Code, rrLegitAcc.Body.String())
	}

	// 7. Teste de Bot legítimo sendo auto-aceito
	createBotRoom, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-trade-bot-auto",
		"city":    "Curitiba",
	})
	rrCreateBot := httptest.NewRecorder()
	router.ServeHTTP(rrCreateBot, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBotRoom)))

	joinSolo, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-trade-bot-auto",
		"name":    "SoloPlayer",
		"color":   "#3b82f6",
	})
	rrSolo := httptest.NewRecorder()
	router.ServeHTTP(rrSolo, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(joinSolo)))
	var soloRes struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rrSolo.Body.Bytes(), &soloRes)

	startSolo, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-trade-bot-auto",
		"player_id": soloRes.Player.ID,
	})
	rrStartSolo := httptest.NewRecorder()
	router.ServeHTTP(rrStartSolo, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startSolo)))

	var stateSolo struct {
		Game struct {
			Players []struct {
				ID    string `json:"id"`
				IsBot bool   `json:"is_bot"`
			} `json:"players"`
		} `json:"game"`
	}
	_ = json.Unmarshal(rrStartSolo.Body.Bytes(), &stateSolo)

	var botID string
	for _, p := range stateSolo.Game.Players {
		if p.ID != soloRes.Player.ID && p.IsBot {
			botID = p.ID
			break
		}
	}
	if botID == "" {
		t.Fatalf("esperava encontrar um bot na sala solo")
	}

	// Propor troca ao bot -> deve ser auto-aceita
	propBot, _ := json.Marshal(map[string]interface{}{
		"room_id":      "sala-trade-bot-auto",
		"action":       "propose",
		"player_id":    soloRes.Player.ID,
		"to_player_id": botID,
		"give": map[string]interface{}{
			"props": []int{},
			"cash":  50,
		},
		"want": map[string]interface{}{
			"props": []int{},
			"cash":  0,
		},
	})
	rrPropBot := httptest.NewRecorder()
	router.ServeHTTP(rrPropBot, httptest.NewRequest("POST", "/api/banco3d/room/trade", bytes.NewReader(propBot)))
	if rrPropBot.Code != http.StatusOK {
		t.Fatalf("falha ao propor troca para bot: %d - %s", rrPropBot.Code, rrPropBot.Body.String())
	}

	var stateAfterBotProp struct {
		PendingTrade *struct {
			Status string `json:"status"`
		} `json:"pending_trade"`
	}
	_ = json.Unmarshal(rrPropBot.Body.Bytes(), &stateAfterBotProp)
	if stateAfterBotProp.PendingTrade != nil {
		t.Fatalf("troca para bot legítimo deveria ter sido auto-aceita e pending_trade limpo, mas está: %+v", stateAfterBotProp.PendingTrade)
	}
}
