package room_test

import (
	"path/filepath"
	"strings"
	"testing"

	"detetive-ia/internal/db"
	"detetive-ia/internal/room"
)

func setupTestManager(t *testing.T) *room.Manager {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_rooms.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	mgr, err := room.NewManager(database)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return mgr
}

func TestCreateRoom(t *testing.T) {
	mgr := setupTestManager(t)

	r, err := mgr.CreateRoom(101, "detective_marlowe", "Philip Marlowe", "Investigador Particular")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	if len(r.Code) != 4 {
		t.Fatalf("expected 4-character room code, got %q (len %d)", r.Code, len(r.Code))
	}

	// Verify unambiguous characters
	for _, forbidden := range []string{"I", "1", "O", "0", "Z", "2"} {
		if strings.Contains(r.Code, forbidden) {
			t.Errorf("room code %q contains forbidden ambiguous character %q", r.Code, forbidden)
		}
	}

	if r.Status != "lobby" {
		t.Errorf("expected room status 'lobby', got %q", r.Status)
	}

	if r.HostID != 101 {
		t.Errorf("expected host ID 101, got %d", r.HostID)
	}

	if len(r.Players) != 1 {
		t.Fatalf("expected 1 player (host), got %d", len(r.Players))
	}

	host := r.Players[0]
	if !host.IsHost {
		t.Error("expected first player to have IsHost = true")
	}
	if host.Username != "detective_marlowe" {
		t.Errorf("expected username detective_marlowe, got %s", host.Username)
	}
	if host.PersonaName != "Philip Marlowe" {
		t.Errorf("expected persona Philip Marlowe, got %s", host.PersonaName)
	}
}

func TestGetRoom(t *testing.T) {
	mgr := setupTestManager(t)

	created, err := mgr.CreateRoom(201, "sherlock", "Sherlock Holmes", "Consultor Forense")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Test uppercase and lowercase lookup
	found, err := mgr.GetRoom(created.Code)
	if err != nil {
		t.Fatalf("GetRoom with exact code failed: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("expected room ID %d, got %d", created.ID, found.ID)
	}

	foundLower, err := mgr.GetRoom(strings.ToLower(created.Code))
	if err != nil {
		t.Fatalf("GetRoom with lowercase code failed: %v", err)
	}
	if foundLower.ID != created.ID {
		t.Errorf("expected room ID %d for lowercase lookup, got %d", created.ID, foundLower.ID)
	}

	// Test non-existent room
	_, err = mgr.GetRoom("XXXX")
	if err == nil {
		t.Fatal("expected error for non-existent room code, got nil")
	}
}

func TestJoinRoom(t *testing.T) {
	mgr := setupTestManager(t)

	r, err := mgr.CreateRoom(301, "host_player", "Inspetor Clouseau", "Detetive Chefe")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	updated, err := mgr.JoinRoom(r.Code, 302, "player_two", "Dra. Watson", "Perita Médica")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	if len(updated.Players) != 2 {
		t.Fatalf("expected 2 players in room, got %d", len(updated.Players))
	}

	secondPlayer := updated.Players[1]
	if secondPlayer.UserID != 302 {
		t.Errorf("expected user ID 302, got %d", secondPlayer.UserID)
	}
	if secondPlayer.IsHost {
		t.Error("expected second player to have IsHost = false")
	}
	if secondPlayer.IsBot {
		t.Error("expected real player to have IsBot = false")
	}
}

func TestAddBot(t *testing.T) {
	mgr := setupTestManager(t)

	r, err := mgr.CreateRoom(401, "solo_player", "Hercule Poirot", "Detetive Belga")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	updated, err := mgr.AddBot(r.Code, "Barnaby Finch", "Mordomo Silencioso", "Guarda cartas confidenciais")
	if err != nil {
		t.Fatalf("AddBot failed: %v", err)
	}

	if len(updated.Players) != 2 {
		t.Fatalf("expected 2 players after adding bot, got %d", len(updated.Players))
	}

	bot := updated.Players[1]
	if !bot.IsBot {
		t.Error("expected IsBot = true for added bot")
	}
	if bot.UserID != 0 {
		t.Errorf("expected bot user ID to be 0, got %d", bot.UserID)
	}
	if bot.PersonaName != "Barnaby Finch" {
		t.Errorf("expected persona Barnaby Finch, got %s", bot.PersonaName)
	}
}

func TestStartRoomAndLocking(t *testing.T) {
	mgr := setupTestManager(t)

	r, err := mgr.CreateRoom(501, "host_agent", "Sam Spade", "Investigador")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Non-host cannot start room
	_, err = mgr.StartRoom(r.Code, 999)
	if err == nil {
		t.Fatal("expected error when non-host tries to start room, got nil")
	}

	// Host starts room
	started, err := mgr.StartRoom(r.Code, 501)
	if err != nil {
		t.Fatalf("StartRoom by host failed: %v", err)
	}

	if started.Status != "active" {
		t.Errorf("expected status 'active', got %q", started.Status)
	}

	// Once active, joining should be blocked
	_, err = mgr.JoinRoom(r.Code, 502, "late_joiner", "Late Persona", "Repórter")
	if err == nil {
		t.Fatal("expected error joining active room, got nil")
	}
}

func TestCodeGenerationUniquenessAndFormat(t *testing.T) {
	mgr := setupTestManager(t)

	seenCodes := make(map[string]bool)
	forbiddenChars := "I1O0Z2"

	for i := 0; i < 40; i++ {
		r, err := mgr.CreateRoom(int64(1000+i), "player", "Persona", "Role")
		if err != nil {
			t.Fatalf("iteration %d: CreateRoom failed: %v", i, err)
		}

		if len(r.Code) != 4 {
			t.Fatalf("expected code length 4, got %q", r.Code)
		}

		for _, char := range r.Code {
			if strings.ContainsRune(forbiddenChars, char) {
				t.Fatalf("code %q contains forbidden char %c", r.Code, char)
			}
		}

		if seenCodes[r.Code] {
			t.Fatalf("duplicate room code generated: %q", r.Code)
		}
		seenCodes[r.Code] = true
	}
}
