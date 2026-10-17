package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
	"detetive-ia/internal/models"
	"detetive-ia/internal/room"
	"detetive-ia/internal/server"
	"detetive-ia/internal/telemetry"
	"detetive-ia/web"
)

func TestDetectiveNightAndAdminE2EJourney(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "detective_night_e2e.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar SQLite: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("falha ao executar migrações DDL: %v", err)
	}

	authService := auth.New(database)
	if _, err := authService.EnsureAdminUser("admin", "admin1940"); err != nil {
		t.Fatalf("falha ao criar admin default: %v", err)
	}

	// Seed Detective Night catalog
	if err := game.SeedDetectiveNightCases(database); err != nil {
		t.Fatalf("falha ao semear Detective Night: %v", err)
	}

	aiClient := game.NewUniversalAIClient(game.AIConfig{
		URL:   "https://router.ofertasshow.shop/v1",
		Model: "ondegastei",
	})
	gameService := game.New(database, aiClient)
	roomManager, err := room.NewManager(database)
	if err != nil {
		t.Fatalf("falha ao inicializar Room Manager: %v", err)
	}
	weaver := game.NewWeaver(database)

	cfg := server.Config{
		Addr:        ":0",
		DB:          database,
		AuthService: authService,
		GameService: gameService,
		RoomManager: roomManager,
		Weaver:      weaver,
		StaticFS:    web.FileServer(),
	}

	router := server.NewRouter(cfg)
	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("falha ao criar cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	// --- 1. Autenticação de Admin & Acesso ao Painel Admin ---
	t.Run("Autenticação e Rota /admin", func(t *testing.T) {
		// Acesso não autenticado à rota /admin (deve servir a página de admin com tela de login, status 200)
		noAuthClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := noAuthClient.Get(ts.URL + "/admin")
		if err != nil {
			t.Fatalf("erro ao acessar /admin não autenticado: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 ao acessar /admin sem autenticação (tela de login), recebido %d", resp.StatusCode)
		}

		// Acesso via Basic Auth ao /admin
		basicReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin", nil)
		basicReq.SetBasicAuth("admin", "admin1940")
		basicResp, err := http.DefaultClient.Do(basicReq)
		if err != nil {
			t.Fatalf("erro no acesso Basic Auth: %v", err)
		}
		basicResp.Body.Close()
		if basicResp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 no Basic Auth de /admin, recebido %d", basicResp.StatusCode)
		}


		// Login como admin
		loginPayload := map[string]string{
			"username": "admin",
			"password": "admin1940",
		}
		body, _ := json.Marshal(loginPayload)
		resp, err = client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("erro ao autenticar admin: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 no login do admin, recebido %d", resp.StatusCode)
		}
		var loginRes struct {
			Token string      `json:"token"`
			User  models.User `json:"user"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&loginRes)
		resp.Body.Close()
		if !loginRes.User.IsAdmin {
			t.Fatalf("esperado user.IsAdmin = true para o admin")
		}

		// Acesso autenticado à rota /admin
		resp, err = client.Get(ts.URL + "/admin")
		if err != nil {
			t.Fatalf("erro ao acessar /admin autenticado: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 para /admin autenticado, recebido %d", resp.StatusCode)
		}
		htmlBytes, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(htmlBytes), "Painel de Administração") {
			t.Errorf("templates/admin.html não contém título esperado")
		}
	})

	// --- 2. Endpoints Administrativos: Configurações, Telemetria e Logs ---
	t.Run("Configurações Dinâmicas e Telemetria de IA", func(t *testing.T) {
		// Obter configurações atuais
		resp, err := client.Get(ts.URL + "/api/admin/settings")
		if err != nil {
			t.Fatalf("erro ao obter settings: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 em /api/admin/settings, recebido %d", resp.StatusCode)
		}
		var settings map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&settings)
		if settings["model"] != "ondegastei" {
			t.Errorf("modelo padrão esperado 'ondegastei', recebido '%v'", settings["model"])
		}

		// Atualizar configurações
		newSettings := map[string]string{
			"ai_url":   "https://router.ofertasshow.shop/v1",
			"ai_key":   "sk-test-key-updated",
			"ai_model": "gpt-4o-mini",
		}
		body, _ := json.Marshal(newSettings)
		resp, err = client.Post(ts.URL+"/api/admin/settings", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("erro ao atualizar settings: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 ao atualizar settings, recebido %d", resp.StatusCode)
		}

		// Inspecionar logs da telemetria
		resp, err = client.Get(ts.URL + "/api/admin/logs")
		if err != nil {
			t.Fatalf("erro ao buscar logs: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 em /api/admin/logs, recebido %d", resp.StatusCode)
		}

		// Inspecionar telemetria OpenTelemetry
		resp, err = client.Get(ts.URL + "/api/admin/telemetry")
		if err != nil {
			t.Fatalf("erro ao buscar telemetria: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 em /api/admin/telemetry, recebido %d", resp.StatusCode)
		}
	})

	// --- 3. Catálogo Detective Night e Envelopes Confidenciais ---
	t.Run("Catálogo Detective Night e 3 Envelopes", func(t *testing.T) {
		// Listar casos Detective Night
		resp, err := client.Get(ts.URL + "/api/game/cases/detective-night")
		if err != nil {
			t.Fatalf("erro ao obter casos Detective Night: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 ao obter catálogo Detective Night, recebido %d", resp.StatusCode)
		}
		var catResp struct {
			Total int                      `json:"total"`
			Cases []map[string]interface{} `json:"cases"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&catResp)
		if catResp.Total < 4 {
			t.Fatalf("esperado ao menos 4 casos Detective Night, recebido %d", catResp.Total)
		}

		// Buscar Envelopes de Fazenda Coração da Serra
		resp, err = client.Get(ts.URL + "/api/game/case/fazenda_coracao/envelopes")
		if err != nil {
			t.Fatalf("erro ao buscar envelopes de fazenda_coracao: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 para envelopes, recebido %d", resp.StatusCode)
		}
		var envelopes []models.CaseEnvelope
		_ = json.NewDecoder(resp.Body).Decode(&envelopes)
		if len(envelopes) != 3 {
			t.Fatalf("esperado exatamente 3 envelopes (O Caso, Vozes Suspeitas, Evidências), recebido %d", len(envelopes))
		}
		if envelopes[0].EnvelopeNumber != 1 || envelopes[1].EnvelopeNumber != 2 || envelopes[2].EnvelopeNumber != 3 {
			t.Errorf("ordenação dos envelopes incorreta: %+v", envelopes)
		}
	})

	// --- 4. Criação de Sala com Caso Detective Night e Personagens Customizados ---
	t.Run("Criação de Sala com Personagens Customizados", func(t *testing.T) {
		customCast := []models.CustomCharacter{
			{Name: "Carlos Almirante", IsReal: true, Notes: "Amigo de infância da vítima, empresário"},
			{Name: "Dr. Helena Vance", IsReal: false, Notes: "Médica legista particular com dívida de jogo"},
			{Name: "Mateus Silva", IsReal: true, Notes: "Gerente financeiro da fazenda"},
			{Name: "Sofia Barreto", IsReal: false, Notes: "Herdeira e sobrinha distante"},
		}

		createReq := map[string]interface{}{
			"case_id":       "fazenda_coracao",
			"room_name":     "Investigação Especial Coração da Serra",
			"custom_cast":   customCast,
			"suspect_count": 4,
		}
		body, _ := json.Marshal(createReq)

		resp, err := client.Post(ts.URL+"/api/room/create-with-case", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("erro ao criar sala com caso Detective Night: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("esperado 200 OK na criação de sala com caso, recebido %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var createdRoom struct {
			Code     string                    `json:"code"`
			CaseID   interface{}               `json:"case_id"`
			Suspects []models.Suspect          `json:"suspects"`
			Cast     []models.CustomCharacter `json:"custom_cast"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&createdRoom)

		if createdRoom.Code == "" {
			t.Errorf("código da sala não pode ser vazio")
		}
		if fmt.Sprintf("%v", createdRoom.CaseID) != "fazenda_coracao" && fmt.Sprintf("%v", createdRoom.CaseID) != "1" {
			t.Errorf("esperado case_id fazenda_coracao ou 1, recebido %v", createdRoom.CaseID)
		}
		if len(createdRoom.Suspects) != 4 {
			t.Errorf("esperado 4 suspeitos gerados a partir do elenco customizado, recebido %d", len(createdRoom.Suspects))
		}
	})

	// --- 5. Validação de Memória e Telemetria ---
	t.Run("Validação de Pegada de Memória (<35MB)", func(t *testing.T) {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		allocMB := float64(memStats.Alloc) / 1024 / 1024
		sysMB := float64(memStats.Sys) / 1024 / 1024

		t.Logf("Consumo pós Detective Night & Admin E2E: Alloc=%.2f MB, Sys=%.2f MB (Alvo Alloc < 35MB, Sys < 60MB)", allocMB, sysMB)

		if allocMB > 35.0 {
			t.Errorf("Consumo de memória alocada excedeu 35MB: %.2f MB", allocMB)
		}
		if sysMB > 60.0 {
			t.Errorf("Consumo de memória do sistema excedeu 60MB: %.2f MB", sysMB)
		}

		// Telemetry trace recording verification
		telemetry.Record(telemetry.TraceSpan{
			Name:       "e2e_test_finish",
			Component:  "test",
			Status:     "ok",
			DurationMs: 10,
		})
	})
}
