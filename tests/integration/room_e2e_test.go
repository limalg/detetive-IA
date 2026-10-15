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

func TestRoomE2EInvestigationJourney(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "room_e2e_detetive.db")

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

	// Helper to create authenticated HTTP client
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

	hostClient := createClient("detetive_marlowe", "senha_secreta_noir_1")
	player2Client := createClient("reporter_vivian", "senha_secreta_noir_2")

	var roomCode string

	// 1. Host cria Sala Coletiva
	t.Run("Criação de Sala Coletiva com Código de 4 Letras", func(t *testing.T) {
		createPayload, _ := json.Marshal(map[string]string{
			"persona_name": "Philip Marlowe",
			"role":         "Detetive Particular",
			"secret":       "Dívidas com agiotas do porto",
		})
		resp, err := hostClient.Post(ts.URL+"/api/room/create", "application/json", bytes.NewReader(createPayload))
		if err != nil {
			t.Fatalf("POST /api/room/create falhou: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("esperado status 200/201 na criação de sala, recebido %d: %s", resp.StatusCode, string(body))
		}

		var res struct {
			Code    string `json:"code"`
			Players []struct {
				PersonaName string `json:"persona_name"`
				IsHost      bool   `json:"is_host"`
			} `json:"players"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode do create room falhou: %v", err)
		}

		if len(res.Code) != 4 {
			t.Fatalf("esperado código de sala com 4 caracteres, recebido %q", res.Code)
		}
		for _, char := range res.Code {
			if strings.ContainsRune("IOZ102", char) {
				t.Errorf("código de sala %q contém caractere proibido/ambíguo %c", res.Code, char)
			}
		}

		if len(res.Players) != 1 || !res.Players[0].IsHost || res.Players[0].PersonaName != "Philip Marlowe" {
			t.Errorf("host não registrado corretamente na sala: %+v", res.Players)
		}

		roomCode = res.Code
	})

	// 2. Segundo Jogador entra na Sala com seu Código e Persona
	t.Run("Entrada de Segundo Jogador na Sala via Código", func(t *testing.T) {
		joinPayload, _ := json.Marshal(map[string]string{
			"code":         roomCode,
			"persona_name": "Vivian Rutledge",
			"role":         "Jornalista Investigativa",
			"secret":       "Conhecia a vítima no passado",
		})
		resp, err := player2Client.Post(ts.URL+"/api/room/join", "application/json", bytes.NewReader(joinPayload))
		if err != nil {
			t.Fatalf("POST /api/room/join falhou: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("esperado 200 OK no join da sala, recebido %d: %s", resp.StatusCode, string(body))
		}

		var res struct {
			Code    string `json:"code"`
			Players []struct {
				PersonaName string `json:"persona_name"`
				Role        string `json:"role"`
			} `json:"players"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode do join room falhou: %v", err)
		}

		if len(res.Players) != 2 {
			t.Fatalf("esperado 2 jogadores na sala após join, recebido %d", len(res.Players))
		}
	})

	// 3. Consulta do Estado da Sala
	t.Run("Consulta do Estado do Lobby da Sala", func(t *testing.T) {
		resp, err := hostClient.Get(ts.URL + "/api/room/" + roomCode)
		if err != nil {
			t.Fatalf("GET /api/room/%s falhou: %v", roomCode, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK na consulta de sala, recebido %d", resp.StatusCode)
		}

		var res struct {
			Code   string `json:"code"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode room info falhou: %v", err)
		}

		if res.Status != "lobby" {
			t.Errorf("esperado status 'lobby' antes do início, recebido %q", res.Status)
		}
	})

	var generatedCaseID int64
	var suspects []struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Role  string `json:"role"`
		Alibi string `json:"alibi"`
	}
	var clues []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	// 4. Início da Rodada Procedural com Preenchimento Automático de Bots
	t.Run("Início da Partida Procedural e Preenchimento com Bots", func(t *testing.T) {
		startResp, err := hostClient.Post(ts.URL+"/api/room/"+roomCode+"/start", "application/json", nil)
		if err != nil {
			t.Fatalf("POST /api/room/%s/start falhou: %v", roomCode, err)
		}
		defer startResp.Body.Close()

		if startResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(startResp.Body)
			t.Fatalf("esperado 200 OK ao iniciar sala, recebido %d: %s", startResp.StatusCode, string(body))
		}

		var startRes struct {
			CaseID int64 `json:"case_id"`
			Room   struct {
				Status  string `json:"status"`
				Players []struct {
					PersonaName string `json:"persona_name"`
					IsBot       bool   `json:"is_bot"`
				} `json:"players"`
			} `json:"room"`
		}
		if err := json.NewDecoder(startResp.Body).Decode(&startRes); err != nil {
			t.Fatalf("decode start response falhou: %v", err)
		}

		if startRes.Room.Status != "active" {
			t.Errorf("esperado status 'active' para sala iniciada, recebido %q", startRes.Room.Status)
		}
		if startRes.CaseID <= 0 {
			t.Fatalf("esperado case_id procedural válido, recebido %d", startRes.CaseID)
		}
		generatedCaseID = startRes.CaseID
	})

	// 5. Obtenção do Dossiê do Caso Gerado com a Regra das Três Pistas
	t.Run("Obtenção do Dossiê Procedural e Regra das Três Pistas", func(t *testing.T) {
		caseResp, err := hostClient.Get(fmt.Sprintf("%s/api/game/case?case_id=%d", ts.URL, generatedCaseID))
		if err != nil {
			t.Fatalf("GET /api/game/case falhou: %v", err)
		}
		defer caseResp.Body.Close()

		if caseResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(caseResp.Body)
			t.Fatalf("esperado 200 OK ao carregar caso, recebido %d: %s", caseResp.StatusCode, string(body))
		}

		var res struct {
			Case struct {
				ID          int64  `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Victim      string `json:"victim"`
			} `json:"case"`
			Suspects []struct {
				ID    int64  `json:"id"`
				Name  string `json:"name"`
				Role  string `json:"role"`
				Alibi string `json:"alibi"`
			} `json:"suspects"`
			Clues []struct {
				ID          int64  `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"clues"`
		}

		if err := json.NewDecoder(caseResp.Body).Decode(&res); err != nil {
			t.Fatalf("decode do dossiê falhou: %v", err)
		}

		if len(res.Suspects) < 4 {
			t.Fatalf("esperado no mínimo 4 suspeitos gerados (jogadores + bots), recebido %d", len(res.Suspects))
		}
		if len(res.Clues) < 3 {
			t.Fatalf("esperado no mínimo 3 pistas geradas pela Regra das Três Pistas, recebido %d", len(res.Clues))
		}

		suspects = res.Suspects
		clues = res.Clues
	})

	// Identificar a pista de contradição de álibi e o suspeito que mente
	var contradictionClueID int64
	var contradictionClueName string
	for _, c := range clues {
		if strings.Contains(strings.ToLower(c.Name), "álibi") ||
			strings.Contains(strings.ToLower(c.Name), "alibi") ||
			strings.Contains(strings.ToLower(c.Description), "contradição") ||
			strings.Contains(strings.ToLower(c.Description), "falso") {
			contradictionClueID = c.ID
			contradictionClueName = c.Name
			break
		}
	}

	// 6. Confrontação no Interrogatório e Elevação de Estresse
	t.Run("Interrogatório com Confronto de Pista e Medidor de Estresse", func(t *testing.T) {
		if len(suspects) == 0 {
			t.Fatal("nenhum suspeito disponível para interrogatório")
		}
		targetSuspect := suspects[0]

		// Interrogatório Regular (sem confronto de pista)
		normalPayload, _ := json.Marshal(map[string]interface{}{
			"suspect_id": targetSuspect.ID,
			"question":   "O que você estava fazendo no momento exato do disparo?",
		})
		respNormal, err := hostClient.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(normalPayload))
		if err != nil {
			t.Fatalf("POST interrogatório normal falhou: %v", err)
		}
		defer respNormal.Body.Close()

		if respNormal.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK no interrogatório normal, recebido %d", respNormal.StatusCode)
		}

		var normalRes struct {
			Reply       string `json:"reply"`
			StressLevel int    `json:"stress_level"`
		}
		if err := json.NewDecoder(respNormal.Body).Decode(&normalRes); err != nil {
			t.Fatalf("decode resposta normal falhou: %v", err)
		}
		if normalRes.Reply == "" {
			t.Error("esperada resposta textual no interrogatório normal")
		}

		// Interrogatório com Confronto de Pista
		if contradictionClueID == 0 && len(clues) > 0 {
			contradictionClueID = clues[0].ID
			contradictionClueName = clues[0].Name
		}

		confrontPayload, _ := json.Marshal(map[string]interface{}{
			"suspect_id":       targetSuspect.ID,
			"question":         fmt.Sprintf("Como você explica esta evidência: %s?", contradictionClueName),
			"confront_clue_id": contradictionClueID,
		})
		respConfront, err := hostClient.Post(ts.URL+"/api/game/interrogate", "application/json", bytes.NewReader(confrontPayload))
		if err != nil {
			t.Fatalf("POST confronto falhou: %v", err)
		}
		defer respConfront.Body.Close()

		if respConfront.StatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK no interrogatório com confronto, recebido %d", respConfront.StatusCode)
		}

		var confrontRes struct {
			Reply       string `json:"reply"`
			StressLevel int    `json:"stress_level"`
		}
		if err := json.NewDecoder(respConfront.Body).Decode(&confrontRes); err != nil {
			t.Fatalf("decode resposta confronto falhou: %v", err)
		}

		if confrontRes.StressLevel <= 0 {
			t.Errorf("esperado stress_level positivo após confronto de evidência, recebido %d", confrontRes.StressLevel)
		}
	})

	// 7. Acusação e Resolução da Partida
	t.Run("Dedução Pericial e Veredito da Sala", func(t *testing.T) {
		// Consultar solução diretamente do banco para testar o fluxo de vitória
		var solSuspectID int64
		var solWeapon, solMotive string
		err := database.QueryRow(`
			SELECT solution_suspect_id, solution_weapon, solution_motive
			FROM cases WHERE id = ?
		`, generatedCaseID).Scan(&solSuspectID, &solWeapon, &solMotive)
		if err != nil {
			t.Fatalf("falha ao consultar solução do caso gerado: %v", err)
		}

		// Acusação falsa
		var innocentID int64
		for _, s := range suspects {
			if s.ID != solSuspectID {
				innocentID = s.ID
				break
			}
		}
		if innocentID == 0 && len(suspects) > 0 {
			innocentID = suspects[0].ID
		}

		falseAccusePayload, _ := json.Marshal(map[string]interface{}{
			"case_id":    generatedCaseID,
			"suspect_id": innocentID,
			"weapon":     "Arma Fictícia",
			"motive":     "Motivo Sem Provas",
		})
		respFalse, err := hostClient.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(falseAccusePayload))
		if err != nil {
			t.Fatalf("POST acusação falsa falhou: %v", err)
		}
		defer respFalse.Body.Close()

		var falseVerdict struct {
			IsCorrect bool `json:"is_correct"`
		}
		_ = json.NewDecoder(respFalse.Body).Decode(&falseVerdict)
		if falseVerdict.IsCorrect {
			t.Error("acusação incorreta deveria retornar is_correct = false")
		}

		// Acusação correta baseada no caso procedural gerado
		correctAccusePayload, _ := json.Marshal(map[string]interface{}{
			"case_id":    generatedCaseID,
			"suspect_id": solSuspectID,
			"weapon":     solWeapon,
			"motive":     solMotive,
		})
		respCorrect, err := hostClient.Post(ts.URL+"/api/game/accuse", "application/json", bytes.NewReader(correctAccusePayload))
		if err != nil {
			t.Fatalf("POST acusação correta falhou: %v", err)
		}
		defer respCorrect.Body.Close()

		var correctVerdict struct {
			IsCorrect bool `json:"is_correct"`
			Score     int  `json:"score"`
		}
		if err := json.NewDecoder(respCorrect.Body).Decode(&correctVerdict); err != nil {
			t.Fatalf("decode do veredito correto falhou: %v", err)
		}

		if !correctVerdict.IsCorrect {
			t.Errorf("esperado is_correct = true para acusação com a solução do Weaver, recebido false")
		}
		if correctVerdict.Score != 100 {
			t.Errorf("esperado pontuação 100 para dedução perfeita, recebido %d", correctVerdict.Score)
		}
	})

	// 8. Medição de Memória Runtime sob Carga
	t.Run("Benchmark de Consumo de RAM (<30MB)", func(t *testing.T) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		allocMB := float64(m.Alloc) / (1024 * 1024)
		sysMB := float64(m.Sys) / (1024 * 1024)

		t.Logf("Consumo de Memória E2E Sala: Alloc=%.2f MB, Sys=%.2f MB (Alvo <30MB, Limite 1GB)", allocMB, sysMB)
		if allocMB > 30.0 {
			t.Errorf("Consumo Alloc excede a meta de 30MB: %.2f MB", allocMB)
		}
	})
}
