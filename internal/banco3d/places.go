package banco3d

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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
	Partial    bool           `json:"partial"` // true when the real sources yielded fewer than 28 landmarks
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
	blockedUntil  time.Time // Nominatim 429 back-off
	minGap        time.Duration
	diskPath      string
	bgRunning     map[string]bool
	bgAttempts    map[string]int
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
		bgRunning:     map[string]bool{},
		bgAttempts:    map[string]int{},
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
	if err := s.throttle(); err != nil {
		return nil, err
	}
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
	s.backoff(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var results []struct {
		PlaceID     int64  `json:"place_id"`
		Class       string `json:"class"`
		PlaceRank   int    `json:"place_rank"`
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
		if r.PlaceRank >= 20 || r.Class == "highway" || r.Class == "building" { // streets and buildings are not cities
			continue
		}
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
		if streetLike(cityName) {
			continue
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
	s.saveDiskLocked() // lock already held; keeps place_id -> city across restarts

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

// realPlace is a named OSM feature after classification and fame scoring.
type realPlace struct {
	Name  string
	Kind  string // "property", "station", "utility"
	Score int
	Top   bool // landmark-grade (park/tourism/historic/wikipedia): eligible as vote candidate
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

// resolvePlaceID maps place_id (e.g. osm_12024671 or pl_londrina) to the real city name and lat/lon.
func (s *PlacesService) resolvePlaceID(ctx context.Context, placeID string) (string, float64, float64, error) {
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
		name, lat, lon, err := s.fetchNominatimDetails(ctx, numID)
		if err != nil && ctx.Err() == nil { // one retry: Nominatim sometimes answers 429/5xx once
			name, lat, lon, err = s.fetchNominatimDetails(ctx, numID)
		}
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
func (s *PlacesService) fetchNominatimDetails(ctx context.Context, numID string) (string, float64, float64, error) {
	var details struct {
		Category    string            `json:"category"`
		RankAddress int               `json:"rank_address"`
		LocalName   string            `json:"localname"`
		Names       map[string]string `json:"names"`
		Centroid    struct {
			Coordinates [2]float64 `json:"coordinates"` // [lon, lat]
		} `json:"centroid"`
	}
	endpoint := "https://nominatim.openstreetmap.org/details?format=json&place_id=" + url.QueryEscape(numID)
	if err := s.getJSON(ctx, endpoint, nominatimTimeout, true, &details); err != nil {
		return "", 0, 0, err
	}
	// Nominatim place_ids are not stable: an id that used to be a city may now be a street.
	if details.RankAddress >= 20 || details.Category == "highway" || details.Category == "building" {
		return "", 0, 0, fmt.Errorf("nominatim place %s is not a city (%s, rank %d)", numID, details.Category, details.RankAddress)
	}
	name := details.LocalName
	if name == "" {
		name = details.Names["name"]
	}
	if streetLike(name) {
		return "", 0, 0, fmt.Errorf("nominatim place %s resolved to a street name %q", numID, name)
	}
	lat, lon := details.Centroid.Coordinates[1], details.Centroid.Coordinates[0]
	if name == "" || (lat == 0 && lon == 0) {
		return "", 0, 0, fmt.Errorf("nominatim place %s has no name/centroid", numID)
	}
	return name, lat, lon, nil
}

// geocodeCity resolves a typed city name to its real name and coordinates via Nominatim search (first hit).
func (s *PlacesService) geocodeCity(ctx context.Context, cityName string) (string, float64, float64, error) {
	var hits []struct {
		Name string `json:"name"`
		Lat  string `json:"lat"`
		Lon  string `json:"lon"`
	}
	endpoint := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(cityName)
	if err := s.getJSON(ctx, endpoint, nominatimTimeout, true, &hits); err != nil {
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

// GetLandmarks returns up to 4 candidates and 28 landmarks (22 properties, 4 stations, 2 utilities) of the chosen city.
// City is always the real name of the resolved place. Names are never invented: when the sources yield less,
// fewer landmarks are returned and Partial is set. Optional extras are user-typed custom places: they enter
// the board first and lead the candidates.
func (s *PlacesService) GetLandmarks(cityName string, placeID string, extras ...string) (*CityLandmarksResult, error) {
	res, err := s.getLandmarks(cityName, placeID)
	if err != nil {
		return nil, err
	}
	return withExtras(res, extras), nil
}

// totalBudget caps one GetLandmarks call; sourceTimeout caps each data source inside it.
const (
	totalBudget   = 13 * time.Second // below the server WriteTimeout (15s)
	sourceTimeout = 8 * time.Second
)

func (s *PlacesService) getLandmarks(cityName string, placeID string) (*CityLandmarksResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), totalBudget)
	defer cancel()
	cityName = strings.TrimSpace(cityName)
	placeID = strings.TrimSpace(placeID)

	var lat, lon float64
	if placeID != "" {
		if rCity, rLat, rLon, err := s.resolvePlaceID(ctx, placeID); err == nil {
			cityName, lat, lon = rCity, rLat, rLon
		}
	}
	if cityName == "" {
		if placeID != "" { // never guess another city when a chosen place could not be resolved
			return nil, s.unavailable(fmt.Sprintf("place_id %s could not be resolved", placeID))
		}
		cityName = "Curitiba"
	}
	if lat == 0 && lon == 0 {
		if oName, oLat, oLon, ok := offlinePlace(cityName); ok {
			cityName, lat, lon = oName, oLat, oLon
		} else if gName, gLat, gLon, err := s.geocodeCity(ctx, cityName); err == nil {
			cityName, lat, lon = gName, gLat, gLon
		}
	}

	cacheKey := placeID
	if cacheKey == "" || lat == 0 && lon == 0 {
		cacheKey = fmt.Sprintf("%s|%.2f,%.2f", strings.ToLower(cityName), lat, lon)
	}
	s.mu.Lock()
	cached, ok := s.landmarkCache[cacheKey]
	s.mu.Unlock()
	if ok {
		if cached.Partial && (lat != 0 || lon != 0) {
			s.warm(cacheKey, cityName, lat, lon) // keep trying to complete it in the background
		}
		return cached, nil
	}

	var real []realPlace
	if lat != 0 || lon != 0 {
		real = s.fetchAllSources(ctx, cityName, lat, lon, false)
	}
	res := buildLandmarks(cityName, real)
	if res.Partial && (lat != 0 || lon != 0) {
		s.warm(cacheKey, cityName, lat, lon) // a second GET returns the completed answer
	}
	if len(res.Landmarks) == 0 { // no source answered and nothing cached: say so instead of an empty 200
		return nil, s.unavailable("no landmark source available for " + cityName)
	}
	if len(real) >= minRealPlaces { // never cache a degraded (sources-failed) answer
		s.mu.Lock()
		s.landmarkCache[cacheKey] = res
		s.saveDiskLocked()
		s.mu.Unlock()
	}
	return res, nil
}

const minRealPlaces = 12

func countKind(ps []realPlace, kind string) (n int) {
	for _, p := range ps {
		if p.Kind == kind {
			n++
		}
	}
	return
}

func countTop(ps []realPlace) (n int) {
	for _, p := range ps {
		if p.Top && p.Kind == "property" {
			n++
		}
	}
	return
}

// withExtras puts user-typed custom places first among the properties and candidates (returns a copy).
func withExtras(res *CityLandmarksResult, extras []string) *CityLandmarksResult {
	var custom []string
	seen := map[string]bool{}
	for _, e := range extras {
		e = strings.TrimSpace(e)
		if k := strings.ToLower(e); len(e) >= 2 && len(e) <= 34 && !seen[k] && len(custom) < 8 {
			seen[k] = true
			custom = append(custom, e)
		}
	}
	if len(custom) == 0 {
		return res
	}
	out := &CityLandmarksResult{City: res.City, Partial: res.Partial}
	for _, n := range custom {
		out.Landmarks = append(out.Landmarks, LandmarkItem{Name: n, Type: "property", ImageKind: ClassifyImageKind(n, TileProperty)})
	}
	props := 0
	for _, l := range res.Landmarks {
		if l.Type == "property" && (seen[strings.ToLower(l.Name)] || props+len(custom) >= 22) {
			continue
		}
		if l.Type == "property" {
			props++
		}
		out.Landmarks = append(out.Landmarks, l)
	}
	out.Candidates = slices.Clone(custom)
	for _, c := range res.Candidates {
		if !seen[strings.ToLower(c)] {
			out.Candidates = append(out.Candidates, c)
		}
	}
	out.Candidates = out.Candidates[:min(4, len(out.Candidates))]
	out.Partial = out.Partial && countProps(out.Landmarks) < 22
	return out
}

func countProps(ls []LandmarkItem) (n int) {
	for _, l := range ls {
		if l.Type == "property" {
			n++
		}
	}
	return
}

// fetchOverpassPlaces lists named features strictly inside the municipality's own boundary.
func (s *PlacesService) fetchOverpassPlaces(ctx context.Context, cityName string, lat, lon float64) []realPlace {
	byArea := fmt.Sprintf(`rel(around:3000,%.4f,%.4f)["boundary"="administrative"]["admin_level"~"^[5-8]$"]["name"="%s"];map_to_area->.a;`,
		lat, lon, strings.ReplaceAll(cityName, `"`, ``))
	for _, scope := range []struct{ prelude, filter string }{
		{byArea, "(area.a)"},
	} {
		els, err := s.overpass(ctx, overpassQuery(scope.prelude, scope.filter, overpassSecs(ctx)))
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		if places := classifyElements(cityName, els); len(places) >= minRealPlaces {
			return places
		}
	}
	return nil
}

func overpassQuery(prelude, f string, secs int) string {
	return fmt.Sprintf(`[out:json][timeout:%d];`, secs) + prelude + `(
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
	"https://maps.mail.ru/osm/tools/overpass/api/interpreter",
	"https://overpass-api.de/api/interpreter",
	"https://overpass.kumi.systems/api/interpreter",
}

// overpass races all mirrors in parallel and returns the first successful answer.
func (s *PlacesService) overpass(ctx context.Context, query string) ([]map[string]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		tags []map[string]string
		err  error
	}
	ch := make(chan result, len(overpassEndpoints))
	for _, ep := range overpassEndpoints {
		go func() {
			var data struct {
				Elements []struct {
					Tags map[string]string `json:"tags"`
				} `json:"elements"`
			}
			if err := s.getJSON(ctx, ep+"?data="+url.QueryEscape(query), srcTimeout(ctx), false, &data); err != nil {
				ch <- result{err: err}
				return
			}
			out := make([]map[string]string, 0, len(data.Elements))
			for _, el := range data.Elements {
				out = append(out, el.Tags)
			}
			ch <- result{tags: out}
		}()
	}
	var lastErr error
	for range overpassEndpoints {
		r := <-ch
		if r.err == nil {
			return r.tags, nil
		}
		lastErr = r.err
	}
	return nil, lastErr
}

// fetchWikipediaPlaces returns geotagged pt.wikipedia articles within 5 km: famous by definition.
func (s *PlacesService) fetchWikipediaPlaces(ctx context.Context, cityName string, lat, lon float64) []realPlace {
	var data struct {
		Query struct {
			Geosearch []struct {
				Title string `json:"title"`
			} `json:"geosearch"`
		} `json:"query"`
	}
	endpoint := fmt.Sprintf("https://pt.wikipedia.org/w/api.php?action=query&list=geosearch&gscoord=%.4f|%.4f&gsradius=5000&gslimit=100&format=json", lat, lon)
	if err := s.getJSON(ctx, endpoint, srcTimeout(ctx), false, &data); err != nil {
		return nil
	}
	var out []realPlace
	for _, g := range data.Query.Geosearch {
		t := strings.TrimSpace(g.Title)
		if i := strings.Index(t, " ("); i > 0 { // "Praça X (Londrina)" -> "Praça X"; "(Duque de Caxias)" is another municipality
			if !strings.EqualFold(strings.Trim(t[i:], " ()"), cityName) {
				continue
			}
			t = t[:i]
		}
		if len(t) < 4 || len(t) > 34 || strings.EqualFold(t, cityName) || isMediaOrClub(t) {
			continue
		}
		if k := wikiInfraKind(t); k != "" { // stations and utilities come from the same geosearch
			out = append(out, realPlace{Name: t, Kind: k, Score: 8})
			continue
		}
		score, top := 20, false
		if isPlaceLike(t) { // only real landmarks may lead the vote candidates
			score, top = 60, true
		} else if !strings.Contains(t, " ") {
			continue // a lone word is usually a neighbouring municipality or a suburb
		}
		out = append(out, realPlace{Name: t, Kind: "property", Score: score, Top: top})
	}
	return out
}

// fetchAllSources queries the city sources in parallel (Overpass municipality, Wikipedia geosearch pt,
// Nominatim city searches), merges them by fame. Never looks outside the municipality. full=true is the background pass:
// every Nominatim query.
func (s *PlacesService) fetchAllSources(ctx context.Context, cityName string, lat, lon float64, full bool) []realPlace {
	var wg sync.WaitGroup
	results := make([][]realPlace, 3)
	for i, fn := range []func() []realPlace{
		func() []realPlace { return s.fetchOverpassPlaces(ctx, cityName, lat, lon) },
		func() []realPlace { return s.fetchWikipediaPlaces(ctx, cityName, lat, lon) },
		func() []realPlace { return s.fetchNominatimPlaces(ctx, cityName, lat, lon, full) },
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = fn()
		}()
	}
	wg.Wait()

	byName := map[string]realPlace{}
	for _, r := range results {
		for _, p := range r {
			k := strings.ToLower(p.Name)
			if old, ok := byName[k]; !ok || p.Score > old.Score {
				byName[k] = p
			}
		}
	}
	merged := make([]realPlace, 0, len(byName))
	for _, p := range byName {
		merged = append(merged, p)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Score != merged[j].Score {
			return merged[i].Score > merged[j].Score
		}
		return merged[i].Name < merged[j].Name
	})

	return dropForeign(cityName, merged)
}

// dropForeign removes names that belong to another catalog city's preset (e.g. Cristo Redentor in Magé):
// only places of the chosen municipality may reach the board.
func dropForeign(cityName string, ps []realPlace) []realPlace {
	foreign := map[string]bool{}
	for _, c := range offlineCities {
		if strings.EqualFold(c.name, cityName) {
			continue
		}
		pr := resolveCityPreset(c.name)
		for _, list := range [][]string{pr.props[:], pr.stations[:], pr.utilities[:], pr.candidates[:]} {
			for _, n := range list {
				foreign[strings.ToLower(n)] = true
			}
		}
	}
	if _, _, _, ok := offlinePlace(cityName); ok {
		for _, n := range resolveCityPreset(cityName).props {
			delete(foreign, strings.ToLower(n))
		}
	}
	out := ps[:0:0]
	for _, p := range ps {
		if !foreign[strings.ToLower(p.Name)] {
			out = append(out, p)
		}
	}
	return out
}

type nominatimQuery struct {
	word  string
	kind  string // "property", "station", "utility"
	score int
	top   bool
}

type nominatimHit struct {
	Name    string            `json:"name"`
	Address map[string]string `json:"address"`
}

// nominatimQueries are Nominatim category phrases ("park in <city>"): they enumerate squares, parks,
// museums, theatres, churches and stations of the city. The first fastQueries run in the foreground
// request (1 req/s each), the rest only in the background pass.
var nominatimQueries = []nominatimQuery{
	{"museum", "property", 25, true}, {"park", "property", 14, true}, {"stadium", "property", 15, true},
	{"bus station", "station", 6, false}, {"airport", "station", 7, false},
	{"theatre", "property", 12, true}, {"railway station", "station", 5, false}, {"church", "property", 4, false},
}

const fastQueries = 5

func validPlaceName(n, cityName string) bool {
	return len(n) >= 4 && len(n) <= 34 && !strings.ContainsAny(n, ";") && !strings.EqualFold(n, cityName) && !isMediaOrClub(n)
}

// cleanName drops a trailing parenthetical: "Museu Casas Bonnet (1877)" -> "Museu Casas Bonnet".
func cleanName(n string) string {
	n = strings.TrimSpace(n)
	if i := strings.Index(n, " ("); i > 0 {
		n = n[:i]
	}
	return n
}

// shortenCity trims a too long name's trailing city: "Estação Rodoviária de Caxias do Sul" -> "Estação Rodoviária".
func shortenCity(n, cityName string) string {
	if len(n) <= 34 {
		return n
	}
	for _, sep := range []string{" de ", " do ", " da ", " - "} {
		if t, ok := strings.CutSuffix(n, sep+cityName); ok {
			return t
		}
	}
	return n
}

// notPlaceNoun rejects subdivisions and access points that merely mention a station word.
func notPlaceNoun(n string) bool {
	l := strings.ToLower(n)
	for _, w := range []string{"residencial", "vila ", "jardim ", "jd ", "condomínio", "acesso", "loteamento", "hotel"} {
		if strings.HasPrefix(l, w) {
			return true
		}
	}
	return false
}

// fetchNominatimPlaces searches by name inside the city and keeps only hits whose address municipality
// is the chosen city.
func (s *PlacesService) fetchNominatimPlaces(ctx context.Context, cityName string, lat, lon float64, full bool) []realPlace {
	queries := nominatimQueries[:fastQueries]
	if full {
		queries = nominatimQueries
	}
	var out []realPlace
	for _, q := range queries {
		var hits []nominatimHit
		endpoint := "https://nominatim.openstreetmap.org/search?format=json&addressdetails=1&limit=40&q=" + url.QueryEscape(q.word+" in "+cityName)
		if err := s.getJSON(ctx, endpoint, srcTimeout(ctx), true, &hits); err != nil {
			if ctx.Err() != nil {
				return out
			}
			continue
		}
		for _, h := range hits {
			n := shortenCity(cleanName(h.Name), cityName)
			if !inCity(h.Address, cityName) || !validPlaceName(n, cityName) || q.kind == "station" && notPlaceNoun(n) {
				continue
			}
			out = append(out, realPlace{Name: n, Kind: q.kind, Score: q.score, Top: q.top})
		}
	}
	if full && countKind(out, "property") < 22 { // Nominatim asks for restraint: sweep only when still short
		out = append(out, s.reverseSweep(ctx, cityName, lat, lon)...)
	}
	return out
}

func inCity(a map[string]string, cityName string) bool {
	return strings.EqualFold(firstNonEmpty(a["city"], a["town"], a["municipality"], a["village"]), cityName)
}

// reverseSweep reverse-geocodes a grid of points around the center (1 req/s): every street the grid
// touches is a real street of the city, and repeated hits rank the main ones higher. Suburb names
// found on the way are returned as neighbourhoods.
func (s *PlacesService) reverseSweep(ctx context.Context, cityName string, lat, lon float64) []realPlace {
	type pt struct{ dLat, dLon float64 }
	pts := []pt{{0, 0}}
	for _, ring := range []struct {
		km float64
		n  int
	}{{1.5, 6}, {4, 10}} {
		for i := 0; i < ring.n; i++ {
			a := 2 * math.Pi * float64(i) / float64(ring.n)
			pts = append(pts, pt{ring.km * math.Sin(a) / 111, ring.km * math.Cos(a) / (111 * math.Max(math.Cos(lat*math.Pi/180), 0.2))})
		}
	}
	roads := map[string]*realPlace{}
	var order []string
	for _, p := range pts {
		var hit struct {
			Address map[string]string `json:"address"`
		}
		endpoint := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&zoom=17&addressdetails=1&lat=%.5f&lon=%.5f", lat+p.dLat, lon+p.dLon)
		if err := s.getJSON(ctx, endpoint, srcTimeout(ctx), true, &hit); err != nil {
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if !inCity(hit.Address, cityName) {
			continue
		}
		for _, f := range []struct {
			key   string
			score int
		}{{"road", 3}, {"suburb", 1}, {"neighbourhood", 1}} {
			n := cleanName(hit.Address[f.key])
			if !validPlaceName(n, cityName) {
				continue
			}
			k := strings.ToLower(n)
			if r, ok := roads[k]; ok {
				r.Score = min(r.Score+1, f.score+8) // repeated hits mark a main street
				continue
			}
			roads[k] = &realPlace{Name: n, Kind: "property", Score: f.score}
			order = append(order, k)
		}
	}
	out := make([]realPlace, 0, len(order))
	for _, k := range order {
		out = append(out, *roads[k])
	}
	return out
}

type ctxKey int

const bgKey ctxKey = 0

// srcTimeout is the per-source timeout: 8s in the foreground request, 40s in the background pass.
func srcTimeout(ctx context.Context) time.Duration {
	if ctx.Value(bgKey) != nil {
		return 40 * time.Second
	}
	return sourceTimeout
}

// overpassSecs is the server-side Overpass timeout matching srcTimeout.
func overpassSecs(ctx context.Context) int { return int(srcTimeout(ctx).Seconds()) - 1 }

const (
	bgBudget      = 3 * time.Minute
	maxBGAttempts = 4
)

// warm completes a partial answer in the background (every Nominatim query, long Overpass timeouts,
// regional sweep) and replaces the cached result when it is better. At most one run per key at a time.
func (s *PlacesService) warm(cacheKey, cityName string, lat, lon float64) {
	s.mu.Lock()
	if s.bgRunning[cacheKey] || s.bgAttempts[cacheKey] >= maxBGAttempts {
		s.mu.Unlock()
		return
	}
	s.bgRunning[cacheKey] = true
	s.bgAttempts[cacheKey]++
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), bgKey, true), bgBudget)
		defer cancel()
		real := s.fetchAllSources(ctx, cityName, lat, lon, true)

		s.mu.Lock()
		defer s.mu.Unlock()
		s.bgRunning[cacheKey] = false
		// Union with what is cached: each pass can only add real places, never lose earlier ones.
		if old := s.landmarkCache[cacheKey]; old != nil {
			for i, l := range old.Landmarks {
				real = append(real, realPlace{Name: l.Name, Kind: l.Type, Score: 1000 - i, Top: slices.Contains(old.Candidates, l.Name)})
			}
		}
		sort.SliceStable(real, func(i, j int) bool { return real[i].Score > real[j].Score })
		res := buildLandmarks(cityName, real)
		if old := s.landmarkCache[cacheKey]; old == nil || len(res.Landmarks) >= len(old.Landmarks) {
			s.landmarkCache[cacheKey] = res
			s.saveDiskLocked()
		}
	}()
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// buildLandmarks assembles up to 22 properties + 4 stations + 2 utilities + 4 candidates from real places.
// Catalog cities pad with their own real preset names; any other city returns fewer landmarks and Partial.
func buildLandmarks(cityName string, real []realPlace) *CityLandmarksResult {
	var preset cityPreset
	if _, _, _, ok := offlinePlace(cityName); ok {
		preset = resolveCityPreset(cityName)
	}

	seen := map[string]bool{strings.ToLower(cityName): true}
	var props, stations, utils, tops, spare []string
	add := func(list *[]string, name string, limit int) {
		k := strings.ToLower(name)
		if name != "" && len(*list) < limit && !seen[k] {
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
			if p.Top && len(tops) < 4 && !slices.Contains(tops, p.Name) {
				tops = append(tops, p.Name)
			}
			if len(props) >= 22 && !seen[strings.ToLower(p.Name)] {
				if k := ClassifyImageKind(p.Name, TileProperty); k != "street" && k != "avenue" {
					spare = append(spare, p.Name) // real, unused: may fill empty utility slots
				}
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
	for _, n := range spare { // no utility found: any real place (mall, hospital, university...) takes the slot
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
	if candidates == nil {
		candidates = []string{}
	}
	if landmarks == nil {
		landmarks = []LandmarkItem{}
	}
	return &CityLandmarksResult{
		City:       cityName,
		Candidates: candidates,
		Landmarks:  landmarks,
		Partial:    len(props) < 22 || len(stations) < 4 || len(utils) < 2 || len(candidates) < 4,
	}
}

// getJSON GETs url (bounded by ctx and timeout, mandatory User-Agent; throttled to 1 req/s for Nominatim)
// and decodes JSON into out.
func (s *PlacesService) getJSON(ctx context.Context, endpoint string, timeout time.Duration, nominatim bool, out any) error {
	if nominatim {
		if err := s.throttle(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
	if nominatim {
		s.backoff(resp)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", req.URL.Host, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

const nominatimTimeout = 5 * time.Second

const userAgent = "DetetiveIABanco3D/0.7.2 (https://github.com/LimaEdu-7x/tabuleiro-ia)"

var errNominatimBlocked = fmt.Errorf("nominatim backing off after HTTP 429")

// throttle enforces Nominatim's 1 request per second policy across goroutines, and refuses
// (errNominatimBlocked) while a 429 back-off is active so callers fall back to cache/Wikipedia.
func (s *PlacesService) throttle() error {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	if time.Now().Before(s.blockedUntil) {
		return errNominatimBlocked
	}
	if wait := s.minGap - time.Since(s.lastReqTime); wait > 0 {
		time.Sleep(wait)
	}
	s.lastReqTime = time.Now()
	return nil
}

// backoff stops all Nominatim traffic after a 429/403 for Retry-After seconds (default 10 min).
func (s *PlacesService) backoff(resp *http.Response) {
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusForbidden {
		return
	}
	d := 10 * time.Minute
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		d = time.Duration(secs) * time.Second
	}
	s.rateMu.Lock()
	s.blockedUntil = time.Now().Add(d)
	s.rateMu.Unlock()
}

// diskSchemaVersion invalidates caches written by older builds (v<2 could hold street names as cities).
const diskSchemaVersion = 3

// diskCache is the on-disk snapshot of resolved places and landmark results.
type diskCache struct {
	Version   int                             `json:"version"`
	Details   map[string]PlaceDetails         `json:"details"`
	Landmarks map[string]*CityLandmarksResult `json:"landmarks"`
}

// WithDiskCache enables persistence at path and loads any existing snapshot.
func (s *PlacesService) WithDiskCache(path string) *PlacesService {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diskPath = path
	var dc diskCache
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &dc) == nil && dc.Version == diskSchemaVersion {
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
	b, err := json.Marshal(diskCache{Version: diskSchemaVersion, Details: s.detailsCache, Landmarks: s.landmarkCache})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.diskPath), 0o755)
	tmp := s.diskPath + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, s.diskPath)
	}
}

// isMediaOrClub drops Wikipedia hits that are broadcasters, clubs or weather stations, not places.
func isMediaOrClub(title string) bool {
	l := strings.ToLower(title)
	for _, w := range []string{"meteorológica", "rádio", "futebol clube", "esporte clube", "clube de regatas", "tv ", " tv", "cnt ", "record", "tvs", "rpc ", "futsal", " fm", "rede "} {
		if strings.Contains(l, w) || strings.HasPrefix(l, w) {
			return true
		}
	}
	return false
}

// isPlaceLike reports whether a title names a visitable landmark (museum, park, church, square...).
func isPlaceLike(title string) bool {
	l := strings.ToLower(title)
	for _, w := range []string{"museu", "parque", "praça", "catedral", "igreja", "basílica", "teatro", "memorial", "bosque", "monumento",
		"mercado", "estação", "palácio", "jardim", "biblioteca", "santuário", "mirante", "zoológico", "ópera", "forte", "casa ", "centro cultural"} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// wikiInfraKind classifies a Wikipedia title as a "station" or "utility" when it names one, else "".
func wikiInfraKind(title string) string {
	l := strings.ToLower(title)
	for _, w := range []string{"estação ferroviária", "estação rodoviária", "rodoviária", "terminal ", "aeroporto", "estação de metrô", "estação central"} {
		if strings.Contains(l, w) {
			return "station"
		}
	}
	for _, w := range []string{"usina", "hidrelétrica", "termelétrica", "subestação", "saneamento", "estação de tratamento", "companhia de", "barragem", "reservatório", "sanepar", "saae", "samae", "copel", "cpfl", "sabesp", "corsan", "celesc"} {
		if strings.Contains(l, w) {
			return "utility"
		}
	}
	return ""
}

// streetLike reports whether a name is a road ("Avenida X", "Rua Y"), never a valid city name.
func streetLike(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	for _, w := range []string{"rua ", "avenida ", "av. ", "rodovia ", "travessa ", "alameda ", "estrada ", "praça ", "largo "} {
		if strings.HasPrefix(l, w) {
			return true
		}
	}
	return false
}

// UnavailableError means no data source could answer now; the HTTP layer maps it to 503 + Retry-After.
type UnavailableError struct {
	Msg        string
	RetryAfter time.Duration
}

func (e *UnavailableError) Error() string { return e.Msg }

func (s *PlacesService) unavailable(msg string) *UnavailableError {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	wait := 30 * time.Second
	if d := time.Until(s.blockedUntil); d > wait {
		wait = d
	}
	return &UnavailableError{Msg: msg, RetryAfter: wait.Round(time.Second)}
}
