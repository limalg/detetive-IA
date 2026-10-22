package banco3d

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
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
	rateMu        sync.Mutex
	client        *http.Client // injectable (tests swap its Transport)
	lastReqTime   time.Time
	minGap        time.Duration
	diskPath      string
	suggestCache  map[string][]PlaceSuggestion
	landmarkCache map[string]*CityLandmarksResult
	detailsCache  map[string]PlaceDetails
}

// GlobalPlacesService is the singleton geocoding and landmark provider.
var GlobalPlacesService = NewPlacesService().WithDiskCache(defaultCachePath())

func defaultCachePath() string {
	if p := os.Getenv("BANCO3D_CACHE_FILE"); p != "" {
		return p
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "detetive-ia", "banco3d_places.json")
	}
	return ""
}

func NewPlacesService() *PlacesService {
	return &PlacesService{
		client:        &http.Client{}, // per-request context timeouts
		minGap:        time.Second,
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
	s.throttle()
	ctx, cancel := context.WithTimeout(context.Background(), nominatimTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

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
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Address     struct {
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

// resolvePlaceID maps place_id (e.g. osm_12024671 or pl_londrina) to the real city name and lat/lon.
func (s *PlacesService) resolvePlaceID(placeID string) (string, float64, float64, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return "", 0, 0, fmt.Errorf("empty place_id")
	}

	s.mu.Lock()
	d, ok := s.detailsCache[placeID]
	s.mu.Unlock()
	if ok && d.City != "" {
		return d.City, d.Lat, d.Lon, nil
	}

	if name, lat, lon, ok := offlinePlace(placeID); ok {
		return name, lat, lon, nil
	}

	if numID, ok := strings.CutPrefix(placeID, "osm_"); ok {
		name, lat, lon, err := s.fetchNominatimDetails(numID)
		if err != nil {
			return "", 0, 0, err
		}
		s.mu.Lock()
		s.detailsCache[placeID] = PlaceDetails{City: name, Lat: lat, Lon: lon}
		s.saveDiskLocked()
		s.mu.Unlock()
		return name, lat, lon, nil
	}
	return "", 0, 0, fmt.Errorf("place_id %s not resolved", placeID)
}

// fetchNominatimDetails resolves a Nominatim place_id to its name and centroid.
func (s *PlacesService) fetchNominatimDetails(numID string) (string, float64, float64, error) {
	var details struct {
		LocalName string            `json:"localname"`
		Names     map[string]string `json:"names"`
		Centroid  struct {
			Coordinates [2]float64 `json:"coordinates"` // [lon, lat]
		} `json:"centroid"`
	}
	endpoint := "https://nominatim.openstreetmap.org/details?format=json&place_id=" + url.QueryEscape(numID)
	if err := s.getJSON(endpoint, nominatimTimeout, true, &details); err != nil {
		return "", 0, 0, err
	}
	name := details.LocalName
	if name == "" {
		name = details.Names["name"]
	}
	lat, lon := details.Centroid.Coordinates[1], details.Centroid.Coordinates[0]
	if name == "" || (lat == 0 && lon == 0) {
		return "", 0, 0, fmt.Errorf("nominatim place %s has no name/centroid", numID)
	}
	return name, lat, lon, nil
}

// geocodeCity resolves a typed city name to its real name and coordinates via Nominatim search (first hit).
func (s *PlacesService) geocodeCity(cityName string) (string, float64, float64, error) {
	var hits []struct {
		Name string `json:"name"`
		Lat  string `json:"lat"`
		Lon  string `json:"lon"`
	}
	endpoint := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(cityName)
	if err := s.getJSON(endpoint, nominatimTimeout, true, &hits); err != nil {
		return "", 0, 0, err
	}
	if len(hits) == 0 {
		return "", 0, 0, fmt.Errorf("city %q not found", cityName)
	}
	var lat, lon float64
	_, _ = fmt.Sscanf(hits[0].Lat, "%f", &lat)
	_, _ = fmt.Sscanf(hits[0].Lon, "%f", &lon)
	name := hits[0].Name
	if name == "" {
		name = cityName
	}
	return name, lat, lon, nil
}

// offlineCities is the offline catalog: name, lat, lon and the lookup aliases (matched by equality).
var offlineCities = []struct {
	name     string
	lat, lon float64
	aliases  []string
}{
	{"Londrina", -23.3113, -51.1595, []string{"pl_londrina", "osm_420024688", "420024688"}},
	{"Recife", -8.0578, -34.8778, []string{"pl_recife"}},
	{"Porto Alegre", -30.0346, -51.2177, []string{"pl_poa", "poa"}},
	{"Curitiba", -25.4284, -49.2733, []string{"pl_curitiba"}},
	{"São Paulo", -23.5505, -46.6333, []string{"pl_sp", "sp", "sao paulo"}},
	{"Rio de Janeiro", -22.9068, -43.1729, []string{"pl_rio", "rio"}},
	{"Brasília", -15.7975, -47.8919, []string{"pl_brasilia", "brasilia", "df"}},
	{"Fortaleza", -3.7319, -38.5267, []string{"pl_fortaleza"}},
	{"Florianópolis", -27.5954, -48.5480, []string{"pl_florianopolis", "florianopolis", "floripa"}},
	{"Belo Horizonte", -19.9167, -43.9345, []string{"pl_bh", "bh"}},
	{"Salvador", -12.9777, -38.5016, []string{"pl_salvador"}},
	{"New York", 40.7128, -74.0060, []string{"nova york", "nyc"}},
	{"Paris", 48.8566, 2.3522, nil},
}

func offlinePlace(key string) (string, float64, float64, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, c := range offlineCities {
		if k == strings.ToLower(c.name) || slices.Contains(c.aliases, k) {
			return c.name, c.lat, c.lon, true
		}
	}
	return "", 0, 0, false
}

// GetLandmarks returns 4 candidates and 28 landmarks (22 properties, 4 stations, 2 utilities) of the chosen city.
// City is always the real name of the resolved place; places never come from another municipality.
func (s *PlacesService) GetLandmarks(cityName string, placeID string) (*CityLandmarksResult, error) {
	cityName = strings.TrimSpace(cityName)
	placeID = strings.TrimSpace(placeID)

	var lat, lon float64
	if placeID != "" {
		if rCity, rLat, rLon, err := s.resolvePlaceID(placeID); err == nil {
			cityName, lat, lon = rCity, rLat, rLon
		}
	}
	if cityName == "" {
		cityName = "Curitiba"
	}
	if lat == 0 && lon == 0 {
		if oName, oLat, oLon, ok := offlinePlace(cityName); ok {
			cityName, lat, lon = oName, oLat, oLon
		} else if gName, gLat, gLon, err := s.geocodeCity(cityName); err == nil {
			cityName, lat, lon = gName, gLat, gLon
		}
	}

	cacheKey := fmt.Sprintf("%s|%.2f,%.2f", strings.ToLower(cityName), lat, lon)
	s.mu.Lock()
	cached, ok := s.landmarkCache[cacheKey]
	s.mu.Unlock()
	if ok {
		return cached, nil
	}

	var real []realPlace
	if lat != 0 || lon != 0 {
		real, _ = s.fetchOverpassPlaces(cityName, lat, lon)
	}
	res := buildLandmarks(cityName, real)
	if len(real) >= minRealPlaces { // never cache a degraded (API-failed) answer
		s.mu.Lock()
		s.landmarkCache[cacheKey] = res
		s.saveDiskLocked()
		s.mu.Unlock()
	}
	return res, nil
}

const minRealPlaces = 12

// realPlace is a named OSM feature after classification and fame scoring.
type realPlace struct {
	Name  string
	Kind  string // "property", "station", "utility"
	Score int
	Top   bool // landmark-grade (park/tourism/historic/wikipedia): eligible as vote candidate
}

func (s *PlacesService) fetchOverpassPlaces(cityName string, lat, lon float64) ([]realPlace, error) {
	// Prefer the municipality's own boundary (never a neighbouring town); fall back to a radius.
	byArea := fmt.Sprintf(`rel(around:3000,%.4f,%.4f)["boundary"="administrative"]["admin_level"~"^[5-8]$"]["name"="%s"];map_to_area->.a;`,
		lat, lon, strings.ReplaceAll(cityName, `"`, ``))
	var lastErr error
	for _, scope := range []struct{ prelude, filter string }{
		{byArea, "(area.a)"},
		{"", fmt.Sprintf("(around:6000,%.4f,%.4f)", lat, lon)},
	} {
		els, err := s.overpass(overpassQuery(scope.prelude, scope.filter))
		if err != nil {
			lastErr = err
			continue
		}
		if places := classifyElements(cityName, els); len(places) >= minRealPlaces {
			return places, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("overpass returned too few named places for %s", cityName)
	}
	return nil, lastErr
}

func overpassQuery(prelude, f string) string {
	return `[out:json][timeout:65];` + prelude + `(
way` + f + `["highway"~"^(primary|secondary|tertiary)$"]["name"];
nwr` + f + `["leisure"="park"]["name"];
nwr` + f + `["tourism"~"^(attraction|museum)$"]["name"];
nwr` + f + `["historic"]["name"];
nwr` + f + `["railway"="station"]["name"];
nwr` + f + `["amenity"="bus_station"]["name"];
nwr` + f + `["man_made"~"^(water_tower|water_works|wastewater_plant)$"]["name"];
nwr` + f + `["power"="plant"]["name"];
nwr` + f + `["place"~"^(suburb|neighbourhood)$"]["name"];
);out tags 4000;`
}

var overpassEndpoints = []string{
	"https://maps.mail.ru/osm/tools/overpass/api/interpreter", // slow but rarely saturated
	"https://overpass-api.de/api/interpreter",
}

func (s *PlacesService) overpass(query string) ([]map[string]string, error) {
	var lastErr error
	for _, ep := range overpassEndpoints {
		var data struct {
			Elements []struct {
				Tags map[string]string `json:"tags"`
			} `json:"elements"`
		}
		if err := s.getJSON(ep+"?data="+url.QueryEscape(query), overpassTimeout, false, &data); err != nil {
			lastErr = err
			continue
		}
		out := make([]map[string]string, 0, len(data.Elements))
		for _, el := range data.Elements {
			out = append(out, el.Tags)
		}
		return out, nil
	}
	return nil, lastErr
}

// classifyElements merges same-named elements (one street = many OSM ways) and scores their fame.
func classifyElements(cityName string, tagsList []map[string]string) []realPlace {
	byName := map[string]*realPlace{}
	var order []string
	for _, t := range tagsList {
		name := strings.TrimSpace(t["name"])
		key := strings.ToLower(name)
		if len(name) < 4 || len(name) > 34 || key == strings.ToLower(cityName) || (name[0] >= '0' && name[0] <= '9') || strings.HasPrefix(key, "área ") || strings.HasPrefix(key, "sistema ") {
			continue
		}
		kind, base, top := "property", 1, false
		switch {
		case t["railway"] == "station" || t["public_transport"] == "station" || t["amenity"] == "bus_station" || t["aeroway"] == "aerodrome":
			kind, base = "station", 5
		case t["man_made"] != "" || t["power"] == "plant":
			kind, base = "utility", 5
		case t["tourism"] != "" || t["historic"] != "":
			base, top = 20, true
		case t["leisure"] == "park":
			base, top = 12, true
		case t["amenity"] == "place_of_worship":
			base = 8
		case t["place"] != "":
			base = 0 // neighbourhoods only pad the board
		}
		if t["wikipedia"] != "" {
			base, top = base+50, true
		}
		if t["wikidata"] != "" {
			base, top = base+40, true
		}
		if p, ok := byName[key]; ok {
			p.Score = max(p.Score, base)
			if p.Score < base+30 { // repeated occurrences count as fame, capped
				p.Score++
			}
			continue
		}
		byName[key] = &realPlace{Name: name, Kind: kind, Score: base, Top: top}
		order = append(order, key)
	}
	places := make([]realPlace, 0, len(order))
	for _, k := range order {
		places = append(places, *byName[k])
	}
	sort.SliceStable(places, func(i, j int) bool { return places[i].Score > places[j].Score })
	return places
}

// buildLandmarks assembles 22 properties + 4 stations + 2 utilities + 4 candidates from real places,
// padding with the city's own catalog preset (offline catalog cities) or procedural names of the same city.
func buildLandmarks(cityName string, real []realPlace) *CityLandmarksResult {
	preset := proceduralPreset(cityName)
	if _, _, _, ok := offlinePlace(cityName); ok {
		preset = resolveCityPreset(cityName)
	}

	seen := map[string]bool{strings.ToLower(cityName): true}
	var props, stations, utils, tops []string
	add := func(list *[]string, name string, limit int) {
		k := strings.ToLower(name)
		if len(*list) < limit && !seen[k] {
			seen[k] = true
			*list = append(*list, name)
		}
	}
	for _, p := range real {
		switch p.Kind {
		case "station":
			add(&stations, p.Name, 4)
		case "utility":
			add(&utils, p.Name, 2)
		default:
			if p.Top && len(tops) < 4 {
				tops = append(tops, p.Name)
			}
			add(&props, p.Name, 22)
		}
	}
	for _, n := range preset.props {
		add(&props, n, 22)
	}
	for _, n := range preset.stations {
		add(&stations, n, 4)
	}
	for _, n := range preset.utilities {
		add(&utils, n, 2)
	}

	candidates := tops
	for _, n := range props { // fewer than 4 landmark-grade places: top up with the best-scored ones
		if len(candidates) >= 4 {
			break
		}
		if !slices.Contains(candidates, n) {
			candidates = append(candidates, n)
		}
	}

	var landmarks []LandmarkItem
	for _, n := range props {
		landmarks = append(landmarks, LandmarkItem{Name: n, Type: "property", ImageKind: ClassifyImageKind(n, TileProperty)})
	}
	for _, n := range stations {
		landmarks = append(landmarks, LandmarkItem{Name: n, Type: "station", ImageKind: "station"})
	}
	for _, n := range utils {
		landmarks = append(landmarks, LandmarkItem{Name: n, Type: "utility", ImageKind: "utility"})
	}
	return &CityLandmarksResult{City: cityName, Candidates: candidates, Landmarks: landmarks}
}

const (
	nominatimTimeout = 5 * time.Second
	overpassTimeout  = 70 * time.Second
)

// getJSON GETs url (context timeout, mandatory User-Agent; throttled to 1 req/s for Nominatim) and decodes JSON into out.
func (s *PlacesService) getJSON(endpoint string, timeout time.Duration, nominatim bool, out any) error {
	if nominatim {
		s.throttle()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", req.URL.Host, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

const userAgent = "DetetiveIABanco3D/0.7.2 (https://github.com/LimaEdu-7x/tabuleiro-ia)"

// throttle enforces Nominatim's 1 request per second policy across goroutines.
func (s *PlacesService) throttle() {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	if wait := s.minGap - time.Since(s.lastReqTime); wait > 0 {
		time.Sleep(wait)
	}
	s.lastReqTime = time.Now()
}

// diskCache is the on-disk snapshot of resolved places and landmark results.
type diskCache struct {
	Details   map[string]PlaceDetails         `json:"details"`
	Landmarks map[string]*CityLandmarksResult `json:"landmarks"`
}

// WithDiskCache enables persistence at path and loads any existing snapshot.
func (s *PlacesService) WithDiskCache(path string) *PlacesService {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diskPath = path
	var dc diskCache
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &dc) == nil {
		maps.Copy(s.detailsCache, dc.Details)
		maps.Copy(s.landmarkCache, dc.Landmarks)
	}
	return s
}

// saveDiskLocked writes the snapshot (caller holds s.mu). Best effort: a failed write only loses persistence.
func (s *PlacesService) saveDiskLocked() {
	if s.diskPath == "" {
		return
	}
	b, err := json.Marshal(diskCache{Details: s.detailsCache, Landmarks: s.landmarkCache})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.diskPath), 0o755)
	tmp := s.diskPath + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, s.diskPath)
	}
}
