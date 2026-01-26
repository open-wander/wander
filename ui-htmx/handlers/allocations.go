package handlers

import (
	"math"
	"net/http"
	"sort"
	"strconv"
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

	// Parse query parameters
	query := r.URL.Query()
	statusFilter := query.Get("status")
	jobFilter := query.Get("job")
	nodeFilter := query.Get("node")
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")
	pageStr := query.Get("page")
	perPageStr := query.Get("per_page")

	// Default sort
	if sortColumn == "" {
		sortColumn = "created"
	}
	if sortDir == "" {
		sortDir = "desc"
	}

	// Default pagination
	page := 1
	perPage := 25
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	// Get all allocations
	allAllocations := h.getAllocationsList()

	// Extract filter options from all allocations (before filtering)
	filterOptions := h.extractFilterOptions(allAllocations)

	// Apply filters
	filteredAllocations := h.filterAllocations(allAllocations, statusFilter, jobFilter, nodeFilter, searchQuery)

	// Apply sorting
	h.sortAllocations(filteredAllocations, sortColumn, sortDir)

	// Calculate pagination
	total := len(filteredAllocations)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	// Apply pagination
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if endIdx > total {
		endIdx = total
	}
	if startIdx > total {
		startIdx = total
	}

	paginatedAllocations := filteredAllocations[startIdx:endIdx]

	// Convert to partial type
	allocationsList := make([]allocationspartials.AllocationSummary, len(paginatedAllocations))
	for i, a := range paginatedAllocations {
		allocationsList[i] = allocationspartials.AllocationSummary{
			ID:        a.ID,
			JobID:     a.JobID,
			TaskGroup: a.TaskGroup,
			NodeID:    a.NodeID,
			NodeName:  a.NodeName,
			Status:    a.Status,
			Created:   a.Created,
		}
	}

	// Build props
	props := allocationspartials.ListProps{
		Allocations: allocationsList,
		FilterOptions: allocationspartials.FilterOptions{
			Statuses: filterOptions.Statuses,
			Jobs:     filterOptions.Jobs,
			Nodes:    filterOptions.Nodes,
		},
		Filters: allocationspartials.ListFilters{
			Status: statusFilter,
			Job:    jobFilter,
			Node:   nodeFilter,
			Search: searchQuery,
		},
		Pagination: allocationspartials.PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		SortColumn: sortColumn,
		SortDir:    sortDir,
	}

	// Check if this is an HTMX request for table update only
	if IsHTMXFromRequest(r) && r.Header.Get("HX-Target") == "allocations-table-container" {
		err := RenderPartial(w, r, allocationspartials.AllocationsTable(props))
		if err != nil {
			h.handler.logger.Error("failed to render allocations table", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Render the allocations list page
	err := Render(w, r,
		allocations.List(props),                // Full page with layout
		allocationspartials.ListContent(props), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render allocations list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// FilterOptions holds the available filter options
type FilterOptions struct {
	Statuses []string
	Jobs     []string
	Nodes    []string
}

// extractFilterOptions extracts unique values for filter dropdowns
func (h *AllocationsHandler) extractFilterOptions(allocs []AllocationSummary) FilterOptions {
	statusSet := make(map[string]bool)
	jobSet := make(map[string]bool)
	nodeSet := make(map[string]bool)

	for _, a := range allocs {
		statusSet[a.Status] = true
		jobSet[a.JobID] = true
		nodeSet[a.NodeName] = true
	}

	// Convert to sorted slices
	statuses := make([]string, 0, len(statusSet))
	for s := range statusSet {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)

	jobs := make([]string, 0, len(jobSet))
	for j := range jobSet {
		jobs = append(jobs, j)
	}
	sort.Strings(jobs)

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	return FilterOptions{
		Statuses: statuses,
		Jobs:     jobs,
		Nodes:    nodes,
	}
}

// filterAllocations applies filters to the allocations list
func (h *AllocationsHandler) filterAllocations(allocs []AllocationSummary, status, job, node, search string) []AllocationSummary {
	filtered := make([]AllocationSummary, 0, len(allocs))

	for _, a := range allocs {
		// Status filter
		if status != "" && a.Status != status {
			continue
		}

		// Job filter
		if job != "" && a.JobID != job {
			continue
		}

		// Node filter
		if node != "" && a.NodeName != node {
			continue
		}

		// Search filter (case-insensitive)
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(a.ID), searchLower) &&
				!strings.Contains(strings.ToLower(a.JobID), searchLower) &&
				!strings.Contains(strings.ToLower(a.TaskGroup), searchLower) &&
				!strings.Contains(strings.ToLower(a.NodeName), searchLower) &&
				!strings.Contains(strings.ToLower(a.NodeID), searchLower) {
				continue
			}
		}

		filtered = append(filtered, a)
	}

	return filtered
}

// sortAllocations sorts the allocations list by the specified column
func (h *AllocationsHandler) sortAllocations(allocs []AllocationSummary, column, dir string) {
	sort.Slice(allocs, func(i, j int) bool {
		var less bool

		switch column {
		case "id":
			less = allocs[i].ID < allocs[j].ID
		case "job":
			less = allocs[i].JobID < allocs[j].JobID
		case "taskgroup":
			less = allocs[i].TaskGroup < allocs[j].TaskGroup
		case "node":
			less = allocs[i].NodeName < allocs[j].NodeName
		case "status":
			less = allocs[i].Status < allocs[j].Status
		case "created":
			less = allocs[i].Created.Before(allocs[j].Created)
		default:
			less = allocs[i].Created.Before(allocs[j].Created)
		}

		if dir == "desc" {
			return !less
		}
		return less
	})
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
		{
			ID:        "c9dae0f1-a2b3-2cd3-be6f-9a0b1c2d3e4f",
			JobID:     "log-shipper",
			TaskGroup: "shipper",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-36 * time.Hour),
		},
		{
			ID:        "dae0f1a2-b3c4-3de4-cf7a-0b1c2d3e4f5a",
			JobID:     "api-gateway",
			TaskGroup: "api",
			NodeID:    "node-4",
			NodeName:  "worker-04.dc2",
			Status:    "running",
			Created:   now.Add(-60 * time.Hour),
		},
		{
			ID:        "e0f1a2b3-c4d5-4ef5-da8b-1c2d3e4f5a6b",
			JobID:     "batch-processor",
			TaskGroup: "batch",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "complete",
			Created:   now.Add(-200 * time.Hour),
		},
		{
			ID:        "f1a2b3c4-d5e6-5fa6-eb9c-2d3e4f5a6b7c",
			JobID:     "data-processor",
			TaskGroup: "processor",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "failed",
			Created:   now.Add(-12 * time.Hour),
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
				Name:       "nginx",
				State:      "running",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: time.Time{}, // Running, so no finish time
				Restarts:   0,
				Resources: allocationspartials.TaskResources{
					CPU:      256,
					Memory:   512,
					CPUUsage: 35,
					MemUsage: 62,
				},
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
					{
						Type:       "Driver",
						Time:       now.Add(-124 * time.Minute),
						DisplayMsg: "Downloading image nginx:1.25-alpine",
						FailsTask:  false,
					},
					{
						Type:       "Received",
						Time:       now.Add(-126 * time.Minute),
						DisplayMsg: "Task received by client",
						FailsTask:  false,
					},
				},
			},
			{
				Name:       "redis",
				State:      "running",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: time.Time{}, // Running, so no finish time
				Restarts:   2,
				Resources: allocationspartials.TaskResources{
					CPU:      256,
					Memory:   512,
					CPUUsage: 12,
					MemUsage: 78,
				},
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Started",
						Time:       now.Add(-30 * time.Minute),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Restarting",
						Time:       now.Add(-31 * time.Minute),
						DisplayMsg: "Task restarting in 10s",
						FailsTask:  false,
					},
					{
						Type:       "Terminated",
						Time:       now.Add(-32 * time.Minute),
						DisplayMsg: "Exit Code: 137, Signal: 9",
						FailsTask:  false,
					},
					{
						Type:       "Started",
						Time:       now.Add(-1 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Restarting",
						Time:       now.Add(-61 * time.Minute),
						DisplayMsg: "Task restarting in 10s",
						FailsTask:  false,
					},
					{
						Type:       "Terminated",
						Time:       now.Add(-62 * time.Minute),
						DisplayMsg: "Exit Code: 1, OOM Killed",
						FailsTask:  true,
					},
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
				},
			},
			{
				Name:       "sidecar",
				State:      "dead",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: now.Add(-1 * time.Hour),
				Restarts:   0,
				Resources: allocationspartials.TaskResources{
					CPU:      100,
					Memory:   128,
					CPUUsage: 0,
					MemUsage: 0,
				},
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Terminated",
						Time:       now.Add(-1 * time.Hour),
						DisplayMsg: "Exit Code: 0, Signal: 0",
						FailsTask:  false,
					},
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
			CPU:    612,
			Memory: 1152,
			Disk:   2048,
		},
		Events: []allocationspartials.AllocationEvent{
			{
				Type:    "Allocation Healthy",
				Time:    now.Add(-115 * time.Minute),
				Message: "All tasks are running",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-119 * time.Minute),
				Message: "Task 'nginx' started",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-119 * time.Minute),
				Message: "Task 'redis' started",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-120 * time.Minute),
				Message: "Task 'sidecar' started",
			},
			{
				Type:    "Received",
				Time:    now.Add(-2 * time.Hour),
				Message: "Allocation received by client",
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
