package banco3d

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PlaceSuggestion describes an autocomplete search hit.
type PlaceSuggestion struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LandmarkItem describes a real landmark, street, or public facility.
type LandmarkItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"`       // "property", "station", "utility", "queen"
	ImageKind   string `json:"image_kind"` // "street", "avenue", "square", "park", "landmark", "station", "utility"
	Description string `json:"description,omitempty"`
}

// CityLandmarksResult wraps the candidates and classified places of a city.
type CityLandmarksResult struct {
	City       string         `json:"city"`
	Candidates []string       `json:"candidates"`
	Landmarks  []LandmarkItem `json:"landmarks"`
}

// PlaceDetails stores resolved geocoding info for a place.
type PlaceDetails struct {
	City string
	Lat  float64
	Lon  float64
}

// PlacesService handles geocoding, autocomplete and landmark extraction.
type PlacesService struct {
	mu            sync.Mutex
	client        *http.Client
	lastReqTime   time.Time
	suggestCache  map[string][]PlaceSuggestion
	landmarkCache map[string]*CityLandmarksResult
	detailsCache  map[string]PlaceDetails
}

// GlobalPlacesService is the singleton geocoding and landmark provider.
var GlobalPlacesService = NewPlacesService()

func NewPlacesService() *PlacesService {
	return &PlacesService{
		client: &http.Client{
			Timeout: 4000 * time.Millisecond,
		},
		suggestCache:  make(map[string][]PlaceSuggestion),
		landmarkCache: make(map[string]*CityLandmarksResult),
		detailsCache:  make(map[string]PlaceDetails),
	}
}

// Suggest returns matching cities or regions for the given query with cache & fallback.
func (s *PlacesService) Suggest(query string) ([]PlaceSuggestion, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []PlaceSuggestion{}, nil
	}
	norm := strings.ToLower(q)

	s.mu.Lock()
	if cached, ok := s.suggestCache[norm]; ok {
		s.mu.Unlock()
		return cached, nil
	}
	// Enforce 1 request per second rate-limit for Nominatim compliance
	elapsed := time.Since(s.lastReqTime)
	if elapsed < 1*time.Second {
		time.Sleep(1*time.Second - elapsed)
	}
	s.lastReqTime = time.Now()
	s.mu.Unlock()

	// Merge known catalog matches with remote suggestions
	offlineMatches := s.filterOfflineCatalog(q)

	// Try OpenStreetMap / Nominatim search
	remoteSuggestions, err := s.fetchNominatimSuggest(q)
	if err == nil && len(remoteSuggestions) > 0 {
		seen := make(map[string]bool)
		var merged []PlaceSuggestion
		for _, o := range offlineMatches {
			key := strings.ToLower(o.Name)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, o)
			}
		}
		for _, r := range remoteSuggestions {
			key := strings.ToLower(r.Name)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, r)
			}
			if len(merged) >= 6 {
				break
			}
		}
		if len(merged) > 6 {
			merged = merged[:6]
		}
		s.mu.Lock()
		s.suggestCache[norm] = merged
		s.mu.Unlock()
		return merged, nil
	}

	// Fallback to offline catalog
	s.mu.Lock()
	s.suggestCache[norm] = offlineMatches
	s.mu.Unlock()
	return offlineMatches, nil
}

func (s *PlacesService) fetchNominatimSuggest(query string) ([]PlaceSuggestion, error) {
	endpoint := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&addressdetails=1&limit=6", url.QueryEscape(query))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DetetiveIABanco3D/0.7 (contact@detetive-ia.local)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var results []struct {
		PlaceID      int64  `json:"place_id"`
		Lat          string `json:"lat"`
		Lon          string `json:"lon"`
		DisplayName  string `json:"display_name"`
		Address      struct {
			City         string `json:"city"`
			Town         string `json:"town"`
			Village      string `json:"village"`
			Municipality string `json:"municipality"`
			State        string `json:"state"`
			Country      string `json:"country"`
		} `json:"address"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	var suggestions []PlaceSuggestion
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range results {
		cityName := r.Address.City
		if cityName == "" {
			cityName = r.Address.Town
		}
		if cityName == "" {
			cityName = r.Address.Municipality
		}
		if cityName == "" {
			cityName = r.Address.Village
		}
		if cityName == "" {
			parts := strings.Split(r.DisplayName, ",")
			cityName = strings.TrimSpace(parts[0])
		}

		desc := r.Address.State
		if desc != "" && r.Address.Country != "" {
			desc = desc + ", " + r.Address.Country
		} else if desc == "" {
			desc = r.Address.Country
		}
		if desc == "" {
			desc = r.DisplayName
		}

		id := fmt.Sprintf("osm_%d", r.PlaceID)
		suggestions = append(suggestions, PlaceSuggestion{
			ID:          id,
			Name:        cityName,
			Description: desc,
		})

		var latF, lonF float64
		_, _ = fmt.Sscanf(r.Lat, "%f", &latF)
		_, _ = fmt.Sscanf(r.Lon, "%f", &lonF)
		s.detailsCache[id] = PlaceDetails{
			City: cityName,
			Lat:  latF,
			Lon:  lonF,
		}
	}

	return suggestions, nil
}

func (s *PlacesService) filterOfflineCatalog(query string) []PlaceSuggestion {
	q := strings.ToLower(strings.TrimSpace(query))
	all := []PlaceSuggestion{
		{ID: "pl_curitiba", Name: "Curitiba", Description: "Paraná, Brasil"},
		{ID: "pl_londrina", Name: "Londrina", Description: "Paraná, Brasil"},
		{ID: "pl_sp", Name: "São Paulo", Description: "São Paulo, Brasil"},
		{ID: "pl_rio", Name: "Rio de Janeiro", Description: "Rio de Janeiro, Brasil"},
		{ID: "pl_bh", Name: "Belo Horizonte", Description: "Minas Gerais, Brasil"},
		{ID: "pl_salvador", Name: "Salvador", Description: "Bahia, Brasil"},
		{ID: "pl_brasilia", Name: "Brasília", Description: "Distrito Federal, Brasil"},
		{ID: "pl_fortaleza", Name: "Fortaleza", Description: "Ceará, Brasil"},
		{ID: "pl_recife", Name: "Recife", Description: "Pernambuco, Brasil"},
		{ID: "pl_poa", Name: "Porto Alegre", Description: "Rio Grande do Sul, Brasil"},
		{ID: "pl_florianopolis", Name: "Florianópolis", Description: "Santa Catarina, Brasil"},
		{ID: "pl_manaus", Name: "Manaus", Description: "Amazonas, Brasil"},
		{ID: "pl_belem", Name: "Belém", Description: "Pará, Brasil"},
		{ID: "pl_goiania", Name: "Goiânia", Description: "Goiás, Brasil"},
		{ID: "pl_ny", Name: "New York", Description: "Nova York, Estados Unidos"},
		{ID: "pl_paris", Name: "Paris", Description: "Île-de-France, França"},
		{ID: "pl_london", Name: "Londres", Description: "Inglaterra, Reino Unido"},
		{ID: "pl_tokyo", Name: "Tóquio", Description: "Kanto, Japão"},
		{ID: "pl_lisboa", Name: "Lisboa", Description: "Estremadura, Portugal"},
		{ID: "pl_buenosaires", Name: "Buenos Aires", Description: "Argentina"},
		{ID: "pl_roma", Name: "Roma", Description: "Lazio, Itália"},
	}

	var matched []PlaceSuggestion
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Description), q) {
			matched = append(matched, p)
			if len(matched) >= 6 {
				break
			}
		}
	}
	return matched
}

// resolvePlaceID maps place_id (e.g. osm_420024688 or pl_londrina) to CityName and Lat/Lon.
func (s *PlacesService) resolvePlaceID(placeID string) (string, float64, float64, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return "", 0, 0, fmt.Errorf("empty place_id")
	}

	s.mu.Lock()
	if d, ok := s.detailsCache[placeID]; ok && d.City != "" {
		s.mu.Unlock()
		return d.City, d.Lat, d.Lon, nil
	}
	s.mu.Unlock()

	// Offline known places
	switch placeID {
	case "osm_420024688", "420024688", "pl_londrina":
		return "Londrina", -23.3113, -51.1595, nil
	case "pl_recife":
		return "Recife", -8.0578, -34.8778, nil
	case "pl_poa":
		return "Porto Alegre", -30.0346, -51.2177, nil
	case "pl_curitiba":
		return "Curitiba", -25.4284, -49.2733, nil
	case "pl_sp":
		return "São Paulo", -23.5505, -46.6333, nil
	case "pl_rio":
		return "Rio de Janeiro", -22.9068, -43.1729, nil
	case "pl_bh":
		return "Belo Horizonte", -19.9167, -43.9345, nil
	case "pl_salvador":
		return "Salvador", -12.9777, -38.5016, nil
	case "pl_brasilia":
		return "Brasília", -15.7975, -47.8919, nil
	case "pl_fortaleza":
		return "Fortaleza", -3.7319, -38.5267, nil
	case "pl_florianopolis":
		return "Florianópolis", -27.5954, -48.5480, nil
	}

	// Remote Nominatim details lookup
	if strings.HasPrefix(placeID, "osm_") {
		numID := strings.TrimPrefix(placeID, "osm_")
		endpoint := fmt.Sprintf("https://nominatim.openstreetmap.org/details?place_id=%s&format=json", numID)
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err == nil {
			req.Header.Set("User-Agent", "DetetiveIABanco3D/0.7 (contact@detetive-ia.local)")
			resp, err := s.client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var details struct {
						LocalName string            `json:"localname"`
						Names     map[string]string `json:"names"`
						Centroid  struct {
							Coordinates [2]float64 `json:"coordinates"` // [lon, lat]
						} `json:"centroid"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&details); err == nil {
						cityName := details.LocalName
						if cityName == "" && details.Names != nil {
							cityName = details.Names["name"]
							if cityName == "" {
								cityName = details.Names["name:pt"]
							}
						}
						lat := details.Centroid.Coordinates[1]
						lon := details.Centroid.Coordinates[0]
						if cityName != "" {
							s.mu.Lock()
							s.detailsCache[placeID] = PlaceDetails{City: cityName, Lat: lat, Lon: lon}
							s.mu.Unlock()
							return cityName, lat, lon, nil
						}
					}
				}
			}
		}
	}

	return "", 0, 0, fmt.Errorf("place_id %s not resolved", placeID)
}

func getCityCoords(cityName string) (float64, float64) {
	norm := strings.ToLower(cityName)
	switch {
	case strings.Contains(norm, "londrina"):
		return -23.3113, -51.1595
	case strings.Contains(norm, "recife"):
		return -8.0578, -34.8778
	case strings.Contains(norm, "porto alegre") || strings.Contains(norm, "poa"):
		return -30.0346, -51.2177
	case strings.Contains(norm, "curitiba"):
		return -25.4284, -49.2733
	case strings.Contains(norm, "são paulo") || strings.Contains(norm, "sao paulo") || strings.Contains(norm, "sp"):
		return -23.5505, -46.6333
	case strings.Contains(norm, "rio"):
		return -22.9068, -43.1729
	case strings.Contains(norm, "brasilia") || strings.Contains(norm, "brasília") || strings.Contains(norm, "df"):
		return -15.7975, -47.8919
	case strings.Contains(norm, "fortaleza"):
		return -3.7319, -38.5267
	case strings.Contains(norm, "florianopolis") || strings.Contains(norm, "florianópolis") || strings.Contains(norm, "floripa"):
		return -27.5954, -48.5480
	case strings.Contains(norm, "belo horizonte") || strings.Contains(norm, "bh"):
		return -19.9167, -43.9345
	case strings.Contains(norm, "salvador"):
		return -12.9777, -38.5016
	default:
		return 0, 0
	}
}

// GetLandmarks returns 4 candidates and landmarks for a city, resolving placeID and fetching real places.
func (s *PlacesService) GetLandmarks(cityName string, placeID string) (*CityLandmarksResult, error) {
	cityName = strings.TrimSpace(cityName)
	placeID = strings.TrimSpace(placeID)

	var lat, lon float64

	// 1. Resolve placeID if provided or if cityName is empty
	if placeID != "" {
		if rCity, rLat, rLon, err := s.resolvePlaceID(placeID); err == nil && rCity != "" {
			if cityName == "" {
				cityName = rCity
			}
			lat = rLat
			lon = rLon
		}
	}

	// 2. Default cityName if still empty
	if cityName == "" {
		cityName = "Curitiba"
	}

	if lat == 0 && lon == 0 {
		lat, lon = getCityCoords(cityName)
	}

	norm := strings.ToLower(cityName)
	cacheKey := norm
	if placeID != "" {
		cacheKey = norm + "_" + placeID
	}

	s.mu.Lock()
	if cached, ok := s.landmarkCache[cacheKey]; ok {
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	// 3. Try fetching real landmarks around coords (Wikipedia geosearch + Overpass)
	if lat != 0 && lon != 0 {
		remoteRes, err := s.fetchRealLandmarks(cityName, lat, lon)
		if err == nil && remoteRes != nil && len(remoteRes.Candidates) == 4 && len(remoteRes.Landmarks) >= 28 {
			s.mu.Lock()
			s.landmarkCache[cacheKey] = remoteRes
			s.mu.Unlock()
			return remoteRes, nil
		}
	}

	// 4. Fallback to authentic city preset (Londrina, Recife, Porto Alegre, Brasília, etc.)
	res := s.fallbackLandmarks(cityName)
	s.mu.Lock()
	s.landmarkCache[cacheKey] = res
	s.mu.Unlock()
	return res, nil
}

func (s *PlacesService) fetchRealLandmarks(cityName string, lat, lon float64) (*CityLandmarksResult, error) {
	// A. Query Wikipedia geosearch (fast and highly relevant)
	wikiLandmarks, _ := s.fetchWikipediaLandmarks(cityName, lat, lon)

	// B. Query Overpass if available
	overpassLandmarks, _ := s.fetchOverpassLandmarksAround(cityName, lat, lon)

	// Merge unique names
	seen := make(map[string]bool)
	var realNames []string

	for _, name := range wikiLandmarks {
		norm := strings.ToLower(name)
		if !seen[norm] && len(name) >= 3 && len(name) <= 35 && !strings.Contains(norm, "estação meteorológica") {
			seen[norm] = true
			realNames = append(realNames, name)
		}
	}
	for _, name := range overpassLandmarks {
		norm := strings.ToLower(name)
		if !seen[norm] && len(name) >= 3 && len(name) <= 35 {
			seen[norm] = true
			realNames = append(realNames, name)
		}
	}

	preset := resolveCityPreset(cityName)

	// Ensure top 4 candidates are famous landmarks
	var candidates []string
	for _, name := range realNames {
		if !strings.EqualFold(name, cityName) {
			candidates = append(candidates, name)
			if len(candidates) >= 4 {
				break
			}
		}
	}
	if len(candidates) < 4 {
		for _, c := range preset.candidates {
			if !seen[strings.ToLower(c)] {
				candidates = append(candidates, c)
				if len(candidates) >= 4 {
					break
				}
			}
		}
	}
	if len(candidates) < 4 {
		candidates = preset.candidates[:]
	}

	// Build 22 properties + 4 stations + 2 utilities
	var landmarks []LandmarkItem
	propCount := 0
	for _, name := range realNames {
		if propCount >= 22 {
			break
		}
		landmarks = append(landmarks, LandmarkItem{
			Name:      name,
			Type:      "property",
			ImageKind: ClassifyImageKind(name, TileProperty),
		})
		propCount++
	}

	// Fill remaining properties from preset
	for _, prop := range preset.props {
		if propCount >= 22 {
			break
		}
		if !seen[strings.ToLower(prop)] {
			landmarks = append(landmarks, LandmarkItem{
				Name:      prop,
				Type:      "property",
				ImageKind: ClassifyImageKind(prop, TileProperty),
			})
			propCount++
		}
	}

	// Add 4 stations
	for _, st := range preset.stations {
		landmarks = append(landmarks, LandmarkItem{
			Name:      st,
			Type:      "station",
			ImageKind: "station",
		})
	}

	// Add 2 utilities
	for _, ut := range preset.utilities {
		landmarks = append(landmarks, LandmarkItem{
			Name:      ut,
			Type:      "utility",
			ImageKind: "utility",
		})
	}

	if len(candidates) == 4 && len(landmarks) >= 28 {
		return &CityLandmarksResult{
			City:       cityName,
			Candidates: candidates[:4],
			Landmarks:  landmarks,
		}, nil
	}

	return nil, fmt.Errorf("insufficient real landmarks retrieved")
}

func (s *PlacesService) fetchWikipediaLandmarks(cityName string, lat, lon float64) ([]string, error) {
	endpoint := fmt.Sprintf("https://pt.wikipedia.org/w/api.php?action=query&list=geosearch&gscoord=%.4f|%.4f&gsradius=15000&gslimit=30&format=json", lat, lon)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DetetiveIABanco3D/0.7 (contact@detetive-ia.local)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia returned status %d", resp.StatusCode)
	}

	var data struct {
		Query struct {
			Geosearch []struct {
				Title string `json:"title"`
			} `json:"geosearch"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var titles []string
	for _, item := range data.Query.Geosearch {
		t := strings.TrimSpace(item.Title)
		// Clean up common parenthetical disclaimers (e.g. "TV Cidade (Londrina)" -> "TV Cidade")
		if idx := strings.Index(t, "("); idx > 0 {
			t = strings.TrimSpace(t[:idx])
		}
		if len(t) >= 4 && len(t) <= 32 && !strings.EqualFold(t, cityName) {
			titles = append(titles, t)
		}
	}

	return titles, nil
}

func (s *PlacesService) fetchOverpassLandmarksAround(cityName string, lat, lon float64) ([]string, error) {
	query := fmt.Sprintf(`[out:json][timeout:3];
(
  node(around:10000,%.4f,%.4f)["tourism"~"attraction|museum|artwork|gallery"]["name"];
  node(around:10000,%.4f,%.4f)["historic"]["name"];
  node(around:10000,%.4f,%.4f)["leisure"="park"]["name"];
  way(around:10000,%.4f,%.4f)["highway"~"primary|secondary|pedestrian"]["name"];
  way(around:10000,%.4f,%.4f)["leisure"="park"]["name"];
);
out tags 25;`, lat, lon, lat, lon, lat, lon, lat, lon, lat, lon)

	endpoint := "https://overpass-api.de/api/interpreter?data=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DetetiveIABanco3D/0.7 (contact@detetive-ia.local)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass returned status %d", resp.StatusCode)
	}

	var data struct {
		Elements []struct {
			Tags map[string]string `json:"tags"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var names []string
	for _, el := range data.Elements {
		if name, ok := el.Tags["name"]; ok && len(name) >= 4 && len(name) <= 32 {
			names = append(names, name)
		}
	}

	return names, nil
}

func (s *PlacesService) fallbackLandmarks(cityName string) *CityLandmarksResult {
	cityName = strings.TrimSpace(cityName)
	if cityName == "" {
		cityName = "Curitiba"
	}
	preset := resolveCityPreset(cityName)

	var landmarks []LandmarkItem
	for _, prop := range preset.props {
		landmarks = append(landmarks, LandmarkItem{
			Name:      prop,
			Type:      "property",
			ImageKind: ClassifyImageKind(prop, TileProperty),
		})
	}
	for _, st := range preset.stations {
		landmarks = append(landmarks, LandmarkItem{
			Name:      st,
			Type:      "station",
			ImageKind: "station",
		})
	}
	for _, ut := range preset.utilities {
		landmarks = append(landmarks, LandmarkItem{
			Name:      ut,
			Type:      "utility",
			ImageKind: "utility",
		})
	}

	return &CityLandmarksResult{
		City:       cityName,
		Candidates: preset.candidates[:],
		Landmarks:  landmarks,
	}
}
