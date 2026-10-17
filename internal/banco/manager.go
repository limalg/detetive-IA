package banco

import (
	"sync"
)

// Manager coordinates concurrent in-memory Banco Imobiliário game sessions.
type Manager struct {
	mu    sync.RWMutex
	games map[string]*Game
}

// NewManager creates a thread-safe Manager instance.
func NewManager() *Manager {
	return &Manager{
		games: make(map[string]*Game),
	}
}

// CreateGame instantiates and registers a new Game.
func (m *Manager) CreateGame(opts GameOptions) *Game {
	m.mu.Lock()
	defer m.mu.Unlock()

	game := NewGame(opts)
	m.games[game.state.ID] = game
	return game
}

// GetGame retrieves an active game by ID.
func (m *Manager) GetGame(gameID string) *Game {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.games[gameID]
}

// DeleteGame removes a finished or abandoned game session.
func (m *Manager) DeleteGame(gameID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.games, gameID)
}
