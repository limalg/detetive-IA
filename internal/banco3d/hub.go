package banco3d

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultStoragePath defines where room states are persisted.
const DefaultStoragePath = "data/banco3d/rooms.json"

var storageFileMu sync.Mutex

// RoomSnapshot represents the serialized form of a room for persistence.
type RoomSnapshot struct {
	ID               string         `json:"id"`
	City             string         `json:"city"`
	Config           RoomConfig     `json:"config"`
	Board            *Board         `json:"board"`
	Candidates       []string       `json:"candidates"`
	Voting           *VotingSession `json:"voting,omitempty"`
	Game             *Game          `json:"game"`
	PendingTrade     *TradeProposal `json:"pending_trade,omitempty"`
	Events           []GameEvent    `json:"events"`
	EventSeq         int            `json:"event_seq"`
	PendingDecision  string         `json:"pending_decision,omitempty"`
	DecisionDeadline *time.Time     `json:"decision_deadline,omitempty"`
	LastActivity     time.Time      `json:"last_activity"`
}

// RoomHub coordinates all active rooms and connections.
type RoomHub struct {
	mu          sync.RWMutex
	Rooms       map[string]*Room
	storagePath string
}

// NewRoomHub creates a thread-safe room registry and recovers rooms from disk.
func NewRoomHub(storagePaths ...string) *RoomHub {
	path := DefaultStoragePath
	if len(storagePaths) > 0 {
		path = storagePaths[0]
	}
	hub := &RoomHub{
		Rooms:       make(map[string]*Room),
		storagePath: path,
	}
	if path != ":memory:" && path != "" {
		_ = hub.LoadFromDisk()
	}
	return hub
}

func (h *RoomHub) saveToDiskLocked() error {
	if h.storagePath == ":memory:" || h.storagePath == "" {
		return nil
	}

	storageFileMu.Lock()
	defer storageFileMu.Unlock()

	snaps := make(map[string]RoomSnapshot)
	for id, r := range h.Rooms {
		r.mu.Lock()
		boardToSave := r.Board
		if r.Game != nil && r.Game.Board != nil {
			boardToSave = r.Game.Board
			r.Board = r.Game.Board
		}
		snaps[id] = RoomSnapshot{
			ID:               r.ID,
			City:             r.City,
			Config:           r.Config,
			Board:            boardToSave,
			Candidates:       r.Candidates,
			Voting:           r.Voting,
			Game:             r.Game,
			PendingTrade:     r.PendingTrade,
			Events:           r.Events,
			EventSeq:         r.EventSeq,
			PendingDecision:  r.PendingDecision,
			DecisionDeadline: r.DecisionDeadline,
			LastActivity:     r.LastActivity,
		}
		r.mu.Unlock()
	}

	_ = os.MkdirAll(filepath.Dir(h.storagePath), 0755)

	data, err := json.MarshalIndent(snaps, "", "  ")
	if err != nil {
		return err
	}

	if h.storagePath == DefaultStoragePath {
		_ = os.WriteFile("data/banco3d/rooms", data, 0644)
		_ = os.WriteFile("data/banco3d_rooms.json", data, 0644)
	}

	return os.WriteFile(h.storagePath, data, 0644)
}

// SaveToDisk writes active rooms state to a persistent JSON snapshot.
func (h *RoomHub) SaveToDisk() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.saveToDiskLocked()
}

// LoadFromDisk reloads rooms from a persistent JSON snapshot.
func (h *RoomHub) LoadFromDisk() error {
	if h.storagePath == ":memory:" || h.storagePath == "" {
		return nil
	}

	storageFileMu.Lock()
	defer storageFileMu.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	paths := []string{h.storagePath, "data/banco3d/rooms.json", "data/banco3d/rooms", "data/banco3d_rooms.json"}
	var data []byte
	var readErr error
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			data = b
			break
		} else {
			readErr = err
		}
	}
	if len(data) == 0 {
		if os.IsNotExist(readErr) {
			return nil
		}
		return readErr
	}

	var snaps map[string]RoomSnapshot
	if err := json.Unmarshal(data, &snaps); err != nil {
		return err
	}

	for id, s := range snaps {
		r := &Room{
			ID:               s.ID,
			City:             s.City,
			Config:           s.Config,
			Board:            s.Board,
			Candidates:       s.Candidates,
			Voting:           s.Voting,
			Game:             s.Game,
			PendingTrade:     s.PendingTrade,
			TurnTimeout:      time.Duration(s.Config.TurnSeconds) * time.Second,
			Events:           s.Events,
			EventSeq:         s.EventSeq,
			PendingDecision:  s.PendingDecision,
			DecisionDeadline: s.DecisionDeadline,
			LastActivity:     s.LastActivity,
		}
		if r.TurnTimeout <= 0 {
			r.TurnTimeout = 30 * time.Second
		}
		if r.Game != nil && r.Game.Board != nil {
			r.Board = r.Game.Board
		}
		h.Rooms[id] = r
	}

	return nil
}

// CreateRoom builds and registers a new room.
func (h *RoomHub) CreateRoom(roomID, city string, opts ...*RoomConfig) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.Rooms[roomID]; exists {
		return nil, errors.New("sala já existe")
	}

	room, err := NewRoom(roomID, city, opts...)
	if err != nil {
		return nil, err
	}

	h.Rooms[roomID] = room
	_ = h.saveToDiskLocked()
	return room, nil
}

// GetOrCreateRoom returns the room if present or creates a new one.
func (h *RoomHub) GetOrCreateRoom(roomID, city string, opts ...*RoomConfig) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.Rooms[roomID]; exists {
		return room, nil
	}

	room, err := NewRoom(roomID, city, opts...)
	if err != nil {
		return nil, err
	}

	h.Rooms[roomID] = room
	_ = h.saveToDiskLocked()
	return room, nil
}

// GetRoom retrieves a room by ID, attempting a disk reload if missing in memory.
func (h *RoomHub) GetRoom(roomID string) (*Room, bool) {
	h.mu.RLock()
	r, ok := h.Rooms[roomID]
	h.mu.RUnlock()
	if ok {
		return r, true
	}

	// Tenta recarregar do disco mesmo sem restart do servidor
	_ = h.LoadFromDisk()

	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok = h.Rooms[roomID]
	return r, ok
}

// RemoveRoom deletes an ended or abandoned room.
func (h *RoomHub) RemoveRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.Rooms, roomID)
	_ = h.saveToDiskLocked()
}
