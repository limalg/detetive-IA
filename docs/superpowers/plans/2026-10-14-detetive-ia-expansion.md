# Plano de Implementação: Detetive IA - Expansão Procedural, Lobbies e Visual Noir

**Data:** 14 de Outubro de 2026  
**Objetivo:** Transformar o Detetive IA em um jogo de mistério procedural dinâmico com lobbies por código (Jackbox-style), geração de casos centrada nos jogadores + bots, confronto de evidências e integração de ativos visuais via Higgsfield AI, mantendo a arquitetura de monolito Go puro ultraleve (<30MB RAM em execução).

---

## 1. Classificação das Melhores Ideias (Baseadas na Pesquisa do Oracle)

1. **Geração Inversa em Grafo (Backwards Mystery Weaver):**  
   O backend em Go é a única fonte da verdade. O caso é construído a partir da solução (Vítima + Assassino + Arma + Motivo) e expandido na **Regra das Três Pistas** (Física, Contradição de Álibi, Documental). A LLM nunca alucina o crime; ela apenas atua como a voz do suspeito baseando-se estritamente no subgrafo de conhecimento do personagem.
2. **Lobbies por Código de 4 Letras (Jackbox Style):**  
   Sistema de salas onde o jogador cria uma sala, recebe um código amigável (ex: `NOIR`), preenche sua persona (ou gera aleatoriamente) e amigos entram pelo celular/navegador. Se jogar solo ou com poucos amigos, o jogo preenche com Bots coerentes.
3. **Mecânica de Confrontação com Medidor de Estresse:**  
   Suspeitos possuem um medidor de estresse (0-100%). Perguntas genéricas não quebram o suspeito; apenas confrontá-lo com uma pista que contradiz seu álibi eleva o estresse e extrai reações dramáticas e confissões parciais.
4. **Gestão Cirúrgica dos 10 Tokens do Higgsfield AI:**  
   Geração focada em 4 retratos mestres de arquétipos noir, 2 cenários de crime atmosféricos, 2 armas periciais e micro-animações de reação/fala combinadas com efeitos CSS no frontend.

---

## 2. Divisão de Tarefas (Micro-Etapas com TDD)

### Task 1: Gerenciamento de Salas e Códigos em Go (`internal/room`)
- **Arquivos:** `internal/room/room.go`, `internal/room/room_test.go`
- **Responsável:** `Cipher`
- **Objetivo:**
  - Modelo `Room`: `Code` (4 caracteres sem ambiguidade), `HostID`, `Status` (`lobby`, `active`, `finished`), `Players` (ID, Username, PersonaName, Role, Secret, IsHost, IsBot).
  - Gerador de códigos curtos (exclui `I, 1, O, 0, Z, 2`).
  - Operações: `CreateRoom`, `JoinRoom`, `GetRoom`, `AddBot`, `StartRoom`.
  - Persistência e thread-safety com `sync.RWMutex` e SQLite.
- **Teste TDD:** Criação de sala, colisão de código, adição de jogadores e bots, bloqueio de sala ativa.

### Task 2: Motor Procedural de Casos em Grafo (`internal/game/weaver.go`)
- **Arquivos:** `internal/game/weaver.go`, `internal/game/weaver_test.go`
- **Responsável:** `Cipher`
- **Objetivo:**
  - `Weaver`: Recebe lista de jogadores da sala.
  - Se `len(jogadores) < 4`, cria Bots arquetípicos com nomes, profissões noir e segredos compatíveis.
  - Sorteia Vítima, Assassino, Arma e Motivo.
  - Gera 3 pistas independentes conectando o assassino e inocentes (com álibis que contêm contradições lógicas claras para o assassino e álibis sólidos com pequenos segredos para os inocentes).
  - Salva o caso no SQLite (`cases`, `suspects`, `clues`, `investigations`).
- **Teste TDD:** Verificação da regra das três pistas e garantia de que o assassino possui exatamente uma contradição identificável.

### Task 3: Mecânica de Confronto de Pistas & Estresse (`internal/game/ai_interrogator.go`)
- **Arquivos:** `internal/game/ai_interrogator.go`, `internal/server/handlers.go`
- **Responsável:** `Cipher`
- **Objetivo:**
  - Extensão do endpoint `/api/game/interrogate` para receber `{ suspect_id, message, confront_clue_id }`.
  - Verificação no backend: a pista confrontada toca no álibi falso do suspeito?
  - Se sim, eleva estresse (+40%), muda estado de ânimo e injeta instrução na LLM para quebrar o álibi.
  - Se não, o suspeito desdenha da pista ("Isso não prova nada, detetive!").
- **Teste TDD:** Testar interrogatório normal vs interrogatório com pista contraditória.

### Task 4: Ativos Visuais e Integração do Higgsfield AI
- **Arquivos:** `web/static/assets/`, `internal/server/server.go`
- **Responsável:** `Antigravity` + `Tracker`
- **Objetivo:**
  - Acessar o portal `higgs` e gerar com os 10 tokens:
    - Retrato 1: A Femme Fatale
    - Retrato 2: O Magnata Corrupto
    - Retrato 3: O Mordomo / Guarda-Livros
    - Retrato 4: O Investigador / Jornalista
    - Cenário 1: Gabinete da Mansão com sombras venesianas
    - Cenário 2: Beco chuvoso sob iluminação âmbar
    - Arma 1: Adaga Veneziana
    - Arma 2: Cálice com Cianeto
  - Salvar ativos em `web/static/assets/` e embutir no binário monolítico via `go:embed`.
  - Implementar efeito dinâmico de fala (ondas sonoras de rádio antigo + pulso de expressão quando o suspeito responde).

### Task 5: Interface do Lobby, Códigos e Confrontação (`web/static/js/app.js`, `index.html`)
- **Arquivos:** `web/templates/index.html`, `web/static/js/app.js`, `web/static/css/style.css`
- **Responsável:** `Tracker`
- **Objetivo:**
  - Tela de Seleção de Modo: "Investigação Solo (Partida Rápida com Bots)" ou "Criar / Entrar em Sala Coletiva".
  - Entrada de Código de Sala com 4 caracteres e link compartilhável.
  - Ficha de Persona Rápida: Nome, Profissão Noir, Segredo Oculto e botão "🎲 Gerar Persona Aleatória".
  - Lobby com lista de participantes em tempo real e botão do Host "Iniciar Investigação".
  - No Interrogatório: Seletor de "Confrontar com Pista Coletada" e Barra de Estresse do Suspeito.

### Task 6: QA Automatizado, Testes E2E e Benchmark de Memória
- **Arquivos:** `tests/integration/room_e2e_test.go`
- **Responsável:** `Tracker`
- **Objetivo:**
  - Simular fluxo completo: Criação de sala por código -> Entrada de 2 jogadores + 2 bots -> Início da rodada procedural -> Obtenção de pista -> Confrontação no interrogatório com quebra de álibi -> Acusação correta -> Veredito.
  - Medição de memória sob carga para assegurar WorkingSet < 30MB (orçamento de 1GB RAM).
