package banco3d_test

import (
	"strings"
	"testing"

	"detetive-ia/internal/banco3d"
)

func TestThematicCardsCatalog(t *testing.T) {
	cities := []string{"Curitiba", "São Paulo", "Rio de Janeiro", "Belo Horizonte", "Salvador", "Nova York"}
	for _, city := range cities {
		deck := banco3d.GetDeckForCity(city)
		if len(deck.Chance) < 6 {
			t.Errorf("deck de Sorte para %s deve ter ao menos 6 cartas, possui %d", city, len(deck.Chance))
		}
		if len(deck.Chest) < 6 {
			t.Errorf("deck de Revés para %s deve ter ao menos 6 cartas, possui %d", city, len(deck.Chest))
		}

		// Valida se as cartas possuem texto não vazio
		for _, c := range deck.Chance {
			if strings.TrimSpace(c.Text) == "" {
				t.Errorf("carta de Sorte com texto vazio em %s", city)
			}
		}
		for _, c := range deck.Chest {
			if strings.TrimSpace(c.Text) == "" {
				t.Errorf("carta de Revés com texto vazio em %s", city)
			}
		}
	}
}

func TestUnknownCityUsesGenericCards(t *testing.T) {
	deck := banco3d.GetDeckForCity("Cidade Desconhecida 123")
	if len(deck.Chance) == 0 || len(deck.Chest) == 0 {
		t.Fatal("cidade desconhecida deve retornar cartas genéricas")
	}
}

func TestCardExecutionInGame(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-cards", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	p2, _ := game.AddPlayer("Ana", "#ef4444", "hat")
	game.Start()

	// 1. Testar aplicação de carta de dinheiro (Recompensa)
	cardMoney := banco3d.Card{
		ID:     "c1",
		Type:   banco3d.CardTypeChance,
		Title:  "Bônus de Inovação",
		Text:   "Ganhe R$ 100",
		Action: banco3d.CardActionMoney,
		Value:  100,
	}
	initialMoney := p1.Money
	game.ApplyCard(p1.ID, cardMoney)
	if p1.Money != initialMoney+100 {
		t.Errorf("esperado R$ %d, obtido %d", initialMoney+100, p1.Money)
	}

	// 2. Testar aplicação de carta de Vá para a Prisão
	cardJail := banco3d.Card{
		ID:     "c2",
		Type:   banco3d.CardTypeChance,
		Title:  "Vá para a Prisão",
		Text:   "Vá direto para a Prisão",
		Action: banco3d.CardActionGoToJail,
	}
	game.ApplyCard(p1.ID, cardJail)
	if p1.Position != 20 {
		t.Errorf("jogador deveria estar na casa 20 (prisão), está em %d", p1.Position)
	}
	if !p1.IsJailed {
		t.Errorf("jogador deveria estar preso")
	}

	// 3. Testar Saída Livre da Prisão
	cardFree := banco3d.Card{
		ID:     "c3",
		Type:   banco3d.CardTypeChest,
		Title:  "Habeas Corpus",
		Text:   "Saída livre da prisão",
		Action: banco3d.CardActionGetOutOfJail,
	}
	game.ApplyCard(p1.ID, cardFree)
	if p1.GetOutOfJailCards != 1 {
		t.Errorf("jogador deveria ter 1 carta de saída da prisão, possui %d", p1.GetOutOfJailCards)
	}

	// 4. Testar Cobrança de Todos os Jogadores
	cardCollectAll := banco3d.Card{
		ID:     "c4",
		Type:   banco3d.CardTypeChance,
		Title:  "Taxa de Turismo",
		Text:   "Colete R$ 50 de cada jogador",
		Action: banco3d.CardActionCollectFromPlayers,
		Value:  50,
	}
	p1Before := p1.Money
	p2Before := p2.Money
	game.ApplyCard(p1.ID, cardCollectAll)
	if p1.Money != p1Before+50 {
		t.Errorf("p1 deveria receber 50, saldo %d", p1.Money)
	}
	if p2.Money != p2Before-50 {
		t.Errorf("p2 deveria pagar 50, saldo %d", p2.Money)
	}
}
