package game_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
)

type mockAIClient struct {
	response string
}

func (m *mockAIClient) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func setupTestGame(t *testing.T) (*game.Service, int64, int64) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_game.db")

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

	caseID, err := gameService.SeedInitialCase()
	if err != nil {
		t.Fatalf("SeedInitialCase failed: %v", err)
	}

	authService := auth.New(database)
	user, err := authService.Register("test_detective", "password123")
	if err != nil {
		t.Fatalf("Register test user failed: %v", err)
	}

	return gameService, caseID, user.ID
}

func TestSeedAndGetCaseDetails(t *testing.T) {
	service, caseID, _ := setupTestGame(t)

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	if details.Case.Title != "O Enigma do Vale das Sombras" {
		t.Errorf("expected title 'O Enigma do Vale das Sombras', got %q", details.Case.Title)
	}

	if len(details.Suspects) != 3 {
		t.Fatalf("expected 3 suspects, got %d", len(details.Suspects))
	}

	if len(details.Clues) < 4 {
		t.Errorf("expected at least 4 clues, got %d", len(details.Clues))
	}

	// Idempotency: seeding again should not create duplicate cases
	secondCaseID, err := service.SeedInitialCase()
	if err != nil {
		t.Fatalf("second SeedInitialCase failed: %v", err)
	}
	if secondCaseID != caseID {
		t.Errorf("expected same case ID %d on re-seed, got %d", caseID, secondCaseID)
	}
}

func TestInterrogate_Fallback(t *testing.T) {
	service, caseID, _ := setupTestGame(t)

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	var julianID int64
	for _, s := range details.Suspects {
		if strings.Contains(s.Name, "Julian") {
			julianID = s.ID
			break
		}
	}
	if julianID == 0 {
		t.Fatal("Julian Vane not found among suspects")
	}

	// Test question about alibi
	answer, err := service.Interrogate("session-123", julianID, "Onde você estava na noite do assassinato?")
	if err != nil {
		t.Fatalf("Interrogate failed: %v", err)
	}
	if len(answer) == 0 {
		t.Fatal("expected non-empty interrogation answer")
	}

	// Test question about secret/money
	answerMotive, err := service.Interrogate("session-123", julianID, "O que você tem a dizer sobre as suas dívidas e o testamento?")
	if err != nil {
		t.Fatalf("Interrogate failed: %v", err)
	}
	if len(answerMotive) == 0 {
		t.Fatal("expected non-empty interrogation answer about motive")
	}
}

func TestInterrogate_MockAI(t *testing.T) {
	service, caseID, _ := setupTestGame(t)

	mockResp := "Detetive, você não tem provas contra mim! Meu álibi é inquestionável."
	service.SetAIClient(&mockAIClient{response: mockResp})

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	answer, err := service.Interrogate("session-ai", details.Suspects[0].ID, "Conte-me a verdade!")
	if err != nil {
		t.Fatalf("Interrogate with AI failed: %v", err)
	}

	if answer != mockResp {
		t.Errorf("expected %q, got %q", mockResp, answer)
	}
}

func TestSubmitAccusation_Correct(t *testing.T) {
	service, caseID, userID := setupTestGame(t)

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	var culpritID int64
	for _, s := range details.Suspects {
		if strings.Contains(s.Name, "Julian") {
			culpritID = s.ID
			break
		}
	}

	verdict, err := service.SubmitAccusation(userID, caseID, culpritID, "Adaga Cerimonial Veneziana", "Herança e dívidas de jogo")
	if err != nil {
		t.Fatalf("SubmitAccusation failed: %v", err)
	}

	if !verdict.Correct {
		t.Errorf("expected accusation to be correct, got incorrect. Feedback: %s", verdict.Feedback)
	}

	if verdict.Score != 100 {
		t.Errorf("expected score 100, got %d", verdict.Score)
	}
}

func TestSubmitAccusation_WrongSuspect(t *testing.T) {
	service, caseID, userID := setupTestGame(t)

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	var innocentID int64
	for _, s := range details.Suspects {
		if strings.Contains(s.Name, "Elena") {
			innocentID = s.ID
			break
		}
	}

	verdict, err := service.SubmitAccusation(userID, caseID, innocentID, "Adaga Cerimonial Veneziana", "Herança")
	if err != nil {
		t.Fatalf("SubmitAccusation failed: %v", err)
	}

	if verdict.Correct {
		t.Error("expected accusation to be incorrect for innocent suspect")
	}

	if verdict.Score > 50 {
		t.Errorf("expected low score for wrong suspect, got %d", verdict.Score)
	}
}

func TestSubmitAccusation_WrongWeapon(t *testing.T) {
	service, caseID, userID := setupTestGame(t)

	details, err := service.GetCaseDetails(caseID)
	if err != nil {
		t.Fatalf("GetCaseDetails failed: %v", err)
	}

	var culpritID int64
	for _, s := range details.Suspects {
		if strings.Contains(s.Name, "Julian") {
			culpritID = s.ID
			break
		}
	}

	verdict, err := service.SubmitAccusation(userID, caseID, culpritID, "Veneno de Arsênico", "Dívidas")
	if err != nil {
		t.Fatalf("SubmitAccusation failed: %v", err)
	}

	if verdict.Correct {
		t.Error("expected accusation to fail due to wrong weapon")
	}
}

func TestInvestigationTimeAndWarrants(t *testing.T) {
	service, caseID, userID := setupTestGame(t)

	// 1. New investigation initializes with 420 min and 2 warrants
	inv, err := service.GetOrCreateInvestigation(userID, caseID)
	if err != nil {
		t.Fatalf("GetOrCreateInvestigation failed: %v", err)
	}
	if inv.TimeRemainingMinutes != 420 {
		t.Errorf("expected 420 minutes initially, got %d", inv.TimeRemainingMinutes)
	}
	if inv.WarrantsRemaining != 2 {
		t.Errorf("expected 2 warrants initially, got %d", inv.WarrantsRemaining)
	}
	if inv.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", inv.Status)
	}

	// 2. Deduct time
	remaining, err := service.DeductTime(inv.ID, 15)
	if err != nil {
		t.Fatalf("DeductTime failed: %v", err)
	}
	if remaining != 405 {
		t.Errorf("expected 405 minutes remaining, got %d", remaining)
	}

	// 3. Warrant strikes
	warrantsLeft, isGameOver, err := service.RecordWarrantStrike(inv.ID)
	if err != nil {
		t.Fatalf("RecordWarrantStrike strike 1 failed: %v", err)
	}
	if warrantsLeft != 1 || isGameOver {
		t.Errorf("expected 1 warrant remaining and isGameOver=false, got warrants=%d, isGameOver=%v", warrantsLeft, isGameOver)
	}

	// Strike 2 causes game over and marks as FAILED
	warrantsLeft, isGameOver, err = service.RecordWarrantStrike(inv.ID)
	if err != nil {
		t.Fatalf("RecordWarrantStrike strike 2 failed: %v", err)
	}
	if warrantsLeft != 0 || !isGameOver {
		t.Errorf("expected 0 warrants remaining and isGameOver=true, got warrants=%d, isGameOver=%v", warrantsLeft, isGameOver)
	}

	updatedInv, err := service.GetInvestigation(inv.ID)
	if err != nil {
		t.Fatalf("GetInvestigation failed: %v", err)
	}
	if updatedInv.Status != "failed" {
		t.Errorf("expected status 'failed' after 2 strikes, got %q", updatedInv.Status)
	}

	// 4. Time exhaustion causes game over and marks as FAILED (guest investigation with userID=0)
	inv2, err := service.GetOrCreateInvestigation(0, caseID)
	if err != nil {
		t.Fatalf("guest GetOrCreateInvestigation failed: %v", err)
	}
	remaining, err = service.DeductTime(inv2.ID, 420)
	if err != nil {
		t.Fatalf("DeductTime 420 failed: %v", err)
	}
	if remaining != 0 {
		t.Errorf("expected 0 minutes remaining, got %d", remaining)
	}
	updatedInv2, err := service.GetInvestigation(inv2.ID)
	if err != nil {
		t.Fatalf("GetInvestigation failed: %v", err)
	}
	if updatedInv2.Status != "failed" {
		t.Errorf("expected status 'failed' after time exhaustion, got %q", updatedInv2.Status)
	}
}

