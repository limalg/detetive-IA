package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"detetive-ia/internal/server"
)

func TestGetHubGames_Success(t *testing.T) {
	router := server.NewRouter(server.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/hub/games", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var res struct {
		Games []struct {
			ID                 string   `json:"id"`
			Title              string   `json:"title"`
			Subtitle           string   `json:"subtitle"`
			Description        string   `json:"description"`
			Genre              string   `json:"genre"`
			MinPlayers         int      `json:"min_players"`
			MaxPlayers         int      `json:"max_players"`
			Difficulty         string   `json:"difficulty"`
			Status             string   `json:"status"`
			Badge              string   `json:"badge"`
			Route              string   `json:"route"`
			Icon               string   `json:"icon"`
			CustomizableFields []string `json:"customizable_fields"`
		} `json:"games"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if len(res.Games) != 8 {
		t.Fatalf("expected 8 games in hub catalog, got %d", len(res.Games))
	}

	expectedGames := map[string]struct {
		status string
		badge  string
		route  string
		icon   string
	}{
		"detetive-ia":         {status: "active", badge: "Disponível", route: "/", icon: "search"},
		"banco-imobiliario-ia": {status: "active", badge: "Disponível", route: "/banco-imobiliario", icon: "building"},
		"codenames-ia":        {status: "in_construction", badge: "Em Construção", route: "/codenames", icon: "grid"},
		"dixit-ia":            {status: "in_construction", badge: "Em Construção", route: "/dixit", icon: "image"},
		"tabu-ia":             {status: "in_construction", badge: "Em Construção", route: "/tabu", icon: "slash"},
		"adivinhe-quem-ia":    {status: "in_construction", badge: "Em Construção", route: "/adivinhe-quem", icon: "users"},
		"ticket-to-ride-ia":   {status: "in_construction", badge: "Em Construção", route: "/ticket-to-ride", icon: "compass"},
		"story-cubes-ia":      {status: "in_construction", badge: "Em Construção", route: "/story-cubes", icon: "box"},
	}

	gamesByID := make(map[string]struct {
		status             string
		badge              string
		route              string
		icon               string
		customizableFields []string
	})

	for _, g := range res.Games {
		gamesByID[g.ID] = struct {
			status             string
			badge              string
			route              string
			icon               string
			customizableFields []string
		}{
			status:             g.Status,
			badge:              g.Badge,
			route:              g.Route,
			icon:               g.Icon,
			customizableFields: g.CustomizableFields,
		}
	}

	for id, expected := range expectedGames {
		actual, found := gamesByID[id]
		if !found {
			t.Errorf("game %q not found in hub catalog", id)
			continue
		}
		if actual.status != expected.status {
			t.Errorf("game %q status: expected %q, got %q", id, expected.status, actual.status)
		}
		if actual.badge != expected.badge {
			t.Errorf("game %q badge: expected %q, got %q", id, expected.badge, actual.badge)
		}
		if actual.route != expected.route {
			t.Errorf("game %q route: expected %q, got %q", id, expected.route, actual.route)
		}
		if actual.icon != expected.icon {
			t.Errorf("game %q icon: expected %q, got %q", id, expected.icon, actual.icon)
		}
	}

	// Validate customizable_fields in Detetive IA
	detetiveFields := gamesByID["detetive-ia"].customizableFields
	if len(detetiveFields) == 0 {
		t.Errorf("expected customizable_fields populated for detetive-ia")
	}
}

func TestGetHubGames_MethodNotAllowed(t *testing.T) {
	router := server.NewRouter(server.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/hub/games", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 Method Not Allowed, got %d", rec.Code)
	}
}
