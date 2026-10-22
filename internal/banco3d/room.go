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

// BotConfig specifies AI bot count and strategic profile.
type BotConfig struct {
	Count      int    `json:"count"`
	Difficulty string `json:"difficulty"`
}

// RuleConfig defines rule modifications for custom matches.
type RuleConfig struct {
	JailBail          bool `json:"jail_bail"`
	JailTripleDoubles bool `json:"jail_triple_doubles"`
	GetOutFreeCard    bool `json:"get_out_free_card"`
	Auction           bool `json:"auction"`
}

// RoomConfig encapsulates match customization parameters.
type RoomConfig struct {
	MaxPlayers      int        `json:"max_players"`
	Bots            BotConfig  `json:"bots"`
	StartingBalance int        `json:"starting_balance"`
	TurnSeconds     int        `json:"turn_seconds"`
	Rules           RuleConfig `json:"rules"`
}

// DefaultRoomConfig returns standard canonical match configuration.
func DefaultRoomConfig() RoomConfig {
	return RoomConfig{
		MaxPlayers:      4,
		Bots:            BotConfig{Count: 0, Difficulty: "normal"},
		StartingBalance: 1500,
		TurnSeconds:     30,
		Rules: RuleConfig{
			JailBail:          true,
			JailTripleDoubles: true,
			GetOutFreeCard:    true,
			Auction:           false,
		},
	}
}

// TradeProposal represents a pending negotiation between two players.
type TradeProposal struct {
	ID             string    `json:"id"`
	SenderID       string    `json:"sender_id"`
	ReceiverID     string    `json:"receiver_id"`
	OfferedTiles   []int     `json:"offered_tiles"`
	OfferedCash    int       `json:"offered_cash"`
	RequestedTiles []int     `json:"requested_tiles"`
	RequestedCash  int       `json:"requested_cash"`
	CreatedAt      time.Time `json:"created_at"`
	Status         string    `json:"status"` // "pending", "accepted", "rejected"
}

// Room represents a multiplayer room for 40-tile Banco Imobiliário.
type Room struct {
	mu           sync.Mutex
	ID           string         `json:"id"`
	City         string         `json:"city"`
	Config       RoomConfig     `json:"config"`
	Board        *Board         `json:"board"`
	Candidates   []string       `json:"candidates"`
	Voting       *VotingSession `json:"voting,omitempty"`
	Game         *Game          `json:"game"`
	PendingTrade *TradeProposal `json:"pending_trade,omitempty"`
	TurnTimeout  time.Duration  `json:"turn_timeout"`
	LastActivity time.Time      `json:"last_activity"`
}

// NewRoom constructs a room with city board generation and game instance.
func NewRoom(id, city string, opts ...*RoomConfig) (*Room, error) {
	board, candidates, err := GenerateCityBoard(city, "")
	if err != nil {
		return nil, err
	}

	game := NewGame(id, board)

	cfg := DefaultRoomConfig()
	if len(opts) > 0 && opts[0] != nil {
		userCfg := opts[0]
		if userCfg.MaxPlayers >= 2 && userCfg.MaxPlayers <= 6 {
			cfg.MaxPlayers = userCfg.MaxPlayers
		}
		if userCfg.StartingBalance >= 500 && userCfg.StartingBalance <= 10000 {
			cfg.StartingBalance = userCfg.StartingBalance
		}
		if userCfg.TurnSeconds >= 10 && userCfg.TurnSeconds <= 90 {
			cfg.TurnSeconds = userCfg.TurnSeconds
		}
		if userCfg.Bots.Count >= 0 {
			cfg.Bots = userCfg.Bots
		}
		cfg.Rules = userCfg.Rules
	}

	return &Room{
		ID:           id,
		City:         city,
		Config:       cfg,
		Board:        board,
		Candidates:   candidates,
		Game:         game,
		TurnTimeout:  time.Duration(cfg.TurnSeconds) * time.Second,
		LastActivity: time.Now(),
	}, nil
}

// JoinPlayer registers a player in the room.
func (r *Room) JoinPlayer(name, color, piece string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Game.Players) >= r.Config.MaxPlayers {
		return nil, errors.New("sala cheia")
	}

	p, err := r.Game.AddPlayer(name, color, piece)
	if err != nil {
		return nil, err
	}
	if r.Config.StartingBalance > 0 {
		p.Money = r.Config.StartingBalance
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

// StartGame starts the game match, auto-filling bots if configured or if only 1 player joined.
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	botNames := []struct {
		name  string
		color string
		piece string
	}{
		{"Gabriel", "#22c55e", "thimble"},
		{"Larissa", "#facc15", "shoe"},
		{"Pedro", "#ef4444", "hat"},
		{"Sofia", "#a855f7", "car"},
		{"Marcos", "#f97316", "ship"},
	}

	neededBots := r.Config.Bots.Count
	if neededBots <= 0 && len(r.Game.Players) == 1 {
		neededBots = 3
	}

	for i := 0; i < neededBots && len(r.Game.Players) < r.Config.MaxPlayers; i++ {
		b := botNames[i%len(botNames)]
		bp, err := r.Game.AddPlayer(b.name, b.color, b.piece)
		if err == nil && r.Config.StartingBalance > 0 {
			bp.Money = r.Config.StartingBalance
		}
	}

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

// ProposeTrade registers a new trade offer in the room.
func (r *Room) ProposeTrade(proposal TradeProposal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	proposal.CreatedAt = time.Now()
	proposal.Status = "pending"
	r.PendingTrade = &proposal
	r.LastActivity = time.Now()
	return nil
}

// AcceptTrade executes the pending trade atomically.
func (r *Room) AcceptTrade() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.PendingTrade == nil || r.PendingTrade.Status != "pending" {
		return errors.New("nenhuma proposta de troca pendente")
	}

	err := r.Game.TradeExchange(TradeExchangeRequest{
		SenderID:       r.PendingTrade.SenderID,
		ReceiverID:     r.PendingTrade.ReceiverID,
		OfferedTiles:   r.PendingTrade.OfferedTiles,
		OfferedCash:    r.PendingTrade.OfferedCash,
		RequestedTiles: r.PendingTrade.RequestedTiles,
		RequestedCash:  r.PendingTrade.RequestedCash,
	})
	if err != nil {
		return err
	}

	r.PendingTrade.Status = "accepted"
	r.PendingTrade = nil
	r.LastActivity = time.Now()
	return nil
}

// DeclineTrade rejects the current trade offer.
func (r *Room) DeclineTrade() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.PendingTrade != nil {
		r.PendingTrade.Status = "rejected"
		r.PendingTrade = nil
	}
	r.LastActivity = time.Now()
	return nil
}
