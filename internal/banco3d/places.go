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

// PlacesService handles geocoding, autocomplete and landmark extraction.
type PlacesService struct {
	mu           sync.Mutex
	client       *http.Client
	lastReqTime  time.Time
	suggestCache map[string][]PlaceSuggestion
	landmarkCache map[string]*CityLandmarksResult
}

// GlobalPlacesService is the singleton geocoding and landmark provider.
var GlobalPlacesService = NewPlacesService()

func NewPlacesService() *PlacesService {
	return &PlacesService{
		client: &http.Client{
			Timeout: 3500 * time.Millisecond,
		},
		suggestCache:  make(map[string][]PlaceSuggestion),
		landmarkCache: make(map[string]*CityLandmarksResult),
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
		PlaceID     int64  `json:"place_id"`
		DisplayName string `json:"display_name"`
		Address     struct {
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			Municipality string `json:"municipality"`
			State       string `json:"state"`
			Country     string `json:"country"`
		} `json:"address"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	var suggestions []PlaceSuggestion
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

		suggestions = append(suggestions, PlaceSuggestion{
			ID:          fmt.Sprintf("osm_%d", r.PlaceID),
			Name:        cityName,
			Description: desc,
		})
	}

	return suggestions, nil
}

func (s *PlacesService) filterOfflineCatalog(query string) []PlaceSuggestion {
	q := strings.ToLower(strings.TrimSpace(query))
	all := []PlaceSuggestion{
		{ID: "pl_curitiba", Name: "Curitiba", Description: "Paraná, Brasil"},
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

// GetLandmarks returns 4 candidates and landmarks for a city, falling back to catalog.
func (s *PlacesService) GetLandmarks(cityName string, placeID string) (*CityLandmarksResult, error) {
	norm := strings.ToLower(strings.TrimSpace(cityName))
	if norm == "" {
		norm = "curitiba"
	}

	s.mu.Lock()
	if cached, ok := s.landmarkCache[norm]; ok {
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	// Try fetching from Overpass API or Nominatim POI if placeID is present
	if strings.HasPrefix(placeID, "osm_") {
		remoteRes, err := s.fetchOverpassLandmarks(cityName)
		if err == nil && remoteRes != nil && len(remoteRes.Candidates) == 4 {
			s.mu.Lock()
			s.landmarkCache[norm] = remoteRes
			s.mu.Unlock()
			return remoteRes, nil
		}
	}

	// Fallback to city presets
	res := s.fallbackLandmarks(cityName)
	s.mu.Lock()
	s.landmarkCache[norm] = res
	s.mu.Unlock()
	return res, nil
}

func (s *PlacesService) fetchOverpassLandmarks(cityName string) (*CityLandmarksResult, error) {
	// Query Overpass for prominent tourism / amenity / highway in city bbox
	query := fmt.Sprintf(`[out:json][timeout:3];
area["name"="%s"]["boundary"="administrative"]->.a;
(
  node(area.a)["tourism"~"attraction|artwork|gallery|museum"];
  way(area.a)["highway"~"primary|secondary|pedestrian"]["name"];
);
out tags 20;`, cityName)

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
		if name, ok := el.Tags["name"]; ok && len(name) > 3 && len(name) < 28 {
			names = append(names, name)
		}
	}

	if len(names) < 4 {
		return nil, fmt.Errorf("not enough landmarks from overpass")
	}

	candidates := names[:4]
	var landmarks []LandmarkItem
	for _, name := range names {
		landmarks = append(landmarks, LandmarkItem{
			Name:      name,
			Type:      "property",
			ImageKind: ClassifyImageKind(name, TileProperty),
		})
	}

	return &CityLandmarksResult{
		City:       cityName,
		Candidates: candidates,
		Landmarks:  landmarks,
	}, nil
}

func (s *PlacesService) fallbackLandmarks(cityName string) *CityLandmarksResult {
	norm := strings.ToLower(cityName)
	var preset cityPreset
	switch {
	case strings.Contains(norm, "curitiba"):
		preset = curitibaPreset()
	case strings.Contains(norm, "são paulo") || strings.Contains(norm, "sao paulo") || strings.Contains(norm, "sp"):
		preset = saoPauloPreset()
	case strings.Contains(norm, "rio"):
		preset = rioPreset()
	case strings.Contains(norm, "belo horizonte") || strings.Contains(norm, "bh"):
		preset = beloHorizontePreset()
	case strings.Contains(norm, "salvador"):
		preset = salvadorPreset()
	case strings.Contains(norm, "nova york") || strings.Contains(norm, "new york") || strings.Contains(norm, "nyc"):
		preset = newYorkPreset()
	case strings.Contains(norm, "paris"):
		preset = parisPreset()
	default:
		preset = proceduralPreset(cityName)
	}

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
