package server

import (
	"net/http"

	"detetive-ia/internal/game"
)

// HandleHubGames serves the catalog of tabletop board games with AI.
func (h *Handler) HandleHubGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	games := game.GetHubGames()
	h.json(w, http.StatusOK, map[string]interface{}{
		"games": games,
	})
}
