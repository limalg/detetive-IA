package server

import (
	"encoding/json"
	"net/http"

	"detetive-ia/internal/models"
)

// HandleCustomInvestigation creates an investigation tailored by custom player dimensions.
func (h *Handler) HandleCustomInvestigation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.cfg.GameService == nil {
		h.error(w, http.StatusInternalServerError, "game service not available")
		return
	}

	var params models.CustomInvestigationParams
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&params)
	}

	var userID int64
	if user, err := h.getAuthenticatedUser(r); err == nil && user != nil {
		userID = user.ID
	}

	details, inv, err := h.cfg.GameService.StartCustomInvestigation(userID, params)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "failed to start custom investigation: "+err.Error())
		return
	}

	// Sanitize output for players
	publicCase := map[string]interface{}{
		"id":          details.Case.ID,
		"title":       details.Case.Title,
		"description": details.Case.Description,
		"victim":      details.Case.Victim,
	}

	var publicSuspects []map[string]interface{}
	for _, s := range details.Suspects {
		publicSuspects = append(publicSuspects, map[string]interface{}{
			"id":          s.ID,
			"name":        s.Name,
			"role":        s.Role,
			"personality": s.Personality,
			"alibi":       s.Alibi,
		})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"case":                   publicCase,
		"suspects":               publicSuspects,
		"clues":                  details.Clues,
		"time_remaining_minutes": inv.TimeRemainingMinutes,
		"warrants_remaining":     inv.WarrantsRemaining,
	})
}
