package banco3d

import (
	"errors"
	"sync"
)

// RoomHub coordinates all active rooms and connections.
type RoomHub struct {
	mu    sync.RWMutex
	Rooms map[string]*Room
}

// NewRoomHub creates a thread-safe room registry.
func NewRoomHub() *RoomHub {
	return &RoomHub{
		Rooms: make(map[string]*Room),
	}
}

// CreateRoom builds and registers a new room.
func (h *RoomHub) CreateRoom(roomID, city string) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.Rooms[roomID]; exists {
		return nil, errors.New("sala já existe")
	}

	room, err := NewRoom(roomID, city)
	if err != nil {
		return nil, err
	}

	h.Rooms[roomID] = room
	return room, nil
}

// GetOrCreateRoom returns the room if present or creates a new one.
func (h *RoomHub) GetOrCreateRoom(roomID, city string) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.Rooms[roomID]; exists {
		return room, nil
	}

	room, err := NewRoom(roomID, city)
	if err != nil {
		return nil, err
	}

	h.Rooms[roomID] = room
	return room, nil
}

// GetRoom retrieves a room by ID.
func (h *RoomHub) GetRoom(roomID string) (*Room, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	r, ok := h.Rooms[roomID]
	return r, ok
}

// RemoveRoom deletes an ended or abandoned room.
func (h *RoomHub) RemoveRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.Rooms, roomID)
}
