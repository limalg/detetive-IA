package game

import "detetive-ia/internal/models"

// BoardGame aliases models.BoardGame for external convenience.
type BoardGame = models.BoardGame

// GetHubGames returns the list of tabletop board games featured in the AI Hub catalog.
func GetHubGames() []models.BoardGame {
	return []models.BoardGame{
		{
			ID:          "detetive-ia",
			Title:       "Detetive IA",
			Subtitle:    "Mistério, Dedução & Interrogatórios Noir",
			Description: "Casos policiais procedurais e Detective Night com 3 envelopes digitais, mesa tátil de investigação e interrogatórios com IA.",
			Genre:       "Investigação & Dedução",
			MinPlayers:  1,
			MaxPlayers:  6,
			Difficulty:  "Médio",
			Status:      "active",
			Badge:       "Disponível",
			Route:       "/",
			Icon:        "search",
		},
		{
			ID:          "banco-imobiliario-ia",
			Title:       "Banco Imobiliário IA",
			Subtitle:    "Estratégia Financeira & Negociações com IA",
			Description: "Negociação de imóveis, leilões e acordos com IA. Construa seu império imobiliário e dispute negócios acirrados com agentes autônomos.",
			Genre:       "Estratégia & Economia",
			MinPlayers:  2,
			MaxPlayers:  4,
			Difficulty:  "Avançado",
			Status:      "in_construction",
			Badge:       "Em Construção",
			Route:       "/banco-imobiliario",
			Icon:        "building",
		},
	}
}
