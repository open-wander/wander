package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestRenderNotFound tests the 404 error page rendering
func TestRenderNotFound(t *testing.T) {
	logger := hclog.NewNullLogger()
	handler := NewHandler(logger)

	tests := []struct {
		name        string
		htmx        bool
		err         NotFoundError
		wantStatus  int
		wantContent []string
	}{
		{
			name: "full page error",
			htmx: false,
			err: NotFoundError{
				Message:     "Test page not found",
				RequestPath: "/ui/test/path",
				Suggestions: []string{"Go back", "Try again"},
			},
			wantStatus: http.StatusNotFound,
			wantContent: []string{
				"404 Not Found",
				"Test page not found",
				"/ui/test/path",
				"Go back",
				"Try again",
				"Return to Home",
			},
		},
		{
			name: "htmx partial error",
			htmx: true,
			err: NotFoundError{
				Message:     "Test partial not found",
				RequestPath: "/ui/test/partial",
				Suggestions: []string{"Suggestion 1"},
			},
			wantStatus: http.StatusNotFound,
			wantContent: []string{
				"Page Not Found",
				"Test partial not found",
				"/ui/test/partial",
				"Suggestion 1",
				"Return to Home",
			},
		},
		{
			name: "default message",
			htmx: false,
			err: NotFoundError{
				RequestPath: "/ui/default",
			},
			wantStatus: http.StatusNotFound,
			wantContent: []string{
				"404 Not Found",
				"The page you are looking for could not be found",
				"/ui/default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.err.RequestPath, nil)
			if tt.htmx {
				req.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()

			handler.RenderNotFound(w, req, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			body := w.Body.String()
			for _, content := range tt.wantContent {
				if !strings.Contains(body, content) {
					t.Errorf("Response body should contain %q, but doesn't.\nBody: %s", content, body)
				}
			}
		})
	}
}

// TestNotFoundError_FullPage tests full page rendering
func TestNotFoundError_FullPage(t *testing.T) {
	logger := hclog.NewNullLogger()
	handler := NewHandler(logger)

	req := httptest.NewRequest(http.MethodGet, "/ui/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.RenderNotFound(w, req, NotFoundError{
		Message:     "Custom not found message",
		RequestPath: "/ui/nonexistent",
		Suggestions: []string{
			`Go to <a href="/ui/jobs" class="underline">Jobs</a>`,
			`Return to <a href="/ui/" class="underline">Home</a>`,
		},
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	body := w.Body.String()

	// Check for HTML structure
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Response should contain full HTML page")
	}

	// Check for custom content
	if !strings.Contains(body, "Custom not found message") {
		t.Error("Response should contain custom message")
	}

	// Check for suggestions with HTML links
	if !strings.Contains(body, `<a href="/ui/jobs" class="underline">Jobs</a>`) {
		t.Error("Response should contain HTML link in suggestions")
	}
}

// TestNotFoundError_HTMXPartial tests HTMX partial rendering
func TestNotFoundError_HTMXPartial(t *testing.T) {
	logger := hclog.NewNullLogger()
	handler := NewHandler(logger)

	req := httptest.NewRequest(http.MethodGet, "/ui/partial", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	handler.RenderNotFound(w, req, NotFoundError{
		Message:     "Partial not found",
		RequestPath: "/ui/partial",
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	body := w.Body.String()

	// Should not contain full HTML structure for HTMX requests
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX response should not contain full HTML page structure")
	}

	// Should contain partial content
	if !strings.Contains(body, "Partial not found") {
		t.Error("Response should contain error message")
	}

	// Should contain error styling
	if !strings.Contains(body, "bg-yellow-50") {
		t.Error("Response should contain error styling classes")
	}
}
