package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/jobs"
	"github.com/open-wander/wander/ui-htmx/templates/partials/jobspartials"
	"github.com/open-wander/wander/ui-htmx/templates/partials/taskgrouppartials"
)

// JobSummary represents a summary view of a job for list display.
// This structure will eventually be populated from the Nomad API.
type JobSummary struct {
	ID        string
	Name      string
	Status    string
	Type      string
	Priority  int
	Submitted time.Time
}

// JobsHandler handles requests for the jobs list page.
type JobsHandler struct {
	handler *Handler
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(h *Handler) *JobsHandler {
	return &JobsHandler{
		handler: h,
	}
}

// ServeHTTP handles GET requests to the jobs list page.
// It renders the jobs list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *JobsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Handle /ui/jobs exactly (list page)
	if path == "/ui/jobs" {
		h.List(w, r)
		return
	}

	// Handle /ui/jobs/{id}/taskgroups/{tgname} (task group detail page)
	if strings.Contains(path, "/taskgroups/") {
		h.TaskGroup(w, r)
		return
	}

	// Handle /ui/jobs/{id} (detail page)
	if strings.HasPrefix(path, "/ui/jobs/") {
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the jobs list page
func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("jobs list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock job data
	jobsList := h.getJobsList()

	// Parse sort parameters from query string
	sortColumn := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	// Default sort if not specified
	if sortColumn == "" {
		sortColumn = "submitted"
		sortDir = "desc"
	}

	// Sort the jobs list based on parameters
	h.sortJobs(jobsList, sortColumn, sortDir)

	// Render the jobs list page
	err := Render(w, r,
		jobs.List(jobsList, sortColumn, sortDir),                    // Full page with layout
		jobspartials.ListContent(jobsList, sortColumn, sortDir), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render jobs list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Detail handles the job detail page
func (h *JobsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	jobID = strings.TrimSuffix(jobID, "/")

	// Handle tab-specific requests (e.g., /ui/jobs/{id}/overview)
	parts := strings.Split(jobID, "/")
	actualJobID := parts[0]
	tabName := ""
	if len(parts) > 1 {
		// Check if this is a task group URL - should have been handled earlier
		if parts[1] == "taskgroups" {
			http.NotFound(w, r)
			return
		}
		tabName = parts[1]
	}

	// Log the request
	h.handler.logger.Debug("job detail request", "jobID", actualJobID, "tab", tabName, "htmx", IsHTMXFromRequest(r))

	// If this is a tab content request, return just the tab partial
	if tabName != "" && IsHTMXFromRequest(r) {
		h.renderTabContent(w, r, actualJobID, tabName)
		return
	}

	// Mock job detail data
	jobDetail := h.getJobDetail(actualJobID)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		jobs.Detail(jobDetail),                    // Full page with layout
		jobspartials.DetailContent(jobDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render job detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getJobsList returns mock job data.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getJobsList() []JobSummary {
	now := time.Now()

	return []JobSummary{
		{
			ID:        "web-frontend",
			Name:      "web-frontend",
			Status:    "running",
			Type:      "service",
			Priority:  50,
			Submitted: now.Add(-24 * time.Hour),
		},
		{
			ID:        "api-gateway",
			Name:      "api-gateway",
			Status:    "running",
			Type:      "service",
			Priority:  75,
			Submitted: now.Add(-48 * time.Hour),
		},
		{
			ID:        "data-processor",
			Name:      "data-processor",
			Status:    "pending",
			Type:      "batch",
			Priority:  30,
			Submitted: now.Add(-1 * time.Hour),
		},
		{
			ID:        "cache-service",
			Name:      "cache-service",
			Status:    "running",
			Type:      "service",
			Priority:  60,
			Submitted: now.Add(-72 * time.Hour),
		},
		{
			ID:        "backup-job",
			Name:      "backup-job",
			Status:    "dead",
			Type:      "batch",
			Priority:  20,
			Submitted: now.Add(-120 * time.Hour),
		},
		{
			ID:        "metrics-collector",
			Name:      "metrics-collector",
			Status:    "running",
			Type:      "system",
			Priority:  80,
			Submitted: now.Add(-96 * time.Hour),
		},
	}
}

// sortJobs sorts the jobs list based on the specified column and direction.
func (h *JobsHandler) sortJobs(jobsList []JobSummary, sortColumn, sortDir string) {
	// For now, this is a placeholder. In a production implementation,
	// we would use sort.Slice with the appropriate comparison function.
	// Since this is mock data and sorting will be done by the Nomad API
	// in the future, we'll keep this simple for now.

	// TODO: Implement sorting when needed
	// Example:
	// sort.Slice(jobsList, func(i, j int) bool {
	//     switch sortColumn {
	//     case "name":
	//         if sortDir == "asc" {
	//             return jobsList[i].Name < jobsList[j].Name
	//         }
	//         return jobsList[i].Name > jobsList[j].Name
	//     // ... other cases
	//     }
	// })
}

// getJobDetail returns mock job detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getJobDetail(jobID string) jobspartials.JobDetail {
	return jobspartials.JobDetail{
		ID:          jobID,
		Name:        "example-job-" + jobID,
		Status:      "running",
		Type:        "service",
		Priority:    50,
		Datacenters: []string{"dc1", "dc2"},
		TaskGroups: []jobspartials.TaskGroupSummary{
			{
				Name:  "web",
				Count: 3,
				Tasks: []string{"nginx", "redis"},
			},
			{
				Name:  "cache",
				Count: 2,
				Tasks: []string{"redis"},
			},
		},
	}
}

// renderTabContent handles rendering individual tab content
func (h *JobsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, jobID, tabName string) {
	// Get job detail data
	jobDetail := h.getJobDetail(jobID)

	var err error
	switch tabName {
	case "overview":
		err = RenderPartial(w, r, jobspartials.OverviewTab(jobDetail))
	case "definition":
		err = RenderPartial(w, r, jobspartials.DefinitionTab())
	case "versions":
		err = RenderPartial(w, r, jobspartials.VersionsTab())
	case "allocations":
		err = RenderPartial(w, r, jobspartials.AllocationsTab())
	case "evaluations":
		err = RenderPartial(w, r, jobspartials.EvaluationsTab())
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		h.handler.logger.Error("failed to render tab content", "tab", tabName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// TaskGroup handles the task group detail page
func (h *JobsHandler) TaskGroup(w http.ResponseWriter, r *http.Request) {
	// Extract job ID and task group name from path
	// Expected format: /ui/jobs/{jobID}/taskgroups/{tgName}
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	parts := strings.Split(path, "/taskgroups/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	jobID := parts[0]
	tgName := strings.TrimSuffix(parts[1], "/")

	// Log the request
	h.handler.logger.Debug("task group detail request", "jobID", jobID, "taskGroup", tgName, "htmx", IsHTMXFromRequest(r))

	// Get mock task group data
	taskGroup := h.getTaskGroupDetail(jobID, tgName)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		jobs.TaskGroup(jobID, taskGroup),                       // Full page with layout
		taskgrouppartials.DetailContent(jobID, taskGroup), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render task group detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getTaskGroupDetail returns mock task group detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getTaskGroupDetail(jobID, tgName string) taskgrouppartials.TaskGroupDetail {
	// Base task group data
	tg := taskgrouppartials.TaskGroupDetail{
		Name:  tgName,
		Count: 3,
		Tasks: []taskgrouppartials.TaskDetail{
			{
				Name:   "nginx",
				Driver: "docker",
				Resources: taskgrouppartials.ResourceConfig{
					CPU:      500,
					Memory:   256,
					DiskMB:   300,
					Networks: []taskgrouppartials.NetworkResource{{MBits: 10}},
				},
			},
			{
				Name:   "redis",
				Driver: "docker",
				Resources: taskgrouppartials.ResourceConfig{
					CPU:      250,
					Memory:   128,
					DiskMB:   100,
					Networks: []taskgrouppartials.NetworkResource{{MBits: 5}},
				},
			},
		},
	}

	// Add placement failures for demonstration if task group is "cache"
	if tgName == "cache" {
		tg.PlacementFailures = []taskgrouppartials.PlacementFailure{
			{
				Node:      "node-1",
				Reason:    "Constraint \"${attr.kernel.name} = linux\" filtered 0 nodes",
				Timestamp: time.Now().Add(-10 * time.Minute),
			},
			{
				Node:      "node-2",
				Reason:    "Resources exhausted on 1 nodes",
				Timestamp: time.Now().Add(-5 * time.Minute),
			},
		}
	}

	// Add scaling config for service jobs
	if tgName == "web" {
		tg.Scaling = &taskgrouppartials.ScalingConfig{
			Enabled: true,
			Min:     1,
			Max:     10,
			Policy: taskgrouppartials.ScalingPolicy{
				Cooldown:            "30s",
				EvaluationInterval:  "10s",
				TargetCPUPercent:    80,
				TargetMemoryPercent: 85,
			},
		}
	}

	return tg
}
