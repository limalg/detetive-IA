package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
	"detetive-ia/internal/server"
	"detetive-ia/web"
)

func setupTestServer(t *testing.T) (http.Handler, *game.Service, int64) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_server.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	authService := auth.New(database)
	gameService := game.New(database, nil)
	caseID, err := gameService.SeedInitialCase()
	if err != nil {
		t.Fatalf("SeedInitialCase failed: %v", err)
	}

	cfg := server.Config{
		Addr:        ":8080",
		DB:          database,
		AuthService: authService,
		GameService: gameService,
		StaticFS:    web.FileServer(),
	}

	router := server.NewRouter(cfg)
	return router, gameService, caseID
}

func TestHealthCheck(t *testing.T) {
	router, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}

	if _, ok := res["alloc_mb"]; !ok {
		t.Error("expected alloc_mb metric in health response")
	}
	if _, ok := res["sys_mb"]; !ok {
		t.Error("expected sys_mb metric in health response")
	}
}

func TestAuthFlow(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. Register
	regBody := `{"username": "testdetective", "password": "supersecretpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for register, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Login
	loginBody := `{"username": "testdetective", "password": "supersecretpassword"}`
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for login, got %d: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session_token cookie to be set on login")
	}

	// 3. Me
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /me, got %d: %s", rec.Code, rec.Body.String())
	}

	var meRes struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&meRes)
	if meRes.User.Username != "testdetective" {
		t.Errorf("expected user testdetective, got %q", meRes.User.Username)
	}

	// 4. Logout
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for logout, got %d", rec.Code)
	}

	// 5. Me after logout should fail
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for /me after logout, got %d", rec.Code)
	}
}

func TestGameEndpoints(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. Get Case Data
	req := httptest.NewRequest(http.MethodGet, "/api/game/case", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /api/game/case, got %d: %s", rec.Code, rec.Body.String())
	}

	var caseRes struct {
		Case struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"case"`
		Suspects []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"suspects"`
		Clues []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"clues"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&caseRes); err != nil {
		t.Fatalf("failed to decode case response: %v", err)
	}

	if caseRes.Case.Title != "O Enigma do Vale das Sombras" {
		t.Errorf("unexpected case title: %s", caseRes.Case.Title)
	}
	if len(caseRes.Suspects) == 0 {
		t.Fatal("expected suspects in response")
	}

	// 2. Interrogate
	suspectID := caseRes.Suspects[0].ID
	interrogateBody := map[string]interface{}{
		"suspect_id": suspectID,
		"question":   "Onde você estava?",
	}
	bodyBytes, _ := json.Marshal(interrogateBody)

	req = httptest.NewRequest(http.MethodPost, "/api/game/interrogate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for interrogate, got %d: %s", rec.Code, rec.Body.String())
	}

	var interRes map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&interRes)
	if _, ok := interRes["reply"]; !ok {
		t.Error("expected 'reply' in interrogate response")
	}

	// 3. Accuse
	accuseBody := map[string]interface{}{
		"case_id":    caseRes.Case.ID,
		"suspect_id": suspectID,
		"weapon":     "Adaga Cerimonial Veneziana",
		"motive":     "Dívidas e herança",
		"clue_ids":   []int64{caseRes.Clues[0].ID, caseRes.Clues[1].ID},
	}
	accBytes, _ := json.Marshal(accuseBody)

	req = httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(accBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for accuse, got %d: %s", rec.Code, rec.Body.String())
	}

	var accRes map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&accRes)
	if _, ok := accRes["is_correct"]; !ok {
		t.Error("expected 'is_correct' in accusation response")
	}
}

func TestStaticAssetsServing(t *testing.T) {
	router, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for index, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
		t.Error("expected html document on root endpoint")
	}
}

func TestRoomEndpoints(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// 1. Create Room (POST /api/rooms)
	createBody := `{"persona_name": "Philip Marlowe", "role": "Detetive Particular"}`
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("expected 201 or 200 for create room, got %d: %s", rec.Code, rec.Body.String())
	}

	var createRes struct {
		Code    string `json:"code"`
		Players []struct {
			PersonaName string `json:"persona_name"`
			Role        string `json:"role"`
			IsHost      bool   `json:"is_host"`
		} `json:"players"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&createRes); err != nil {
		t.Fatalf("failed to decode create room response: %v", err)
	}

	if len(createRes.Code) != 4 {
		t.Fatalf("expected 4-letter room code, got %q", createRes.Code)
	}
	if len(createRes.Players) != 1 || !createRes.Players[0].IsHost {
		t.Fatalf("expected host player in room, got %+v", createRes.Players)
	}

	roomCode := createRes.Code

	// 2. Join Room (POST /api/rooms/join)
	joinBody := fmt.Sprintf(`{"code": %q, "persona_name": "Clara Vance", "role": "Repórter"}`, roomCode)
	req = httptest.NewRequest(http.MethodPost, "/api/rooms/join", strings.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for join room, got %d: %s", rec.Code, rec.Body.String())
	}

	var joinRes struct {
		Players []struct {
			PersonaName string `json:"persona_name"`
		} `json:"players"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&joinRes)
	if len(joinRes.Players) != 2 {
		t.Fatalf("expected 2 players after join, got %d", len(joinRes.Players))
	}

	// 3. Get Room Details (GET /api/rooms/{code})
	req = httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomCode, nil)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get room, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Start Room (POST /api/rooms/{code}/start)
	req = httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomCode+"/start", nil)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for start room, got %d: %s", rec.Code, rec.Body.String())
	}

	var startRes struct {
		CaseID int64 `json:"case_id"`
		Room   struct {
			Status  string `json:"status"`
			Players []struct {
				IsBot bool `json:"is_bot"`
			} `json:"players"`
		} `json:"room"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&startRes)
	if startRes.CaseID == 0 {
		t.Error("expected non-zero case_id upon starting room")
	}
	if startRes.Room.Status != "active" {
		t.Errorf("expected room status 'active', got %q", startRes.Room.Status)
	}
}

func TestInterrogateWithClueConfrontation(t *testing.T) {
	router, gameService, caseID := setupTestServer(t)

	details, err := gameService.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	// Find suspect and a clue
	var culpritID int64
	for _, s := range details.Suspects {
		if strings.Contains(s.Name, "Julian") {
			culpritID = s.ID
			break
		}
	}
	if culpritID == 0 {
		culpritID = details.Suspects[0].ID
	}

	var clueID int64
	for _, c := range details.Clues {
		if strings.Contains(strings.ToLower(c.Name), "adaga") || strings.Contains(strings.ToLower(c.Description), "sangue") {
			clueID = c.ID
			break
		}
	}
	if clueID == 0 && len(details.Clues) > 0 {
		clueID = details.Clues[0].ID
	}

	// Interrogate with confrontation
	reqBody := map[string]interface{}{
		"suspect_id":       culpritID,
		"question":         "Como explica esta evidência?",
		"confront_clue_id": clueID,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/game/interrogate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for confrontation interrogation, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Reply       string `json:"reply"`
		StressLevel int    `json:"stress_level"`
		Confronted  bool   `json:"confronted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Confronted {
		t.Error("expected confronted=true in response")
	}
	if res.StressLevel < 40 {
		t.Errorf("expected elevated stress_level (>=40), got %d", res.StressLevel)
	}
}

func TestAccuseRequiresTwoClues(t *testing.T) {
	router, gameService, caseID := setupTestServer(t)

	details, err := gameService.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	innocentID := details.Suspects[0].ID
	for _, s := range details.Suspects {
		if !s.IsCulprit {
			innocentID = s.ID
			break
		}
	}

	clue1 := details.Clues[0].ID
	clue2 := details.Clues[1].ID

	// 1. Accuse without clues -> 400 Bad Request
	accuseNoClues := map[string]interface{}{
		"case_id":    caseID,
		"suspect_id": innocentID,
		"weapon":     "Revólver",
		"motive":     "Vingança",
	}
	body, _ := json.Marshal(accuseNoClues)
	req := httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for accusation without clues, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "anexe duas pistas") {
		t.Errorf("expected error to mention 'anexe duas pistas', got %s", rec.Body.String())
	}

	// 2. Accuse with 1 clue -> 400 Bad Request
	accuseOneClue := map[string]interface{}{
		"case_id":    caseID,
		"suspect_id": innocentID,
		"weapon":     "Revólver",
		"motive":     "Vingança",
		"clue_ids":   []int64{clue1},
	}
	body, _ = json.Marshal(accuseOneClue)
	req = httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for accusation with 1 clue, got %d", rec.Code)
	}

	// 3. First wrong accusation with 2 clues: Strike 1 (-1 warrant, -30 min)
	accuseWrong1 := map[string]interface{}{
		"case_id":    caseID,
		"suspect_id": innocentID,
		"weapon":     "Cálice",
		"motive":     "Chantagem",
		"clue_ids":   []int64{clue1, clue2},
	}
	body, _ = json.Marshal(accuseWrong1)
	req = httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accusation evaluation, got %d: %s", rec.Code, rec.Body.String())
	}

	var res1 struct {
		IsCorrect            bool `json:"is_correct"`
		WarrantsRemaining    int  `json:"warrants_remaining"`
		TimeRemainingMinutes int  `json:"time_remaining_minutes"`
		GameOver             bool `json:"game_over"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&res1)

	if res1.IsCorrect {
		t.Error("expected is_correct=false for wrong accusation")
	}
	if res1.WarrantsRemaining != 1 {
		t.Errorf("expected 1 warrant remaining after strike 1, got %d", res1.WarrantsRemaining)
	}
	if res1.TimeRemainingMinutes != 390 {
		t.Errorf("expected 390 minutes remaining (420 - 30), got %d", res1.TimeRemainingMinutes)
	}
	if res1.GameOver {
		t.Error("expected gameOver=false after first strike")
	}

	// 4. Second wrong accusation: Strike 2 (-1 warrant -> 0 warrants, game_over=true)
	req = httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for strike 2 evaluation, got %d: %s", rec.Code, rec.Body.String())
	}
	var res2 struct {
		WarrantsRemaining int  `json:"warrants_remaining"`
		GameOver          bool `json:"game_over"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&res2)
	if res2.WarrantsRemaining != 0 || !res2.GameOver {
		t.Errorf("expected warrants=0 and gameOver=true after strike 2, got warrants=%d, gameOver=%v", res2.WarrantsRemaining, res2.GameOver)
	}

	// 5. Subsequent action after strike exhaustion -> 400 Game Over
	req = httptest.NewRequest(http.MethodPost, "/api/game/accuse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for action when warrants exhausted, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInterrogationTimeDeductions(t *testing.T) {
	router, gameService, caseID := setupTestServer(t)

	details, err := gameService.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	suspectID := details.Suspects[0].ID
	clueID := details.Clues[0].ID

	// 1. Normal question: deducts 15 min (420 -> 405)
	q1 := map[string]interface{}{
		"suspect_id": suspectID,
		"question":   "Onde você estava?",
	}
	body, _ := json.Marshal(q1)
	req := httptest.NewRequest(http.MethodPost, "/api/game/interrogate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for normal interrogation, got %d", rec.Code)
	}
	var res1 struct {
		TimeRemainingMinutes int `json:"time_remaining_minutes"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&res1)
	if res1.TimeRemainingMinutes != 405 {
		t.Errorf("expected 405 minutes remaining after 15 min question, got %d", res1.TimeRemainingMinutes)
	}

	// 2. Confrontation question: deducts 20 min (405 -> 385)
	q2 := map[string]interface{}{
		"suspect_id":       suspectID,
		"question":         "Como explica isto?",
		"confront_clue_id": clueID,
	}
	body, _ = json.Marshal(q2)
	req = httptest.NewRequest(http.MethodPost, "/api/game/interrogate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for confrontation, got %d", rec.Code)
	}
	var res2 struct {
		TimeRemainingMinutes int `json:"time_remaining_minutes"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&res2)
	if res2.TimeRemainingMinutes != 385 {
		t.Errorf("expected 385 minutes remaining after 20 min confrontation, got %d", res2.TimeRemainingMinutes)
	}
}


