package banco

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// Charset defines unambiguous uppercase characters for 4-letter room codes.
const Charset = "ABCDEFGHJKLMNPQRSTUVWXY"

// Manager coordinates concurrent in-memory and SQLite-backed Banco Imobiliário game sessions.
type Manager struct {
	mu    sync.RWMutex
	db    *sql.DB
	games map[string]*Game
	rooms map[string]*Room
}

// NewManager creates a thread-safe Manager instance with optional SQLite persistence.
func NewManager(dbs ...*sql.DB) *Manager {
	var db *sql.DB
	if len(dbs) > 0 {
		db = dbs[0]
	}

	m := &Manager{
		db:    db,
		games: make(map[string]*Game),
		rooms: make(map[string]*Room),
	}

	if db != nil {
		_ = m.initSchema()
	}

	return m
}

func (m *Manager) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS banco_games (
		id TEXT PRIMARY KEY,
		city TEXT,
		state_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS banco_rooms (
		code TEXT PRIMARY KEY,
		state_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := m.db.Exec(schema)
	return err
}

// CreateGame instantiates, registers, and optionally persists a new Game.
func (m *Manager) CreateGame(opts GameOptions) *Game {
	m.mu.Lock()
	defer m.mu.Unlock()

	game := NewGame(opts)
	m.games[game.state.ID] = game

	if m.db != nil {
		m.saveGameLocked(game)
	}

	return game
}

// GetGame retrieves an active game by ID from memory or SQLite.
func (m *Manager) GetGame(gameID string) *Game {
	m.mu.RLock()
	game, exists := m.games[gameID]
	m.mu.RUnlock()

	if exists {
		return game
	}

	if m.db != nil {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Double-check under write lock
		if game, exists := m.games[gameID]; exists {
			return game
		}

		var stateJSON string
		err := m.db.QueryRow("SELECT state_json FROM banco_games WHERE id = ?", gameID).Scan(&stateJSON)
		if err == nil {
			var state GameState
			if err := json.Unmarshal([]byte(stateJSON), &state); err == nil {
				cards := GenerateLuckCards(state.City)
				loadedGame := &Game{
					state: state,
					cards: cards,
				}
				m.games[gameID] = loadedGame
				return loadedGame
			}
		}
	}

	return nil
}

// SaveGame persists the current game state to SQLite if a database is configured.
func (m *Manager) SaveGame(game *Game) error {
	if m.db == nil || game == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveGameLocked(game)
}

func (m *Manager) saveGameLocked(game *Game) error {
	state := game.GetState()
	bytes, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`
		INSERT INTO banco_games (id, city, state_json, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = CURRENT_TIMESTAMP
	`, state.ID, state.City, string(bytes))
	return err
}

// DeleteGame removes a finished or abandoned game session.
func (m *Manager) DeleteGame(gameID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.games, gameID)
	if m.db != nil {
		_, _ = m.db.Exec("DELETE FROM banco_games WHERE id = ?", gameID)
	}
}

// GenerateRoomCode produces an unambiguous 4-letter uppercase code.
func (m *Manager) GenerateRoomCode() (string, error) {
	charsetLen := big.NewInt(int64(len(Charset)))
	for attempts := 0; attempts < 1000; attempts++ {
		var sb strings.Builder
		for i := 0; i < 4; i++ {
			idx, err := rand.Int(rand.Reader, charsetLen)
			if err != nil {
				return "", err
			}
			sb.WriteByte(Charset[idx.Int64()])
		}
		code := sb.String()
		if _, exists := m.rooms[code]; !exists {
			if m.db != nil {
				var count int
				_ = m.db.QueryRow("SELECT COUNT(*) FROM banco_rooms WHERE code = ?", code).Scan(&count)
				if count == 0 {
					return code, nil
				}
			} else {
				return code, nil
			}
		}
	}
	return "", errors.New("falha ao gerar código único de sala após múltiplas tentativas")
}

// CreateRoom registers a new multiplayer lobby with a 4-letter code.
func (m *Manager) CreateRoom(hostName, city, difficulty string, age int, schoolGrade string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	code, err := m.GenerateRoomCode()
	if err != nil {
		return nil, err
	}

	hName := strings.TrimSpace(hostName)
	if hName == "" {
		hName = "Investidor Anfitrião"
	}
	cName := strings.TrimSpace(city)
	if cName == "" {
		cName = "São Paulo"
	}
	diff := strings.TrimSpace(difficulty)
	if diff == "" {
		diff = "medio"
	}

	hostPlayer := &RoomPlayer{
		ID:     1,
		Name:   hName,
		Avatar: "🎩",
		Color:  "#d4a359",
		IsHost: true,
	}

	room := &Room{
		Code:        code,
		HostName:    hName,
		City:        cName,
		Difficulty:  diff,
		Age:         age,
		SchoolGrade: schoolGrade,
		Status:      "lobby",
		Players:     []*RoomPlayer{hostPlayer},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	m.rooms[code] = room
	if m.db != nil {
		_ = m.saveRoomLocked(room)
	}

	return cloneRoom(room), nil
}

// GetRoom retrieves a multiplayer room by its 4-letter code.
func (m *Manager) GetRoom(code string) (*Room, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.RLock()
	r, exists := m.rooms[normalized]
	m.mu.RUnlock()

	if exists {
		return cloneRoom(r), nil
	}

	if m.db != nil {
		m.mu.Lock()
		defer m.mu.Unlock()

		if r, exists := m.rooms[normalized]; exists {
			return cloneRoom(r), nil
		}

		var stateJSON string
		err := m.db.QueryRow("SELECT state_json FROM banco_rooms WHERE code = ?", normalized).Scan(&stateJSON)
		if err == nil {
			var loadedRoom Room
			if err := json.Unmarshal([]byte(stateJSON), &loadedRoom); err == nil {
				m.rooms[normalized] = &loadedRoom
				return cloneRoom(&loadedRoom), nil
			}
		}
	}

	return nil, errors.New("sala não encontrada")
}

// JoinRoom adds a friend to an existing lobby.
func (m *Manager) JoinRoom(code, playerName, avatar, color string) (*RoomPlayer, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[normalized]
	if !exists {
		return nil, errors.New("sala não encontrada")
	}

	if room.Status != "lobby" {
		return nil, errors.New("esta sala já iniciou ou foi finalizada")
	}

	if len(room.Players) >= 4 {
		return nil, errors.New("a sala já atingiu a capacidade máxima (4 jogadores)")
	}

	name := strings.TrimSpace(playerName)
	if name == "" {
		name = fmt.Sprintf("Amigo %d", len(room.Players)+1)
	}

	pAvatar := strings.TrimSpace(avatar)
	if pAvatar == "" {
		pAvatar = "🤝"
	}

	pColor := strings.TrimSpace(color)
	if pColor == "" {
		colors := []string{"#1890ff", "#52c41a", "#eb2f96", "#fa8c16"}
		pColor = colors[len(room.Players)%len(colors)]
	}

	newPlayer := &RoomPlayer{
		ID:     len(room.Players) + 1,
		Name:   name,
		Avatar: pAvatar,
		Color:  pColor,
		IsHost: false,
	}

	room.Players = append(room.Players, newPlayer)

	if m.db != nil {
		_ = m.saveRoomLocked(room)
	}

	return newPlayer, nil
}

// StartRoomGame launches a collective Banco Imobiliário match with all room participants.
func (m *Manager) StartRoomGame(code string) (*Game, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[normalized]
	if !exists {
		return nil, errors.New("sala não encontrada")
	}

	if room.Status != "lobby" {
		return nil, errors.New("a partida desta sala já foi iniciada")
	}

	if len(room.Players) == 0 {
		return nil, errors.New("a sala não possui jogadores")
	}

	// Prepare GameOptions
	host := room.Players[0]
	var friendNames []string
	for i := 1; i < len(room.Players); i++ {
		friendNames = append(friendNames, room.Players[i].Name)
	}

	opts := GameOptions{
		City:        room.City,
		Age:         room.Age,
		SchoolGrade: room.SchoolGrade,
		Difficulty:  room.Difficulty,
		HumanName:   host.Name,
		Friends:     friendNames,
		BotsCount:   0,
	}

	// If fewer than 2 total players, add 1 bot to keep competition lively
	if len(room.Players) < 2 {
		opts.BotsCount = 1
	}

	game := NewGame(opts)

	// Apply custom avatars and colors to human players
	for i, rp := range room.Players {
		if i < len(game.state.Players) {
			game.state.Players[i].Avatar = rp.Avatar
			game.state.Players[i].Color = rp.Color
		}
	}

	room.Status = "in_progress"
	room.GameID = game.state.ID

	m.games[game.state.ID] = game

	if m.db != nil {
		_ = m.saveRoomLocked(room)
		_ = m.saveGameLocked(game)
	}

	return game, nil
}

func (m *Manager) saveRoomLocked(room *Room) error {
	bytes, err := json.Marshal(room)
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`
		INSERT INTO banco_rooms (code, state_json, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(code) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = CURRENT_TIMESTAMP
	`, room.Code, string(bytes))
	return err
}

func cloneRoom(r *Room) *Room {
	if r == nil {
		return nil
	}
	playersCopy := make([]*RoomPlayer, len(r.Players))
	for i, p := range r.Players {
		pCopy := *p
		playersCopy[i] = &pCopy
	}
	rCopy := *r
	rCopy.Players = playersCopy
	return &rCopy
}
