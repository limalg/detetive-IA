package auth_test

import (
	"path/filepath"
	"testing"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
)

func setupTestDB(t *testing.T) *auth.Service {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_auth.db")

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

	return auth.New(database)
}

func TestRegister_Success(t *testing.T) {
	service := setupTestDB(t)

	user, err := service.Register("detective_noir", "secret123")
	if err != nil {
		t.Fatalf("expected successful registration, got: %v", err)
	}

	if user.ID <= 0 {
		t.Errorf("expected positive user ID, got %d", user.ID)
	}
	if user.Username != "detective_noir" {
		t.Errorf("expected username detective_noir, got %s", user.Username)
	}
}

func TestRegister_DuplicateUser(t *testing.T) {
	service := setupTestDB(t)

	_, err := service.Register("duplicate_agent", "pass1")
	if err != nil {
		t.Fatalf("initial registration failed: %v", err)
	}

	_, err = service.Register("duplicate_agent", "pass2")
	if err == nil {
		t.Fatal("expected error registering duplicate username, got nil")
	}
}

func TestRegister_InvalidInput(t *testing.T) {
	service := setupTestDB(t)

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{"empty username", "", "validpass123"},
		{"empty password", "validuser", ""},
		{"short username", "ab", "validpass123"},
		{"short password", "validuser", "12"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Register(tc.username, tc.password)
			if err == nil {
				t.Errorf("expected error for case %q, got nil", tc.name)
			}
		})
	}
}

func TestLogin_Success(t *testing.T) {
	service := setupTestDB(t)

	_, err := service.Register("sherlock", "elementary")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	token, err := service.Login("sherlock", "elementary")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty session token")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	service := setupTestDB(t)

	_, err := service.Register("watson", "doctor123")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Wrong password
	_, err = service.Login("watson", "wrongpass")
	if err == nil {
		t.Fatal("expected error with wrong password, got nil")
	}

	// Unknown user
	_, err = service.Login("moriarty", "doctor123")
	if err == nil {
		t.Fatal("expected error with unknown user, got nil")
	}
}

func TestValidateSession(t *testing.T) {
	service := setupTestDB(t)

	_, err := service.Register("poirot", "mustache")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	token, err := service.Login("poirot", "mustache")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	user, err := service.ValidateSession(token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}

	if user.Username != "poirot" {
		t.Errorf("expected user poirot, got %s", user.Username)
	}

	// Invalid token
	_, err = service.ValidateSession("invalid-token-xyz")
	if err == nil {
		t.Fatal("expected error for invalid session token, got nil")
	}
}

func TestLogout(t *testing.T) {
	service := setupTestDB(t)

	_, err := service.Register("marple", "knitting")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	token, err := service.Login("marple", "knitting")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if err := service.Logout(token); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Token should now be invalid
	_, err = service.ValidateSession(token)
	if err == nil {
		t.Fatal("expected error validating session after logout, got nil")
	}
}
