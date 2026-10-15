# Relatório de Testes de Estresse, Robustez e Vulnerabilidades (Chaos QA)
**Data:** 14/10/2026  
**Responsável:** Buster (Chaos QA & Resilience Engineer)  
**Ambiente Alvo:** `http://localhost:8080` (Go Monolith Puro, SQLite modernc.org)  
**Suíte de Testes Criada:** `tests/chaos/chaos_test.go` (Status: **100% PASS**)

---

## 1. Resumo Executivo
Executamos uma bateria intensiva de testes de estresse, concorrência, boundary values e injeções de payloads maliciosos contra o servidor local do **Detetive IA**.

O servidor demonstrou **excelente estabilidade de memória e runtime** (Alloc pós-estresse de apenas **1.55 MB**, com 39 goroutines e sem nenhum crash sob payloads massivos). Entretanto, foram identificados **3 bugs de status HTTP (500 em vez de 400)**, **1 vulnerabilidade crítica de lógica no sistema de acusação** e **2 falhas de estado/concorrência em salas**.

---

## 2. Diagnóstico Detalhado das Falhas Encontradas

### 🚨 VULNERABILIDADE CRÍTICA #1: Exploit de Acusação com Vitória Instantânea
* **Endpoint:** `POST /api/game/accuse`
* **Localização no Código:** `internal/game/game.go` (Linhas 284 e 288)
* **Causa Raiz:**
  ```go
  weaponMatch := strings.Contains(wLower, "adaga") || ... || strings.Contains(solWLower, wLower)
  motiveMatch := strings.Contains(mLower, solMLower) || ... || strings.Contains(solMLower, mLower)
  ```
  Em Go, `strings.Contains(qualquerCoisa, "")` retorna `true`.
* **Exploit:**
  Ao enviar o payload:
  ```json
  {"case_id": 1, "suspect_id": 1, "weapon": "", "motive": ""}
  ```
  Ambos `weaponMatch` e `motiveMatch` avaliam para `true`. O servidor retorna imediatamente vitória com `score: 100` e desfecho perfeito sem que o jogador informe arma ou motivo!
* **Correção Recomendada:**
  Validar explicitamente no handler (`HandleAccuse`) e no service que `strings.TrimSpace(weapon) != ""` e `strings.TrimSpace(motive) != ""` antes de qualquer comparação. Se vazio, rejeitar com `400 Bad Request`.

---

### ⚠️ BUG #2: Erros 500 por Falta de Validação em IDs (`/api/game/accuse`)
* **Endpoint:** `POST /api/game/accuse`
* **Localização no Código:** `internal/server/handlers.go` (Linhas 368-392)
* **Comportamento:**
  - `suspect_id: -1` -> Retorna `500 Internal Server Error` (`sql: no rows in result set`).
  - `suspect_id: 999999` -> Retorna `500 Internal Server Error`.
  - `suspect_id: 0` ou omitido -> Retorna `500 Internal Server Error`.
  - Payload `"null"` -> Retorna `500 Internal Server Error`.
* **Correção Recomendada:**
  Adicionar validação no início de `HandleAccuse`:
  ```go
  if req.SuspectID <= 0 || strings.TrimSpace(req.Weapon) == "" || strings.TrimSpace(req.Motive) == "" {
      h.error(w, http.StatusBadRequest, "suspect_id, weapon and motive are required")
      return
  }
  ```
  E se a consulta no banco retornar `sql.ErrNoRows`, retornar `400 Bad Request` ou `404 Not Found`, nunca `500`.

---

### ⚠️ BUG #3: SuspectID Inexistente Retorna 500 em Interrogatório
* **Endpoint:** `POST /api/game/interrogate`
* **Localização no Código:** `internal/game/game.go` (Linha 229) e `internal/server/handlers.go` (Linha 348)
* **Comportamento:**
  - Enquanto `suspect_id: -1` é tratado com `400`, um ID positivo inexistente (ex: `999999`) gera `sql: no rows in result set` e retorna `500 Internal Server Error`.
* **Correção Recomendada:**
  Tratar `sql.ErrNoRows` retornando erro de domínio amigável mapeado para `404 Not Found` ou `400 Bad Request`.

---

### ⚠️ BUG #4: Senhas Maiores que 72 Bytes Quebram com 500 no Bcrypt
* **Endpoint:** `POST /api/auth/register`
* **Localização no Código:** `internal/auth/auth.go` (Linha 41)
* **Comportamento:**
  - O algoritmo `bcrypt` possui teto nativo de 72 bytes. Ao registrar usuário com senha de 80 caracteres, `bcrypt.GenerateFromPassword` retorna erro interno e o endpoint responde `500 Internal Server Error` ("failed to register user").
* **Correção Recomendada:**
  Validar em `auth.Register`:
  ```go
  if len(password) > 72 {
      return nil, errors.New("password must not exceed 72 bytes")
  }
  ```
  E mapear esse erro no handler para `400 Bad Request`.

---

### ⚠️ BUG #5: Concorrência e Colisão de Convidados Não-Autenticados
* **Endpoint:** `POST /api/rooms/join`
* **Localização no Código:** `internal/server/handlers.go` (Linha 490) e `internal/room/room.go` (Linhas 248-254)
* **Comportamento:**
  - Jogadores sem sessão ativa recebem `userID := int64(2)`.
  - Ao entrar mais de um convidado na mesma sala, o `JoinRoom` detecta `p.UserID == 2` e sobrescreve a persona anterior acreditando ser uma reconexão.
  - Resultado: Múltiplos convidados disputam e sobrescrevem o mesmo slot de jogador.
* **Correção Recomendada:**
  Atribuir IDs pseudo-únicos para convidados (ou gerar sessão guest automática) e associar o slot de reconexão por token/cookie em vez de `userID = 2`.

---

### ⚠️ BUG #6: Start Duplicado Sobrescreve Mistério em Sala Ativa
* **Endpoint:** `POST /api/rooms/{code}/start`
* **Localização no Código:** `internal/server/handlers.go` (Linha 558) e `internal/room/room.go` (Linha 332)
* **Comportamento:**
  - Chamar `start` em uma sala que já possui `status: "active"` não é rejeitado.
  - O servidor aciona novamente o `Weaver.WeaveCase`, gera outro mistério do zero e sobrescreve o `case_id` da sala no SQLite, cancelando a partida em andamento dos participantes.
* **Correção Recomendada:**
  Em `StartRoom` e `handleStartRoom`, verificar:
  ```go
  if roomObj.Status != "lobby" {
      h.error(w, http.StatusBadRequest, "room is already active or finished")
      return
  }
  ```

---

## 3. Resiliência e Eficiência Aprovadas
* **Injeção de SQL e XSS:** Payloads como `' OR '1'='1' --`, `'; DROP TABLE suspects; --`, `<script>alert(1)</script>` foram absorvidos e sanitizados via prepared statements sem qualquer anomalia.
* **Stress de Texto Massivo:** Pergunta de **50.000 caracteres** enviada ao interrogatório foi tratada e gravada com sucesso sem estourar memória.
* **Unicode & Emojis:** Caracteres especiais (`ÁÉÍÓÚãõç`, emojis noir `🕵️‍♂️🔍🩸💀🍷`) foram persistidos perfeitamente.
* **Concorrência de Capacidade:** 15 jogadores autenticados disputando simultaneamente uma sala de 8 vagas resultou em exatamente 8 aceitos e 7 rejeitados, sem deadlocks ou over-capacity.
* **Consumo de Memória Pós-Caos:**
  - **1.55 MB Alloc**
  - **39 Goroutines**
  - **0 leaks / 0 crashes**
