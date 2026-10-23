package banco3d_test

import (
	"testing"

	"detetive-ia/internal/banco3d"
)

// TestBoardSingleSourceOfTruth verifies that room.Board always reflects OwnerID, Houses, and Mortgages
// matching room.Game.Board (uma única fonte de verdade).
func TestBoardSingleSourceOfTruth(t *testing.T) {
	room, err := banco3d.NewRoom("room-board-truth", "Curitiba")
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	_, _ = room.JoinPlayer("Bob", "#ef4444", "hat")
	if err := room.StartGame(); err != nil {
		t.Fatalf("erro ao iniciar jogo: %v", err)
	}

	// Força compra da propriedade no tile 1 (Brown 60)
	room.Game.CurrentTurnPlayerID = p1.ID
	room.Game.DiceRolledThisTurn = true
	p1.Position = 1

	if err := room.BuyProperty(p1.ID); err != nil {
		t.Fatalf("erro ao comprar propriedade: %v", err)
	}

	// 1. Validar que no game.Board o dono é p1
	if room.Game.Board.Tiles[1].OwnerID != p1.ID {
		t.Fatalf("game.Board tile 1 deveria ter owner_id=%s, obtido %s", p1.ID, room.Game.Board.Tiles[1].OwnerID)
	}

	// 2. Validar que no board de TOPO (room.Board) o dono é refletido imediatamente!
	if room.Board.Tiles[1].OwnerID != p1.ID {
		t.Errorf("board de topo (room.Board) deve refletir owner_id=%s, obtido vazio ou incorreto: %s", p1.ID, room.Board.Tiles[1].OwnerID)
	}
}

// TestBotDoubleRollPurchasingPlaysAgain verifies that a bot rolling doubles to buy a property
// does NOT pass the turn, and instead repeats its turn (joga novamente).
func TestBotDoubleRollPurchasingPlaysAgain(t *testing.T) {
	cfg := banco3d.DefaultRoomConfig()
	cfg.Bots.Count = 1
	room, err := banco3d.NewRoom("room-bot-double-buy", "Curitiba", &cfg)
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	_, _ = room.JoinPlayer("Human", "#3b82f6", "car")
	if err := room.StartGame(); err != nil {
		t.Fatalf("erro ao iniciar jogo: %v", err)
	}

	// Acha o jogador bot
	var bot *banco3d.Player
	for _, p := range room.Game.Players {
		if p.IsBot {
			bot = p
			break
		}
	}
	if bot == nil {
		t.Fatalf("bot não encontrado")
	}

	// Coloca a vez no bot na posição 0
	for i, p := range room.Game.Players {
		if p.ID == bot.ID {
			room.Game.TurnIndex = i
			room.Game.CurrentTurnPlayerID = bot.ID
			break
		}
	}
	room.Game.DiceRolledThisTurn = false
	bot.Position = 0
	bot.Money = 1500

	// Tile 6 é LightBlue (Preço 100). Rolando [3, 3] = 6 (dupla!)
	err = room.ExecuteBotTurnWithDice(bot, 3, 3)
	if err != nil {
		t.Fatalf("erro no turno do bot: %v", err)
	}

	// Validar que o bot comprou a propriedade da casa 6
	if room.Game.Board.Tiles[6].OwnerID != bot.ID {
		t.Errorf("bot deveria ter comprado a casa 6, dono atual: %s", room.Game.Board.Tiles[6].OwnerID)
	}

	// REGRA CRÍTICA: Ao comprar com dupla, NÃO passa o turno! Deve repetir a vez do bot.
	if room.Game.CurrentTurnPlayerID != bot.ID {
		t.Errorf("dupla de bot com compra NÃO pode passar a vez! Esperava CurrentTurnPlayerID=%s, obtido %s", bot.ID, room.Game.CurrentTurnPlayerID)
	}
	if room.Game.DiceRolledThisTurn {
		t.Errorf("DiceRolledThisTurn deve ser false para permitir novo arremesso")
	}

	// Segundo arremesso do bot (não-dupla: 1+2=3) -> anda da casa 6 para a 9 e agora sim passa a vez
	err = room.ExecuteBotTurnWithDice(bot, 1, 2)
	if err != nil {
		t.Fatalf("erro no 2º arremesso do bot: %v", err)
	}
	if room.Game.CurrentTurnPlayerID == bot.ID {
		t.Errorf("após arremesso sem dupla, o turno deveria ter passado para o próximo jogador")
	}
}

// TestBotDoubleRollPayingRentPlaysAgain verifies that a bot rolling doubles onto an opponent's tile
// pays rent and repeats its turn (joga novamente).
func TestBotDoubleRollPayingRentPlaysAgain(t *testing.T) {
	cfg := banco3d.DefaultRoomConfig()
	cfg.Bots.Count = 1
	room, err := banco3d.NewRoom("room-bot-double-rent", "Curitiba", &cfg)
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	human, _ := room.JoinPlayer("Human", "#3b82f6", "car")
	if err := room.StartGame(); err != nil {
		t.Fatalf("erro ao iniciar jogo: %v", err)
	}

	var bot *banco3d.Player
	for _, p := range room.Game.Players {
		if p.IsBot {
			bot = p
			break
		}
	}

	// Casa 6 pertence ao humano
	room.Game.Board.Tiles[6].OwnerID = human.ID
	human.Properties = append(human.Properties, 6)

	room.Game.CurrentTurnPlayerID = bot.ID
	room.Game.DiceRolledThisTurn = false
	bot.Position = 0
	botInitialCash := bot.Money

	// Rola [3, 3] = 6 (dupla!)
	err = room.ExecuteBotTurnWithDice(bot, 3, 3)
	if err != nil {
		t.Fatalf("erro no turno do bot: %v", err)
	}

	// Pagou aluguel
	if bot.Money >= botInitialCash {
		t.Errorf("bot deveria ter pago aluguel na casa 6")
	}

	// REGRA: Tirou dupla -> joga novamente (NÃO passa a vez)
	if room.Game.CurrentTurnPlayerID != bot.ID {
		t.Errorf("dupla de bot com aluguel NÃO pode passar a vez! Esperava CurrentTurnPlayerID=%s, obtido %s", bot.ID, room.Game.CurrentTurnPlayerID)
	}
}

// TestBotDoubleRollDrawingCardPlaysAgain verifies that a bot rolling doubles onto Sorte/Revés
// draws card and repeats its turn (joga novamente).
func TestBotDoubleRollDrawingCardPlaysAgain(t *testing.T) {
	cfg := banco3d.DefaultRoomConfig()
	cfg.Bots.Count = 1
	room, err := banco3d.NewRoom("room-bot-double-card", "Curitiba", &cfg)
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	_, _ = room.JoinPlayer("Human", "#3b82f6", "car")
	if err := room.StartGame(); err != nil {
		t.Fatalf("erro ao iniciar jogo: %v", err)
	}

	var bot *banco3d.Player
	for _, p := range room.Game.Players {
		if p.IsBot {
			bot = p
			break
		}
	}

	// Casa 2 é Revés (Chest). Rola [1, 1] = 2 (dupla!)
	room.Game.CurrentTurnPlayerID = bot.ID
	room.Game.DiceRolledThisTurn = false
	bot.Position = 0

	err = room.ExecuteBotTurnWithDice(bot, 1, 1)
	if err != nil {
		t.Fatalf("erro no turno do bot: %v", err)
	}

	// Se não foi mandado para a prisão por carta, deve jogar de novo
	if !bot.InJail {
		if room.Game.CurrentTurnPlayerID != bot.ID {
			t.Errorf("dupla de bot com carta NÃO pode passar a vez! Esperava CurrentTurnPlayerID=%s, obtido %s", bot.ID, room.Game.CurrentTurnPlayerID)
		}
	}
}

// TestThreeConsecutiveDoublesSendsToJail verifies that 3 consecutive doubles in the same turn
// sends the player (bot or human) immediately to Jail (tile 10) and ends the turn.
func TestThreeConsecutiveDoublesSendsToJail(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-triple-doubles", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// 1ª dupla [1, 1]
	res1, err := game.RollDice(p1.ID, 1, 1)
	if err != nil {
		t.Fatalf("erro roll 1: %v", err)
	}
	if !res1.IsDouble || p1.InJail {
		t.Fatalf("esperava 1ª dupla sem prisão")
	}
	game.ResetForDoubleRoll()

	// 2ª dupla [2, 2]
	res2, err := game.RollDice(p1.ID, 2, 2)
	if err != nil {
		t.Fatalf("erro roll 2: %v", err)
	}
	if !res2.IsDouble || p1.InJail {
		t.Fatalf("esperava 2ª dupla sem prisão")
	}
	game.ResetForDoubleRoll()

	// 3ª dupla [3, 3] -> DEVE IR PARA A PRISÃO IMEDIATAMENTE!
	res3, err := game.RollDice(p1.ID, 3, 3)
	if err != nil {
		t.Fatalf("erro roll 3: %v", err)
	}
	if !p1.InJail || p1.Position != 10 {
		t.Errorf("na 3ª dupla consecutiva o jogador DEVE ir para a prisão (casa 10, in_jail=true), posição atual: %d, inJail: %v", p1.Position, p1.InJail)
	}
	if res3.NewPosition != 10 {
		t.Errorf("res3.NewPosition deve ser 10, obtido %d", res3.NewPosition)
	}
}
