package banco3d

import (
	"errors"
	"sync"
	"time"
)

// WSMessage represents a websocket event exchanged with clients.
type WSMessage struct {
	Type      string      `json:"type"`
	PlayerID  string      `json:"player_id,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// Room represents a multiplayer room for 40-tile Banco Imobiliário.
type Room struct {
	mu           sync.Mutex
	ID           string         `json:"id"`
	City         string         `json:"city"`
	Board        *Board         `json:"board"`
	Candidates   []string       `json:"candidates"`
	Voting       *VotingSession `json:"voting,omitempty"`
	Game         *Game          `json:"game"`
	TurnTimeout  time.Duration  `json:"turn_timeout"`
	LastActivity time.Time      `json:"last_activity"`
}

// NewRoom constructs a room with city board generation and game instance.
func NewRoom(id, city string) (*Room, error) {
	board, candidates, err := GenerateCityBoard(city, "")
	if err != nil {
		return nil, err
	}

	game := NewGame(id, board)

	return &Room{
		ID:           id,
		City:         city,
		Board:        board,
		Candidates:   candidates,
		Game:         game,
		TurnTimeout:  30 * time.Second,
		LastActivity: time.Now(),
	}, nil
}

// JoinPlayer registers a player in the room.
func (r *Room) JoinPlayer(name, color, piece string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, err := r.Game.AddPlayer(name, color, piece)
	if err != nil {
		return nil, err
	}
	r.LastActivity = time.Now()
	return p, nil
}

// ReconnectPlayer restores an existing player connection.
func (r *Room) ReconnectPlayer(playerID string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.Game.getPlayer(playerID)
	if p == nil {
		return nil, errors.New("jogador não encontrado na sala")
	}
	r.LastActivity = time.Now()
	return p, nil
}

// StartVoting triggers the 15s pre-game voting session.
func (r *Room) StartVoting() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Game.IsStarted {
		return errors.New("partida já foi iniciada")
	}

	r.Voting = NewVotingSession(r.Candidates, 15*time.Second)
	r.LastActivity = time.Now()
	return nil
}

// CastVote records a player's vote.
func (r *Room) CastVote(playerID string, candIdx int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Voting == nil {
		return errors.New("nenhuma votação em andamento")
	}
	r.LastActivity = time.Now()
	return r.Voting.Vote(playerID, candIdx)
}

// ConcludeVoting calculates winner, updates tile 39, and closes voting.
func (r *Room) ConcludeVoting() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Voting == nil {
		return "", errors.New("nenhuma votação ativa")
	}

	winner, err := r.Voting.CloseAndElect(r.Board)
	if err != nil {
		return "", err
	}
	r.LastActivity = time.Now()
	return winner, nil
}

// StartGame starts the game match.
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	err := r.Game.Start()
	if err != nil {
		return err
	}
	r.LastActivity = time.Now()
	return nil
}

// CheckTurnTimeout verifies if the 30-second inactive turn timeout has passed.
func (r *Room) CheckTurnTimeout() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.Game.IsStarted || r.Game.IsOver {
		return false
	}

	if time.Since(r.Game.TurnStartTime) >= r.TurnTimeout {
		_ = r.Game.AutoPassTurn()
		r.LastActivity = time.Now()
		return true
	}
	return false
}
