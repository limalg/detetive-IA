package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"


	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
	"detetive-ia/internal/room"
	"detetive-ia/internal/server"
)

func setupAdminTestServer(t *testing.T) (http.Handler, *auth.Service, string) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init in-memory db: %v", err)
	}

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	authService := auth.New(database)
	adminUser, err := authService.EnsureAdminUser("admin", "admin12345")
	if err != nil {
		t.Fatalf("failed to ensure admin user: %v", err)
	}

	token, err := authService.Login("admin", "admin12345")
	if err != nil {
		t.Fatalf("failed to login admin: %v", err)
	}
	_ = adminUser

	gameService := game.New(database, nil)
	_, _ = gameService.SeedInitialCase()
	_ = game.SeedDetectiveNightCases(database)

	roomManager, _ := room.NewManager(database)
	weaver := game.NewWeaver(database)

	cfg := server.Config{
		DB:          database,
		AuthService: authService,
		GameService: gameService,
		RoomManager: roomManager,
		Weaver:      weaver,
	}

	router := server.NewRouter(cfg)
	return router, authService, token
}

func TestAdminSettingsAndTelemetry(t *testing.T) {
	router, _, token := setupAdminTestServer(t)

	// 1. Test GET /api/admin/settings
	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on GET /api/admin/settings, got %d: %s", w.Code, w.Body.String())
	}

	var settingsResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&settingsResp); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}
	if settingsResp["model"] != "ondegastei" {
		t.Errorf("expected default model ondegastei, got %v", settingsResp["model"])
	}
	if settingsResp["url"] != "https://router.ofertasshow.shop/v1" {
		t.Errorf("expected default url https://router.ofertasshow.shop/v1, got %v", settingsResp["url"])
	}

	// 2. Test POST /api/admin/settings
	updatePayload := `{"url": "https://api.openai.com/v1", "api_key": "sk-test12345678", "model": "gpt-4o", "provider": "openai_compatible"}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/admin/settings", bytes.NewBufferString(updatePayload))
	reqPost.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	wPost := httptest.NewRecorder()
	router.ServeHTTP(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected status 200 on POST /api/admin/settings, got %d: %s", wPost.Code, wPost.Body.String())
	}

	// 3. Test GET /api/admin/telemetry
	reqTel := httptest.NewRequest(http.MethodGet, "/api/admin/telemetry", nil)
	wTel := httptest.NewRecorder()
	router.ServeHTTP(wTel, reqTel)
	if wTel.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/admin/telemetry, got %d", wTel.Code)
	}

	// 4. Test GET /api/admin/logs
	reqLogs := httptest.NewRequest(http.MethodGet, "/api/admin/logs?limit=20", nil)
	wLogs := httptest.NewRecorder()
	router.ServeHTTP(wLogs, reqLogs)
	if wLogs.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/admin/logs, got %d", wLogs.Code)
	}

	// 5. Test GET /api/game/cases/detective-night
	reqCases := httptest.NewRequest(http.MethodGet, "/api/game/cases/detective-night", nil)
	wCases := httptest.NewRecorder()
	router.ServeHTTP(wCases, reqCases)
	if wCases.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/game/cases/detective-night, got %d", wCases.Code)
	}

	var casesData struct {
		Total int `json:"total"`
		Cases []struct {
			Title string `json:"title"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(wCases.Body).Decode(&casesData); err != nil {
		t.Fatalf("failed to decode cases: %v", err)
	}
	if casesData.Total < 4 {
		t.Errorf("expected at least 4 Detective Night cases, got %d", casesData.Total)
	}

	// 6. Test POST /api/room/create-with-case
	roomPayload := `{
		"case_id": 1,
		"custom_characters": [
			{"name": "Leandro Lima", "notes": "Amigo de infância da vítima", "is_real": true},
			{"name": "Inspetor Clouseau", "notes": "Perito desastrado convidado", "is_real": false}
		]
	}`
	reqRoom := httptest.NewRequest(http.MethodPost, "/api/room/create-with-case", bytes.NewBufferString(roomPayload))
	reqRoom.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	wRoom := httptest.NewRecorder()
	router.ServeHTTP(wRoom, reqRoom)

	if wRoom.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /api/room/create-with-case, got %d: %s", wRoom.Code, wRoom.Body.String())
	}

	var roomResp struct {
		Code           string `json:"code"`
		CustomCastSize int    `json:"custom_cast_size"`
	}
	if err := json.NewDecoder(wRoom.Body).Decode(&roomResp); err != nil {
		t.Fatalf("failed to decode room response: %v", err)
	}
	if len(roomResp.Code) != 4 {
		t.Errorf("expected 4-letter room code, got %s", roomResp.Code)
	}
	if roomResp.CustomCastSize != 2 {
		t.Errorf("expected custom cast size 2, got %d", roomResp.CustomCastSize)
	}
}

func TestAdminPage_BasicAuth_Success(t *testing.T) {
	router, _, _ := setupAdminTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "admin12345")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /admin with valid Basic Auth, got %d. Body: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	// Verify session cookie was set
	cookies := w.Result().Cookies()
	var hasSessionCookie bool
	for _, c := range cookies {
		if c.Name == "session_token" && c.Value != "" {
			hasSessionCookie = true
			break
		}
	}
	if !hasSessionCookie {
		t.Errorf("expected session_token cookie to be set upon successful Basic Auth")
	}
}

func TestAdminPage_BasicAuth_InvalidCredentials(t *testing.T) {
	router, _, _ := setupAdminTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "wrongpass")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for invalid Basic Auth, got %d", w.Code)
	}

	authHeader := w.Header().Get("WWW-Authenticate")
	if authHeader != `Basic realm="Tabuleiro IA Admin"` {
		t.Errorf("expected header WWW-Authenticate: Basic realm=\"Tabuleiro IA Admin\", got %q", authHeader)
	}
}

func TestAdminPage_QueryAuthBasic_Unauthorized(t *testing.T) {
	router, _, _ := setupAdminTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin?auth=basic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for /admin?auth=basic, got %d", w.Code)
	}

	authHeader := w.Header().Get("WWW-Authenticate")
	if authHeader != `Basic realm="Tabuleiro IA Admin"` {
		t.Errorf("expected header WWW-Authenticate: Basic realm=\"Tabuleiro IA Admin\", got %q", authHeader)
	}
}

func TestAdminPage_UnauthenticatedBrowser_ServesAdminPage(t *testing.T) {
	router, _, _ := setupAdminTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK serving admin page for browser, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	// Must NOT redirect (302) to /?auth=login_required
	location := w.Header().Get("Location")
	if location != "" {
		t.Errorf("expected no redirect location, got %s", location)
	}
}

