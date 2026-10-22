package banco3d

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Player represents a participant in the game.
type Player struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Color             string `json:"color"`
	Piece             string `json:"piece"`
	Money             int    `json:"money"`
	Position          int    `json:"position"`
	InJail            bool   `json:"in_jail"`
	IsJailed          bool   `json:"is_jailed"`
	JailTurns         int    `json:"jail_turns"`
	GetOutOfJailCards int    `json:"get_out_of_jail_cards"`
	IsBankrupt        bool   `json:"is_bankrupt"`
	Properties        []int  `json:"properties"` // Índices das casas possuídas
}

// DiceResult describes the outcome of rolling two dice.
type DiceResult struct {
	Die1        int         `json:"die1"`
	Die2        int         `json:"die2"`
	Total       int         `json:"total"`
	IsDouble    bool        `json:"is_double"`
	OldPosition int         `json:"old_position"`
	NewPosition int         `json:"new_position"`
	RentPaid    int         `json:"rent_paid,omitempty"`
	RentTo      string      `json:"rent_to,omitempty"`
	CardDrawn   *CardResult `json:"card_drawn,omitempty"`
}

// Game represents the state and rules engine for a 40-tile match.
type Game struct {
	mu                  sync.Mutex
	RoomID              string       `json:"room_id"`
	Board               *Board       `json:"board"`
	Players             []*Player    `json:"players"`
	CurrentTurnPlayerID string       `json:"current_turn_player_id"`
	TurnIndex           int          `json:"turn_index"`
	DiceRolledThisTurn  bool         `json:"dice_rolled_this_turn"`
	TurnStartTime       time.Time    `json:"turn_start_time"`
	IsStarted           bool         `json:"is_started"`
	IsOver              bool         `json:"is_over"`
	WinnerID            string       `json:"winner_id,omitempty"`
	Deck                *DeckManager `json:"deck,omitempty"`
	LastCard            *Card        `json:"last_card,omitempty"`
	LastCardResult      *CardResult  `json:"last_card_result,omitempty"`
}

// NewGame initializes a game instance for the given room and board.
func NewGame(roomID string, board *Board) *Game {
	var deck *DeckManager
	if board != nil {
		deck = NewDeckManager(board.City)
	}
	return &Game{
		RoomID:    roomID,
		Board:     board,
		Players:   make([]*Player, 0),
		TurnIndex: 0,
		Deck:      deck,
	}
}

// AddPlayer registers a new player before start.
func (g *Game) AddPlayer(name, color, piece string) (*Player, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.IsStarted {
		return nil, errors.New("partida já iniciada")
	}
	if len(g.Players) >= 6 {
		return nil, errors.New("sala cheia (máximo 6 jogadores)")
	}

	p := &Player{
		ID:         fmt.Sprintf("p%d", len(g.Players)+1),
		Name:       name,
		Color:      color,
		Piece:      piece,
		Money:      1500, // R$ 1.500 inicial conforme spec
		Position:   0,
		Properties: make([]int, 0),
	}
	g.Players = append(g.Players, p)
	return p, nil
}

// Start begins the match.
func (g *Game) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.Players) < 2 {
		return errors.New("mínimo de 2 jogadores necessário")
	}

	g.IsStarted = true
	g.CurrentTurnPlayerID = g.Players[0].ID
	g.TurnIndex = 0
	g.DiceRolledThisTurn = false
	g.TurnStartTime = time.Now()
	return nil
}

// RollDice simulates dice rolling and resolves movement/rent.
// If d1 and d2 are > 0, they are used (for testing); otherwise rolled randomly.
func (g *Game) RollDice(playerID string, d1, d2 int) (*DiceResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.IsStarted {
		return nil, errors.New("jogo não iniciado")
	}
	if g.CurrentTurnPlayerID != playerID {
		return nil, errors.New("não é a vez deste jogador")
	}
	if g.DiceRolledThisTurn {
		return nil, errors.New("dados já rolados neste turno")
	}

	if d1 == 0 && d2 == 0 {
		d1 = rand.Intn(6) + 1
		d2 = rand.Intn(6) + 1
	}

	total := d1 + d2
	p := g.getPlayer(playerID)
	if p == nil || p.IsBankrupt {
		return nil, errors.New("jogador inválido ou falido")
	}

	oldPos := p.Position
	newPos := (oldPos + total) % 40
	p.Position = newPos

	// Checar bônus do GO (0)
	if newPos == 0 {
		p.Money += 400 // R$ 400 se cair exatamente no GO
	} else if newPos < oldPos {
		p.Money += 200 // R$ 200 ao passar pelo GO
	}

	g.DiceRolledThisTurn = true

	res := &DiceResult{
		Die1:        d1,
		Die2:        d2,
		Total:       total,
		IsDouble:    d1 == d2,
		OldPosition: oldPos,
		NewPosition: newPos,
	}

	// Resolução automática de taxa/aluguel/cartas
	tile := &g.Board.Tiles[newPos]
	switch tile.Type {
	case TileTax:
		tax := tile.TaxAmount
		if tax == 0 {
			tax = 100
		}
		p.Money -= tax
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case TileProperty, TileStation, TileUtility:
		if tile.OwnerID != "" && tile.OwnerID != p.ID {
			rent := g.calculateRent(tile, total)
			p.Money -= rent
			owner := g.getPlayer(tile.OwnerID)
			if owner != nil {
				owner.Money += rent
			}
			res.RentPaid = rent
			res.RentTo = tile.OwnerID

			if p.Money < 0 {
				g.declareBankruptcy(p)
			}
		}
	case TileChance:
		if g.Deck != nil {
			card := g.Deck.DrawChance()
			if cardResult, err := g.ResolveCard(p.ID, card); err == nil {
				res.CardDrawn = cardResult
				g.LastCardResult = cardResult
			}
		}
	case TileChest:
		if g.Deck != nil {
			card := g.Deck.DrawChest()
			if cardResult, err := g.ResolveCard(p.ID, card); err == nil {
				res.CardDrawn = cardResult
				g.LastCardResult = cardResult
			}
		}
	case TileGoToJail:
		p.Position = 10
		p.InJail = true
		p.IsJailed = true
	}

	return res, nil
}

// BuyProperty purchases the tile at current player position.
func (g *Game) BuyProperty(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.IsStarted || g.CurrentTurnPlayerID != playerID {
		return errors.New("não é a vez deste jogador")
	}
	if !g.DiceRolledThisTurn {
		return errors.New("é necessário rolar os dados antes de comprar")
	}

	p := g.getPlayer(playerID)
	if p == nil {
		return errors.New("jogador não encontrado")
	}

	tile := &g.Board.Tiles[p.Position]
	if tile.Type != TileProperty && tile.Type != TileStation && tile.Type != TileUtility {
		return errors.New("esta casa não pode ser comprada")
	}
	if tile.OwnerID != "" {
		return errors.New("esta propriedade já possui dono")
	}
	if p.Money < tile.Price {
		return errors.New("saldo insuficiente")
	}

	p.Money -= tile.Price
	tile.OwnerID = p.ID
	p.Properties = append(p.Properties, tile.Index)
	return nil
}

// BuildHouse constructs a house/hotel on an eligible property.
func (g *Game) BuildHouse(playerID string, tileIndex int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.IsStarted || g.CurrentTurnPlayerID != playerID {
		return errors.New("não é a vez deste jogador")
	}
	if tileIndex < 0 || tileIndex >= len(g.Board.Tiles) {
		return errors.New("casa inválida")
	}

	tile := &g.Board.Tiles[tileIndex]
	if tile.Type != TileProperty || tile.OwnerID != playerID {
		return errors.New("você não é o proprietário desta casa")
	}
	if !g.hasFullGroup(playerID, tile.Group) {
		return errors.New("necessário possuir o grupo de cor completo para construir")
	}
	if tile.Houses >= 5 {
		return errors.New("limite máximo de construções atingido (hotel construído)")
	}

	p := g.getPlayer(playerID)
	if p.Money < tile.HouseCost {
		return errors.New("saldo insuficiente para construir")
	}

	p.Money -= tile.HouseCost
	tile.Houses++
	return nil
}

// resolveTileLanding resolves tax or rent after a player moves.
func (g *Game) resolveTileLanding(p *Player) {
	if p.Position < 0 || p.Position >= len(g.Board.Tiles) {
		return
	}
	tile := &g.Board.Tiles[p.Position]
	switch tile.Type {
	case TileTax:
		tax := tile.TaxAmount
		if tax == 0 {
			tax = 100
		}
		p.Money -= tax
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case TileProperty, TileStation, TileUtility:
		if tile.OwnerID != "" && tile.OwnerID != p.ID {
			rent := g.calculateRent(tile, 7)
			p.Money -= rent
			owner := g.getPlayer(tile.OwnerID)
			if owner != nil {
				owner.Money += rent
			}
			if p.Money < 0 {
				g.declareBankruptcy(p)
			}
		}
	case TileGoToJail:
		p.Position = 10
		p.InJail = true
		p.IsJailed = true
	}
}

// TradeExchangeRequest specifies properties and cash to swap between two players.
type TradeExchangeRequest struct {
	SenderID       string `json:"sender_id"`
	ReceiverID     string `json:"receiver_id"`
	OfferedTiles   []int  `json:"offered_tiles"`
	OfferedCash    int    `json:"offered_cash"`
	RequestedTiles []int  `json:"requested_tiles"`
	RequestedCash  int    `json:"requested_cash"`
}

// TradeExchange executes a multi-property and cash swap atomically.
func (g *Game) TradeExchange(req TradeExchangeRequest) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	sender := g.getPlayer(req.SenderID)
	receiver := g.getPlayer(req.ReceiverID)
	if sender == nil || receiver == nil {
		return errors.New("jogadores inválidos para negociação")
	}
	if sender.IsBankrupt || receiver.IsBankrupt {
		return errors.New("jogador falido não pode negociar")
	}
	if sender.Money < req.OfferedCash {
		return errors.New("remetente com saldo insuficiente para oferta em dinheiro")
	}
	if receiver.Money < req.RequestedCash {
		return errors.New("destinatário com saldo insuficiente para contrapartida em dinheiro")
	}

	for _, idx := range req.OfferedTiles {
		if idx < 0 || idx >= len(g.Board.Tiles) {
			return errors.New("propriedade ofertada inválida")
		}
		t := &g.Board.Tiles[idx]
		if t.OwnerID != req.SenderID {
			return errors.New("remetente não é dono da propriedade ofertada")
		}
		if t.Houses > 0 {
			return errors.New("não é permitido negociar propriedades com casas ou hotel construídos")
		}
	}

	for _, idx := range req.RequestedTiles {
		if idx < 0 || idx >= len(g.Board.Tiles) {
			return errors.New("propriedade solicitada inválida")
		}
		t := &g.Board.Tiles[idx]
		if t.OwnerID != req.ReceiverID {
			return errors.New("destinatário não é dono da propriedade solicitada")
		}
		if t.Houses > 0 {
			return errors.New("não é permitido negociar propriedades com casas ou hotel construídos")
		}
	}

	// Execução atômica
	sender.Money = sender.Money - req.OfferedCash + req.RequestedCash
	receiver.Money = receiver.Money + req.OfferedCash - req.RequestedCash

	for _, idx := range req.OfferedTiles {
		g.Board.Tiles[idx].OwnerID = req.ReceiverID
		sender.Properties = removeInt(sender.Properties, idx)
		receiver.Properties = append(receiver.Properties, idx)
	}

	for _, idx := range req.RequestedTiles {
		g.Board.Tiles[idx].OwnerID = req.SenderID
		receiver.Properties = removeInt(receiver.Properties, idx)
		sender.Properties = append(sender.Properties, idx)
	}

	return nil
}

func removeInt(slice []int, val int) []int {
	out := make([]int, 0, len(slice))
	for _, x := range slice {
		if x != val {
			out = append(out, x)
		}
	}
	return out
}

// TradeProperties transfers a property between two players for an agreed cash amount (wrapper around TradeExchange).
func (g *Game) TradeProperties(sellerID, buyerID string, tileIndex, cashAmount int) error {
	return g.TradeExchange(TradeExchangeRequest{
		SenderID:       sellerID,
		ReceiverID:     buyerID,
		OfferedTiles:   []int{tileIndex},
		OfferedCash:    0,
		RequestedTiles: nil,
		RequestedCash:  cashAmount,
	})
}

// PassTurn advances the turn to next active player.
func (g *Game) PassTurn(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.IsStarted || g.CurrentTurnPlayerID != playerID {
		return errors.New("não é a vez deste jogador")
	}
	if !g.DiceRolledThisTurn {
		return errors.New("é necessário rolar os dados antes de passar")
	}

	g.advanceTurn()
	return nil
}

// AutoPassTurn automatically advances turn on 30s timeout.
func (g *Game) AutoPassTurn() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.IsStarted || g.IsOver {
		return nil
	}

	g.advanceTurn()
	return nil
}

func (g *Game) advanceTurn() {
	activeCount := 0
	var lastActive *Player
	for _, p := range g.Players {
		if !p.IsBankrupt {
			activeCount++
			lastActive = p
		}
	}

	if activeCount <= 1 && lastActive != nil {
		g.IsOver = true
		g.WinnerID = lastActive.ID
		return
	}

	// Próximo jogador ativo
	for i := 0; i < len(g.Players); i++ {
		g.TurnIndex = (g.TurnIndex + 1) % len(g.Players)
		candidate := g.Players[g.TurnIndex]
		if !candidate.IsBankrupt {
			g.CurrentTurnPlayerID = candidate.ID
			g.DiceRolledThisTurn = false
			g.TurnStartTime = time.Now()
			return
		}
	}
}

func (g *Game) calculateRent(tile *Tile, diceTotal int) int {
	if tile.Type == TileProperty {
		if tile.Houses > 0 && tile.Houses <= 5 {
			return tile.Rent[tile.Houses]
		}
		// Sem casas: verificar se o dono tem o grupo completo (dobro do aluguel)
		if g.hasFullGroup(tile.OwnerID, tile.Group) {
			return tile.Rent[0] * 2
		}
		return tile.Rent[0]
	}

	if tile.Type == TileStation {
		count := g.countOwnedType(tile.OwnerID, TileStation)
		switch count {
		case 1:
			return 25
		case 2:
			return 50
		case 3:
			return 100
		default:
			return 200
		}
	}

	if tile.Type == TileUtility {
		count := g.countOwnedType(tile.OwnerID, TileUtility)
		if count >= 2 {
			return 10 * diceTotal
		}
		return 4 * diceTotal
	}

	return 0
}

func (g *Game) hasFullGroup(ownerID string, group ColorGroup) bool {
	if group == ColorNone {
		return false
	}
	for i := range g.Board.Tiles {
		t := &g.Board.Tiles[i]
		if t.Type == TileProperty && t.Group == group {
			if t.OwnerID != ownerID {
				return false
			}
		}
	}
	return true
}

func (g *Game) countOwnedType(ownerID string, tType TileType) int {
	c := 0
	for i := range g.Board.Tiles {
		t := &g.Board.Tiles[i]
		if t.Type == tType && t.OwnerID == ownerID {
			c++
		}
	}
	return c
}

func (g *Game) declareBankruptcy(p *Player) {
	p.IsBankrupt = true
	// Retornar propriedades ao banco
	for i := range g.Board.Tiles {
		t := &g.Board.Tiles[i]
		if t.OwnerID == p.ID {
			t.OwnerID = ""
			t.Houses = 0
		}
	}
}

func (g *Game) getPlayer(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}
