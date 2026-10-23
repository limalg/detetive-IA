package banco3d_test

import (
	"testing"
	"time"

	"detetive-ia/internal/banco3d"
)

// TestGoToJailCardMechanics verifies that the 'Vá para a Prisão' card sends player
// directly to Jail (tile 10) without salary and without passing Go.
func TestGoToJailCardMechanics(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-card-jail", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// Posiciona jogador na casa 36 (antes da saída) com R$ 1500
	p1.Position = 36
	mBefore := p1.Money

	cardJail := banco3d.Card{
		ID:        "card-go-to-jail",
		Deck:      banco3d.CardTypeChance,
		Title:     "Vá para a Prisão",
		Text:      "Vá direto para a Prisão sem passar pela Saída e sem receber salário.",
		Effect:    banco3d.CardActionGoToJail,
		ImageKind: "jail",
	}

	res, err := game.ResolveCard(p1.ID, cardJail)
	if err != nil {
		t.Fatalf("erro ao resolver carta de prisão: %v", err)
	}

	if !p1.InJail || !p1.IsJailed {
		t.Errorf("jogador deveria estar preso (in_jail=true)")
	}
	if p1.Position != 10 {
		t.Errorf("jogador deveria estar na casa 10 (prisão), atual: %d", p1.Position)
	}
	if res.SalaryPaid != 0 {
		t.Errorf("não deve receber salário ao ir para a prisão, recebido: %d", res.SalaryPaid)
	}
	if p1.Money != mBefore {
		t.Errorf("saldo não deveria ter mudado por salário, era %d, atual %d", mBefore, p1.Money)
	}
	if res.ImageKind != "jail" {
		t.Errorf("image_kind deve ser 'jail', obtido %s", res.ImageKind)
	}

	// Testar fiança de R$ 50
	err = game.PayJailBail(p1.ID)
	if err != nil {
		t.Fatalf("erro ao pagar fiança: %v", err)
	}
	if p1.InJail || p1.IsJailed {
		t.Errorf("jogador não deveria mais estar preso após pagar fiança")
	}
	if p1.Money != mBefore-50 {
		t.Errorf("fiança deve custar R$ 50, saldo atual %d", p1.Money)
	}
}

// TestMatchMinutesTimeoutAndNetWorthRanking verifies match duration timer
// and winner election by total net worth (patrimônio).
func TestMatchMinutesTimeoutAndNetWorthRanking(t *testing.T) {
	cfg := banco3d.DefaultRoomConfig()
	cfg.MatchMinutes = 15 // 15 minutos
	room, err := banco3d.NewRoom("room-match-time", "Curitiba", &cfg)
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	if room.MatchEndsAt == nil {
		t.Fatalf("room.MatchEndsAt deve estar definido quando MatchMinutes > 0")
	}

	// Alice tem R$ 1000 e propriedade de 400 com 1 casa (custo 200) -> NetWorth = 1000 + 400 + 200 = 1600
	p1.Money = 1000
	room.Game.Board.Tiles[39].OwnerID = p1.ID
	room.Game.Board.Tiles[39].Houses = 1
	p1.Properties = []int{39}

	// Bob tem R$ 1200 e propriedade de 200 sem construções -> NetWorth = 1200 + 200 = 1400
	p2.Money = 1200
	room.Game.Board.Tiles[15].OwnerID = p2.ID
	p2.Properties = []int{15}

	nw1 := room.Game.CalculateNetWorth(p1)
	nw2 := room.Game.CalculateNetWorth(p2)
	if nw1 != 1600 {
		t.Errorf("patrimônio de Alice esperado 1600, obtido %d", nw1)
	}
	if nw2 != 1400 {
		t.Errorf("patrimônio de Bob esperado 1400, obtido %d", nw2)
	}

	// Simula tempo expirado
	past := time.Now().Add(-1 * time.Minute)
	room.MatchEndsAt = &past

	// CheckTurnTimeout deve detectar fim de jogo por tempo
	isTimeout := room.CheckTurnTimeout()
	if !isTimeout {
		t.Errorf("esperava CheckTurnTimeout true por estouro de tempo de partida")
	}
	if !room.Game.IsOver {
		t.Errorf("game.IsOver deve ser true após estouro de tempo")
	}
	if room.Game.WinnerID != p1.ID {
		t.Errorf("Alice (maior patrimônio 1600 vs 1400) deveria ser a vencedora, obtido: %s", room.Game.WinnerID)
	}
}

// TestBotRefusesUnfairTradesAndDemandsMargin verifies that bots do not accept scams
// and require at least ~10% gain in their favor.
func TestBotRefusesUnfairTradesAndDemandsMargin(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-bot-trades", board)
	human, _ := game.AddPlayer("Human", "#3b82f6", "car")
	bot, _ := game.AddPlayer("BotNormal", "#22c55e", "hat")
	bot.IsBot = true
	_ = game.Start()

	// Bot é dono de uma propriedade cara: tile 39 (R$ 400)
	board.Tiles[39].OwnerID = bot.ID
	bot.Properties = []int{39}

	// Humano é dono de uma propriedade barata: tile 1 (R$ 60)
	board.Tiles[1].OwnerID = human.ID
	human.Properties = []int{1}

	// Oferta injusta (Trapaça): Humano oferece casa 1 (R$ 60) e R$ 10 em troca da casa 39 (R$ 400)
	scamProposal := banco3d.TradeProposal{
		SenderID:       human.ID,
		ReceiverID:     bot.ID,
		OfferedTiles:   []int{1},
		OfferedCash:    10,
		RequestedTiles: []int{39},
		RequestedCash:  0,
		Status:         "pending",
	}

	if banco3d.BotAI.EvaluateTrade(game, bot, scamProposal, "normal") {
		t.Errorf("bot NÃO pode aceitar oferta injusta/trapaça!")
	}

	// Oferta vantajosa: Humano oferece R$ 600 em dinheiro pela casa 39 (R$ 400 + margem)
	fairProposal := banco3d.TradeProposal{
		SenderID:       human.ID,
		ReceiverID:     bot.ID,
		OfferedTiles:   nil,
		OfferedCash:    600,
		RequestedTiles: []int{39},
		RequestedCash:  0,
		Status:         "pending",
	}

	if !banco3d.BotAI.EvaluateTrade(game, bot, fairProposal, "normal") {
		t.Errorf("bot deveria aceitar oferta vantajosa com ganho > 10%% a seu favor")
	}
}

// TestPlayerLeaveForfeitMechanics verifies that when a player leaves,
// their properties return to bank and match concludes if only 1 active remains.
func TestPlayerLeaveForfeitMechanics(t *testing.T) {
	room, err := banco3d.NewRoom("room-player-leave", "Curitiba")
	if err != nil {
		t.Fatalf("erro ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	// Alice compra casa 1
	room.Game.Board.Tiles[1].OwnerID = p1.ID
	p1.Properties = []int{1}

	// Alice abandona a partida
	err = room.LeavePlayer(p1.ID)
	if err != nil {
		t.Fatalf("erro ao sair da sala: %v", err)
	}

	if !p1.IsBankrupt {
		t.Errorf("jogador que sai deve ficar falido (is_bankrupt=true)")
	}
	if len(p1.Properties) != 0 {
		t.Errorf("propriedades do jogador que saiu devem ser zeradas")
	}
	if room.Game.Board.Tiles[1].OwnerID != "" {
		t.Errorf("propriedades do jogador que saiu devem voltar ao banco (owner_id vazio), atual: %s", room.Game.Board.Tiles[1].OwnerID)
	}

	// Como restou apenas Bob (1 ativo), a partida deve terminar com vitória de Bob!
	if !room.Game.IsOver {
		t.Errorf("partida deve terminar se restar apenas 1 jogador ativo")
	}
	if room.Game.WinnerID != p2.ID {
		t.Errorf("Bob deveria ser o vencedor, obtido %s", room.Game.WinnerID)
	}
}
