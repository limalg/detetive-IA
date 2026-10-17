package banco

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// Game manages a running session of Banco Imobiliário IA.
type Game struct {
	mu    sync.Mutex
	state GameState
	cards []Card
}

// NewGame initializes a new game with players, board, and economic settings.
func NewGame(opts GameOptions) *Game {
	city := strings.TrimSpace(opts.City)
	if city == "" {
		city = "São Paulo"
	}
	grade := strings.TrimSpace(opts.SchoolGrade)
	if grade == "" {
		grade = "Geral"
	}
	difficulty := strings.ToLower(strings.TrimSpace(opts.Difficulty))
	if difficulty == "" {
		difficulty = "medio"
	}

	startBalance := opts.StartingBalance
	if startBalance <= 0 {
		startBalance = 1500
	}

	humanName := strings.TrimSpace(opts.HumanName)
	if humanName == "" {
		humanName = "Você (Investidor)"
	}

	citySlug := ""
	for _, r := range strings.ToLower(city) {
		if r >= 'a' && r <= 'z' {
			citySlug += string(r)
			if len(citySlug) >= 3 {
				break
			}
		}
	}
	if citySlug == "" {
		citySlug = "gam"
	}
	gameID := fmt.Sprintf("banco_%s_%d", citySlug, cryptoRandInt(100000, 999999))

	var players []*Player
	// Human Player
	players = append(players, &Player{
		ID:         1,
		Name:       humanName,
		IsBot:      false,
		Color:      "#d4a359", // Dourado Art Déco
		Avatar:     "🎩",
		Balance:    startBalance,
		Position:   0,
		Properties: make([]int, 0),
	})

	// Optional Friends
	botPool := []struct {
		name   string
		avatar string
		color  string
	}{
		{"Barão de Mauá (IA)", "🏦", "#1890ff"},
		{"Condessa de Vila Rica (IA)", "💎", "#52c41a"},
		{"Magnata Matarazzo (IA)", "🏭", "#eb2f96"},
		{"Investidor Solar (IA)", "⚡", "#fa8c16"},
	}

	playerID := 2
	for _, friend := range opts.Friends {
		friend = strings.TrimSpace(friend)
		if friend == "" || len(players) >= 4 {
			continue
		}
		players = append(players, &Player{
			ID:         playerID,
			Name:       friend,
			IsBot:      false,
			Color:      "#722ed1",
			Avatar:     "🤝",
			Balance:    startBalance,
			Position:   0,
			Properties: make([]int, 0),
		})
		playerID++
	}

	// Bots
	botsCount := opts.BotsCount
	if botsCount <= 0 && len(players) < 2 {
		botsCount = 2
	}
	if botsCount > 3 {
		botsCount = 3
	}

	for i := 0; i < botsCount && len(players) < 4; i++ {
		botMeta := botPool[i%len(botPool)]
		players = append(players, &Player{
			ID:         playerID,
			Name:       botMeta.name,
			IsBot:      true,
			Color:      botMeta.color,
			Avatar:     botMeta.avatar,
			Balance:    startBalance,
			Position:   0,
			Properties: make([]int, 0),
		})
		playerID++
	}

	board := GenerateBoard(city, grade, opts.Age)
	cards := GenerateLuckCards(city)

	state := GameState{
		ID:           gameID,
		City:         city,
		Age:          opts.Age,
		SchoolGrade:  grade,
		Difficulty:   difficulty,
		CurrentTurn:  1,
		Round:        1,
		Players:      players,
		Board:        board,
		History:      []string{fmt.Sprintf("Partida de Banco Imobiliário iniciada em %s com %d jogadores!", city, len(players))},
		Status:       "in_progress",
		CanRoll:      true,
		CanBuy:       false,
		CanEndTurn:   false,
	}

	return &Game{
		state: state,
		cards: cards,
	}
}

// GetState returns a clone of the current GameState.
func (g *Game) GetState() GameState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state
}

// GetPlayer retrieves a player by ID.
func (g *Game) GetPlayer(playerID int) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range g.state.Players {
		if p.ID == playerID {
			return p
		}
	}
	return nil
}

// ForcePlayerPosition is a testing and internal helper to place a player at a specific tile.
func (g *Game) ForcePlayerPosition(playerID int, position int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range g.state.Players {
		if p.ID == playerID {
			p.Position = position % len(g.state.Board)
			return
		}
	}
}

// SetCurrentTurn explicitly sets the current turn (useful in tests or recovery).
func (g *Game) SetCurrentTurn(playerID int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state.CurrentTurn = playerID
	g.state.CanRoll = true
	g.state.CanBuy = false
	g.state.CanEndTurn = false
}

// RollDice rolls 2d6, handles movement, doubles and landing logic.
func (g *Game) RollDice(playerID int) (*DiceRollResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state.Status == "finished" {
		return nil, errors.New("a partida já foi finalizada")
	}

	if g.state.CurrentTurn != playerID {
		return nil, fmt.Errorf("não é a vez do jogador %d (vez do jogador %d)", playerID, g.state.CurrentTurn)
	}

	if !g.state.CanRoll {
		return nil, errors.New("os dados já foram rolados nesta rodada")
	}

	p := g.findPlayer(playerID)
	if p == nil || p.Bankrupt {
		return nil, errors.New("jogador inválido ou falido")
	}

	d1 := cryptoRandInt(1, 6)
	d2 := cryptoRandInt(1, 6)
	total := d1 + d2
	isDouble := (d1 == d2)

	g.state.LastDice = [2]int{d1, d2}
	result := &DiceRollResult{
		Dice:     [2]int{d1, d2},
		Total:    total,
		IsDouble: isDouble,
	}

	// Jail handling
	if p.InJail {
		if isDouble {
			p.InJail = false
			p.JailTurns = 0
			g.addHistory(fmt.Sprintf("🎲 %s tirou duplos (%d e %d) e saiu da Prisão!", p.Name, d1, d2))
		} else {
			p.JailTurns++
			if p.JailTurns >= 3 {
				p.InJail = false
				p.JailTurns = 0
				p.Balance -= 50
				g.addHistory(fmt.Sprintf("⚖️ %s completou 3 rodadas na Prisão, pagou fiança de R$ 50 e foi liberado.", p.Name))
			} else {
				g.state.CanRoll = false
				g.state.CanEndTurn = true
				result.EventLog = fmt.Sprintf("%s não tirou duplos e permanece na prisão (%d/3 rodadas).", p.Name, p.JailTurns)
				g.addHistory("🔒 " + result.EventLog)
				return result, nil
			}
		}
	}

	if isDouble {
		g.state.DoublesCount++
		if g.state.DoublesCount >= 3 {
			p.InJail = true
			p.Position = 7 // Jail position
			p.JailTurns = 0
			g.state.DoublesCount = 0
			g.state.CanRoll = false
			g.state.CanEndTurn = true
			result.NewPosition = 7
			result.EventLog = fmt.Sprintf("🚨 %s tirou 3 duplos seguidos e foi direto para a Prisão por excesso de velocidade!", p.Name)
			g.addHistory(result.EventLog)
			return result, nil
		}
		// Double grants roll again unless jailed
		g.state.CanRoll = true
	} else {
		g.state.DoublesCount = 0
		g.state.CanRoll = false
	}
	g.state.CanEndTurn = true

	// Move player along board
	oldPos := p.Position
	newPos := (oldPos + total) % len(g.state.Board)
	p.Position = newPos
	result.NewPosition = newPos

	// Passing Start: collect salary R$ 200
	if newPos < oldPos && oldPos != 0 {
		p.Balance += 200
		result.PassedStart = true
		g.addHistory(fmt.Sprintf("🚩 %s passou pelo Ponto de Partida e recebeu R$ 200 de salário!", p.Name))
	}

	// Process Landing
	g.processLandingInternal(p, result)
	return result, nil
}

// ProcessLanding executes the effects of landing on the current tile for a player.
func (g *Game) ProcessLanding(playerID int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := g.findPlayer(playerID)
	if p != nil {
		g.processLandingInternal(p, nil)
	}
}

func (g *Game) processLandingInternal(p *Player, res *DiceRollResult) {
	tile := &g.state.Board[p.Position]

	switch tile.Type {
	case TileStart:
		p.Balance += 200
		log := fmt.Sprintf("🚩 %s parou no Ponto de Partida e recebeu R$ 200 de bonificação!", p.Name)
		if res != nil {
			res.EventLog = log
		}
		g.addHistory(log)

	case TileProperty, TileStation, TileUtility:
		if tile.OwnerID == 0 {
			// Unowned property
			if p.Balance >= tile.Price {
				g.state.CanBuy = true
				if res != nil {
					res.CanBuy = true
				}
			}
			log := fmt.Sprintf("🏢 %s parou em %s (%s, Preço: R$ %d). Disponível para compra!", p.Name, tile.Name, tile.Group, tile.Price)
			if res != nil {
				res.EventLog = log
			}
			g.addHistory(log)
		} else if tile.OwnerID != p.ID {
			// Owned by another player: calculate and pay rent
			owner := g.findPlayer(tile.OwnerID)
			if owner != nil && !owner.Bankrupt {
				rent := g.calculateRent(tile)
				if p.Balance < rent {
					rent = p.Balance
					p.Bankrupt = true
					g.handleBankruptcy(p, owner)
				} else {
					p.Balance -= rent
					owner.Balance += rent
				}
				if res != nil {
					res.RentPaid = rent
					res.RentTo = owner.ID
				}
				log := fmt.Sprintf("💸 %s pagou R$ %d de aluguel para %s em %s.", p.Name, rent, owner.Name, tile.Name)
				if res != nil {
					res.EventLog = log
				}
				g.addHistory(log)
			}
		} else {
			log := fmt.Sprintf("🏡 %s visitou sua própria propriedade: %s.", p.Name, tile.Name)
			if res != nil {
				res.EventLog = log
			}
			g.addHistory(log)
		}

	case TileGoToJail:
		p.InJail = true
		p.Position = 7 // Pos 7 is Jail
		p.JailTurns = 0
		g.state.CanRoll = false
		g.state.CanEndTurn = true
		log := fmt.Sprintf("🚔 %s caiu em 'Vá para a Prisão!' e foi detido pelas autoridades!", p.Name)
		if res != nil {
			res.NewPosition = 7
			res.EventLog = log
		}
		g.addHistory(log)

	case TileTax:
		taxAmount := tile.Price
		if taxAmount <= 0 {
			taxAmount = 100
		}
		p.Balance -= taxAmount
		if p.Balance < 0 {
			p.Bankrupt = true
			g.handleBankruptcy(p, nil)
		}
		log := fmt.Sprintf("🏛️ %s pagou R$ %d de %s ao tesouro municipal.", p.Name, taxAmount, tile.Name)
		if res != nil {
			res.EventLog = log
		}
		g.addHistory(log)

	case TileLuck:
		// Draw card
		if len(g.cards) > 0 {
			cardIdx := cryptoRandInt(0, len(g.cards)-1)
			card := g.cards[cardIdx]
			p.Balance += card.Amount
			if p.Balance < 0 {
				p.Bankrupt = true
				g.handleBankruptcy(p, nil)
			}
			log := fmt.Sprintf("🃏 Sorte ou Revés para %s: '%s' - %s", p.Name, card.Title, card.Description)
			if res != nil {
				res.EventLog = log
			}
			g.addHistory(log)
		}

	case TileJail:
		log := fmt.Sprintf("👀 %s está apenas visitando a Prisão.", p.Name)
		if res != nil {
			res.EventLog = log
		}
		g.addHistory(log)

	case TileParking:
		log := fmt.Sprintf("☕ %s parou na Praça Central para descansar.", p.Name)
		if res != nil {
			res.EventLog = log
		}
		g.addHistory(log)
	}

	g.checkVictoryCondition()
}

// BuyProperty purchases the property at the given tile for the player.
func (g *Game) BuyProperty(playerID int, tilePos int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if tilePos < 0 || tilePos >= len(g.state.Board) {
		return errors.New("posição inválida no tabuleiro")
	}

	tile := &g.state.Board[tilePos]
	if tile.Type != TileProperty && tile.Type != TileStation && tile.Type != TileUtility {
		return errors.New("esta casa não é uma propriedade negociável")
	}

	if tile.OwnerID != 0 {
		return errors.New("esta propriedade já tem dono")
	}

	p := g.findPlayer(playerID)
	if p == nil || p.Bankrupt {
		return errors.New("jogador inválido ou falido")
	}

	if p.Balance < tile.Price {
		return fmt.Errorf("saldo insuficiente (saldo: R$ %d, preço: R$ %d)", p.Balance, tile.Price)
	}

	p.Balance -= tile.Price
	tile.OwnerID = p.ID
	p.Properties = append(p.Properties, tile.Position)
	g.state.CanBuy = false

	g.addHistory(fmt.Sprintf("📜 %s comprou %s por R$ %d!", p.Name, tile.Name, tile.Price))
	return nil
}

// BuildHouse constructs a house or hotel on a property if the player has the complete color group.
func (g *Game) BuildHouse(playerID int, tilePos int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if tilePos < 0 || tilePos >= len(g.state.Board) {
		return errors.New("posição inválida")
	}

	tile := &g.state.Board[tilePos]
	if tile.Type != TileProperty {
		return errors.New("só é possível construir em terrenos de bairros")
	}

	if tile.OwnerID != playerID {
		return errors.New("você não é o proprietário deste terreno")
	}

	p := g.findPlayer(playerID)
	if p == nil || p.Bankrupt {
		return errors.New("jogador inválido")
	}

	// Verify monopoly on color group
	if !g.hasMonopoly(playerID, tile.Group) {
		return errors.New("é necessário possuir todas as propriedades do mesmo grupo para construir")
	}

	if tile.Hotel {
		return errors.New("esta propriedade já atingiu a melhoria máxima (Hotel)")
	}

	if p.Balance < tile.BuildCost {
		return fmt.Errorf("saldo insuficiente para construir (custo: R$ %d, saldo: R$ %d)", tile.BuildCost, p.Balance)
	}

	p.Balance -= tile.BuildCost
	if tile.Houses < 4 {
		tile.Houses++
		g.addHistory(fmt.Sprintf("🏠 %s construiu a %dª casa em %s!", p.Name, tile.Houses, tile.Name))
	} else {
		tile.Houses = 0
		tile.Hotel = true
		g.addHistory(fmt.Sprintf("🏨 %s construiu um luxuoso HOTEL em %s!", p.Name, tile.Name))
	}

	return nil
}

// EndTurn completes the current player's turn and advances to the next active player.
func (g *Game) EndTurn(playerID int) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state.CurrentTurn != playerID {
		return 0, fmt.Errorf("não é o seu turno (turno atual: %d)", g.state.CurrentTurn)
	}

	nextID := g.advanceTurnInternal()
	return nextID, nil
}

func (g *Game) advanceTurnInternal() int {
	g.state.CanRoll = true
	g.state.CanBuy = false
	g.state.CanEndTurn = false
	g.state.DoublesCount = 0

	activeCount := 0
	for _, p := range g.state.Players {
		if !p.Bankrupt {
			activeCount++
		}
	}
	if activeCount <= 1 {
		g.checkVictoryCondition()
		return g.state.CurrentTurn
	}

	currentIdx := -1
	for i, p := range g.state.Players {
		if p.ID == g.state.CurrentTurn {
			currentIdx = i
			break
		}
	}

	for step := 1; step <= len(g.state.Players); step++ {
		nextIdx := (currentIdx + step) % len(g.state.Players)
		if nextIdx == 0 {
			g.state.Round++
		}
		candidate := g.state.Players[nextIdx]
		if !candidate.Bankrupt {
			g.state.CurrentTurn = candidate.ID
			return candidate.ID
		}
	}

	return g.state.CurrentTurn
}

// PlayBotTurns automatically processes turns for all consecutive autonomous bot players.
func (g *Game) PlayBotTurns() ([]string, int, error) {
	var logs []string

	for {
		g.mu.Lock()
		if g.state.Status == "finished" {
			g.mu.Unlock()
			break
		}

		curr := g.findPlayer(g.state.CurrentTurn)
		if curr == nil || !curr.IsBot {
			turnID := g.state.CurrentTurn
			g.mu.Unlock()
			return logs, turnID, nil
		}
		g.mu.Unlock()

		// Execute Bot Turn
		botLog := g.executeSingleBotTurn(curr.ID)
		logs = append(logs, botLog...)

		// Advance Turn
		g.mu.Lock()
		g.advanceTurnInternal()
		g.mu.Unlock()
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	return logs, g.state.CurrentTurn, nil
}

func (g *Game) executeSingleBotTurn(botID int) []string {
	var botLogs []string

	g.mu.Lock()
	bot := g.findPlayer(botID)
	if bot == nil || bot.Bankrupt {
		g.mu.Unlock()
		return botLogs
	}

	// 1. Jail bailout decision
	if bot.InJail && bot.Balance > 200 {
		bot.Balance -= 50
		bot.InJail = false
		bot.JailTurns = 0
		log := fmt.Sprintf("💼 %s pagou R$ 50 de fiança e saiu da Prisão para investir.", bot.Name)
		botLogs = append(botLogs, log)
		g.addHistory(log)
	}
	g.mu.Unlock()

	// 2. Roll Dice
	rollRes, err := g.RollDice(botID)
	if err == nil && rollRes != nil {
		botLogs = append(botLogs, fmt.Sprintf("🎲 %s rolou [%d + %d = %d]. %s", bot.Name, rollRes.Dice[0], rollRes.Dice[1], rollRes.Total, rollRes.EventLog))
	}

	// 3. Purchase evaluation
	g.mu.Lock()
	bot = g.findPlayer(botID)
	if bot != nil && !bot.Bankrupt && g.state.CanBuy {
		tile := &g.state.Board[bot.Position]
		shouldBuy := false
		switch g.state.Difficulty {
		case "facil":
			shouldBuy = bot.Balance > tile.Price*2
		case "dificil":
			shouldBuy = bot.Balance >= tile.Price
		default: // medio
			shouldBuy = bot.Balance >= tile.Price+100
		}

		if shouldBuy {
			bot.Balance -= tile.Price
			tile.OwnerID = bot.ID
			bot.Properties = append(bot.Properties, tile.Position)
			g.state.CanBuy = false
			log := fmt.Sprintf("🤝 %s comprou %s por R$ %d!", bot.Name, tile.Name, tile.Price)
			botLogs = append(botLogs, log)
			g.addHistory(log)
		}
	}

	// 4. Building evaluation
	if bot != nil && !bot.Bankrupt {
		for _, pos := range bot.Properties {
			tile := &g.state.Board[pos]
			if tile.Type == TileProperty && !tile.Hotel && g.hasMonopoly(bot.ID, tile.Group) {
				if bot.Balance > tile.BuildCost*2 {
					bot.Balance -= tile.BuildCost
					if tile.Houses < 4 {
						tile.Houses++
						log := fmt.Sprintf("🏗️ %s expandiu %s adicionando a %dª casa!", bot.Name, tile.Name, tile.Houses)
						botLogs = append(botLogs, log)
						g.addHistory(log)
					} else {
						tile.Houses = 0
						tile.Hotel = true
						log := fmt.Sprintf("🌟 %s inaugurou um HOTEL em %s!", bot.Name, tile.Name)
						botLogs = append(botLogs, log)
						g.addHistory(log)
					}
				}
			}
		}
	}
	g.mu.Unlock()

	return botLogs
}

func (g *Game) calculateRent(t *Tile) int {
	if t.Type == TileStation {
		owner := g.findPlayer(t.OwnerID)
		if owner == nil {
			return t.RentBase
		}
		stationCount := 0
		for _, p := range owner.Properties {
			if g.state.Board[p].Type == TileStation {
				stationCount++
			}
		}
		switch stationCount {
		case 1:
			return 25
		case 2:
			return 50
		case 3:
			return 100
		case 4:
			return 200
		default:
			return 25
		}
	}

	if t.Type == TileUtility {
		return 30
	}

	// Property with houses / hotel
	if t.Hotel {
		return t.RentHouse[4]
	}
	if t.Houses > 0 && t.Houses <= 4 {
		return t.RentHouse[t.Houses-1]
	}

	// Base rent: doubles if owner possesses full color monopoly
	rent := t.RentBase
	if g.hasMonopoly(t.OwnerID, t.Group) {
		rent *= 2
	}
	return rent
}

func (g *Game) hasMonopoly(playerID int, group string) bool {
	if group == "special" || group == "station" || group == "utility" {
		return false
	}
	totalInGroup := 0
	ownedInGroup := 0
	for _, t := range g.state.Board {
		if t.Group == group {
			totalInGroup++
			if t.OwnerID == playerID {
				ownedInGroup++
			}
		}
	}
	return totalInGroup > 0 && totalInGroup == ownedInGroup
}

func (g *Game) handleBankruptcy(bankruptPlayer *Player, creditor *Player) {
	for _, pos := range bankruptPlayer.Properties {
		t := &g.state.Board[pos]
		if creditor != nil {
			t.OwnerID = creditor.ID
			creditor.Properties = append(creditor.Properties, pos)
		} else {
			t.OwnerID = 0
			t.Houses = 0
			t.Hotel = false
		}
	}
	bankruptPlayer.Properties = make([]int, 0)
	g.addHistory(fmt.Sprintf("💀 %s declarou FALÊNCIA e foi eliminado do jogo!", bankruptPlayer.Name))
}

func (g *Game) checkVictoryCondition() {
	activeCount := 0
	var lastAlive *Player
	for _, p := range g.state.Players {
		if !p.Bankrupt {
			activeCount++
			lastAlive = p
		}
	}

	if activeCount == 1 && lastAlive != nil {
		g.state.Status = "finished"
		g.state.Winner = lastAlive.Name
		g.addHistory(fmt.Sprintf("🏆 %s É O GRANDE VENCEDOR DO BANCO IMOBILIÁRIO!", lastAlive.Name))
	}
}

func (g *Game) findPlayer(id int) *Player {
	for _, p := range g.state.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (g *Game) addHistory(entry string) {
	g.state.History = append(g.state.History, entry)
	if len(g.state.History) > 100 {
		g.state.History = g.state.History[len(g.state.History)-100:]
	}
}

func cryptoRandInt(minVal, maxVal int) int {
	if minVal >= maxVal {
		return minVal
	}
	diff := int64(maxVal - minVal + 1)
	nBig, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return minVal
	}
	return minVal + int(nBig.Int64())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TradeProperties allows swapping properties and cash between two players.
// If the recipient is a bot, it evaluates market value and group completion.
func (g *Game) TradeProperties(proposal TradeProposal) (*TradeResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state.Status == "finished" {
		return nil, errors.New("a partida já foi finalizada")
	}

	if proposal.FromPlayerID == proposal.ToPlayerID {
		return nil, errors.New("jogadores de origem e destino devem ser diferentes")
	}

	from := g.findPlayer(proposal.FromPlayerID)
	if from == nil || from.Bankrupt {
		return nil, errors.New("jogador proponente inválido ou falido")
	}

	to := g.findPlayer(proposal.ToPlayerID)
	if to == nil || to.Bankrupt {
		return nil, errors.New("jogador destinatário inválido ou falido")
	}

	// Validate offered properties
	for _, pos := range proposal.OfferedProps {
		if pos < 0 || pos >= len(g.state.Board) {
			return nil, fmt.Errorf("posição %d inválida", pos)
		}
		tile := &g.state.Board[pos]
		if tile.OwnerID != from.ID {
			return nil, fmt.Errorf("%s não é proprietário de %s", from.Name, tile.Name)
		}
		if tile.Houses > 0 || tile.Hotel {
			return nil, fmt.Errorf("não é permitido negociar propriedades com casas construídas (%s)", tile.Name)
		}
	}

	// Validate requested properties
	for _, pos := range proposal.RequestedProps {
		if pos < 0 || pos >= len(g.state.Board) {
			return nil, fmt.Errorf("posição %d inválida", pos)
		}
		tile := &g.state.Board[pos]
		if tile.OwnerID != to.ID {
			return nil, fmt.Errorf("%s não é proprietário de %s", to.Name, tile.Name)
		}
		if tile.Houses > 0 || tile.Hotel {
			return nil, fmt.Errorf("não é permitido negociar propriedades com casas construídas (%s)", tile.Name)
		}
	}

	// Validate cash
	if proposal.CashOffer < 0 || proposal.CashRequest < 0 {
		return nil, errors.New("valores em dinheiro não podem ser negativos")
	}
	if from.Balance < proposal.CashOffer {
		return &TradeResult{
			Accepted: false,
			Message:  fmt.Sprintf("%s não possui saldo suficiente para a oferta de R$ %d.", from.Name, proposal.CashOffer),
			Reason:   "insufficient_funds",
		}, nil
	}
	if proposal.CashRequest > 0 && to.Balance < proposal.CashRequest {
		return &TradeResult{
			Accepted: false,
			Message:  fmt.Sprintf("%s não possui saldo suficiente para pagar os R$ %d solicitados.", to.Name, proposal.CashRequest),
			Reason:   "insufficient_funds",
		}, nil
	}

	// If destination is a Bot, evaluate the proposal
	if to.IsBot {
		valOffered := proposal.CashOffer
		for _, pos := range proposal.OfferedProps {
			valOffered += g.state.Board[pos].Price
		}

		valRequested := proposal.CashRequest
		for _, pos := range proposal.RequestedProps {
			valRequested += g.state.Board[pos].Price
		}

		// Check if any requested property breaks a monopoly that the bot already holds
		for _, pos := range proposal.RequestedProps {
			reqTile := &g.state.Board[pos]
			if g.hasMonopoly(to.ID, reqTile.Group) {
				return &TradeResult{
					Accepted: false,
					Message:  fmt.Sprintf("🏦 %s analisou o mercado e recusou: não desfaz o monopólio no grupo %s!", to.Name, reqTile.Group),
					Reason:   "breaks_monopoly",
				}, nil
			}
		}

		// Bonus if offered property completes a monopoly for the bot
		for _, pos := range proposal.OfferedProps {
			offeredTile := &g.state.Board[pos]
			if offeredTile.Group != "special" && offeredTile.Group != "station" && offeredTile.Group != "utility" {
				ownedCount := 0
				totalInGroup := 0
				for _, t := range g.state.Board {
					if t.Group == offeredTile.Group {
						totalInGroup++
						if t.OwnerID == to.ID {
							ownedCount++
						}
					}
				}
				if ownedCount+1 == totalInGroup {
					valOffered += 250
				}
			}
		}

		// Acceptance threshold based on difficulty
		requiredVal := valRequested
		switch g.state.Difficulty {
		case "facil":
			requiredVal = int(float64(valRequested) * 0.85)
		case "dificil":
			requiredVal = int(float64(valRequested) * 1.15)
		default: // medio
			requiredVal = valRequested
		}

		if valOffered < requiredVal {
			return &TradeResult{
				Accepted: false,
				Message:  fmt.Sprintf("🏦 %s recusou a proposta: valor oferecido (R$ %d) é inferior à avaliação de mercado esperada (R$ %d).", to.Name, valOffered, requiredVal),
				Reason:   "insufficient_value",
			}, nil
		}
	}

	// Execute trade
	for _, pos := range proposal.OfferedProps {
		tile := &g.state.Board[pos]
		tile.OwnerID = to.ID
		from.Properties = removeIntFromSlice(from.Properties, pos)
		to.Properties = append(to.Properties, pos)
	}

	for _, pos := range proposal.RequestedProps {
		tile := &g.state.Board[pos]
		tile.OwnerID = from.ID
		to.Properties = removeIntFromSlice(to.Properties, pos)
		from.Properties = append(from.Properties, pos)
	}

	from.Balance -= proposal.CashOffer
	to.Balance += proposal.CashOffer
	to.Balance -= proposal.CashRequest
	from.Balance += proposal.CashRequest

	logEntry := fmt.Sprintf("🤝 Negociação concluída com sucesso entre %s e %s!", from.Name, to.Name)
	g.addHistory(logEntry)

	return &TradeResult{
		Accepted: true,
		Message:  logEntry,
	}, nil
}

// RequestLoan grants an emergency liquidity loan from the bank when balance is low.
func (g *Game) RequestLoan(playerID int, requestedAmount int) (*LoanResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state.Status == "finished" {
		return nil, errors.New("a partida já foi finalizada")
	}

	p := g.findPlayer(playerID)
	if p == nil || p.Bankrupt {
		return nil, errors.New("jogador inválido ou falido")
	}

	if p.Balance > 300 {
		return &LoanResult{
			Approved: false,
			Amount:   0,
			Message:  fmt.Sprintf("Empréstimo negado: o saldo de %s (R$ %d) está acima do limite de emergência (R$ 300).", p.Name, p.Balance),
		}, nil
	}

	amount := requestedAmount
	if amount <= 0 {
		amount = 300
	}
	if amount > 500 {
		amount = 500
	}

	p.Balance += amount
	logEntry := fmt.Sprintf("🏦 [Banqueiro IA] Concedeu empréstimo emergencial de R$ %d para %s evitar a falência!", amount, p.Name)
	g.addHistory(logEntry)

	return &LoanResult{
		Approved: true,
		Amount:   amount,
		Message:  logEntry,
	}, nil
}

func removeIntFromSlice(slice []int, val int) []int {
	result := make([]int, 0, len(slice))
	for _, v := range slice {
		if v != val {
			result = append(result, v)
		}
	}
	return result
}

