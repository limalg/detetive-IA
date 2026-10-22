package banco3d

import (
	"testing"
	"time"
)

func TestRoomLifecycleAndEvents(t *testing.T) {
	hub := NewRoomHub()
	room, err := hub.CreateRoom("SP-123", "São Paulo")
	if err != nil || room == nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}
	if room.City != "São Paulo" {
		t.Errorf("cidade incorreta: %s", room.City)
	}

	// 1. Jogadores entram na sala
	p1, err := room.JoinPlayer("Lucas", "#3b82f6", "car")
	if err != nil {
		t.Fatalf("falha ao entrar p1: %v", err)
	}
	p2, err := room.JoinPlayer("Ana", "#ef4444", "hat")
	if err != nil {
		t.Fatalf("falha ao entrar p2: %v", err)
	}

	if len(room.Game.Players) != 2 {
		t.Errorf("esperado 2 jogadores na sala, recebido %d", len(room.Game.Players))
	}

	// 2. Iniciar votação relâmpago de 15s
	if err := room.StartVoting(); err != nil {
		t.Fatalf("falha ao iniciar votação: %v", err)
	}
	if room.Voting == nil || len(room.Voting.Candidates) != 4 {
		t.Fatalf("esperado sessão de votação com 4 candidatos")
	}

	// 3. Votar e concluir votação
	if err := room.CastVote(p1.ID, 0); err != nil {
		t.Fatalf("falha ao votar: %v", err)
	}
	winner, err := room.ConcludeVoting()
	if err != nil {
		t.Fatalf("falha ao concluir votação: %v", err)
	}
	if winner == "" {
		t.Errorf("vencedor não pode ser vazio")
	}

	// 4. Iniciar partida e validar turno
	if err := room.StartGame(); err != nil {
		t.Fatalf("falha ao iniciar partida: %v", err)
	}
	if !room.Game.IsStarted {
		t.Errorf("jogo deveria estar iniciado")
	}

	// 5. Reconexão de jogador caído
	reconnectedP1, err := room.ReconnectPlayer(p1.ID)
	if err != nil {
		t.Fatalf("falha ao reconectar p1: %v", err)
	}
	if reconnectedP1.ID != p1.ID || reconnectedP1.Name != "Lucas" {
		t.Errorf("dados do jogador reconectado divergem: %+v", reconnectedP1)
	}

	// 6. Testar timeout de turno de 30s
	room.Game.TurnStartTime = time.Now().Add(-35 * time.Second)
	room.CheckTurnTimeout()
	if room.Game.CurrentTurnPlayerID != p2.ID {
		t.Errorf("auto-pass deveria ter passado a vez para p2 por timeout, atual: %s", room.Game.CurrentTurnPlayerID)
	}
}
