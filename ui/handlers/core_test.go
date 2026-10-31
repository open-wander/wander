package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestHandlerCreation tests basic handler creation without templates
func TestHandlerCreation(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Test Handler creation
	h := NewHandler(logger)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.Logger() == nil {
		t.Error("Handler logger is nil")
	}

	// Test StaticHandler creation
	static := NewStaticHandler(h)
	if static == nil {
		t.Fatal("NewStaticHandler returned nil")
	}

	// Test HomeHandler creation
	home := NewHomeHandler(h)
	if home == nil {
		t.Fatal("NewHomeHandler returned nil")
	}

	// Test Router creation
	router := NewRouter(h)
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
	if router.Handler() == nil {
		t.Error("Router Handler() returned nil")
	}
}

// TestStaticHandler404 tests static handler returns 404 for missing files
func TestStaticHandler404(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	static := NewStaticHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent.css", nil)
	w := httptest.NewRecorder()

	static.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestStaticHandlerMethodNotAllowed tests static handler rejects non-GET/HEAD requests
func TestStaticHandlerMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	static := NewStaticHandler(h)

	req := httptest.NewRequest(http.MethodPost, "/test.css", nil)
	w := httptest.NewRecorder()

	static.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
