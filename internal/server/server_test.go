package server_test

import (
	"bytes"
	"encoding/json"
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
