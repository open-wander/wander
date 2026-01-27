package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestSearchHandler_EmptyQuery(t *testing.T) {
	// Create a handler with no Nomad client (will use mock data)
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create request with empty query
	req := httptest.NewRequest(http.MethodGet, "/ui/search?q=", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSearchHandler_ShortQuery(t *testing.T) {
	// Create a handler with no Nomad client (will use mock data)
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create request with query too short (only 1 char)
	req := httptest.NewRequest(http.MethodGet, "/ui/search?q=a", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSearchHandler_MethodNotAllowed(t *testing.T) {
	// Create a handler
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create POST request (should only accept GET)
	req := httptest.NewRequest(http.MethodPost, "/ui/search?q=test", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
