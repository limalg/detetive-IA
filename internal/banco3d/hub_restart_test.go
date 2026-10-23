package banco3d

import (
	"os"
	"path/filepath"
	"testing"
)

// Sala viva -> "rebuild" (novo hub lendo o mesmo arquivo) -> sala reaberta com o jogo intacto.
func TestHubRoomSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.json")
	h1 := NewRoomHub(path)
	r, err := h1.CreateRoom("viva", "Londrina")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.JoinPlayer("Ana", "#fff", "car"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.JoinPlayer("Bia", "#000", "hat"); err != nil {
		t.Fatal(err)
	}
	if err := r.StartGame(); err != nil {
		t.Fatal(err)
	}
	if err := h1.SaveToDisk(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal("temp file left behind: write must be atomic (tmp + rename)")
	}

	h2 := NewRoomHub(path)
	r2, ok := h2.GetRoom("viva")
	if !ok || r2.Game == nil || !r2.Game.IsStarted || len(r2.Game.Players) < 2 {
		t.Fatalf("room not restored intact: ok=%v", ok)
	}
}

// Um arquivo principal truncado (kill no meio da escrita) não pode apagar as salas: usa o backup.
func TestHubFallsBackWhenSnapshotCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.json")
	h1 := NewRoomHub(path)
	if _, err := h1.CreateRoom("backup", "Londrina"); err != nil {
		t.Fatal(err)
	}
	_ = h1.SaveToDisk() // 2º save rotaciona o anterior para .bak
	if err := os.WriteFile(path, []byte(`{"backup": {"id":`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := NewRoomHub(path).GetRoom("backup"); !ok {
		t.Fatal("room lost after corrupt snapshot")
	}
}
