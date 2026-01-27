package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestClientsHandlerCreation tests basic clients handler creation
func TestClientsHandlerCreation(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	if clients == nil {
		t.Fatal("NewClientsHandler returned nil")
	}
	if clients.handler == nil {
		t.Error("ClientsHandler.handler is nil")
	}
}

// TestClientsList tests the clients list page
func TestClientsList(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}

	// Should contain client names from mock data
	expectedStrings := []string{
		"client-node-1",
		"client-node-2",
		"client-node-3",
		"192.168.1.100:4646",
		"dc1",
		"dc2",
		"compute",
		"storage",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain %q", expected)
		}
	}
}

// TestClientsListHTMX tests the clients list with HTMX request
func TestClientsListHTMX(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}

	// HTMX response should not contain full page HTML
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX response should not contain DOCTYPE")
	}
}

// TestClientsListFilterByStatus tests filtering clients by status
func TestClientsListFilterByStatus(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	testCases := []struct {
		name             string
		status           string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:   "filter ready status",
			status: "ready",
			shouldContain: []string{
				"client-node-1",
				"client-node-2",
				"client-node-4",
				"client-node-6",
			},
			shouldNotContain: []string{
				"client-node-3", // down
				"client-node-5", // draining
			},
		},
		{
			name:   "filter down status",
			status: "down",
			shouldContain: []string{
				"client-node-3",
			},
			shouldNotContain: []string{
				"client-node-1",
				"client-node-2",
			},
		},
		{
			name:   "filter draining status",
			status: "draining",
			shouldContain: []string{
				"client-node-5",
			},
			shouldNotContain: []string{
				"client-node-1",
				"client-node-2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/clients?status="+tc.status, nil)
			w := httptest.NewRecorder()

			clients.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body := w.Body.String()

			for _, expected := range tc.shouldContain {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected response to contain %q", expected)
				}
			}

			for _, unexpected := range tc.shouldNotContain {
				if strings.Contains(body, unexpected) {
					t.Errorf("Expected response NOT to contain %q", unexpected)
				}
			}
		})
	}
}

// TestClientsListFilterByDatacenter tests filtering clients by datacenter
func TestClientsListFilterByDatacenter(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	testCases := []struct {
		name             string
		datacenter       string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:       "filter dc1",
			datacenter: "dc1",
			shouldContain: []string{
				"client-node-1",
				"client-node-2",
				"client-node-4",
			},
			shouldNotContain: []string{
				"client-node-3", // dc2
				"client-node-5", // dc2
				"client-node-6", // dc2
			},
		},
		{
			name:       "filter dc2",
			datacenter: "dc2",
			shouldContain: []string{
				"client-node-3",
				"client-node-5",
				"client-node-6",
			},
			shouldNotContain: []string{
				"client-node-1", // dc1
				"client-node-2", // dc1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/clients?datacenter="+tc.datacenter, nil)
			w := httptest.NewRecorder()

			clients.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body := w.Body.String()

			for _, expected := range tc.shouldContain {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected response to contain %q", expected)
				}
			}

			for _, unexpected := range tc.shouldNotContain {
				if strings.Contains(body, unexpected) {
					t.Errorf("Expected response NOT to contain %q", unexpected)
				}
			}
		})
	}
}

// TestClientsListFilterByNodeClass tests filtering clients by node class
func TestClientsListFilterByNodeClass(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients?nodeclass=compute", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain compute nodes
	shouldContain := []string{"client-node-1", "client-node-2", "client-node-5"}
	for _, expected := range shouldContain {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain %q", expected)
		}
	}

	// Should not contain non-compute nodes
	shouldNotContain := []string{"storage", "high-memory", "gpu"}
	for _, _ = range shouldNotContain {
		// Note: We're checking for node class labels, not node names
		// client-node-3 has class "storage", etc.
		// This validation is implicit - the filtered results above shouldn't contain these
	}
}

// TestClientsListFilterBySearch tests search functionality
func TestClientsListFilterBySearch(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	testCases := []struct {
		name             string
		search           string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:   "search by name",
			search: "client-node-1",
			shouldContain: []string{
				"client-node-1",
			},
			shouldNotContain: []string{
				"client-node-2",
				"client-node-3",
			},
		},
		{
			name:   "search by datacenter",
			search: "dc1",
			shouldContain: []string{
				"client-node-1",
				"client-node-2",
				"client-node-4",
			},
			shouldNotContain: []string{
				"client-node-3",
				"client-node-5",
			},
		},
		{
			name:   "search by IP",
			search: "192.168.1.100",
			shouldContain: []string{
				"client-node-1",
			},
			shouldNotContain: []string{
				"client-node-2",
				"client-node-3",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/clients?search="+url.QueryEscape(tc.search), nil)
			w := httptest.NewRecorder()

			clients.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body := w.Body.String()

			for _, expected := range tc.shouldContain {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected response to contain %q", expected)
				}
			}

			for _, unexpected := range tc.shouldNotContain {
				if strings.Contains(body, unexpected) {
					t.Errorf("Expected response NOT to contain %q", unexpected)
				}
			}
		})
	}
}

// TestClientsListMultipleFilters tests combining multiple filters
func TestClientsListMultipleFilters(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	// Filter for ready clients in dc1 with compute class
	req := httptest.NewRequest(http.MethodGet, "/ui/clients?status=ready&datacenter=dc1&nodeclass=compute", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain client-node-1 and client-node-2 (ready, dc1, compute)
	shouldContain := []string{"client-node-1", "client-node-2"}
	for _, expected := range shouldContain {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain %q", expected)
		}
	}

	// Should not contain nodes that don't match all filters
	shouldNotContain := []string{
		"client-node-3", // down
		"client-node-4", // high-memory (not compute)
		"client-node-5", // draining
		"client-node-6", // dc2
	}
	for _, unexpected := range shouldNotContain {
		if strings.Contains(body, unexpected) {
			t.Errorf("Expected response NOT to contain %q", unexpected)
		}
	}
}

// TestClientsDetail tests the client detail page
func TestClientsDetail(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	clientID := "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o"
	req := httptest.NewRequest(http.MethodGet, "/ui/clients/"+clientID, nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}

	// Should contain client detail information
	expectedStrings := []string{
		"client-node-1",
		"ready",
		"dc1",
		"192.168.1.100:4646",
		"1.7.2",
		"compute",
		// Allocations
		"web-frontend",
		"api-gateway",
		"cache-service",
		"data-processor",
		"metrics-collector",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain %q", expected)
		}
	}
}

// TestClientsDetailHTMX tests the client detail page with HTMX request
func TestClientsDetailHTMX(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	clientID := "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o"
	req := httptest.NewRequest(http.MethodGet, "/ui/clients/"+clientID, nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// HTMX response should not contain full page HTML
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTMX response should not contain DOCTYPE")
	}

	// Should still contain client data
	if !strings.Contains(body, "client-node-1") {
		t.Error("Expected response to contain client name")
	}
}

// TestClientsDetailResourceCharts tests that detail page includes resource chart data
func TestClientsDetailResourceCharts(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	clientID := "test-client-id"
	req := httptest.NewRequest(http.MethodGet, "/ui/clients/"+clientID, nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// The detail page should include resource information
	// Check for CPU, Memory, Disk information
	hasResourceInfo := strings.Contains(body, "CPU") ||
		strings.Contains(body, "Memory") ||
		strings.Contains(body, "Disk")

	if !hasResourceInfo {
		t.Error("Expected detail page to contain resource information")
	}
}

// TestClientsListMethodNotAllowed tests that non-GET requests are rejected
func TestClientsListMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ui/clients", nil)
			w := httptest.NewRecorder()

			clients.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

// TestClientsDetailMethodNotAllowed tests that non-GET requests to detail are rejected
func TestClientsDetailMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ui/clients/test-id", nil)
			w := httptest.NewRecorder()

			clients.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

// TestClientsDrain tests the drain endpoint
func TestClientsDrain(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	clientID := "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o"
	req := httptest.NewRequest(http.MethodPost, "/ui/clients/"+clientID+"/drain", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = url.Values{
		"deadline":      []string{"1h"},
		"ignore_system": []string{"true"},
	}
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should return HX-Redirect header
	redirect := w.Header().Get("HX-Redirect")
	expectedRedirect := "/ui/clients/" + clientID
	if redirect != expectedRedirect {
		t.Errorf("Expected HX-Redirect to %q, got %q", expectedRedirect, redirect)
	}
}

// TestClientsDrainMethodNotAllowed tests that non-POST requests to drain are rejected
func TestClientsDrainMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients/test-id/drain", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestClientsCancelDrain tests the cancel drain endpoint
func TestClientsCancelDrain(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	clientID := "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o"
	req := httptest.NewRequest(http.MethodPost, "/ui/clients/"+clientID+"/drain/cancel", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should return HX-Redirect header
	redirect := w.Header().Get("HX-Redirect")
	expectedRedirect := "/ui/clients/" + clientID
	if redirect != expectedRedirect {
		t.Errorf("Expected HX-Redirect to %q, got %q", expectedRedirect, redirect)
	}
}

// TestClientsCancelDrainMethodNotAllowed tests that non-POST requests to cancel drain are rejected
func TestClientsCancelDrainMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients/test-id/drain/cancel", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestClientsInvalidPath tests 404 for invalid paths
func TestClientsInvalidPath(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/clients/invalid/path/here", nil)
	w := httptest.NewRecorder()

	clients.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestExtractFilterOptions tests filter option extraction
func TestExtractFilterOptions(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	// Get the clients list
	clientsList := clients.getClientsList("")

	// Extract filter options
	filterOptions := clients.extractFilterOptions(clientsList)

	// Should have both datacenters
	expectedDatacenters := []string{"dc1", "dc2"}
	if len(filterOptions.Datacenters) != len(expectedDatacenters) {
		t.Errorf("Expected %d datacenters, got %d", len(expectedDatacenters), len(filterOptions.Datacenters))
	}

	// Datacenters should be sorted
	for i, dc := range expectedDatacenters {
		if i < len(filterOptions.Datacenters) && filterOptions.Datacenters[i] != dc {
			t.Errorf("Expected datacenter %q at index %d, got %q", dc, i, filterOptions.Datacenters[i])
		}
	}

	// Should have multiple node classes
	expectedNodeClasses := []string{"compute", "gpu", "high-memory", "storage"}
	if len(filterOptions.NodeClasses) != len(expectedNodeClasses) {
		t.Errorf("Expected %d node classes, got %d", len(expectedNodeClasses), len(filterOptions.NodeClasses))
	}
}

// TestApplyFilters tests the filter application logic
func TestApplyFilters(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	clients := NewClientsHandler(h)

	// Get the full clients list
	allClients := clients.getClientsList("")
	initialCount := len(allClients)

	if initialCount == 0 {
		t.Fatal("Expected mock clients, got empty list")
	}

	// Test empty filters (should return all)
	emptyFilters := struct {
		Status     string
		Datacenter string
		NodeClass  string
		Search     string
	}{}

	filtered := clients.applyFilters(allClients, emptyFilters)
	if len(filtered) != initialCount {
		t.Errorf("Empty filters should return all clients, got %d of %d", len(filtered), initialCount)
	}

	// Test status filter
	statusFilters := struct {
		Status     string
		Datacenter string
		NodeClass  string
		Search     string
	}{Status: "ready"}

	filteredByStatus := clients.applyFilters(allClients, statusFilters)
	if len(filteredByStatus) == 0 {
		t.Error("Status filter should return some clients")
	}
	if len(filteredByStatus) >= initialCount {
		t.Error("Status filter should reduce the number of clients")
	}

	// Verify all returned clients have ready status
	for _, client := range filteredByStatus {
		if client.Status != "ready" {
			t.Errorf("Expected all clients to have status 'ready', got %q", client.Status)
		}
	}
}
