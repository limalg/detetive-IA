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

	// 3. POST /api/banco3d/room/join
	joinPayload := []byte(`{"room_id":"sala-curitiba","name":"Lucas","color":"#3b82f6","piece":"car"}`)
	reqJoin := httptest.NewRequest("POST", "/api/banco3d/room/join", bytes.NewReader(joinPayload))
	rrJoin := httptest.NewRecorder()
	router.ServeHTTP(rrJoin, reqJoin)

	if rrJoin.Code != http.StatusOK {
		t.Fatalf("expected 200 joining room, got %d: %s", rrJoin.Code, rrJoin.Body.String())
	}

	var joinResp struct {
		Player struct {
			ID string `json:"id"`
		} `json:"player"`
	}
	_ = json.Unmarshal(rrJoin.Body.Bytes(), &joinResp)
	pID := joinResp.Player.ID

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
		"player_id": pID,
		"candidate": 1,
		"action":    "vote",
	})
	reqVote := httptest.NewRequest("POST", "/api/banco3d/room/vote", bytes.NewReader(votePayload))
	rrVote := httptest.NewRecorder()
	router.ServeHTTP(rrVote, reqVote)
	if rrVote.Code != http.StatusOK {
		t.Fatalf("expected 200 casting vote, got %d", rrVote.Code)
	}

	// 5. GET /api/banco3d/room/state
	reqState := httptest.NewRequest("GET", "/api/banco3d/room/state?room_id=sala-curitiba", nil)
	rrState := httptest.NewRecorder()
	router.ServeHTTP(rrState, reqState)
	if rrState.Code != http.StatusOK {
		t.Fatalf("expected 200 getting state, got %d", rrState.Code)
	}
}
