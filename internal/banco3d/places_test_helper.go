package banco3d

import (
	"net/http"
	"time"
)

// SetHTTPClient configures the underlying HTTP client for PlacesService (used for mock isolation in tests).
func (s *PlacesService) SetHTTPClient(client *http.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
}

// SetMinGap configures rate limiting min gap for testing.
func (s *PlacesService) SetMinGap(d time.Duration) {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	s.minGap = d
}
