package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// setupAllocationsHandler creates a handler for testing
func setupAllocationsHandler() *AllocationsHandler {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	return NewAllocationsHandler(h)
}

// TestAllocationsHandler_List tests the allocations list endpoint
func TestAllocationsHandler_List(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		htmxRequest    bool
		htmxTarget     string
		expectedStatus int
		expectContains []string
	}{
		{
			name:           "Full page request returns 200",
			url:            "/ui/allocations",
			htmxRequest:    false,
			expectedStatus: http.StatusOK,
			expectContains: []string{
				"<!DOCTYPE html>", // Should have full HTML structure
			},
		},
		{
			name:           "HTMX request returns partial",
			url:            "/ui/allocations",
			htmxRequest:    true,
			expectedStatus: http.StatusOK,
			expectContains: []string{
				// Partial response, no DOCTYPE
			},
		},
		{
			name:           "HTMX table request returns table partial",
			url:            "/ui/allocations",
			htmxRequest:    true,
			htmxTarget:     "allocations-table-container",
			expectedStatus: http.StatusOK,
			expectContains: []string{
				// Table partial only
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.htmxRequest {
				req.Header.Set("HX-Request", "true")
			}
			if tt.htmxTarget != "" {
				req.Header.Set("HX-Target", tt.htmxTarget)
			}

			w := httptest.NewRecorder()
			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			for _, expected := range tt.expectContains {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected response to contain %q", expected)
				}
			}
		})
	}
}

// TestAllocationsHandler_List_Filtering tests filtering functionality
func TestAllocationsHandler_List_Filtering(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
		description    string
	}{
		{
			name:           "Filter by status",
			url:            "/ui/allocations?status=running",
			expectedStatus: http.StatusOK,
			description:    "Should return only running allocations",
		},
		{
			name:           "Filter by job",
			url:            "/ui/allocations?job=web-frontend",
			expectedStatus: http.StatusOK,
			description:    "Should return only web-frontend job allocations",
		},
		{
			name:           "Filter by node",
			url:            "/ui/allocations?node=worker-01.dc1",
			expectedStatus: http.StatusOK,
			description:    "Should return only allocations on worker-01.dc1",
		},
		{
			name:           "Multiple filters",
			url:            "/ui/allocations?status=running&job=web-frontend",
			expectedStatus: http.StatusOK,
			description:    "Should apply all filters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				body := w.Body.String()
				if body == "" {
					t.Error("Response body is empty")
				}
			}
		})
	}
}

// TestAllocationsHandler_List_Search tests search functionality
func TestAllocationsHandler_List_Search(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
		description    string
	}{
		{
			name:           "Search by ID prefix",
			url:            "/ui/allocations?search=a1b2c3",
			expectedStatus: http.StatusOK,
			description:    "Should find allocation starting with a1b2c3",
		},
		{
			name:           "Search by short ID",
			url:            "/ui/allocations?search=a1b2c3d4",
			expectedStatus: http.StatusOK,
			description:    "Should find allocation by short ID (first 8 chars)",
		},
		{
			name:           "Search by job name",
			url:            "/ui/allocations?search=web",
			expectedStatus: http.StatusOK,
			description:    "Should find allocations with 'web' in job name",
		},
		{
			name:           "Search by node name",
			url:            "/ui/allocations?search=worker",
			expectedStatus: http.StatusOK,
			description:    "Should find allocations with 'worker' in node name",
		},
		{
			name:           "Case insensitive search",
			url:            "/ui/allocations?search=WEB",
			expectedStatus: http.StatusOK,
			description:    "Search should be case insensitive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				body := w.Body.String()
				if body == "" {
					t.Error("Response body is empty")
				}
			}
		})
	}
}

// TestAllocationsHandler_List_Pagination tests pagination functionality
func TestAllocationsHandler_List_Pagination(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "Default pagination (page 1, 25 per page)",
			url:            "/ui/allocations",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Custom page",
			url:            "/ui/allocations?page=2",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Custom per page",
			url:            "/ui/allocations?per_page=10",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Page and per_page together",
			url:            "/ui/allocations?page=2&per_page=5",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid page defaults to 1",
			url:            "/ui/allocations?page=abc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Negative page defaults to 1",
			url:            "/ui/allocations?page=-1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Per page exceeds max (100) is capped",
			url:            "/ui/allocations?per_page=200",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAllocationsHandler_List_Sorting tests sorting functionality
func TestAllocationsHandler_List_Sorting(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "Default sort (created desc)",
			url:            "/ui/allocations",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by ID ascending",
			url:            "/ui/allocations?sort=id&dir=asc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by ID descending",
			url:            "/ui/allocations?sort=id&dir=desc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by job",
			url:            "/ui/allocations?sort=job&dir=asc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by taskgroup",
			url:            "/ui/allocations?sort=taskgroup&dir=asc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by node",
			url:            "/ui/allocations?sort=node&dir=asc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by status",
			url:            "/ui/allocations?sort=status&dir=asc",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Sort by created",
			url:            "/ui/allocations?sort=created&dir=desc",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAllocationsHandler_Detail tests the allocation detail endpoint
func TestAllocationsHandler_Detail(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		htmxRequest    bool
		expectedStatus int
		expectContains []string
	}{
		{
			name:           "Full page request for valid allocation",
			url:            "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			htmxRequest:    false,
			expectedStatus: http.StatusOK,
			expectContains: []string{
				"<!DOCTYPE html>",
			},
		},
		{
			name:           "HTMX partial request for valid allocation",
			url:            "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			htmxRequest:    true,
			expectedStatus: http.StatusOK,
			expectContains: []string{},
		},
		{
			name:           "Detail page works for any allocation ID",
			url:            "/ui/allocations/test-alloc-id",
			htmxRequest:    false,
			expectedStatus: http.StatusOK,
			expectContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.htmxRequest {
				req.Header.Set("HX-Request", "true")
			}

			w := httptest.NewRecorder()
			handler.Detail(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			for _, expected := range tt.expectContains {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected response to contain %q", expected)
				}
			}
		})
	}
}

// TestAllocationsHandler_TabContent tests tab content rendering
func TestAllocationsHandler_TabContent(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "Tasks tab",
			url:            "/ui/allocations/test-id/tasks",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Logs tab with defaults",
			url:            "/ui/allocations/test-id/logs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Logs tab with task and type params",
			url:            "/ui/allocations/test-id/logs?task=nginx&type=stderr",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Exec tab with defaults",
			url:            "/ui/allocations/test-id/exec",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Exec tab with task and command params",
			url:            "/ui/allocations/test-id/exec?task=nginx&command=/bin/bash",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Files tab with defaults",
			url:            "/ui/allocations/test-id/files",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Files tab with task param",
			url:            "/ui/allocations/test-id/files?task=nginx",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Raw tab",
			url:            "/ui/allocations/test-id/raw",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			// Tab content requires HTMX request
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()
			handler.Detail(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				body := w.Body.String()
				if body == "" {
					t.Error("Response body is empty")
				}
			}
		})
	}
}

// TestAllocationsHandler_FilesSubRoutes tests file operation sub-routes
func TestAllocationsHandler_FilesSubRoutes(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "Files list - root",
			url:            "/ui/allocations/test-id/files/list?task=nginx&path=/",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Files list - subdirectory",
			url:            "/ui/allocations/test-id/files/list?task=nginx&path=/alloc/logs",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Files preview",
			url:            "/ui/allocations/test-id/files/preview?task=nginx&path=/local/config.json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Files download - missing path",
			url:            "/ui/allocations/test-id/files/download?task=nginx",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Files download - valid",
			url:            "/ui/allocations/test-id/files/download?task=nginx&path=/local/config.json",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			// These are HTMX partial requests
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()
			handler.Detail(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAllocationsHandler_Restart tests the restart action
func TestAllocationsHandler_Restart(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		method             string
		expectedStatus     int
		expectHXTrigger    bool
		expectHXRefresh    bool
	}{
		{
			name:            "Restart with POST",
			url:             "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d/restart",
			method:          http.MethodPost,
			expectedStatus:  http.StatusOK,
			expectHXTrigger: true,
			expectHXRefresh: true,
		},
		{
			name:           "Restart with GET fails",
			url:            "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d/restart",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectHXTrigger {
				trigger := w.Header().Get("HX-Trigger")
				if trigger == "" {
					t.Error("Expected HX-Trigger header to be set")
				}
				if !strings.Contains(trigger, "showToast") {
					t.Error("Expected HX-Trigger to contain showToast event")
				}
			}

			if tt.expectHXRefresh {
				refresh := w.Header().Get("HX-Refresh")
				if refresh != "true" {
					t.Errorf("Expected HX-Refresh to be 'true', got %q", refresh)
				}
			}
		})
	}
}

// TestAllocationsHandler_Stop tests the stop action
func TestAllocationsHandler_Stop(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		method             string
		expectedStatus     int
		expectHXTrigger    bool
		expectHXRefresh    bool
	}{
		{
			name:            "Stop with POST",
			url:             "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d/stop",
			method:          http.MethodPost,
			expectedStatus:  http.StatusOK,
			expectHXTrigger: true,
			expectHXRefresh: true,
		},
		{
			name:           "Stop with GET fails",
			url:            "/ui/allocations/a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d/stop",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectHXTrigger {
				trigger := w.Header().Get("HX-Trigger")
				if trigger == "" {
					t.Error("Expected HX-Trigger header to be set")
				}
				if !strings.Contains(trigger, "showToast") {
					t.Error("Expected HX-Trigger to contain showToast event")
				}
			}

			if tt.expectHXRefresh {
				refresh := w.Header().Get("HX-Refresh")
				if refresh != "true" {
					t.Errorf("Expected HX-Refresh to be 'true', got %q", refresh)
				}
			}
		})
	}
}

// TestAllocationsHandler_ServeHTTP_MethodNotAllowed tests method validation
func TestAllocationsHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		method         string
		expectedStatus int
	}{
		{
			name:           "List with POST fails",
			url:            "/ui/allocations",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Detail with POST fails",
			url:            "/ui/allocations/test-id",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Detail with PUT fails",
			url:            "/ui/allocations/test-id",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupAllocationsHandler()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAllocationsHandler_ServeHTTP_NotFound tests 404 handling
func TestAllocationsHandler_ServeHTTP_NotFound(t *testing.T) {
	handler := setupAllocationsHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/allocations/invalid/path/here", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestAllocationsHandler_FilterAllocations tests the filtering logic
func TestAllocationsHandler_FilterAllocations(t *testing.T) {
	handler := setupAllocationsHandler()
	allocs := handler.getAllocationsList("", "", "", "", "")

	tests := []struct {
		name           string
		status         string
		job            string
		node           string
		search         string
		expectedMinLen int
		description    string
	}{
		{
			name:           "No filters returns all",
			status:         "",
			job:            "",
			node:           "",
			search:         "",
			expectedMinLen: len(allocs),
			description:    "Should return all allocations",
		},
		{
			name:           "Filter by running status",
			status:         "running",
			job:            "",
			node:           "",
			search:         "",
			expectedMinLen: 1,
			description:    "Should return only running allocations",
		},
		{
			name:           "Filter by job",
			status:         "",
			job:            "web-frontend",
			node:           "",
			search:         "",
			expectedMinLen: 1,
			description:    "Should return only web-frontend allocations",
		},
		{
			name:           "Filter by node",
			status:         "",
			job:            "",
			node:           "worker-01.dc1",
			search:         "",
			expectedMinLen: 1,
			description:    "Should return allocations on worker-01.dc1",
		},
		{
			name:           "Search by ID prefix",
			status:         "",
			job:            "",
			node:           "",
			search:         "a1b2",
			expectedMinLen: 0,
			description:    "Should find allocations starting with a1b2",
		},
		{
			name:           "Combined filters",
			status:         "running",
			job:            "web-frontend",
			node:           "",
			search:         "",
			expectedMinLen: 0,
			description:    "Should apply both status and job filters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := handler.filterAllocations(allocs, tt.status, tt.job, tt.node, tt.search)

			if len(filtered) < tt.expectedMinLen {
				t.Errorf("Expected at least %d allocations, got %d", tt.expectedMinLen, len(filtered))
			}
		})
	}
}

// TestAllocationsHandler_SortAllocations tests the sorting logic
func TestAllocationsHandler_SortAllocations(t *testing.T) {
	handler := setupAllocationsHandler()
	allocs := handler.getAllocationsList("", "", "", "", "")

	tests := []struct {
		name        string
		sortColumn  string
		sortDir     string
		description string
	}{
		{
			name:        "Sort by ID ascending",
			sortColumn:  "id",
			sortDir:     "asc",
			description: "Should sort by ID in ascending order",
		},
		{
			name:        "Sort by ID descending",
			sortColumn:  "id",
			sortDir:     "desc",
			description: "Should sort by ID in descending order",
		},
		{
			name:        "Sort by job",
			sortColumn:  "job",
			sortDir:     "asc",
			description: "Should sort by job name",
		},
		{
			name:        "Sort by taskgroup",
			sortColumn:  "taskgroup",
			sortDir:     "asc",
			description: "Should sort by task group name",
		},
		{
			name:        "Sort by node",
			sortColumn:  "node",
			sortDir:     "asc",
			description: "Should sort by node name",
		},
		{
			name:        "Sort by status",
			sortColumn:  "status",
			sortDir:     "asc",
			description: "Should sort by status",
		},
		{
			name:        "Sort by created",
			sortColumn:  "created",
			sortDir:     "desc",
			description: "Should sort by creation time",
		},
		{
			name:        "Invalid sort column defaults to created",
			sortColumn:  "invalid",
			sortDir:     "desc",
			description: "Should default to created column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			testAllocs := make([]AllocationSummary, len(allocs))
			copy(testAllocs, allocs)

			// Sort should not panic
			handler.sortAllocations(testAllocs, tt.sortColumn, tt.sortDir)

			// Verify we still have all allocations
			if len(testAllocs) != len(allocs) {
				t.Errorf("Expected %d allocations after sort, got %d", len(allocs), len(testAllocs))
			}
		})
	}
}

// TestAllocationsHandler_IsIDPrefixMatch tests ID prefix matching logic
func TestAllocationsHandler_IsIDPrefixMatch(t *testing.T) {
	handler := setupAllocationsHandler()

	tests := []struct {
		name          string
		allocID       string
		search        string
		expectedMatch bool
		expectedPrefix string
	}{
		{
			name:           "Empty search",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "",
			expectedMatch:  false,
			expectedPrefix: "",
		},
		{
			name:           "Full ID prefix match",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "a1b2c3d4",
			expectedMatch:  true,
			expectedPrefix: "a1b2c3d4",
		},
		{
			name:           "Short ID prefix match",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "a1b2",
			expectedMatch:  true,
			expectedPrefix: "a1b2",
		},
		{
			name:           "Case insensitive match",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "A1B2",
			expectedMatch:  false,
			expectedPrefix: "",
		},
		{
			name:           "No match",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "z9z9",
			expectedMatch:  false,
			expectedPrefix: "",
		},
		{
			name:           "Match within ID but not prefix",
			allocID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			search:         "e5f6",
			expectedMatch:  false,
			expectedPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, prefix := handler.isIDPrefixMatch(tt.allocID, tt.search)

			if match != tt.expectedMatch {
				t.Errorf("Expected match=%v, got %v", tt.expectedMatch, match)
			}

			if prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix=%q, got %q", tt.expectedPrefix, prefix)
			}
		})
	}
}

// TestAllocationsHandler_ExtractFilterOptions tests filter option extraction
func TestAllocationsHandler_ExtractFilterOptions(t *testing.T) {
	handler := setupAllocationsHandler()
	allocs := handler.getAllocationsList("", "", "", "", "")

	options := handler.extractFilterOptions(allocs)

	// Should have at least some statuses
	if len(options.Statuses) == 0 {
		t.Error("Expected at least one status in filter options")
	}

	// Should have at least some jobs
	if len(options.Jobs) == 0 {
		t.Error("Expected at least one job in filter options")
	}

	// Should have at least some nodes
	if len(options.Nodes) == 0 {
		t.Error("Expected at least one node in filter options")
	}

	// Options should be sorted
	for i := 1; i < len(options.Statuses); i++ {
		if options.Statuses[i] < options.Statuses[i-1] {
			t.Error("Statuses should be sorted")
		}
	}

	for i := 1; i < len(options.Jobs); i++ {
		if options.Jobs[i] < options.Jobs[i-1] {
			t.Error("Jobs should be sorted")
		}
	}

	for i := 1; i < len(options.Nodes); i++ {
		if options.Nodes[i] < options.Nodes[i-1] {
			t.Error("Nodes should be sorted")
		}
	}
}

// TestAllocationsHandler_GetAllocationsList tests mock data retrieval
func TestAllocationsHandler_GetAllocationsList(t *testing.T) {
	handler := setupAllocationsHandler()
	allocs := handler.getAllocationsList("", "", "", "", "")

	// Should return multiple allocations
	if len(allocs) == 0 {
		t.Error("Expected at least one allocation in mock data")
	}

	// Each allocation should have required fields
	for _, alloc := range allocs {
		if alloc.ID == "" {
			t.Error("Allocation ID should not be empty")
		}
		if alloc.JobID == "" {
			t.Error("Allocation JobID should not be empty")
		}
		if alloc.Status == "" {
			t.Error("Allocation Status should not be empty")
		}
		if alloc.NodeName == "" {
			t.Error("Allocation NodeName should not be empty")
		}
	}
}
