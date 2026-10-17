package banco_test

import (
	"strings"
	"testing"

	"detetive-ia/internal/banco"
)

func TestNewGame_Initialization(t *testing.T) {
	opts := banco.GameOptions{
		City:         "São Paulo",
		Age:          14,
		SchoolGrade:  "9º Ano",
		Difficulty:   "medio",
		BotsCount:    3,
		HumanName:    "Detetive Player",
		Friends:      []string{"Amigo 1"},
	}

	game := banco.NewGame(opts)
	if game == nil {
		t.Fatalf("expected game instance, got nil")
	}

	state := game.GetState()
	if state.City != "São Paulo" {
		t.Errorf("expected city 'São Paulo', got %q", state.City)
	}

	if len(state.Players) < 3 {
		t.Fatalf("expected at least 3 players, got %d", len(state.Players))
	}

	// First player must be the human player
	human := state.Players[0]
	if human.IsBot {
		t.Errorf("expected player 0 to be human, got bot")
	}
	if human.Name != "Detetive Player" {
		t.Errorf("expected human name 'Detetive Player', got %q", human.Name)
	}
	if human.Balance <= 0 {
		t.Errorf("expected positive starting balance, got %d", human.Balance)
	}

	// Board checks
	if len(state.Board) != 28 {
		t.Errorf("expected 28 board tiles, got %d", len(state.Board))
	}

	// Tile 0 must be START
	if state.Board[0].Type != banco.TileStart {
		t.Errorf("expected tile 0 to be START, got %s", state.Board[0].Type)
	}

	// Check São Paulo landmarks present
	var hasPaulista bool
	for _, tile := range state.Board {
		if strings.Contains(tile.Name, "Paulista") || strings.Contains(tile.Name, "Faria Lima") {
			hasPaulista = true
			break
		}
	}
	if !hasPaulista {
		t.Errorf("expected São Paulo landmarks (Paulista/Faria Lima) in board tiles")
	}
}

func TestProceduralCityBoard(t *testing.T) {
	opts := banco.GameOptions{
		City:        "Florianópolis",
		Age:         12,
		SchoolGrade: "7º Ano",
		BotsCount:   2,
	}

	game := banco.NewGame(opts)
	state := game.GetState()

	if state.City != "Florianópolis" {
		t.Errorf("expected city 'Florianópolis', got %q", state.City)
	}

	var hasCustomCityRef bool
	for _, tile := range state.Board {
		if strings.Contains(tile.Name, "Florianópolis") || tile.Price > 0 {
			hasCustomCityRef = true
			break
		}
	}
	if !hasCustomCityRef {
		t.Errorf("expected procedurally generated tiles for Florianópolis")
	}
}

func TestGame_RollAndMove(t *testing.T) {
	opts := banco.GameOptions{
		City:      "Curitiba",
		BotsCount: 1,
	}
	game := banco.NewGame(opts)
	state := game.GetState()

	human := state.Players[0]
	initialPos := human.Position

	rollRes, err := game.RollDice(human.ID)
	if err != nil {
		t.Fatalf("unexpected error rolling dice: %v", err)
	}

	if rollRes.Total < 2 || rollRes.Total > 12 {
		t.Errorf("expected dice total between 2 and 12, got %d", rollRes.Total)
	}

	newState := game.GetState()
	expectedPos := (initialPos + rollRes.Total) % len(newState.Board)
	if newState.Players[0].Position != expectedPos {
		t.Errorf("expected position %d, got %d", expectedPos, newState.Players[0].Position)
	}
}

func TestGame_BuyPropertyAndPayRent(t *testing.T) {
	opts := banco.GameOptions{
		City:      "Rio de Janeiro",
		BotsCount: 1,
	}
	game := banco.NewGame(opts)

	// Move player 0 manually to a property tile
	game.ForcePlayerPosition(1, 1) // Player 1 to Tile 1 (Property)
	tile1 := game.GetState().Board[1]
	if tile1.Type != banco.TileProperty {
		t.Fatalf("expected tile 1 to be property, got %s", tile1.Type)
	}

	balanceBefore := game.GetPlayer(1).Balance
	err := game.BuyProperty(1, 1)
	if err != nil {
		t.Fatalf("unexpected error buying property: %v", err)
	}

	p1After := game.GetPlayer(1)
	if p1After.Balance != balanceBefore-tile1.Price {
		t.Errorf("expected balance %d, got %d", balanceBefore-tile1.Price, p1After.Balance)
	}

	// Verify ownership
	updatedTile := game.GetState().Board[1]
	if updatedTile.OwnerID != 1 {
		t.Errorf("expected tile 1 owner 1, got %d", updatedTile.OwnerID)
	}

	// Move player 2 (bot) to tile 1 -> should trigger rent payment
	p2BalanceBefore := game.GetPlayer(2).Balance
	p1BalanceBefore := game.GetPlayer(1).Balance

	game.ForcePlayerPosition(2, 1)
	game.ProcessLanding(2)

	p2After := game.GetPlayer(2)
	p1Final := game.GetPlayer(1)

	if p2After.Balance >= p2BalanceBefore {
		t.Errorf("expected player 2 balance to decrease after rent, was %d now %d", p2BalanceBefore, p2After.Balance)
	}
	if p1Final.Balance <= p1BalanceBefore {
		t.Errorf("expected player 1 balance to increase after rent, was %d now %d", p1BalanceBefore, p1Final.Balance)
	}

}

func TestGame_BotTurnExecution(t *testing.T) {
	opts := banco.GameOptions{
		City:       "São Paulo",
		BotsCount:  2,
		Difficulty: "dificil", // aggressive bot
	}
	game := banco.NewGame(opts)

	// Set turn to player 2 (bot)
	game.SetCurrentTurn(2)
	logs, nextTurn, err := game.PlayBotTurns()
	if err != nil {
		t.Fatalf("unexpected error playing bot turns: %v", err)
	}

	if len(logs) == 0 {
		t.Errorf("expected bot action logs, got empty")
	}

	// Next turn should loop back to human (player 1)
	if nextTurn != 1 {
		t.Errorf("expected next turn to be human (1), got %d", nextTurn)
	}
}
