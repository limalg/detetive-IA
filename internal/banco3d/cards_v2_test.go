package banco3d_test

import (
	"testing"

	"detetive-ia/internal/banco3d"
)

func TestNewThematicCardsCatalogAndImageKind(t *testing.T) {
	deck := banco3d.GetDeckForCity("Curitiba")
	allCards := append([]banco3d.Card{}, deck.Chance...)
	allCards = append(allCards, deck.Chest...)

	imageKindsFound := make(map[string]bool)
	for _, c := range allCards {
		if c.ImageKind != "" {
			imageKindsFound[c.ImageKind] = true
		}
	}

	expectedKinds := []string{
		"earthquake",
		"flood",
		"fire",
		"blackout",
		"strike",
		"lottery",
		"inheritance",
		"festival",
		"inspection",
		"crisis",
	}

	for _, k := range expectedKinds {
		if !imageKindsFound[k] {
			t.Errorf("esperava encontrar ao menos uma carta com image_kind '%s'", k)
		}
	}

	// Pelo menos 12 novas cartas com image_kind preenchido
	countWithImage := 0
	for _, c := range allCards {
		if c.ImageKind != "" {
			countWithImage++
		}
	}
	if countWithImage < 12 {
		t.Errorf("esperava pelo menos 12 cartas com image_kind, encontrado %d", countWithImage)
	}
}

func TestEarthquakeDestroysHouseAndEmitsDestroyEvent(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-earthquake", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	// 1. Dá grupo Marrom para p1 e constrói 1 casa na casa 1
	board.Tiles[1].OwnerID = p1.ID
	board.Tiles[2].OwnerID = p1.ID
	p1.Properties = []int{1, 2}
	_ = game.BuildHouse(p1.ID, 1)

	if board.Tiles[1].Houses != 1 {
		t.Fatalf("casa 1 deveria ter 1 casa construída")
	}

	cardEarthquake := banco3d.Card{
		ID:        "card-eq",
		Deck:      banco3d.CardTypeChance,
		Title:     "Terremoto Urbano",
		Text:      "Um tremor abalou a cidade: 1 casa foi destruída!",
		Effect:    banco3d.CardActionEarthquake,
		ImageKind: "earthquake",
	}

	res, err := game.ResolveCard(p1.ID, cardEarthquake)
	if err != nil {
		t.Fatalf("erro ao resolver carta terremoto: %v", err)
	}

	if res.DestroyedWhat != "house" {
		t.Errorf("esperava DestroyedWhat 'house', obtido '%s'", res.DestroyedWhat)
	}
	if res.DestroyedTile != 1 {
		t.Errorf("esperava DestroyedTile 1, obtido %d", res.DestroyedTile)
	}
	if board.Tiles[1].Houses != 0 {
		t.Errorf("casa 1 deveria estar sem casas após terremoto, possui %d", board.Tiles[1].Houses)
	}

	// 2. Agora constrói hotel (5 casas) na casa 1 e testa terremoto destruindo hotel
	board.Tiles[1].Houses = 5
	resHotel, err := game.ResolveCard(p1.ID, cardEarthquake)
	if err != nil {
		t.Fatalf("erro ao resolver carta terremoto com hotel: %v", err)
	}
	if resHotel.DestroyedWhat != "hotel" {
		t.Errorf("esperava DestroyedWhat 'hotel', obtido '%s'", resHotel.DestroyedWhat)
	}
	if board.Tiles[1].Houses != 4 {
		t.Errorf("hotel destruído deve reverter para 4 casas, obtido %d", board.Tiles[1].Houses)
	}
}

func TestFreeUpgradeBuildsHouse(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-upgrade", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	board.Tiles[1].OwnerID = p1.ID
	board.Tiles[2].OwnerID = p1.ID
	p1.Properties = []int{1, 2}

	cardUpgrade := banco3d.Card{
		ID:        "card-up",
		Deck:      banco3d.CardTypeChest,
		Title:     "Reforma Gratuita",
		Text:      "Ganhe 1 casa grátis em sua propriedade!",
		Effect:    banco3d.CardActionFreeUpgrade,
		ImageKind: "free_upgrade",
	}

	_, err := game.ResolveCard(p1.ID, cardUpgrade)
	if err != nil {
		t.Fatalf("erro ao resolver reforma gratuita: %v", err)
	}

	// Uma das propriedades (1 ou 2) deve ter ganhado 1 casa
	totalHouses := board.Tiles[1].Houses + board.Tiles[2].Houses
	if totalHouses != 1 {
		t.Errorf("esperava 1 casa construída gratuitamente, obtido %d", totalHouses)
	}
}

func TestStrikeMovesBackThreeTiles(t *testing.T) {
	board := banco3d.NewStandardBoard("Curitiba", nil)
	game := banco3d.NewGame("room-strike", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	_, _ = game.AddPlayer("Bot", "#ef4444", "hat")
	_ = game.Start()

	p1.Position = 15
	cardStrike := banco3d.Card{
		ID:        "card-strike",
		Deck:      banco3d.CardTypeChance,
		Title:     "Greve Geral de Ônibus",
		Text:      "Paralisação no transporte: volte 3 casas.",
		Effect:    banco3d.CardActionStrike,
		Amount:    -3,
		ImageKind: "strike",
	}

	res, err := game.ResolveCard(p1.ID, cardStrike)
	if err != nil {
		t.Fatalf("erro ao resolver greve: %v", err)
	}
	if res.NewPosition != 12 {
		t.Errorf("esperava recuar para casa 12, obtido %d", res.NewPosition)
	}
	if p1.Position != 12 {
		t.Errorf("jogador deveria estar na casa 12, obtido %d", p1.Position)
	}
}

func TestDestroyEventEmittedInRoom(t *testing.T) {
	hub := banco3d.NewRoomHub(":memory:")
	room, _ := hub.CreateRoom("DESTROY-ROOM", "Curitiba")
	p1, _ := room.JoinPlayer("Lucas", "#3b82f6", "car")
	_, _ = room.JoinPlayer("Bot", "#ef4444", "hat")
	_ = room.StartGame()

	// Mock de DiceResult com carta de terremoto que destruiu uma casa
	mockCard := banco3d.Card{
		ID:        "eq-1",
		Deck:      banco3d.CardTypeChance,
		Title:     "Terremoto",
		Text:      "Tremor derrubou uma casa!",
		Effect:    banco3d.CardActionEarthquake,
		ImageKind: "earthquake",
	}
	mockDice := &banco3d.DiceResult{
		Die1:        3,
		Die2:        4,
		Total:       7,
		OldPosition: 0,
		NewPosition: 7,
		CardDrawn: &banco3d.CardResult{
			Card:          mockCard,
			PlayerID:      p1.ID,
			NewPosition:   7,
			ImageKind:     "earthquake",
			DestroyedTile: 3,
			DestroyedWhat: "house",
		},
	}

	room.RecordTurnRollAndMove(p1.ID, p1.Name, mockDice)

	var destroyEvt *banco3d.GameEvent
	var cardEvt *banco3d.GameEvent
	for i := range room.Events {
		e := &room.Events[i]
		if e.Type == "destroy" {
			destroyEvt = e
		}
		if e.Type == "card" {
			cardEvt = e
		}
	}

	if cardEvt == nil {
		t.Fatalf("esperava evento 'card'")
	}
	if cardEvt.ImageKind != "earthquake" {
		t.Errorf("cardEvt.ImageKind esperado 'earthquake', obtido '%s'", cardEvt.ImageKind)
	}

	if destroyEvt == nil {
		t.Fatalf("esperava evento 'destroy' quando casa é destruída")
	}
	if destroyEvt.PlayerID != p1.ID {
		t.Errorf("destroyEvt.PlayerID esperado %s, obtido %s", p1.ID, destroyEvt.PlayerID)
	}
	if destroyEvt.Tile != 3 {
		t.Errorf("destroyEvt.Tile esperado 3, obtido %d", destroyEvt.Tile)
	}
	if destroyEvt.What != "house" {
		t.Errorf("destroyEvt.What esperado 'house', obtido '%s'", destroyEvt.What)
	}
}
