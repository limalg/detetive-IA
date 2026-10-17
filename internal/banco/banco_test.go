package banco_test

import (
	"path/filepath"
	"strings"
	"testing"

	"detetive-ia/internal/banco"
	"detetive-ia/internal/db"
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

func TestGame_TradeProperties_Success(t *testing.T) {
	opts := banco.GameOptions{
		City:       "São Paulo",
		BotsCount:  1,
		Difficulty: "facil",
	}
	game := banco.NewGame(opts)

	// Buy tile 1 for player 1 and tile 3 for player 2
	game.ForcePlayerPosition(1, 1)
	if err := game.BuyProperty(1, 1); err != nil {
		t.Fatalf("failed to buy tile 1 for player 1: %v", err)
	}
	game.ForcePlayerPosition(2, 3)
	if err := game.BuyProperty(2, 3); err != nil {
		t.Fatalf("failed to buy tile 3 for player 2: %v", err)
	}

	p1BalanceBefore := game.GetPlayer(1).Balance
	p2BalanceBefore := game.GetPlayer(2).Balance

	// Player 1 proposes trade: offered tile 1 + R$ 100 cash offer, requested tile 3
	tradeProp := banco.TradeProposal{
		FromPlayerID:   1,
		ToPlayerID:     2,
		OfferedProps:   []int{1},
		RequestedProps: []int{3},
		CashOffer:      100,
		CashRequest:    0,
	}

	result, err := game.TradeProperties(tradeProp)
	if err != nil {
		t.Fatalf("unexpected error on trade: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected trade to be accepted, reason: %s, message: %s", result.Reason, result.Message)
	}

	// Verify property ownership swapped
	tile1 := game.GetState().Board[1]
	tile3 := game.GetState().Board[3]
	if tile1.OwnerID != 2 {
		t.Errorf("expected tile 1 to be owned by player 2, got %d", tile1.OwnerID)
	}
	if tile3.OwnerID != 1 {
		t.Errorf("expected tile 3 to be owned by player 1, got %d", tile3.OwnerID)
	}

	// Verify cash adjusted
	p1After := game.GetPlayer(1).Balance
	p2After := game.GetPlayer(2).Balance
	if p1After != p1BalanceBefore-100 {
		t.Errorf("expected player 1 balance %d, got %d", p1BalanceBefore-100, p1After)
	}
	if p2After != p2BalanceBefore+100 {
		t.Errorf("expected player 2 balance %d, got %d", p2BalanceBefore+100, p2After)
	}
}

func TestGame_TradeProperties_BotRejection(t *testing.T) {
	opts := banco.GameOptions{
		City:       "São Paulo",
		BotsCount:  1,
		Difficulty: "dificil", // strict bot
	}
	game := banco.NewGame(opts)

	game.ForcePlayerPosition(2, 3)
	if err := game.BuyProperty(2, 3); err != nil {
		t.Fatalf("failed to buy tile 3 for player 2: %v", err)
	}

	// Player 1 offers nothing and asks for tile 3 with 0 cash
	tradeProp := banco.TradeProposal{
		FromPlayerID:   1,
		ToPlayerID:     2,
		OfferedProps:   []int{},
		RequestedProps: []int{3},
		CashOffer:      0,
		CashRequest:    0,
	}

	result, err := game.TradeProperties(tradeProp)
	if err != nil {
		t.Fatalf("unexpected error on trade: %v", err)
	}
	if result.Accepted {
		t.Errorf("expected bot to reject unfair trade")
	}
	if result.Reason != "insufficient_value" {
		t.Errorf("expected reason 'insufficient_value', got %q", result.Reason)
	}
}

func TestGame_RequestLoan(t *testing.T) {
	opts := banco.GameOptions{
		City:      "Curitiba",
		BotsCount: 1,
	}
	game := banco.NewGame(opts)

	// High balance: loan should be denied
	res1, err := game.RequestLoan(1, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1.Approved {
		t.Errorf("expected loan to be denied when balance is high")
	}

	// Reduce balance to R$ 50 (emergency)
	p1 := game.GetPlayer(1)
	p1.Balance = 50

	res2, err := game.RequestLoan(1, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.Approved {
		t.Fatalf("expected loan to be approved when balance is low")
	}
	if res2.Amount != 300 {
		t.Errorf("expected amount 300, got %d", res2.Amount)
	}
	if p1.Balance != 350 {
		t.Errorf("expected new balance 350, got %d", p1.Balance)
	}
}

func TestManager_Rooms_Flow(t *testing.T) {
	mgr := banco.NewManager()

	// 1. Create Room
	room, err := mgr.CreateRoom("HostUser", "Curitiba", "medio", 14, "9º Ano")
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	if len(room.Code) != 4 {
		t.Errorf("expected 4-letter room code, got %q", room.Code)
	}
	if len(room.Players) != 1 {
		t.Errorf("expected 1 host player, got %d", len(room.Players))
	}

	// 2. Join Room
	joinedPlayer, err := mgr.JoinRoom(room.Code, "AmigoConvidado", "🎩", "#1890ff")
	if err != nil {
		t.Fatalf("failed to join room: %v", err)
	}
	if joinedPlayer.Name != "AmigoConvidado" {
		t.Errorf("expected joined player name 'AmigoConvidado', got %q", joinedPlayer.Name)
	}

	updatedRoom, err := mgr.GetRoom(room.Code)
	if err != nil {
		t.Fatalf("failed to get room: %v", err)
	}
	if len(updatedRoom.Players) != 2 {
		t.Errorf("expected 2 players in room, got %d", len(updatedRoom.Players))
	}

	// 3. Start Room Game
	game, err := mgr.StartRoomGame(room.Code)
	if err != nil {
		t.Fatalf("failed to start room game: %v", err)
	}
	if game == nil {
		t.Fatalf("expected game instance, got nil")
	}

	state := game.GetState()
	if len(state.Players) < 2 {
		t.Errorf("expected at least 2 players in game, got %d", len(state.Players))
	}
	if state.Players[0].Name != "HostUser" {
		t.Errorf("expected player 0 to be HostUser, got %q", state.Players[0].Name)
	}
	if state.Players[1].Name != "AmigoConvidado" {
		t.Errorf("expected player 1 to be AmigoConvidado, got %q", state.Players[1].Name)
	}
}

func TestManager_SQLitePersistence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "banco_test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	mgr := banco.NewManager(database)

	opts := banco.GameOptions{
		City:       "Curitiba",
		BotsCount:  1,
		HumanName:  "TestPlayer",
	}
	game := mgr.CreateGame(opts)
	gameID := game.GetState().ID

	// Advance state: force position and buy property
	game.ForcePlayerPosition(1, 1)
	if err := game.BuyProperty(1, 1); err != nil {
		t.Fatalf("failed to buy property: %v", err)
	}
	_ = mgr.SaveGame(game)

	// Now create a fresh manager instance pointing to the same database
	freshMgr := banco.NewManager(database)
	loadedGame := freshMgr.GetGame(gameID)
	if loadedGame == nil {
		t.Fatalf("expected game %s to be loaded from database, got nil", gameID)
	}

	loadedState := loadedGame.GetState()
	if loadedState.City != "Curitiba" {
		t.Errorf("expected loaded city Curitiba, got %s", loadedState.City)
	}
	if loadedState.Board[1].OwnerID != 1 {
		t.Errorf("expected property 1 owner to be 1, got %d", loadedState.Board[1].OwnerID)
	}
}


