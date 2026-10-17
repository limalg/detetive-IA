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
			ID          string `json:"id"`
			Title       string `json:"title"`
			Subtitle    string `json:"subtitle"`
			Description string `json:"description"`
			Genre       string `json:"genre"`
			MinPlayers  int    `json:"min_players"`
			MaxPlayers  int    `json:"max_players"`
			Difficulty  string `json:"difficulty"`
			Status      string `json:"status"`
			Badge       string `json:"badge"`
			Route       string `json:"route"`
			Icon        string `json:"icon"`
		} `json:"games"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if len(res.Games) < 2 {
		t.Fatalf("expected at least 2 games in hub catalog, got %d", len(res.Games))
	}

	// Validate Detetive IA
	var detetiveFound bool
	for _, g := range res.Games {
		if g.ID == "detetive-ia" {
			detetiveFound = true
			if g.Status != "active" {
				t.Errorf("expected detetive-ia status 'active', got %q", g.Status)
			}
			if g.Badge != "Disponível" {
				t.Errorf("expected detetive-ia badge 'Disponível', got %q", g.Badge)
			}
			if g.MinPlayers != 1 || g.MaxPlayers != 6 {
				t.Errorf("expected players 1-6 for detetive-ia, got %d-%d", g.MinPlayers, g.MaxPlayers)
			}
			if g.Route != "/" {
				t.Errorf("expected detetive-ia route '/', got %q", g.Route)
			}
			if g.Title != "Detetive IA" {
				t.Errorf("expected detetive-ia title 'Detetive IA', got %q", g.Title)
			}
		}
	}
	if !detetiveFound {
		t.Errorf("detetive-ia not found in hub games")
	}

	// Validate Banco Imobiliário IA
	var bancoFound bool
	for _, g := range res.Games {
		if g.ID == "banco-imobiliario-ia" {
			bancoFound = true
			if g.Status != "in_construction" {
				t.Errorf("expected banco-imobiliario-ia status 'in_construction', got %q", g.Status)
			}
			if g.Badge != "Em Construção" {
				t.Errorf("expected banco-imobiliario-ia badge 'Em Construção', got %q", g.Badge)
			}
			if g.MinPlayers != 2 || g.MaxPlayers != 4 {
				t.Errorf("expected players 2-4 for banco-imobiliario-ia, got %d-%d", g.MinPlayers, g.MaxPlayers)
			}
			if g.Route != "/banco-imobiliario" {
				t.Errorf("expected banco-imobiliario-ia route '/banco-imobiliario', got %q", g.Route)
			}
			if g.Title != "Banco Imobiliário IA" {
				t.Errorf("expected banco-imobiliario-ia title 'Banco Imobiliário IA', got %q", g.Title)
			}
		}
	}
	if !bancoFound {
		t.Errorf("banco-imobiliario-ia not found in hub games")
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
