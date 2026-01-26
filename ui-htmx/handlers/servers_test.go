package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestServersHandlerCreation tests basic servers handler creation
func TestServersHandlerCreation(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	if servers == nil {
		t.Fatal("NewServersHandler returned nil")
	}
	if servers.handler == nil {
		t.Error("ServersHandler.handler is nil")
	}
}

// TestServersList tests the servers list page
func TestServersList(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}

	// Should contain server names from mock data
	expectedStrings := []string{
		"nomad-server-1",
		"nomad-server-2",
		"nomad-server-3",
		"nomad-server-4",
		"nomad-server-5",
		"10.0.1.10:4647",
		"10.0.1.11:4647",
		"dc1",
		"dc2",
		"us-east-1",
		"us-west-2",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain %q", expected)
		}
	}
}

// TestServersListHTMX tests the servers list with HTMX request
func TestServersListHTMX(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

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

	// Should still contain server data
	if !strings.Contains(body, "nomad-server") {
		t.Error("Expected response to contain server names")
	}
}

// TestServersListLeaderBadge tests that leader badge is displayed
func TestServersListLeaderBadge(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// The leader server (nomad-server-1) should be marked somehow
	// This could be a badge, icon, or specific text
	// Check for common leader indicators
	hasLeaderIndicator := strings.Contains(body, "Leader") ||
		strings.Contains(body, "leader") ||
		strings.Contains(body, "LEADER")

	if !hasLeaderIndicator {
		t.Error("Expected response to contain leader indicator")
	}
}

// TestServersListStatuses tests that different server statuses are displayed
func TestServersListStatuses(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should show various statuses from mock data
	expectedStatuses := []string{"alive", "failed"}
	for _, status := range expectedStatuses {
		if !strings.Contains(body, status) {
			t.Errorf("Expected response to contain status %q", status)
		}
	}
}

// TestServersListVersions tests that server versions are displayed
func TestServersListVersions(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain version numbers
	expectedVersions := []string{"1.7.2", "1.7.1"}
	for _, version := range expectedVersions {
		if !strings.Contains(body, version) {
			t.Errorf("Expected response to contain version %q", version)
		}
	}
}

// TestServersListRaftVoter tests that Raft voter status is displayed
func TestServersListRaftVoter(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Mock data has both voters and non-voters
	// nomad-server-5 is not a Raft voter
	// This information should be displayed in some form
	if !strings.Contains(body, "nomad-server-5") {
		t.Error("Expected response to contain all servers including non-voters")
	}
}

// TestServersListWithSort tests sorting functionality
func TestServersListWithSort(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	testCases := []struct {
		name       string
		sortColumn string
		sortDir    string
	}{
		{
			name:       "sort by name ascending",
			sortColumn: "name",
			sortDir:    "asc",
		},
		{
			name:       "sort by name descending",
			sortColumn: "name",
			sortDir:    "desc",
		},
		{
			name:       "sort by status",
			sortColumn: "status",
			sortDir:    "asc",
		},
		{
			name:       "sort by datacenter",
			sortColumn: "datacenter",
			sortDir:    "asc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/ui/servers?sort=" + tc.sortColumn + "&dir=" + tc.sortDir
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			servers.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body := w.Body.String()
			if body == "" {
				t.Error("Response body is empty")
			}

			// Should still contain server data
			if !strings.Contains(body, "nomad-server") {
				t.Error("Expected response to contain server names")
			}
		})
	}
}

// TestServersListDefaultSort tests default sorting behavior
func TestServersListDefaultSort(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	// Request without sort parameters should use defaults
	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}

	// Should contain all servers
	expectedServers := []string{
		"nomad-server-1",
		"nomad-server-2",
		"nomad-server-3",
		"nomad-server-4",
		"nomad-server-5",
	}

	for _, server := range expectedServers {
		if !strings.Contains(body, server) {
			t.Errorf("Expected response to contain %q", server)
		}
	}
}

// TestServersListMethodNotAllowed tests that non-GET requests are rejected
func TestServersListMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ui/servers", nil)
			w := httptest.NewRecorder()

			servers.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

// TestServersInvalidPath tests 404 for invalid paths
func TestServersInvalidPath(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers/invalid/path", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestGetServersList tests the mock data retrieval
func TestGetServersList(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	serversList := servers.getServersList()

	if len(serversList) == 0 {
		t.Fatal("Expected mock servers, got empty list")
	}

	// Should have 5 servers in mock data
	expectedCount := 5
	if len(serversList) != expectedCount {
		t.Errorf("Expected %d servers, got %d", expectedCount, len(serversList))
	}

	// Verify the leader server exists
	foundLeader := false
	for _, server := range serversList {
		if server.Leader {
			foundLeader = true
			// Leader should be nomad-server-1
			if server.Name != "nomad-server-1" {
				t.Errorf("Expected leader to be nomad-server-1, got %q", server.Name)
			}
			break
		}
	}

	if !foundLeader {
		t.Error("Expected to find a leader server in mock data")
	}

	// Verify server data structure
	for i, server := range serversList {
		if server.Name == "" {
			t.Errorf("Server %d has empty name", i)
		}
		if server.Address == "" {
			t.Errorf("Server %d has empty address", i)
		}
		if server.Status == "" {
			t.Errorf("Server %d has empty status", i)
		}
		if server.Region == "" {
			t.Errorf("Server %d has empty region", i)
		}
		if server.Datacenter == "" {
			t.Errorf("Server %d has empty datacenter", i)
		}
		if server.Version == "" {
			t.Errorf("Server %d has empty version", i)
		}
	}
}

// TestServersListAllRegions tests that servers from different regions are shown
func TestServersListAllRegions(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain servers from both regions
	expectedRegions := []string{"us-east-1", "us-west-2"}
	for _, region := range expectedRegions {
		if !strings.Contains(body, region) {
			t.Errorf("Expected response to contain region %q", region)
		}
	}
}

// TestServersListAllDatacenters tests that servers from different datacenters are shown
func TestServersListAllDatacenters(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain servers from both datacenters
	expectedDatacenters := []string{"dc1", "dc2"}
	for _, dc := range expectedDatacenters {
		if !strings.Contains(body, dc) {
			t.Errorf("Expected response to contain datacenter %q", dc)
		}
	}
}

// TestServersListFailedServer tests that failed servers are displayed
func TestServersListFailedServer(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// nomad-server-5 has failed status
	if !strings.Contains(body, "nomad-server-5") {
		t.Error("Expected response to contain failed server")
	}

	if !strings.Contains(body, "failed") {
		t.Error("Expected response to show failed status")
	}
}

// TestServersListAddresses tests that server addresses are displayed
func TestServersListAddresses(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain server addresses
	expectedAddresses := []string{
		"10.0.1.10:4647",
		"10.0.1.11:4647",
		"10.0.1.12:4647",
		"10.0.2.10:4647",
		"10.0.2.11:4647",
	}

	for _, address := range expectedAddresses {
		if !strings.Contains(body, address) {
			t.Errorf("Expected response to contain address %q", address)
		}
	}
}

// TestServersListHTMXPartialRendering tests that HTMX requests return partials
func TestServersListHTMXPartialRendering(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	servers := NewServersHandler(h)

	// Create HTMX request
	req := httptest.NewRequest(http.MethodGet, "/ui/servers", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	servers.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// HTMX partial should not have full HTML structure
	if strings.Contains(body, "<html") {
		t.Error("HTMX response should not contain <html> tag")
	}
	if strings.Contains(body, "<head") {
		t.Error("HTMX response should not contain <head> tag")
	}

	// But should still have server data
	if !strings.Contains(body, "nomad-server-1") {
		t.Error("Expected HTMX response to contain server data")
	}
}
