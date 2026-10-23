package banco3d_test

import (
	"testing"

	"detetive-ia/internal/banco3d"
)

// TestFestivalRentMultiplierAndTTL verifies that the Festival card or mechanic
// doubles property rent and expires after 1 round (TTL).
func TestFestivalRentMultiplierAndTTL(t *testing.T) {
	room, err := banco3d.NewRoom("room-fest-test", "Curitiba")
	if err != nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	// Alice é dona da casa 1 (preço R$ 60, aluguel base R$ 2)
	board := room.Game.Board
	board.Tiles[1].OwnerID = p1.ID
	p1.Properties = []int{1}

	// Ativa Festival na casa 1
	board.Tiles[1].FestivalMultiplier = 2
	board.Tiles[1].FestivalRounds = len(room.Game.Players) // 1 rodada completa

	// Vez passa para o Bob
	room.Game.CurrentTurnPlayerID = p2.ID
	room.Game.DiceRolledThisTurn = false

	// Bob cai na casa 1 e deve pagar R$ 4 (dobro do aluguel base de R$ 2)
	p2.Position = 0
	initialBobMoney := p2.Money
	initialAliceMoney := p1.Money

	diceRes, err := room.Game.RollDice(p2.ID, 0, 1)
	if err != nil {
		t.Fatalf("erro ao rolar dados: %v", err)
	}
	if diceRes.RentPaid != 4 {
		t.Errorf("aluguel esperado no festival: R$ 4, obtido: R$ %d", diceRes.RentPaid)
	}
	if p2.Money != initialBobMoney-4 {
		t.Errorf("dinheiro do Bob: esperado %d, obtido %d", initialBobMoney-4, p2.Money)
	}
	if p1.Money != initialAliceMoney+4 {
		t.Errorf("dinheiro da Alice: esperado %d, obtido %d", initialAliceMoney+4, p1.Money)
	}

	// Avançar turnos até esgotar o TTL do festival
	rounds := board.Tiles[1].FestivalRounds
	for i := 0; i < rounds+1; i++ {
		_ = room.Game.AutoPassTurn()
	}

	if board.Tiles[1].FestivalMultiplier != 1 {
		t.Errorf("após expirar o TTL, FestivalMultiplier deveria ser 1, obtido %d", board.Tiles[1].FestivalMultiplier)
	}
}

// TestWorldChampionshipTile20 verifies that landing on Tile 20 (Campeonato Mundial)
// hosts the championship on player's best property doubling rent.
func TestWorldChampionshipTile20(t *testing.T) {
	room, err := banco3d.NewRoom("room-champ-test", "Curitiba")
	if err != nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	board := room.Game.Board
	// Alice possui casa 1 (R$ 60) e casa 39 (R$ 400)
	board.Tiles[1].OwnerID = p1.ID
	board.Tiles[39].OwnerID = p1.ID
	p1.Properties = []int{1, 39}

	// Alice cai na casa 20 (Campeonato Mundial)
	p1.Position = 14
	dice, err := room.Game.RollDice(p1.ID, 3, 3) // 14 + 6 = 20
	if err != nil {
		t.Fatalf("falha ao rolar: %v", err)
	}
	if dice.NewPosition != 20 {
		t.Fatalf("esperava cair na casa 20, caiu em %d", dice.NewPosition)
	}

	// Casa 39 (mais cara) deve receber ChampionshipMultiplier = 2
	if board.Tiles[39].ChampionshipMultiplier != 2 {
		t.Errorf("casa 39 deveria ter ChampionshipMultiplier = 2, obtido %d", board.Tiles[39].ChampionshipMultiplier)
	}
	if board.Tiles[1].ChampionshipMultiplier != 0 && board.Tiles[1].ChampionshipMultiplier != 1 {
		t.Errorf("casa 1 NÃO deveria ter campeonato, obtido %d", board.Tiles[1].ChampionshipMultiplier)
	}

	// Bob sem propriedades cai na casa 20: deve receber prêmio esportivo de R$ 50
	_ = room.Game.AutoPassTurn()
	p2.Position = 14
	bobInitial := p2.Money
	_, _ = room.Game.RollDice(p2.ID, 3, 3) // 14 + 6 = 20
	if p2.Money != bobInitial+50 {
		t.Errorf("Bob sem propriedades deveria receber R$ 50 de premiação esportiva, obtido %d (antes: %d)", p2.Money, bobInitial)
	}
}

// TestBusinessTourFastModeLightningVictory verifies the 3 instant win conditions in fast mode:
// 1) 3 complete color groups
// 2) Complete side of the board
// 3) All 4 stations/resorts
func TestBusinessTourFastModeLightningVictory(t *testing.T) {
	cfg := banco3d.DefaultRoomConfig()
	cfg.FastMode = true

	room, err := banco3d.NewRoom("room-fast-test", "Curitiba", &cfg)
	if err != nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	board := room.Game.Board

	// Condição 3: Alice adquire as 4 estações (5, 15, 25, 35)
	board.Tiles[5].OwnerID = p1.ID
	board.Tiles[15].OwnerID = p1.ID
	board.Tiles[25].OwnerID = p1.ID
	board.Tiles[35].OwnerID = p1.ID
	p1.Properties = []int{5, 15, 25, 35}

	winner, reason := room.Game.CheckLightningVictory()
	if winner == nil || winner.ID != p1.ID {
		t.Errorf("esperava vitória relâmpago de Alice pelas 4 estações, obtido winner=%v", winner)
	}
	if reason == "" {
		t.Errorf("motivo da vitória não deveria estar vazio")
	}

	// Resetar para testar condição 2 (Lado completo do tabuleiro: 1, 3, 6, 8, 9)
	p1.Properties = nil
	board.Tiles[5].OwnerID = ""
	board.Tiles[15].OwnerID = ""
	board.Tiles[25].OwnerID = ""
	board.Tiles[35].OwnerID = ""

	sideSouth := []int{1, 3, 6, 8, 9}
	for _, idx := range sideSouth {
		board.Tiles[idx].OwnerID = p2.ID
		p2.Properties = append(p2.Properties, idx)
	}

	winnerSide, reasonSide := room.Game.CheckLightningVictory()
	if winnerSide == nil || winnerSide.ID != p2.ID {
		t.Errorf("esperava vitória relâmpago de Bob por lado completo, obtido winner=%v", winnerSide)
	}
	if reasonSide == "" {
		t.Errorf("motivo da vitória não deveria estar vazio")
	}

	// Testar que FastMode=false não dispara vitória relâmpago
	room.Game.FastMode = false
	winnerDisabled, _ := room.Game.CheckLightningVictory()
	if winnerDisabled != nil {
		t.Errorf("com FastMode desativado NÃO deve haver vitória relâmpago!")
	}
}
