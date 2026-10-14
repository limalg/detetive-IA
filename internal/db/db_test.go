package db_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"detetive-ia/internal/db"
)

func TestInitDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestRunMigrations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_migrations.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	expectedTables := []string{
		"users",
		"sessions",
		"cases",
		"suspects",
		"clues",
		"interrogation_messages",
		"investigations",
	}

	for _, table := range expectedTables {
		var name string
		query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
		err := database.QueryRow(query, table).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("expected table %q to exist in database, but it was not found", table)
		} else if err != nil {
			t.Fatalf("error checking table %q: %v", table, err)
		}
	}

	// Test idempotency: running migrations again should not fail
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations idempotency check failed: %v", err)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_fk.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	var fkEnabled int
	if err := database.QueryRow("PRAGMA foreign_keys;").Scan(&fkEnabled); err != nil {
		t.Fatalf("failed to query PRAGMA foreign_keys: %v", err)
	}

	if fkEnabled != 1 {
		t.Errorf("expected foreign_keys to be 1, got %d", fkEnabled)
	}
}
