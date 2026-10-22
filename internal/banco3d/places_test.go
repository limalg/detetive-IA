package banco3d

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	svc := mockService(func(*http.Request) (int, string) { return 503, "" })

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

func TestPlacesLandmarksByPlaceID(t *testing.T) {
	svc := mockService(func(*http.Request) (int, string) { return 503, "" })

	// 1. Repro case: place_id=osm_420024688 with empty city name
	res, err := svc.GetLandmarks("", "osm_420024688")
	if err != nil {
		t.Fatalf("unexpected error getting landmarks by place_id osm_420024688: %v", err)
	}
	if res.City != "Londrina" {
		t.Errorf("expected city 'Londrina', got '%s'", res.City)
	}
	if len(res.Candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(res.Candidates))
	}
	for _, cand := range res.Candidates {
		if strings.Contains(cand, "Praça Central de ") || strings.Contains(cand, "Centro Histórico de ") || cand == "" {
			t.Errorf("got procedural placeholder candidate: '%s'", cand)
		}
	}
	if len(res.Landmarks) < 28 {
		t.Errorf("expected at least 28 landmarks, got %d", len(res.Landmarks))
	}

	// 2. Offline catalog place_ids: Recife and Porto Alegre with empty city
	cases := []struct {
		placeID      string
		expectedCity string
	}{
		{"pl_recife", "Recife"},
		{"pl_poa", "Porto Alegre"},
	}

	for _, c := range cases {
		r, err := svc.GetLandmarks("", c.placeID)
		if err != nil {
			t.Fatalf("[%s] error: %v", c.placeID, err)
		}
		if r.City != c.expectedCity {
			t.Errorf("[%s] expected city '%s', got '%s'", c.placeID, c.expectedCity, r.City)
		}
		if len(r.Candidates) != 4 {
			t.Errorf("[%s] expected 4 candidates, got %d", c.placeID, len(r.Candidates))
		}
		for _, cand := range r.Candidates {
			if strings.Contains(cand, "Praça Central de ") || cand == "" {
				t.Errorf("[%s] got procedural candidate: %s", c.placeID, cand)
			}
		}
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockService(h func(r *http.Request) (int, string)) *PlacesService {
	svc := NewPlacesService()
	svc.minGap = 0
	svc.client = &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		code, body := h(r)
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	return svc
}

func overpassBody(n int) string {
	var els []string
	for i := 0; i < n; i++ {
		els = append(els, fmt.Sprintf(`{"tags":{"name":"Rua Real %d","highway":"tertiary"}}`, i))
	}
	els = append(els,
		`{"tags":{"name":"Catedral Basílica","historic":"church","wikidata":"Q1"}}`,
		`{"tags":{"name":"Parque do Ingá","leisure":"park","wikipedia":"pt:Parque"}}`,
		`{"tags":{"name":"Avenida Colombo","highway":"primary"}}`,
		`{"tags":{"name":"Avenida Colombo","highway":"primary"}}`,
		`{"tags":{"name":"Rodoviária Velha","amenity":"bus_station"}}`,
		`{"tags":{"name":"Torre d'Água","man_made":"water_tower"}}`,
		`{"tags":{"name":"Maringá","place":"suburb"}}`)
	return `{"elements":[` + strings.Join(els, ",") + `]}`
}

func TestGetLandmarksRealPlacesByPlaceID(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "nominatim.openstreetmap.org":
			if r.Header.Get("User-Agent") == "" {
				t.Error("missing User-Agent")
			}
			return 200, `{"localname":"Maringá","centroid":{"coordinates":[-51.93,-23.42]}}`
		default:
			return 200, overpassBody(30)
		}
	})
	res, err := svc.GetLandmarks("", "osm_12024671")
	if err != nil {
		t.Fatal(err)
	}
	if res.City != "Maringá" || len(res.Candidates) != 4 || len(res.Landmarks) != 28 {
		t.Fatalf("city=%q cands=%d lm=%d", res.City, len(res.Candidates), len(res.Landmarks))
	}
	if res.Candidates[0] != "Catedral Basílica" && res.Candidates[0] != "Parque do Ingá" {
		t.Errorf("famous places should lead candidates: %v", res.Candidates)
	}
	seen := map[string]bool{}
	for _, l := range res.Landmarks {
		if seen[l.Name] {
			t.Errorf("duplicate %s", l.Name)
		}
		seen[l.Name] = true
		if strings.Contains(l.Name, "Curitiba") {
			t.Errorf("other municipality leaked: %s", l.Name)
		}
	}
	if !seen["Rodoviária Velha"] || !seen["Torre d'Água"] {
		t.Errorf("station/utility not classified: %v", seen)
	}
	if seen["Maringá"] {
		t.Error("city name itself must not be a tile")
	}
}

func TestGetLandmarksAPIFailure(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) { return 503, "" })

	// Not in catalog: pad with the city's own names, never another municipality.
	res, _ := svc.GetLandmarks("Sorocaba", "")
	if res.City != "Sorocaba" || len(res.Landmarks) != 28 || len(res.Candidates) != 4 {
		t.Fatalf("bad result %+v", res)
	}
	for _, l := range res.Landmarks {
		if !strings.Contains(l.Name, "Sorocaba") {
			t.Errorf("non-municipal name %q", l.Name)
		}
	}
	// In catalog: preset names.
	res, _ = svc.GetLandmarks("Londrina", "")
	if res.City != "Londrina" || len(res.Landmarks) != 28 {
		t.Fatalf("bad catalog result %+v", res)
	}
}

func TestDiskCachePersistsDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	svc := mockService(func(r *http.Request) (int, string) {
		return 200, `{"localname":"Caxias do Sul","centroid":{"coordinates":[-51.18,-29.16]}}`
	}).WithDiskCache(path)
	if _, _, _, err := svc.resolvePlaceID("osm_1"); err != nil {
		t.Fatal(err)
	}
	down := mockService(func(r *http.Request) (int, string) { return 500, "" }).WithDiskCache(path)
	if name, _, _, err := down.resolvePlaceID("osm_1"); err != nil || name != "Caxias do Sul" {
		t.Fatalf("disk cache miss: %q %v", name, err)
	}
}

func TestPlacesIntegrationOptional(t *testing.T) {
	if os.Getenv("BANCO3D_INTEGRATION") == "" {
		t.Skip("set BANCO3D_INTEGRATION=1 for live Nominatim/Overpass")
	}
	res, err := NewPlacesService().GetLandmarks("", "osm_12024671")
	if err != nil || res.City != "Maringá" {
		t.Fatalf("%+v %v", res, err)
	}
}
