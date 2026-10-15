package game_test

import (
	"path/filepath"
	"strings"
	"testing"

	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
	"detetive-ia/internal/room"
)

func setupWeaverTest(t *testing.T) (*game.Service, *game.Weaver) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_weaver.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	gameService := game.New(database, nil)
	weaver := game.NewWeaver(database)

	return gameService, weaver
}

func TestWeaver_BotFillWhenUnderFourPlayers(t *testing.T) {
	_, weaver := setupWeaverTest(t)

	// Single player joining lobby
	soloPlayers := []room.Player{
		{
			ID:          1,
			UserID:      100,
			Username:    "solo_detective",
			PersonaName: "Sam Spade",
			Role:        "Detetive Cínico",
			IsHost:      true,
		},
	}

	result, err := weaver.WeaveCase(soloPlayers)
	if err != nil {
		t.Fatalf("WeaveCase failed: %v", err)
	}

	if len(result.Suspects) < 4 {
		t.Fatalf("expected at least 4 suspects (filled with bots), got %d", len(result.Suspects))
	}

	// Verify exactly one culprit is chosen
	culpritCount := 0
	var culprit *game.WeaverSuspect
	for i := range result.Suspects {
		s := &result.Suspects[i]
		if s.IsCulprit {
			culpritCount++
			culprit = s
		}
	}

	if culpritCount != 1 {
		t.Fatalf("expected exactly 1 culprit, got %d", culpritCount)
	}

	if culprit == nil || culprit.ID == 0 {
		t.Fatal("culprit must have a valid non-zero ID")
	}
}

func TestWeaver_ThreeClueRule(t *testing.T) {
	_, weaver := setupWeaverTest(t)

	players := []room.Player{
		{ID: 1, UserID: 10, Username: "player1", PersonaName: "Victoria Sterling", Role: "Femme Fatale"},
		{ID: 2, UserID: 20, Username: "player2", PersonaName: "Barão Klaus", Role: "Magnata"},
		{ID: 3, UserID: 30, Username: "player3", PersonaName: "Edgar Vance", Role: "Jornalista"},
		{ID: 4, UserID: 40, Username: "player4", PersonaName: "Dra. Nicole", Role: "Médica"},
	}

	result, err := weaver.WeaveCase(players)
	if err != nil {
		t.Fatalf("WeaveCase failed: %v", err)
	}

	// Must have at least 3 independent clues (Physical, Alibi Contradiction, Documentary)
	if len(result.Clues) < 3 {
		t.Fatalf("expected at least 3 clues under Three-Clue Rule, got %d", len(result.Clues))
	}

	hasPhysical := false
	hasContradiction := false
	hasDocumentary := false

	weaponNameLower := strings.ToLower(result.Case.SolutionWeapon)

	for _, clue := range result.Clues {
		descLower := strings.ToLower(clue.Description + " " + clue.Name)

		// 1. Physical clue mentions weapon or physical residue
		if strings.Contains(descLower, weaponNameLower) || strings.Contains(descLower, "adaga") || strings.Contains(descLower, "veneno") || strings.Contains(descLower, "cálice") || strings.Contains(descLower, "revólver") {
			hasPhysical = true
		}

		// 2. Contradiction clue exposes culprit's alibi flaw
		if strings.Contains(descLower, "álibi") || strings.Contains(descLower, "contradição") || strings.Contains(descLower, "registro") || strings.Contains(descLower, "falso") || strings.Contains(descLower, "câmera") || strings.Contains(descLower, "testemunha") {
			hasContradiction = true
		}

		// 3. Documentary clue reveals motive or secret
		if strings.Contains(descLower, "testamento") || strings.Contains(descLower, "dívida") || strings.Contains(descLower, "carta") || strings.Contains(descLower, "documento") || strings.Contains(descLower, "extrato") {
			hasDocumentary = true
		}
	}

	if !hasPhysical {
		t.Error("expected at least one physical clue connecting to weapon")
	}
	if !hasContradiction {
		t.Error("expected at least one alibi contradiction clue")
	}
	if !hasDocumentary {
		t.Error("expected at least one documentary clue exposing motive")
	}
}

func TestWeaver_DatabasePersistenceAndAccusation(t *testing.T) {
	service, weaver := setupWeaverTest(t)

	players := []room.Player{
		{ID: 1, UserID: 101, Username: "det1", PersonaName: "Philip Marlowe", Role: "Investigador"},
		{ID: 2, UserID: 102, Username: "det2", PersonaName: "Helena Voss", Role: "Herdeira"},
	}

	result, err := weaver.WeaveCase(players)
	if err != nil {
		t.Fatalf("WeaveCase failed: %v", err)
	}

	// Verify case is queryable via GetCaseDetails
	details, err := service.GetCaseDetails(result.CaseID)
	if err != nil {
		t.Fatalf("GetCaseDetails for woven case %d failed: %v", result.CaseID, err)
	}

	if details.Case.Title == "" {
		t.Error("expected non-empty case title")
	}
	if len(details.Suspects) < 4 {
		t.Errorf("expected at least 4 persisted suspects, got %d", len(details.Suspects))
	}
	if len(details.Clues) < 3 {
		t.Errorf("expected at least 3 persisted clues, got %d", len(details.Clues))
	}

	// Test SubmitAccusation with correct culprit, weapon and motive
	verdict, err := service.SubmitAccusation(101, result.CaseID, result.Culprit.ID, result.Case.SolutionWeapon, result.Case.SolutionMotive)
	if err != nil {
		t.Fatalf("SubmitAccusation failed: %v", err)
	}

	if !verdict.Correct {
		t.Errorf("expected accusation to be correct for generated case. Feedback: %s", verdict.Feedback)
	}
	if verdict.Score != 100 {
		t.Errorf("expected score 100, got %d", verdict.Score)
	}
}
