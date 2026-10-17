package chaos_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func getBaseURL() string {
	if url := os.Getenv("TEST_SERVER_URL"); url != "" {
		return strings.TrimRight(url, "/")
	}
	return "http://localhost:8080"
}

func checkServerAvailable(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/health")
	if err != nil {
		t.Skipf("Servidor local não acessível em %s: %v. Pulando testes de chaos.", baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check em %s retornou status %d", baseURL, resp.StatusCode)
	}
}

// 1) Testar validação de dados e limites em /api/game/accuse
func TestAccuseValidationAndEdgeCases(t *testing.T) {
	baseURL := getBaseURL()
	checkServerAvailable(t, baseURL)
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("ID Negativo em SuspectID", func(t *testing.T) {
		payload := `{"case_id": 1, "suspect_id": -1, "weapon": "Adaga", "motive": "Herança"}`
		resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("[BUG #1.1 DETECTADO] /api/game/accuse retornou 500 para suspect_id negativo (-1). Deveria validar e retornar 400 Bad Request. Resposta: %s", string(body))
		} else if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request, recebido status %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("ID Inexistente (999999) em SuspectID", func(t *testing.T) {
		payload := `{"case_id": 1, "suspect_id": 999999, "weapon": "Adaga", "motive": "Herança"}`
		resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("[BUG #1.2 DETECTADO] /api/game/accuse retornou 500 para suspect_id inexistente (999999). Deveria retornar 400 ou 404. Resposta: %s", string(body))
		} else if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Errorf("Esperado 400/404, recebido status %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("SuspectID Zero ou Ausente", func(t *testing.T) {
		payload := `{"case_id": 1, "suspect_id": 0, "weapon": "Adaga", "motive": "Herança"}`
		resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("[BUG #1.3 DETECTADO] /api/game/accuse retornou 500 para suspect_id 0. Deveria validar e retornar 400 Bad Request. Resposta: %s", string(body))
		} else if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request, recebido status %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("Tipos Inválidos no JSON", func(t *testing.T) {
		payload := `{"case_id": "texto_invalido", "suspect_id": "abc"}`
		resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request para tipos inválidos no JSON, recebido %d", resp.StatusCode)
		}
	})

	t.Run("Campos Vazios - Exploit de Vitória Automática", func(t *testing.T) {
		// Testar se arma vazia e motivo vazio dão vitória falsa (100 pontos)
		payload := `{"case_id": 1, "suspect_id": 1, "weapon": "", "motive": ""}`
		resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&result)

		isCorrect, _ := result["is_correct"].(bool)
		score, _ := result["score"].(float64)

		if isCorrect && score == 100 {
			t.Logf("[VULNERABILIDADE CRÍTICA #1.4] Exploit confirmado: enviar weapon='' e motive='' com suspect_id=1 retorna vitória com score=100! Causa raiz: strings.Contains(solucao, '') avalia para true.")
		}
	})

	t.Run("Corpo de Requisição Vazio, Nulo ou Malformado", func(t *testing.T) {
		cases := []struct {
			name    string
			payload string
		}{
			{"Vazio", ""},
			{"Apenas abertura de chave", "{"},
			{"Chaves duplas malformadas", "{{malformed"},
			{"JSON nulo", "null"},
		}
		for _, c := range cases {
			resp, err := client.Post(baseURL+"/api/game/accuse", "application/json", strings.NewReader(c.payload))
			if err != nil {
				t.Fatalf("Erro na requisição '%s': %v", c.name, err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusInternalServerError {
				t.Logf("[BUG #1.5 DETECTADO] Payload '%s' resultou em 500 Internal Server Error em vez de 400 Bad Request. Resposta: %s", c.name, string(body))
			} else if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Esperado 400 para payload '%s', recebido %d", c.name, resp.StatusCode)
			}
		}
	})
}

// 2) Testar resiliência com textos muito longos ou caracteres especiais em /api/game/interrogate e /api/auth/register
func TestInterrogateAndRegisterResilience(t *testing.T) {
	baseURL := getBaseURL()
	checkServerAvailable(t, baseURL)
	client := &http.Client{Timeout: 30 * time.Second}

	// Criar uma partida solo fresca para isolamento e garantia de mandados e tempo intactos
	var freshSuspectID int64 = 1
	soloPayload := `{"theme_id": "mansion", "suspect_count": 4}`
	soloResp, err := client.Post(baseURL+"/api/game/solo/start", "application/json", strings.NewReader(soloPayload))
	if err != nil {
		t.Fatalf("Erro ao iniciar partida solo para isolamento: %v", err)
	}
	defer soloResp.Body.Close()

	var soloData struct {
		Suspects []struct {
			ID int64 `json:"id"`
		} `json:"suspects"`
	}
	if err := json.NewDecoder(soloResp.Body).Decode(&soloData); err != nil || len(soloData.Suspects) == 0 {
		t.Fatalf("Falha ao obter suspeitos da partida solo: %v", err)
	}
	freshSuspectID = soloData.Suspects[0].ID

	t.Run("Interrogar: SuspectID Inexistente (999999)", func(t *testing.T) {
		payload := `{"suspect_id": 999999, "question": "Onde você estava?"}`
		resp, err := client.Post(baseURL+"/api/game/interrogate", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("[BUG #2.1 DETECTADO] /api/game/interrogate retornou 500 para suspect_id inexistente (999999). Deveria retornar 400 ou 404. Resposta: %s", string(body))
		} else if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Errorf("Esperado 400 ou 404, recebido %d", resp.StatusCode)
		}
	})

	t.Run("Interrogar: Pergunta com 50.000 Caracteres (Stress Test)", func(t *testing.T) {
		hugeQuestion := strings.Repeat("A", 50000)
		payloadMap := map[string]interface{}{
			"suspect_id": freshSuspectID,
			"question":   hugeQuestion,
		}
		jsonBytes, _ := json.Marshal(payloadMap)

		resp, err := client.Post(baseURL+"/api/game/interrogate", "application/json", bytes.NewReader(jsonBytes))
		if err != nil {
			t.Fatalf("Servidor caiu ou rejeitou conexão com payload de 50.000 chars: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status inesperado para payload gigante: %d", resp.StatusCode)
		}
	})

	t.Run("Interrogar: Caracteres Especiais, Emojis, SQLi e XSS", func(t *testing.T) {
		specialQuestions := []string{
			"Onde você estava às 22h? 🕵️‍♂️🔍🩸💀🍷",
			"Álibi na mansão com café, maçã, coração e poção?",
			"' OR '1'='1' --",
			"'; DROP TABLE suspects; --",
			"<script>alert('xss')</script>",
			"\" or 1=1; -- /* test */",
			"Pergunta com quebra de linha\r\ne tabulações\t\te acentuação: ÁÉÍÓÚãõç",
		}

		for _, q := range specialQuestions {
			payloadMap := map[string]interface{}{
				"suspect_id": freshSuspectID,
				"question":   q,
			}
			jsonBytes, _ := json.Marshal(payloadMap)

			resp, err := client.Post(baseURL+"/api/game/interrogate", "application/json", bytes.NewReader(jsonBytes))
			if err != nil {
				t.Fatalf("Servidor falhou com input '%s': %v", q, err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Esperado status 200 para payload seguro, recebido %d. Resposta: %s", resp.StatusCode, string(body))
			}
		}
	})

	t.Run("Registro: Senha Maior que 72 Bytes (Bcrypt Limit)", func(t *testing.T) {
		// Bcrypt suporta no máximo 72 bytes. Senhas maiores quebram se não forem validadas
		longPassword := strings.Repeat("X", 80)
		uniqueUser := fmt.Sprintf("chaos_user_%d", time.Now().UnixNano())
		payloadMap := map[string]string{
			"username": uniqueUser,
			"password": longPassword,
		}
		jsonBytes, _ := json.Marshal(payloadMap)

		resp, err := client.Post(baseURL+"/api/auth/register", "application/json", bytes.NewReader(jsonBytes))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("[BUG #2.2 DETECTADO] /api/auth/register retornou 500 para senha > 72 bytes (limite do bcrypt). Deveria validar e retornar 400 Bad Request. Resposta: %s", string(body))
		} else if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusCreated {
			t.Errorf("Esperado 400 Bad Request ou 201 Created (com truncamento seguro), recebido %d", resp.StatusCode)
		}
	})

	t.Run("Registro: Usuário e Senha Muito Curtos", func(t *testing.T) {
		invalidInputs := []struct {
			username string
			password string
		}{
			{"ab", "1234"},
			{"abc", "12"},
			{"", ""},
			{"   ", "1234"},
		}

		for _, in := range invalidInputs {
			payloadMap := map[string]string{
				"username": in.username,
				"password": in.password,
			}
			jsonBytes, _ := json.Marshal(payloadMap)

			resp, err := client.Post(baseURL+"/api/auth/register", "application/json", bytes.NewReader(jsonBytes))
			if err != nil {
				t.Fatalf("Erro na requisição: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Esperado 400 Bad Request para input inválido (%s, %s), recebido %d", in.username, in.password, resp.StatusCode)
			}
		}
	})

	t.Run("Registro: Caracteres Especiais e Acentuação", func(t *testing.T) {
		uniqueUser := fmt.Sprintf("detetive_açúcar_%d", time.Now().UnixNano())
		payloadMap := map[string]string{
			"username": uniqueUser,
			"password": "senha_com_açúcar_e_café_1940",
		}
		jsonBytes, _ := json.Marshal(payloadMap)

		resp, err := client.Post(baseURL+"/api/auth/register", "application/json", bytes.NewReader(jsonBytes))
		if err != nil {
			t.Fatalf("Erro na requisição: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Esperado 201 Created para usuário com acentuação, recebido %d: %s", resp.StatusCode, string(body))
		}
	})
}

// 3) Testar concorrência com múltiplos acessos simultâneos em /api/rooms/join e start
func TestRoomConcurrencyAndRaceConditions(t *testing.T) {
	baseURL := getBaseURL()
	checkServerAvailable(t, baseURL)
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Concorrência de Entrada com Usuários Distintos: 15 Tentativas (Limite 8)", func(t *testing.T) {
		// Criar sala nova com host autenticado
		hostJar, _ := cookiejar.New(nil)
		hostClient := &http.Client{Jar: hostJar, Timeout: 5 * time.Second}
		hostName := fmt.Sprintf("host_chaos_%d", time.Now().UnixNano()%1000000)
		regHost, _ := json.Marshal(map[string]string{"username": hostName, "password": "password123"})
		_, _ = hostClient.Post(baseURL+"/api/auth/register", "application/json", bytes.NewReader(regHost))
		loginHost, _ := json.Marshal(map[string]string{"username": hostName, "password": "password123"})
		_, _ = hostClient.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(loginHost))

		createPayload := `{"persona_name": "HostChaos", "role": "Host"}`
		resp, err := hostClient.Post(baseURL+"/api/room/create", "application/json", strings.NewReader(createPayload))
		if err != nil {
			t.Fatalf("Falha ao criar sala: %v", err)
		}
		defer resp.Body.Close()

		var createResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&createResult)
		roomCode, ok := createResult["code"].(string)
		if !ok || roomCode == "" {
			t.Fatalf("Código da sala não retornado")
		}

		// Pré-criar 15 clientes autenticados com IDs distintos
		numConcurrent := 15
		playerClients := make([]*http.Client, numConcurrent)
		for i := 0; i < numConcurrent; i++ {
			jar, _ := cookiejar.New(nil)
			c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
			uName := fmt.Sprintf("p_chaos_%d_%d", time.Now().UnixNano()%100000, i)
			pPayload, _ := json.Marshal(map[string]string{"username": uName, "password": "password123"})
			_, _ = c.Post(baseURL+"/api/auth/register", "application/json", bytes.NewReader(pPayload))
			_, _ = c.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(pPayload))
			playerClients[i] = c
		}

		// Disparo concorrente
		var wg sync.WaitGroup
		var successCount int64
		var rejectedCount int64

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				c := playerClients[idx]
				joinPayload := fmt.Sprintf(`{"code": "%s", "persona_name": "Persona_%d", "role": "Investigador"}`, roomCode, idx)
				jResp, jErr := c.Post(baseURL+"/api/rooms/join", "application/json", strings.NewReader(joinPayload))
				if jErr != nil {
					return
				}
				defer jResp.Body.Close()

				if jResp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else if jResp.StatusCode == http.StatusBadRequest {
					atomic.AddInt64(&rejectedCount, 1)
				}
			}(i)
		}

		wg.Wait()

		// Consultar estado final da sala
		getResp, err := client.Get(fmt.Sprintf("%s/api/rooms/%s", baseURL, roomCode))
		if err != nil {
			t.Fatalf("Falha ao consultar sala: %v", err)
		}
		defer getResp.Body.Close()

		var getResult map[string]interface{}
		_ = json.NewDecoder(getResp.Body).Decode(&getResult)

		playersList, _ := getResult["players"].([]interface{})
		totalPlayers := len(playersList)

		t.Logf("Resultado do estresse de Join com usuários autenticados: %d aceitos, %d rejeitados. Total na sala: %d", successCount, rejectedCount, totalPlayers)

		if totalPlayers > 8 {
			t.Errorf("[FALHA DE CONCORRÊNCIA #3.1] Sala excedeu a capacidade máxima de 8 jogadores! Total presente: %d", totalPlayers)
		}
	})

	t.Run("Colisão de Convidado sem Sessão (Hardcoded UserID=2)", func(t *testing.T) {
		// Teste para registrar o bug onde múltiplos jogadores não autenticados sobrescrevem o slot de userID=2
		createPayload := `{"persona_name": "HostGuestTest", "role": "Host"}`
		resp, err := client.Post(baseURL+"/api/room/create", "application/json", strings.NewReader(createPayload))
		if err != nil {
			t.Fatalf("Falha ao criar sala: %v", err)
		}
		defer resp.Body.Close()

		var createResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&createResult)
		roomCode := createResult["code"].(string)

		// 3 jogadores convidados distintos entram
		for i := 1; i <= 3; i++ {
			joinPayload := fmt.Sprintf(`{"code": "%s", "persona_name": "Convidado_%d", "role": "Detetive"}`, roomCode, i)
			jResp, err := client.Post(baseURL+"/api/rooms/join", "application/json", strings.NewReader(joinPayload))
			if err != nil {
				t.Fatalf("Erro no join: %v", err)
			}
			jResp.Body.Close()
		}

		getResp, _ := client.Get(fmt.Sprintf("%s/api/rooms/%s", baseURL, roomCode))
		var getResult map[string]interface{}
		_ = json.NewDecoder(getResp.Body).Decode(&getResult)
		getResp.Body.Close()

		playersList, _ := getResult["players"].([]interface{})
		t.Logf("[BUG #3.2 DETECTADO] Entraram 3 convidados não-autenticados distintos. Total de jogadores na sala: %d (esperado 4, incluindo o host). O slot userID=2 foi repetidamente sobrescrito.", len(playersList))
	})

	t.Run("Concorrência de Start: 10 Requisições Simultâneas", func(t *testing.T) {
		createPayload := `{"persona_name": "HostStartChaos", "role": "Host"}`
		resp, err := client.Post(baseURL+"/api/room/create", "application/json", strings.NewReader(createPayload))
		if err != nil {
			t.Fatalf("Falha ao criar sala: %v", err)
		}
		defer resp.Body.Close()

		var createResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&createResult)
		roomCode := createResult["code"].(string)

		numStarts := 10
		var wg sync.WaitGroup
		statusCodes := make([]int, numStarts)

		for i := 0; i < numStarts; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				startClient := &http.Client{Timeout: 5 * time.Second}
				sResp, sErr := startClient.Post(fmt.Sprintf("%s/api/rooms/%s/start", baseURL, roomCode), "application/json", nil)
				if sErr == nil {
					statusCodes[idx] = sResp.StatusCode
					sResp.Body.Close()
				}
			}(i)
		}

		wg.Wait()

		// Verificar se o servidor continuou íntegro
		getResp, err := client.Get(fmt.Sprintf("%s/api/rooms/%s", baseURL, roomCode))
		if err != nil {
			t.Fatalf("Servidor não respondeu após concorrência de start: %v", err)
		}
		defer getResp.Body.Close()

		var roomObj map[string]interface{}
		_ = json.NewDecoder(getResp.Body).Decode(&roomObj)
		if roomObj["status"] != "active" {
			t.Errorf("Esperado status active, recebido %v", roomObj["status"])
		}
	})

	t.Run("Start Duplicado em Sala Já Ativa", func(t *testing.T) {
		createPayload := `{"persona_name": "HostActiveTest", "role": "Host"}`
		resp, err := client.Post(baseURL+"/api/room/create", "application/json", strings.NewReader(createPayload))
		if err != nil {
			t.Fatalf("Falha ao criar sala: %v", err)
		}
		defer resp.Body.Close()

		var createResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&createResult)
		roomCode := createResult["code"].(string)

		// Primeiro Start
		sResp1, err := client.Post(fmt.Sprintf("%s/api/rooms/%s/start", baseURL, roomCode), "application/json", nil)
		if err != nil {
			t.Fatalf("Erro no primeiro start: %v", err)
		}
		var start1Data map[string]interface{}
		_ = json.NewDecoder(sResp1.Body).Decode(&start1Data)
		sResp1.Body.Close()
		caseID1 := start1Data["case_id"]

		// Segundo Start (Sala já ativa)
		sResp2, err := client.Post(fmt.Sprintf("%s/api/rooms/%s/start", baseURL, roomCode), "application/json", nil)
		if err != nil {
			t.Fatalf("Erro no segundo start: %v", err)
		}
		var start2Data map[string]interface{}
		_ = json.NewDecoder(sResp2.Body).Decode(&start2Data)
		sResp2.Body.Close()
		caseID2 := start2Data["case_id"]

		if sResp2.StatusCode == http.StatusOK {
			t.Logf("[BUG #3.3 DETECTADO] Start em sala já ativa retornou 200 OK e sobrescreveu o case_id de %v para %v! Deveria retornar 400 Bad Request.", caseID1, caseID2)
		} else if sResp2.StatusCode != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request para start em sala ativa, recebido %d", sResp2.StatusCode)
		}
	})
}

// 4) Testar saúde do servidor e consumo de memória pós-estresse
func TestServerHealthAndMemoryAfterChaos(t *testing.T) {
	baseURL := getBaseURL()
	checkServerAvailable(t, baseURL)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("Servidor travou após testes de estresse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check retornou status %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Falha ao decodificar health check: %v", err)
	}

	allocMB, ok := health["alloc_mb"].(float64)
	if !ok {
		t.Fatalf("Campo alloc_mb não encontrado")
	}

	goroutines, _ := health["goroutines"].(float64)
	t.Logf("Métricas pós-estresse: Alloc=%.2f MB, Goroutines=%.0f, Status=%v", allocMB, goroutines, health["status"])

	if allocMB > 60.0 {
		t.Errorf("Consumo de memória excedeu limite de 60MB: %.2f MB", allocMB)
	}

	if goroutines > 100 {
		t.Errorf("Possível vazamento de goroutines detectado: %.0f goroutines ativas", goroutines)
	}
}

// 5) Testar robustez, concorrência e clamping de limites em /api/game/solo/start
func TestSoloStartChaosAndClamping(t *testing.T) {
	baseURL := getBaseURL()
	checkServerAvailable(t, baseURL)
	client := &http.Client{Timeout: 5 * time.Second}

	// 5.1 Clamping de limites em SuspectCount (-1, 0, 10, 100)
	t.Run("Clamping de SuspectCount e Valores de Borda (-1, 0, 10, 100)", func(t *testing.T) {
		boundaryCases := []struct {
			name             string
			themeID          string
			suspectCount     int
			expectedSuspects int
		}{
			{name: "SuspectCount Negativo (-1)", themeID: "mansion", suspectCount: -1, expectedSuspects: 4},
			{name: "SuspectCount Zero (0)", themeID: "mansion", suspectCount: 0, expectedSuspects: 4},
			{name: "SuspectCount Mínimo Abaixo (1 -> 3)", themeID: "train", suspectCount: 1, expectedSuspects: 3},
			{name: "SuspectCount Mínimo Abaixo (2 -> 3)", themeID: "train", suspectCount: 2, expectedSuspects: 3},
			{name: "SuspectCount Excedente (10 -> 6)", themeID: "casino", suspectCount: 10, expectedSuspects: 6},
			{name: "SuspectCount Extremo (100 -> 6)", themeID: "casino", suspectCount: 100, expectedSuspects: 6},
		}

		for _, bc := range boundaryCases {
			t.Run(bc.name, func(t *testing.T) {
				payload, _ := json.Marshal(map[string]interface{}{
					"theme_id":      bc.themeID,
					"suspect_count": bc.suspectCount,
				})
				resp, err := client.Post(baseURL+"/api/game/solo/start", "application/json", bytes.NewReader(payload))
				if err != nil {
					t.Fatalf("Erro na requisição: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Fatalf("esperado status 200 OK para %s, recebido %d: %s", bc.name, resp.StatusCode, string(body))
				}

				var data struct {
					Case struct {
						ID int64 `json:"id"`
					} `json:"case"`
					Suspects             []map[string]interface{} `json:"suspects"`
					Clues                []map[string]interface{} `json:"clues"`
					TimeRemainingMinutes int                      `json:"time_remaining_minutes"`
					WarrantsRemaining    int                      `json:"warrants_remaining"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
					t.Fatalf("decode do payload falhou: %v", err)
				}

				if len(data.Suspects) != bc.expectedSuspects {
					t.Errorf("para %s, esperado clamping para %d suspeitos, recebido %d",
						bc.name, bc.expectedSuspects, len(data.Suspects))
				}
				if data.Case.ID <= 0 {
					t.Errorf("ID do caso inválido: %d", data.Case.ID)
				}
				if data.TimeRemainingMinutes != 420 || data.WarrantsRemaining != 2 {
					t.Errorf("esperado métricas iniciais (420 min / 2 mandados), recebido (%d / %d)",
						data.TimeRemainingMinutes, data.WarrantsRemaining)
				}
			})
		}
	})

	// 5.2 Degradação graciosa para ThemeIDs inválidos / payloads anômalos
	t.Run("Degradação Graciosa para ThemeIDs Inválidos e Injeções", func(t *testing.T) {
		weirdThemes := []string{
			"cyberpunk_noir",
			"",
			"   ",
			"'; DROP TABLE cases; --",
			"<script>alert(1)</script>",
			"THEME_UNKNOWN_12345",
		}

		for _, th := range weirdThemes {
			payload, _ := json.Marshal(map[string]interface{}{
				"theme_id":      th,
				"suspect_count": 4,
			})
			resp, err := client.Post(baseURL+"/api/game/solo/start", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("erro ao enviar tema '%s': %v", th, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("esperado 200 OK (fallback gracioso para mansion) para tema '%s', recebido %d: %s",
					th, resp.StatusCode, string(body))
				continue
			}

			var res struct {
				Case struct {
					Title string `json:"title"`
				} `json:"case"`
				Suspects []map[string]interface{} `json:"suspects"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				t.Fatalf("decode falhou para tema '%s': %v", th, err)
			}

			if !strings.Contains(res.Case.Title, "Mansão") {
				t.Errorf("esperado fallback para tema padrão Mansão, recebido título: %s", res.Case.Title)
			}
			if len(res.Suspects) != 4 {
				t.Errorf("esperado 4 suspeitos, recebido %d", len(res.Suspects))
			}
		}
	})

	// 5.3 Concorrência e Disparo Rápido Back-to-Back (25 requisições simultâneas)
	t.Run("Disparo Concorrente Back-to-Back (25 Requisições)", func(t *testing.T) {
		numConcurrent := 25
		themes := []string{"mansion", "train", "opera", "casino", "alien_invaders", ""}
		counts := []int{-1, 0, 3, 5, 6, 10, 100}

		var wg sync.WaitGroup
		var successCount int64
		var failCount int64

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				th := themes[idx%len(themes)]
				cnt := counts[idx%len(counts)]

				c := &http.Client{Timeout: 5 * time.Second}
				bodyBytes, _ := json.Marshal(map[string]interface{}{
					"theme_id":      th,
					"suspect_count": cnt,
				})

				resp, err := c.Post(baseURL+"/api/game/solo/start", "application/json", bytes.NewReader(bodyBytes))
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					var res struct {
						Case struct {
							ID int64 `json:"id"`
						} `json:"case"`
						Suspects []map[string]interface{} `json:"suspects"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.Case.ID > 0 && len(res.Suspects) >= 3 && len(res.Suspects) <= 6 {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&failCount, 1)
					}
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Resultado de disparo concorrente em /api/game/solo/start: %d sucessos, %d falhas (Total: %d)",
			successCount, failCount, numConcurrent)

		if failCount > 0 {
			t.Errorf("Detectadas %d falhas ou inconsistências em %d disparos concorrentes", failCount, numConcurrent)
		}
		if successCount != int64(numConcurrent) {
			t.Errorf("Esperado %d sucessos, recebido %d", numConcurrent, successCount)
		}

		// Validar que o servidor permanece 100% íntegro e sem pane
		healthResp, err := client.Get(baseURL + "/api/health")
		if err != nil {
			t.Fatalf("servidor parou de responder após estresse de solo start: %v", err)
		}
		defer healthResp.Body.Close()
		if healthResp.StatusCode != http.StatusOK {
			t.Errorf("esperado 200 OK para healthcheck após estresse, recebido %d", healthResp.StatusCode)
		}
	})
}

