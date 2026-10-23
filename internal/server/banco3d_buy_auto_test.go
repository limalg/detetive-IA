package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"detetive-ia/internal/banco3d"
	"detetive-ia/internal/server"
)

func TestBuyAutoPassAndDoubleRollRule(t *testing.T) {
	router := server.NewRouter(server.Config{
		Banco3DHub: banco3d.NewRoomHub(":memory:"),
	})

	// 1. Criar sala
	createBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-buy-test",
		"city":    "Curitiba",
	})
	rrCreate := httptest.NewRecorder()
	router.ServeHTTP(rrCreate, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody)))
	if rrCreate.Code != http.StatusOK {
		t.Fatalf("falha ao criar sala: %d", rrCreate.Code)
	}

	// 2. Entrar p1 e p2
	join1, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "name": "Jogador1", "color": "#3b82f6"})
	rrJoin1 := httptest.NewRecorder()
	router.ServeHTTP(rrJoin1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join1)))
	var j1 struct {
		Player struct{ ID string } `json:"player"`
	}
	_ = json.Unmarshal(rrJoin1.Body.Bytes(), &j1)
	p1ID := j1.Player.ID

	join2, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "name": "Jogador2", "color": "#ef4444"})
	rrJoin2 := httptest.NewRecorder()
	router.ServeHTTP(rrJoin2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2)))
	var j2 struct {
		Player struct{ ID string } `json:"player"`
	}
	_ = json.Unmarshal(rrJoin2.Body.Bytes(), &j2)
	p2ID := j2.Player.ID

	// 3. Iniciar jogo
	startBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p1ID})
	rrStart := httptest.NewRecorder()
	router.ServeHTTP(rrStart, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startBody)))
	if rrStart.Code != http.StatusOK {
		t.Fatalf("falha ao iniciar: %d", rrStart.Code)
	}

	// 4. Cenário A: p1 rola SEM dupla (d1=1, d2=2 -> soma 3, casa 3 é comprável)
	rollBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p1ID, "d1": 1, "d2": 2})
	rrRoll := httptest.NewRecorder()
	router.ServeHTTP(rrRoll, httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollBody)))
	if rrRoll.Code != http.StatusOK {
		t.Fatalf("falha ao rolar: %d", rrRoll.Code)
	}

	// 5. p1 compra a casa 3
	buyBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p1ID})
	rrBuy := httptest.NewRecorder()
	router.ServeHTTP(rrBuy, httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyBody)))
	if rrBuy.Code != http.StatusOK {
		t.Fatalf("falha ao comprar: %d - %s", rrBuy.Code, rrBuy.Body.String())
	}

	var stateAfterBuy struct {
		Game struct {
			CurrentTurnPlayerID string `json:"current_turn_player_id"`
		} `json:"game"`
	}
	_ = json.Unmarshal(rrBuy.Body.Bytes(), &stateAfterBuy)

	// Regra nova: sem dupla, turno passa AUTOMATICAMENTE para o próximo (p2)
	if stateAfterBuy.Game.CurrentTurnPlayerID != p2ID {
		t.Errorf("esperava que o turno passasse automaticamente para p2ID (%s), mas está com %s", p2ID, stateAfterBuy.Game.CurrentTurnPlayerID)
	}

	// 6. Cenário B: Agora é a vez de p2. p2 rola COM DUPLA (d1=2, d2=2 -> soma 4? casa 4 é imposto, vamos fazer d1=3, d2=3 -> soma 6, casa 6 é comprável)
	rollDoubleBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p2ID, "d1": 3, "d2": 3})
	rrRollDouble := httptest.NewRecorder()
	router.ServeHTTP(rrRollDouble, httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollDoubleBody)))
	if rrRollDouble.Code != http.StatusOK {
		t.Fatalf("falha ao rolar dupla para p2: %d", rrRollDouble.Code)
	}

	// p2 compra a casa 6
	buyDoubleBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p2ID})
	rrBuyDouble := httptest.NewRecorder()
	router.ServeHTTP(rrBuyDouble, httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyDoubleBody)))
	if rrBuyDouble.Code != http.StatusOK {
		t.Fatalf("falha ao comprar casa 6 para p2: %d - %s", rrBuyDouble.Code, rrBuyDouble.Body.String())
	}

	var stateAfterDoubleBuy struct {
		Game struct {
			CurrentTurnPlayerID string `json:"current_turn_player_id"`
			DiceRolledThisTurn  bool   `json:"dice_rolled_this_turn"`
		} `json:"game"`
	}
	_ = json.Unmarshal(rrBuyDouble.Body.Bytes(), &stateAfterDoubleBuy)

	// Regra nova: com DUPLA, p2 joga de novo! O turno NÃO pode passar!
	if stateAfterDoubleBuy.Game.CurrentTurnPlayerID != p2ID {
		t.Errorf("ao tirar dupla e comprar, o turno deveria continuar com p2 (%s), mas foi para %s", p2ID, stateAfterDoubleBuy.Game.CurrentTurnPlayerID)
	}
	if stateAfterDoubleBuy.Game.DiceRolledThisTurn {
		t.Errorf("ao tirar dupla, DiceRolledThisTurn deve ser resetado para false para permitir novo roll")
	}

	// p2 deve conseguir rolar novamente agora!
	rollAgainBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-buy-test", "player_id": p2ID, "d1": 1, "d2": 2})
	rrRollAgain := httptest.NewRecorder()
	router.ServeHTTP(rrRollAgain, httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollAgainBody)))
	if rrRollAgain.Code != http.StatusOK {
		t.Fatalf("p2 deveria conseguir rolar novamente apos dupla: %d - %s", rrRollAgain.Code, rrRollAgain.Body.String())
	}
}

func TestPendingDecisionBuyDoesNotExpireFromTurnInactiveTimeout(t *testing.T) {
	hub := banco3d.NewRoomHub(":memory:")
	room, err := hub.CreateRoom("TIMEOUT-BUY-ROOM", "Curitiba")
	if err != nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}
	p1, _ := room.JoinPlayer("P1", "#3b82f6", "car")
	_, _ = room.JoinPlayer("P2", "#ef4444", "hat")
	_ = room.StartGame()

	// Simula que o turno começou há 35 segundos (tempo inativo normal expiraria)
	room.Game.TurnStartTime = time.Now().Add(-35 * time.Second)
	// Mas há uma decisão de compra aberta (restam 6 segundos)
	deadline := time.Now().Add(6 * time.Second)
	room.PendingDecision = "buy"
	room.DecisionDeadline = &deadline
	room.DecisionPlayerID = p1.ID

	// CheckTurnTimeout chamado pelo polling
	expired := room.CheckTurnTimeout()
	if expired {
		t.Errorf("CheckTurnTimeout não deve expirar se a janela de compra de 8s/12s ainda está aberta!")
	}
	if room.Game.CurrentTurnPlayerID != p1.ID {
		t.Errorf("vez ainda deve ser do p1 durante a decisão de compra")
	}
}
