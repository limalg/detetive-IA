package banco3d

import (
	"testing"
	"time"
)

func TestVotingElectsIconicProperty(t *testing.T) {
	candidates := []string{"Largo da Ordem", "Jardim Botânico", "Ópera de Arame", "Parque Barigui"}
	v := NewVotingSession(candidates, 15*time.Second)

	// Simular votos: 2 votos para Jardim Botânico (índice 1), 1 para Largo da Ordem (índice 0)
	if err := v.Vote("p1", 1); err != nil {
		t.Fatalf("falha ao votar p1: %v", err)
	}
	if err := v.Vote("p2", 1); err != nil {
		t.Fatalf("falha ao votar p2: %v", err)
	}
	if err := v.Vote("p3", 0); err != nil {
		t.Fatalf("falha ao votar p3: %v", err)
	}

	board := NewStandardBoard("Curitiba", nil)
	winner, err := v.CloseAndElect(board)
	if err != nil {
		t.Fatalf("erro ao eleger: %v", err)
	}
	if winner != "Jardim Botânico" {
		t.Fatalf("esperado Jardim Botânico, recebido %s", winner)
	}

	// Validar que a casa 39 é rainha e tem o nome do vencedor
	if board.Tiles[39].Name != "Jardim Botânico" {
		t.Errorf("casa 39 deve ter nome do vencedor, recebido %s", board.Tiles[39].Name)
	}
	if !board.Tiles[39].IsQueen {
		t.Errorf("casa 39 deve ser rainha")
	}
}

func TestVotingDeterministicTieBreak(t *testing.T) {
	// Empate entre A (0) e C (2) com 2 votos cada
	candidates := []string{"Candidato A", "Candidato B", "Candidato C", "Candidato D"}
	v := NewVotingSession(candidates, 15*time.Second)

	v.Vote("p1", 0) // A
	v.Vote("p2", 2) // C
	v.Vote("p3", 0) // A
	v.Vote("p4", 2) // C

	board := NewStandardBoard("Teste", nil)
	winner, err := v.CloseAndElect(board)
	if err != nil {
		t.Fatalf("erro no desempate: %v", err)
	}

	// Determinístico: menor índice (0 vs 2 -> 0)
	if winner != "Candidato A" {
		t.Errorf("esperado desempate pelo menor índice original (Candidato A), recebido %s", winner)
	}
}

func TestVotingZeroVotesFallback(t *testing.T) {
	candidates := []string{"Fallback Alpha", "Beta", "Gamma", "Delta"}
	v := NewVotingSession(candidates, 15*time.Second)

	board := NewStandardBoard("Teste", nil)
	winner, err := v.CloseAndElect(board)
	if err != nil {
		t.Fatalf("erro com zero votos: %v", err)
	}
	if winner != "Fallback Alpha" {
		t.Errorf("esperado C[0] como fallback de zero votos, recebido %s", winner)
	}
}
