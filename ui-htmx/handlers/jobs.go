package handlers

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/jobs"
	"github.com/open-wander/wander/ui-htmx/templates/partials/jobspartials"
	"github.com/open-wander/wander/ui-htmx/templates/partials/taskgrouppartials"
)

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

// ServeHTTP handles requests to the jobs pages.
// It renders the jobs list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *JobsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/jobs exactly (list page)
	if path == "/ui/jobs" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/jobs/{id}/allocations/poll (polling endpoint for allocations table)
	if strings.HasSuffix(path, "/allocations/poll") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.AllocationsPoll(w, r)
		return
	}

	// Handle /ui/jobs/{id}/versions/{version}/revert (revert to version)
	if strings.Contains(path, "/versions/") && strings.HasSuffix(path, "/revert") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.RevertToVersion(w, r)
		return
	}

	// Handle /ui/jobs/{id}/versions/{v1}/diff/{v2} (version diff view)
	if strings.Contains(path, "/versions/") && strings.Contains(path, "/diff/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.VersionDiff(w, r)
		return
	}

	// Handle /ui/jobs/{id}/taskgroups/{tgname}/scale (scale action)
	if strings.Contains(path, "/taskgroups/") && strings.HasSuffix(path, "/scale") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.ScaleTaskGroup(w, r)
		return
	}

	// Handle /ui/jobs/{id}/taskgroups/{tgname} (task group detail page)
	if strings.Contains(path, "/taskgroups/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.TaskGroup(w, r)
		return
	}

	// Handle /ui/jobs/{id}/plan (plan job)
	if strings.HasSuffix(path, "/plan") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PlanJob(w, r)
		return
	}

	// Handle /ui/jobs/{id}/run (run job)
	if strings.HasSuffix(path, "/run") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.RunJob(w, r)
		return
	}

	// Handle /ui/jobs/{id} (detail page)
	if strings.HasPrefix(path, "/ui/jobs/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
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

	// Parse search query from query string
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))

	// Filter jobs by search query if provided
	if searchQuery != "" {
		jobsList = h.filterJobs(jobsList, searchQuery)
	}

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

	// Parse pagination parameters
	page := parsePage(r.URL.Query().Get("page"))
	perPage := parsePerPage(r.URL.Query().Get("per_page"))

	// Calculate total before pagination
	total := len(jobsList)

	// Calculate pagination metadata
	pagination := jobspartials.PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	}

	// Paginate the jobs list
	jobsList = paginateJobs(jobsList, page, perPage)

	// Build layout props with app context
	layoutProps := h.handler.BaseLayoutProps(
		"Jobs - Wander",
		"View and manage Nomad jobs",
		"/ui/jobs",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Jobs", URL: ""},
		},
	)

	// Render the jobs list page
	err := Render(w, r,
		jobs.List(layoutProps, jobsList, sortColumn, sortDir, searchQuery, pagination),   // Full page with layout
		jobspartials.ListContent(jobsList, sortColumn, sortDir, searchQuery, pagination), // Partial content for HTMX requests
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

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		jobDetail.Name+" - Job Detail - Wander",
		"Job detail for "+jobDetail.Name,
		"/ui/jobs/"+actualJobID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Jobs", URL: "/ui/jobs"},
			{Label: jobDetail.Name, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		jobs.Detail(layoutProps, jobDetail),                // Full page with layout
		jobspartials.DetailContent(jobDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render job detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getJobsList fetches jobs from the Nomad API.
// Falls back to empty list if API is unavailable.
func (h *JobsHandler) getJobsList() []jobspartials.JobSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty job list")
		return []jobspartials.JobSummary{}
	}

	// Fetch jobs from the API
	jobs, _, err := client.Jobs().List(nil)
	if err != nil {
		h.handler.logger.Error("failed to fetch jobs from Nomad API", "error", err)
		return []jobspartials.JobSummary{}
	}

	// Convert API response to our summary type
	result := make([]jobspartials.JobSummary, 0, len(jobs))
	for _, job := range jobs {
		submitted := time.Time{}
		if job.SubmitTime > 0 {
			submitted = time.Unix(0, job.SubmitTime)
		}

		result = append(result, jobspartials.JobSummary{
			ID:        job.ID,
			Name:      job.Name,
			Status:    job.Status,
			Type:      job.Type,
			Priority:  job.Priority,
			Submitted: submitted,
		})
	}

	return result
}

// filterJobs filters the jobs list based on search query.
// The search is case-insensitive and matches against job ID and name.
func (h *JobsHandler) filterJobs(jobsList []jobspartials.JobSummary, query string) []jobspartials.JobSummary {
	if query == "" {
		return jobsList
	}

	query = strings.ToLower(query)
	filtered := make([]jobspartials.JobSummary, 0, len(jobsList))

	for _, job := range jobsList {
		// Match against ID or Name (case-insensitive)
		if strings.Contains(strings.ToLower(job.ID), query) ||
			strings.Contains(strings.ToLower(job.Name), query) {
			filtered = append(filtered, job)
		}
	}

	return filtered
}

// sortJobs sorts the jobs list based on the specified column and direction.
func (h *JobsHandler) sortJobs(jobsList []jobspartials.JobSummary, sortColumn, sortDir string) {
	sort.Slice(jobsList, func(i, j int) bool {
		var less bool

		switch sortColumn {
		case "name":
			less = jobsList[i].Name < jobsList[j].Name
		case "status":
			less = jobsList[i].Status < jobsList[j].Status
		case "type":
			less = jobsList[i].Type < jobsList[j].Type
		case "priority":
			less = jobsList[i].Priority < jobsList[j].Priority
		case "submitted":
			less = jobsList[i].Submitted.Before(jobsList[j].Submitted)
		default:
			// Default to sorting by submitted time descending
			less = jobsList[i].Submitted.After(jobsList[j].Submitted)
		}

		// Reverse if descending
		if sortDir == "desc" {
			return !less
		}
		return less
	})
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
		jobDef := h.getJobDefinition(jobID)
		err = RenderPartial(w, r, jobspartials.DefinitionTab(jobDef))
	case "versions":
		versions := h.getJobVersions(jobID)
		err = RenderPartial(w, r, jobspartials.VersionsTab(versions, jobID))
	case "allocations":
		// Parse pagination parameters for allocations
		page := parsePage(r.URL.Query().Get("page"))
		perPage := parsePerPage(r.URL.Query().Get("per_page"))
		allocData := h.getJobAllocations(jobID, page, perPage)
		allocData.LastUpdated = time.Now()
		err = RenderPartial(w, r, jobspartials.AllocationsTabWithData(allocData))
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

// getJobVersions returns mock job version data.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getJobVersions(jobID string) []jobspartials.JobVersion {
	now := time.Now()

	return []jobspartials.JobVersion{
		{
			Version:    4,
			Stable:     true,
			SubmitTime: now.Add(-2 * time.Hour),
			Active:     true,
			Diff:       "/ui/jobs/" + jobID + "/versions/4/diff",
		},
		{
			Version:    3,
			Stable:     true,
			SubmitTime: now.Add(-24 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/3/diff",
		},
		{
			Version:    2,
			Stable:     false,
			SubmitTime: now.Add(-48 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/2/diff",
		},
		{
			Version:    1,
			Stable:     true,
			SubmitTime: now.Add(-72 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/1/diff",
		},
		{
			Version:    0,
			Stable:     false,
			SubmitTime: now.Add(-96 * time.Hour),
			Active:     false,
			Diff:       "", // No diff for version 0 (initial version)
		},
	}
}

// getJobDefinition returns mock job definition data.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getJobDefinition(jobID string) jobspartials.JobDefinition {
	// Mock HCL job definition
	hclContent := `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  # Update strategy for rolling deployments
  update {
    max_parallel      = 1
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 0
  }

  # Task group for web servers
  group "web" {
    count = 3

    # Restart policy on task failure
    restart {
      attempts = 3
      interval = "5m"
      delay    = "25s"
      mode     = "delay"
    }

    # Network configuration
    network {
      port "http" {
        to = 8080
      }
      port "metrics" {
        to = 9090
      }
    }

    # Service registration for Consul
    service {
      name = "` + jobID + `-web"
      port = "http"
      tags = ["urlprefix-/"]

      check {
        name     = "health"
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # Main web task
    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]

        volumes = [
          "local/nginx.conf:/etc/nginx/nginx.conf",
        ]
      }

      # Resource requirements
      resources {
        cpu    = 500
        memory = 256
      }

      # Template for nginx config
      template {
        data = <<EOF
events {
  worker_connections 1024;
}

http {
  server {
    listen 8080;

    location / {
      root /usr/share/nginx/html;
      index index.html;
    }

    location /health {
      return 200 "healthy\n";
      add_header Content-Type text/plain;
    }
  }
}
EOF
        destination = "local/nginx.conf"
      }

      # Environment variables
      env {
        ENVIRONMENT = "production"
        LOG_LEVEL   = "info"
      }
    }

    # Redis sidecar task
    task "redis" {
      driver = "docker"

      config {
        image = "redis:7-alpine"
        ports = ["metrics"]
      }

      resources {
        cpu    = 250
        memory = 128
      }

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
    }
  }

  # Constraints
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  constraint {
    operator  = "distinct_hosts"
    value     = "true"
  }
}`

	return jobspartials.JobDefinition{
		JobID:    jobID,
		Content:  hclContent,
		Format:   "hcl",
		Version:  4,
		ReadOnly: false,
	}
}

// VersionDiff handles requests for version diff view
// Route: GET /ui/jobs/{id}/versions/{v1}/diff/{v2}
func (h *JobsHandler) VersionDiff(w http.ResponseWriter, r *http.Request) {
	// Extract job ID and version numbers from path
	// Expected format: /ui/jobs/{jobID}/versions/{v1}/diff/{v2}
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")

	// Split by /versions/
	parts := strings.Split(path, "/versions/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	jobID := parts[0]
	versionPart := parts[1]

	// Split by /diff/
	versionParts := strings.Split(versionPart, "/diff/")
	if len(versionParts) != 2 {
		http.NotFound(w, r)
		return
	}

	newVersionStr := versionParts[0]
	oldVersionStr := strings.TrimSuffix(versionParts[1], "/")

	newVersion, err := strconv.Atoi(newVersionStr)
	if err != nil {
		http.Error(w, "Invalid new version number", http.StatusBadRequest)
		return
	}

	oldVersion, err := strconv.Atoi(oldVersionStr)
	if err != nil {
		http.Error(w, "Invalid old version number", http.StatusBadRequest)
		return
	}

	h.handler.logger.Debug("version diff request",
		"jobID", jobID,
		"newVersion", newVersion,
		"oldVersion", oldVersion,
		"htmx", IsHTMXFromRequest(r),
	)

	// Get mock definitions for both versions
	oldDef := h.getJobDefinitionForVersion(jobID, oldVersion)
	newDef := h.getJobDefinitionForVersion(jobID, newVersion)

	// Compute the diff
	diffLines := computeDiff(oldDef, newDef)
	added, removed := countDiffStats(diffLines)

	diff := jobspartials.VersionDiff{
		JobID:        jobID,
		OldVersion:   oldVersion,
		NewVersion:   newVersion,
		Lines:        diffLines,
		TotalAdded:   added,
		TotalRemoved: removed,
	}

	// Render the diff modal
	err = RenderPartial(w, r, jobspartials.VersionDiffModal(diff))
	if err != nil {
		h.handler.logger.Error("failed to render version diff", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getJobDefinitionForVersion returns mock job definition for a specific version.
// This simulates version differences for demonstration purposes.
func (h *JobsHandler) getJobDefinitionForVersion(jobID string, version int) string {
	// Base definition
	base := `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  update {
    max_parallel      = 1
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 0
  }

  group "web" {
    count = 3

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`

	// Simulate changes based on version
	switch version {
	case 0:
		// Initial version - minimal config
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 1

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.20-alpine"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}`
	case 1:
		// Version 1 - added network config
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.20-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}`
	case 2:
		// Version 2 - increased resources
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`
	case 3:
		// Version 3 - added second datacenter
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`
	default:
		// Latest version (4+) - added update stanza and increased count
		return base
	}
}

// ScaleTaskGroup handles the scale task group action
func (h *JobsHandler) ScaleTaskGroup(w http.ResponseWriter, r *http.Request) {
	// Extract job ID and task group name from path
	// Expected format: /ui/jobs/{jobID}/taskgroups/{tgName}/scale
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/scale")
	parts := strings.Split(path, "/taskgroups/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	jobID := parts[0]
	tgName := parts[1]

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(renderScaleError("Invalid form data")))
		return
	}

	// Get the count from the form
	countStr := r.FormValue("count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 0 {
		h.handler.logger.Error("invalid count value", "count", countStr, "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(renderScaleError("Invalid count value")))
		return
	}

	// Log the scaling operation
	h.handler.logger.Info("scaling task group",
		"jobID", jobID,
		"taskGroup", tgName,
		"newCount", count,
	)

	// TODO: In a real implementation, this would call the Nomad API:
	// client.Jobs().Scale(jobID, tgName, &count, "Scaling via Wander UI", false, nil, nil)

	// For now, simulate the scaling operation with a success response
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(renderScaleSuccess(tgName, count)))
}

// renderScaleSuccess returns the HTML for a successful scale operation
func renderScaleSuccess(taskGroupName string, newCount int) string {
	return `<div class="rounded-lg bg-green-50 border border-green-200 p-4">
		<div class="flex items-center">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-green-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>
			</div>
			<div class="ml-3">
				<p class="text-sm font-medium text-green-800">
					Task group <span class="font-semibold">` + taskGroupName + `</span> scaled to ` + strconv.Itoa(newCount) + ` instance(s)
				</p>
			</div>
		</div>
	</div>`
}

// renderScaleError returns the HTML for a failed scale operation
func renderScaleError(message string) string {
	return `<div class="rounded-lg bg-red-50 border border-red-200 p-4">
		<div class="flex items-center">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-red-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"/>
				</svg>
			</div>
			<div class="ml-3">
				<p class="text-sm font-medium text-red-800">
					Failed to scale: ` + message + `
				</p>
			</div>
		</div>
	</div>`
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
		jobs.TaskGroup(jobID, taskGroup),                  // Full page with layout
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

// parsePage parses the page query parameter and returns a valid page number.
// Returns 1 if the parameter is invalid or not provided.
func parsePage(pageStr string) int {
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// parsePerPage parses the per_page query parameter and returns a valid per page value.
// Returns 10 if the parameter is invalid or not provided.
// Maximum value is capped at 100.
func parsePerPage(perPageStr string) int {
	perPage, err := strconv.Atoi(perPageStr)
	if err != nil || perPage < 1 {
		return 10
	}
	if perPage > 100 {
		return 100
	}
	return perPage
}

// paginateJobs returns a slice of jobs for the requested page.
// Returns an empty slice if the page is beyond available data.
func paginateJobs(jobs []jobspartials.JobSummary, page, perPage int) []jobspartials.JobSummary {
	start := (page - 1) * perPage
	if start >= len(jobs) {
		return []jobspartials.JobSummary{}
	}

	end := start + perPage
	if end > len(jobs) {
		end = len(jobs)
	}

	return jobs[start:end]
}

// getJobAllocations returns mock allocation data for a specific job.
// This will be replaced with actual Nomad API calls in the future.
func (h *JobsHandler) getJobAllocations(jobID string, page, perPage int) jobspartials.AllocationsTabData {
	now := time.Now()

	// Generate mock allocations based on job ID
	// In reality, this would filter allocations from the Nomad API
	allAllocations := h.generateMockAllocations(jobID, now)

	// Calculate pagination
	total := len(allAllocations)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// Apply pagination
	start := (page - 1) * perPage
	end := start + perPage
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}

	var paginatedAllocs []jobspartials.JobAllocation
	if start < total {
		paginatedAllocs = allAllocations[start:end]
	}

	return jobspartials.AllocationsTabData{
		JobID:       jobID,
		Allocations: paginatedAllocs,
		Pagination: jobspartials.JobAllocationsPagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// AllocationsPoll handles the polling endpoint for allocations table body.
// This returns just the table body for efficient HTMX updates.
func (h *JobsHandler) AllocationsPoll(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /ui/jobs/{id}/allocations/poll
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/allocations/poll")
	jobID := path

	// Log the polling request
	h.handler.logger.Debug("allocations poll request", "jobID", jobID)

	// Parse pagination parameters
	page := parsePage(r.URL.Query().Get("page"))
	perPage := parsePerPage(r.URL.Query().Get("per_page"))

	// Get allocation data
	allocData := h.getJobAllocations(jobID, page, perPage)
	allocData.LastUpdated = time.Now()

	// Render just the table body partial
	err := RenderPartial(w, r, jobspartials.AllocationsTableBody(allocData))
	if err != nil {
		h.handler.logger.Error("failed to render allocations poll", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// generateMockAllocations generates mock allocation data for testing.
// This simulates allocations that would be returned from the Nomad API.
func (h *JobsHandler) generateMockAllocations(jobID string, now time.Time) []jobspartials.JobAllocation {
	// Task groups and statuses for variation
	taskGroups := []string{"web", "api", "cache", "worker"}
	statuses := []string{"running", "running", "running", "pending", "complete", "failed"}
	nodes := []struct {
		id   string
		name string
	}{
		{"node-1a2b3c4d", "worker-01.dc1"},
		{"node-2b3c4d5e", "worker-02.dc1"},
		{"node-3c4d5e6f", "worker-03.dc1"},
		{"node-4d5e6f7g", "worker-04.dc2"},
		{"node-5e6f7g8h", "worker-05.dc2"},
	}

	// Generate 12 allocations for this job
	allocations := make([]jobspartials.JobAllocation, 0, 12)

	// Use a deterministic seed based on job ID for consistent mock data
	r := rand.New(rand.NewSource(hashString(jobID)))

	for i := 0; i < 12; i++ {
		// Generate a pseudo-random allocation ID
		allocID := generateMockID(r)
		shortID := allocID[:8]

		// Select mock data
		tgIndex := r.Intn(len(taskGroups))
		statusIndex := r.Intn(len(statuses))
		nodeIndex := r.Intn(len(nodes))

		// Generate resource usage (random but realistic)
		cpuMax := 500 + r.Intn(500)
		cpuUsed := int(float64(cpuMax) * (0.2 + r.Float64()*0.7))
		memMax := 256 + r.Intn(768)
		memUsed := int(float64(memMax) * (0.3 + r.Float64()*0.6))

		// Vary creation time
		hoursAgo := r.Intn(168) + 1 // 1 to 168 hours (1 week)

		alloc := jobspartials.JobAllocation{
			ID:        allocID,
			ShortID:   shortID,
			TaskGroup: taskGroups[tgIndex],
			NodeID:    nodes[nodeIndex].id,
			NodeName:  nodes[nodeIndex].name,
			Status:    statuses[statusIndex],
			CPU:       cpuUsed,
			CPUMax:    cpuMax,
			Memory:    memUsed,
			MemoryMax: memMax,
			Created:   now.Add(-time.Duration(hoursAgo) * time.Hour),
		}

		allocations = append(allocations, alloc)
	}

	// Sort by created time (newest first)
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].Created.After(allocations[j].Created)
	})

	return allocations
}

// hashString creates a simple hash from a string for deterministic random seeding.
func hashString(s string) int64 {
	var h int64
	for _, c := range s {
		h = h*31 + int64(c)
	}
	return h
}

// generateMockID generates a mock UUID-like ID.
func generateMockID(r *rand.Rand) string {
	const chars = "0123456789abcdef"
	id := make([]byte, 36)
	for i := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			id[i] = '-'
		} else {
			id[i] = chars[r.Intn(len(chars))]
		}
	}
	return string(id)
}

// PlanJobRequest represents the request body for plan endpoint
type PlanJobRequest struct {
	Definition string `json:"definition"`
	Format     string `json:"format"`
}

// PlanJobResponse represents the response from plan endpoint
type PlanJobResponse struct {
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Destroyed int      `json:"destroyed"`
	Diff      string   `json:"diff"`
	Warnings  []string `json:"warnings,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// RunJobRequest represents the request body for run endpoint
type RunJobRequest struct {
	Definition string `json:"definition"`
	Format     string `json:"format"`
}

// RunJobResponse represents the response from run endpoint
type RunJobResponse struct {
	EvalID    string   `json:"evalId"`
	JobID     string   `json:"jobId"`
	Warnings  []string `json:"warnings,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// PlanJob handles POST requests to plan a job (dry run).
// Route: POST /ui/jobs/{id}/plan
func (h *JobsHandler) PlanJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /ui/jobs/{id}/plan
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/plan")
	jobID := path

	// Log the request
	h.handler.logger.Info("plan job request", "jobID", jobID)

	// Parse the request body
	var req PlanJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to decode plan request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{Error: "Invalid request body"})
		return
	}

	// TODO: In a real implementation, this would:
	// 1. Parse the job definition (HCL or JSON)
	// 2. Call the Nomad API: client.Jobs().Plan(job, true, nil)
	// 3. Return the actual plan result

	// Mock plan result for demonstration
	response := PlanJobResponse{
		Created:   1,
		Updated:   2,
		Destroyed: 0,
		Diff: `+/- Job: "` + jobID + `"
+/- Task Group: "web" (2 create, 1 in-place update)
  +/- Task: "nginx" (forces create/destroy update)
    +/- Resources {
      +/- CPU:    500 => 750
      +/- Memory: 256 => 512
    }
+ Task Group: "cache" (1 create)
  + Task: "redis"
    + Resources {
      + CPU:    250
      + Memory: 128
    }`,
		Warnings: []string{},
	}

	// Add a warning if the definition is very long (mock example)
	if len(req.Definition) > 5000 {
		response.Warnings = append(response.Warnings, "Large job definition may take longer to deploy")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode plan response", "error", err)
	}
}

// RunJob handles POST requests to run/submit a job.
// Route: POST /ui/jobs/{id}/run
func (h *JobsHandler) RunJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /ui/jobs/{id}/run
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/run")
	jobID := path

	// Log the request
	h.handler.logger.Info("run job request", "jobID", jobID)

	// Parse the request body
	var req RunJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to decode run request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RunJobResponse{Error: "Invalid request body"})
		return
	}

	// TODO: In a real implementation, this would:
	// 1. Parse the job definition (HCL or JSON)
	// 2. Call the Nomad API: client.Jobs().Register(job, nil)
	// 3. Return the actual evaluation ID

	// Mock run result for demonstration
	// Generate a mock evaluation ID
	mockEvalID := generateMockEvalID()

	response := RunJobResponse{
		EvalID:   mockEvalID,
		JobID:    jobID,
		Warnings: []string{},
	}

	h.handler.logger.Info("job submitted successfully",
		"jobID", jobID,
		"evalID", mockEvalID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode run response", "error", err)
	}
}

// generateMockEvalID generates a mock evaluation ID for testing.
func generateMockEvalID() string {
	const chars = "0123456789abcdef"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := make([]byte, 36)
	for i := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			id[i] = '-'
		} else {
			id[i] = chars[r.Intn(len(chars))]
		}
	}
	return string(id)
}

// RevertToVersionResponse represents the response from the revert endpoint
type RevertToVersionResponse struct {
	EvalID   string `json:"evalId"`
	JobID    string `json:"jobId"`
	Version  int    `json:"version"`
	Error    string `json:"error,omitempty"`
}

// RevertToVersion handles POST requests to revert a job to a previous version.
// Route: POST /ui/jobs/{id}/versions/{version}/revert
func (h *JobsHandler) RevertToVersion(w http.ResponseWriter, r *http.Request) {
	// Extract job ID and version from path: /ui/jobs/{id}/versions/{version}/revert
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/revert")

	// Split by /versions/
	parts := strings.Split(path, "/versions/")
	if len(parts) != 2 {
		h.handler.logger.Error("invalid revert path", "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{Error: "Invalid path"})
		return
	}

	jobID := parts[0]
	versionStr := parts[1]

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.handler.logger.Error("invalid version number", "version", versionStr, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{Error: "Invalid version number"})
		return
	}

	// Log the revert operation
	h.handler.logger.Info("revert job request",
		"jobID", jobID,
		"version", version,
	)

	// TODO: In a real implementation, this would:
	// 1. Fetch the job definition for the specified version from Nomad API
	//    using client.Jobs().Versions(jobID, false, nil)
	// 2. Find the version in the returned list
	// 3. Re-submit the job with that version's configuration
	//    using client.Jobs().Register(job, nil)

	// Mock revert result for demonstration
	mockEvalID := generateMockEvalID()

	response := RevertToVersionResponse{
		EvalID:  mockEvalID,
		JobID:   jobID,
		Version: version,
	}

	h.handler.logger.Info("job reverted successfully",
		"jobID", jobID,
		"version", version,
		"evalID", mockEvalID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode revert response", "error", err)
	}
}
