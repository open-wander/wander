package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
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

	h.handler.RenderNotFound(w, r, NotFoundError{
		Message:     "The job page you requested could not be found.",
		RequestPath: path,
		Suggestions: []string{
			`Go to <a href="/ui/jobs" class="underline">Jobs List</a>`,
			`Return to <a href="/ui/" class="underline">Home</a>`,
		},
	})
}

// List handles the jobs list page
func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("jobs list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	queryParams := r.URL.Query()
	namespace := strings.TrimSpace(queryParams.Get("namespace"))
	if namespace == "" {
		namespace = "default"
	}

	// Check permissions for the requested namespace
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListJobs(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to list jobs",
			Capability: "list-jobs",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'list-jobs' capability",
				"Ensure you are using a valid ACL token with appropriate permissions",
			},
		})
		return
	}

	// Get ACL token from request context (if available)
	token := GetTokenFromRequest(r)

	// Fetch jobs from Nomad API
	jobsList, err := h.getJobsList(r, namespace, token)
	if err != nil {
		// Check if this is a cluster unavailability error first
		if h.handler.CheckClusterError(w, r, err) {
			return
		}
		h.handler.logger.Error("failed to fetch jobs", "error", err, "namespace", namespace)
		http.Error(w, "Failed to fetch jobs from Nomad API", http.StatusInternalServerError)
		return
	}

	// Parse search query from query string
	searchQuery := strings.TrimSpace(queryParams.Get("search"))

	// Filter jobs by search query if provided (client-side)
	if searchQuery != "" {
		jobsList = h.filterJobs(jobsList, searchQuery)
	}

	// Parse sort parameters from query string
	sortColumn := queryParams.Get("sort")
	sortDir := queryParams.Get("dir")

	// Default sort if not specified
	if sortColumn == "" {
		sortColumn = "submitted"
		sortDir = "desc"
	}

	// Sort the jobs list based on parameters
	h.sortJobs(jobsList, sortColumn, sortDir)

	// Parse pagination parameters
	page := parsePage(queryParams.Get("page"))
	perPage := parsePerPage(queryParams.Get("per_page"))

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
	err = Render(w, r,
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

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := actualJobID
	if idx := strings.LastIndex(actualJobID, "@"); idx != -1 {
		jobName = actualJobID[:idx]
		namespace = actualJobID[idx+1:]
	}

	// Log the request
	h.handler.logger.Debug("job detail request", "jobID", actualJobID, "jobName", jobName, "namespace", namespace, "tab", tabName, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read-job capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadJob(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view this job",
			Capability: "read-job",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'read-job' capability",
				"Ensure you are using a valid ACL token with appropriate permissions",
			},
		})
		return
	}

	// If this is a tab content request, return just the tab partial
	if tabName != "" && IsHTMXFromRequest(r) {
		h.renderTabContent(w, r, actualJobID, tabName)
		return
	}

	// Fetch job detail from Nomad API
	jobDetail, err := h.getJobDetail(jobName, namespace, GetTokenFromRequest(r))
	if err != nil {
		// Check if this is a cluster unavailability error first
		if h.handler.CheckClusterError(w, r, err) {
			return
		}
		h.handler.logger.Error("failed to fetch job detail", "error", err, "jobID", actualJobID)
		http.Error(w, "Failed to fetch job from Nomad API", http.StatusInternalServerError)
		return
	}

	if jobDetail == nil {
		http.NotFound(w, r)
		return
	}

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
	err = Render(w, r,
		jobs.Detail(layoutProps, *jobDetail),                // Full page with layout
		jobspartials.DetailContent(*jobDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render job detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getJobsList fetches jobs from the Nomad API with the specified parameters.
// Returns error if API is unavailable or request fails.
func (h *JobsHandler) getJobsList(r *http.Request, namespace, token string) ([]jobspartials.JobSummary, error) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty job list")
		return []jobspartials.JobSummary{}, nil
	}

	// Build query options
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Parse additional filter parameters from query string
	queryParams := r.URL.Query()
	filterExpr := strings.TrimSpace(queryParams.Get("filter"))
	statusFilter := strings.TrimSpace(queryParams.Get("status"))
	typeFilter := strings.TrimSpace(queryParams.Get("type"))

	// Build Nomad filter expression if filters are specified
	var filters []string
	if statusFilter != "" {
		filters = append(filters, fmt.Sprintf("Status == %q", statusFilter))
	}
	if typeFilter != "" {
		filters = append(filters, fmt.Sprintf("Type == %q", typeFilter))
	}
	if filterExpr != "" {
		filters = append(filters, filterExpr)
	}
	if len(filters) > 0 {
		queryOpts.Filter = strings.Join(filters, " and ")
	}

	// Fetch jobs from the API
	h.handler.logger.Debug("fetching jobs from API",
		"namespace", namespace,
		"filter", queryOpts.Filter,
		"hasToken", token != "",
	)

	jobs, _, err := client.Jobs().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch jobs from Nomad API", "error", err)
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	h.handler.logger.Debug("fetched jobs from API", "count", len(jobs))

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

	return result, nil
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

// getJobDetail fetches job detail data from the Nomad API.
// Returns nil if the job is not found, or an error if the API call fails.
func (h *JobsHandler) getJobDetail(jobID, namespace, token string) (*jobspartials.JobDetail, error) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return nil, fmt.Errorf("no Nomad client available")
	}

	// Build query options
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch job from the API
	h.handler.logger.Debug("fetching job from API",
		"jobID", jobID,
		"namespace", namespace,
		"hasToken", token != "",
	)

	job, _, err := client.Jobs().Info(jobID, queryOpts)
	if err != nil {
		// Check if this is a 404 error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			h.handler.logger.Debug("job not found", "jobID", jobID, "namespace", namespace)
			return nil, nil
		}
		h.handler.logger.Error("failed to fetch job from Nomad API", "error", err)
		return nil, fmt.Errorf("failed to fetch job: %w", err)
	}

	// Convert API response to our JobDetail type
	jobDetail := jobspartials.JobDetail{
		ID:          safeString(job.ID),
		Name:        safeString(job.Name),
		Status:      safeString(job.Status),
		Type:        safeString(job.Type),
		Priority:    safeInt(job.Priority),
		Datacenters: job.Datacenters,
		TaskGroups:  make([]jobspartials.TaskGroupSummary, 0, len(job.TaskGroups)),
	}

	// Extract task group summaries
	for _, tg := range job.TaskGroups {
		if tg == nil {
			continue
		}

		// Extract task names from the task group
		taskNames := make([]string, 0, len(tg.Tasks))
		for _, task := range tg.Tasks {
			if task != nil {
				taskNames = append(taskNames, task.Name)
			}
		}

		taskGroupSummary := jobspartials.TaskGroupSummary{
			Name:  safeString(tg.Name),
			Count: safeInt(tg.Count),
			Tasks: taskNames,
		}

		jobDetail.TaskGroups = append(jobDetail.TaskGroups, taskGroupSummary)
	}

	h.handler.logger.Debug("fetched job from API",
		"jobID", jobID,
		"name", jobDetail.Name,
		"status", jobDetail.Status,
		"taskGroups", len(jobDetail.TaskGroups),
	)

	return &jobDetail, nil
}

// safeString safely dereferences a string pointer, returning empty string if nil
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// safeInt safely dereferences an int pointer, returning 0 if nil
func safeInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// renderTabContent handles rendering individual tab content
func (h *JobsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, jobID, tabName string) {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	// Get job detail data
	jobDetail, err := h.getJobDetail(jobName, namespace, GetTokenFromRequest(r))
	if err != nil {
		h.handler.logger.Error("failed to fetch job for tab content", "error", err, "jobID", jobID, "tab", tabName)
		http.Error(w, "Failed to fetch job from Nomad API", http.StatusInternalServerError)
		return
	}

	if jobDetail == nil {
		http.NotFound(w, r)
		return
	}

	switch tabName {
	case "overview":
		err = RenderPartial(w, r, jobspartials.OverviewTab(*jobDetail))
	case "definition":
		jobDef := h.getJobDefinition(jobID)
		err = RenderPartial(w, r, jobspartials.DefinitionTab(jobDef))
	case "versions":
		versions := h.getJobVersions(r, jobID)
		err = RenderPartial(w, r, jobspartials.VersionsTab(versions, jobID))
	case "allocations":
		// Parse pagination parameters for allocations
		page := parsePage(r.URL.Query().Get("page"))
		perPage := parsePerPage(r.URL.Query().Get("per_page"))
		allocData := h.getJobAllocations(jobID, page, perPage)
		allocData.LastUpdated = time.Now()
		err = RenderPartial(w, r, jobspartials.AllocationsTabWithData(allocData))
	case "deployments":
		deployments := h.getJobDeployments(r, jobID)
		err = RenderPartial(w, r, jobspartials.DeploymentsTab(deployments, jobID))
	case "evaluations":
		evaluations := h.getJobEvaluations(r, jobID)
		err = RenderPartial(w, r, jobspartials.EvaluationsTab(evaluations, jobID))
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		h.handler.logger.Error("failed to render tab content", "tab", tabName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getJobVersions fetches job version history from the Nomad API.
// It returns a list of job versions sorted from newest to oldest.
func (h *JobsHandler) getJobVersions(r *http.Request, jobID string) []jobspartials.JobVersion {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty versions list")
		return []jobspartials.JobVersion{}
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build query options
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Note: For now we're not requesting diffs from the API (diffs=false)
	// because we compute diffs on-demand when the user clicks "Diff"
	h.handler.logger.Debug("fetching job versions from API",
		"jobID", jobName,
		"namespace", namespace,
		"hasToken", token != "",
	)

	versions, _, _, err := client.Jobs().Versions(jobName, false, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch job versions", "error", err, "jobID", jobName, "namespace", namespace)
		return []jobspartials.JobVersion{}
	}

	h.handler.logger.Debug("fetched job versions from API", "count", len(versions))

	if len(versions) == 0 {
		return []jobspartials.JobVersion{}
	}

	// Find the current/active version (highest version number)
	var maxVersion uint64
	for _, v := range versions {
		if v.Version != nil && *v.Version > maxVersion {
			maxVersion = *v.Version
		}
	}

	// Convert API response to our JobVersion type
	result := make([]jobspartials.JobVersion, 0, len(versions))
	for _, v := range versions {
		if v == nil {
			continue
		}

		// Extract version number (default to 0 if nil)
		version := uint64(0)
		if v.Version != nil {
			version = *v.Version
		}

		// Extract stable flag (default to false if nil)
		stable := false
		if v.Stable != nil {
			stable = *v.Stable
		}

		// Extract submit time (convert from nanoseconds to time.Time)
		submitTime := time.Time{}
		if v.SubmitTime != nil && *v.SubmitTime > 0 {
			submitTime = time.Unix(0, *v.SubmitTime)
		}

		// Determine if this is the active version
		active := (version == maxVersion)

		// Set diff URL only if this is not version 0 (no previous version to compare)
		diffURL := ""
		if version > 0 {
			diffURL = fmt.Sprintf("/ui/jobs/%s/versions/%d/diff/%d", jobID, version, version-1)
		}

		result = append(result, jobspartials.JobVersion{
			Version:    int(version),
			Stable:     stable,
			SubmitTime: submitTime,
			Active:     active,
			Diff:       diffURL,
		})
	}

	// Sort by version number descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})

	return result
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

	// Check permissions - need scale-job capability to scale task groups
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanScaleJob("default") {
		h.handler.logger.Warn("scale permission denied", "jobID", jobID, "taskGroup", tgName)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(renderScaleError("Permission denied: you do not have the 'scale-job' capability in the default namespace")))
		return
	}

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

// getJobAllocations fetches allocations for a specific job from the Nomad API.
func (h *JobsHandler) getJobAllocations(jobID string, page, perPage int) jobspartials.AllocationsTabData {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty allocations list")
		return jobspartials.AllocationsTabData{
			JobID:       jobID,
			Allocations: []jobspartials.JobAllocation{},
			Pagination: jobspartials.JobAllocationsPagination{
				Page:       page,
				PerPage:    perPage,
				Total:      0,
				TotalPages: 0,
			},
		}
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Fetch allocations from the API - include all allocations (not just running)
	h.handler.logger.Debug("fetching job allocations from API",
		"jobID", jobName,
		"namespace", namespace,
	)

	allocs, _, err := client.Jobs().Allocations(jobName, true, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch job allocations from Nomad API", "error", err)
		return jobspartials.AllocationsTabData{
			JobID:       jobID,
			Allocations: []jobspartials.JobAllocation{},
			Pagination: jobspartials.JobAllocationsPagination{
				Page:       page,
				PerPage:    perPage,
				Total:      0,
				TotalPages: 0,
			},
		}
	}

	h.handler.logger.Debug("fetched job allocations from API", "count", len(allocs))

	// Convert API allocations to our template type
	allAllocations := make([]jobspartials.JobAllocation, 0, len(allocs))
	for _, alloc := range allocs {
		if alloc == nil {
			continue
		}

		// Extract resource information from AllocatedResources if available
		cpuUsed := 0
		cpuMax := 0
		memUsed := 0
		memMax := 0

		if alloc.AllocatedResources != nil && len(alloc.AllocatedResources.Tasks) > 0 {
			// Sum up resources from all tasks in the allocation
			for _, taskRes := range alloc.AllocatedResources.Tasks {
				if taskRes != nil {
					cpuMax += int(taskRes.Cpu.CpuShares)
					memMax += int(taskRes.Memory.MemoryMB)
				}
			}

			// For current usage, we'd need to fetch stats separately
			// For now, use mock values as a percentage of max (50-80%)
			// In a future enhancement, we could fetch stats for running allocations
			if alloc.ClientStatus == "running" && cpuMax > 0 {
				cpuUsed = int(float64(cpuMax) * 0.65)
				memUsed = int(float64(memMax) * 0.70)
			}
		}

		// Fallback if no resource info available
		if cpuMax == 0 {
			cpuMax = 500
			memMax = 256
			if alloc.ClientStatus == "running" {
				cpuUsed = 325
				memUsed = 179
			}
		}

		// Convert create time from nanoseconds to time.Time
		created := time.Time{}
		if alloc.CreateTime > 0 {
			created = time.Unix(0, alloc.CreateTime)
		}

		jobAlloc := jobspartials.JobAllocation{
			ID:        alloc.ID,
			ShortID:   alloc.ID[:8], // First 8 chars for short ID
			TaskGroup: alloc.TaskGroup,
			NodeID:    alloc.NodeID,
			NodeName:  alloc.NodeName,
			Status:    alloc.ClientStatus,
			CPU:       cpuUsed,
			CPUMax:    cpuMax,
			Memory:    memUsed,
			MemoryMax: memMax,
			Created:   created,
		}

		allAllocations = append(allAllocations, jobAlloc)
	}

	// Sort by creation time (newest first)
	sort.Slice(allAllocations, func(i, j int) bool {
		return allAllocations[i].Created.After(allAllocations[j].Created)
	})

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

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		namespace = jobID[idx+1:]
	}

	// Check permissions - need submit-job capability to plan
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{
			Error: fmt.Sprintf("Permission denied: you do not have the 'submit-job' capability in the %s namespace", namespace),
		})
		return
	}

	// Log the request
	h.handler.logger.Info("plan job request", "jobID", jobID, "namespace", namespace)

	// Parse the request body
	var req PlanJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to decode plan request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{Error: "Invalid request body"})
		return
	}

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{Error: "Nomad client not available"})
		return
	}

	// Parse the job definition based on format
	var job *api.Job
	var parseErr error

	if req.Format == "json" {
		// Parse JSON job definition
		h.handler.logger.Debug("parsing JSON job definition", "length", len(req.Definition))
		parseErr = json.Unmarshal([]byte(req.Definition), &job)
		if parseErr != nil {
			h.handler.logger.Error("failed to parse JSON job definition", "error", parseErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(PlanJobResponse{
				Error: fmt.Sprintf("Invalid JSON job definition: %v", parseErr),
			})
			return
		}
	} else {
		// Parse HCL job definition using Nomad API
		h.handler.logger.Debug("parsing HCL job definition", "length", len(req.Definition))
		job, parseErr = client.Jobs().ParseHCL(req.Definition, true)
		if parseErr != nil {
			h.handler.logger.Error("failed to parse HCL job definition", "error", parseErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(PlanJobResponse{
				Error: fmt.Sprintf("Invalid HCL job definition: %v", parseErr),
			})
			return
		}
	}

	// Ensure job has required fields
	if job.ID == nil {
		h.handler.logger.Error("job definition missing ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{Error: "Job definition must include an ID"})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with namespace and token
	writeOpts := &api.WriteOptions{
		Namespace: namespace,
	}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Call the Plan API with diff=true
	h.handler.logger.Debug("calling Nomad Plan API", "jobID", *job.ID, "namespace", namespace)
	planResp, _, err := client.Jobs().Plan(job, true, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to plan job", "error", err, "jobID", *job.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{
			Error: fmt.Sprintf("Failed to plan job: %v", err),
		})
		return
	}

	// Build response with structured diff
	response := h.buildPlanResponse(planResp)

	h.handler.logger.Info("job plan completed",
		"jobID", *job.ID,
		"created", response.Created,
		"updated", response.Updated,
		"destroyed", response.Destroyed,
	)

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

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		namespace = jobID[idx+1:]
	}

	// Check permissions - need submit-job capability to run
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(RunJobResponse{
			Error: fmt.Sprintf("Permission denied: you do not have the 'submit-job' capability in the %s namespace", namespace),
		})
		return
	}

	// Log the request
	h.handler.logger.Info("run job request", "jobID", jobID, "namespace", namespace)

	// Parse the request body
	var req RunJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to decode run request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RunJobResponse{Error: "Invalid request body"})
		return
	}

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RunJobResponse{Error: "Nomad client not available"})
		return
	}

	// Parse the job definition based on format
	var job *api.Job
	var parseErr error

	if req.Format == "hcl" || req.Format == "" {
		// Parse HCL job definition
		h.handler.logger.Debug("parsing HCL job definition", "length", len(req.Definition))
		job, parseErr = client.Jobs().ParseHCL(req.Definition, true)
		if parseErr != nil {
			h.handler.logger.Error("failed to parse HCL job definition", "error", parseErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(RunJobResponse{
				Error: fmt.Sprintf("Failed to parse HCL: %s", parseErr.Error()),
			})
			return
		}
	} else if req.Format == "json" {
		// Parse JSON job definition
		h.handler.logger.Debug("parsing JSON job definition", "length", len(req.Definition))
		job = &api.Job{}
		if err := json.Unmarshal([]byte(req.Definition), job); err != nil {
			h.handler.logger.Error("failed to parse JSON job definition", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(RunJobResponse{
				Error: fmt.Sprintf("Failed to parse JSON: %s", err.Error()),
			})
			return
		}
	} else {
		h.handler.logger.Error("invalid job format", "format", req.Format)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RunJobResponse{
			Error: fmt.Sprintf("Invalid format: %s (must be 'hcl' or 'json')", req.Format),
		})
		return
	}

	// Get ACL token for the request
	token := GetTokenFromRequest(r)

	// Build write options with namespace and token
	writeOpts := &api.WriteOptions{
		Namespace: namespace,
	}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Submit the job to Nomad
	h.handler.logger.Info("submitting job to Nomad",
		"jobID", safeString(job.ID),
		"namespace", namespace,
		"hasToken", token != "",
	)

	registerResp, _, err := client.Jobs().Register(job, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to register job", "error", err, "jobID", safeString(job.ID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RunJobResponse{
			Error: fmt.Sprintf("Failed to register job: %s", err.Error()),
		})
		return
	}

	// Build response with eval ID and warnings
	var warnings []string
	if registerResp.Warnings != "" {
		warnings = strings.Split(registerResp.Warnings, "\n")
	}

	response := RunJobResponse{
		EvalID:   registerResp.EvalID,
		JobID:    safeString(job.ID),
		Warnings: warnings,
	}

	h.handler.logger.Info("job submitted successfully",
		"jobID", safeString(job.ID),
		"evalID", registerResp.EvalID,
		"warnings", len(warnings),
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

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		namespace = jobID[idx+1:]
	}

	// Check permissions - need submit-job capability to revert versions
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("revert permission denied", "jobID", jobID, "version", versionStr, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{
			Error: fmt.Sprintf("Permission denied: you do not have the 'submit-job' capability in the %s namespace", namespace),
		})
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.handler.logger.Error("invalid version number", "version", versionStr, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{Error: "Invalid version number"})
		return
	}

	// Extract job name from job ID (remove namespace suffix)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
	}

	// Log the revert operation
	h.handler.logger.Info("revert job request",
		"jobID", jobName,
		"namespace", namespace,
		"version", version,
	)

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{Error: "Nomad client not available"})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with namespace and token
	writeOpts := &api.WriteOptions{
		Namespace: namespace,
	}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Call the Revert API
	h.handler.logger.Debug("calling Nomad Revert API",
		"jobID", jobName,
		"namespace", namespace,
		"version", version,
	)

	registerResp, _, err := client.Jobs().Revert(jobName, uint64(version), nil, writeOpts, "", "")
	if err != nil {
		h.handler.logger.Error("failed to revert job", "error", err, "jobID", jobName, "version", version)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{
			Error: fmt.Sprintf("Failed to revert job: %v", err),
		})
		return
	}

	response := RevertToVersionResponse{
		EvalID:  registerResp.EvalID,
		JobID:   jobName,
		Version: version,
	}

	h.handler.logger.Info("job reverted successfully",
		"jobID", jobName,
		"version", version,
		"evalID", registerResp.EvalID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode revert response", "error", err)
	}
}

// getJobDeployments fetches deployments for a specific job from the Nomad API.
func (h *JobsHandler) getJobDeployments(r *http.Request, jobID string) []jobspartials.JobDeployment {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty deployments list")
		return []jobspartials.JobDeployment{}
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch deployments from the API - include all deployments (not just running)
	h.handler.logger.Debug("fetching job deployments from API",
		"jobID", jobName,
		"namespace", namespace,
	)

	deployments, _, err := client.Jobs().Deployments(jobName, true, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch job deployments from Nomad API", "error", err)
		return []jobspartials.JobDeployment{}
	}

	h.handler.logger.Debug("fetched job deployments from API", "count", len(deployments))

	// Convert API deployments to our template type
	result := make([]jobspartials.JobDeployment, 0, len(deployments))
	for _, d := range deployments {
		if d == nil {
			continue
		}

		// Create task group states map
		taskGroups := make(map[string]jobspartials.DeploymentTaskGroupState)
		for tgName, tgState := range d.TaskGroups {
			if tgState == nil {
				continue
			}

			taskGroups[tgName] = jobspartials.DeploymentTaskGroupState{
				Name:            tgName,
				DesiredCanaries: tgState.DesiredCanaries,
				DesiredTotal:    tgState.DesiredTotal,
				PlacedAllocs:    tgState.PlacedAllocs,
				HealthyAllocs:   tgState.HealthyAllocs,
				UnhealthyAllocs: tgState.UnhealthyAllocs,
				Promoted:        tgState.Promoted,
				AutoRevert:      tgState.AutoRevert,
			}
		}

		// Convert create index to time (Nomad stores as nanoseconds)
		// Note: CreateIndex is not a timestamp, but we can use ModifyIndex for sorting
		// For the actual creation time, we would need to track it separately
		// For now, we'll use the current time minus an offset based on index
		// In production, you might want to fetch this from the deployment metadata
		created := time.Now().Add(-time.Duration(len(result)) * time.Hour)

		deployment := jobspartials.JobDeployment{
			ID:                d.ID,
			ShortID:           d.ID[:8], // First 8 chars for short ID
			Status:            d.Status,
			StatusDescription: d.StatusDescription,
			JobVersion:        d.JobVersion,
			TaskGroups:        taskGroups,
			Created:           created,
		}

		result = append(result, deployment)
	}

	return result
}

// getJobEvaluations fetches evaluations for a specific job from the Nomad API.
func (h *JobsHandler) getJobEvaluations(r *http.Request, jobID string) []jobspartials.JobEvaluation {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty evaluations list")
		return []jobspartials.JobEvaluation{}
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch evaluations from the API
	h.handler.logger.Debug("fetching job evaluations from API",
		"jobID", jobName,
		"namespace", namespace,
	)

	evals, _, err := client.Jobs().Evaluations(jobName, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch job evaluations from Nomad API", "error", err)
		return []jobspartials.JobEvaluation{}
	}

	h.handler.logger.Debug("fetched job evaluations from API", "count", len(evals))

	// Convert API evaluations to our template type
	result := make([]jobspartials.JobEvaluation, 0, len(evals))
	for _, eval := range evals {
		if eval == nil {
			continue
		}

		// Convert create time from nanoseconds to time.Time
		created := time.Time{}
		if eval.CreateTime > 0 {
			created = time.Unix(0, eval.CreateTime)
		}

		// Check if there are placement failures
		hasPlacementFailures := len(eval.FailedTGAllocs) > 0

		// Extract placement failures details if any
		var placementFailures []jobspartials.PlacementFailure
		if hasPlacementFailures {
			for tgName, metrics := range eval.FailedTGAllocs {
				if metrics == nil {
					continue
				}

				// Build a summary of the failure reasons
				var reasons []string
				if metrics.NodesExhausted > 0 {
					reasons = append(reasons, fmt.Sprintf("%d nodes exhausted", metrics.NodesExhausted))
				}
				if metrics.NodesAvailable != nil {
					for dc, count := range metrics.NodesAvailable {
						if count == 0 {
							reasons = append(reasons, fmt.Sprintf("No nodes available in %s", dc))
						}
					}
				}
				if metrics.ClassFiltered != nil && len(metrics.ClassFiltered) > 0 {
					reasons = append(reasons, fmt.Sprintf("%d node classes filtered", len(metrics.ClassFiltered)))
				}
				if metrics.ConstraintFiltered != nil && len(metrics.ConstraintFiltered) > 0 {
					reasons = append(reasons, fmt.Sprintf("%d constraints filtered", len(metrics.ConstraintFiltered)))
				}

				reasonStr := strings.Join(reasons, ", ")
				if reasonStr == "" {
					reasonStr = "Placement failed"
				}

				placementFailures = append(placementFailures, jobspartials.PlacementFailure{
					TaskGroup: tgName,
					Reason:    reasonStr,
				})
			}
		}

		jobEval := jobspartials.JobEvaluation{
			ID:                   eval.ID,
			ShortID:              eval.ID[:8], // First 8 chars for short ID
			Status:               eval.Status,
			StatusDescription:    eval.StatusDescription,
			TriggeredBy:          eval.TriggeredBy,
			Created:              created,
			HasPlacementFailures: hasPlacementFailures,
			PlacementFailures:    placementFailures,
		}

		result = append(result, jobEval)
	}

	// Sort by creation time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Created.After(result[j].Created)
	})

	return result
}

// buildPlanResponse converts a Nomad JobPlanResponse to our PlanJobResponse format.
// It extracts the diff information and counts the number of creates/updates/destroys.
func (h *JobsHandler) buildPlanResponse(planResp *api.JobPlanResponse) PlanJobResponse {
	if planResp == nil {
		return PlanJobResponse{
			Error: "No plan response received",
		}
	}

	response := PlanJobResponse{
		Created:   0,
		Updated:   0,
		Destroyed: 0,
		Diff:      "",
		Warnings:  []string{},
	}

	// Add warnings if present
	if planResp.Warnings != "" {
		response.Warnings = append(response.Warnings, planResp.Warnings)
	}

	// Count creates/updates/destroys from annotations
	if planResp.Annotations != nil && planResp.Annotations.DesiredTGUpdates != nil {
		for _, update := range planResp.Annotations.DesiredTGUpdates {
			if update != nil {
				response.Created += int(update.Place)
				response.Updated += int(update.InPlaceUpdate)
				response.Destroyed += int(update.Stop + update.DestructiveUpdate)
			}
		}
	}

	// Build structured diff string from JobDiff
	if planResp.Diff != nil {
		response.Diff = h.formatJobDiff(planResp.Diff)
	} else {
		response.Diff = "No changes detected"
	}

	return response
}

// formatJobDiff converts a JobDiff structure to a human-readable string format.
func (h *JobsHandler) formatJobDiff(diff *api.JobDiff) string {
	if diff == nil {
		return "No changes"
	}

	var builder strings.Builder

	// Job-level header
	if diff.Type != "None" {
		builder.WriteString(h.diffTypeSymbol(diff.Type))
		builder.WriteString(" Job: \"")
		builder.WriteString(diff.ID)
		builder.WriteString("\"\n")

		// Job-level fields
		for _, field := range diff.Fields {
			h.formatFieldDiff(&builder, field, "  ")
		}

		// Job-level objects
		for _, obj := range diff.Objects {
			h.formatObjectDiff(&builder, obj, "  ")
		}
	}

	// Task groups
	for _, tg := range diff.TaskGroups {
		if tg == nil {
			continue
		}

		builder.WriteString(h.diffTypeSymbol(tg.Type))
		builder.WriteString(" Task Group: \"")
		builder.WriteString(tg.Name)
		builder.WriteString("\"")

		// Add update counts if available
		if len(tg.Updates) > 0 {
			builder.WriteString(" (")
			var parts []string
			for action, count := range tg.Updates {
				if count > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", count, action))
				}
			}
			builder.WriteString(strings.Join(parts, ", "))
			builder.WriteString(")")
		}
		builder.WriteString("\n")

		// Task group fields
		for _, field := range tg.Fields {
			h.formatFieldDiff(&builder, field, "  ")
		}

		// Task group objects
		for _, obj := range tg.Objects {
			h.formatObjectDiff(&builder, obj, "  ")
		}

		// Tasks
		for _, task := range tg.Tasks {
			if task == nil {
				continue
			}

			builder.WriteString("  ")
			builder.WriteString(h.diffTypeSymbol(task.Type))
			builder.WriteString(" Task: \"")
			builder.WriteString(task.Name)
			builder.WriteString("\"\n")

			// Task fields
			for _, field := range task.Fields {
				h.formatFieldDiff(&builder, field, "    ")
			}

			// Task objects
			for _, obj := range task.Objects {
				h.formatObjectDiff(&builder, obj, "    ")
			}
		}
	}

	return builder.String()
}

// formatFieldDiff formats a single field diff with proper indentation.
func (h *JobsHandler) formatFieldDiff(builder *strings.Builder, field *api.FieldDiff, indent string) {
	if field == nil {
		return
	}

	builder.WriteString(indent)
	builder.WriteString(h.diffTypeSymbol(field.Type))
	builder.WriteString(" ")
	builder.WriteString(field.Name)
	builder.WriteString(": ")

	switch field.Type {
	case "Added":
		builder.WriteString("\"")
		builder.WriteString(field.New)
		builder.WriteString("\"")
	case "Deleted":
		builder.WriteString("\"")
		builder.WriteString(field.Old)
		builder.WriteString("\"")
	case "Edited":
		builder.WriteString("\"")
		builder.WriteString(field.Old)
		builder.WriteString("\" => \"")
		builder.WriteString(field.New)
		builder.WriteString("\"")
	}

	builder.WriteString("\n")
}

// formatObjectDiff formats a nested object diff with proper indentation.
func (h *JobsHandler) formatObjectDiff(builder *strings.Builder, obj *api.ObjectDiff, indent string) {
	if obj == nil {
		return
	}

	builder.WriteString(indent)
	builder.WriteString(h.diffTypeSymbol(obj.Type))
	builder.WriteString(" ")
	builder.WriteString(obj.Name)
	builder.WriteString(" {\n")

	// Object fields
	for _, field := range obj.Fields {
		h.formatFieldDiff(builder, field, indent+"  ")
	}

	// Nested objects
	for _, nestedObj := range obj.Objects {
		h.formatObjectDiff(builder, nestedObj, indent+"  ")
	}

	builder.WriteString(indent)
	builder.WriteString("}\n")
}

// diffTypeSymbol returns the symbol prefix for a diff type.
func (h *JobsHandler) diffTypeSymbol(diffType string) string {
	switch diffType {
	case "Added":
		return "+"
	case "Deleted":
		return "-"
	case "Edited":
		return "+/-"
	default:
		return " "
	}
}
