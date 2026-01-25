package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/allocations"
	"github.com/open-wander/wander/ui-htmx/templates/partials/allocationspartials"
)

// AllocationSummary represents a summary view of an allocation for list display.
// This structure will eventually be populated from the Nomad API.
type AllocationSummary struct {
	ID        string
	JobID     string
	TaskGroup string
	NodeID    string
	NodeName  string
	Status    string
	Created   time.Time
}

// AllocationsHandler handles requests for the allocations list page.
type AllocationsHandler struct {
	handler *Handler
}

// NewAllocationsHandler creates a new allocations handler.
func NewAllocationsHandler(h *Handler) *AllocationsHandler {
	return &AllocationsHandler{
		handler: h,
	}
}

// ServeHTTP handles GET requests to the allocations pages.
// It routes to either the list page or detail page based on the URL path.
func (h *AllocationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Handle /ui/allocations exactly (list page)
	if path == "/ui/allocations" {
		h.List(w, r)
		return
	}

	// Handle /ui/allocations/{id} (detail page)
	if strings.HasPrefix(path, "/ui/allocations/") {
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the allocations list page
func (h *AllocationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("allocations list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock allocation data
	allocationsList := h.getAllocationsList()

	// Render the allocations list page
	err := Render(w, r,
		allocations.List(allocationsList),                // Full page with layout
		allocationspartials.ListContent(allocationsList), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render allocations list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getAllocationsList returns mock allocation data.
// This will be replaced with actual Nomad API calls in the future.
func (h *AllocationsHandler) getAllocationsList() []AllocationSummary {
	now := time.Now()

	return []AllocationSummary{
		{
			ID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			JobID:     "web-frontend",
			TaskGroup: "web",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-24 * time.Hour),
		},
		{
			ID:        "b2c3d4e5-f6a7-5b6c-ad9e-2f3a4b5c6d7e",
			JobID:     "web-frontend",
			TaskGroup: "web",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-24 * time.Hour),
		},
		{
			ID:        "c3d4e5f6-a7b8-6c7d-be0f-3a4b5c6d7e8f",
			JobID:     "api-gateway",
			TaskGroup: "api",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-48 * time.Hour),
		},
		{
			ID:        "d4e5f6a7-b8c9-7d8e-cf1a-4b5c6d7e8f9a",
			JobID:     "data-processor",
			TaskGroup: "processor",
			NodeID:    "node-3",
			NodeName:  "worker-03.dc1",
			Status:    "pending",
			Created:   now.Add(-1 * time.Hour),
		},
		{
			ID:        "e5f6a7b8-c9da-8e9f-da2b-5c6d7e8f9a0b",
			JobID:     "cache-service",
			TaskGroup: "cache",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-72 * time.Hour),
		},
		{
			ID:        "f6a7b8c9-dae0-9fa0-eb3c-6d7e8f9a0b1c",
			JobID:     "backup-job",
			TaskGroup: "backup",
			NodeID:    "node-4",
			NodeName:  "worker-04.dc2",
			Status:    "failed",
			Created:   now.Add(-120 * time.Hour),
		},
		{
			ID:        "a7b8c9da-e0f1-0ab1-fc4d-7e8f9a0b1c2d",
			JobID:     "metrics-collector",
			TaskGroup: "collector",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-96 * time.Hour),
		},
		{
			ID:        "b8c9dae0-f1a2-1bc2-ad5e-8f9a0b1c2d3e",
			JobID:     "web-frontend",
			TaskGroup: "cache",
			NodeID:    "node-3",
			NodeName:  "worker-03.dc1",
			Status:    "complete",
			Created:   now.Add(-168 * time.Hour),
		},
	}
}

// Detail handles the allocation detail page
func (h *AllocationsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract allocation ID from path
	allocID := strings.TrimPrefix(r.URL.Path, "/ui/allocations/")
	allocID = strings.TrimSuffix(allocID, "/")

	// Handle tab-specific requests (e.g., /ui/allocations/{id}/tasks)
	parts := strings.Split(allocID, "/")
	actualAllocID := parts[0]
	tabName := ""
	if len(parts) > 1 {
		tabName = parts[1]
	}

	// Log the request
	h.handler.logger.Debug("allocation detail request", "allocID", actualAllocID, "tab", tabName, "htmx", IsHTMXFromRequest(r))

	// If this is a tab content request, return just the tab partial
	if tabName != "" && IsHTMXFromRequest(r) {
		h.renderTabContent(w, r, actualAllocID, tabName)
		return
	}

	// Mock allocation detail data
	allocDetail := h.getAllocationDetail(actualAllocID)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		allocations.Detail(allocDetail),                // Full page with layout
		allocationspartials.DetailContent(allocDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render allocation detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getAllocationDetail returns mock allocation detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *AllocationsHandler) getAllocationDetail(allocID string) allocationspartials.AllocationDetail {
	now := time.Now()

	return allocationspartials.AllocationDetail{
		ID:        allocID,
		JobID:     "web-frontend",
		TaskGroup: "web",
		NodeID:    "node-1234",
		NodeName:  "worker-01.example.com",
		Status:    "running",
		Created:   now.Add(-2 * time.Hour),
		Modified:  now.Add(-5 * time.Minute),
		Tasks: []allocationspartials.TaskStatus{
			{
				Name:   "nginx",
				State:  "running",
				Failed: false,
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Task Setup",
						Time:       now.Add(-125 * time.Minute),
						DisplayMsg: "Building Task Directory",
						FailsTask:  false,
					},
				},
			},
			{
				Name:   "redis",
				State:  "running",
				Failed: false,
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
				},
			},
		},
		Resources: allocationspartials.ResourceUsage{
			CPU:    512,
			Memory: 1024,
			Disk:   2048,
		},
		Events: []allocationspartials.AllocationEvent{
			{
				Type:    "Received",
				Time:    now.Add(-2 * time.Hour),
				Message: "Allocation received by client",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-115 * time.Minute),
				Message: "Task 'nginx' started",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-115 * time.Minute),
				Message: "Task 'redis' started",
			},
		},
	}
}

// renderTabContent handles rendering individual tab content
func (h *AllocationsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, allocID, tabName string) {
	// Get allocation detail data
	allocDetail := h.getAllocationDetail(allocID)

	var err error
	switch tabName {
	case "tasks":
		err = RenderPartial(w, r, allocationspartials.TasksTab(allocDetail))
	case "logs":
		err = RenderPartial(w, r, allocationspartials.LogsTab())
	case "files":
		err = RenderPartial(w, r, allocationspartials.FilesTab())
	case "raw":
		err = RenderPartial(w, r, allocationspartials.RawTab())
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		h.handler.logger.Error("failed to render tab content", "tab", tabName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
