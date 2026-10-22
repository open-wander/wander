package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestSelectionMiddlewarePrecedence(t *testing.T) {
	tests := []struct {
		name              string
		target            string
		namespaceCookie   string
		regionCookie      string
		expectedNamespace string
		expectedRegion    string
	}{
		{
			name:              "defaults",
			target:            "/ui/jobs",
			expectedNamespace: defaultNamespace,
			expectedRegion:    defaultRegion,
		},
		{
			name:              "browser cookies",
			target:            "/ui/jobs",
			namespaceCookie:   "team-cookie",
			regionCookie:      "region-cookie",
			expectedNamespace: "team-cookie",
			expectedRegion:    "region-cookie",
		},
		{
			name:              "URL overrides cookies",
			target:            "/ui/jobs?namespace=team-url&region=region-url",
			namespaceCookie:   "team-cookie",
			regionCookie:      "region-cookie",
			expectedNamespace: "team-url",
			expectedRegion:    "region-url",
		},
		{
			name:              "invalid values use defaults",
			target:            "/ui/jobs?namespace=bad%2Fvalue&region=bad%20value",
			namespaceCookie:   "also/bad",
			regionCookie:      "also bad",
			expectedNamespace: defaultNamespace,
			expectedRegion:    defaultRegion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			if test.namespaceCookie != "" {
				request.AddCookie(&http.Cookie{Name: WanderNamespaceCookie, Value: test.namespaceCookie})
			}
			if test.regionCookie != "" {
				request.AddCookie(&http.Cookie{Name: WanderRegionCookie, Value: test.regionCookie})
			}

			wrapped := SelectionMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if got := GetNamespaceFromRequest(r); got != test.expectedNamespace {
					t.Errorf("namespace = %q, want %q", got, test.expectedNamespace)
				}
				if got := GetRegionFromRequest(r); got != test.expectedRegion {
					t.Errorf("region = %q, want %q", got, test.expectedRegion)
				}
			}))
			wrapped.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func TestSelectionMiddlewareIsolatesConcurrentBrowsers(t *testing.T) {
	const browsers = 32
	results := make(chan string, browsers)
	wrapped := SelectionMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		results <- GetNamespaceFromRequest(r) + "/" + GetRegionFromRequest(r)
	}))

	var wait sync.WaitGroup
	for i := 0; i < browsers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			request.AddCookie(&http.Cookie{Name: WanderNamespaceCookie, Value: fmt.Sprintf("team-%d", index)})
			request.AddCookie(&http.Cookie{Name: WanderRegionCookie, Value: fmt.Sprintf("region-%d", index)})
			wrapped.ServeHTTP(httptest.NewRecorder(), request)
		}(i)
	}
	wait.Wait()
	close(results)

	seen := make(map[string]bool, browsers)
	for result := range results {
		seen[result] = true
	}
	for i := 0; i < browsers; i++ {
		expected := fmt.Sprintf("team-%d/region-%d", i, i)
		if !seen[expected] {
			t.Errorf("missing isolated selection %q", expected)
		}
	}
}

// TestACLMiddleware_ExtractTokenFromHeader verifies that the ACL middleware
// correctly extracts tokens from the X-Nomad-Token header.
func TestACLMiddleware_ExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		expectedToken string
	}{
		{
			name:          "valid token in header",
			headerValue:   "test-token-123",
			expectedToken: "test-token-123",
		},
		{
			name:          "empty header",
			headerValue:   "",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that checks the context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, ok := GetTokenFromContext(r.Context())
				if tt.expectedToken == "" {
					if ok {
						t.Errorf("expected no token in context, but found: %s", token)
					}
				} else {
					if !ok {
						t.Errorf("expected token in context, but none found")
						return
					}
					if token != tt.expectedToken {
						t.Errorf("expected token %q, got %q", tt.expectedToken, token)
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with ACL middleware
			handler := ACLMiddleware(testHandler)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set(XNomadToken, tt.headerValue)
			}
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestACLMiddleware_ExtractTokenFromCookie verifies that the ACL middleware
// correctly extracts tokens from the wander_token cookie.
func TestACLMiddleware_ExtractTokenFromCookie(t *testing.T) {
	tests := []struct {
		name          string
		cookieValue   string
		expectedToken string
	}{
		{
			name:          "valid token in cookie",
			cookieValue:   "cookie-token-456",
			expectedToken: "cookie-token-456",
		},
		{
			name:          "empty cookie",
			cookieValue:   "",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that checks the context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, ok := GetTokenFromContext(r.Context())
				if tt.expectedToken == "" {
					if ok {
						t.Errorf("expected no token in context, but found: %s", token)
					}
				} else {
					if !ok {
						t.Errorf("expected token in context, but none found")
						return
					}
					if token != tt.expectedToken {
						t.Errorf("expected token %q, got %q", tt.expectedToken, token)
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with ACL middleware
			handler := ACLMiddleware(testHandler)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			addWanderTokenCookie(req, tt.cookieValue)
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestACLMiddleware_HeaderPrecedence verifies that the X-Nomad-Token header
// takes precedence over the cookie when both are present.
func TestACLMiddleware_HeaderPrecedence(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := GetTokenFromContext(r.Context())
		if !ok {
			t.Error("expected token in context, but none found")
			return
		}
		// Header should take precedence
		if token != "header-token" {
			t.Errorf("expected header token to take precedence, got: %s", token)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := ACLMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(XNomadToken, "header-token")
	req.AddCookie(&http.Cookie{
		Name:  WanderTokenCookie,
		Value: "cookie-token",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestGetTokenFromRequest verifies the convenience function that extracts
// tokens from either context or directly from the request.
func TestGetTokenFromRequest(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		cookieValue   string
		expectedToken string
	}{
		{
			name:          "token in header",
			headerValue:   "header-token",
			expectedToken: "header-token",
		},
		{
			name:          "token in cookie",
			cookieValue:   "cookie-token",
			expectedToken: "cookie-token",
		},
		{
			name:          "header takes precedence over cookie",
			headerValue:   "header-token",
			cookieValue:   "cookie-token",
			expectedToken: "header-token",
		},
		{
			name:          "no token",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set(XNomadToken, tt.headerValue)
			}
			addWanderTokenCookie(req, tt.cookieValue)

			token := GetTokenFromRequest(req)
			if token != tt.expectedToken {
				t.Errorf("expected token %q, got %q", tt.expectedToken, token)
			}
		})
	}
}

func addWanderTokenCookie(req *http.Request, value string) {
	if value != "" {
		req.AddCookie(&http.Cookie{Name: WanderTokenCookie, Value: value})
	}
}
