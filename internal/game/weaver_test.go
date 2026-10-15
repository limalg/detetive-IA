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

func TestLayeredSecretsAndSocialWeb(t *testing.T) {
	service, weaver := setupWeaverTest(t)

	players := []room.Player{
		{ID: 1, UserID: 101, Username: "det1", PersonaName: "Philip Marlowe", Role: "Investigador"},
		{ID: 2, UserID: 102, Username: "det2", PersonaName: "Helena Voss", Role: "Herdeira"},
	}

	result, err := weaver.WeaveCase(players)
	if err != nil {
		t.Fatalf("WeaveCase failed: %v", err)
	}

	details, err := service.GetCaseDetails(result.CaseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	for _, s := range details.Suspects {
		if strings.TrimSpace(s.Tier1Alibi) == "" {
			t.Errorf("suspect %s missing Tier1Alibi", s.Name)
		}
		if strings.TrimSpace(s.Tier2PettyCrime) == "" {
			t.Errorf("suspect %s missing Tier2PettyCrime", s.Name)
		}
		if strings.TrimSpace(s.Tier3Truth) == "" {
			t.Errorf("suspect %s missing Tier3Truth", s.Name)
		}
		if strings.TrimSpace(s.CrossSuspicion) == "" {
			t.Errorf("suspect %s missing CrossSuspicion", s.Name)
		}

		// At least one cross-suspicion must mention another suspect's name
		hasMention := false
		for _, other := range details.Suspects {
			if other.ID != s.ID && strings.Contains(s.CrossSuspicion, strings.Fields(other.Name)[0]) {
				hasMention = true
				break
			}
		}
		if !hasMention {
			t.Errorf("suspect %s cross-suspicion %q does not mention any other suspect", s.Name, s.CrossSuspicion)
		}
	}

	// Test stress-based fallback interrogation responses:
	// <30% stress: Tier 1 (Alibi)
	// 30%-70% stress: Tier 2 (Petty Crime)
	// >70% stress: Tier 3 (Truth/Breakdown)
	culprit := details.Suspects[0]
	for _, s := range details.Suspects {
		if s.IsCulprit {
			culprit = s
			break
		}
	}

	respTier1 := game.GenerateFallbackResponseWithStress(&culprit, "O que aconteceu?", 20)
	if !strings.Contains(respTier1, culprit.Tier1Alibi) && !strings.Contains(respTier1, culprit.Alibi) {
		t.Errorf("expected Tier 1 alibi in low stress response (<30%%), got: %s", respTier1)
	}

	respTier2 := game.GenerateFallbackResponseWithStress(&culprit, "O que você tem a esconder?", 50)
	if !strings.Contains(respTier2, culprit.Tier2PettyCrime) {
		t.Errorf("expected Tier 2 petty crime in medium stress response (30-70%%), got: %s", respTier2)
	}

	respTier3 := game.GenerateFallbackResponseWithStress(&culprit, "Confesse!", 85)
	if !strings.Contains(respTier3, culprit.Tier3Truth) {
		t.Errorf("expected Tier 3 truth in high stress response (>70%%), got: %s", respTier3)
	}
}

func TestWeaverMultiThemeAndSuspectCounts(t *testing.T) {
	service, weaver := setupWeaverTest(t)

	themes := []struct {
		id             string
		expectedTitle  string
		expectedVictim string
	}{
		{
			id:             "mansion",
			expectedTitle:  "O Mistério da Mansão do Vale das Sombras",
			expectedVictim: "Sir Arthur Pendelton (Magnata das Ferrovias e Colecionador de Arte)",
		},
		{
			id:             "train",
			expectedTitle:  "O Mistério do Expresso da Meia-Noite",
			expectedVictim: "Viktor Orlov (Diplomata e Colecionador de Segredos)",
		},
		{
			id:             "opera",
			expectedTitle:  "A Última Ária no Teatro Municipal",
			expectedVictim: "Gabriel Fontaine (Maestro Principal e Tenor de Renome)",
		},
		{
			id:             "casino",
			expectedTitle:  "A Dívida do Cassino Clandestino",
			expectedVictim: "Emilio Ramos (Dono de Bancas Ilegais e Agiota do Submundo)",
		},
	}

	suspectCounts := []int{3, 4, 5, 6}

	for _, th := range themes {
		for _, count := range suspectCounts {
			testName := th.id + "_" + string(rune('0'+count))
			t.Run(testName, func(t *testing.T) {
				result, err := weaver.WeaveCaseWithTheme(nil, th.id, count)
				if err != nil {
					t.Fatalf("WeaveCaseWithTheme(%q, %d) failed: %v", th.id, count, err)
				}

				if result.Case.Title != th.expectedTitle {
					t.Errorf("expected title %q, got %q", th.expectedTitle, result.Case.Title)
				}
				if result.Case.Victim != th.expectedVictim {
					t.Errorf("expected victim %q, got %q", th.expectedVictim, result.Case.Victim)
				}

				if len(result.Suspects) != count {
					t.Fatalf("expected exactly %d suspects, got %d", count, len(result.Suspects))
				}

				culpritCount := 0
				for _, s := range result.Suspects {
					if s.IsCulprit {
						culpritCount++
					}
				}
				if culpritCount != 1 {
					t.Fatalf("expected exactly 1 culprit, got %d", culpritCount)
				}

				if len(result.Clues) < 3 {
					t.Fatalf("expected at least 3 clues under Three-Clue Rule, got %d", len(result.Clues))
				}

				// Check weapon validity (one of the 5 weapons)
				validWeapons := map[string]bool{
					"Adaga Cerimonial Veneziana":    true,
					"Cálice com Resíduo de Cianeto": true,
					"Revólver .38 com Silenciador":  true,
					"Castiçal de Bronze Maciço":     true,
					"Navalha Pericial de Aço":       true,
				}
				if !validWeapons[result.Case.SolutionWeapon] {
					t.Errorf("unexpected weapon generated: %q", result.Case.SolutionWeapon)
				}

				// Verify deduction graph with SubmitAccusation
				verdict, err := service.SubmitAccusation(1, result.CaseID, result.Culprit.ID, result.Case.SolutionWeapon, result.Case.SolutionMotive)
				if err != nil {
					t.Fatalf("SubmitAccusation failed: %v", err)
				}
				if !verdict.Correct {
					t.Errorf("expected accusation to be correct for woven case, got feedback: %s", verdict.Feedback)
				}
			})
		}
	}
}

func TestWeaverSuspectCountClamping(t *testing.T) {
	_, weaver := setupWeaverTest(t)

	// count < 3 should clamp to 3
	resSmall, err := weaver.WeaveCaseWithTheme(nil, "mansion", 1)
	if err != nil {
		t.Fatalf("small count failed: %v", err)
	}
	if len(resSmall.Suspects) != 3 {
		t.Errorf("expected 3 suspects for count 1, got %d", len(resSmall.Suspects))
	}

	// count > 6 should clamp to 6
	resLarge, err := weaver.WeaveCaseWithTheme(nil, "mansion", 10)
	if err != nil {
		t.Fatalf("large count failed: %v", err)
	}
	if len(resLarge.Suspects) != 6 {
		t.Errorf("expected 6 suspects for count 10, got %d", len(resLarge.Suspects))
	}

	// count 0 should default to 4
	resDefault, err := weaver.WeaveCaseWithTheme(nil, "mansion", 0)
	if err != nil {
		t.Fatalf("zero count failed: %v", err)
	}
	if len(resDefault.Suspects) != 4 {
		t.Errorf("expected 4 suspects for default count 0, got %d", len(resDefault.Suspects))
	}
}

func TestStartSoloGame_MultiTheme(t *testing.T) {
	service, _ := setupWeaverTest(t)

	details, inv, err := service.StartSoloGame(0, "train", 5)
	if err != nil {
		t.Fatalf("StartSoloGame failed: %v", err)
	}

	if details.Case.Title != "O Mistério do Expresso da Meia-Noite" {
		t.Errorf("expected train title, got %q", details.Case.Title)
	}
	if len(details.Suspects) != 5 {
		t.Errorf("expected 5 suspects, got %d", len(details.Suspects))
	}
	if inv.Status != "in_progress" {
		t.Errorf("expected in_progress status, got %s", inv.Status)
	}
}

func TestSubmitAccusation_AllFiveWeapons(t *testing.T) {
	service, weaver := setupWeaverTest(t)

	weaponsSeen := make(map[string]bool)

	// Run multiple iterations to exercise all 5 weapons procedurally
	for i := 0; i < 40; i++ {
		result, err := weaver.WeaveCaseWithTheme(nil, "mansion", 4)
		if err != nil {
			t.Fatalf("WeaveCase failed: %v", err)
		}

		weaponsSeen[result.Case.SolutionWeapon] = true

		// Exact weapon name accusation
		v1, err := service.SubmitAccusation(1, result.CaseID, result.Culprit.ID, result.Case.SolutionWeapon, result.Case.SolutionMotive)
		if err != nil {
			t.Fatalf("SubmitAccusation exact failed: %v", err)
		}
		if !v1.Correct {
			t.Errorf("expected exact weapon %q to be correct, feedback: %s", result.Case.SolutionWeapon, v1.Feedback)
		}

		// Accusation with keyword (e.g. "navalha" if Navalha Pericial de Aço)
		var keyword string
		switch {
		case strings.Contains(result.Case.SolutionWeapon, "Navalha"):
			keyword = "navalha"
		case strings.Contains(result.Case.SolutionWeapon, "Adaga"):
			keyword = "adaga"
		case strings.Contains(result.Case.SolutionWeapon, "Cálice"):
			keyword = "cianeto"
		case strings.Contains(result.Case.SolutionWeapon, "Revólver"):
			keyword = "revólver"
		case strings.Contains(result.Case.SolutionWeapon, "Castiçal"):
			keyword = "castiçal"
		}

		v2, err := service.SubmitAccusation(1, result.CaseID, result.Culprit.ID, keyword, result.Case.SolutionMotive)
		if err != nil {
			t.Fatalf("SubmitAccusation keyword failed: %v", err)
		}
		if !v2.Correct {
			t.Errorf("expected keyword %q for weapon %q to be correct, feedback: %s", keyword, result.Case.SolutionWeapon, v2.Feedback)
		}

		// Wrong weapon should fail
		vWrong, err := service.SubmitAccusation(1, result.CaseID, result.Culprit.ID, "Espada Medieval Fantasma", result.Case.SolutionMotive)
		if err != nil {
			t.Fatalf("SubmitAccusation wrong weapon failed: %v", err)
		}
		if vWrong.Correct {
			t.Errorf("expected wrong weapon to be incorrect")
		}
	}

	if len(weaponsSeen) < 3 {
		t.Logf("tested with %d distinct weapons out of 5", len(weaponsSeen))
	}
}

