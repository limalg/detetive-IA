package banco3d

import (
	"errors"
	"sync"
	"time"
)

// VotingSession manages the 15-second lightning vote for the city's iconic landmark.
type VotingSession struct {
	mu         sync.Mutex
	Candidates []string          `json:"candidates"`
	Duration   time.Duration     `json:"duration"`
	Deadline   time.Time         `json:"deadline"`
	Votes      map[string]int    `json:"votes"` // playerID -> candidateIndex (0..3)
	IsClosed   bool              `json:"is_closed"`
	Winner     string            `json:"winner,omitempty"`
}

// NewVotingSession initializes a voting session for the candidates list.
func NewVotingSession(candidates []string, duration time.Duration) *VotingSession {
	if duration <= 0 {
		duration = 15 * time.Second
	}
	clean := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c != "" {
			clean = append(clean, c)
		}
	}
	if len(clean) == 0 {
		clean = []string{"Centro Histórico", "Praça Central", "Marco Zero", "Mirante Principal"}
	}

	return &VotingSession{
		Candidates: clean,
		Duration:   duration,
		Deadline:   time.Now().Add(duration),
		Votes:      make(map[string]int),
	}
}

// Vote casts or updates a player's vote for candidate index.
func (v *VotingSession) Vote(playerID string, candidateIndex int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.IsClosed {
		return errors.New("votação já encerrada")
	}
	if candidateIndex < 0 || candidateIndex >= len(v.Candidates) {
		return errors.New("candidato inválido")
	}

	v.Votes[playerID] = candidateIndex
	return nil
}

// CloseAndElect concludes voting, computes winner deterministically, and assigns to tile 39.
func (v *VotingSession) CloseAndElect(board *Board) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.IsClosed && v.Winner != "" {
		return v.Winner, nil
	}

	v.IsClosed = true

	// Contar votos por índice de candidato
	counts := make([]int, len(v.Candidates))
	totalVotes := 0
	for _, candIdx := range v.Votes {
		if candIdx >= 0 && candIdx < len(counts) {
			counts[candIdx]++
			totalVotes++
		}
	}

	bestIdx := 0
	maxVotes := -1

	// Desempate determinístico: percorre em ordem crescente de índice.
	// Se count > maxVotes, atualiza. Se empatar, mantém o menor índice.
	for idx, count := range counts {
		if count > maxVotes {
			maxVotes = count
			bestIdx = idx
		}
	}

	if totalVotes == 0 {
		bestIdx = 0
	}

	winner := v.Candidates[bestIdx]
	v.Winner = winner

	// Atribuir à casa rainha (índice 39) no tabuleiro
	if board != nil && len(board.Tiles) == 40 {
		queen := &board.Tiles[39]

		// Swap seguro caso o vencedor já esteja em outra casa de propriedade
		for i := 0; i < 39; i++ {
			if board.Tiles[i].Type == TileProperty && board.Tiles[i].Name == winner {
				// Trocar o nome da casa duplicada pelo nome que estava na 39
				board.Tiles[i].Name = queen.Name
				break
			}
		}

		queen.Name = winner
		queen.Group = ColorDarkBlue
		queen.IsQueen = true
		queen.Price = 800
		queen.Rent = [6]int{60, 300, 900, 1750, 2000, 2500}
		queen.HouseCost = 200
	}

	return winner, nil
}
