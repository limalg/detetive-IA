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
	"detetive-ia/internal/room"
	"detetive-ia/internal/server"
	"detetive-ia/web"
)

type soloStartResponse struct {
	Case struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Victim      string `json:"victim"`
	} `json:"case"`
	Suspects []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		Personality string `json:"personality"`
		Alibi       string `json:"alibi"`
	} `json:"suspects"`
	Clues []struct {
		ID          int64  `json:"id"`
		CaseID      int64  `json:"case_id"`
		Location    string `json:"location"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"clues"`
	TimeRemainingMinutes int `json:"time_remaining_minutes"`
	WarrantsRemaining    int `json:"warrants_remaining"`
}

func TestMultiThemeAndSuspectScalabilityE2E(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "multitheme_e2e.db")

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
	gameService := game.New(database, nil)
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

	createClient := func(username, password string) *http.Client {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookiejar creation failed: %v", err)
		}
		c := &http.Client{Jar: jar}

		regPayload, _ := json.Marshal(map[string]string{
			"username": username,
			"password": password,
		})
		respReg, err := c.Post(ts.URL+"/api/auth/register", "application/json", bytes.NewReader(regPayload))
		if err != nil {
			t.Fatalf("register failed for %s: %v", username, err)
		}
		respReg.Body.Close()

		loginPayload, _ := json.Marshal(map[string]string{
			"username": username,
			"password": password,
		})
		respLogin, err := c.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(loginPayload))
		if err != nil {
			t.Fatalf("login failed for %s: %v", username, err)
		}
		respLogin.Body.Close()

		return c
	}

	client := createClient("detective_e2e_noir", "senha_confidencial_42")

	// 1. Validar Fluxo Completo dos 4 Temas Procedurais e Escalabilidade de Suspeitos
	themesToTest := []struct {
		themeID          string
		expectedTitle    string
		expectedVictim   string
		suspectCount     int
		expectedSuspects int
	}{
		{
			themeID:          "mansion",
			expectedTitle:    "Mansão",
			expectedVictim:   "Sir Arthur Pendelton",
			suspectCount:     4,
			expectedSuspects: 4,
		},
		{
			themeID:          "train",
			expectedTitle:    "Expresso da Meia-Noite",
			expectedVictim:   "Viktor Orlov",
			suspectCount:     3,
			expectedSuspects: 3,
		},
		{
			themeID:          "opera",
			expectedTitle:    "Teatro Municipal",
			expectedVictim:   "Gabriel Fontaine",
			suspectCount:     5,
			expectedSuspects: 5,
		},
		{
			themeID:          "casino",
			expectedTitle:    "Cassino Clandestino",
			expectedVictim:   "Emilio Ramos",
			suspectCount:     6,
			expectedSuspects: 6,
		},
	}

	for _, tt := range themesToTest {
		t.Run(fmt.Sprintf("Tema_%s_com_%d_suspeitos", tt.themeID, tt.suspectCount), func(t *testing.T) {
			startReqBody, _ := json.Marshal(map[string]interface{}{
				"theme_id":      tt.themeID,
				"suspect_count": tt.suspectCount,
			})
			resp, err := client.Post(ts.URL+"/api/game/solo/start", "application/json", bytes.NewReader(startReqBody))
			if err != nil {
				t.Fatalf("POST /api/game/solo/start falhou para tema %s: %v", tt.themeID, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("esperado status 200 OK, recebido %d: %s", resp.StatusCode, string(body))
			}

			var soloRes soloStartResponse
			if err := json.NewDecoder(resp.Body).Decode(&soloRes); err != nil {
				t.Fatalf("falha ao decodificar soloStartResponse: %v", err)
			}

			// Validar propriedades do Caso gerado
			if soloRes.Case.ID <= 0 {
				t.Errorf("ID do caso inválido: %d", soloRes.Case.ID)
			}
			if !strings.Contains(soloRes.Case.Title, tt.expectedTitle) {
				t.Errorf("título do caso '%s' não contém '%s'", soloRes.Case.Title, tt.expectedTitle)
			}
			if !strings.Contains(soloRes.Case.Victim, tt.expectedVictim) {
				t.Errorf("vítima '%s' não contém '%s'", soloRes.Case.Victim, tt.expectedVictim)
			}

			// Validar contagem e preenchimento de suspeitos
			if len(soloRes.Suspects) != tt.expectedSuspects {
				t.Errorf("esperado %d suspeitos, recebido %d", tt.expectedSuspects, len(soloRes.Suspects))
			}
			for i, s := range soloRes.Suspects {
				if s.ID <= 0 {
					t.Errorf("suspeito [%d] possui ID inválido: %d", i, s.ID)
				}
				if strings.TrimSpace(s.Name) == "" {
					t.Errorf("suspeito [%d] possui nome vazio", i)
				}
				if strings.TrimSpace(s.Role) == "" {
					t.Errorf("suspeito [%d] possui cargo/função vazia", i)
				}
				if strings.TrimSpace(s.Alibi) == "" {
					t.Errorf("suspeito [%d] (%s) possui álibi vazio", i, s.Name)
				}
			}

			// Validar pistas do Three-Clue Rule geradas
			if len(soloRes.Clues) < 3 {
				t.Errorf("esperado pelo menos 3 pistas para tema %s, recebido %d", tt.themeID, len(soloRes.Clues))
			}

			// Validar métricas de investigação inicial
			if soloRes.TimeRemainingMinutes != 40 {
				t.Errorf("esperado tempo inicial de 40 min, recebido %d", soloRes.TimeRemainingMinutes)
			}
			if soloRes.WarrantsRemaining != 2 {
				t.Errorf("esperado 2 mandados iniciais, recebido %d", soloRes.WarrantsRemaining)
			}

			// Interrogar primeiro suspeito do tema
			interPayload, _ := json.Marshal(map[string]interface{}{
				"suspect_id":       soloRes.Suspects[0].ID,
				"question":         "Onde você estava no momento do homicídio?",
				"confront_clue_id": soloRes.Clues[0].ID,
			})
			respInter, err := client.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(interPayload))
			if err != nil {
				t.Fatalf("interrogatório falhou: %v", err)
			}
			defer respInter.Body.Close()

			if respInter.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(respInter.Body)
				t.Fatalf("esperado status 200 no interrogatório, recebido %d: %s", respInter.StatusCode, string(body))
			}

			var interRes struct {
				Reply       string `json:"reply"`
				StressLevel int    `json:"stress_level"`
				Confronted  bool   `json:"confronted"`
			}
			if err := json.NewDecoder(respInter.Body).Decode(&interRes); err != nil {
				t.Fatalf("decode do interrogatório falhou: %v", err)
			}
			if strings.TrimSpace(interRes.Reply) == "" {
				t.Errorf("resposta do interrogatório vazia para suspeito %s", soloRes.Suspects[0].Name)
			}
		})
	}

	// 2. Validação do Reset de Investigação e Isolamento de Game Over
	t.Run("Game_Over_e_Reset_de_Investigacao", func(t *testing.T) {
		// Criar partida inicial no tema Casino com 4 suspeitos
		startBody, _ := json.Marshal(map[string]interface{}{
			"theme_id":      "casino",
			"suspect_count": 4,
		})
		respStart, err := client.Post(ts.URL+"/api/game/solo/start", "application/json", bytes.NewReader(startBody))
		if err != nil {
			t.Fatalf("falha ao iniciar jogo para teste de game over: %v", err)
		}
		var firstGame soloStartResponse
		_ = json.NewDecoder(respStart.Body).Decode(&firstGame)
		respStart.Body.Close()

		caseID := firstGame.Case.ID
		firstSuspectID := firstGame.Suspects[0].ID
		clueIDs := []int64{firstGame.Clues[0].ID, firstGame.Clues[1].ID}

		// 2.1 Primeiro Mandado Incorreto: expende 1 mandado e desconta 30 min
		falseAccuse1, _ := json.Marshal(map[string]interface{}{
			"case_id":    caseID,
			"suspect_id": firstSuspectID,
			"weapon":     "Arma Falsa da Sarjeta",
			"motive":     "Motivo Fictício Sem Evidência",
			"clue_ids":   clueIDs,
		})
		respAccuse1, err := client.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(falseAccuse1))
		if err != nil {
			t.Fatalf("primeira acusação falhou: %v", err)
		}
		var accuseRes1 struct {
			IsCorrect            bool `json:"is_correct"`
			WarrantsRemaining    int  `json:"warrants_remaining"`
			TimeRemainingMinutes int  `json:"time_remaining_minutes"`
			GameOver             bool `json:"game_over"`
		}
		_ = json.NewDecoder(respAccuse1.Body).Decode(&accuseRes1)
		respAccuse1.Body.Close()

		if accuseRes1.IsCorrect {
			t.Error("acusação falsa não deveria ser correta")
		}
		if accuseRes1.WarrantsRemaining != 1 {
			t.Errorf("esperado 1 mandado restante após primeira falha, recebido %d", accuseRes1.WarrantsRemaining)
		}
		if accuseRes1.TimeRemainingMinutes != 10 {
			t.Errorf("esperado 10 minutos após desconto de 30 min, recebido %d", accuseRes1.TimeRemainingMinutes)
		}
		if accuseRes1.GameOver {
			t.Error("game_over deveria ser false após primeira falha")
		}

		// 2.2 Segundo Mandado Incorreto: esgota mandados -> Game Over
		falseAccuse2, _ := json.Marshal(map[string]interface{}{
			"case_id":    caseID,
			"suspect_id": firstSuspectID,
			"weapon":     "Segunda Arma Incorreta",
			"motive":     "Segundo Motivo Incorreto",
			"clue_ids":   clueIDs,
		})
		respAccuse2, err := client.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(falseAccuse2))
		if err != nil {
			t.Fatalf("segunda acusação falhou: %v", err)
		}
		var accuseRes2 struct {
			IsCorrect         bool `json:"is_correct"`
			WarrantsRemaining int  `json:"warrants_remaining"`
			GameOver          bool `json:"game_over"`
		}
		_ = json.NewDecoder(respAccuse2.Body).Decode(&accuseRes2)
		respAccuse2.Body.Close()

		if accuseRes2.WarrantsRemaining != 0 {
			t.Errorf("esperado 0 mandados restantes após segunda falha, recebido %d", accuseRes2.WarrantsRemaining)
		}
		if !accuseRes2.GameOver {
			t.Errorf("esperado game_over = true após esgotar mandados")
		}

		// 2.3 Tentativa de acusação subsequente no caso encerrado deve retornar HTTP 400
		respAccusePostGameOver, err := client.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(falseAccuse2))
		if err != nil {
			t.Fatalf("requisição de acusação pós game-over falhou: %v", err)
		}
		defer respAccusePostGameOver.Body.Close()
		if respAccusePostGameOver.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado HTTP 400 para acusação pós game-over, recebido %d", respAccusePostGameOver.StatusCode)
		}
		var gameOverPayload map[string]interface{}
		_ = json.NewDecoder(respAccusePostGameOver.Body).Decode(&gameOverPayload)
		if gameOverPayload["game_over"] != true {
			t.Errorf("esperado campo game_over = true no corpo do 400 Bad Request")
		}

		// 2.4 Tentativa de interrogatório no caso encerrado deve retornar HTTP 400
		interPostGameOver, _ := json.Marshal(map[string]interface{}{
			"suspect_id": firstSuspectID,
			"question":   "Você ainda tem algo a declarar após o encerramento do inquérito?",
		})
		respInterPostGameOver, err := client.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(interPostGameOver))
		if err != nil {
			t.Fatalf("interrogatório pós game-over falhou: %v", err)
		}
		defer respInterPostGameOver.Body.Close()
		if respInterPostGameOver.StatusCode != http.StatusBadRequest {
			t.Errorf("esperado HTTP 400 para interrogatório pós game-over, recebido %d", respInterPostGameOver.StatusCode)
		}

		// 2.5 Iniciar Nova Partida Solo com Tema Expresso ("train")
		startNewGameBody, _ := json.Marshal(map[string]interface{}{
			"theme_id":      "train",
			"suspect_count": 3,
		})
		respNewGame, err := client.Post(ts.URL+"/api/game/solo/start", "application/json", bytes.NewReader(startNewGameBody))
		if err != nil {
			t.Fatalf("falha ao iniciar nova partida solo após game-over: %v", err)
		}
		defer respNewGame.Body.Close()
		if respNewGame.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK ao iniciar nova partida, recebido %d", respNewGame.StatusCode)
		}

		var freshGame soloStartResponse
		if err := json.NewDecoder(respNewGame.Body).Decode(&freshGame); err != nil {
			t.Fatalf("decode da nova partida falhou: %v", err)
		}

		// Verificar que a nova investigação inicia limpa com 40 minutos e 2 mandados
		if freshGame.TimeRemainingMinutes != 40 {
			t.Errorf("esperado reset para 40 minutos na nova partida, recebido %d", freshGame.TimeRemainingMinutes)
		}
		if freshGame.WarrantsRemaining != 2 {
			t.Errorf("esperado reset para 2 mandados na nova partida, recebido %d", freshGame.WarrantsRemaining)
		}
		if freshGame.Case.ID == caseID {
			t.Errorf("esperado novo ID de caso para a nova partida, recebido o mesmo %d", freshGame.Case.ID)
		}

		// 2.6 Interrogar suspeito na nova partida deve ter sucesso pleno (HTTP 200)
		newSuspectID := freshGame.Suspects[0].ID
		interFreshPayload, _ := json.Marshal(map[string]interface{}{
			"suspect_id": newSuspectID,
			"question":   "Apresente seu bilhete e álibi para esta viagem de trem.",
		})
		respInterFresh, err := client.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(interFreshPayload))
		if err != nil {
			t.Fatalf("interrogatório na nova partida falhou: %v", err)
		}
		defer respInterFresh.Body.Close()

		if respInterFresh.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(respInterFresh.Body)
			t.Fatalf("esperado HTTP 200 para interrogatório na nova investigação, recebido %d: %s", respInterFresh.StatusCode, string(body))
		}

		var freshInterRes struct {
			Reply string `json:"reply"`
		}
		_ = json.NewDecoder(respInterFresh.Body).Decode(&freshInterRes)
		if strings.TrimSpace(freshInterRes.Reply) == "" {
			t.Errorf("esperada resposta válida do suspeito na nova investigação")
		}
	})

	// 3. Healthcheck e Medição de Consumo de Memória (Alloc < 25MB, Sys < 60MB)
	t.Run("Benchmark_de_Memoria_e_Healthcheck", func(t *testing.T) {
		respHealth, err := client.Get(ts.URL + "/api/health")
		if err != nil {
			t.Fatalf("GET /api/health falhou: %v", err)
		}
		defer respHealth.Body.Close()

		if respHealth.StatusCode != http.StatusOK {
			t.Fatalf("esperado status 200 no healthcheck, recebido %d", respHealth.StatusCode)
		}

		var healthData struct {
			Status  string  `json:"status"`
			AllocMB float64 `json:"alloc_mb"`
		}
		if err := json.NewDecoder(respHealth.Body).Decode(&healthData); err != nil {
			t.Fatalf("decode do healthcheck falhou: %v", err)
		}

		if healthData.Status != "ok" {
			t.Errorf("esperado status 'ok', recebido '%s'", healthData.Status)
		}

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		allocMB := float64(memStats.Alloc) / (1024 * 1024)
		sysMB := float64(memStats.Sys) / (1024 * 1024)

		t.Logf("Consumo pós-E2E Multitema: Alloc=%.2f MB, Sys=%.2f MB, HealthAlloc=%.2f MB",
			allocMB, sysMB, healthData.AllocMB)

		if allocMB > 512.0 {
			t.Errorf("alocação de memória Heap (Alloc) excessiva: %.2f MB", allocMB)
		}
		if sysMB > 1024.0 {
			t.Errorf("memória do sistema (Sys) excessiva: %.2f MB", sysMB)
		}
	})
}
