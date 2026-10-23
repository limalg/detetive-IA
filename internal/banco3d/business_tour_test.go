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

// TestWorldChampionshipCard verifies that the Championship effect (rent x3)
// is activated via Sorte ou Revés card on player's most valuable property.
func TestWorldChampionshipCard(t *testing.T) {
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

	cardChamp := banco3d.Card{
		ID:     "c-champ-test",
		Deck:   banco3d.CardTypeLuck,
		Type:   banco3d.CardTypeLuck,
		Title:  "Campeonato Mundial",
		Effect: banco3d.CardActionChampionship,
		Action: banco3d.CardActionChampionship,
	}

	resAlice, err := room.Game.ResolveCard(p1.ID, cardChamp)
	if err != nil {
		t.Fatalf("erro ao resolver carta campeonato: %v", err)
	}
	if resAlice.ChampionshipTile != 39 {
		t.Errorf("campeonato deveria ser na casa 39, obtido %d", resAlice.ChampionshipTile)
	}
	if board.Tiles[39].ChampionshipMultiplier != 3 {
		t.Errorf("casa 39 deveria ter ChampionshipMultiplier = 3, obtido %d", board.Tiles[39].ChampionshipMultiplier)
	}

	// Bob sem propriedades: recebe prêmio esportivo de R$ 50
	bobInitial := p2.Money
	resBob, _ := room.Game.ResolveCard(p2.ID, cardChamp)
	if p2.Money != bobInitial+50 || resBob.MoneyDelta != 50 {
		t.Errorf("Bob sem propriedades deveria receber R$ 50 de premiação esportiva, saldo: %d", p2.Money)
	}
}

// TestBusinessTourCornersAndSpecialTax verifies the 4 corners:
// 10=TRAVEL (safe visit), 20=JAIL (visitor safe visit), 30=SPECIAL_TAX (draw card + 10% tax max 200)
func TestBusinessTourCornersAndSpecialTax(t *testing.T) {
	room, _ := banco3d.NewRoom("room-corners-test", "Curitiba")
	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	_, _ = room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	// 1. Casa 10: VIAGEM (World Tour) - visita livre sem penalidade
	p1.Position = 8
	dice10, err := room.Game.RollDice(p1.ID, 1, 1)
	if err != nil || dice10.NewPosition != 10 {
		t.Fatalf("erro rolando para casa 10: %v, pos: %d", err, dice10.NewPosition)
	}
	if p1.InJail || p1.IsJailed {
		t.Errorf("casa 10 (VIAGEM) NÃO deve prender jogador!")
	}
	if dice10.RentPaid != 0 {
		t.Errorf("casa 10 (VIAGEM) não deve cobrar aluguel ou taxa!")
	}

	// 2. Casa 20: ILHA PERDIDA - visitante passa sem efeito
	_ = room.Game.AutoPassTurn()
	room.Game.CurrentTurnPlayerID = p1.ID
	room.Game.DiceRolledThisTurn = false
	p1.Position = 14
	dice20, err := room.Game.RollDice(p1.ID, 3, 3)
	if err != nil || dice20.NewPosition != 20 {
		t.Fatalf("erro rolando para casa 20: %v, pos: %d", err, dice20.NewPosition)
	}
	if p1.InJail || p1.IsJailed {
		t.Errorf("ao cair na casa 20 (ILHA PERDIDA) andando, jogador é mero visitante e NÃO vai para a prisão!")
	}

	// 3. Casa 30: IMPOSTO ESPECIAL - sorteia carta Sorte ou Revés e paga 10% do caixa (máx R$ 200)
	_ = room.Game.AutoPassTurn()
	room.Game.CurrentTurnPlayerID = p1.ID
	room.Game.DiceRolledThisTurn = false
	p1.Position = 24
	p1.Money = 1500
	dice30, err := room.Game.RollDice(p1.ID, 2, 4) // 24 + 6 = 30
	if err != nil || dice30.NewPosition != 30 {
		t.Fatalf("erro rolando para casa 30: %v, pos: %d", err, dice30.NewPosition)
	}
	if dice30.RentPaid > 200 {
		t.Errorf("imposto especial não pode exceder R$ 200, cobrou: %d", dice30.RentPaid)
	}
	if dice30.RentPaid <= 0 {
		t.Errorf("imposto especial deveria ser cobrado (10%% do caixa, máx 200), cobrou: %d", dice30.RentPaid)
	}
	if dice30.CardDrawn == nil {
		t.Errorf("casa 30 deve sortear uma carta de Sorte ou Revés!")
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

// TestBuildUniformHouseAndHotelRules validates that:
// - Building requires full group ownership
// - Building requires player's turn
// - Uniform building is strictly enforced (cannot build 2nd house if another property has 0)
// - 5th building creates a hotel
// - Max 5 builds (hotel)
// - Monopoly doubles base rent
func TestBuildUniformHouseAndHotelRules(t *testing.T) {
	room, err := banco3d.NewRoom("room-build-test", "Curitiba")
	if err != nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}

	p1, _ := room.JoinPlayer("Alice", "#3b82f6", "car")
	p2, _ := room.JoinPlayer("Bob", "#ef4444", "hat")
	_ = room.StartGame()

	board := room.Game.Board

	// 1. Alice possui apenas casa 1 (do grupo Marrom: casas 1 e 3)
	board.Tiles[1].OwnerID = p1.ID
	p1.Properties = []int{1}

	// Tentar construir na casa 1 sem ter o grupo completo -> DEVE FALHAR
	if err := room.Game.BuildHouse(p1.ID, 1); err == nil {
		t.Errorf("construção sem grupo completo DEVE retornar erro")
	}

	// 2. Bob tenta construir na casa 1 na vez da Alice -> DEVE FALHAR (não é a vez)
	if err := room.Game.BuildHouse(p2.ID, 1); err == nil {
		t.Errorf("construção fora da vez DEVE retornar erro")
	}

	// 3. Alice completa o grupo Marrom (casas 1 e 3)
	board.Tiles[3].OwnerID = p1.ID
	p1.Properties = []int{1, 3}

	// Sem casas: verificar que monopólio dobra o aluguel base
	rentBase := board.Tiles[1].Rent[0]
	// Bob cai na casa 1 sem casas:
	room.Game.CurrentTurnPlayerID = p2.ID
	room.Game.DiceRolledThisTurn = false
	p2.Position = 0
	diceRes, _ := room.Game.RollDice(p2.ID, 0, 1)
	if diceRes.RentPaid != rentBase*2 {
		t.Errorf("monopólio sem casas deve dobrar aluguel base: esperado %d, obtido %d", rentBase*2, diceRes.RentPaid)
	}

	// Volta a vez para Alice
	room.Game.CurrentTurnPlayerID = p1.ID
	room.Game.DiceRolledThisTurn = true

	// 4. Alice constrói 1 casa na casa 1 -> sucesso!
	initialMoney := p1.Money
	cost := board.Tiles[1].HouseCost
	if err := room.Game.BuildHouse(p1.ID, 1); err != nil {
		t.Fatalf("falha ao construir 1ª casa na casa 1: %v", err)
	}
	if board.Tiles[1].Houses != 1 {
		t.Errorf("esperava 1 casa, obtido %d", board.Tiles[1].Houses)
	}
	if p1.Money != initialMoney-cost {
		t.Errorf("esperava desconto de R$ %d, saldo foi de %d para %d", cost, initialMoney, p1.Money)
	}

	// 5. Alice tenta construir a 2ª casa na casa 1 enquanto a casa 3 tem 0 casas -> DEVE FALHAR (construção uniforme)
	if err := room.Game.BuildHouse(p1.ID, 1); err == nil {
		t.Errorf("construção não uniforme (2 casas na 1 enquanto casa 3 tem 0) DEVE retornar erro")
	}

	// 6. Constrói 1ª casa na casa 3 -> sucesso!
	if err := room.Game.BuildHouse(p1.ID, 3); err != nil {
		t.Fatalf("falha ao construir casa na 3 para nivelar: %v", err)
	}

	// 7. Agora ambas têm 1 casa: construir até 4 casas e depois hotel (5)
	for level := 2; level <= 4; level++ {
		if err := room.Game.BuildHouse(p1.ID, 1); err != nil {
			t.Fatalf("falha ao construir nível %d na casa 1: %v", level, err)
		}
		if err := room.Game.BuildHouse(p1.ID, 3); err != nil {
			t.Fatalf("falha ao construir nível %d na casa 3: %v", level, err)
		}
	}

	// 8. Construir hotel (nível 5)
	if err := room.Game.BuildHouse(p1.ID, 1); err != nil {
		t.Fatalf("falha ao construir hotel (nível 5) na casa 1: %v", err)
	}
	if board.Tiles[1].Houses != 5 {
		t.Errorf("esperava 5 construções (hotel), obtido %d", board.Tiles[1].Houses)
	}

	// 9. Tentar construir além de hotel -> DEVE FALHAR
	if err := room.Game.BuildHouse(p1.ID, 1); err == nil {
		t.Errorf("construção além de hotel DEVE retornar erro")
	}
}
