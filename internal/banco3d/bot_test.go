package banco3d

import (
	"testing"
)

func TestBotBuyDecisions(t *testing.T) {
	board := NewStandardBoard("São Paulo", nil)
	game := NewGame("room-bot-1", board)
	p1, _ := game.AddPlayer("BotEasy", "#2563eb", "car")
	p1.IsBot = true

	tileBrown := &board.Tiles[1] // Price 60, Brown

	// 1. Easy bot with cash $120 -> 120 - 60 = 60 < 100 reserve -> should NOT buy
	p1.Money = 120
	if BotAI.ShouldBuy(game, p1, tileBrown, "easy") {
		t.Errorf("easy bot não deveria comprar se dinheiro restante ($60) for menor que reserva ($100)")
	}

	// 2. Easy bot with cash $200 -> 200 - 60 = 140 >= 100 reserve -> SHOULD buy
	p1.Money = 200
	if !BotAI.ShouldBuy(game, p1, tileBrown, "easy") {
		t.Errorf("easy bot deveria comprar se tiver saldo superior à reserva")
	}

	// 3. Hard bot completing group: Squeeze rule
	// Brown group: tile 1 and tile 3.
	tile3 := &board.Tiles[3]
	tile3.OwnerID = p1.ID // bot already owns tile 3

	// Hard reserve is 400. If cash is 440, 440 - 60 = 380 < 400.
	// But completing group halves required reserve to 200! So 380 >= 200 -> should buy!
	p1.Money = 440
	if !BotAI.ShouldBuy(game, p1, tileBrown, "hard") {
		t.Errorf("hard bot deveria realizar squeeze e comprar para completar o grupo de cor")
	}
}

func TestBotBuildDecisions(t *testing.T) {
	board := NewStandardBoard("Curitiba", nil)
	game := NewGame("room-bot-2", board)
	bot, _ := game.AddPlayer("BotBuilder", "#22c55e", "hat")
	bot.IsBot = true
	_, _ = game.AddPlayer("Human", "#3b82f6", "car")

	// Grant bot both Brown properties (tiles 1 and 3)
	board.Tiles[1].OwnerID = bot.ID
	board.Tiles[3].OwnerID = bot.ID
	bot.Properties = []int{1, 3}
	bot.Money = 1000 // plenty of cash
	_ = game.Start()
	game.CurrentTurnPlayerID = bot.ID

	// Should decide to build on tile 1 or 3
	tileIdx, canBuild := BotAI.DecideBuild(game, bot, "normal")
	if !canBuild || (tileIdx != 1 && tileIdx != 3) {
		t.Fatalf("bot deveria conseguir construir no grupo marrom completo, obteve tile %d, canBuild=%v", tileIdx, canBuild)
	}

	// Build on tile 1
	if err := game.BuildHouse(bot.ID, tileIdx); err != nil {
		t.Fatalf("build error on tile %d: %v", tileIdx, err)
	}
	if board.Tiles[tileIdx].Houses != 1 {
		t.Fatalf("casa não foi construída no tile %d", tileIdx)
	}

	// Next build MUST be on the other tile (uniformity)
	otherIdx, canBuild2 := BotAI.DecideBuild(game, bot, "normal")
	if !canBuild2 || otherIdx == tileIdx {
		t.Fatalf("segunda construção deveria ser no outro terreno para manter uniformidade, obteve %d", otherIdx)
	}
}

func TestBotTradeEvaluation(t *testing.T) {
	board := NewStandardBoard("Rio de Janeiro", nil)
	game := NewGame("room-bot-3", board)
	bot, _ := game.AddPlayer("BotTrader", "#ef4444", "ship")
	bot.IsBot = true
	bot.Money = 500

	// Proposal offering $200 and tile 1 (value 60+4=64 -> total 264)
	// Requesting tile 2 (value 60+4=64)
	propGood := TradeProposal{
		OfferedCash:    200,
		OfferedTiles:   []int{1},
		RequestedCash:  0,
		RequestedTiles: []int{2},
	}
	if !BotAI.EvaluateTrade(game, bot, propGood, "normal") {
		t.Errorf("bot normal deveria aceitar proposta vantajosa")
	}

	// Proposal taking $400 for nothing
	propBad := TradeProposal{
		OfferedCash:    10,
		OfferedTiles:   []int{},
		RequestedCash:  400,
		RequestedTiles: []int{},
	}
	if BotAI.EvaluateTrade(game, bot, propBad, "normal") {
		t.Errorf("bot não deve aceitar proposta prejudicial")
	}
}
