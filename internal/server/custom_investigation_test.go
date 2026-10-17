package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomInvestigation_Endpoint(t *testing.T) {
	router, _, _ := setupTestServer(t)

	payload := map[string]interface{}{
		"city":          "Curitiba",
		"age":           14,
		"school_grade":  "9º Ano",
		"difficulty":    "dificil",
		"friends":       []string{"Lucas", "Mariana"},
		"theme":         "mansion",
		"focus_subject": "Química",
		"tone":          "Suspense Investigativo",
		"weather_era":   "Noite de Tempestade em 1948",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/game/custom-investigation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Case struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Victim      string `json:"victim"`
		} `json:"case"`
		Suspects []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"suspects"`
		Clues []struct {
			ID          int64  `json:"id"`
			Location    string `json:"location"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"clues"`
		TimeRemainingMinutes int `json:"time_remaining_minutes"`
		WarrantsRemaining    int `json:"warrants_remaining"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if res.Case.ID == 0 {
		t.Errorf("expected valid case ID > 0")
	}

	// Verify City applied in Case description or Clues location
	cityFound := strings.Contains(res.Case.Description, "Curitiba") || strings.Contains(res.Case.Title, "Curitiba")
	for _, c := range res.Clues {
		if strings.Contains(c.Location, "Curitiba") || strings.Contains(c.Description, "Curitiba") {
			cityFound = true
			break
		}
	}
	if !cityFound {
		t.Errorf("expected city 'Curitiba' to be reflected in case description or clues")
	}

	// Verify Friends in suspects
	suspectNames := make(map[string]bool)
	for _, s := range res.Suspects {
		suspectNames[s.Name] = true
	}
	if !suspectNames["Lucas"] && !suspectNames["Mariana"] {
		t.Errorf("expected friends 'Lucas' or 'Mariana' to be included in suspects, got: %+v", suspectNames)
	}

	// Verify Difficulty limits (dificil -> 300 min, 1 warrant)
	if res.WarrantsRemaining != 1 {
		t.Errorf("expected 1 warrant for difficulty 'dificil', got %d", res.WarrantsRemaining)
	}
	if res.TimeRemainingMinutes != 300 {
		t.Errorf("expected 300 minutes for difficulty 'dificil', got %d", res.TimeRemainingMinutes)
	}
}

func TestCustomInvestigation_SoloStartCompatibility(t *testing.T) {
	router, _, _ := setupTestServer(t)

	payload := map[string]interface{}{
		"city":       "São Paulo",
		"difficulty": "facil",
		"friends":    []string{"Roberto"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/game/solo/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Suspects []struct {
			Name string `json:"name"`
		} `json:"suspects"`
		TimeRemainingMinutes int `json:"time_remaining_minutes"`
		WarrantsRemaining    int `json:"warrants_remaining"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	// Verify Difficulty limits (facil -> 540 min, 3 warrants)
	if res.WarrantsRemaining != 3 {
		t.Errorf("expected 3 warrants for difficulty 'facil', got %d", res.WarrantsRemaining)
	}
	if res.TimeRemainingMinutes != 540 {
		t.Errorf("expected 540 minutes for difficulty 'facil', got %d", res.TimeRemainingMinutes)
	}

	// Verify friend Roberto in suspects
	var robertoFound bool
	for _, s := range res.Suspects {
		if s.Name == "Roberto" {
			robertoFound = true
			break
		}
	}
	if !robertoFound {
		t.Errorf("expected friend 'Roberto' in suspects")
	}
}

func TestCustomInvestigation_MethodNotAllowed(t *testing.T) {
	router, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/game/custom-investigation", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 Method Not Allowed, got %d", rec.Code)
	}
}
