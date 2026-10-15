package room

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// Ambiguity-free uppercase letter charset (excludes I, O, Z and digits 1, 0, 2).
const Charset = "ABCDEFGHJKLMNPQRSTUVWXY"

var (
	ErrRoomNotFound    = errors.New("room not found")
	ErrRoomNotJoinable = errors.New("room is already active or finished")
	ErrNotHost         = errors.New("only room host can perform this action")
	ErrRoomFull        = errors.New("room is full (maximum 8 players)")
)

// Player represents a human player or bot in a room.
type Player struct {
	ID          int64     `json:"id"`
	RoomID      int64     `json:"room_id"`
	UserID      int64     `json:"user_id"`
	Username    string    `json:"username"`
	PersonaName string    `json:"persona_name"`
	Role        string    `json:"role"`
	Secret      string    `json:"secret,omitempty"`
	IsHost      bool      `json:"is_host"`
	IsBot       bool      `json:"is_bot"`
	JoinedAt    time.Time `json:"joined_at"`
}

// Room represents an active mystery lobby.
type Room struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	HostID    int64     `json:"host_id"`
	Status    string    `json:"status"` // "lobby", "active", "finished"
	CaseID    *int64    `json:"case_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Players   []Player  `json:"players"`
}

// Manager coordinates room creation, state synchronization, and thread-safe access.
type Manager struct {
	mu    sync.RWMutex
	db    *sql.DB
	rooms map[string]*Room
}

// NewManager initializes the Room Manager and ensures database schema exists.
func NewManager(db *sql.DB) (*Manager, error) {
	if db != nil {
		schema := `
		CREATE TABLE IF NOT EXISTS rooms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT UNIQUE NOT NULL,
			host_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'lobby',
			case_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS room_players (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			persona_name TEXT NOT NULL,
			role TEXT NOT NULL,
			secret TEXT NOT NULL,
			is_host BOOLEAN NOT NULL DEFAULT 0,
			is_bot BOOLEAN NOT NULL DEFAULT 0,
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_rooms_code ON rooms(code);
		CREATE INDEX IF NOT EXISTS idx_room_players_room_id ON room_players(room_id);
		`
		if _, err := db.Exec(schema); err != nil {
			return nil, fmt.Errorf("failed to create room schema: %w", err)
		}
	}

	return &Manager{
		db:    db,
		rooms: make(map[string]*Room),
	}, nil
}

// GenerateCode generates a unique 4-character code without ambiguous characters.
func (m *Manager) GenerateCode() (string, error) {
	charsetLen := big.NewInt(int64(len(Charset)))
	for attempts := 0; attempts < 1000; attempts++ {
		var codeBuilder strings.Builder
		for i := 0; i < 4; i++ {
			idx, err := rand.Int(rand.Reader, charsetLen)
			if err != nil {
				return "", err
			}
			codeBuilder.WriteByte(Charset[idx.Int64()])
		}
		code := codeBuilder.String()

		if _, exists := m.rooms[code]; !exists {
			if m.db != nil {
				var existingID int64
				err := m.db.QueryRow("SELECT id FROM rooms WHERE code = ?", code).Scan(&existingID)
				if errors.Is(err, sql.ErrNoRows) {
					return code, nil
				}
			} else {
				return code, nil
			}
		}
	}
	return "", errors.New("failed to generate unique room code after multiple attempts")
}

// CreateRoom registers a new game room with an unambiguous 4-letter code.
func (m *Manager) CreateRoom(hostID int64, hostUsername string, hostPersona string, hostRole string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	code, err := m.GenerateCode()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var roomID int64

	if m.db != nil {
		res, err := m.db.Exec(`
			INSERT INTO rooms (code, host_id, status, created_at)
			VALUES (?, ?, 'lobby', ?)
		`, code, hostID, now.Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("failed to insert room into database: %w", err)
		}
		roomID, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	}

	hostPlayer := Player{
		RoomID:      roomID,
		UserID:      hostID,
		Username:    hostUsername,
		PersonaName: hostPersona,
		Role:        hostRole,
		IsHost:      true,
		IsBot:       false,
		JoinedAt:    now,
	}

	if m.db != nil {
		pRes, err := m.db.Exec(`
			INSERT INTO room_players (room_id, user_id, username, persona_name, role, secret, is_host, is_bot, joined_at)
			VALUES (?, ?, ?, ?, ?, '', 1, 0, ?)
		`, roomID, hostID, hostUsername, hostPersona, hostRole, now.Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("failed to insert host player: %w", err)
		}
		hostPlayer.ID, _ = pRes.LastInsertId()
	}

	r := &Room{
		ID:        roomID,
		Code:      code,
		HostID:    hostID,
		Status:    "lobby",
		CreatedAt: now,
		Players:   []Player{hostPlayer},
	}

	m.rooms[code] = r
	return cloneRoom(r), nil
}

// GetRoom retrieves a room by its 4-character code (case-insensitive).
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
		// Double check under write lock
		if r, exists := m.rooms[normalized]; exists {
			return cloneRoom(r), nil
		}

		loadedRoom, err := m.loadRoomFromDB(normalized)
		if err != nil {
			return nil, err
		}
		m.rooms[normalized] = loadedRoom
		return cloneRoom(loadedRoom), nil
	}

	return nil, ErrRoomNotFound
}

// JoinRoom adds a new player to an existing lobby.
func (m *Manager) JoinRoom(code string, userID int64, username string, personaName string, role string) (*Room, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.rooms[normalized]
	if !exists {
		if m.db != nil {
			var err error
			r, err = m.loadRoomFromDB(normalized)
			if err != nil {
				return nil, err
			}
			m.rooms[normalized] = r
		} else {
			return nil, ErrRoomNotFound
		}
	}

	if r.Status != "lobby" {
		return nil, ErrRoomNotJoinable
	}

	if len(r.Players) >= 8 {
		return nil, ErrRoomFull
	}

	// Check if already in room
	for i, p := range r.Players {
		if p.UserID == userID && userID > 0 {
			r.Players[i].PersonaName = personaName
			r.Players[i].Role = role
			return cloneRoom(r), nil
		}
	}

	now := time.Now().UTC()
	newPlayer := Player{
		RoomID:      r.ID,
		UserID:      userID,
		Username:    username,
		PersonaName: personaName,
		Role:        role,
		IsHost:      false,
		IsBot:       false,
		JoinedAt:    now,
	}

	if m.db != nil {
		pRes, err := m.db.Exec(`
			INSERT INTO room_players (room_id, user_id, username, persona_name, role, secret, is_host, is_bot, joined_at)
			VALUES (?, ?, ?, ?, ?, '', 0, 0, ?)
		`, r.ID, userID, username, personaName, role, now.Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("failed to save joining player: %w", err)
		}
		newPlayer.ID, _ = pRes.LastInsertId()
	}

	r.Players = append(r.Players, newPlayer)
	return cloneRoom(r), nil
}

// AddBot adds an automated character to the room.
func (m *Manager) AddBot(code string, botName string, botRole string, botSecret string) (*Room, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.rooms[normalized]
	if !exists {
		return nil, ErrRoomNotFound
	}

	if r.Status != "lobby" {
		return nil, ErrRoomNotJoinable
	}

	if len(r.Players) >= 8 {
		return nil, ErrRoomFull
	}

	now := time.Now().UTC()
	botPlayer := Player{
		RoomID:      r.ID,
		UserID:      0,
		Username:    "[BOT] " + botName,
		PersonaName: botName,
		Role:        botRole,
		Secret:      botSecret,
		IsHost:      false,
		IsBot:       true,
		JoinedAt:    now,
	}

	if m.db != nil {
		pRes, err := m.db.Exec(`
			INSERT INTO room_players (room_id, user_id, username, persona_name, role, secret, is_host, is_bot, joined_at)
			VALUES (?, 0, ?, ?, ?, ?, 0, 1, ?)
		`, r.ID, botPlayer.Username, botName, botRole, botSecret, now.Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("failed to save bot player: %w", err)
		}
		botPlayer.ID, _ = pRes.LastInsertId()
	}

	r.Players = append(r.Players, botPlayer)
	return cloneRoom(r), nil
}

// StartRoom locks the room and changes status to active.
func (m *Manager) StartRoom(code string, hostID int64) (*Room, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))

	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.rooms[normalized]
	if !exists {
		return nil, ErrRoomNotFound
	}

	if r.HostID != hostID {
		return nil, ErrNotHost
	}

	r.Status = "active"

	if m.db != nil {
		_, err := m.db.Exec("UPDATE rooms SET status = 'active' WHERE id = ?", r.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update room status in database: %w", err)
		}
	}

	return cloneRoom(r), nil
}

func (m *Manager) loadRoomFromDB(code string) (*Room, error) {
	var r Room
	var createdAtStr string
	err := m.db.QueryRow(`
		SELECT id, code, host_id, status, case_id, created_at
		FROM rooms WHERE code = ?
	`, code).Scan(&r.ID, &r.Code, &r.HostID, &r.Status, &r.CaseID, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)

	rows, err := m.db.Query(`
		SELECT id, room_id, user_id, username, persona_name, role, secret, is_host, is_bot, joined_at
		FROM room_players WHERE room_id = ? ORDER BY id ASC
	`, r.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p Player
		var joinedAtStr string
		if err := rows.Scan(&p.ID, &p.RoomID, &p.UserID, &p.Username, &p.PersonaName, &p.Role, &p.Secret, &p.IsHost, &p.IsBot, &joinedAtStr); err != nil {
			return nil, err
		}
		p.JoinedAt, _ = time.Parse(time.RFC3339, joinedAtStr)
		r.Players = append(r.Players, p)
	}

	return &r, nil
}

func cloneRoom(r *Room) *Room {
	if r == nil {
		return nil
	}
	clone := *r
	clone.Players = make([]Player, len(r.Players))
	copy(clone.Players, r.Players)
	return &clone
}
