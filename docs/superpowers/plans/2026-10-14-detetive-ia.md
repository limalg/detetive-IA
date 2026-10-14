# Detetive IA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir uma aplicação web investigativa completa (jogo Detetive IA) em Go monolítico leve (<60MB RAM), com suporte PWA, tela de login simples, SQLite embutido, interrogatório com IA (Gemini/Fallback) e interface Noir imersiva.

**Architecture:** Monolito Go compilado sem CGO utilizando `modernc.org/sqlite`. Os assets web e templates HTML são embutidos no binário via `embed.FS`. O backend expõe endpoints REST para autenticação, obtenção do caso, interrogatório dinâmico com IA e envio do veredito de acusação, além de endpoint `/api/health` para monitoramento de memória.

**Tech Stack:** Go 1.26, `modernc.org/sqlite`, `golang.org/x/crypto/bcrypt`, `embed.FS`, Tailwind CSS, JavaScript Vanilla, Service Worker PWA.

## Global Constraints
- Memória: Consumo de memória em repouso deve ser inferior a 30MB e pico sob teste inferior a 60MB RAM.
- Dependências: Sem CGO (`CGO_ENABLED=0`), sem necessidade de servidor de banco de dados externo ou Node.js em runtime.
- PWA: Deve conter `manifest.json`, ícone de aplicativo e `service-worker.js` funcional para cache offline.
- Testes: Todos os pacotes Go devem ter testes com cobertura e validação por `go test -v ./...`.

---

### Task 1: Estrutura do Monolito e Gerenciamento de Banco SQLite

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/schema.go`
- Create: `internal/db/db_test.go`

**Interfaces:**
- Produces: `db.InitDB(filepath string) (*sql.DB, error)`, `db.RunMigrations(db *sql.DB) error`

- [x] **Step 1: Escrever teste de criação e migração do SQLite**
- [x] **Step 2: Rodar teste para verificar falha (`go test ./internal/db`)**
- [x] **Step 3: Implementar inicialização do SQLite puro Go e migrações DDL**
- [x] **Step 4: Executar `go test -v ./internal/db` e validar aprovação**
- [x] **Step 5: Commit (`git commit -m "feat(db): add sqlite embedded database connection and migrations"`)**

---

### Task 2: Autenticação, Usuários e Sessões

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`
- Create: `internal/models/models.go`

**Interfaces:**
- Consumes: `*sql.DB` de `internal/db`
- Produces: `auth.Register(username, password)`, `auth.Login(username, password) (token, error)`, `auth.ValidateSession(token) (*models.User, error)`

- [x] **Step 1: Escrever testes unitários para registro, hash de senha e sessão**
- [x] **Step 2: Rodar testes e constatar falha**
- [x] **Step 3: Implementar serviço de autenticação com bcrypt e tokens de sessão no SQLite**
- [x] **Step 4: Rodar `go test -v ./internal/auth` e verificar sucesso**
- [x] **Step 5: Commit (`git commit -m "feat(auth): implement user registration, login, and session validation"`)**

---

### Task 3: Lógica do Jogo, Pistas, Interrogatório e Veredito

**Files:**
- Create: `internal/game/game.go`
- Create: `internal/game/ai_interrogator.go`
- Create: `internal/game/game_test.go`

**Interfaces:**
- Consumes: `*sql.DB`, `*models.User`
- Produces: `game.GetCaseDetails(caseID int64)`, `game.Interrogate(suspectID int64, question string) (string, error)`, `game.SubmitAccusation(caseID int64, suspectID int64, weapon string, motive string) (*models.Verdict, error)`

- [x] **Step 1: Escrever testes para carregamento de caso, interrogatório (com mock/fallback) e resolução do mistério**
- [x] **Step 2: Validar falha dos testes**
- [x] **Step 3: Implementar motor do jogo com seed de caso inicial ("O Enigma do Vale das Sombras"), módulo de IA com fallback e validação de veredito**
- [x] **Step 4: Rodar `go test -v ./internal/game` e garantir passagem**
- [x] **Step 5: Commit (`git commit -m "feat(game): add game engine, clues, interrogation engine with AI fallback, and verdict"`)**

---

### Task 4: Servidor HTTP, Rotas da API e Healthcheck com Métricas de RAM

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/handlers.go`
- Create: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `internal/auth`, `internal/game`, `internal/db`
- Produces: `server.NewServer(cfg) *http.Server`, rotas `/api/auth/*`, `/api/game/*`, `/api/health`

- [ ] **Step 1: Escrever testes HTTP para endpoints de auth, game e healthcheck**
- [ ] **Step 2: Rodar testes e constatar falha**
- [ ] **Step 3: Implementar roteador HTTP nativo, middleware de sessão e handler de memória (`runtime.MemStats`)**
- [ ] **Step 4: Executar testes HTTP e validar resposta e status codes**
- [ ] **Step 5: Commit (`git commit -m "feat(server): implement HTTP server, REST endpoints, and memory healthcheck"`)**

---

### Task 5: Interface Web Noir, Suporte PWA e Embed.FS

**Files:**
- Create: `web/static/manifest.json`
- Create: `web/static/sw.js`
- Create: `web/static/css/style.css`
- Create: `web/static/js/app.js`
- Create: `web/templates/index.html`
- Create: `web/web.go` (com `//go:embed static/* templates/*`)
- Create: `cmd/server/main.go`

**Interfaces:**
- Consumes: `web.Assets`, `server.NewServer`
- Produces: Binário executável único `bin/detetive-ia`

- [ ] **Step 1: Criar arquivos estáticos Web, manifest.json, sw.js e templates HTML com tema Noir**
- [ ] **Step 2: Implementar JavaScript interativo para login, exploração de pistas, interrogatório em chat e acusação**
- [ ] **Step 3: Criar ponto de entrada `cmd/server/main.go` integrando `embed.FS` e `db.InitDB`**
- [ ] **Step 4: Compilar e testar servidor completo (`go build -o bin/detetive-ia ./cmd/server`)**
- [ ] **Step 5: Commit (`git commit -m "feat(ui): add noir web interface, PWA support, and embedded assets in Go binary"`)**

---

### Task 6: QA Automatizado, Testes de Carga de Memória e Validação de 1GB RAM

**Files:**
- Create: `tests/integration/e2e_test.go`
- Create: `scripts/test_memory.sh`
- Create: `Dockerfile` (otimizado multi-stage em scratch ou alpine leve)

**Interfaces:**
- Valida o sistema ponta a ponta e assegura conformidade com o limite de 1GB de RAM.

- [ ] **Step 1: Escrever teste de integração E2E (registro -> login -> obter caso -> interrogar -> acusar)**
- [ ] **Step 2: Executar testes E2E e validar 100% de sucesso**
- [ ] **Step 3: Criar script de medição de RSS de memória e Dockerfile enxuto (<30MB image)**
- [ ] **Step 4: Atualizar a nota "Projeto" no Maestri refletindo a conclusão dos fixes**
- [ ] **Step 5: Commit e push para o repositório GitHub**
