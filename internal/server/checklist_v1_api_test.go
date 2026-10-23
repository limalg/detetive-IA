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

func TestBuyTurnSecurityAndTimeoutValidation(t *testing.T) {
	hub := banco3d.NewRoomHub(":memory:")
	h := server.NewHandler(server.Config{Banco3DHub: hub})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/banco3d/room/create", h.HandleBanco3DRoomCreate)
	mux.HandleFunc("/api/banco3d/room/join", h.HandleBanco3DRoomJoin)
	mux.HandleFunc("/api/banco3d/room/start", h.HandleBanco3DRoomStart)
	mux.HandleFunc("/api/banco3d/room/roll", h.HandleBanco3DRoomRoll)
	mux.HandleFunc("/api/banco3d/room/buy", h.HandleBanco3DRoomBuy)
	mux.HandleFunc("/api/banco3d/room/leave", h.HandleBanco3DRoomLeave)

	// 1. Criar sala
	createBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-sec-test",
		"city":    "Curitiba",
	})
	rrCreate := httptest.NewRecorder()
	mux.ServeHTTP(rrCreate, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody)))

	// 2. Entrar Alice (p1) e Bob (p2)
	join1Body, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-sec-test", "name": "Alice", "color": "#3b82f6", "piece": "car",
	})
	rrJoin1 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join1Body)))
	var j1 struct{ Player struct{ ID string } }
	_ = json.Unmarshal(rrJoin1.Body.Bytes(), &j1)
	p1ID := j1.Player.ID

	join2Body, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-sec-test", "name": "Bob", "color": "#ef4444", "piece": "hat",
	})
	rrJoin2 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2Body)))
	var j2 struct{ Player struct{ ID string } }
	_ = json.Unmarshal(rrJoin2.Body.Bytes(), &j2)
	p2ID := j2.Player.ID

	// 3. Iniciar
	startBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-sec-test", "player_id": p1ID})
	rrStart := httptest.NewRecorder()
	mux.ServeHTTP(rrStart, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startBody)))

	// Vez é de p1 (Alice). Bob tenta comprar fora da vez -> DEVE RETORNAR 403!
	buyBobBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-sec-test", "player_id": p2ID})
	rrBuyBob := httptest.NewRecorder()
	mux.ServeHTTP(rrBuyBob, httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyBobBody)))
	if rrBuyBob.Code != http.StatusForbidden {
		t.Errorf("buy fora da vez deve retornar 403 Forbidden, obtido %d", rrBuyBob.Code)
	}

	// Alice rola dados (1+2=3, caindo na casa 3 que é comprável)
	rollBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-sec-test", "player_id": p1ID, "d1": 1, "d2": 2})
	rrRoll := httptest.NewRecorder()
	mux.ServeHTTP(rrRoll, httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollBody)))
	if rrRoll.Code != http.StatusOK {
		t.Fatalf("erro ao rolar: %d", rrRoll.Code)
	}

	// Simula timeout da decisão de compra expirado no passado
	room, _ := hub.GetRoom("sala-sec-test")
	past := time.Now().Add(-1 * time.Second)
	room.DecisionDeadline = &past

	// Alice tenta comprar após timeout expirado -> DEVE DAR ERRO!
	buyAliceBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-sec-test", "player_id": p1ID})
	rrBuyAlice := httptest.NewRecorder()
	mux.ServeHTTP(rrBuyAlice, httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyAliceBody)))
	if rrBuyAlice.Code == http.StatusOK {
		t.Errorf("buy após timeout de decisão NÃO deve ser permitido!")
	}

	// Testar POST /api/banco3d/room/leave
	leaveBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-sec-test", "player_id": p1ID})
	rrLeave := httptest.NewRecorder()
	mux.ServeHTTP(rrLeave, httptest.NewRequest("POST", "/api/banco3d/room/leave", bytes.NewReader(leaveBody)))
	if rrLeave.Code != http.StatusOK {
		t.Errorf("leave deve retornar 200 OK, obtido %d", rrLeave.Code)
	}

	var stateResp struct {
		Game struct {
			IsOver   bool   `json:"is_over"`
			WinnerID string `json:"winner_id"`
		} `json:"game"`
	}
	_ = json.Unmarshal(rrLeave.Body.Bytes(), &stateResp)
	if !stateResp.Game.IsOver {
		t.Errorf("após saída de p1 restando apenas p2 sem bots, o jogo deve encerrar")
	}
	if stateResp.Game.WinnerID != p2ID {
		t.Errorf("vencedor deveria ser p2 (%s), obtido %s", p2ID, stateResp.Game.WinnerID)
	}
}

func TestBuildEndpointTurnValidationAndEvent(t *testing.T) {
	hub := banco3d.NewRoomHub(":memory:")
	h := server.NewHandler(server.Config{Banco3DHub: hub})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/banco3d/room/create", h.HandleBanco3DRoomCreate)
	mux.HandleFunc("/api/banco3d/room/join", h.HandleBanco3DRoomJoin)
	mux.HandleFunc("/api/banco3d/room/start", h.HandleBanco3DRoomStart)
	mux.HandleFunc("/api/banco3d/room/build", h.HandleBanco3DRoomBuild)

	// 1. Criar sala
	createBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-bld-api", "city": "Curitiba"})
	rrCreate := httptest.NewRecorder()
	mux.ServeHTTP(rrCreate, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody)))

	// 2. Entrar Alice (p1) e Bob (p2)
	join1Body, _ := json.Marshal(map[string]interface{}{"room_id": "sala-bld-api", "name": "Alice", "color": "#3b82f6", "piece": "car"})
	rrJoin1 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join1Body)))
	var j1 struct{ Player struct{ ID string } }
	_ = json.Unmarshal(rrJoin1.Body.Bytes(), &j1)
	p1ID := j1.Player.ID

	join2Body, _ := json.Marshal(map[string]interface{}{"room_id": "sala-bld-api", "name": "Bob", "color": "#ef4444", "piece": "hat"})
	rrJoin2 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2Body)))
	var j2 struct{ Player struct{ ID string } }
	_ = json.Unmarshal(rrJoin2.Body.Bytes(), &j2)
	p2ID := j2.Player.ID

	// 3. Iniciar
	startBody, _ := json.Marshal(map[string]interface{}{"room_id": "sala-bld-api", "player_id": p1ID})
	rrStart := httptest.NewRecorder()
	mux.ServeHTTP(rrStart, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startBody)))

	// Dá grupo completo (casas 1 e 3) para Alice
	room, _ := hub.GetRoom("sala-bld-api")
	room.Game.Board.Tiles[1].OwnerID = p1ID
	room.Game.Board.Tiles[3].OwnerID = p1ID
	p1 := room.Game.GetPlayer(p1ID)
	p1.Properties = []int{1, 3}

	// Vez é de p1 (Alice). Bob tenta construir fora da vez -> DEVE RETORNAR 403 Forbidden!
	bldBobBody, _ := json.Marshal(map[string]interface{}{
		"room_id":    "sala-bld-api",
		"player_id":  p2ID,
		"tile_index": 1,
	})
	rrBldBob := httptest.NewRecorder()
	mux.ServeHTTP(rrBldBob, httptest.NewRequest("POST", "/api/banco3d/room/build", bytes.NewReader(bldBobBody)))
	if rrBldBob.Code != http.StatusForbidden {
		t.Errorf("build fora da vez deve retornar 403 Forbidden, obtido %d", rrBldBob.Code)
	}

	// Alice constrói na vez dela -> DEVE RETORNAR 200 OK!
	bldAliceBody, _ := json.Marshal(map[string]interface{}{
		"room_id":    "sala-bld-api",
		"player_id":  p1ID,
		"tile_index": 1,
	})
	rrBldAlice := httptest.NewRecorder()
	mux.ServeHTTP(rrBldAlice, httptest.NewRequest("POST", "/api/banco3d/room/build", bytes.NewReader(bldAliceBody)))
	if rrBldAlice.Code != http.StatusOK {
		t.Fatalf("build de Alice deveria retornar 200 OK, obtido %d: %s", rrBldAlice.Code, rrBldAlice.Body.String())
	}

	// Verificar se emitiu evento type='build' com level=1
	var buildResp struct {
		Events []struct {
			Type     string `json:"type"`
			PlayerID string `json:"player_id"`
			Tile     int    `json:"tile"`
			Level    int    `json:"level"`
		} `json:"events"`
	}
	_ = json.Unmarshal(rrBldAlice.Body.Bytes(), &buildResp)
	var foundBuildEvent bool
	for _, ev := range buildResp.Events {
		if ev.Type == "build" && ev.PlayerID == p1ID && ev.Tile == 1 && ev.Level == 1 {
			foundBuildEvent = true
			break
		}
	}
	if !foundBuildEvent {
		t.Errorf("evento build com player_id, tile e level não encontrado nos eventos da resposta")
	}
}
