package banco3d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	if res.City != "Maringá" || len(res.Candidates) != 4 || len(res.Landmarks) != 24 || !res.Partial {
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

	// Not in catalog: nothing is invented and an empty answer is an error (503), never an empty 200.
	var ue *UnavailableError
	if res, err := svc.GetLandmarks("Sorocaba", ""); !errors.As(err, &ue) || res != nil {
		t.Fatalf("want UnavailableError, got %+v %v", res, err)
	}
	// In catalog: preset names.
	res, _ := svc.GetLandmarks("Londrina", "")
	if res.City != "Londrina" || len(res.Landmarks) != 28 || res.Partial {
		t.Fatalf("bad catalog result %+v", res)
	}
}

func TestDiskCachePersistsDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	svc := mockService(func(r *http.Request) (int, string) {
		return 200, `{"localname":"Caxias do Sul","centroid":{"coordinates":[-51.18,-29.16]}}`
	}).WithDiskCache(path)
	if _, _, _, err := svc.resolvePlaceID(context.Background(), "osm_1"); err != nil {
		t.Fatal(err)
	}
	down := mockService(func(r *http.Request) (int, string) { return 500, "" }).WithDiskCache(path)
	if name, _, _, err := down.resolvePlaceID(context.Background(), "osm_1"); err != nil || name != "Caxias do Sul" {
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

func TestMageNeverLeaksOtherMunicipality(t *testing.T) {
	foreign := []string{"Ipanema", "Cristo Redentor", "Pão de Açúcar", "Praça Vizinha"}
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "nominatim.openstreetmap.org":
			if strings.Contains(r.URL.Path, "details") {
				return 200, `{"localname":"Magé","centroid":{"coordinates":[-43.04,-22.65]}}`
			}
			return 200, `[{"name":"Cristo Redentor","address":{"city":"Rio de Janeiro"}},{"name":"Museu Magé Antigo","address":{"city":"Magé"}}]`
		case "pt.wikipedia.org":
			return 200, `{"query":{"geosearch":[{"title":"Cristo Redentor"},{"title":"Ipanema"},{"title":"Praça Vizinha (Duque de Caxias)"},{"title":"Igreja da Piedade (Magé)"}]}}`
		}
		q := r.URL.Query().Get("data")
		if strings.Contains(q, "80000") || strings.Contains(q, "around:6000") {
			t.Errorf("regional/radius query must not exist: %s", q)
		}
		var els []string
		for i := 0; i < 14; i++ {
			els = append(els, fmt.Sprintf(`{"tags":{"name":"Rua Local %d","highway":"secondary"}}`, i))
		}
		els = append(els, `{"tags":{"name":"Praça da Matriz","historic":"yes","wikidata":"Q1"}}`,
			`{"tags":{"name":"Ipanema","place":"suburb"}}`, `{"tags":{"name":"Pão de Açúcar","tourism":"attraction"}}`)
		return 200, `{"elements":[` + strings.Join(els, ",") + `]}`
	})
	res, err := svc.GetLandmarks("", "osm_mage")
	if err != nil {
		t.Fatal(err)
	}
	if res.City != "Magé" || !res.Partial {
		t.Fatalf("city=%q partial=%v", res.City, res.Partial)
	}
	names := append([]string{}, res.Candidates...)
	for _, l := range res.Landmarks {
		names = append(names, l.Name)
	}
	for _, n := range names {
		if slices.Contains(foreign, n) {
			t.Errorf("other municipality leaked: %s in %v", n, names)
		}
	}
	if !slices.Contains(res.Candidates, "Praça da Matriz") || !slices.Contains(names, "Igreja da Piedade") {
		t.Errorf("own places must lead: %v", res.Candidates)
	}
}

func TestWithExtras(t *testing.T) {
	base := buildLandmarks("Curitiba", nil)
	res := withExtras(base, []string{" Minha Casa ", "minha casa", "Praça do Avô"})
	if res.Candidates[0] != "Minha Casa" || res.Candidates[1] != "Praça do Avô" || len(res.Candidates) != 4 {
		t.Errorf("candidates: %v", res.Candidates)
	}
	if len(res.Landmarks) != 28 || res.Landmarks[1].ImageKind != "square" {
		t.Errorf("want 28 tiles with custom first, got %d %+v", len(res.Landmarks), res.Landmarks[1])
	}
	if withExtras(base, []string{" "}) != base {
		t.Error("no valid extras must return the base result")
	}
}

func TestWikipediaIsMainSourceWhenOverpassDown(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "pt.wikipedia.org":
			var gs []string
			for _, n := range []string{"Catedral de Maringá", "Parque do Japão (Maringá)", "Estação Meteorológica de Maringá", "Teatro Calil Haddad"} {
				gs = append(gs, fmt.Sprintf(`{"title":%q}`, n))
			}
			return 200, `{"query":{"geosearch":[` + strings.Join(gs, ",") + `]}}`
		case "nominatim.openstreetmap.org":
			return 200, `{"localname":"Maringá","centroid":{"coordinates":[-51.9,-23.4]}}`
		}
		return 502, ""
	})
	res, _ := svc.GetLandmarks("", "osm_9")
	got := strings.Join(res.Candidates, "|")
	if !strings.Contains(got, "Parque do Japão") || strings.Contains(got, "Meteorológica") || !res.Partial {
		t.Errorf("candidates %v partial=%v", res.Candidates, res.Partial)
	}
}

func TestBackgroundPassCompletesCachedAnswer(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "nominatim.openstreetmap.org":
			if strings.HasSuffix(r.URL.Path, "/details") {
				return 200, `{"localname":"Maringá","centroid":{"coordinates":[-51.9,-23.4]}}`
			}
			if strings.HasSuffix(r.URL.Path, "/reverse") {
				return 200, fmt.Sprintf(`{"address":{"city":"Maringá","road":"Rua Varrida %s","suburb":"Zona %s"}}`, r.URL.Query().Get("lat")[:7], r.URL.Query().Get("lon")[:7])
			}
			word := strings.Fields(r.URL.Query().Get("q"))[0]
			var hs []string
			for i := 0; i < 5; i++ {
				hs = append(hs, fmt.Sprintf(`{"name":"%s Real %d","address":{"city":"Maringá"}}`, strings.ToUpper(word[:1])+word[1:], i))
			}
			hs = append(hs, `{"name":"Avenida Vizinha","address":{"city":"Sarandi"}}`)
			return 200, `[` + strings.Join(hs, ",") + `]`
		}
		return 503, ""
	})
	first, err := svc.GetLandmarks("", "osm_12066357")
	if err != nil || !first.Partial {
		t.Fatalf("foreground answer should be partial: %+v %v", first, err)
	}
	var second *CityLandmarksResult
	for i := 0; i < 100; i++ {
		time.Sleep(50 * time.Millisecond)
		if second, _ = svc.GetLandmarks("", "osm_12066357"); len(second.Landmarks) > len(first.Landmarks) {
			break
		}
	}
	if len(second.Landmarks) <= len(first.Landmarks) {
		t.Fatalf("background pass did not improve: %d -> %d", len(first.Landmarks), len(second.Landmarks))
	}
	if second.City != "Maringá" {
		t.Errorf("city must stay the resolved one, got %q", second.City)
	}
	var stations int
	for _, l := range second.Landmarks {
		if strings.Contains(l.Name, "Vizinha") {
			t.Errorf("other municipality leaked: %s", l.Name)
		}
		if l.Type == "station" {
			stations++
		}
	}
	if stations == 0 {
		t.Error("terminal/rodoviária searches should yield stations")
	}
}

func TestNominatim429BacksOff(t *testing.T) {
	var nominatimCalls int
	svc := mockService(func(r *http.Request) (int, string) {
		if r.URL.Host == "nominatim.openstreetmap.org" {
			nominatimCalls++
			return 429, ""
		}
		return 503, ""
	})
	if _, err := svc.Suggest("zzbackoff"); err != nil {
		t.Fatal(err)
	}
	var out any
	err := svc.getJSON(context.Background(), "https://nominatim.openstreetmap.org/search", time.Second, true, &out)
	if err != errNominatimBlocked {
		t.Fatalf("want back-off error, got %v", err)
	}
	if _, _, _, err := svc.geocodeCity(context.Background(), "Qualquer"); err != errNominatimBlocked {
		t.Fatalf("geocode must back off too: %v", err)
	}
	if nominatimCalls != 1 {
		t.Errorf("after a 429 no more Nominatim requests are allowed, got %d", nominatimCalls)
	}
}

func TestEmptyUtilitySlotsTakeRealPlaces(t *testing.T) {
	var real []realPlace
	for i := 0; i < 20; i++ {
		real = append(real, realPlace{Name: fmt.Sprintf("Praça Real %d", i), Kind: "property", Score: 50 - i, Top: i < 4})
	}
	real = append(real,
		realPlace{Name: "Shopping Real", Kind: "property", Score: 9},
		realPlace{Name: "Avenida Longa", Kind: "property", Score: 8},
		realPlace{Name: "Hospital Real", Kind: "property", Score: 7},
		realPlace{Name: "Universidade Real", Kind: "property", Score: 6},
		realPlace{Name: "Terminal Um", Kind: "station", Score: 5}, realPlace{Name: "Terminal Dois", Kind: "station", Score: 5},
		realPlace{Name: "Terminal Três", Kind: "station", Score: 5}, realPlace{Name: "Terminal Quatro", Kind: "station", Score: 5})
	res := buildLandmarks("Cidadela", real)
	var utils []string
	for _, l := range res.Landmarks {
		if l.Type == "utility" {
			utils = append(utils, l.Name)
		}
	}
	if len(utils) != 2 || strings.Contains(strings.Join(utils, ","), "Avenida") {
		t.Fatalf("utilities should be real non-street places, got %v", utils)
	}
	if len(res.Landmarks) != 28 || res.Partial {
		t.Errorf("want 28 real names and partial=false, got %d partial=%v", len(res.Landmarks), res.Partial)
	}
}

func TestStreetPlaceIDNeverBecomesCity(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) {
		if r.URL.Host == "nominatim.openstreetmap.org" {
			return 200, `{"category":"highway","rank_address":26,"localname":"Avenida Doutor Júlio Lucant","centroid":{"coordinates":[-51.9,-23.4]}}`
		}
		return 503, ""
	})
	res, err := svc.GetLandmarks("", "osm_12066357")
	var ue *UnavailableError
	if !errors.As(err, &ue) || res != nil {
		t.Fatalf("a street place_id must be refused (503), got res=%+v err=%v", res, err)
	}
}

func TestNoSourceNeverReturnsEmpty200(t *testing.T) {
	svc := mockService(func(r *http.Request) (int, string) {
		if r.URL.Host == "nominatim.openstreetmap.org" {
			return 429, ""
		}
		return 503, ""
	})
	_, err := svc.GetLandmarks("Maringá", "")
	var ue *UnavailableError
	if !errors.As(err, &ue) || ue.RetryAfter < 30*time.Second {
		t.Fatalf("empty cache + 429 must be an UnavailableError with retry_after, got %v", err)
	}
	b, _ := json.Marshal(buildLandmarks("Maringá", nil))
	if strings.Contains(string(b), "null") {
		t.Errorf("lists must never serialize as null: %s", b)
	}
}

func TestOldDiskCacheIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.json")
	old := `{"details":{"osm_1":{"City":"Avenida Suja","Lat":1,"Lon":1}},"landmarks":{}}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewPlacesService().WithDiskCache(path)
	if _, ok := svc.detailsCache["osm_1"]; ok {
		t.Error("cache without the current schema version must not be reused")
	}
}

func TestQualityFilterDropsBadNames(t *testing.T) {
	in := []realPlace{
		{Name: "parte verde", Kind: "property", Score: 9}, {Name: "praça com bancos", Kind: "property", Score: 9},
		{Name: "Campo de Futebol  - VILA INCA FC", Kind: "property", Score: 9}, {Name: "Chácara do castelinho", Kind: "property", Score: 9},
		{Name: "Joaquim leitão", Kind: "property", Score: 9}, {Name: "Praça principal de Piabetá", Kind: "property", Score: 9},
		{Name: "Praça  Da Capela", Kind: "property", Score: 8}, {Name: "Praça Da Capela", Kind: "property", Score: 7},
		{Name: "Igreja de Nossa Senhora da Piedade", Kind: "property", Score: 60, Top: true}, {Name: "Torre d'Água", Kind: "utility", Score: 5},
	}
	var got []string
	for _, p := range dedupeVariants(goodNames(in)) {
		got = append(got, p.Name)
	}
	want := "Praça  Da Capela|Igreja de Nossa Senhora da Piedade|Torre d'Água"
	if len(got) != 3 || strings.Join(got, "|") != want && !strings.Contains(strings.Join(got, "|"), "Igreja de Nossa") {
		t.Fatalf("got %v", got)
	}
	res := buildLandmarks("Magé", dedupeVariants(goodNames(in)))
	if len(res.Candidates) != 2 || res.Candidates[0] != "Igreja de Nossa Senhora da Piedade" {
		t.Errorf("candidates must be notable only: %v", res.Candidates)
	}
}

func TestWikidataAndCategoriesFillMage(t *testing.T) {
	var sparql string
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "nominatim.openstreetmap.org":
			if strings.Contains(r.URL.Path, "details") {
				return 200, `{"localname":"Magé","centroid":{"coordinates":[-43.04,-22.65]}}`
			}
			return 200, `[]`
		case "query.wikidata.org":
			sparql = r.URL.Query().Get("query")
			return 200, `{"results":{"bindings":[
{"label":{"value":"Igreja de Nossa Senhora da Piedade"},"links":{"value":"9"},"station":{"value":"0"}},
{"label":{"value":"Estação de Fragoso"},"links":{"value":"4"},"station":{"value":"1"}},
{"label":{"value":"Guia de Pacobaíba"},"links":{"value":"6"},"station":{"value":"0"}}]}}`
		case "pt.wikipedia.org":
			q := r.URL.Query()
			if q.Get("list") == "categorymembers" {
				switch q.Get("cmtitle") {
				case "Categoria:Magé":
					return 200, `{"query":{"categorymembers":[{"title":"Categoria:Bairros de Magé","ns":14},{"title":"Categoria:Naturais de Magé","ns":14},{"title":"Fulano de Tal","ns":0}]}}`
				case "Categoria:Bairros de Magé":
					var m []string
					for _, n := range []string{"Piabetá", "Santo Aleixo", "Suruí", "Fragoso", "Inhomirim", "Rio do Ouro", "Parada Modelo", "Ipanema (Rio de Janeiro)"} {
						m = append(m, `{"title":"`+n+`","ns":0}`)
					}
					return 200, `{"query":{"categorymembers":[` + strings.Join(m, ",") + `]}}`
				}
				t.Errorf("unexpected category %s", q.Get("cmtitle"))
			}
			return 200, `{"query":{"geosearch":[]}}`
		}
		return 200, `{"elements":[]}`
	})
	res, err := svc.GetLandmarks("", "osm_mage2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sparql, `"Magé"@pt`) || !strings.Contains(sparql, "wdt:P131+") {
		t.Errorf("sparql must be bound to the municipality: %s", sparql)
	}
	names := map[string]bool{}
	for _, l := range res.Landmarks {
		names[l.Name] = true
	}
	for _, want := range []string{"Guia de Pacobaíba", "Santo Aleixo", "Piabetá", "Estação de Fragoso"} {
		if !names[want] {
			t.Errorf("missing real place %s in %v", want, names)
		}
	}
	for _, bad := range []string{"Fulano de Tal", "Ipanema"} {
		if names[bad] {
			t.Errorf("must not appear: %s", bad)
		}
	}
	if len(res.Candidates) != 4 || res.Candidates[0] != "Igreja de Nossa Senhora da Piedade" {
		t.Errorf("candidates ranked by sitelinks: %v", res.Candidates)
	}
}

func TestSuggestBrazilFirstAndStateAware(t *testing.T) {
	var queries []string
	svc := mockService(func(r *http.Request) (int, string) {
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Get("countrycodes") != "br" {
			return 200, `[{"place_id":307444734,"class":"boundary","place_rank":16,"display_name":"Magdalena, Colombia","address":{"city":"Magdalena","country":"Colômbia"}}]`
		}
		return 200, `[{"place_id":1,"class":"boundary","place_rank":16,"display_name":"Magé","address":{"city":"Magé","state":"Rio de Janeiro","country":"Brasil"}},
{"place_id":2,"class":"boundary","place_rank":16,"display_name":"Magé, MG","address":{"city":"Magé","state":"Minas Gerais","country":"Brasil"}}]`
	})
	got, err := svc.Suggest("Mage RJ")
	if err != nil || len(got) != 1 || got[0].ID != "osm_1" || got[0].Description != "Rio de Janeiro, Brasil" {
		t.Fatalf("want only Magé/RJ, got %+v %v", got, err)
	}
	if len(queries) != 1 || !strings.Contains(queries[0], "countrycodes=br") || !strings.Contains(queries[0], "Rio+de+Janeiro") {
		t.Errorf("UF typed: single Brazil-only query expected: %v", queries)
	}
	got, _ = svc.Suggest("Magé")
	if len(got) == 0 || got[0].ID != "osm_1" || strings.Contains(got[0].Name, "Magdalena") {
		t.Errorf("Brazilian Magé must lead: %+v", got)
	}
	if p, st := parseStateSuffix("Caxias do Sul, Rio Grande do Sul"); p != "Caxias do Sul" || st != "Rio Grande do Sul" {
		t.Errorf("parse: %q %q", p, st)
	}
	empty := mockService(func(r *http.Request) (int, string) { return 200, `[]` })
	if got, _ := empty.Suggest("Zzzzxx RJ"); got == nil || len(got) != 0 {
		t.Errorf("suggestions must be [] not nil: %#v", got)
	}
}

func TestIBGEDistrictsFillSmallCity(t *testing.T) {
	var ibgeCalls []string
	svc := mockService(func(r *http.Request) (int, string) {
		switch r.URL.Host {
		case "nominatim.openstreetmap.org":
			return 200, `[{"place_id":10351030,"class":"boundary","place_rank":16,"lat":"-22.65","lon":"-43.04","display_name":"Magé","address":{"city":"Magé","state":"Rio de Janeiro","country":"Brasil"}}]`
		case "servicodados.ibge.gov.br":
			ibgeCalls = append(ibgeCalls, r.URL.Path)
			switch {
			case strings.HasSuffix(r.URL.Path, "/estados/RJ/municipios"):
				return 200, `[{"id":3302502,"nome":"Mangaratiba"},{"id":3302809,"nome":"Magé"}]`
			case strings.HasSuffix(r.URL.Path, "/3302809/distritos"):
				return 200, `[{"nome":"Magé"},{"nome":"Santo Aleixo"},{"nome":"Guia de Pacobaíba"},{"nome":"Inhomirim"}]`
			case strings.HasSuffix(r.URL.Path, "/3302809/subdistritos"):
				return 200, `[{"nome":"Piabetá"},{"nome":"Fragoso"}]`
			}
		}
		return 200, `{"elements":[]}`
	})
	sg, _ := svc.Suggest("Magé RJ")
	if len(sg) != 1 {
		t.Fatalf("suggest: %+v", sg)
	}
	res, err := svc.GetLandmarks("", sg[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, l := range res.Landmarks {
		names[l.Name] = true
	}
	for _, want := range []string{"Santo Aleixo", "Guia de Pacobaíba", "Inhomirim", "Piabetá", "Fragoso"} {
		if !names[want] {
			t.Errorf("official district %s missing: %v", want, names)
		}
	}
	if names["Magé"] || len(ibgeCalls) != 3 {
		t.Errorf("city itself must not be a tile; IBGE calls: %v", ibgeCalls)
	}
}
