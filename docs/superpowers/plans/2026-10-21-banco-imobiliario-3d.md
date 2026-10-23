# Banco Imobiliário 3D Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir um módulo independente de Banco Imobiliário (Monopoly) 3D multiplayer em tempo real, onde o usuário digita sua cidade ou endereço e o tabuleiro de 40 casas é gerado por IA com ruas e bairros reais, com votação relâmpago pré-jogo para o local mais icônico, mesa 3D estilo arcade vista de cima e interface 100% contida (zero rolagem) com frases objetivas.

**Architecture:** Monólito modular em Go composto por motor de regras e estado em memória (`internal/banco3d/`), hub de salas e WebSockets para broadcast em tempo real, gerador de cidades com fallback offline e frontend estático leve em Three.js WebGL + HTML5/CSS3 (`web/banco3d/`), executável através de `cmd/banco3d/main.go`.

**Tech Stack:** Go 1.22+, Gorilla/Nhooyr WebSocket, Three.js (WebGL), HTML5 / Vanilla CSS3 responsivo (100vh).

**Spec:** [`docs/superpowers/specs/2026-10-18-banco-imobiliario-3d-design.md`](file:///C:/Users/limae/Documents/Projects/Detiv/docs/superpowers/specs/2026-10-18-banco-imobiliario-3d-design.md)

## Global Constraints

- O código do Detetive IA existente em `internal/`, `cmd/` e `web/` deve permanecer 100% inalterado e funcional.
- Interface visual obrigatória: Opção C (Mesa 3D Arcade, visão superior/isométrica das peças e do tabuleiro, dados físicos centrais, zero scroll).
- Zero rolagem de tela: tudo deve caber em 100vh tanto no desktop quanto no mobile.
- Textos ultra-objetivos: botões diretos de 1 clique ("Comprar", "Passar", "Rolar") e notificações curtas.
- Multiplayer via link de sala compartilhável (`/sala/:id`), sincronização de ações em tempo real via WebSockets.

## Review Focus

1. **Reconexão de jogador caído:** Jogador que atualiza o navegador ou oscila a rede deve se reconectar à sala pelo seu ID de sessão sem reiniciar a partida.
2. **Timeout de turno inativo:** Se o jogador da vez não rolar dados ou não decidir uma compra em 30 segundos, o servidor deve acionar auto-pass para não travar a mesa.
3. **Cidades não encontradas ou sem internet:** O gerador de cidades por IA deve ter fallback instantâneo de dados estruturados para cidades conhecidas e modelo genérico para entradas arbitrárias.
4. **Votação empatada do ponto icônico:** Caso a votação de 15 segundos resulte em empate de votos, o servidor deve desempatar determinísticamente sem travar o início da partida.
5. **Falência e saída:** Jogador com saldo negativo sem fundos é declarado falido e suas propriedades retornam ao banco sem quebrar a rotação dos turnos restantes.

---

### Task 1: Modelo do Tabuleiro de 40 Casas & Propriedades

**Files:**
- Create: `internal/banco3d/board.go`
- Test: `internal/banco3d/board_test.go`

**Interfaces:**
- Produces: `TileType`, `ColorGroup`, `Tile`, `Board`, `NewStandardBoard(cityName string, tiles []Tile) *Board`

- [ ] **Step 1: Escrever teste de definição das 40 casas do tabuleiro**

```go
func TestBoardStructure(t *testing.T) {
    b := NewStandardBoard("Curitiba, PR", nil)
    if len(b.Tiles) != 40 {
        t.Fatalf("esperado 40 casas, recebido %d", len(b.Tiles))
    }
    if b.Tiles[0].Type != TileGo {
        t.Errorf("casa 0 deve ser Início/Go")
    }
    if b.Tiles[10].Type != TileJail {
        t.Errorf("casa 10 deve ser Prisão")
    }
}
```

- [ ] **Step 2: Executar teste e validar falha**

Run: `go test -v ./internal/banco3d -run TestBoardStructure`  
Expected: FAIL (tipos e funções não definidos)

- [ ] **Step 3: Implementar estruturas de dados em `internal/banco3d/board.go`**

Criar enum de `TileType` (Go, Property, Station, Utility, Tax, Chance, Chest, Jail, FreeParking, GoToJail), `ColorGroup` (Brown, LightBlue, Pink, Orange, Red, Yellow, Green, DarkBlue), struct `Tile` com `Index`, `Name`, `Type`, `Group`, `Price`, `RentBase`, `OwnerID`, e fábrica `NewStandardBoard`.

- [ ] **Step 4: Executar teste e validar sucesso**

Run: `go test -v ./internal/banco3d -run TestBoardStructure`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/banco3d/board.go internal/banco3d/board_test.go
git commit -m "feat(banco3d): add 40-tile board domain model"
```

---

### Task 2: Máquina de Estados da Partida & Turnos

**Files:**
- Create: `internal/banco3d/game.go`
- Test: `internal/banco3d/game_test.go`

**Interfaces:**
- Consumes: `Board`, `Tile`, `TileType` de `board.go`
- Produces: `Player`, `GameState`, `NewGame(roomID string, board *Board) *Game`, `AddPlayer`, `RollDice`, `BuyProperty`, `PassTurn`

- [ ] **Step 1: Escrever teste de movimentação e compra de propriedade**

```go
func TestGameMovementAndBuy(t *testing.T) {
    board := NewStandardBoard("São Paulo", nil)
    game := NewGame("room-1", board)
    p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
    game.Start()
    
    // Simular jogada de dados
    diceResult, err := game.RollDice(p1.ID, 3, 4) // 7 casas
    if err != nil {
        t.Fatalf("erro ao rolar dados: %v", err)
    }
    if p1.Position != 7 {
        t.Errorf("esperado posicao 7, recebido %d", p1.Position)
    }
}
```

- [ ] **Step 2: Executar teste e validar falha**

Run: `go test -v ./internal/banco3d -run TestGameMovementAndBuy`  
Expected: FAIL

- [ ] **Step 3: Implementar lógica de partida e regras de turno em `internal/banco3d/game.go`**

Implementar estrutura de `Player` (ID, Name, Color, Piece, Money, Position, InJail, IsBankrupt), regras de rotação de turnos, cobrança de aluguel quando cair em propriedade de outro jogador, e compra de propriedades disponíveis.

- [ ] **Step 4: Executar teste e validar sucesso**

Run: `go test -v ./internal/banco3d -run TestGameMovementAndBuy`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/banco3d/game.go internal/banco3d/game_test.go
git commit -m "feat(banco3d): implement game state machine and turn resolution"
```

---

### Task 3: Gerador de Cidades por IA com Catálogo Offline

**Files:**
- Create: `internal/banco3d/generator.go`
- Test: `internal/banco3d/generator_test.go`

**Interfaces:**
- Produces: `CityTheme`, `GenerateCityBoard(query string, apiKey string) (*Board, []string, error)`

- [ ] **Step 1: Escrever teste do gerador e fallback de catálogo local**

```go
func TestCityBoardGeneratorFallback(t *testing.T) {
    board, candidates, err := GenerateCityBoard("Curitiba", "")
    if err != nil {
        t.Fatalf("geração falhou: %v", err)
    }
    if len(candidates) < 3 {
        t.Errorf("esperado ao menos 3 candidatos a ponto icônico, recebido %d", len(candidates))
    }
    if len(board.Tiles) != 40 {
        t.Errorf("tabuleiro deve ter 40 casas")
    }
}
```

- [ ] **Step 2: Executar teste e validar falha**

Run: `go test -v ./internal/banco3d -run TestCityBoardGeneratorFallback`  
Expected: FAIL

- [ ] **Step 3: Implementar gerador com catálogo pré-carregado e suporte a IA em `generator.go`**

Adicionar catálogo com cidades populares (São Paulo, Rio de Janeiro, Curitiba, Belo Horizonte, Salvador, Nova York, Paris) e algoritmo de substituição de nomes para qualquer entrada arbitrária com 4 candidatos a ícone da cidade.

- [ ] **Step 4: Executar teste e validar sucesso**

Run: `go test -v ./internal/banco3d -run TestCityBoardGeneratorFallback`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/banco3d/generator.go internal/banco3d/generator_test.go
git commit -m "feat(banco3d): add city board generator with rich offline catalog"
```

---

### Task 4: Sistema de Votação Relâmpago do Local Mais Icônico

**Files:**
- Create: `internal/banco3d/voting.go`
- Test: `internal/banco3d/voting_test.go`

**Interfaces:**
- Produces: `VotingSession`, `NewVotingSession(candidates []string, duration time.Duration) *VotingSession`, `Vote(playerID string, candidateIndex int) error`, `CloseAndElect(board *Board) (string, error)`

- [ ] **Step 1: Escrever teste de votação e atribuição da casa nobre**

```go
func TestVotingElectsIconicProperty(t *testing.T) {
    candidates := []string{"Av. Batel", "Jardim Botânico", "Rua XV de Novembro"}
    v := NewVotingSession(candidates, 15*time.Second)
    v.Vote("p1", 1) // Jardim Botânico
    v.Vote("p2", 1) // Jardim Botânico
    v.Vote("p3", 0) // Av. Batel
    
    winner, err := v.GetWinner()
    if err != nil || winner != "Jardim Botânico" {
        t.Fatalf("esperado vencedor 'Jardim Botânico', recebido %s", winner)
    }
}
```

- [ ] **Step 2: Executar teste e validar falha**

Run: `go test -v ./internal/banco3d -run TestVotingElectsIconicProperty`  
Expected: FAIL

- [ ] **Step 3: Implementar lógica de votação e atribuição da propriedade Boardwalk (casa 39)**

Implementar `VotingSession`, registro concorrente seguro de votos (`sync.Mutex`), desempate determinístico e substituição na casa de maior aluguel (`Index: 39`).

- [ ] **Step 4: Executar teste e validar sucesso**

Run: `go test -v ./internal/banco3d -run TestVotingElectsIconicProperty`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/banco3d/voting.go internal/banco3d/voting_test.go
git commit -m "feat(banco3d): add iconic location pre-game voting system"
```

---

### Task 5: Hub de Salas & Protocolo WebSocket em Tempo Real

**Files:**
- Create: `internal/banco3d/room.go`
- Create: `internal/banco3d/hub.go`
- Test: `internal/banco3d/room_test.go`

**Interfaces:**
- Produces: `Room`, `RoomHub`, `NewRoomHub() *RoomHub`, `GetOrCreateRoom(roomID, city string) *Room`

- [ ] **Step 1: Escrever teste de criação de sala e broadcast de eventos**

```go
func TestRoomLifecycle(t *testing.T) {
    hub := NewRoomHub()
    room, err := hub.CreateRoom("SP-123", "São Paulo")
    if err != nil || room == nil {
        t.Fatalf("falha ao criar sala: %v", err)
    }
    if room.City != "São Paulo" {
        t.Errorf("cidade incorreta: %s", room.City)
    }
}
```

- [ ] **Step 2: Executar teste e validar falha**

Run: `go test -v ./internal/banco3d -run TestRoomLifecycle`  
Expected: FAIL

- [ ] **Step 3: Implementar gerenciador de salas e protocolo JSON de eventos WebSocket**

Mensagens cliente -> servidor: `join`, `vote`, `roll_dice`, `buy_property`, `pass_turn`.
Mensagens servidor -> cliente: `room_state`, `voting_started`, `voting_result`, `turn_change`, `dice_rolled`, `player_moved`, `property_bought`, `bankrupt`.

- [ ] **Step 4: Executar teste e validar sucesso**

Run: `go test -v ./internal/banco3d -run TestRoomLifecycle`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/banco3d/room.go internal/banco3d/hub.go internal/banco3d/room_test.go
git commit -m "feat(banco3d): add websocket room hub and multiplayer protocol"
```

---

### Task 6: Servidor Standalone Executável

**Files:**
- Create: `cmd/banco3d/main.go`

- [ ] **Step 1: Criar servidor HTTP/WebSocket com flag de porta**

Servir rotas `/` (lobby), `/sala/:id` (sala do jogo), `/ws/:id` (conexão WebSocket) e arquivos estáticos de `web/banco3d/`. Porta padrão `:8081` (para não conflitar com a porta 8080 do Detetive).

- [ ] **Step 2: Testar compilação do binário**

Run: `go build -o bin/banco3d.exe ./cmd/banco3d`  
Expected: Binário compilado sem erros.

- [ ] **Step 3: Commit**

```bash
git add cmd/banco3d/main.go
git commit -m "feat(banco3d): add standalone server entrypoint"
```

---

### Task 7: Frontend Zero-Scroll HUD & WebSocket Client

**Files:**
- Create: `web/banco3d/index.html`
- Create: `web/banco3d/css/style.css`
- Create: `web/banco3d/js/app.js`

- [ ] **Step 1: Criar layout 100vh em HTML/CSS**
- Container de tela cheia sem rolagem vertical (`overflow: hidden; height: 100vh`).
- Barra de topo com saldo, lista compacta de jogadores e indicador de turno ativo.
- Overlay inicial de Lobby (digitar cidade, link para copiar e seleção de cor/peça).
- Modal de votação do ponto mais icônico com cronômetro de 15s.
- Barra de ações rápidas no rodapé ("Rolar Dados", "Comprar", "Passar").

- [ ] **Step 2: Implementar cliente WebSocket em `app.js`**
- Conexão e reconexão automática.
- Tratamento de mensagens de estado e disparo de eventos para o renderizador 3D.

- [ ] **Step 3: Commit**

```bash
git add web/banco3d/index.html web/banco3d/css/style.css web/banco3d/js/app.js
git commit -m "feat(banco3d): implement zero-scroll HUD and websocket client"
```

---

### Task 8: Renderizador 3D Arcade do Tabuleiro & Dados Físicos em Three.js

**Files:**
- Create: `web/banco3d/js/board3d.js`

- [ ] **Step 1: Implementar cena 3D e câmera isométrica superior (Opção C)**
- Criação do tabuleiro quadrado de 40 casas em perspectiva aérea suave.
- Renderização procedural dos blocos de cores, nomes das ruas e casas de canto.
- Peões 3D para cada jogador se movendo suavemente entre as casas.
- Dados 3D no centro da mesa com animação física ao rolar.

- [ ] **Step 2: Testar renderização no navegador e fluxo completo**
- Abrir no navegador, criar uma sala com cidade (ex: "São Paulo"), votar no local e rolar dados na mesa 3D.

- [ ] **Step 3: Commit**

```bash
git add web/banco3d/js/board3d.js
git commit -m "feat(banco3d): add Three.js 3D tabletop board and physical dice"
```
