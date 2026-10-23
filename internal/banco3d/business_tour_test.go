package banco3d_test

import (
	"math/rand"
	"testing"
	"time"

	"detetive-ia/internal/banco3d"
)

func TestBusinessTourBoardLayoutAndAscendingPrices(t *testing.T) {
	board, candidates, err := banco3d.GenerateCityBoard("Curitiba", "")
	if err != nil {
		t.Fatalf("erro ao gerar tabuleiro: %v", err)
	}

	if len(board.Tiles) != 40 {
		t.Fatalf("esperado 40 casas, obtido %d", len(board.Tiles))
	}

	// 1. Validar posições clássicas dos cantos
	if board.Tiles[0].Type != banco3d.TileGo {
		t.Errorf("casa 0 deve ser GO, obtido %s", board.Tiles[0].Type)
	}
	if board.Tiles[10].Type != banco3d.TileJail {
		t.Errorf("casa 10 deve ser JAIL, obtido %s", board.Tiles[10].Type)
	}
	if board.Tiles[20].Type != banco3d.TileFreeParking {
		t.Errorf("casa 20 deve ser FREE_PARKING, obtido %s", board.Tiles[20].Type)
	}
	if board.Tiles[30].Type != banco3d.TileGoToJail {
		t.Errorf("casa 30 deve ser GO_TO_JAIL, obtido %s", board.Tiles[30].Type)
	}

	// 2. Validar estações clássicas (5, 15, 25, 35)
	stationPositions := []int{5, 15, 25, 35}
	for _, pos := range stationPositions {
		if board.Tiles[pos].Type != banco3d.TileStation {
			t.Errorf("casa %d deve ser STATION no padrão clássico/Business Tour, obtido %s", pos, board.Tiles[pos].Type)
		}
		if board.Tiles[pos].Price != 200 {
			t.Errorf("estação na casa %d deve custar 200, obtido %d", pos, board.Tiles[pos].Price)
		}
	}

	// 3. Validar companhias/utilitários clássicos (12, 28)
	utilityPositions := []int{12, 28}
	for _, pos := range utilityPositions {
		if board.Tiles[pos].Type != banco3d.TileUtility {
			t.Errorf("casa %d deve ser UTILITY no padrão clássico/Business Tour, obtido %s", pos, board.Tiles[pos].Type)
		}
		if board.Tiles[pos].Price != 150 {
			t.Errorf("utilitário na casa %d deve custar 150, obtido %d", pos, board.Tiles[pos].Price)
		}
	}

	// 4. Validar curva crescente de preços nas propriedades (60 -> 400)
	lastPrice := 0
	propIndices := []int{1, 3, 6, 8, 9, 11, 13, 14, 16, 18, 19, 21, 23, 24, 26, 27, 29, 31, 32, 34, 37, 39}
	for _, idx := range propIndices {
		tile := board.Tiles[idx]
		if tile.Type != banco3d.TileProperty {
			t.Errorf("casa %d deveria ser PROPERTY, obtido %s", idx, tile.Type)
		}
		if tile.Price < lastPrice {
			t.Errorf("preço da casa %d (%d) deve ser >= ao da propriedade anterior (%d)", idx, tile.Price, lastPrice)
		}
		lastPrice = tile.Price
	}

	// Primeira propriedade deve ser R$ 60 (Marrom) e última R$ 400 (Azul escuro)
	if board.Tiles[1].Price != 60 {
		t.Errorf("primeira propriedade (casa 1) deve custar 60, obtido %d", board.Tiles[1].Price)
	}
	if board.Tiles[39].Price != 400 {
		t.Errorf("última propriedade (casa 39) deve custar 400, obtido %d", board.Tiles[39].Price)
	}

	// 5. Validar votação: o vencedor deve ser alocado na casa 39 (topo de prestígio)
	if len(candidates) != 4 {
		t.Fatalf("esperava 4 candidatos, obtido %d", len(candidates))
	}

	session := banco3d.NewVotingSession(candidates, 15*time.Second)
	_ = session.Vote("p1", 2) // vota no candidato de índice 2
	winner, err := session.CloseAndElect(board)
	if err != nil {
		t.Fatalf("erro ao concluir votação: %v", err)
	}
	if winner != candidates[2] {
		t.Errorf("vencedor esperado %s, obtido %s", candidates[2], winner)
	}
	if board.Tiles[39].Name != winner {
		t.Errorf("casa 39 (topo) deve ter o nome do vencedor da votação '%s', obtido '%s'", winner, board.Tiles[39].Name)
	}
	if board.Tiles[39].Price != 400 {
		t.Errorf("casa 39 deve ter preço clássico 400, obtido %d", board.Tiles[39].Price)
	}
}

func TestEarthquakeRarePropertyLossWhenNoBuildings(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-eq-prop", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// Dá casa 1 para p1 sem nenhuma construção (Houses == 0)
	board.Tiles[1].OwnerID = p1.ID
	p1.Properties = []int{1}

	cardEarthquake := banco3d.Card{
		ID:        "card-eq-rare",
		Deck:      banco3d.CardTypeChance,
		Title:     "Grande Falha Geológica",
		Text:      "Terremoto severo causou deslizamento de encosta!",
		Effect:    banco3d.CardActionEarthquake,
		ImageKind: "earthquake",
	}

	// Força semente ou simulação para o caso raro (10%):
	// Testamos chamando a resolução onde a propriedade volta ao banco
	rand.Seed(42) // Semente controlada ou loop
	successPropLoss := false
	for i := 0; i < 50; i++ {
		board.Tiles[1].OwnerID = p1.ID
		board.Tiles[1].Houses = 0
		p1.Properties = []int{1}

		res, err := game.ResolveCard(p1.ID, cardEarthquake)
		if err != nil {
			t.Fatalf("erro ao resolver carta: %v", err)
		}
		if res.DestroyedWhat == "property" {
			successPropLoss = true
			if res.DestroyedTile != 1 {
				t.Errorf("esperado DestroyedTile=1, obtido %d", res.DestroyedTile)
			}
			if board.Tiles[1].OwnerID != "" {
				t.Errorf("propriedade destruída deveria voltar ao banco (owner_id vazio), atual: %s", board.Tiles[1].OwnerID)
			}
			if len(p1.Properties) != 0 {
				t.Errorf("propriedade deveria ter sido removida da lista do jogador")
			}
			break
		}
	}

	if !successPropLoss {
		t.Errorf("esperava que em 50 iterações com chance de 10%% ocorresse o caso raro de perda de propriedade")
	}
}
