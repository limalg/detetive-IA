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
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Color      string   `json:"color"`
	Piece      string   `json:"piece"`
	Money      int      `json:"money"`
	Position   int      `json:"position"`
	InJail     bool     `json:"in_jail"`
	JailTurns  int      `json:"jail_turns"`
	IsBankrupt bool     `json:"is_bankrupt"`
	Properties []int    `json:"properties"` // Índices das casas possuídas
}

// DiceResult describes the outcome of rolling two dice.
type DiceResult struct {
	Die1        int  `json:"die1"`
	Die2        int  `json:"die2"`
	Total       int  `json:"total"`
	IsDouble    bool `json:"is_double"`
	OldPosition int  `json:"old_position"`
	NewPosition int  `json:"new_position"`
	RentPaid    int  `json:"rent_paid,omitempty"`
	RentTo      string `json:"rent_to,omitempty"`
}

// Game represents the state and rules engine for a 40-tile match.
type Game struct {
	mu                  sync.Mutex
	RoomID              string    `json:"room_id"`
	Board               *Board    `json:"board"`
	Players             []*Player `json:"players"`
	CurrentTurnPlayerID string    `json:"current_turn_player_id"`
	TurnIndex           int       `json:"turn_index"`
	DiceRolledThisTurn  bool      `json:"dice_rolled_this_turn"`
	TurnStartTime       time.Time `json:"turn_start_time"`
	IsStarted           bool      `json:"is_started"`
	IsOver              bool      `json:"is_over"`
	WinnerID            string    `json:"winner_id,omitempty"`
}

// NewGame initializes a game instance for the given room and board.
func NewGame(roomID string, board *Board) *Game {
	return &Game{
		RoomID:    roomID,
		Board:     board,
		Players:   make([]*Player, 0),
		TurnIndex: 0,
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

	// Resolução automática de taxa/aluguel
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
	case TileGoToJail:
		p.Position = 10
		p.InJail = true
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

// TradeProperties transfers a property between two players for an agreed cash amount.
func (g *Game) TradeProperties(sellerID, buyerID string, tileIndex, cashAmount int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	seller := g.getPlayer(sellerID)
	buyer := g.getPlayer(buyerID)
	if seller == nil || buyer == nil {
		return errors.New("jogadores inválidos")
	}
	if buyer.Money < cashAmount {
		return errors.New("comprador com saldo insuficiente")
	}
	if tileIndex < 0 || tileIndex >= len(g.Board.Tiles) {
		return errors.New("propriedade inválida")
	}
	tile := &g.Board.Tiles[tileIndex]
	if tile.OwnerID != sellerID {
		return errors.New("vendedor não é dono da propriedade")
	}

	buyer.Money -= cashAmount
	seller.Money += cashAmount
	tile.OwnerID = buyerID

	newSellerProps := make([]int, 0, len(seller.Properties))
	for _, idx := range seller.Properties {
		if idx != tileIndex {
			newSellerProps = append(newSellerProps, idx)
		}
	}
	seller.Properties = newSellerProps
	buyer.Properties = append(buyer.Properties, tileIndex)
	return nil
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
