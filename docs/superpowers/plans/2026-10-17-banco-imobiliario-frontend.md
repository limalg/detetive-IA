# Banco Imobiliário IA - Frontend Implementation Plan (FIX-030)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the complete visual frontend for Banco Imobiliário IA, including the Hub card integration, setup/lobby screen with bot and invite modes, AI customization modal, and the emerald-felt art déco 28-tile interactive tabletop board.

**Architecture:** A lightweight, vanilla JavaScript and semantic HTML5 single-page application integrated into the existing Go monolith. The 28-tile board is rendered via a responsive 8x8 CSS Grid perimeter on an emerald felt background with gold art déco styling. State is synchronized via REST endpoints (`/api/banco/start`, `/api/banco/state`, `/api/banco/roll`, `/api/banco/buy`, `/api/banco/build`, `/api/banco/end-turn`) and diegetic Web Audio sound effects.

**Tech Stack:** Go 1.22+ (embed.FS), Vanilla HTML5/CSS3, JavaScript (ES6+), Web Audio API, SVG/Unicode diegetic art.

**Spec:** FIX-030 prompt requirements from User & Architecture from FIX-029 backend commit `1f6afbd`.

## Global Constraints

- Monolith in Go with embedded assets (`embed.FS`).
- Total memory usage must remain within 1GB RAM budget (frontend zero heavy frameworks, zero external CDN dependencies).
- Semantic commits, clean code, TDD verification (`go test ./...`).
- Maestri note 'Projeto' kept updated with status in descending order.

## Review Focus

1. Active Hub Card transition: clicking Banco Imobiliário when a game is in progress goes straight to the board; otherwise opens the lobby.
2. 28-tile CSS grid alignment: tiles 0 to 27 must map clockwise around the 8x8 perimeter without gaps or overlapping the center area.
3. Turn flow controls: Roll Dice, Buy Property, Build, and End Turn buttons must properly enable/disable based on `can_roll`, `can_buy`, `can_end_turn`, and whether it is the human's turn.
4. Autonomous bot moves: calling `/api/banco/end-turn` triggers bot turns on the backend and immediately reflects new positions, balances, purchases, and log entries in the UI.
5. Audio feedback: `playPaper`, `playStamp`, and `playTypewriter` must be called at diegetically appropriate moments (rolling dice, buying property, turn handover).

---

### Task 1: Web Tests & HTML Scaffolding for Banco Imobiliário

**Files:**
- Modify: `web/templates/index.html`
- Modify: `web/web_test.go`

**Interfaces:**
- Consumes: Existing DOM IDs and CSS classes in `web/templates/index.html`.
- Produces: `#banco-section`, `#banco-lobby-section`, `#banco-custom-modal`, `#banco-board-grid`, `#banco-dice-panel`, `#banco-action-panel`, `#banco-portfolio-panel`, `#banco-players-panel`, `#banco-ticker-feed`.

- [ ] **Step 1: Write failing test in `web/web_test.go`**
  Add `TestBancoImobiliarioFrontend` to verify required DOM elements and attributes exist in the embedded HTML template.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./web -run TestBancoImobiliarioFrontend`
  Expected: FAIL (missing DOM IDs like `banco-section`).

- [ ] **Step 3: Update `web/templates/index.html`**
  - Update the fallback Hub card for `banco-imobiliario-ia` to be active (`hub-card-active`, `Disponível`, with actions `open-banco` and `open-custom-banco`).
  - Add `#banco-lobby-section` with:
    * Mode selector: "🤖 Iniciar com Bots (IA)" and "✉️ Convidar Amigos".
    * Bots panel with AI agent chips, difficulty badge, and "🚀 Iniciar com Bots" button.
    * Invite friends panel with link generator and copy button.
    * "⚙️ Personalizar com IA" button opening the customization modal.
  - Add `#banco-custom-modal` with City, Age, Grade, Difficulty, Bots Count, Starting Balance, Friends list.
  - Add `#banco-section` with emerald felt table container, 8x8 perimeter grid, center control desk (dice, actions, banker diary ticker), portfolio sidebar, and players roster.
  - Add Hub return navigation button in `#banco-section`.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./web -run TestBancoImobiliarioFrontend`
  Expected: PASS.

- [ ] **Step 5: Commit**
  Run `git commit` with message `feat(banco): add Banco Imobiliário HTML structure and web template tests`.

---

### Task 2: Emerald Felt Art Déco CSS Styling

**Files:**
- Modify: `web/static/css/style.css`

**Interfaces:**
- Consumes: `#banco-section`, `#banco-board-grid`, `#banco-lobby-section`, `.banco-tile`, `.banco-pawn`.
- Produces: Emerald felt styling (`#082a1c` background, radial texture), gold art déco double borders (`#d4af37`), 8x8 perimeter grid tile placement, 3D dice styling, property color bands, pawn markers, and responsive layouts.

- [ ] **Step 1: Write CSS rules in `web/static/css/style.css`**
  - Grid layout: 8 columns x 8 rows, mapping tiles 0..27 to perimeter coordinates and center area to row 2..7 / col 2..7.
  - Emerald felt theme: textured background, subtle vignette, gold leaf typography, art déco trims.
  - Property tiles: color group bars (brown, light blue, pink, orange, red, green, dark blue, station, utility, special), price tags, house/hotel badges.
  - Player pawns: colored badges/avatars positioned on tiles with smooth transitions.
  - Dice: 3D die faces with dot pips and roll shake animations.
  - Action buttons and banker ticker styling.
  - Responsive media queries for tablet and mobile screens.

- [ ] **Step 2: Run test to verify CSS integrity**
  Run: `go test ./web`
  Expected: PASS.

- [ ] **Step 3: Commit**
  Run `git commit` with message `feat(banco): add emerald felt and art deco board styling`.

---

### Task 3: JavaScript State Management & Board Interactions

**Files:**
- Modify: `web/static/js/app.js`

**Interfaces:**
- Consumes: Endpoints `/api/banco/start`, `/api/banco/state`, `/api/banco/roll`, `/api/banco/buy`, `/api/banco/build`, `/api/banco/end-turn`.
- Produces: Functions `showBancoLobbyView()`, `showBancoView()`, `renderBancoBoard()`, `renderBancoPlayers()`, `renderBancoPortfolio()`, `rollBancoDice()`, `buyBancoProperty()`, `buildBancoHouse()`, `endBancoTurn()`, `startBancoGame()`.

- [ ] **Step 1: Implement Hub integration in `web/static/js/app.js`**
  - Update `HUB_FALLBACK_GAMES` so Banco Imobiliário is active.
  - Update `renderHubCard` to render specific buttons for `banco-imobiliario-ia`.
  - Handle `open-banco` and `open-custom-banco` clicks in hub grid listener.

- [ ] **Step 2: Implement Banco Lobby & Customization Modal in `web/static/js/app.js`**
  - Lobby mode switching (bots vs invite friends).
  - Quick-start with bots calling `/api/banco/start` with default city and options.
  - Customization form submission gathering city, age, school_grade, difficulty, bots_count, starting_balance, friends.

- [ ] **Step 3: Implement Board Rendering & Perimeter Tile Placement**
  - Map each of the 28 tiles to its exact 8x8 grid coordinates.
  - Render color band, tile name, price, owner color, and houses/hotel.
  - Render player pawns inside each player's current tile position.

- [ ] **Step 4: Implement Game Actions & Bot Turns**
  - "Rolar Dados" button calling `/api/banco/roll` with sound effects and dice animations.
  - "Comprar Imóvel" button calling `/api/banco/buy`.
  - "Construir" button calling `/api/banco/build`.
  - "Passar a Vez" calling `/api/banco/end-turn`, playing bot actions, updating banker ticker feed.
  - Check active game on load: if cookie or active game exists, restore session.

- [ ] **Step 5: Run tests**
  Run: `go test ./...`
  Expected: PASS.

- [ ] **Step 6: Commit**
  Run `git commit` with message `feat(banco): implement Banco Imobiliário game loop and interactive board UI`.

---

### Task 4: Diegetic Audio, Visual Polish, Verification & Maestri Sync

**Files:**
- Modify: `web/static/js/app.js`
- Modify: `web/static/css/style.css`
- Update: Maestri note "Projeto" (FIX-030)

**Interfaces:**
- Consumes: `playPaper`, `playTypewriter`, `playStamp`, Maestri CLI.
- Produces: Complete verified experience, updated Maestri note, semantic git commit.

- [ ] **Step 1: Audio and transition polish**
  - Wire dice roll sound, paper purchase sound, typewriter event logs, and stamp turn confirmation.
  - Ensure Hub return navigation works smoothly from anywhere.

- [ ] **Step 2: Run full test suite & static validation**
  Run: `go build ./...`
  Run: `go test ./...`
  Expected: All builds and tests pass with 0 errors.

- [ ] **Step 3: Update Maestri note 'Projeto'**
  Prepend FIX-030 entry at line 1 of note "Projeto".

- [ ] **Step 4: Commit and finalize**
  Run `git commit` with semantic message.
