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

func TestBancoNegotiate_Endpoints(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. Start a game
	startPayload := map[string]interface{}{
		"city":       "São Paulo",
		"difficulty": "facil",
		"bots_count": 1,
	}
	body, _ := json.Marshal(startPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/banco/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on start, got %d: %s", w.Code, w.Body.String())
	}
	var startRes struct {
		GameID string `json:"game_id"`
	}
	_ = json.NewDecoder(w.Body).Decode(&startRes)

	// 2. Test Emergency Loan when balance is low:
	// First loan request with normal balance -> denied
	loanPayload := map[string]interface{}{
		"game_id":     startRes.GameID,
		"action":      "loan",
		"player_id":   1,
		"loan_amount": 300,
	}
	loanBody, _ := json.Marshal(loanPayload)
	reqLoan := httptest.NewRequest(http.MethodPost, "/api/banco/negotiate", bytes.NewReader(loanBody))
	reqLoan.Header.Set("Content-Type", "application/json")
	wLoan := httptest.NewRecorder()
	router.ServeHTTP(wLoan, reqLoan)

	if wLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/banco/negotiate loan, got %d: %s", wLoan.Code, wLoan.Body.String())
	}
	var loanRes struct {
		Approved bool   `json:"approved"`
		Message  string `json:"message"`
	}
	_ = json.NewDecoder(wLoan.Body).Decode(&loanRes)
	if loanRes.Approved {
		t.Errorf("expected loan not approved for high initial balance")
	}

	// 3. Test Trade Negotiation
	tradePayload := map[string]interface{}{
		"game_id":        startRes.GameID,
		"action":         "trade",
		"from_player_id": 1,
		"to_player_id":   2,
		"offered_props":  []int{},
		"requested_props": []int{},
		"cash_offer":     50,
		"cash_request":   0,
	}
	tradeBody, _ := json.Marshal(tradePayload)
	reqTrade := httptest.NewRequest(http.MethodPost, "/api/banco/negotiate", bytes.NewReader(tradeBody))
	reqTrade.Header.Set("Content-Type", "application/json")
	wTrade := httptest.NewRecorder()
	router.ServeHTTP(wTrade, reqTrade)

	if wTrade.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/banco/negotiate trade, got %d: %s", wTrade.Code, wTrade.Body.String())
	}
	var tradeRes struct {
		Accepted bool   `json:"accepted"`
		Message  string `json:"message"`
	}
	_ = json.NewDecoder(wTrade.Body).Decode(&tradeRes)
	if !tradeRes.Accepted {
		t.Errorf("expected trade accepted, got false (message: %s)", tradeRes.Message)
	}
}

func TestBancoRoom_Endpoints(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. POST /api/banco/room/create
	createPayload := map[string]interface{}{
		"host_name":    "Leandro Host",
		"city":         "Rio de Janeiro",
		"difficulty":   "medio",
		"age":          16,
		"school_grade": "2º EM",
	}
	body, _ := json.Marshal(createPayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/banco/room/create", bytes.NewReader(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	router.ServeHTTP(wCreate, reqCreate)

	if wCreate.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/banco/room/create, got %d: %s", wCreate.Code, wCreate.Body.String())
	}
	var createRes struct {
		Code string                 `json:"code"`
		Room map[string]interface{} `json:"room"`
	}
	_ = json.NewDecoder(wCreate.Body).Decode(&createRes)
	if len(createRes.Code) != 4 {
		t.Fatalf("expected 4-letter room code, got %q", createRes.Code)
	}

	// 2. POST /api/banco/room/join
	joinPayload := map[string]interface{}{
		"code":        createRes.Code,
		"player_name": "Carol Amiga",
		"avatar":      "💎",
		"color":       "#52c41a",
	}
	joinBody, _ := json.Marshal(joinPayload)
	reqJoin := httptest.NewRequest(http.MethodPost, "/api/banco/room/join", bytes.NewReader(joinBody))
	reqJoin.Header.Set("Content-Type", "application/json")
	wJoin := httptest.NewRecorder()
	router.ServeHTTP(wJoin, reqJoin)

	if wJoin.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/banco/room/join, got %d: %s", wJoin.Code, wJoin.Body.String())
	}

	// 3. POST /api/banco/room/start
	startPayload := map[string]interface{}{
		"code": createRes.Code,
	}
	startBody, _ := json.Marshal(startPayload)
	reqStart := httptest.NewRequest(http.MethodPost, "/api/banco/room/start", bytes.NewReader(startBody))
	reqStart.Header.Set("Content-Type", "application/json")
	wStart := httptest.NewRecorder()
	router.ServeHTTP(wStart, reqStart)

	if wStart.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/banco/room/start, got %d: %s", wStart.Code, wStart.Body.String())
	}
	var startRoomRes struct {
		Code   string                 `json:"code"`
		GameID string                 `json:"game_id"`
		State  map[string]interface{} `json:"state"`
	}
	_ = json.NewDecoder(wStart.Body).Decode(&startRoomRes)
	if startRoomRes.GameID == "" {
		t.Errorf("expected non-empty game_id after room start")
	}
}

