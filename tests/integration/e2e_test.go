package integration_test

import (
	"bytes"
	"encoding/json"
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
	"detetive-ia/internal/server"
	"detetive-ia/web"
)

func TestE2EInvestigationJourney(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "e2e_detetive.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar SQLite: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("falha ao executar migrações DDL: %v", err)
	}

	authService := auth.New(database)
	gameService := game.New(database, nil)
	caseID, err := gameService.SeedInitialCase()
	if err != nil {
		t.Fatalf("falha ao semear caso inicial: %v", err)
	}

	cfg := server.Config{
		Addr:        ":0",
		DB:          database,
		AuthService: authService,
		GameService: gameService,
		StaticFS:    web.FileServer(),
	}

	router := server.NewRouter(cfg)
	ts := httptest.NewServer(router)
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("falha ao criar cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	// 1. Healthcheck & Memória
	t.Run("Healthcheck e Métricas de RAM", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/health")
		if err != nil {
			t.Fatalf("GET /api/health falhou: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("esperado status 200, recebido %d", resp.StatusCode)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("decode healthcheck falhou: %v", err)
		}

		if health["status"] != "ok" {
			t.Errorf("esperado status ok, recebido %v", health["status"])
		}

		allocMB, ok := health["alloc_mb"].(float64)
		if !ok {
			t.Error("esperado campo alloc_mb")
		} else if allocMB > 50.0 {
			t.Errorf("consumo de memória excessivo: %.2f MB (alvo < 50MB)", allocMB)
		}
	})

	// 2. Assets Estáticos & PWA
	t.Run("PWA Assets e Interface Web", func(t *testing.T) {
		// Index
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET / falhou: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Detetive IA") {
			t.Errorf("esperado 'Detetive IA' na raiz")
		}

		// Manifest
		respM, err := client.Get(ts.URL + "/static/manifest.json")
		if err != nil {
			t.Fatalf("GET manifest falhou: %v", err)
		}
		defer respM.Body.Close()
		if respM.StatusCode != http.StatusOK {
			t.Errorf("esperado 200 para manifest, recebido %d", respM.StatusCode)
		}

		// Service Worker
		respSW, err := client.Get(ts.URL + "/static/sw.js")
		if err != nil {
			t.Fatalf("GET sw.js falhou: %v", err)
		}
		defer respSW.Body.Close()
		if respSW.StatusCode != http.StatusOK {
			t.Errorf("esperado 200 para sw.js, recebido %d", respSW.StatusCode)
		}
	})

	// 3. Fluxo de Autenticação Completo
	t.Run("Autenticação E2E (Register -> Login -> Me)", func(t *testing.T) {
		// Register
		regPayload, _ := json.Marshal(map[string]string{
			"username": "investigador_marlowe",
			"password": "senha_confidencial_123",
		})
		resp, err := client.Post(ts.URL+"/api/auth/register", "application/json", bytes.NewReader(regPayload))
		if err != nil {
			t.Fatalf("POST /api/auth/register falhou: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("esperado 201 Created para registro, recebido %d: %s", resp.StatusCode, string(body))
		}

		// Login
		loginPayload, _ := json.Marshal(map[string]string{
			"username": "investigador_marlowe",
			"password": "senha_confidencial_123",
		})
		respLogin, err := client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(loginPayload))
		if err != nil {
			t.Fatalf("POST /api/auth/login falhou: %v", err)
		}
		defer respLogin.Body.Close()
		if respLogin.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(respLogin.Body)
			t.Fatalf("esperado 200 OK para login, recebido %d: %s", respLogin.StatusCode, string(body))
		}

		// Me
		respMe, err := client.Get(ts.URL + "/api/auth/me")
		if err != nil {
			t.Fatalf("GET /api/auth/me falhou: %v", err)
		}
		defer respMe.Body.Close()
		if respMe.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK para /me, recebido %d", respMe.StatusCode)
		}

		var meData struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		}
		_ = json.NewDecoder(respMe.Body).Decode(&meData)
		if meData.User.Username != "investigador_marlowe" {
			t.Errorf("esperado username 'investigador_marlowe', recebido %q", meData.User.Username)
		}
	})

	// 4. Investigação: Obter Caso, Pistas e Interrogar Suspeitos
	var suspects []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	t.Run("Exploração do Caso e Interrogatório", func(t *testing.T) {
		respCase, err := client.Get(ts.URL + "/api/game/case")
		if err != nil {
			t.Fatalf("GET /api/game/case falhou: %v", err)
		}
		defer respCase.Body.Close()
		if respCase.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK para /api/game/case, recebido %d", respCase.StatusCode)
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
		if err := json.NewDecoder(respCase.Body).Decode(&caseRes); err != nil {
			t.Fatalf("decode case falhou: %v", err)
		}

		if len(caseRes.Suspects) == 0 {
			t.Fatal("nenhum suspeito encontrado no caso")
		}
		if len(caseRes.Clues) == 0 {
			t.Fatal("nenhuma pista encontrada no caso")
		}
		suspects = caseRes.Suspects

		// Interrogar primeiro suspeito
		interPayload, _ := json.Marshal(map[string]interface{}{
			"suspect_id": suspects[0].ID,
			"question":   "Onde você estava quando Arthur foi assassinado?",
		})
		respInter, err := client.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(interPayload))
		if err != nil {
			t.Fatalf("POST /api/game/interrogate falhou: %v", err)
		}
		defer respInter.Body.Close()
		if respInter.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK para interrogação, recebido %d", respInter.StatusCode)
		}

		var replyRes struct {
			Reply string `json:"reply"`
		}
		_ = json.NewDecoder(respInter.Body).Decode(&replyRes)
		if strings.TrimSpace(replyRes.Reply) == "" {
			t.Error("esperada resposta textual do suspeito interrogado")
		}
	})

	// 5. Acusação e Veredito
	t.Run("Dedução e Resolução do Mistério", func(t *testing.T) {
		// Acusação Falsa
		fakePayload, _ := json.Marshal(map[string]interface{}{
			"case_id":    caseID,
			"suspect_id": suspects[0].ID,
			"weapon":     "Arma Inexistente",
			"motive":     "Motivo Incorreto",
		})
		respFake, err := client.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(fakePayload))
		if err != nil {
			t.Fatalf("POST /api/game/accuse falhou: %v", err)
		}
		defer respFake.Body.Close()

		var fakeRes struct {
			IsCorrect bool `json:"is_correct"`
		}
		_ = json.NewDecoder(respFake.Body).Decode(&fakeRes)
		if fakeRes.IsCorrect {
			t.Error("acusação incorreta deveria retornar is_correct = false")
		}

		// Acusação Correta
		correctPayload, _ := json.Marshal(map[string]interface{}{
			"case_id":    caseID,
			"suspect_id": 1, // Dra. Helena Voss
			"weapon":     "Adaga Cerimonial Veneziana",
			"motive":     "Dívidas e herança",
		})
		respCorrect, err := client.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(correctPayload))
		if err != nil {
			t.Fatalf("POST acusação correta falhou: %v", err)
		}
		defer respCorrect.Body.Close()
		var correctRes struct {
			IsCorrect bool `json:"is_correct"`
		}
		_ = json.NewDecoder(respCorrect.Body).Decode(&correctRes)
	})

	// 6. Logout
	t.Run("Logout e Invalidação de Sessão", func(t *testing.T) {
		respLogout, err := client.Post(ts.URL+"/api/auth/logout", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /api/auth/logout falhou: %v", err)
		}
		defer respLogout.Body.Close()
		if respLogout.StatusCode != http.StatusOK {
			t.Errorf("esperado 200 OK para logout, recebido %d", respLogout.StatusCode)
		}

		// Validar que /me agora retorna 401
		respMe, err := client.Get(ts.URL + "/api/auth/me")
		if err != nil {
			t.Fatalf("GET /api/auth/me pós-logout falhou: %v", err)
		}
		defer respMe.Body.Close()
		if respMe.StatusCode != http.StatusUnauthorized {
			t.Errorf("esperado 401 Unauthorized pós-logout, recebido %d", respMe.StatusCode)
		}
	})

	// 7. Verificação de Memória Runtime sob Teste
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocMB := float64(m.Alloc) / (1024 * 1024)
	sysMB := float64(m.Sys) / (1024 * 1024)
	t.Logf("Consumo pós-E2E: Alloc=%.2f MB, Sys=%.2f MB (Alvo <60MB)", allocMB, sysMB)
	if allocMB > 60.0 {
		t.Errorf("Consumo de memória Alloc excede o limite alvo de 60MB: %.2f MB", allocMB)
	}
}
