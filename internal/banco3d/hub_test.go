package banco3d

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHubSaveAndLoadFromDisk(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_rooms.json")

	// 1. Initialize hub with custom storage path
	hub1 := NewRoomHub(storagePath)

	cfg := DefaultRoomConfig()
	cfg.TurnSeconds = 15
	room, err := hub1.CreateRoom("room-f5-test", "Londrina", &cfg)
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Add a player and start game
	p1, err := room.JoinPlayer("Eduardo", "Vermelho", "carro")
	if err != nil {
		t.Fatalf("failed to join player: %v", err)
	}
	p2, err := room.JoinPlayer("Bot IA", "Azul", "barco")
	if err != nil {
		t.Fatalf("failed to join bot: %v", err)
	}
	p2.IsBot = true

	if err := room.StartGame(); err != nil {
		t.Fatalf("failed to start game: %v", err)
	}

	// Set pending decision
	deadline := time.Now().Add(8 * time.Second)
	room.PendingDecision = "buy"
	room.DecisionDeadline = &deadline
	room.DecisionPlayerID = p1.ID

	// Record an event
	diceVals := [2]int{3, 4}
	room.RecordEvent(GameEvent{
		Seq:        1,
		Type:       "roll",
		PlayerID:   p1.ID,
		PlayerName: p1.Name,
		Dice:       &diceVals,
		From:       0,
		To:         7,
		Text:       "Eduardo tirou 7",
	})

	// Explicitly save to disk
	if err := hub1.SaveToDisk(); err != nil {
		t.Fatalf("failed to save hub to disk: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Fatalf("snapshot file was not created: %s", storagePath)
	}

	// 2. Simulate server restart: create new hub from same disk path
	hub2 := NewRoomHub(storagePath)

	recoveredRoom, ok := hub2.GetRoom("room-f5-test")
	if !ok {
		t.Fatalf("failed to recover room 'room-f5-test' after reload")
	}

	if recoveredRoom.City != "Londrina" {
		t.Errorf("expected city 'Londrina', got '%s'", recoveredRoom.City)
	}

	if len(recoveredRoom.Game.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(recoveredRoom.Game.Players))
	}

	if recoveredRoom.Game.Players[0].Name != "Eduardo" {
		t.Errorf("expected first player name 'Eduardo', got '%s'", recoveredRoom.Game.Players[0].Name)
	}

	if !recoveredRoom.Game.Players[1].IsBot {
		t.Errorf("expected second player to be marked as bot")
	}

	if recoveredRoom.PendingDecision != "buy" {
		t.Errorf("expected pending_decision 'buy', got '%s'", recoveredRoom.PendingDecision)
	}

	if recoveredRoom.DecisionDeadline == nil {
		t.Errorf("expected non-nil decision deadline")
	}

	if len(recoveredRoom.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(recoveredRoom.Events))
	} else if recoveredRoom.Events[0].Type != "roll" {
		t.Errorf("expected event type 'roll', got '%s'", recoveredRoom.Events[0].Type)
	}

	// 3. Remove room and verify snapshot updates
	hub2.RemoveRoom("room-f5-test")
	_ = hub2.SaveToDisk()

	hub3 := NewRoomHub(storagePath)
	if _, ok := hub3.GetRoom("room-f5-test"); ok {
		t.Errorf("room should have been removed from persisted state")
	}
}
