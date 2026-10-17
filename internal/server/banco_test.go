package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBancoEndpoints_E2EFlow(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. POST /api/banco/start
	startPayload := map[string]interface{}{
		"city":         "Curitiba",
		"age":          15,
		"school_grade": "1º EM",
		"difficulty":   "medio",
		"bots_count":   3,
	}
	body, _ := json.Marshal(startPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/banco/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/banco/start, got %d: %s", w.Code, w.Body.String())
	}

	var startRes struct {
		GameID  string                 `json:"game_id"`
		State   map[string]interface{} `json:"state"`
		Message string                 `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&startRes); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}
	if startRes.GameID == "" {
		t.Errorf("expected non-empty game_id")
	}

	// Extract session cookie if set, or pass game_id in header / param
	cookies := w.Result().Cookies()

	// 2. GET /api/banco/state
	reqState := httptest.NewRequest(http.MethodGet, "/api/banco/state?game_id="+startRes.GameID, nil)
	for _, c := range cookies {
		reqState.AddCookie(c)
	}
	wState := httptest.NewRecorder()
	router.ServeHTTP(wState, reqState)

	if wState.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/banco/state, got %d: %s", wState.Code, wState.Body.String())
	}

	var stateRes struct {
		ID      string                   `json:"id"`
		City    string                   `json:"city"`
		Board   []map[string]interface{} `json:"board"`
		Players []map[string]interface{} `json:"players"`
	}
	if err := json.NewDecoder(wState.Body).Decode(&stateRes); err != nil {
		t.Fatalf("failed to decode state response: %v", err)
	}
	if stateRes.City != "Curitiba" {
		t.Errorf("expected city Curitiba, got %s", stateRes.City)
	}
	if len(stateRes.Board) != 28 {
		t.Errorf("expected 28 board tiles, got %d", len(stateRes.Board))
	}

	// 3. POST /api/banco/roll
	rollPayload := map[string]interface{}{
		"game_id": startRes.GameID,
	}
	rollBody, _ := json.Marshal(rollPayload)
	reqRoll := httptest.NewRequest(http.MethodPost, "/api/banco/roll", bytes.NewReader(rollBody))
	reqRoll.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		reqRoll.AddCookie(c)
	}
	wRoll := httptest.NewRecorder()
	router.ServeHTTP(wRoll, reqRoll)

	if wRoll.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/banco/roll, got %d: %s", wRoll.Code, wRoll.Body.String())
	}

	var rollRes struct {
		Dice    [2]int                 `json:"dice"`
		Total   int                    `json:"total"`
		Message string                 `json:"message"`
		State   map[string]interface{} `json:"state"`
	}
	if err := json.NewDecoder(wRoll.Body).Decode(&rollRes); err != nil {
		t.Fatalf("failed to decode roll response: %v", err)
	}
	if rollRes.Total < 2 || rollRes.Total > 12 {
		t.Errorf("expected dice total 2..12, got %d", rollRes.Total)
	}

	// 4. POST /api/banco/buy
	buyPayload := map[string]interface{}{
		"game_id": startRes.GameID,
	}
	buyBody, _ := json.Marshal(buyPayload)
	reqBuy := httptest.NewRequest(http.MethodPost, "/api/banco/buy", bytes.NewReader(buyBody))
	reqBuy.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		reqBuy.AddCookie(c)
	}
	wBuy := httptest.NewRecorder()
	router.ServeHTTP(wBuy, reqBuy)
	// Buying can return 200 (bought) or 400 (not a property / already owned / insufficient funds)
	if wBuy.Code != http.StatusOK && wBuy.Code != http.StatusBadRequest {
		t.Errorf("expected 200 or 400 on buy, got %d", wBuy.Code)
	}

	// 5. POST /api/banco/end-turn
	endPayload := map[string]interface{}{
		"game_id": startRes.GameID,
	}
	endBody, _ := json.Marshal(endPayload)
	reqEnd := httptest.NewRequest(http.MethodPost, "/api/banco/end-turn", bytes.NewReader(endBody))
	reqEnd.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		reqEnd.AddCookie(c)
	}
	wEnd := httptest.NewRecorder()
	router.ServeHTTP(wEnd, reqEnd)

	if wEnd.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/banco/end-turn, got %d: %s", wEnd.Code, wEnd.Body.String())
	}

	var endRes struct {
		BotActions []string               `json:"bot_actions"`
		State      map[string]interface{} `json:"state"`
	}
	if err := json.NewDecoder(wEnd.Body).Decode(&endRes); err != nil {
		t.Fatalf("failed to decode end-turn response: %v", err)
	}
	if len(endRes.BotActions) == 0 {
		t.Errorf("expected bot actions executed during end-turn")
	}
}
