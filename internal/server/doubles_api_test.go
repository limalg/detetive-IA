package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"detetive-ia/internal/banco3d"
	"detetive-ia/internal/server"
)

func TestAPIRoomStateBoardHasOwnerIDAfterBuy(t *testing.T) {
	hub := banco3d.NewRoomHub(":memory:")
	h := server.NewHandler(server.Config{Banco3DHub: hub})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/banco3d/room/create", h.HandleBanco3DRoomCreate)
	mux.HandleFunc("/api/banco3d/room/join", h.HandleBanco3DRoomJoin)
	mux.HandleFunc("/api/banco3d/room/start", h.HandleBanco3DRoomStart)
	mux.HandleFunc("/api/banco3d/room/roll", h.HandleBanco3DRoomRoll)
	mux.HandleFunc("/api/banco3d/room/buy", h.HandleBanco3DRoomBuy)
	mux.HandleFunc("/api/banco3d/room/state", h.HandleBanco3DRoomState)

	// 1. Criar sala
	createBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-state-owner-test",
		"city":    "Curitiba",
	})
	rrCreate := httptest.NewRecorder()
	mux.ServeHTTP(rrCreate, httptest.NewRequest("POST", "/api/banco3d/room/create", bytes.NewReader(createBody)))
	if rrCreate.Code != http.StatusOK {
		t.Fatalf("falha ao criar sala: %d", rrCreate.Code)
	}

	// 2. Entrar dois jogadores
	joinBody, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-state-owner-test",
		"name":    "Alice",
		"color":   "#3b82f6",
		"piece":   "car",
	})
	rrJoin1 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin1, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(joinBody)))
	var join1Res struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rrJoin1.Body.Bytes(), &join1Res)
	p1ID := join1Res.Player.ID

	join2Body, _ := json.Marshal(map[string]interface{}{
		"room_id": "sala-state-owner-test",
		"name":    "Bob",
		"color":   "#ef4444",
		"piece":   "hat",
	})
	rrJoin2 := httptest.NewRecorder()
	mux.ServeHTTP(rrJoin2, httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(join2Body)))

	// 3. Iniciar sala
	startBody, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-state-owner-test",
		"player_id": p1ID,
	})
	rrStart := httptest.NewRecorder()
	mux.ServeHTTP(rrStart, httptest.NewRequest("POST", "/api/banco3d/room/start", bytes.NewReader(startBody)))
	if rrStart.Code != http.StatusOK {
		t.Fatalf("falha ao iniciar sala: %d", rrStart.Code)
	}

	// 4. Rola dados (1+2=3, caindo na casa 3 que é Brown B)
	rollBody, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-state-owner-test",
		"player_id": p1ID,
		"d1":        1,
		"d2":        2,
	})
	rrRoll := httptest.NewRecorder()
	mux.ServeHTTP(rrRoll, httptest.NewRequest("POST", "/api/banco3d/room/roll", bytes.NewReader(rollBody)))
	if rrRoll.Code != http.StatusOK {
		t.Fatalf("falha ao rolar: %d", rrRoll.Code)
	}

	// 5. Compra a propriedade
	buyBody, _ := json.Marshal(map[string]interface{}{
		"room_id":   "sala-state-owner-test",
		"player_id": p1ID,
	})
	rrBuy := httptest.NewRecorder()
	mux.ServeHTTP(rrBuy, httptest.NewRequest("POST", "/api/banco3d/room/buy", bytes.NewReader(buyBody)))
	if rrBuy.Code != http.StatusOK {
		t.Fatalf("falha ao comprar: %d", rrBuy.Code)
	}

	// 6. GET /api/banco3d/room/state: valida que o board de TOPO contém o owner_id preenchido!
	rrState := httptest.NewRecorder()
	mux.ServeHTTP(rrState, httptest.NewRequest("GET", "/api/banco3d/room/state?room_id=sala-state-owner-test", nil))
	if rrState.Code != http.StatusOK {
		t.Fatalf("falha ao obter state: %d", rrState.Code)
	}

	var stateResp struct {
		Board struct {
			Tiles []struct {
				Index   int    `json:"index"`
				OwnerID string `json:"owner_id"`
			} `json:"tiles"`
		} `json:"board"`
		Game struct {
			Board struct {
				Tiles []struct {
					Index   int    `json:"index"`
					OwnerID string `json:"owner_id"`
				} `json:"tiles"`
			} `json:"board"`
		} `json:"game"`
	}
	if err := json.Unmarshal(rrState.Body.Bytes(), &stateResp); err != nil {
		t.Fatalf("erro ao decodificar state: %v", err)
	}

	// Verifica se a casa 3 tem owner_id preenchido tanto no topo quanto em game.board
	topOwner := stateResp.Board.Tiles[3].OwnerID
	gameOwner := stateResp.Game.Board.Tiles[3].OwnerID

	if topOwner != p1ID {
		t.Errorf("E2E BUG 1: board de TOPO no /api/banco3d/room/state veio com owner_id vazio ou incorreto! Esperado '%s', obtido '%s'", p1ID, topOwner)
	}
	if gameOwner != p1ID {
		t.Errorf("game.board.tiles[3].owner_id esperado '%s', obtido '%s'", p1ID, gameOwner)
	}
}
