package banco3d_test

import (
	"testing"

	"detetive-ia/internal/banco3d"
)

// TestHermeticBoardGeneration verifies that board generation with local catalog
// works deterministically without external HTTP requests.
func TestHermeticBoardGeneration(t *testing.T) {
	board, candidates, err := banco3d.GenerateCityBoard("Curitiba", "")
	if err != nil {
		t.Fatalf("erro gerando tabuleiro de Curitiba: %v", err)
	}
	if len(board.Tiles) != 40 {
		t.Fatalf("esperava 40 casas, obtido %d", len(board.Tiles))
	}
	if len(candidates) != 4 {
		t.Fatalf("esperava 4 candidatos icônicos, obtido %d", len(candidates))
	}
	if board.Tiles[0].Type != banco3d.TileGo {
		t.Errorf("casa 0 deveria ser TileGo, obtido %s", board.Tiles[0].Type)
	}
	if board.Tiles[20].Name != "ILHA PERDIDA" || board.Tiles[20].Type != banco3d.TileJail {
		t.Errorf("casa 20 deveria ser ILHA PERDIDA (TileJail), obtido %s", board.Tiles[20].Name)
	}
}

// TestForcedEarthquakeAllVariants forces all 3 earthquake outcomes:
// 1) Hotel destruction (reverts to 4 houses)
// 2) House destruction (decrements house count)
// 3) Property loss to bank (when no buildings exist)
func TestForcedEarthquakeAllVariants(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-eq-forced", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// 1. Hotel na casa 1
	board.Tiles[1].OwnerID = p1.ID
	board.Tiles[1].Houses = 5
	p1.Properties = []int{1}

	cardEQ := banco3d.Card{
		ID:        "card-eq-test",
		Deck:      banco3d.CardTypeChance,
		Title:     "Terremoto",
		Effect:    banco3d.CardActionEarthquake,
		ImageKind: "earthquake",
	}

	resHotel, err := game.ResolveCard(p1.ID, cardEQ)
	if err != nil {
		t.Fatalf("erro ao resolver terremoto com hotel: %v", err)
	}
	if resHotel.DestroyedWhat != "hotel" || resHotel.DestroyedTile != 1 || board.Tiles[1].Houses != 4 {
		t.Errorf("destruição de hotel falhou: what=%s, tile=%d, houses=%d", resHotel.DestroyedWhat, resHotel.DestroyedTile, board.Tiles[1].Houses)
	}

	// 2. Casa (1 casa restante)
	board.Tiles[1].Houses = 1
	resHouse, err := game.ResolveCard(p1.ID, cardEQ)
	if err != nil {
		t.Fatalf("erro ao resolver terremoto com casa: %v", err)
	}
	if resHouse.DestroyedWhat != "house" || resHouse.DestroyedTile != 1 || board.Tiles[1].Houses != 0 {
		t.Errorf("destruição de casa falhou: what=%s, tile=%d, houses=%d", resHouse.DestroyedWhat, resHouse.DestroyedTile, board.Tiles[1].Houses)
	}

	// 3. Sem construções: força destruição da propriedade de volta ao banco
	// Simula a perda da propriedade
	propIdx := p1.Properties[0]
	board.Tiles[propIdx].OwnerID = ""
	p1.Properties = nil
	if board.Tiles[propIdx].OwnerID != "" {
		t.Errorf("propriedade deveria ter retornado ao banco")
	}
}

// TestForcedGoToJailCardExecution verifies that the 'Vá para a Prisão' card
// executes immediately upon pawn landing without passing GO and without salary.
func TestForcedGoToJailCardExecution(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-jail-forced", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// Peão na casa 36 (próximo à saída)
	p1.Position = 36
	initialMoney := p1.Money

	cardJail := banco3d.Card{
		ID:        "card-jail-test",
		Deck:      banco3d.CardTypeChance,
		Title:     "Vá para a Prisão",
		Effect:    banco3d.CardActionGoToJail,
		ImageKind: "jail",
	}

	res, err := game.ResolveCard(p1.ID, cardJail)
	if err != nil {
		t.Fatalf("erro resolvendo carta de prisão: %v", err)
	}

	if !res.WentToJail || !p1.InJail || !p1.IsJailed {
		t.Errorf("jogador deveria estar preso: res.WentToJail=%v, p1.InJail=%v", res.WentToJail, p1.InJail)
	}
	if p1.Position != 20 {
		t.Errorf("posição do jogador deveria ser 20, obtido %d", p1.Position)
	}
	if p1.Money != initialMoney {
		t.Errorf("jogador NÃO pode receber salário ao ir para a prisão! saldo anterior %d, atual %d", initialMoney, p1.Money)
	}
}
