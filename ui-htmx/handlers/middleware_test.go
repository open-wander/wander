package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsHTMXRequest(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{
			name:     "HTMX request",
			header:   "true",
			expected: true,
		},
		{
			name:     "Non-HTMX request",
			header:   "",
			expected: false,
		},
		{
			name:     "Invalid header value",
			header:   "false",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set(HXRequest, tt.header)
			}

			result := IsHTMXRequest(req)
			if result != tt.expected {
				t.Errorf("IsHTMXRequest() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWantsHistoryPush(t *testing.T) {
	tests := []struct {
		name           string
		hxRequest      string
		historyRestore string
		expected       bool
	}{
		{
			name:           "HTMX request without history restore",
			hxRequest:      "true",
			historyRestore: "",
			expected:       true,
		},
		{
			name:           "HTMX request with history restore",
			hxRequest:      "true",
			historyRestore: "true",
			expected:       false,
		},
		{
			name:           "Non-HTMX request",
			hxRequest:      "",
			historyRestore: "",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}
			if tt.historyRestore != "" {
				req.Header.Set(HXHistoryRestoreRequest, tt.historyRestore)
			}

			result := WantsHistoryPush(req)
			if result != tt.expected {
				t.Errorf("WantsHistoryPush() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetCurrentURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL present",
			url:      "http://example.com/path",
			expected: "http://example.com/path",
		},
		{
			name:     "URL absent",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.url != "" {
				req.Header.Set(HXCurrentURL, tt.url)
			}

			result := GetCurrentURL(req)
			if result != tt.expected {
				t.Errorf("GetCurrentURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHTMXMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		hxRequest  string
		currentURL string
		target     string
		trigger    string
	}{
		{
			name:       "HTMX request with full context",
			hxRequest:  "true",
			currentURL: "http://example.com/page",
			target:     "content-div",
			trigger:    "submit-btn",
		},
		{
			name:      "Non-HTMX request",
			hxRequest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler that checks the context
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				isHTMX := IsHTMXRequestFromContext(r.Context())
				expectedHTMX := tt.hxRequest == "true"
				if isHTMX != expectedHTMX {
					t.Errorf("Context isHTMX = %v, want %v", isHTMX, expectedHTMX)
				}

				if tt.currentURL != "" {
					url, ok := GetCurrentURLFromContext(r.Context())
					if !ok || url != tt.currentURL {
						t.Errorf("Context currentURL = %v (ok=%v), want %v", url, ok, tt.currentURL)
					}
				}

				if tt.target != "" {
					target, ok := GetTargetFromContext(r.Context())
					if !ok || target != tt.target {
						t.Errorf("Context target = %v (ok=%v), want %v", target, ok, tt.target)
					}
				}
			})

			// Wrap with middleware
			wrapped := HTMXMiddleware(handler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.hxRequest != "" {
				req.Header.Set(HXRequest, tt.hxRequest)
			}
			if tt.currentURL != "" {
				req.Header.Set(HXCurrentURL, tt.currentURL)
			}
			if tt.target != "" {
				req.Header.Set(HXTarget, tt.target)
			}
			if tt.trigger != "" {
				req.Header.Set(HXTrigger, tt.trigger)
			}

			// Execute
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		})
	}
}

func TestGetHTMXInfo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HXRequest, "true")
	req.Header.Set(HXCurrentURL, "http://example.com")
	req.Header.Set(HXTarget, "target-id")
	req.Header.Set(HXTrigger, "trigger-id")
	req.Header.Set(HXTriggerName, "trigger-name")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := GetHTMXInfo(r.Context())

		if !info.IsHTMX {
			t.Error("Expected IsHTMX to be true")
		}
		if info.CurrentURL != "http://example.com" {
			t.Errorf("CurrentURL = %v, want http://example.com", info.CurrentURL)
		}
		if info.Target != "target-id" {
			t.Errorf("Target = %v, want target-id", info.Target)
		}
		if info.Trigger != "trigger-id" {
			t.Errorf("Trigger = %v, want trigger-id", info.Trigger)
		}
		if info.TriggerName != "trigger-name" {
			t.Errorf("TriggerName = %v, want trigger-name", info.TriggerName)
		}
	})

	wrapped := HTMXMiddleware(handler)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
}
