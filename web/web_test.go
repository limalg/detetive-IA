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
		"static/assets/scene_train.webp",
		"static/assets/scene_opera.webp",
		"static/assets/scene_casino.webp",
		"static/assets/clue_document.webp",
		"static/assets/clue_contradiction.webp",
		"static/assets/clue_trace.webp",
		"static/assets/char_nicole.webp",
		"static/assets/char_vincent.webp",
		"static/assets/char_klaus.webp",
		"static/assets/char_clarice.webp",
		"static/assets/char_barnaby.webp",
		"static/assets/char_hector.webp",
		"static/assets/char_genevieve.webp",
		"static/assets/suspect_femme.webp",
		"static/assets/suspect_magnate.webp",
		"static/assets/suspect_butler.webp",
		"static/assets/murder_weapon.webp",
		"static/assets/weapon_poison.webp",
		"static/assets/weapon_dagger.webp",
		"static/assets/weapon_candlestick.webp",
		"static/assets/weapon_razor.webp",
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

	// Verify app.js contains getWeaponPhotoUrl, getCluePhotoUrl, getThemeScenePhotoUrl and mappings
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

	requiredSnippets := []string{
		"getWeaponPhotoUrl",
		"getThemeScenePhotoUrl",
		"getCluePhotoUrl",
		"weapon_poison.webp",
		"weapon_dagger.webp",
		"weapon_candlestick.webp",
		"weapon_razor.webp",
		"murder_weapon.webp",
		"clue_document.webp",
		"clue_contradiction.webp",
		"clue_trace.webp",
		"scene_train.webp",
		"scene_opera.webp",
		"scene_casino.webp",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(jsContent, snippet) {
			t.Errorf("static/js/app.js is missing asset mapping snippet: %q", snippet)
		}
	}
}

func TestAudioEngineAsset(t *testing.T) {
	// 1. Verify index.html references audio.js and contains sound-toggle button
	indexFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer indexFile.Close()

	htmlBytes, err := io.ReadAll(indexFile)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	html := string(htmlBytes)

	if !strings.Contains(html, `<script src="/static/js/audio.js"></script>`) {
		t.Errorf("templates/index.html missing script tag for /static/js/audio.js")
	}
	if !strings.Contains(html, `id="sound-toggle"`) {
		t.Errorf("templates/index.html missing #sound-toggle button")
	}

	// 2. Verify static/js/audio.js exists in embedded FS and contains expected NoirAudio API
	jsFile, err := web.Assets.Open("static/js/audio.js")
	if err != nil {
		t.Fatalf("failed to open static/js/audio.js from embed.FS: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/audio.js: %v", err)
	}
	js := string(jsBytes)

	expectedSnippets := []string{
		"window.NoirAudio",
		"init",
		"playTypewriter",
		"playClockTick",
		"playStamp",
		"playPaper",
		"toggleMute",
		"isMuted",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("static/js/audio.js missing required API/snippet: %q", snippet)
		}
	}

	// 3. Verify HTTP 200 via web.FileServer()
	handler := web.FileServer()
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/static/js/audio.js")
	if err != nil {
		t.Fatalf("failed to GET /static/js/audio.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /static/js/audio.js, got %d", resp.StatusCode)
	}
}

func TestTactileDeskElements(t *testing.T) {
	// 1. Verify HTML elements for Tactile Desk, Manila Kraft Tabs, Forensic Loupe & Stamp Button
	indexFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer indexFile.Close()

	htmlBytes, err := io.ReadAll(indexFile)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	html := string(htmlBytes)

	requiredElements := []string{
		`id="desk-workspace"`,
		`id="theme-select"`,
		`id="suspect-count-select"`,
		`id="forensic-loupe-modal"`,
		`id="stamp-accuse-btn"`,
		`desk-mat`,
		`kraft-folder`,
		`value="mansion"`,
		`value="train"`,
		`value="opera"`,
		`value="casino"`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing required tactile desk element: %s", elem)
		}
	}

	// 2. Verify CSS styling for Tactile Desk, Mat, Kraft Tabs, Stamp Hit Animation & Loupe
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("failed to read static/css/style.css: %v", err)
	}
	css := string(cssBytes)

	requiredCSS := []string{
		".desk-mat",
		".nav-tab-btn.kraft-folder",
		"#stamp-accuse-btn",
		"@keyframes stampHit",
		".forensic-loupe-modal",
	}

	for _, elem := range requiredCSS {
		if !strings.Contains(css, elem) {
			t.Errorf("static/css/style.css missing required tactile desk style: %s", elem)
		}
	}

	// 3. Verify JS integration: solo/start API, playStamp, playPaper, playClockTick, forensic loupe
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/app.js: %v", err)
	}
	js := string(jsBytes)

	requiredJS := []string{
		"/api/game/solo/start",
		"openForensicLoupe",
		"playStamp",
		"playPaper",
		"playClockTick",
	}

	for _, elem := range requiredJS {
		if !strings.Contains(js, elem) {
			t.Errorf("static/js/app.js missing required tactile desk JS logic: %s", elem)
		}
	}
}

func TestCentralizedInterrogationStage(t *testing.T) {
	// 1. Verify index.html centralized interrogation stage structure
	indexFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer indexFile.Close()

	htmlBytes, err := io.ReadAll(indexFile)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	html := string(htmlBytes)

	requiredHTMLElements := []string{
		"interrogate-stage",
		"suspect-spotlight",
		"suspect-portrait-frame",
		`id="interrogate-portrait-img"`,
		"smoke-particles",
		`id="interrogate-sweat-drops"`,
		`id="interrogate-suspect-name"`,
		`id="interrogate-speech-balloon"`,
		`id="suspect-stress-container"`,
	}

	for _, elem := range requiredHTMLElements {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing required interrogation stage element: %s", elem)
		}
	}

	// 2. Verify style.css rules and keyframes
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("failed to read static/css/style.css: %v", err)
	}
	css := string(cssBytes)

	requiredCSSRules := []string{
		".interrogate-stage",
		".suspect-spotlight",
		".suspect-portrait-frame",
		"@keyframes smokeDrift",
		"@keyframes nervousBreath",
		"@keyframes stressPulse",
	}

	for _, elem := range requiredCSSRules {
		if !strings.Contains(css, elem) {
			t.Errorf("static/css/style.css missing required interrogation stage rule: %s", elem)
		}
	}

	// 3. Verify app.js dynamic suspect portrait, stress reaction, and typewriter audio
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/app.js: %v", err)
	}
	js := string(jsBytes)

	requiredJSSnippets := []string{
		"interrogate-portrait-img",
		"interrogate-sweat-drops",
		"high-stress",
		"playTypewriter",
	}

	for _, snippet := range requiredJSSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("static/js/app.js missing required interrogation stage JS snippet: %s", snippet)
		}
	}
}




func TestBoardGameHub(t *testing.T) {
	// 1. Verify index.html hub section structure: felt table, tactile cards,
	// header navigation button and Banco Imobiliário preview modal
	htmlFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer htmlFile.Close()

	htmlBytes, err := io.ReadAll(htmlFile)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	html := string(htmlBytes)

	requiredHubHTML := []string{
		`id="hub-section"`,
		`id="hub-games-grid"`,
		`id="hub-game-preview-modal"`,
		`id="close-hub-preview-btn"`,
		`id="btn-hub-nav"`,
		"detetive-ia",
		"banco-imobiliario-ia",
		"Detetive IA",
		"Banco Imobiliário IA",
		"Disponível",
		"Em Construção",
		"Em Breve",
		"Investigar / Abrir Dossiê",
	}

	for _, elem := range requiredHubHTML {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing required hub element: %s", elem)
		}
	}

	// 2. Security rule: /admin must NEVER be linked or surfaced in the UI
	adminRefs := []string{`href="/admin"`, `href='/admin'`, `action="/admin"`}
	for _, ref := range adminRefs {
		if strings.Contains(html, ref) {
			t.Errorf("templates/index.html must not link to %s (hidden route rule)", ref)
		}
	}
	jsAdminRefs := []string{`"href", "/admin"`, `location.href = "/admin"`, `window.location = "/admin"`}
	for _, ref := range jsAdminRefs {
		if strings.Contains(html, ref) || strings.Contains(html, ref) {
			t.Errorf("hub UI must not redirect to %s (hidden route rule)", ref)
		}
	}

	// 3. Verify style.css noir felt table & card rules
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("failed to read static/css/style.css: %v", err)
	}
	css := string(cssBytes)

	requiredHubCSS := []string{
		".hub-card-btn-sub",
		".custom-investigation-modal",
		".hub-felt-table",
		".hub-games-grid",
		".hub-card",
		".hub-badge-active",
		".hub-badge-construction",
		".hub-card-under-construction",
		".hub-tag",
		".btn-hub-nav",
		"@keyframes hubCardRise",
	}

	for _, elem := range requiredHubCSS {
		if !strings.Contains(css, elem) {
			t.Errorf("static/css/style.css missing required hub rule: %s", elem)
		}
	}

	// 4. Verify app.js: hub navigation, dynamic fetch with static fallback and modal
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/app.js: %v", err)
	}
	js := string(jsBytes)

	requiredHubJS := []string{
		"/api/hub/games",
		"/api/game/custom-investigation",
		"applyCaseResponse",
		"openCustomInvestigation",
		"showHubView",
		"setupHubEvents",
		"renderHubGames",
		"openGamePreview",
		"closeGamePreview",
	}

	for _, snippet := range requiredHubJS {
		if !strings.Contains(js, snippet) {
			t.Errorf("static/js/app.js missing required hub JS logic: %s", snippet)
		}
	}

	// Static fallback must exist so cards render even if the API has not loaded
	fallbackMarkers := []string{"HUB_FALLBACK_GAMES"}
	for _, marker := range fallbackMarkers {
		if !strings.Contains(js, marker) {
			t.Errorf("static/js/app.js missing hub static fallback: %s", marker)
		}
	}
}

func TestCustomInvestigationFrontend(t *testing.T) {
	// 1. index.html: modal de personalização com campos diegéticos
	htmlFile, err := web.Assets.Open("templates/index.html")
	if err != nil {
		t.Fatalf("failed to open templates/index.html: %v", err)
	}
	defer htmlFile.Close()

	htmlBytes, err := io.ReadAll(htmlFile)
	if err != nil {
		t.Fatalf("failed to read templates/index.html: %v", err)
	}
	html := string(htmlBytes)

	requiredElements := []string{
		`id="custom-investigation-modal"`,
		`id="close-custom-invest-btn"`,
		`id="custom-investigation-form"`,
		`id="custom-city"`,
		`id="custom-age"`,
		`id="custom-school-grade"`,
		`id="custom-difficulty"`,
		`id="custom-friends"`,
		`id="custom-theme"`,
		`id="btn-generate-investigation"`,
		"Gerar Investigação Personalizada com IA",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("templates/index.html missing custom investigation element: %s", elem)
		}
	}

	// 2. style.css: sistema visual do modal de personalização
	cssFile, err := web.Assets.Open("static/css/style.css")
	if err != nil {
		t.Fatalf("failed to open static/css/style.css: %v", err)
	}
	defer cssFile.Close()

	cssBytes, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("failed to read static/css/style.css: %v", err)
	}
	css := string(cssBytes)

	for _, rule := range []string{".custom-invest-form-grid", ".hub-preview-field", "@keyframes hubPreviewIn"} {
		if !strings.Contains(css, rule) {
			t.Errorf("static/css/style.css missing rule: %s", rule)
		}
	}

	// 3. app.js: campo city enviado no POST e resposta aplicada ao estado do jogo
	jsFile, err := web.Assets.Open("static/js/app.js")
	if err != nil {
		t.Fatalf("failed to open static/js/app.js: %v", err)
	}
	defer jsFile.Close()

	jsBytes, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("failed to read static/js/app.js: %v", err)
	}
	js := string(jsBytes)

	for _, snippet := range []string{
		"custom-city",
		"school_grade: ",
		"focus_subject",
		"applyCaseResponse",
		"renderDossier()",
		"Gerar Investigação Personalizada",
	} {
		if !strings.Contains(js, snippet) {
			t.Errorf("static/js/app.js missing custom investigation logic: %s", snippet)
		}
	}
}
