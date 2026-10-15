package web_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"detetive-ia/web"
)


func TestEmbeddedTemplates(t *testing.T) {
	file, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html from embed.FS: %v", err)
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	content := string(contentBytes)

	expectedSnippets := []string{
		"<!DOCTYPE html>",
		`<link rel="manifest" href="/static/manifest.json">`,
		`<link rel="stylesheet" href="/static/css/style.css">`,
		`<script src="/static/js/app.js"`,
		`id="auth-section"`,
		`id="game-section"`,
		`id="case-summary"`,
		`id="suspects-list"`,
		`id="interrogation-modal"`,
		`id="accuse-modal"`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("templates/index.html is missing snippet: %q", snippet)
		}
	}
}

func TestEmbeddedManifest(t *testing.T) {
	file, err := web.Assets.Open("static/manifest.json")
	if err != nil {
		t.Fatalf("failed to open static/manifest.json from embed.FS: %v", err)
	}
	defer file.Close()

	var manifest struct {
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		StartURL        string `json:"start_url"`
		Display         string `json:"display"`
		BackgroundColor string `json:"background_color"`
		ThemeColor      string `json:"theme_color"`
		Icons           []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("static/manifest.json is not valid JSON: %v", err)
	}

	if manifest.Name == "" || manifest.ShortName == "" {
		t.Errorf("manifest missing name or short_name: %+v", manifest)
	}
	if manifest.Display != "standalone" {
		t.Errorf("expected display standalone, got %q", manifest.Display)
	}
	if len(manifest.Icons) == 0 {
		t.Errorf("manifest has no icons defined")
	}
}

func TestEmbeddedServiceWorker(t *testing.T) {
	file, err := web.Assets.Open("static/sw.js")
	if err != nil {
		t.Fatalf("failed to open static/sw.js from embed.FS: %v", err)
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("failed to read static/sw.js: %v", err)
	}
	content := string(contentBytes)

	requiredSnippets := []string{
		"addEventListener('install'",
		"addEventListener('fetch'",
		"addEventListener('activate'",
		"caches.open",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("static/sw.js is missing required PWA logic: %q", snippet)
		}
	}
}

func TestEmbeddedStyleAndApp(t *testing.T) {
	// Verify CSS
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("failed to read static/css/style.css: %v", err)
	}
	cssContent := string(cssBytes)
	if !strings.Contains(cssContent, "#121214") && !strings.Contains(cssContent, "noir") {
		t.Errorf("static/css/style.css does not appear to have Noir theme styling")
	}

	// Verify JS
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/app.js: %v", err)
	}
	jsContent := string(jsBytes)
	requiredJSSnippets := []string{
		"/api/auth/login",
		"/api/game/case",
		"/api/game/interrogate",
		"/api/game/accuse",
	}
	for _, snippet := range requiredJSSnippets {
		if !strings.Contains(jsContent, snippet) {
			t.Errorf("static/js/app.js is missing API endpoint call: %q", snippet)
		}
	}
}

func TestFileServer(t *testing.T) {
	handler := web.FileServer()
	if handler == nil {
		t.Fatal("expected non-nil FileServer handler")
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Test root GET /
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("failed to GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Detetive IA") {
		t.Errorf("expected root page to contain 'Detetive IA'")
	}

	// Test static file GET /static/manifest.json
	manifestResp, err := http.Get(server.URL + "/static/manifest.json")
	if err != nil {
		t.Fatalf("failed to GET /static/manifest.json: %v", err)
	}
	defer manifestResp.Body.Close()

	if manifestResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /static/manifest.json, got %d", manifestResp.StatusCode)
	}

	// Test 404 for unknown route
	notFoundResp, err := http.Get(server.URL + "/non-existent-page")
	if err != nil {
		t.Fatalf("failed to GET /non-existent-page: %v", err)
	}
	defer notFoundResp.Body.Close()

	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", notFoundResp.StatusCode)
	}
}

func TestLobbyAndConfrontationUI(t *testing.T) {
	// Verify HTML structure for Lobby and Confrontation
	indexFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer indexFile.Close()

	htmlBytes, _ := io.ReadAll(indexFile)
	html := string(htmlBytes)

	requiredHTMLElements := []string{
		`id="lobby-section"`,
		`id="btn-mode-solo"`,
		`id="btn-mode-multi"`,
		`id="room-code-input"`,
		`id="btn-random-persona"`,
		`id="persona-name"`,
		`id="persona-role"`,
		`id="persona-secret"`,
		`id="btn-start-game"`,
		`id="confront-clue-select"`,
		`id="suspect-stress-meter"`,
	}

	for _, elem := range requiredHTMLElements {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing required element for Task 5: %s", elem)
		}
	}

	// Verify JS logic for Lobby and Confrontation
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, _ := io.ReadAll(jsFile)
	js := string(jsBytes)

	requiredJSElements := []string{
		"generateRandomPersona",
		"confront_clue_id",
		"/api/room",
		"stress_level",
	}

	for _, elem := range requiredJSElements {
		if !strings.Contains(js, elem) {
			t.Errorf("static/js/app.js missing required logic for Task 5: %s", elem)
		}
	}
}

func TestDiegeticNoirDeskAndNightClockUI(t *testing.T) {
	// 1. Verify HTML Structure for Diegetic Desk, Night Clock, Warrants and Modals
	indexFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer indexFile.Close()

	htmlBytes, _ := io.ReadAll(indexFile)
	html := string(htmlBytes)

	requiredHTMLElements := []string{
		`desk-surface`,
		`id="night-clock"`,
		`id="warrants-badge"`,
		`folder-kraft`,
		`CONFIDENCIAL`,
		`polaroid-card`,
		`pushpin`,
		`id="accuse-clues-container"`,
		`id="game-over-modal"`,
	}

	for _, elem := range requiredHTMLElements {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing required element for Sprint v0.3 Task 3: %s", elem)
		}
	}

	// 2. Verify CSS Styling for Diegetic Wood Desk, Amber Lamp Vignette, Kraft Dossier and Polaroids
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, _ := io.ReadAll(cssFile)
	css := string(cssBytes)

	requiredCSSSnippets := []string{
		"#14110f",
		".desk-surface",
		".folder-kraft",
		".polaroid-card",
		".pushpin",
	}

	for _, snippet := range requiredCSSSnippets {
		if !strings.Contains(css, snippet) {
			t.Errorf("static/css/style.css missing required styling for Sprint v0.3 Task 3: %s", snippet)
		}
	}

	// 3. Verify JS Logic for Night Clock, Warrants, 2-Clue Accusation, and Game Over Modals
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, _ := io.ReadAll(jsFile)
	js := string(jsBytes)

	requiredJSSnippets := []string{
		"updateNightClock",
		"updateWarrantsBadge",
		"showGameOverModal",
		"clue_ids",
	}

	for _, snippet := range requiredJSSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("static/js/app.js missing required logic for Sprint v0.3 Task 3: %s", snippet)
		}
	}
}

func TestEmbeddedVisualAssets(t *testing.T) {
	assets := []string{
		"static/assets/scene_noir.webp",
		"static/assets/suspect_femme.webp",
		"static/assets/suspect_magnate.webp",
		"static/assets/suspect_butler.webp",
		"static/assets/murder_weapon.webp",
	}

	handler := web.FileServer()

	for _, a := range assets {
		file, err := web.Assets.Open(a)
		if err != nil {
			t.Fatalf("failed to open %s from embed.FS: %v", a, err)
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil || len(data) == 0 {
			t.Errorf("asset %s is empty or unreadable: %v", a, err)
		}

		req := httptest.NewRequest(http.MethodGet, "/"+a, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /%s returned status %d, expected 200", a, rec.Code)
		}
	}
}
