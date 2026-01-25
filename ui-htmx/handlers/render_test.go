package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Mock templ components for testing
func mockFullPage(content string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<html><body>" + content + "</body></html>"))
		return err
	})
}

func mockPartial(content string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<div>" + content + "</div>"))
		return err
	})
}

func TestRender(t *testing.T) {
	tests := []struct {
		name              string
		hxRequest         string
		historyRestore    string
		expectedContent   string
		expectedPushURL   string
		expectPushHeader  bool
	}{
		{
			name:              "Full page request",
			hxRequest:         "",
			historyRestore:    "",
			expectedContent:   "<html><body>test content</body></html>",
			expectedPushURL:   "",
			expectPushHeader:  false,
		},
		{
			name:              "HTMX request returns partial",
			hxRequest:         "true",
			historyRestore:    "",
			expectedContent:   "<div>test content</div>",
			expectedPushURL:   "/test",
			expectPushHeader:  true,
		},
		{
			name:              "HTMX history restore returns partial without push",
			hxRequest:         "true",
			historyRestore:    "true",
			expectedContent:   "<div>test content</div>",
			expectedPushURL:   "",
			expectPushHeader:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}
			if tt.historyRestore != "" {
				req.Header.Set(HXHistoryRestoreRequest, tt.historyRestore)
			}

			w := httptest.NewRecorder()

			fullPage := mockFullPage("test content")
			partial := mockPartial("test content")

			err := Render(w, req, fullPage, partial)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// Check content type
			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/html") {
				t.Errorf("Content-Type = %v, want text/html", contentType)
			}

			// Check response body
			body := w.Body.String()
			if body != tt.expectedContent {
				t.Errorf("Response body = %v, want %v", body, tt.expectedContent)
			}

			// Check HX-Push-Url header
			pushURL := w.Header().Get(HXPushURL)
			if tt.expectPushHeader {
				if pushURL != tt.expectedPushURL {
					t.Errorf("HX-Push-Url = %v, want %v", pushURL, tt.expectedPushURL)
				}
			} else {
				if pushURL != "" {
					t.Errorf("HX-Push-Url = %v, want empty", pushURL)
				}
			}
		})
	}
}

func TestRenderPartial(t *testing.T) {
	tests := []struct {
		name            string
		hxRequest       string
		expectedContent string
	}{
		{
			name:            "Always renders partial for HTMX request",
			hxRequest:       "true",
			expectedContent: "<div>partial content</div>",
		},
		{
			name:            "Always renders partial for non-HTMX request",
			hxRequest:       "",
			expectedContent: "<div>partial content</div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}

			w := httptest.NewRecorder()
			partial := mockPartial("partial content")

			err := RenderPartial(w, req, partial)
			if err != nil {
				t.Fatalf("RenderPartial() error = %v", err)
			}

			body := w.Body.String()
			if body != tt.expectedContent {
				t.Errorf("Response body = %v, want %v", body, tt.expectedContent)
			}
		})
	}
}

func TestRenderFullPage(t *testing.T) {
	tests := []struct {
		name            string
		hxRequest       string
		expectedContent string
	}{
		{
			name:            "Always renders full page for HTMX request",
			hxRequest:       "true",
			expectedContent: "<html><body>full page content</body></html>",
		},
		{
			name:            "Always renders full page for non-HTMX request",
			hxRequest:       "",
			expectedContent: "<html><body>full page content</body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}

			w := httptest.NewRecorder()
			fullPage := mockFullPage("full page content")

			err := RenderFullPage(w, req, fullPage)
			if err != nil {
				t.Fatalf("RenderFullPage() error = %v", err)
			}

			body := w.Body.String()
			if body != tt.expectedContent {
				t.Errorf("Response body = %v, want %v", body, tt.expectedContent)
			}
		})
	}
}

func TestSetHXPushURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedURL string
	}{
		{
			name:        "Set push URL",
			url:         "/test/path",
			expectedURL: "/test/path",
		},
		{
			name:        "Set false to prevent push",
			url:         "false",
			expectedURL: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetHXPushURL(w, tt.url)

			pushURL := w.Header().Get(HXPushURL)
			if pushURL != tt.expectedURL {
				t.Errorf("HX-Push-Url = %v, want %v", pushURL, tt.expectedURL)
			}
		})
	}
}

func TestSetHXRedirect(t *testing.T) {
	w := httptest.NewRecorder()
	SetHXRedirect(w, "/redirect/path")

	redirect := w.Header().Get(HXRedirect)
	if redirect != "/redirect/path" {
		t.Errorf("HX-Redirect = %v, want /redirect/path", redirect)
	}
}

func TestSetHXRefresh(t *testing.T) {
	tests := []struct {
		name     string
		refresh  bool
		expected string
	}{
		{
			name:     "Set refresh true",
			refresh:  true,
			expected: "true",
		},
		{
			name:     "Set refresh false",
			refresh:  false,
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetHXRefresh(w, tt.refresh)

			refresh := w.Header().Get(HXRefresh)
			if refresh != tt.expected {
				t.Errorf("HX-Refresh = %v, want %v", refresh, tt.expected)
			}
		})
	}
}

func TestSetHXTrigger(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		expected string
	}{
		{
			name:     "Simple event",
			event:    "myEvent",
			expected: "myEvent",
		},
		{
			name:     "Event with JSON data",
			event:    `{"myEvent": {"key": "value"}}`,
			expected: `{"myEvent": {"key": "value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetHXTrigger(w, tt.event)

			trigger := w.Header().Get(HXTriggerResponse)
			if trigger != tt.expected {
				t.Errorf("HX-Trigger = %v, want %v", trigger, tt.expected)
			}
		})
	}
}

func TestSetHXTriggerAfterSettle(t *testing.T) {
	w := httptest.NewRecorder()
	SetHXTriggerAfterSettle(w, "settleEvent")

	trigger := w.Header().Get("HX-Trigger-After-Settle")
	if trigger != "settleEvent" {
		t.Errorf("HX-Trigger-After-Settle = %v, want settleEvent", trigger)
	}
}

func TestSetHXTriggerAfterSwap(t *testing.T) {
	w := httptest.NewRecorder()
	SetHXTriggerAfterSwap(w, "swapEvent")

	trigger := w.Header().Get("HX-Trigger-After-Swap")
	if trigger != "swapEvent" {
		t.Errorf("HX-Trigger-After-Swap = %v, want swapEvent", trigger)
	}
}

func TestSetHXRetarget(t *testing.T) {
	w := httptest.NewRecorder()
	SetHXRetarget(w, "#new-target")

	retarget := w.Header().Get("HX-Retarget")
	if retarget != "#new-target" {
		t.Errorf("HX-Retarget = %v, want #new-target", retarget)
	}
}

func TestSetHXReswap(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
	}{
		{
			name:     "innerHTML strategy",
			strategy: "innerHTML",
		},
		{
			name:     "outerHTML strategy",
			strategy: "outerHTML",
		},
		{
			name:     "beforebegin strategy",
			strategy: "beforebegin",
		},
		{
			name:     "afterbegin strategy",
			strategy: "afterbegin",
		},
		{
			name:     "beforeend strategy",
			strategy: "beforeend",
		},
		{
			name:     "afterend strategy",
			strategy: "afterend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SetHXReswap(w, tt.strategy)

			reswap := w.Header().Get("HX-Reswap")
			if reswap != tt.strategy {
				t.Errorf("HX-Reswap = %v, want %v", reswap, tt.strategy)
			}
		})
	}
}

func TestRenderWithMiddleware(t *testing.T) {
	// Test that Render works correctly when used with HTMXMiddleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fullPage := mockFullPage("test content")
		partial := mockPartial("test content")
		err := Render(w, r, fullPage, partial)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
	})

	wrapped := HTMXMiddleware(handler)

	tests := []struct {
		name            string
		hxRequest       string
		expectedContent string
	}{
		{
			name:            "HTMX request via middleware",
			hxRequest:       "true",
			expectedContent: "<div>test content</div>",
		},
		{
			name:            "Non-HTMX request via middleware",
			hxRequest:       "",
			expectedContent: "<html><body>test content</body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}

			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)

			body := w.Body.String()
			if body != tt.expectedContent {
				t.Errorf("Response body = %v, want %v", body, tt.expectedContent)
			}
		})
	}
}
