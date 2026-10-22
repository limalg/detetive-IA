package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"detetive-ia/web"
)

func TestBanco3DRouteAndAssets(t *testing.T) {
	// 1. Verify embedded assets in banco3d
	files := []string{
		"banco3d/index.html",
		"banco3d/css/style.css",
		"banco3d/js/app.js",
	}
	for _, f := range files {
		rc, err := web.Assets.Open(f)
		if err != nil {
			t.Fatalf("failed to open embedded %s: %v", f, err)
		}
		rc.Close()
	}

	// 2. Test FileServer routing for /banco/
	handler := web.FileServer()

	// Test /banco/
	req := httptest.NewRequest("GET", "/banco/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /banco/, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Banco Imobiliário") || !strings.Contains(body, "css/style.css") {
		t.Errorf("unexpected body content for /banco/: %s", body)
	}

	// Test /banco/css/style.css
	reqCSS := httptest.NewRequest("GET", "/banco/css/style.css", nil)
	rrCSS := httptest.NewRecorder()
	handler.ServeHTTP(rrCSS, reqCSS)
	if rrCSS.Code != http.StatusOK {
		t.Fatalf("expected 200 for /banco/css/style.css, got %d", rrCSS.Code)
	}

	// Test /banco/js/app.js
	reqJS := httptest.NewRequest("GET", "/banco/js/app.js", nil)
	rrJS := httptest.NewRecorder()
	handler.ServeHTTP(rrJS, reqJS)
	if rrJS.Code != http.StatusOK {
		t.Fatalf("expected 200 for /banco/js/app.js, got %d", rrJS.Code)
	}

	// Test root / still serves Detetive IA
	reqRoot := httptest.NewRequest("GET", "/", nil)
	rrRoot := httptest.NewRecorder()
	handler.ServeHTTP(rrRoot, reqRoot)
	if rrRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", rrRoot.Code)
	}
	if !strings.Contains(rrRoot.Body.String(), "auth-section") {
		t.Errorf("root / is not serving Detetive IA")
	}
}
