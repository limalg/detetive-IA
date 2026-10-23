package main

import (
	"os"
	"path/filepath"
	"testing"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/db"
)

func newAuth(t *testing.T) *auth.Service {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.RunMigrations(d); err != nil {
		t.Fatal(err)
	}
	return auth.New(d)
}

func TestEnsureAdminGeneratesPasswordFile(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("ADMIN_USER", "root")
	dir := t.TempDir()
	a := newAuth(t)
	ensureAdmin(a, dir)
	b, err := os.ReadFile(filepath.Join(dir, "admin_password.txt"))
	if err != nil || len(b) < 20 {
		t.Fatalf("password file missing/short: %v %q", err, b)
	}
	if _, err := a.Login("root", string(b[:len(b)-1])); err != nil {
		t.Fatalf("generated password does not log in: %v", err)
	}
}

func TestEnsureAdminUsesEnvPasswordAndWritesNoFile(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "from-env-pw")
	dir := t.TempDir()
	ensureAdmin(newAuth(t), dir)
	if _, err := os.Stat(filepath.Join(dir, "admin_password.txt")); err == nil {
		t.Fatal("no file expected when password comes from env")
	}
}
