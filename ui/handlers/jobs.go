package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/jobs"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
	"github.com/open-wander/wander/ui/templates/partials/taskgrouppartials"
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

	// Handle /ui/jobs/run (job submission page) - must come before /plan and /run POST handlers
	if path == "/ui/jobs/run" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.RunPage(w, r)
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
	namespace := GetNamespaceFromRequest(r)

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
		http.Error(w, "Failed to fetch jobs from Wander API", http.StatusInternalServerError)
		return
	}

	filters := parseJobListFilters(queryParams)
	filterOptions := buildJobFilterOptions(jobsList)
	jobsList = filterJobs(jobsList, filters)

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
	layoutProps := h.handler.BaseLayoutProps(r,
		"Jobs - Wander",
		"View and manage Wander jobs",
		"/ui/jobs",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Jobs", URL: ""},
		},
	)

	// Render the jobs list page
	err = Render(w, r,
		jobs.List(layoutProps, jobspartials.ListProps{
			Jobs: jobsList, Filters: filters, FilterOptions: filterOptions,
			SortColumn: sortColumn, SortDir: sortDir, Pagination: pagination,
		}),
		jobspartials.ListContent(jobspartials.ListProps{
			Jobs: jobsList, Filters: filters, FilterOptions: filterOptions,
			SortColumn: sortColumn, SortDir: sortDir, Pagination: pagination,
		}),
	)

	if err != nil {
		h.handler.logger.Error("failed to render jobs list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// RunPage handles the job submission page (GET /ui/jobs/run)
func (h *JobsHandler) RunPage(w http.ResponseWriter, r *http.Request) {
	// Parse namespace from query parameters
	namespace := GetNamespaceFromRequest(r)

	// Log the request
	h.handler.logger.Debug("job submission page request", "namespace", namespace, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need submit-job capability to access submission page
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to submit jobs",
			Capability: "submit-job",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'submit-job' capability",
				"Ensure you are using a valid ACL token with appropriate permissions",
			},
		})
		return
	}

	// Build layout props with app context
	layoutProps := h.handler.BaseLayoutProps(r,
		"Run Job - Wander",
		"Submit a new job to Wander",
		"/ui/jobs/run",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Jobs", URL: "/ui/jobs"},
			{Label: "Run Job", URL: ""},
		},
	)

	// Render the job submission page
	err := Render(w, r,
		jobs.Run(layoutProps, namespace),   // Full page with layout
		jobspartials.RunContent(namespace), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render job submission page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Detail handles the job detail page
func (h *JobsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	jobID = strings.TrimSuffix(jobID, "/")

	// A dispatched or periodic child job ID contains a slash. Treat only a
	// recognized final segment as a tab so those child jobs remain addressable.
	actualJobID, tabName := splitJobDetailPath(jobID)

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
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
	jobDetail, err := h.getJobOverview(r, jobName, namespace, GetTokenFromRequest(r))
	if err != nil {
		// Check if this is a cluster unavailability error first
		if h.handler.CheckClusterError(w, r, err) {
			return
		}
		h.handler.logger.Error("failed to fetch job detail", "error", err, "jobID", actualJobID)
		http.Error(w, "Failed to fetch job from Wander API", http.StatusInternalServerError)
		return
	}

	if jobDetail == nil {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
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
		jobs.Detail(layoutProps, *jobDetail),   // Full page with layout
		jobspartials.DetailContent(*jobDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render job detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func splitJobDetailPath(path string) (string, string) {
	for _, tab := range []string{"overview", "definition", "versions", "allocations", "clients", "services", "deployments", "evaluations"} {
		suffix := "/" + tab
		if strings.HasSuffix(path, suffix) {
			return strings.TrimSuffix(path, suffix), tab
		}
	}
	return path, ""
}

// getJobsList fetches jobs from the Nomad API with the specified parameters.
// Returns error if API is unavailable or request fails.
func (h *JobsHandler) getJobsList(r *http.Request, namespace, token string) ([]jobspartials.JobSummary, error) {
	client := h.handler.NomadClientForRequest(r)
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

	// Preserve the raw API filter parameter for callers that use the Wander UI
	// route as an HTMX endpoint. Visible list controls are applied locally so
	// their option sets can be derived from the same single API response.
	if filterExpr != "" {
		queryOpts.Filter = filterExpr
	}

	// Fetch jobs from the API
	h.handler.logger.Debug("fetching jobs from API",
		"namespace", namespace,
		"filter", queryOpts.Filter,
		"hasToken", token != "",
	)

	jobs, _, err := client.Jobs().ListOptions(&api.JobListOptions{
		Fields: &api.JobListFields{Meta: true},
	}, queryOpts)
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
			ID:          job.ID,
			Name:        job.Name,
			Namespace:   job.Namespace,
			Datacenters: append([]string(nil), job.Datacenters...),
			NodePool:    job.NodePool,
			Status:      job.Status,
			Type:        job.Type,
			Priority:    job.Priority,
			Submitted:   submitted,
			PackName:    job.Meta["pack.name"],
			PackVersion: job.Meta["pack.version"],
			Allocations: summarizeJobAllocations(job.JobSummary),
		})
	}

	return result, nil
}

func filterJobs(jobsList []jobspartials.JobSummary, filters jobspartials.JobListFilters) []jobspartials.JobSummary {
	filtered := make([]jobspartials.JobSummary, 0, len(jobsList))
	for _, job := range jobsList {
		search := strings.ToLower(filters.Search)
		if search != "" && !strings.Contains(strings.ToLower(job.ID), search) &&
			!strings.Contains(strings.ToLower(job.Name), search) {
			continue
		}
		if filters.Prefix != "" && !strings.HasPrefix(job.ID, filters.Prefix) {
			continue
		}
		if !matchesJobFilter(job.Status, filters.Statuses) || !matchesJobFilter(job.Type, filters.Types) {
			continue
		}
		if filters.Datacenter != "" && !containsString(job.Datacenters, filters.Datacenter) {
			continue
		}
		if filters.NodePool != "" && job.NodePool != filters.NodePool {
			continue
		}
		filtered = append(filtered, job)
	}
	return filtered
}

func parseJobListFilters(values url.Values) jobspartials.JobListFilters {
	return jobspartials.JobListFilters{
		Search:     strings.TrimSpace(values.Get("search")),
		Prefix:     strings.TrimSpace(values.Get("prefix")),
		Statuses:   normalizedQueryValues(values["status"]),
		Types:      normalizedQueryValues(values["type"]),
		Datacenter: strings.TrimSpace(values.Get("datacenter")),
		NodePool:   strings.TrimSpace(values.Get("node_pool")),
	}
}

func normalizedQueryValues(raw []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(raw))
	for _, value := range raw {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func matchesJobFilter(value string, selected []string) bool {
	return len(selected) == 0 || containsString(selected, value)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func summarizeJobAllocations(summary *api.JobSummary) jobspartials.AllocationSummary {
	result := jobspartials.AllocationSummary{}
	if summary == nil {
		return result
	}
	result.Available = true
	result.TaskGroups = len(summary.Summary)
	for _, group := range summary.Summary {
		result.Queued += group.Queued
		result.Starting += group.Starting
		result.Running += group.Running
		result.Complete += group.Complete
		result.Failed += group.Failed
		result.Lost += group.Lost
		result.Unknown += group.Unknown
	}
	return result
}

func buildJobFilterOptions(jobsList []jobspartials.JobSummary) jobspartials.JobFilterOptions {
	statuses := make(map[string]struct{})
	types := make(map[string]struct{})
	datacenters := make(map[string]struct{})
	nodePools := make(map[string]struct{})
	for _, job := range jobsList {
		addNonEmpty(statuses, job.Status)
		addNonEmpty(types, job.Type)
		addNonEmpty(nodePools, job.NodePool)
		for _, datacenter := range job.Datacenters {
			addNonEmpty(datacenters, datacenter)
		}
	}
	return jobspartials.JobFilterOptions{
		Statuses: sortedKeys(statuses), Types: sortedKeys(types),
		Datacenters: sortedKeys(datacenters), NodePools: sortedKeys(nodePools),
	}
}

func addNonEmpty(set map[string]struct{}, value string) {
	if value != "" {
		set[value] = struct{}{}
	}
}

func sortedKeys(set map[string]struct{}) []string {
	return slices.Sorted(maps.Keys(set))
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
		case "namespace":
			less = jobsList[i].Namespace < jobsList[j].Namespace
		case "datacenter":
			less = strings.Join(jobsList[i].Datacenters, ",") < strings.Join(jobsList[j].Datacenters, ",")
		case "node_pool":
			less = jobsList[i].NodePool < jobsList[j].NodePool
		case "task_groups":
			less = jobsList[i].Allocations.TaskGroups < jobsList[j].Allocations.TaskGroups
		case "allocations":
			less = jobsList[i].Allocations.Total() < jobsList[j].Allocations.Total()
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
func (h *JobsHandler) getJobDetail(r *http.Request, jobID, namespace, token string) (*jobspartials.JobDetail, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return nil, fmt.Errorf("no Wander client available")
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
		ID:             safeString(job.ID),
		Name:           safeString(job.Name),
		Status:         safeString(job.Status),
		Type:           safeString(job.Type),
		Priority:       safeInt(job.Priority),
		Datacenters:    job.Datacenters,
		Periodic:       job.IsPeriodic(),
		NodePool:       safeString(job.NodePool),
		AllAtOnce:      safeBool(job.AllAtOnce),
		ParentID:       safeString(job.ParentID),
		Dispatched:     job.Dispatched,
		TaskGroups:     make([]jobspartials.TaskGroupSummary, 0, len(job.TaskGroups)),
		Metadata:       make([]jobspartials.JobMetadata, 0, len(job.Meta)),
		Volumes:        make([]jobspartials.JobVolume, 0),
		LifecycleTasks: make([]jobspartials.JobLifecycleTask, 0),
	}
	if job.Periodic != nil {
		specs := append([]string(nil), job.Periodic.Specs...)
		if spec := safeString(job.Periodic.Spec); spec != "" {
			specs = append([]string{spec}, specs...)
		}
		jobDetail.PeriodicDetails = &jobspartials.PeriodicDetails{
			Enabled:         safeBool(job.Periodic.Enabled),
			Specs:           specs,
			TimeZone:        safeString(job.Periodic.TimeZone),
			ProhibitOverlap: safeBool(job.Periodic.ProhibitOverlap),
		}
	}
	if job.Multiregion != nil {
		jobDetail.Multiregion = make([]jobspartials.JobRegion, 0, len(job.Multiregion.Regions))
		for _, region := range job.Multiregion.Regions {
			if region == nil {
				continue
			}
			jobDetail.Multiregion = append(jobDetail.Multiregion, jobspartials.JobRegion{
				Name: region.Name, Count: safeInt(region.Count), Datacenters: region.Datacenters, NodePool: region.NodePool,
			})
		}
		sort.Slice(jobDetail.Multiregion, func(i, j int) bool { return jobDetail.Multiregion[i].Name < jobDetail.Multiregion[j].Name })
	}
	metadataKeys := make([]string, 0, len(job.Meta))
	for key := range job.Meta {
		metadataKeys = append(metadataKeys, key)
	}
	sort.Strings(metadataKeys)
	for _, key := range metadataKeys {
		jobDetail.Metadata = append(jobDetail.Metadata, jobspartials.JobMetadata{Key: key, Value: job.Meta[key]})
	}
	if job.IsParameterized() {
		jobDetail.Parameterized = &jobspartials.ParameterizedJob{
			Payload:      job.ParameterizedJob.Payload,
			MetaRequired: job.ParameterizedJob.MetaRequired,
			MetaOptional: job.ParameterizedJob.MetaOptional,
		}
		if jobDetail.Parameterized.Payload == "" {
			jobDetail.Parameterized.Payload = "optional"
		}
	}
	jobDetail.ChildJobs.Applicable = jobDetail.Periodic || jobDetail.Parameterized != nil

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
				if task.Lifecycle != nil {
					jobDetail.LifecycleTasks = append(jobDetail.LifecycleTasks, jobspartials.JobLifecycleTask{
						TaskGroup: safeString(tg.Name), Task: task.Name, Hook: task.Lifecycle.Hook, Sidecar: task.Lifecycle.Sidecar,
					})
				}
			}
		}
		volumeNames := make([]string, 0, len(tg.Volumes))
		for name := range tg.Volumes {
			volumeNames = append(volumeNames, name)
		}
		sort.Strings(volumeNames)
		for _, name := range volumeNames {
			volume := tg.Volumes[name]
			if volume == nil {
				continue
			}
			jobDetail.Volumes = append(jobDetail.Volumes, jobspartials.JobVolume{
				TaskGroup: safeString(tg.Name), Name: name, Type: volume.Type, Source: volume.Source,
				ReadOnly: volume.ReadOnly, PerAllocation: volume.PerAlloc,
			})
		}

		taskGroupSummary := jobspartials.TaskGroupSummary{
			Name:  safeString(tg.Name),
			Count: safeInt(tg.Count),
			Tasks: taskNames,
		}

		jobDetail.TaskGroups = append(jobDetail.TaskGroups, taskGroupSummary)
	}
	sort.Slice(jobDetail.LifecycleTasks, func(i, j int) bool {
		if jobDetail.LifecycleTasks[i].TaskGroup != jobDetail.LifecycleTasks[j].TaskGroup {
			return jobDetail.LifecycleTasks[i].TaskGroup < jobDetail.LifecycleTasks[j].TaskGroup
		}
		return jobDetail.LifecycleTasks[i].Task < jobDetail.LifecycleTasks[j].Task
	})

	h.handler.logger.Debug("fetched job from API",
		"jobID", jobID,
		"name", jobDetail.Name,
		"status", jobDetail.Status,
		"taskGroups", len(jobDetail.TaskGroups),
	)

	return &jobDetail, nil
}

// getJobOverview enriches core job detail with overview-only reads. Summary
// and allocation failures remain local to their sections so operators can
// still use the job header and other tabs while the cluster is degraded.
func (h *JobsHandler) getJobOverview(r *http.Request, jobID, namespace, token string) (*jobspartials.JobDetail, error) {
	jobDetail, err := h.getJobDetail(r, jobID, namespace, token)
	if err != nil || jobDetail == nil {
		return jobDetail, err
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return jobDetail, nil
	}
	queryOpts := &api.QueryOptions{Namespace: namespace, AuthToken: token}

	summary, _, summaryErr := client.Jobs().Summary(jobID, queryOpts)
	if summaryErr != nil {
		h.handler.logger.Warn("failed to fetch job allocation summary", "jobID", jobID, "namespace", namespace, "error", summaryErr)
	} else {
		jobDetail.Allocations = summarizeJobAllocations(summary)
		if jobDetail.ChildJobs.Applicable && summary != nil {
			jobDetail.ChildJobs.Available = true
			if summary.Children != nil {
				jobDetail.ChildJobs.Pending = summary.Children.Pending
				jobDetail.ChildJobs.Running = summary.Children.Running
				jobDetail.ChildJobs.Dead = summary.Children.Dead
			}
		}
	}

	evaluations, evaluationsErr := h.fetchJobEvaluations(r, jobID, namespace, token)
	if evaluationsErr != nil {
		h.handler.logger.Warn("failed to fetch job placement status", "jobID", jobID, "namespace", namespace, "error", evaluationsErr)
	} else {
		jobDetail.PlacementFailuresAvailable = true
		if len(evaluations) > 0 {
			jobDetail.PlacementFailures = evaluations[0].PlacementFailures
		}
	}

	allocations, _, allocationsErr := client.Jobs().Allocations(jobID, true, queryOpts)
	if allocationsErr != nil {
		h.handler.logger.Warn("failed to fetch recent job allocations", "jobID", jobID, "namespace", namespace, "error", allocationsErr)
		return jobDetail, nil
	}
	jobDetail.RecentAllocationsAvailable = true
	projected := projectJobAllocations(allocations)
	if len(projected) > 5 {
		projected = projected[:5]
	}
	jobDetail.RecentAllocations = projected
	return jobDetail, nil
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

func safeBool(value *bool) bool {
	return value != nil && *value
}

func checkedUint64ToInt(value uint64) (int, bool) {
	if value > uint64(^uint(0)>>1) {
		return 0, false
	}
	return int(value), true // #nosec G115 -- guarded by the platform int maximum above.
}

// renderTabContent handles rendering individual tab content
func (h *JobsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, jobID, tabName string) {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	// Only the overview needs summary and recent-allocation reads. Other tabs
	// retain the single core job read before fetching their own data.
	var jobDetail *jobspartials.JobDetail
	var err error
	if tabName == "overview" {
		jobDetail, err = h.getJobOverview(r, jobName, namespace, GetTokenFromRequest(r))
	} else {
		jobDetail, err = h.getJobDetail(r, jobName, namespace, GetTokenFromRequest(r))
	}
	if err != nil {
		h.handler.logger.Error("failed to fetch job for tab content", "error", err, "jobID", jobID, "tab", tabName)
		http.Error(w, "Failed to fetch job from Wander API", http.StatusInternalServerError)
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
		jobDef, defErr := h.getLiveJobDefinition(r, jobID)
		if defErr != nil {
			h.handler.logger.Error("failed to fetch job definition", "error", defErr, "jobID", jobID)
			http.Error(w, "Failed to fetch job definition", http.StatusBadGateway)
			return
		}
		err = RenderPartial(w, r, jobspartials.DefinitionTab(jobDef))
	case "versions":
		versions := h.getJobVersions(r, jobID)
		err = RenderPartial(w, r, jobspartials.VersionsTab(versions, jobID))
	case "allocations":
		// Parse pagination parameters for allocations
		page := parsePage(r.URL.Query().Get("page"))
		perPage := parsePerPage(r.URL.Query().Get("per_page"))
		allocData := h.getJobAllocations(r, jobID, page, perPage)
		allocData.LastUpdated = time.Now()
		err = RenderPartial(w, r, jobspartials.AllocationsTabWithData(allocData))
	case "clients":
		clients, clientsErr := h.getJobClients(r, jobID)
		if clientsErr != nil {
			h.renderJobTabError(w, r, clientsErr, "clients", jobID)
			return
		}
		err = RenderPartial(w, r, jobspartials.ClientsTab(clients))
	case "services":
		services, servicesErr := h.getJobServices(r, jobID)
		if servicesErr != nil {
			h.renderJobTabError(w, r, servicesErr, "services", jobID)
			return
		}
		err = RenderPartial(w, r, jobspartials.ServicesTab(services))
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

func (h *JobsHandler) getJobServices(r *http.Request, jobID string) ([]*api.ServiceRegistration, error) {
	jobName, namespace := splitJobReference(jobID, GetNamespaceFromRequest(r))
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, fmt.Errorf("Wander API client is unavailable")
	}
	services, _, err := client.Jobs().Services(jobName, serviceQueryOptions(r, namespace))
	if err != nil {
		return nil, fmt.Errorf("fetch job services: %w", err)
	}
	return services, nil
}

func (h *JobsHandler) getJobClients(r *http.Request, jobID string) ([]jobspartials.JobClient, error) {
	jobName, namespace := splitJobReference(jobID, GetNamespaceFromRequest(r))
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, fmt.Errorf("Wander API client is unavailable")
	}
	allocations, _, err := client.Jobs().Allocations(jobName, true, serviceQueryOptions(r, namespace))
	if err != nil {
		return nil, fmt.Errorf("fetch job allocations: %w", err)
	}

	byNode := make(map[string]*jobspartials.JobClient)
	for _, allocation := range allocations {
		if allocation == nil || allocation.NodeID == "" {
			continue
		}
		clientSummary, exists := byNode[allocation.NodeID]
		if !exists {
			clientSummary = &jobspartials.JobClient{
				ID:             allocation.NodeID,
				Name:           allocation.NodeName,
				LatestStatus:   allocation.ClientStatus,
				LatestModified: allocation.ModifyIndex,
			}
			byNode[allocation.NodeID] = clientSummary
		}
		clientSummary.Allocations++
		if allocation.ClientStatus != api.AllocClientStatusComplete &&
			allocation.ClientStatus != api.AllocClientStatusFailed &&
			allocation.ClientStatus != api.AllocClientStatusLost {
			clientSummary.ActiveAllocations++
		}
		if allocation.ModifyIndex >= clientSummary.LatestModified {
			clientSummary.LatestModified = allocation.ModifyIndex
			clientSummary.LatestStatus = allocation.ClientStatus
			if allocation.NodeName != "" {
				clientSummary.Name = allocation.NodeName
			}
		}
	}

	result := make([]jobspartials.JobClient, 0, len(byNode))
	for _, clientSummary := range byNode {
		result = append(result, *clientSummary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ID < result[j].ID
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (h *JobsHandler) renderJobTabError(w http.ResponseWriter, r *http.Request, err error, tab, jobID string) {
	if h.handler.CheckClusterError(w, r, err) {
		return
	}
	h.handler.logger.Error("failed to fetch job tab", "error", err, "tab", tab, "jobID", jobID)
	http.Error(w, "Failed to fetch job "+tab+" from Wander API", http.StatusBadGateway)
}

func (h *JobsHandler) getLiveJobDefinition(r *http.Request, jobID string) (jobspartials.JobDefinition, error) {
	jobName, _ := splitJobReference(jobID, GetNamespaceFromRequest(r))
	client, job, queryOpts, err := h.fetchLiveJob(r, jobID)
	if err != nil {
		return jobspartials.JobDefinition{}, err
	}

	version := uint64(0)
	if job.Version != nil {
		version = *job.Version
	}
	definition := jobspartials.JobDefinition{
		JobID:   jobID,
		Format:  "json",
		Version: version,
	}

	jobJSON, marshalErr := json.MarshalIndent(job, "", "  ")
	if marshalErr != nil {
		return jobspartials.JobDefinition{}, fmt.Errorf("encode job: %w", marshalErr)
	}
	definition.JSONContent = string(jobJSON)
	definition.Content = definition.JSONContent

	submissionVersion, supported := checkedUint64ToInt(version)
	if !supported {
		definition.ReadOnly = true
		return definition, nil
	}
	submission, _, err := client.Jobs().Submission(jobName, submissionVersion, queryOpts)
	if err == nil && submission != nil && submission.Source != "" {
		definition.Format = submission.Format
		if strings.HasPrefix(definition.Format, "hcl") {
			definition.Format = "hcl"
			definition.HCLContent = submission.Source
			definition.Content = definition.HCLContent
		} else if definition.Format == "json" {
			definition.JSONContent = submission.Source
			definition.Content = definition.JSONContent
		}
		return definition, nil
	}

	definition.ReadOnly = true
	return definition, nil
}

func (h *JobsHandler) fetchLiveJob(r *http.Request, jobID string) (*api.Client, *api.Job, *api.QueryOptions, error) {
	jobName, namespace := splitJobReference(jobID, GetNamespaceFromRequest(r))
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, nil, nil, fmt.Errorf("Wander API client is unavailable")
	}

	queryOpts := &api.QueryOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
	job, _, err := client.Jobs().Info(jobName, queryOpts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch job: %w", err)
	}
	return client, job, queryOpts, nil
}

// getJobVersions fetches job version history from the Nomad API.
// It returns a list of job versions sorted from newest to oldest.
func (h *JobsHandler) getJobVersions(r *http.Request, jobID string) []jobspartials.JobVersion {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClientForRequest(r)
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
			Version:    version,
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

	newVersion, err := strconv.ParseUint(newVersionStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid new version number", http.StatusBadRequest)
		return
	}

	oldVersion, err := strconv.ParseUint(oldVersionStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid old version number", http.StatusBadRequest)
		return
	}

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	h.handler.logger.Debug("version diff request",
		"jobID", jobName,
		"namespace", namespace,
		"newVersion", newVersion,
		"oldVersion", oldVersion,
		"htmx", IsHTMXFromRequest(r),
	)

	// Get job definitions from Nomad API
	oldDef, err := h.getJobVersionDefinition(r, jobName, namespace, oldVersion)
	if err != nil {
		h.handler.logger.Error("failed to fetch old version", "error", err, "version", oldVersion)
		http.Error(w, "Failed to fetch old version from Wander API", http.StatusInternalServerError)
		return
	}

	newDef, err := h.getJobVersionDefinition(r, jobName, namespace, newVersion)
	if err != nil {
		h.handler.logger.Error("failed to fetch new version", "error", err, "version", newVersion)
		http.Error(w, "Failed to fetch new version from Wander API", http.StatusInternalServerError)
		return
	}

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

// getJobVersionDefinition fetches a specific job version from the Nomad API
// and returns it as an HCL string for diff comparison.
func (h *JobsHandler) getJobVersionDefinition(r *http.Request, jobName, namespace string, version uint64) (string, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return "", fmt.Errorf("no Wander client available")
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build query options
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch all versions with diffs=false (we don't need the built-in diffs)
	h.handler.logger.Debug("fetching job versions for diff",
		"jobID", jobName,
		"namespace", namespace,
		"targetVersion", version,
	)

	versions, _, _, err := client.Jobs().Versions(jobName, false, queryOpts)
	if err != nil {
		return "", fmt.Errorf("failed to fetch job versions: %w", err)
	}

	// Find the specific version we need
	var targetJob *api.Job
	for _, v := range versions {
		if v == nil || v.Version == nil {
			continue
		}
		if *v.Version == version {
			targetJob = v
			break
		}
	}

	if targetJob == nil {
		return "", fmt.Errorf("version %d not found", version)
	}

	// Convert the Job struct to JSON, then to HCL using Nomad's parser
	// For now, we'll convert to JSON as a string representation
	// In a future enhancement, we could use the Nomad HCL formatter
	jobJSON, err := json.MarshalIndent(targetJob, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal job to JSON: %w", err)
	}

	return string(jobJSON), nil
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

	jobID, namespace := splitJobReference(parts[0], GetNamespaceFromRequest(r))
	tgName := parts[1]

	// Check permissions - need scale-job capability to scale task groups
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanScaleJob(namespace) {
		h.handler.logger.Warn("scale permission denied", "jobID", jobID, "taskGroup", tgName)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(renderScaleError("Permission denied: you do not have the 'scale-job' capability in this namespace")))
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

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(renderScaleError("Wander API client is unavailable")))
		return
	}

	writeOpts := &api.WriteOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
	if _, _, err := client.Jobs().Scale(jobID, tgName, &count, "Scaling via Wander UI", false, nil, writeOpts); err != nil {
		h.handler.logger.Error("failed to scale task group", "jobID", jobID, "taskGroup", tgName, "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(renderScaleError("The cluster rejected the scale request")))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(renderScaleSuccess(tgName, count))) // #nosec G705 -- renderScaleSuccess HTML-escapes its only string input.
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
					Task group <span class="font-semibold">` + html.EscapeString(taskGroupName) + `</span> scaled to ` + strconv.Itoa(newCount) + ` instance(s)
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
					Failed to scale: ` + html.EscapeString(message) + `
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

	taskGroup, err := h.getLiveTaskGroupDetail(r, jobID, tgName)
	if err != nil {
		h.handler.logger.Error("failed to fetch task group", "error", err, "jobID", jobID, "taskGroup", tgName)
		http.Error(w, "Failed to fetch task group", http.StatusBadGateway)
		return
	}

	// Render full page or partial based on HTMX request
	err = Render(w, r,
		jobs.TaskGroup(jobID, taskGroup),                  // Full page with layout
		taskgrouppartials.DetailContent(jobID, taskGroup), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render task group detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *JobsHandler) getLiveTaskGroupDetail(r *http.Request, jobID, taskGroupName string) (taskgrouppartials.TaskGroupDetail, error) {
	_, job, _, err := h.fetchLiveJob(r, jobID)
	if err != nil {
		return taskgrouppartials.TaskGroupDetail{}, err
	}

	for _, group := range job.TaskGroups {
		if group == nil || safeString(group.Name) != taskGroupName {
			continue
		}

		detail := taskgrouppartials.TaskGroupDetail{
			Name:  taskGroupName,
			Count: safeInt(group.Count),
			Tasks: make([]taskgrouppartials.TaskDetail, 0, len(group.Tasks)),
		}
		for _, task := range group.Tasks {
			if task == nil {
				continue
			}
			resources := taskgrouppartials.ResourceConfig{}
			if task.Resources != nil {
				resources.CPU = safeInt(task.Resources.CPU)
				resources.Memory = safeInt(task.Resources.MemoryMB)
				resources.DiskMB = safeInt(task.Resources.DiskMB)
				for _, network := range task.Resources.Networks {
					if network != nil {
						resources.Networks = append(resources.Networks, taskgrouppartials.NetworkResource{MBits: network.Megabits()})
					}
				}
			}
			detail.Tasks = append(detail.Tasks, taskgrouppartials.TaskDetail{
				Name:      task.Name,
				Driver:    task.Driver,
				Resources: resources,
			})
		}
		if group.Scaling != nil {
			detail.Scaling = &taskgrouppartials.ScalingConfig{
				Enabled: group.Scaling.Enabled != nil && *group.Scaling.Enabled,
				Min:     intValue64(group.Scaling.Min),
				Max:     intValue64(group.Scaling.Max),
			}
		}
		return detail, nil
	}

	return taskgrouppartials.TaskGroupDetail{}, fmt.Errorf("task group %q not found", taskGroupName)
}

func intValue64(value *int64) int {
	if value == nil {
		return 0
	}
	return int(*value)
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
func (h *JobsHandler) getJobAllocations(r *http.Request, jobID string, page, perPage int) jobspartials.AllocationsTabData {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClientForRequest(r)
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

	allAllocations := projectJobAllocations(allocs)

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

func projectJobAllocations(allocs []*api.AllocationListStub) []jobspartials.JobAllocation {
	result := make([]jobspartials.JobAllocation, 0, len(allocs))
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

		}

		// Convert create time from nanoseconds to time.Time
		created := time.Time{}
		if alloc.CreateTime > 0 {
			created = time.Unix(0, alloc.CreateTime)
		}

		result = append(result, jobspartials.JobAllocation{
			ID:        alloc.ID,
			ShortID:   shortIdentifier(alloc.ID),
			TaskGroup: alloc.TaskGroup,
			NodeID:    alloc.NodeID,
			NodeName:  alloc.NodeName,
			Status:    alloc.ClientStatus,
			CPU:       cpuUsed,
			CPUMax:    cpuMax,
			Memory:    memUsed,
			MemoryMax: memMax,
			Created:   created,
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Created.After(result[j].Created)
	})
	return result
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
	allocData := h.getJobAllocations(r, jobID, page, perPage)
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
	Created   uint64   `json:"created"`
	Updated   uint64   `json:"updated"`
	Destroyed uint64   `json:"destroyed"`
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
	EvalID   string   `json:"evalId"`
	JobID    string   `json:"jobId"`
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// PlanJob handles POST requests to plan a job (dry run).
// Route: POST /ui/jobs/{id}/plan
func (h *JobsHandler) PlanJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /ui/jobs/{id}/plan
	path := strings.TrimPrefix(r.URL.Path, "/ui/jobs/")
	path = strings.TrimSuffix(path, "/plan")
	jobID := path

	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PlanJobResponse{Error: "Wander client not available"})
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
	namespace := GetNamespaceFromRequest(r)
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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RunJobResponse{Error: "Wander client not available"})
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

// RevertToVersionResponse represents the response from the revert endpoint
type RevertToVersionResponse struct {
	EvalID  string `json:"evalId"`
	JobID   string `json:"jobId"`
	Version uint64 `json:"version"`
	Error   string `json:"error,omitempty"`
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
	namespace := GetNamespaceFromRequest(r)
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

	version, err := strconv.ParseUint(versionStr, 10, 64)
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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RevertToVersionResponse{Error: "Wander client not available"})
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

	registerResp, _, err := client.Jobs().Revert(jobName, version, nil, writeOpts, "", "")
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
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	client := h.handler.NomadClientForRequest(r)
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

		deployment := jobspartials.JobDeployment{
			ID:                d.ID,
			ShortID:           shortIdentifier(d.ID),
			Status:            d.Status,
			StatusDescription: d.StatusDescription,
			JobVersion:        d.JobVersion,
			TaskGroups:        taskGroups,
			Created:           time.Time{},
		}

		result = append(result, deployment)
	}

	return result
}

// getJobEvaluations fetches evaluations for a specific job from the Nomad API.
func (h *JobsHandler) getJobEvaluations(r *http.Request, jobID string) []jobspartials.JobEvaluation {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}
	evaluations, err := h.fetchJobEvaluations(r, jobName, namespace, GetTokenFromRequest(r))
	if err != nil {
		h.handler.logger.Error("failed to fetch job evaluations from Nomad API", "error", err)
		return []jobspartials.JobEvaluation{}
	}
	return evaluations
}

func (h *JobsHandler) fetchJobEvaluations(r *http.Request, jobName, namespace, token string) ([]jobspartials.JobEvaluation, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, fmt.Errorf("no Nomad client available")
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
		AuthToken: token,
	}

	// Fetch evaluations from the API
	h.handler.logger.Debug("fetching job evaluations from API",
		"jobID", jobName,
		"namespace", namespace,
	)

	evals, _, err := client.Jobs().Evaluations(jobName, queryOpts)
	if err != nil {
		return nil, err
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
			taskGroups := make([]string, 0, len(eval.FailedTGAllocs))
			for taskGroup := range eval.FailedTGAllocs {
				taskGroups = append(taskGroups, taskGroup)
			}
			sort.Strings(taskGroups)
			for _, tgName := range taskGroups {
				metrics := eval.FailedTGAllocs[tgName]
				if metrics == nil {
					continue
				}

				// Build a summary of the failure reasons
				var reasons []string
				if metrics.NodesExhausted > 0 {
					reasons = append(reasons, fmt.Sprintf("%d nodes exhausted", metrics.NodesExhausted))
				}
				if metrics.NodesAvailable != nil {
					datacenters := make([]string, 0, len(metrics.NodesAvailable))
					for datacenter := range metrics.NodesAvailable {
						datacenters = append(datacenters, datacenter)
					}
					sort.Strings(datacenters)
					for _, dc := range datacenters {
						count := metrics.NodesAvailable[dc]
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
			ShortID:              shortIdentifier(eval.ID),
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

	return result, nil
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
				response.Created += update.Place
				response.Updated += update.InPlaceUpdate
				response.Destroyed += update.Stop + update.DestructiveUpdate
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
