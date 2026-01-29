package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

// TestJobsHandlerCreation tests basic handler creation
func TestJobsHandlerCreation(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)

	jobsHandler := NewJobsHandler(h)
	if jobsHandler == nil {
		t.Fatal("NewJobsHandler returned nil")
	}
	if jobsHandler.handler == nil {
		t.Error("JobsHandler.handler is nil")
	}
}

// TestJobsList tests the jobs list endpoint (GET /ui/jobs)
func TestJobsList(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	t.Run("returns 200 with full page content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Full page should contain DOCTYPE
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Full page response should contain DOCTYPE")
		}

		// Should contain job table elements
		if !strings.Contains(body, "web-frontend") {
			t.Error("Response should contain mock job data (web-frontend)")
		}
	})

	t.Run("returns partial content for HTMX requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Partial response should not have DOCTYPE
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Partial response should not contain DOCTYPE")
		}

		// Should still contain job data
		if !strings.Contains(body, "web-frontend") {
			t.Error("Response should contain mock job data (web-frontend)")
		}
	})

	t.Run("filters jobs by search query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?search=web", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should contain jobs matching "web"
		if !strings.Contains(body, "web-frontend") {
			t.Error("Response should contain web-frontend")
		}

		// Should not contain unrelated jobs
		if strings.Contains(body, "backup-job") {
			t.Error("Response should not contain jobs not matching search query")
		}
	})

	t.Run("filters jobs case-insensitively", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?search=API", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should contain jobs matching "api" case-insensitively
		if !strings.Contains(body, "api-gateway") {
			t.Error("Response should contain api-gateway (case-insensitive search)")
		}
	})

	t.Run("sorts jobs by name ascending", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?sort=name&dir=asc", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that jobs appear in sorted order
		// api-gateway should appear before web-frontend
		apiPos := strings.Index(body, "api-gateway")
		webPos := strings.Index(body, "web-frontend")

		if apiPos == -1 || webPos == -1 {
			t.Error("Response should contain both api-gateway and web-frontend")
		}

		if apiPos > webPos {
			t.Error("Jobs should be sorted alphabetically: api-gateway should appear before web-frontend")
		}
	})

	t.Run("sorts jobs by priority descending", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?sort=priority&dir=desc", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// metrics-collector (priority 80) should appear before web-frontend (priority 50)
		metricsPos := strings.Index(body, "metrics-collector")
		webPos := strings.Index(body, "web-frontend")

		if metricsPos == -1 || webPos == -1 {
			t.Error("Response should contain both metrics-collector and web-frontend")
		}

		if metricsPos > webPos {
			t.Error("Jobs should be sorted by priority descending: metrics-collector (80) before web-frontend (50)")
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		// Page 1 with 3 items per page
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?page=1&per_page=3", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Should contain pagination information
		// Count jobs in response - there should be at most 3
		jobCount := strings.Count(body, `web-frontend`) +
			strings.Count(body, `api-gateway`) +
			strings.Count(body, `data-processor`)

		if jobCount == 0 {
			t.Error("Response should contain at least some jobs")
		}
	})

	t.Run("handles page 2 of pagination", func(t *testing.T) {
		// Page 2 with 3 items per page
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?page=2&per_page=3", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("handles invalid pagination parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs?page=invalid&per_page=invalid", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		// Should not error, should use defaults
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 with default pagination, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("defaults to submitted time descending sort", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// data-processor (submitted 1 hour ago) should appear before
		// web-frontend (submitted 24 hours ago)
		dataPos := strings.Index(body, "data-processor")
		webPos := strings.Index(body, "web-frontend")

		if dataPos == -1 || webPos == -1 {
			t.Error("Response should contain both data-processor and web-frontend")
		}

		if dataPos > webPos {
			t.Error("Jobs should be sorted by submitted time descending by default")
		}
	})
}

// TestJobsListMethodNotAllowed tests that non-GET requests to /ui/jobs return 405
func TestJobsListMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ui/jobs", nil)
			w := httptest.NewRecorder()

			jobsHandler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s method, got %d", method, w.Code)
			}
		})
	}
}

// TestJobsDetail tests the job detail endpoint (GET /ui/jobs/{id})
func TestJobsDetail(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	t.Run("returns 200 for valid job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job-123", nil)
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Should contain DOCTYPE for full page
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Full page response should contain DOCTYPE")
		}

		// Should contain job ID
		if !strings.Contains(body, "test-job-123") {
			t.Error("Response should contain job ID")
		}
	})

	t.Run("returns partial content for HTMX requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job-123", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Partial response should not have DOCTYPE
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Partial response should not contain DOCTYPE")
		}

		// Should contain job ID
		if !strings.Contains(body, "test-job-123") {
			t.Error("Response should contain job ID")
		}
	})

	t.Run("contains job header information", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/my-service", nil)
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should contain job name
		if !strings.Contains(body, "example-job-my-service") {
			t.Error("Response should contain job name")
		}

		// Should contain status
		if !strings.Contains(body, "running") {
			t.Error("Response should contain job status")
		}
	})

	t.Run("contains task group information", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/my-service", nil)
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should contain task groups
		if !strings.Contains(body, "web") {
			t.Error("Response should contain 'web' task group")
		}

		if !strings.Contains(body, "cache") {
			t.Error("Response should contain 'cache' task group")
		}
	})

	t.Run("handles job IDs with special characters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/my-job-123_test", nil)
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})
}

// TestJobsDetailMethodNotAllowed tests that non-GET requests to detail return 405
func TestJobsDetailMethodNotAllowed(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/ui/jobs/test-job", nil)
			w := httptest.NewRecorder()

			jobsHandler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s method, got %d", method, w.Code)
			}
		})
	}
}

// TestJobsTabContent tests tab-specific content rendering
func TestJobsTabContent(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	t.Run("renders overview tab for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/overview", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}

		// Should not have DOCTYPE (partial content)
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Tab partial should not contain DOCTYPE")
		}
	})

	t.Run("renders definition tab for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/definition", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("renders versions tab for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/versions", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("renders allocations tab for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/allocations", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("renders allocations tab with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/allocations?page=1&per_page=5", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("renders evaluations tab for HTMX request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/evaluations", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Response body is empty")
		}
	})

	t.Run("returns 404 for unknown tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs/test-job/unknown-tab", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.Detail(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for unknown tab, got %d", w.Code)
		}
	})
}

// TestPaginationHelpers tests the pagination helper functions
func TestPaginationHelpers(t *testing.T) {
	t.Run("parsePage with valid input", func(t *testing.T) {
		page := parsePage("5")
		if page != 5 {
			t.Errorf("Expected page 5, got %d", page)
		}
	})

	t.Run("parsePage with invalid input returns default", func(t *testing.T) {
		page := parsePage("invalid")
		if page != 1 {
			t.Errorf("Expected default page 1, got %d", page)
		}
	})

	t.Run("parsePage with negative input returns default", func(t *testing.T) {
		page := parsePage("-5")
		if page != 1 {
			t.Errorf("Expected default page 1, got %d", page)
		}
	})

	t.Run("parsePage with zero returns default", func(t *testing.T) {
		page := parsePage("0")
		if page != 1 {
			t.Errorf("Expected default page 1, got %d", page)
		}
	})

	t.Run("parsePerPage with valid input", func(t *testing.T) {
		perPage := parsePerPage("25")
		if perPage != 25 {
			t.Errorf("Expected perPage 25, got %d", perPage)
		}
	})

	t.Run("parsePerPage with invalid input returns default", func(t *testing.T) {
		perPage := parsePerPage("invalid")
		if perPage != 10 {
			t.Errorf("Expected default perPage 10, got %d", perPage)
		}
	})

	t.Run("parsePerPage caps at maximum", func(t *testing.T) {
		perPage := parsePerPage("200")
		if perPage != 100 {
			t.Errorf("Expected capped perPage 100, got %d", perPage)
		}
	})

	t.Run("parsePerPage with negative input returns default", func(t *testing.T) {
		perPage := parsePerPage("-10")
		if perPage != 10 {
			t.Errorf("Expected default perPage 10, got %d", perPage)
		}
	})
}

// TestPaginateJobs tests the job pagination logic
func TestPaginateJobs(t *testing.T) {
	jobs := []jobspartials.JobSummary{
		{ID: "job1", Name: "Job 1"},
		{ID: "job2", Name: "Job 2"},
		{ID: "job3", Name: "Job 3"},
		{ID: "job4", Name: "Job 4"},
		{ID: "job5", Name: "Job 5"},
	}

	t.Run("first page", func(t *testing.T) {
		result := paginateJobs(jobs, 1, 2)
		if len(result) != 2 {
			t.Errorf("Expected 2 jobs, got %d", len(result))
		}
		if result[0].ID != "job1" || result[1].ID != "job2" {
			t.Error("First page should contain job1 and job2")
		}
	})

	t.Run("second page", func(t *testing.T) {
		result := paginateJobs(jobs, 2, 2)
		if len(result) != 2 {
			t.Errorf("Expected 2 jobs, got %d", len(result))
		}
		if result[0].ID != "job3" || result[1].ID != "job4" {
			t.Error("Second page should contain job3 and job4")
		}
	})

	t.Run("last page with fewer items", func(t *testing.T) {
		result := paginateJobs(jobs, 3, 2)
		if len(result) != 1 {
			t.Errorf("Expected 1 job, got %d", len(result))
		}
		if result[0].ID != "job5" {
			t.Error("Last page should contain job5")
		}
	})

	t.Run("page beyond available data", func(t *testing.T) {
		result := paginateJobs(jobs, 10, 2)
		if len(result) != 0 {
			t.Errorf("Expected 0 jobs for page beyond data, got %d", len(result))
		}
	})

	t.Run("empty job list", func(t *testing.T) {
		result := paginateJobs([]jobspartials.JobSummary{}, 1, 10)
		if len(result) != 0 {
			t.Errorf("Expected 0 jobs for empty list, got %d", len(result))
		}
	})
}

// TestFilterJobs tests job filtering logic
func TestFilterJobs(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	jobs := []jobspartials.JobSummary{
		{ID: "web-frontend", Name: "web-frontend"},
		{ID: "api-gateway", Name: "api-gateway"},
		{ID: "data-processor", Name: "data-processor"},
		{ID: "cache-service", Name: "cache-service"},
	}

	t.Run("filters by ID", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "web")
		if len(result) != 1 {
			t.Errorf("Expected 1 job matching 'web', got %d", len(result))
		}
		if len(result) > 0 && result[0].ID != "web-frontend" {
			t.Error("Expected web-frontend")
		}
	})

	t.Run("filters by name", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "api")
		if len(result) != 1 {
			t.Errorf("Expected 1 job matching 'api', got %d", len(result))
		}
		if len(result) > 0 && result[0].ID != "api-gateway" {
			t.Error("Expected api-gateway")
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "WEB")
		if len(result) != 1 {
			t.Errorf("Expected 1 job matching 'WEB' (case insensitive), got %d", len(result))
		}
		if len(result) > 0 && result[0].ID != "web-frontend" {
			t.Error("Expected web-frontend")
		}
	})

	t.Run("partial match", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "service")
		if len(result) != 1 {
			t.Errorf("Expected 1 job matching 'service', got %d", len(result))
		}
		if len(result) > 0 && result[0].ID != "cache-service" {
			t.Error("Expected cache-service")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "nonexistent")
		if len(result) != 0 {
			t.Errorf("Expected 0 jobs matching 'nonexistent', got %d", len(result))
		}
	})

	t.Run("empty query returns all", func(t *testing.T) {
		result := jobsHandler.filterJobs(jobs, "")
		if len(result) != len(jobs) {
			t.Errorf("Expected all %d jobs for empty query, got %d", len(jobs), len(result))
		}
	})
}

// TestSortJobs tests job sorting logic
func TestSortJobs(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	t.Run("sorts by name ascending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Name: "zebra"},
			{Name: "alpha"},
			{Name: "beta"},
		}

		jobsHandler.sortJobs(jobs, "name", "asc")

		if jobs[0].Name != "alpha" || jobs[1].Name != "beta" || jobs[2].Name != "zebra" {
			t.Error("Jobs not sorted by name ascending")
		}
	})

	t.Run("sorts by name descending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Name: "alpha"},
			{Name: "beta"},
			{Name: "zebra"},
		}

		jobsHandler.sortJobs(jobs, "name", "desc")

		if jobs[0].Name != "zebra" || jobs[1].Name != "beta" || jobs[2].Name != "alpha" {
			t.Error("Jobs not sorted by name descending")
		}
	})

	t.Run("sorts by priority ascending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Priority: 50},
			{Priority: 10},
			{Priority: 80},
		}

		jobsHandler.sortJobs(jobs, "priority", "asc")

		if jobs[0].Priority != 10 || jobs[1].Priority != 50 || jobs[2].Priority != 80 {
			t.Error("Jobs not sorted by priority ascending")
		}
	})

	t.Run("sorts by priority descending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Priority: 50},
			{Priority: 10},
			{Priority: 80},
		}

		jobsHandler.sortJobs(jobs, "priority", "desc")

		if jobs[0].Priority != 80 || jobs[1].Priority != 50 || jobs[2].Priority != 10 {
			t.Error("Jobs not sorted by priority descending")
		}
	})

	t.Run("sorts by status ascending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Status: "running"},
			{Status: "dead"},
			{Status: "pending"},
		}

		jobsHandler.sortJobs(jobs, "status", "asc")

		if jobs[0].Status != "dead" || jobs[1].Status != "pending" || jobs[2].Status != "running" {
			t.Error("Jobs not sorted by status ascending")
		}
	})

	t.Run("sorts by type ascending", func(t *testing.T) {
		jobs := []jobspartials.JobSummary{
			{Type: "service"},
			{Type: "batch"},
			{Type: "system"},
		}

		jobsHandler.sortJobs(jobs, "type", "asc")

		if jobs[0].Type != "batch" || jobs[1].Type != "service" || jobs[2].Type != "system" {
			t.Error("Jobs not sorted by type ascending")
		}
	})
}

// TestHTMXHeaderDetection tests HTMX request detection
func TestHTMXHeaderDetection(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	jobsHandler := NewJobsHandler(h)

	t.Run("detects HTMX request header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		body := w.Body.String()

		// HTMX request should not include DOCTYPE
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("HTMX request should return partial content without DOCTYPE")
		}
	})

	t.Run("normal request without HTMX header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
		w := httptest.NewRecorder()

		jobsHandler.List(w, req)

		body := w.Body.String()

		// Normal request should include DOCTYPE
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Normal request should return full page with DOCTYPE")
		}
	})
}
