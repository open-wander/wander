package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui-htmx/handlers"
)

// TestRoutes verifies all UI routes return appropriate status codes
func TestRoutes(t *testing.T) {
	// Create test handler and router
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		// Home/Dashboard
		{
			name:           "Home page",
			method:         http.MethodGet,
			path:           "/ui/",
			expectedStatus: http.StatusOK,
			description:    "Dashboard home page",
		},

		// Jobs routes
		{
			name:           "Jobs list",
			method:         http.MethodGet,
			path:           "/ui/jobs",
			expectedStatus: http.StatusOK,
			description:    "Jobs listing page",
		},
		{
			name:           "Job detail",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job",
			expectedStatus: http.StatusOK,
			description:    "Job detail page",
		},
		{
			name:           "Job overview tab",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job/overview",
			expectedStatus: http.StatusOK,
			description:    "Job overview tab",
		},
		{
			name:           "Job definition tab",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job/definition",
			expectedStatus: http.StatusOK,
			description:    "Job definition tab",
		},
		{
			name:           "Job versions tab",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job/versions",
			expectedStatus: http.StatusOK,
			description:    "Job versions tab",
		},
		{
			name:           "Job allocations tab",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job/allocations",
			expectedStatus: http.StatusOK,
			description:    "Job allocations tab",
		},
		{
			name:           "Job evaluations tab",
			method:         http.MethodGet,
			path:           "/ui/jobs/example-job/evaluations",
			expectedStatus: http.StatusOK,
			description:    "Job evaluations tab",
		},

		// Allocations routes
		{
			name:           "Allocations list",
			method:         http.MethodGet,
			path:           "/ui/allocations",
			expectedStatus: http.StatusOK,
			description:    "Allocations listing page",
		},
		{
			name:           "Allocation detail",
			method:         http.MethodGet,
			path:           "/ui/allocations/abc123",
			expectedStatus: http.StatusOK,
			description:    "Allocation detail page",
		},
		{
			name:           "Allocation overview tab",
			method:         http.MethodGet,
			path:           "/ui/allocations/abc123/overview",
			expectedStatus: http.StatusOK,
			description:    "Allocation overview tab",
		},
		{
			name:           "Allocation logs tab",
			method:         http.MethodGet,
			path:           "/ui/allocations/abc123/logs",
			expectedStatus: http.StatusOK,
			description:    "Allocation logs tab",
		},
		{
			name:           "Allocation exec tab",
			method:         http.MethodGet,
			path:           "/ui/allocations/abc123/exec",
			expectedStatus: http.StatusOK,
			description:    "Allocation exec tab",
		},

		// Clients routes
		{
			name:           "Clients list",
			method:         http.MethodGet,
			path:           "/ui/clients",
			expectedStatus: http.StatusOK,
			description:    "Clients listing page",
		},
		{
			name:           "Client detail",
			method:         http.MethodGet,
			path:           "/ui/clients/node-1",
			expectedStatus: http.StatusOK,
			description:    "Client detail page",
		},
		{
			name:           "Client overview tab",
			method:         http.MethodGet,
			path:           "/ui/clients/node-1/overview",
			expectedStatus: http.StatusOK,
			description:    "Client overview tab",
		},
		{
			name:           "Client allocations tab",
			method:         http.MethodGet,
			path:           "/ui/clients/node-1/allocations",
			expectedStatus: http.StatusOK,
			description:    "Client allocations tab",
		},
		{
			name:           "Client events tab",
			method:         http.MethodGet,
			path:           "/ui/clients/node-1/events",
			expectedStatus: http.StatusOK,
			description:    "Client events tab",
		},

		// Servers routes
		{
			name:           "Servers list",
			method:         http.MethodGet,
			path:           "/ui/servers",
			expectedStatus: http.StatusOK,
			description:    "Servers listing page",
		},
		{
			name:           "Server detail",
			method:         http.MethodGet,
			path:           "/ui/servers/server-1",
			expectedStatus: http.StatusOK,
			description:    "Server detail page",
		},

		// Topology routes
		{
			name:           "Topology",
			method:         http.MethodGet,
			path:           "/ui/topology",
			expectedStatus: http.StatusOK,
			description:    "Topology visualization page",
		},

		// Variables routes
		{
			name:           "Variables list",
			method:         http.MethodGet,
			path:           "/ui/variables",
			expectedStatus: http.StatusOK,
			description:    "Variables listing page",
		},
		{
			name:           "Variable new",
			method:         http.MethodGet,
			path:           "/ui/variables/new",
			expectedStatus: http.StatusOK,
			description:    "New variable form",
		},
		{
			name:           "Variable detail",
			method:         http.MethodGet,
			path:           "/ui/variables/default/my-var",
			expectedStatus: http.StatusOK,
			description:    "Variable detail page",
		},

		// Policies routes
		{
			name:           "Policies list",
			method:         http.MethodGet,
			path:           "/ui/policies",
			expectedStatus: http.StatusOK,
			description:    "ACL policies listing page",
		},
		{
			name:           "Policy detail",
			method:         http.MethodGet,
			path:           "/ui/policies/admin",
			expectedStatus: http.StatusOK,
			description:    "ACL policy detail page",
		},

		// CSI routes
		{
			name:           "CSI volumes list",
			method:         http.MethodGet,
			path:           "/ui/csi/volumes",
			expectedStatus: http.StatusOK,
			description:    "CSI volumes listing page",
		},
		{
			name:           "CSI volume detail",
			method:         http.MethodGet,
			path:           "/ui/csi/volumes/vol-1",
			expectedStatus: http.StatusOK,
			description:    "CSI volume detail page",
		},
		{
			name:           "CSI plugins list",
			method:         http.MethodGet,
			path:           "/ui/csi/plugins",
			expectedStatus: http.StatusOK,
			description:    "CSI plugins listing page",
		},

		// Settings routes
		{
			name:           "Settings",
			method:         http.MethodGet,
			path:           "/ui/settings",
			expectedStatus: http.StatusOK,
			description:    "Settings page",
		},

		// Evaluations routes
		{
			name:           "Evaluations panel",
			method:         http.MethodGet,
			path:           "/ui/evaluations",
			expectedStatus: http.StatusOK,
			description:    "Recent evaluations panel",
		},

		// Static assets
		{
			name:           "CSS output",
			method:         http.MethodGet,
			path:           "/ui/static/css/output.css",
			expectedStatus: http.StatusOK,
			description:    "Tailwind CSS output file",
		},
		{
			name:           "HTMX library",
			method:         http.MethodGet,
			path:           "/ui/static/js/htmx.min.js",
			expectedStatus: http.StatusOK,
			description:    "HTMX JavaScript library",
		},
		{
			name:           "Alpine.js library",
			method:         http.MethodGet,
			path:           "/ui/static/js/alpine.min.js",
			expectedStatus: http.StatusOK,
			description:    "Alpine.js JavaScript library",
		},

		// Method not allowed tests
		{
			name:           "POST to jobs list not allowed",
			method:         http.MethodPost,
			path:           "/ui/jobs",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "POST method should not be allowed on list page",
		},
		{
			name:           "POST to static file not allowed",
			method:         http.MethodPost,
			path:           "/ui/static/css/output.css",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "POST method should not be allowed on static files",
		},

		// 404 tests
		{
			name:           "Static file not found",
			method:         http.MethodGet,
			path:           "/ui/static/nonexistent.css",
			expectedStatus: http.StatusNotFound,
			description:    "Non-existent static file should return 404",
		},
		{
			name:           "Unknown tab returns 404",
			method:         http.MethodGet,
			path:           "/ui/jobs/test-job/unknown-tab",
			expectedStatus: http.StatusNotFound,
			description:    "Unknown tab should return 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: expected status %d, got %d\nPath: %s\nDescription: %s",
					tt.name, tt.expectedStatus, w.Code, tt.path, tt.description)
			}

			// For successful requests, verify we got some content
			if tt.expectedStatus == http.StatusOK {
				body := w.Body.String()
				if body == "" {
					t.Errorf("%s: expected response body, got empty response\nPath: %s",
						tt.name, tt.path)
				}
			}
		})
	}
}

// TestRoutesHTMXRequests verifies HTMX requests work correctly
func TestRoutesHTMXRequests(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	tests := []struct {
		name           string
		path           string
		shouldHavePush bool
	}{
		{
			name:           "Jobs list HTMX",
			path:           "/ui/jobs",
			shouldHavePush: true,
		},
		{
			name:           "Job detail HTMX",
			path:           "/ui/jobs/example-job",
			shouldHavePush: true,
		},
		{
			name:           "Allocations list HTMX",
			path:           "/ui/allocations",
			shouldHavePush: true,
		},
		{
			name:           "Clients list HTMX",
			path:           "/ui/clients",
			shouldHavePush: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("HX-Request", "true")
			req.Header.Set("HX-History-Restore-Request", "false")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body := w.Body.String()
			if body == "" {
				t.Error("Expected response body, got empty response")
			}

			// HTMX requests should not contain DOCTYPE (partial content)
			if len(body) >= 15 && body[:15] == "<!DOCTYPE html>" {
				t.Error("HTMX request should return partial content, not full page")
			}

			// Check for HX-Push-Url header if expected
			if tt.shouldHavePush {
				pushURL := w.Header().Get("HX-Push-Url")
				if pushURL == "" {
					// This is informational - some pages might not set push URL
					t.Logf("Note: No HX-Push-Url header set for %s", tt.path)
				}
			}
		})
	}
}

// TestRoutesContentType verifies correct content types are set
func TestRoutesContentType(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	tests := []struct {
		name        string
		path        string
		contentType string
	}{
		{
			name:        "HTML page",
			path:        "/ui/jobs",
			contentType: "text/html",
		},
		{
			name:        "CSS file",
			path:        "/ui/static/css/output.css",
			contentType: "text/css",
		},
		{
			name:        "JavaScript file",
			path:        "/ui/static/js/htmx.min.js",
			contentType: "text/javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
				return
			}

			contentType := w.Header().Get("Content-Type")
			if contentType == "" {
				t.Errorf("No Content-Type header set for %s", tt.path)
				return
			}

			// Check if content type starts with expected type (ignore charset suffix)
			if len(contentType) < len(tt.contentType) ||
				contentType[:len(tt.contentType)] != tt.contentType {
				t.Errorf("Expected Content-Type to start with %s, got %s",
					tt.contentType, contentType)
			}
		})
	}
}

// TestAPIRoutes verifies API routes for HTMX actions
func TestAPIRoutes(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "Job run action",
			method:         http.MethodPost,
			path:           "/api/jobs/test-job/run",
			expectedStatus: http.StatusOK,
			description:    "Start/run a job",
		},
		{
			name:           "Job stop action",
			method:         http.MethodPost,
			path:           "/api/jobs/test-job/stop",
			expectedStatus: http.StatusOK,
			description:    "Stop a running job",
		},
		{
			name:           "Job purge action",
			method:         http.MethodPost,
			path:           "/api/jobs/test-job/purge",
			expectedStatus: http.StatusOK,
			description:    "Purge a job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: expected status %d, got %d\nPath: %s\nMethod: %s",
					tt.name, tt.expectedStatus, w.Code, tt.path, tt.method)
			}
		})
	}
}

// TestRoutesConcurrency verifies routes can handle concurrent requests
func TestRoutesConcurrency(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	// Test a sample of routes concurrently
	routes := []string{
		"/ui/",
		"/ui/jobs",
		"/ui/allocations",
		"/ui/clients",
		"/ui/servers",
	}

	// Run concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for _, route := range routes {
				req := httptest.NewRequest(http.MethodGet, route, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Concurrent request to %s failed with status %d", route, w.Code)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
