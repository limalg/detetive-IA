package banco3d

import (
	"strings"
	"testing"
)

func TestPlacesSuggestOfflineAndCache(t *testing.T) {
	svc := NewPlacesService()

	// 1. Search for "cur"
	res, err := svc.Suggest("cur")
	if err != nil {
		t.Fatalf("unexpected error suggesting places: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected suggestions for 'cur', got none")
	}

	foundCuritiba := false
	for _, s := range res {
		if s.Name == "Curitiba" {
			foundCuritiba = true
			if s.Description == "" {
				t.Errorf("expected description for Curitiba, got empty")
			}
			break
		}
	}
	if !foundCuritiba {
		t.Errorf("Curitiba not found in suggestions for 'cur'")
	}

	// 2. Cache hit test (should return immediately from cache)
	cached, err2 := svc.Suggest("cur")
	if err2 != nil || len(cached) != len(res) {
		t.Errorf("cache retrieval failed")
	}

	// 3. Search for international city "paris"
	parisRes, err3 := svc.Suggest("paris")
	if err3 != nil || len(parisRes) == 0 {
		t.Fatalf("expected suggestions for 'paris'")
	}
	if parisRes[0].Name != "Paris" {
		t.Errorf("expected Paris as first result, got %s", parisRes[0].Name)
	}

	// 4. Search for "londrina"
	londrinaRes, err4 := svc.Suggest("londrina")
	if err4 != nil || len(londrinaRes) == 0 {
		t.Fatalf("expected suggestions for 'londrina'")
	}
	if londrinaRes[0].Name != "Londrina" {
		t.Errorf("expected Londrina as first result, got %s", londrinaRes[0].Name)
	}
}

func TestPlacesLandmarks(t *testing.T) {
	svc := NewPlacesService()

	cities := []string{
		"Curitiba",
		"Londrina",
		"Recife",
		"Porto Alegre",
		"Brasília",
		"Fortaleza",
		"Florianópolis",
	}

	for _, city := range cities {
		t.Run(city, func(t *testing.T) {
			res, err := svc.GetLandmarks(city, "")
			if err != nil {
				t.Fatalf("unexpected error getting landmarks for %s: %v", city, err)
			}
			if len(res.Candidates) != 4 {
				t.Fatalf("[%s] expected exactly 4 candidates, got %d", city, len(res.Candidates))
			}
			if len(res.Landmarks) < 28 {
				t.Fatalf("[%s] expected at least 28 landmarks, got %d", city, len(res.Landmarks))
			}

			// Verify candidates have real names and no empty or generic procedural placeholders
			for _, cand := range res.Candidates {
				if cand == "" || strings.HasPrefix(cand, "Praça Central de ") {
					t.Errorf("[%s] candidate is generic procedural stub: %s", city, cand)
				}
			}

			// Check image kinds in landmarks
			hasStreet := false
			hasStation := false
			hasUtility := false
			for _, l := range res.Landmarks {
				if l.ImageKind == "street" || l.ImageKind == "avenue" || l.ImageKind == "square" || l.ImageKind == "park" || l.ImageKind == "landmark" {
					hasStreet = true
				}
				if l.ImageKind == "station" {
					hasStation = true
				}
				if l.ImageKind == "utility" {
					hasUtility = true
				}
				// Verify not a procedural placeholder
				if strings.HasSuffix(l.Name, " de "+city) && strings.HasPrefix(l.Name, "Bairro ") {
					t.Errorf("[%s] landmark looks like procedural stub: %s", city, l.Name)
				}
			}
			if !hasStreet || !hasStation || !hasUtility {
				t.Errorf("[%s] landmarks missing essential kinds: street=%v, station=%v, utility=%v", city, hasStreet, hasStation, hasUtility)
			}
		})
	}
}
