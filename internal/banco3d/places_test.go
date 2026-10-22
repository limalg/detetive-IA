package banco3d

import (
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
}

func TestPlacesLandmarks(t *testing.T) {
	svc := NewPlacesService()

	// 1. Get landmarks for Curitiba
	res, err := svc.GetLandmarks("Curitiba", "")
	if err != nil {
		t.Fatalf("unexpected error getting landmarks: %v", err)
	}
	if len(res.Candidates) != 4 {
		t.Fatalf("expected exactly 4 candidates, got %d", len(res.Candidates))
	}
	if len(res.Landmarks) == 0 {
		t.Fatalf("expected landmarks list to be populated")
	}

	// Check image kinds in landmarks
	hasStreet := false
	hasStation := false
	hasUtility := false
	for _, l := range res.Landmarks {
		if l.ImageKind == "street" || l.ImageKind == "avenue" || l.ImageKind == "square" {
			hasStreet = true
		}
		if l.ImageKind == "station" {
			hasStation = true
		}
		if l.ImageKind == "utility" {
			hasUtility = true
		}
	}
	if !hasStreet || !hasStation || !hasUtility {
		t.Errorf("landmarks missing essential kinds: street=%v, station=%v, utility=%v", hasStreet, hasStation, hasUtility)
	}
}
