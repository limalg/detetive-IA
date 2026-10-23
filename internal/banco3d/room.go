package banco3d

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
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
	SalaryAmount    int        `json:"salary_amount"` // Salário ao passar ou parar no início (padrão 200)
	Rules           RuleConfig `json:"rules"`
}

// DefaultRoomConfig returns standard canonical match configuration.
func DefaultRoomConfig() RoomConfig {
	return RoomConfig{
		MaxPlayers:      4,
		Bots:            BotConfig{Count: 0, Difficulty: "normal"},
		StartingBalance: 1500,
		TurnSeconds:     30,
		SalaryAmount:    200,
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

// GameEvent represents an action in the match for frontend animation and logs.
type GameEvent struct {
	Seq        int       `json:"seq"`
	ID         string    `json:"id"`
	Type       string    `json:"type"` // "roll", "move", "card", "salary", "buy", "rent", "pass", "jail", "bankrupt", "build", "trade"
	Turn       int       `json:"turn,omitempty"`
	PlayerID   string    `json:"player_id"`
	PlayerName string    `json:"player_name,omitempty"`
	Deck       string    `json:"deck,omitempty"` // "chance" | "chest"
	Title      string    `json:"title,omitempty"`
	Effect     string    `json:"effect,omitempty"`
	ImageKind  string    `json:"image_kind,omitempty"`
	What       string    `json:"what,omitempty"` // "house" | "hotel"
	Dice       *[2]int   `json:"dice,omitempty"`
	Die1       int       `json:"die1,omitempty"`
	Die2       int       `json:"die2,omitempty"`
	From       int       `json:"from,omitempty"`
	To         int       `json:"to,omitempty"`
	Amount     int       `json:"amount,omitempty"`
	Value      int       `json:"value,omitempty"`
	ToPlayerID string    `json:"to_player_id,omitempty"`
	Tile       int       `json:"tile,omitempty"`
	TileIndex  int       `json:"tile_index,omitempty"`
	Text       string    `json:"text"`
	Message    string    `json:"message,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	TS         int64     `json:"ts"` // unix ms
}

// Room represents a multiplayer room for 40-tile Banco Imobiliário.
type Room struct {
	mu               sync.Mutex
	ID               string         `json:"id"`
	City             string         `json:"city"`
	Config           RoomConfig     `json:"config"`
	Board            *Board         `json:"board"`
	Candidates       []string       `json:"candidates"`
	Voting           *VotingSession `json:"voting,omitempty"`
	Game             *Game          `json:"game"`
	PendingTrade     *TradeProposal `json:"pending_trade,omitempty"`
	TurnTimeout      time.Duration  `json:"turn_timeout"`
	LastActivity     time.Time      `json:"last_activity"`
	Events           []GameEvent    `json:"events"`
	EventSeq         int            `json:"event_seq"`
	DecisionDeadline *time.Time     `json:"decision_deadline,omitempty"`
	PendingDecision  string         `json:"pending_decision,omitempty"`
	DecisionPlayerID string         `json:"decision_player_id,omitempty"`
}

// RecordEvent records an action in the room event stream.
func (r *Room) RecordEvent(evt GameEvent) {
	r.EventSeq++
	now := time.Now()
	evt.Seq = r.EventSeq
	evt.ID = fmt.Sprintf("evt_%d_%d", r.EventSeq, now.UnixMilli())
	evt.Timestamp = now
	evt.TS = now.UnixMilli()
	if evt.Value == 0 && evt.Amount != 0 {
		evt.Value = evt.Amount
	} else if evt.Amount == 0 && evt.Value != 0 {
		evt.Amount = evt.Value
	}
	if evt.TileIndex == 0 && evt.Tile != 0 {
		evt.TileIndex = evt.Tile
	} else if evt.Tile == 0 && evt.TileIndex != 0 {
		evt.Tile = evt.TileIndex
	}
	if evt.Message == "" {
		evt.Message = evt.Text
	}
	if evt.Dice != nil {
		evt.Die1 = evt.Dice[0]
		evt.Die2 = evt.Dice[1]
	}
	if r.Game != nil {
		evt.Turn = r.Game.TurnIndex
	}
	r.Events = append(r.Events, evt)
	if len(r.Events) > 30 {
		r.Events = r.Events[len(r.Events)-30:]
	}
}

// RecordTurnRollAndMove records the standard sequence of events for a player's dice roll:
// roll -> move -> card -> salary (and rent if applicable).
func (r *Room) RecordTurnRollAndMove(playerID, pName string, dice *DiceResult) {
	if dice == nil {
		return
	}

	// 1. roll
	diceArr := [2]int{dice.Die1, dice.Die2}
	r.RecordEvent(GameEvent{
		Type:       "roll",
		PlayerID:   playerID,
		PlayerName: pName,
		Dice:       &diceArr,
		Die1:       dice.Die1,
		Die2:       dice.Die2,
		From:       dice.OldPosition,
		To:         dice.NewPosition,
		Text:       fmt.Sprintf("%s tirou %d (%d+%d)", pName, dice.Total, dice.Die1, dice.Die2),
	})

	// 2. move
	tileName := ""
	if r.Board != nil && dice.NewPosition >= 0 && dice.NewPosition < len(r.Board.Tiles) {
		tileName = r.Board.Tiles[dice.NewPosition].Name
	}
	moveText := fmt.Sprintf("%s avançou para a casa %d", pName, dice.NewPosition)
	if tileName != "" {
		moveText = fmt.Sprintf("%s avançou para %s (casa %d)", pName, tileName, dice.NewPosition)
	}
	r.RecordEvent(GameEvent{
		Type:       "move",
		PlayerID:   playerID,
		PlayerName: pName,
		From:       dice.OldPosition,
		To:         dice.NewPosition,
		Tile:       dice.NewPosition,
		Text:       moveText,
	})

	// 3. card (se tirou carta Sorte ou Revés)
	if dice.CardDrawn != nil {
		card := dice.CardDrawn.Card
		deck := strings.ToLower(string(card.Deck))
		if deck == "" {
			deck = strings.ToLower(string(card.Type))
		}
		if deck != "chance" && deck != "chest" {
			deck = "chance"
		}
		effect := string(card.Effect)
		if effect == "" {
			effect = string(card.Action)
		}
		cardAmount := card.Amount
		if cardAmount == 0 {
			cardAmount = card.Value
		}
		if cardAmount == 0 && dice.CardDrawn.MoneyDelta != 0 {
			cardAmount = dice.CardDrawn.MoneyDelta
		}
		cardText := card.Text
		if cardText == "" {
			cardText = fmt.Sprintf("%s tirou carta: %s", pName, card.Title)
		}

		imgKind := card.ImageKind
		if imgKind == "" && dice.CardDrawn.ImageKind != "" {
			imgKind = dice.CardDrawn.ImageKind
		}

		r.RecordEvent(GameEvent{
			Type:       "card",
			PlayerID:   playerID,
			PlayerName: pName,
			Deck:       deck,
			Title:      card.Title,
			Text:       cardText,
			Amount:     cardAmount,
			Value:      cardAmount,
			Effect:     effect,
			ImageKind:  imgKind,
			Tile:       dice.NewPosition,
		})

		// Evento destroy se alguma construção foi destruída
		if dice.CardDrawn.DestroyedWhat != "" {
			r.RecordEvent(GameEvent{
				Type:       "destroy",
				PlayerID:   playerID,
				PlayerName: pName,
				Tile:       dice.CardDrawn.DestroyedTile,
				TileIndex:  dice.CardDrawn.DestroyedTile,
				What:       dice.CardDrawn.DestroyedWhat,
				Text:       fmt.Sprintf("%s teve 1 %s destruído na casa %d", pName, dice.CardDrawn.DestroyedWhat, dice.CardDrawn.DestroyedTile),
			})
		}
	}

	// 4. salary (se houve salário pelo dado ou pela carta)
	totalSalary := dice.SalaryPaid
	if dice.CardDrawn != nil && dice.CardDrawn.SalaryPaid > 0 {
		totalSalary += dice.CardDrawn.SalaryPaid
	}
	if totalSalary > 0 {
		r.RecordEvent(GameEvent{
			Type:       "salary",
			PlayerID:   playerID,
			PlayerName: pName,
			Amount:     totalSalary,
			Value:      totalSalary,
			Text:       fmt.Sprintf("%s recebeu salário de R$ %d ao passar/parar no Início", pName, totalSalary),
		})
	}

	// 5. rent (se pagou aluguel)
	if dice.RentPaid > 0 {
		r.RecordEvent(GameEvent{
			Type:       "rent",
			PlayerID:   playerID,
			PlayerName: pName,
			Amount:     dice.RentPaid,
			Value:      dice.RentPaid,
			ToPlayerID: dice.RentTo,
			Tile:       dice.NewPosition,
			Text:       fmt.Sprintf("%s pagou R$ %d de aluguel", pName, dice.RentPaid),
		})
	}
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
		if userCfg.SalaryAmount > 0 {
			cfg.SalaryAmount = userCfg.SalaryAmount
		}
		if userCfg.Bots.Count >= 0 {
			cfg.Bots = userCfg.Bots
		}
		cfg.Rules = userCfg.Rules
	}
	if cfg.SalaryAmount <= 0 {
		cfg.SalaryAmount = 200
	}
	game.SalaryAmount = cfg.SalaryAmount

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
		if err == nil {
			bp.IsBot = true
			if r.Config.StartingBalance > 0 {
				bp.Money = r.Config.StartingBalance
			}
		}
	}

	err := r.Game.Start()
	if err != nil {
		return err
	}
	r.LastActivity = time.Now()
	return nil
}

// executeBotTurnLocked executes a bot player's turn (caller must hold r.mu).
func (r *Room) executeBotTurnLocked(bot *Player) error {
	if !r.Game.IsStarted || r.Game.IsOver || r.Game.CurrentTurnPlayerID != bot.ID {
		return errors.New("não é a vez deste bot")
	}

	diff := r.Config.Bots.Difficulty
	if diff == "" {
		diff = "normal"
	}

	d1 := 1 + rand.Intn(6)
	d2 := 1 + rand.Intn(6)
	diceRes, err := r.Game.RollDice(bot.ID, d1, d2)
	if err != nil {
		_ = r.Game.AutoPassTurn()
		return err
	}

	r.RecordTurnRollAndMove(bot.ID, bot.Name, diceRes)

	tile := &r.Board.Tiles[bot.Position]

	// Buy decision
	if (tile.Type == TileProperty || tile.Type == TileStation || tile.Type == TileUtility) && !bot.InJail {
		if tile.OwnerID == "" && BotAI.ShouldBuy(r.Game, bot, tile, diff) {
			if buyErr := r.Game.BuyProperty(bot.ID); buyErr == nil {
				r.RecordEvent(GameEvent{
					Type:       "buy",
					PlayerID:   bot.ID,
					PlayerName: bot.Name,
					Tile:       tile.Index,
					Amount:     tile.Price,
					Text:       fmt.Sprintf("%s comprou %s por R$ %d", bot.Name, tile.Name, tile.Price),
				})
			}
		}
	}

	// Build decision
	for {
		tileIdx, canBuild := BotAI.DecideBuild(r.Game, bot, diff)
		if !canBuild {
			break
		}
		if bldErr := r.Game.BuildHouse(bot.ID, tileIdx); bldErr == nil {
			t := &r.Board.Tiles[tileIdx]
			r.RecordEvent(GameEvent{
				Type:       "build",
				PlayerID:   bot.ID,
				PlayerName: bot.Name,
				Tile:       tileIdx,
				Amount:     t.HouseCost,
				Text:       fmt.Sprintf("%s construiu casa em %s ($ %d)", bot.Name, t.Name, t.HouseCost),
			})
		} else {
			break
		}
	}

	// Pass turn
	_ = r.Game.AutoPassTurn()
	r.RecordEvent(GameEvent{
		Type:       "pass",
		PlayerID:   bot.ID,
		PlayerName: bot.Name,
		Text:       fmt.Sprintf("%s passou a vez", bot.Name),
	})

	r.LastActivity = time.Now()
	return nil
}

// ExecuteBotTurn runs an automated turn for a bot player.
func (r *Room) ExecuteBotTurn(bot *Player) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.executeBotTurnLocked(bot)
}

// CheckTurnTimeout verifies if the decision deadline or inactive turn timeout has passed.
func (r *Room) CheckTurnTimeout() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.Game.IsStarted || r.Game.IsOver {
		return false
	}

	// 1. Pending buy decision deadline expired -> auto-pass
	if r.PendingDecision == "buy" && r.DecisionDeadline != nil && time.Now().After(*r.DecisionDeadline) {
		r.PendingDecision = ""
		r.DecisionDeadline = nil
		r.DecisionPlayerID = ""
		_ = r.Game.AutoPassTurn()
		r.LastActivity = time.Now()
		next := r.Game.getPlayer(r.Game.CurrentTurnPlayerID)
		if next != nil && next.IsBot {
			_ = r.executeBotTurnLocked(next)
		}
		return true
	}

	// 2. If current player is bot and no decision is pending, execute bot turn
	cur := r.Game.getPlayer(r.Game.CurrentTurnPlayerID)
	if cur != nil && cur.IsBot && r.PendingDecision == "" {
		_ = r.executeBotTurnLocked(cur)
		r.LastActivity = time.Now()
		return true
	}

	// 3. Inactive turn timeout (30s) - apenas quando não há decisão de compra pendente
	if r.PendingDecision == "" && time.Since(r.Game.TurnStartTime) >= r.TurnTimeout {
		r.PendingDecision = ""
		r.DecisionDeadline = nil
		r.DecisionPlayerID = ""
		_ = r.Game.AutoPassTurn()
		r.LastActivity = time.Now()
		next := r.Game.getPlayer(r.Game.CurrentTurnPlayerID)
		if next != nil && next.IsBot {
			_ = r.executeBotTurnLocked(next)
		}
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

// AcceptTrade executes the pending trade atomically, ensuring caller is the recipient.
func (r *Room) AcceptTrade(callerPlayerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.PendingTrade == nil || r.PendingTrade.Status != "pending" {
		return errors.New("nenhuma proposta de troca pendente")
	}

	if callerPlayerID != "" && r.PendingTrade.ReceiverID != callerPlayerID {
		return errors.New("apenas o destinatário pode aceitar a proposta")
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

// DeclineTrade rejects the current trade offer, ensuring caller is the recipient.
func (r *Room) DeclineTrade(callerPlayerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.PendingTrade == nil || r.PendingTrade.Status != "pending" {
		return errors.New("nenhuma proposta de troca pendente")
	}

	if callerPlayerID != "" && r.PendingTrade.ReceiverID != callerPlayerID {
		return errors.New("apenas o destinatário pode recusar a proposta")
	}

	r.PendingTrade.Status = "rejected"
	r.PendingTrade = nil
	r.LastActivity = time.Now()
	return nil
}
