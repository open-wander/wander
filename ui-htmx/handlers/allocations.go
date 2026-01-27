package handlers

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
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

// ServeHTTP handles requests to the allocations pages.
// It routes to either the list page or detail page based on the URL path.
func (h *AllocationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/allocations exactly (list page)
	if path == "/ui/allocations" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/allocations/{id}/restart (restart action)
	if strings.HasSuffix(path, "/restart") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Restart(w, r)
		return
	}

	// Handle /ui/allocations/{id}/stop (stop action)
	if strings.HasSuffix(path, "/stop") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Stop(w, r)
		return
	}

	// Handle /ui/allocations/{id} (detail page)
	if strings.HasPrefix(path, "/ui/allocations/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the allocations list page
func (h *AllocationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("allocations list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters first to get namespace
	query := r.URL.Query()
	namespace := strings.TrimSpace(query.Get("namespace"))
	if namespace == "" {
		namespace = "default"
	}

	// Check permissions - allocations inherit job permissions
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListJobs(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to list allocations",
			Capability: "list-jobs",
			Namespace:  namespace,
			Suggestions: []string{
				"Allocations inherit job permissions. You need the 'list-jobs' capability",
				"Contact your administrator to request appropriate permissions",
			},
		})
		return
	}
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

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all allocations with server-side filtering
	allAllocations := h.getAllocationsList(namespace, token, statusFilter, jobFilter, nodeFilter)

	// Extract filter options from all allocations (before filtering)
	filterOptions := h.extractFilterOptions(allAllocations)

	// Apply client-side search filter (status/job/node already filtered server-side)
	filteredAllocations := h.filterAllocations(allAllocations, "", "", "", searchQuery)

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

	// Normalize search query for ID prefix matching detection
	searchNormalized := strings.ToLower(strings.TrimSpace(searchQuery))

	// Convert to partial type
	allocationsList := make([]allocationspartials.AllocationSummary, len(paginatedAllocations))
	for i, a := range paginatedAllocations {
		idPrefixMatch, matchedPrefix := h.isIDPrefixMatch(a.ID, searchNormalized)
		allocationsList[i] = allocationspartials.AllocationSummary{
			ID:            a.ID,
			JobID:         a.JobID,
			TaskGroup:     a.TaskGroup,
			NodeID:        a.NodeID,
			NodeName:      a.NodeName,
			Status:        a.Status,
			Created:       a.Created,
			IDPrefixMatch: idPrefixMatch,
			MatchedPrefix: matchedPrefix,
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

	// Build layout props with app context
	layoutProps := h.handler.BaseLayoutProps(
		"Allocations - Wander",
		"View and manage Nomad allocations",
		"/ui/allocations",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Allocations", URL: ""},
		},
	)

	// Render the allocations list page
	err := Render(w, r,
		allocations.List(layoutProps, props),   // Full page with layout
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

// filterAllocations applies filters to the allocations list.
// When searching, ID prefix matches are prioritized over other matches.
func (h *AllocationsHandler) filterAllocations(allocs []AllocationSummary, status, job, node, search string) []AllocationSummary {
	// Apply non-search filters first
	preFiltered := make([]AllocationSummary, 0, len(allocs))

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

		preFiltered = append(preFiltered, a)
	}

	// If no search query, return pre-filtered results
	if search == "" {
		return preFiltered
	}

	// Apply search filter with ID prefix prioritization
	searchLower := strings.ToLower(strings.TrimSpace(search))

	// Separate into ID prefix matches (exact start) and other matches
	idPrefixMatches := make([]AllocationSummary, 0)
	otherMatches := make([]AllocationSummary, 0)

	for _, a := range preFiltered {
		idLower := strings.ToLower(a.ID)

		// Check if search matches ID prefix (supports short ID search)
		// This matches both full UUID prefix and short ID (first 8 chars)
		if strings.HasPrefix(idLower, searchLower) {
			idPrefixMatches = append(idPrefixMatches, a)
			continue
		}

		// Check if search matches the short ID prefix (first 8 chars of UUID)
		shortID := idLower
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		if strings.HasPrefix(shortID, searchLower) {
			idPrefixMatches = append(idPrefixMatches, a)
			continue
		}

		// Check other fields for matches
		if strings.Contains(idLower, searchLower) ||
			strings.Contains(strings.ToLower(a.JobID), searchLower) ||
			strings.Contains(strings.ToLower(a.TaskGroup), searchLower) ||
			strings.Contains(strings.ToLower(a.NodeName), searchLower) ||
			strings.Contains(strings.ToLower(a.NodeID), searchLower) {
			otherMatches = append(otherMatches, a)
		}
	}

	// Combine results with ID prefix matches first
	result := make([]AllocationSummary, 0, len(idPrefixMatches)+len(otherMatches))
	result = append(result, idPrefixMatches...)
	result = append(result, otherMatches...)

	return result
}

// isIDPrefixMatch checks if the search query matches the allocation ID as a prefix.
// Returns true and the matched prefix if it's an ID prefix match.
func (h *AllocationsHandler) isIDPrefixMatch(allocID, search string) (bool, string) {
	if search == "" {
		return false, ""
	}

	idLower := strings.ToLower(allocID)

	// Check if search matches full ID prefix
	if strings.HasPrefix(idLower, search) {
		return true, search
	}

	// Check if search matches short ID prefix (first 8 chars)
	shortID := idLower
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if strings.HasPrefix(shortID, search) {
		return true, search
	}

	return false, ""
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

// getAllocationsList fetches allocations from the Nomad API with server-side filtering.
// Falls back to empty list if API is unavailable.
func (h *AllocationsHandler) getAllocationsList(namespace, token, statusFilter, jobFilter, nodeFilter string) []AllocationSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty allocation list")
		return []AllocationSummary{}
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Build Nomad filter expression for server-side filtering
	var filters []string
	if statusFilter != "" {
		filters = append(filters, fmt.Sprintf("ClientStatus == %q", statusFilter))
	}
	if jobFilter != "" {
		filters = append(filters, fmt.Sprintf("JobID == %q", jobFilter))
	}
	if nodeFilter != "" {
		// NodeFilter can match by NodeID or NodeName
		filters = append(filters, fmt.Sprintf("(NodeID == %q or NodeName == %q)", nodeFilter, nodeFilter))
	}
	if len(filters) > 0 {
		queryOpts.Filter = strings.Join(filters, " and ")
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching allocations from API",
		"namespace", namespace,
		"filter", queryOpts.Filter,
		"hasToken", token != "",
	)

	// Fetch allocations from the API
	allocs, _, err := client.Allocations().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocations from Nomad API", "error", err)
		return []AllocationSummary{}
	}

	h.handler.logger.Debug("fetched allocations from API", "count", len(allocs))

	// Convert API response to our summary type
	result := make([]AllocationSummary, 0, len(allocs))
	for _, alloc := range allocs {
		created := time.Time{}
		if alloc.CreateTime > 0 {
			created = time.Unix(0, alloc.CreateTime)
		}

		result = append(result, AllocationSummary{
			ID:        alloc.ID,
			JobID:     alloc.JobID,
			TaskGroup: alloc.TaskGroup,
			NodeID:    alloc.NodeID,
			NodeName:  alloc.NodeName,
			Status:    alloc.ClientStatus,
			Created:   created,
		})
	}

	return result
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
	subAction := ""
	if len(parts) > 1 {
		tabName = parts[1]
	}
	if len(parts) > 2 {
		subAction = parts[2]
	}

	// Log the request
	h.handler.logger.Debug("allocation detail request", "allocID", actualAllocID, "tab", tabName, "subAction", subAction, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read-job capability to view allocation details
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadJob("default") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view this allocation",
			Capability: "read-job",
			Namespace:  "default",
			Suggestions: []string{
				"Allocations inherit job permissions. You need the 'read-job' capability",
				"Contact your administrator to request appropriate permissions",
			},
		})
		return
	}

	// Handle files sub-routes (list, preview, download)
	if tabName == "files" && subAction != "" {
		switch subAction {
		case "list":
			h.FilesListHandler(w, r, actualAllocID)
			return
		case "preview":
			h.FilesPreviewHandler(w, r, actualAllocID)
			return
		case "download":
			h.FilesDownloadHandler(w, r, actualAllocID)
			return
		}
	}

	// If this is a tab content request, return just the tab partial
	if tabName != "" && IsHTMXFromRequest(r) {
		h.renderTabContent(w, r, actualAllocID, tabName)
		return
	}

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	// Fetch allocation detail data from API
	allocDetail := h.getAllocationDetail(actualAllocID, token)

	// Build layout props - use short ID for display
	shortID := actualAllocID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	layoutProps := h.handler.BaseLayoutProps(
		"Allocation "+shortID+" - Wander",
		"Allocation detail for "+actualAllocID,
		"/ui/allocations/"+actualAllocID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Allocations", URL: "/ui/allocations"},
			{Label: shortID, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		allocations.Detail(layoutProps, allocDetail),                // Full page with layout
		allocationspartials.DetailContent(allocDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render allocation detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getAllocationDetail fetches allocation details from the Nomad API.
// Returns allocation detail structure for template rendering.
func (h *AllocationsHandler) getAllocationDetail(allocID, token string) allocationspartials.AllocationDetail {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty allocation detail")
		return allocationspartials.AllocationDetail{ID: allocID}
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocation from the API
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation from Nomad API", "allocID", allocID, "error", err)
		return allocationspartials.AllocationDetail{ID: allocID}
	}

	h.handler.logger.Debug("fetched allocation from API", "allocID", allocID, "status", alloc.ClientStatus)

	// Convert timestamps
	created := time.Time{}
	if alloc.CreateTime > 0 {
		created = time.Unix(0, alloc.CreateTime)
	}
	modified := time.Time{}
	if alloc.ModifyTime > 0 {
		modified = time.Unix(0, alloc.ModifyTime)
	}

	// Convert task states to TaskStatus
	tasks := make([]allocationspartials.TaskStatus, 0, len(alloc.TaskStates))
	for taskName, taskState := range alloc.TaskStates {
		// Convert task events
		events := make([]allocationspartials.TaskEvent, 0, len(taskState.Events))
		for _, event := range taskState.Events {
			eventTime := time.Time{}
			if event.Time > 0 {
				eventTime = time.Unix(0, event.Time)
			}

			// Use DisplayMessage if available, otherwise construct from other fields
			displayMsg := event.DisplayMessage
			if displayMsg == "" && event.Message != "" {
				displayMsg = event.Message
			}

			events = append(events, allocationspartials.TaskEvent{
				Type:       event.Type,
				Time:       eventTime,
				DisplayMsg: displayMsg,
				FailsTask:  event.FailsTask,
			})
		}

		// Get task resources from allocation
		taskResources := allocationspartials.TaskResources{}
		if alloc.TaskResources != nil {
			if res, ok := alloc.TaskResources[taskName]; ok && res != nil {
				if res.CPU != nil {
					taskResources.CPU = *res.CPU
				}
				if res.MemoryMB != nil {
					taskResources.Memory = *res.MemoryMB
				}
			}
		}

		// Alternatively, check AllocatedResources for more detailed info
		if alloc.AllocatedResources != nil && alloc.AllocatedResources.Tasks != nil {
			if taskAlloc, ok := alloc.AllocatedResources.Tasks[taskName]; ok && taskAlloc != nil {
				if taskAlloc.Cpu.CpuShares > 0 {
					taskResources.CPU = int(taskAlloc.Cpu.CpuShares)
				}
				if taskAlloc.Memory.MemoryMB > 0 {
					taskResources.Memory = int(taskAlloc.Memory.MemoryMB)
				}
			}
		}

		// Note: CPU and memory usage percentages are not available from Info() call
		// These would require metrics/stats endpoint, so we leave them at 0 for now

		tasks = append(tasks, allocationspartials.TaskStatus{
			Name:       taskName,
			State:      taskState.State,
			Failed:     taskState.Failed,
			StartedAt:  taskState.StartedAt,
			FinishedAt: taskState.FinishedAt,
			Restarts:   int(taskState.Restarts),
			Resources:  taskResources,
			Events:     events,
		})
	}

	// Sort tasks by name for consistent display
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name < tasks[j].Name
	})

	// Calculate total resources
	resources := allocationspartials.ResourceUsage{}
	if alloc.AllocatedResources != nil {
		// Get disk from shared resources
		resources.Disk = int(alloc.AllocatedResources.Shared.DiskMB)

		// Sum up task resources for CPU and Memory
		for _, taskAlloc := range alloc.AllocatedResources.Tasks {
			if taskAlloc != nil {
				resources.CPU += int(taskAlloc.Cpu.CpuShares)
				resources.Memory += int(taskAlloc.Memory.MemoryMB)
			}
		}
	}

	// Build allocation-level events from task states
	// In the real API, allocation events come from TaskStates events
	allocEvents := make([]allocationspartials.AllocationEvent, 0)

	// Add a received event based on creation time
	if !created.IsZero() {
		allocEvents = append(allocEvents, allocationspartials.AllocationEvent{
			Type:    "Received",
			Time:    created,
			Message: "Allocation received by client",
		})
	}

	// Add task start events from task states
	for _, task := range tasks {
		if !task.StartedAt.IsZero() {
			allocEvents = append(allocEvents, allocationspartials.AllocationEvent{
				Type:    "Task Started",
				Time:    task.StartedAt,
				Message: fmt.Sprintf("Task '%s' started", task.Name),
			})
		}
	}

	// Sort events by time (most recent last for display purposes)
	sort.Slice(allocEvents, func(i, j int) bool {
		return allocEvents[i].Time.Before(allocEvents[j].Time)
	})

	return allocationspartials.AllocationDetail{
		ID:        alloc.ID,
		JobID:     alloc.JobID,
		TaskGroup: alloc.TaskGroup,
		NodeID:    alloc.NodeID,
		NodeName:  alloc.NodeName,
		Status:    alloc.ClientStatus,
		Created:   created,
		Modified:  modified,
		Tasks:     tasks,
		Resources: resources,
		Events:    allocEvents,
	}
}

// renderTabContent handles rendering individual tab content
func (h *AllocationsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, allocID, tabName string) {
	// Check permissions based on tab type
	perms := GetPermissionsFromRequest(r)

	// Logs tab requires read-logs capability
	if tabName == "logs" {
		if perms != nil && !perms.CanReadLogs("default") {
			h.handler.RenderForbidden(w, r, ForbiddenError{
				Message:    "You do not have permission to view allocation logs",
				Capability: "read-logs",
				Namespace:  "default",
				Suggestions: []string{
					"Contact your administrator to request the 'read-logs' capability",
					"You may still be able to view other allocation details",
				},
			})
			return
		}
	}

	// Exec tab requires alloc-exec capability
	if tabName == "exec" {
		if perms != nil && !perms.CanExec("default") {
			h.handler.RenderForbidden(w, r, ForbiddenError{
				Message:    "You do not have permission to execute commands in allocations",
				Capability: "alloc-exec",
				Namespace:  "default",
				Suggestions: []string{
					"Contact your administrator to request the 'alloc-exec' capability",
					"This capability allows interactive command execution in running tasks",
				},
			})
			return
		}
	}

	// Files tab requires read-fs capability
	if tabName == "files" {
		if perms != nil && !perms.CanReadFS("default") {
			h.handler.RenderForbidden(w, r, ForbiddenError{
				Message:    "You do not have permission to browse allocation filesystem",
				Capability: "read-fs",
				Namespace:  "default",
				Suggestions: []string{
					"Contact your administrator to request the 'read-fs' capability",
					"This capability allows browsing and downloading allocation files",
				},
			})
			return
		}
	}

	// Get ACL token and allocation detail data
	token := GetTokenFromRequest(r)
	allocDetail := h.getAllocationDetail(allocID, token)

	var err error
	switch tabName {
	case "tasks":
		err = RenderPartial(w, r, allocationspartials.TasksTab(allocDetail))
	case "logs":
		// Render the logs tab with SSE streaming support
		task := r.URL.Query().Get("task")
		logType := r.URL.Query().Get("type")
		if logType == "" {
			logType = "stdout"
		}
		tasks := h.getTasksForAllocation(allocID, token)
		if task == "" && len(tasks) > 0 {
			task = tasks[0]
		}
		props := allocationspartials.LogsTabProps{
			AllocID:      allocID,
			Tasks:        tasks,
			SelectedTask: task,
			LogType:      logType,
		}
		err = RenderPartial(w, r, allocationspartials.LogsTabContent(props))
	case "exec":
		// Render the exec terminal tab
		task := r.URL.Query().Get("task")
		command := r.URL.Query().Get("command")
		tasks := h.getTasksForAllocation(allocID, token)
		if task == "" && len(tasks) > 0 {
			task = tasks[0]
		}
		if command == "" {
			command = "/bin/sh"
		}
		execProps := allocationspartials.ExecTabProps{
			AllocID:      allocID,
			Tasks:        tasks,
			SelectedTask: task,
			Command:      command,
		}
		err = RenderPartial(w, r, allocationspartials.ExecTabContent(execProps))
	case "files":
		// Get task list and file data
		tasks := h.getTaskNames(allocDetail)
		selectedTask := r.URL.Query().Get("task")
		if selectedTask == "" && len(tasks) > 0 {
			selectedTask = tasks[0]
		}
		files := h.getMockFiles(allocID, selectedTask, "/")
		err = RenderPartial(w, r, allocationspartials.FilesTab(allocID, tasks, selectedTask, files))
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

// FilesListHandler handles requests for listing directory contents
func (h *AllocationsHandler) FilesListHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	h.handler.logger.Debug("files list request", "allocID", allocID, "task", task, "path", path)

	// Get directory contents
	files := h.getMockFiles(allocID, task, path)

	// Determine depth based on path
	depth := 0
	for _, c := range path {
		if c == '/' {
			depth++
		}
	}

	err := RenderPartial(w, r, allocationspartials.DirectoryListing(files, allocID, task, depth))
	if err != nil {
		h.handler.logger.Error("failed to render directory listing", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// MaxPreviewSize is the maximum number of bytes to show in file preview (100KB)
const MaxPreviewSize int64 = 100 * 1024

// FilesPreviewHandler handles requests for previewing file contents
func (h *AllocationsHandler) FilesPreviewHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")

	h.handler.logger.Debug("files preview request", "allocID", allocID, "task", task, "path", path)

	// Get mock file content
	content, size, isText := h.getMockFileContent(path)

	// Get file name from path
	fileName := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			fileName = path[i+1:]
			break
		}
	}

	// Check for binary files
	if !isText {
		props := allocationspartials.FilePreviewProps{
			FileName:    fileName,
			Path:        path,
			Content:     content,
			IsText:      false,
			Size:        size,
			IsTruncated: false,
			MaxSize:     MaxPreviewSize,
		}
		err := RenderPartial(w, r, allocationspartials.FilePreviewEnhanced(props))
		if err != nil {
			h.handler.logger.Error("failed to render file preview", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Handle large files by truncating content
	isTruncated := false
	displayContent := content
	if int64(len(content)) > MaxPreviewSize {
		isTruncated = true
		displayContent = content[:MaxPreviewSize]
		// Try to cut at a newline to avoid breaking mid-line
		for i := len(displayContent) - 1; i > int(MaxPreviewSize)-1000 && i >= 0; i-- {
			if displayContent[i] == '\n' {
				displayContent = displayContent[:i+1]
				break
			}
		}
		h.handler.logger.Debug("truncated large file for preview", "path", path, "originalSize", size, "truncatedTo", len(displayContent))
	}

	props := allocationspartials.FilePreviewProps{
		FileName:    fileName,
		Path:        path,
		Content:     displayContent,
		IsText:      true,
		Size:        size,
		IsTruncated: isTruncated,
		MaxSize:     MaxPreviewSize,
	}

	err := RenderPartial(w, r, allocationspartials.FilePreviewEnhanced(props))
	if err != nil {
		h.handler.logger.Error("failed to render file preview", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// FilesDownloadHandler handles file download requests
func (h *AllocationsHandler) FilesDownloadHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")

	h.handler.logger.Debug("files download request", "allocID", allocID, "task", task, "path", path)

	// Validate required parameters
	if path == "" {
		h.handler.logger.Warn("file download missing path parameter", "allocID", allocID)
		http.Error(w, "Missing required parameter: path", http.StatusBadRequest)
		return
	}

	// Get file name from path
	fileName := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			fileName = path[i+1:]
			break
		}
	}

	// Get mock file content
	content, size, isText := h.getMockFileContent(path)

	// Check if file exists (in production, this would check the actual filesystem)
	if content == "File content not available" && size == 0 {
		h.handler.logger.Warn("file not found", "allocID", allocID, "path", path)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Determine Content-Type based on file extension
	contentType := getContentTypeForFile(fileName, isText)

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(content)), 10))

	w.Write([]byte(content))
}

// getContentTypeForFile determines the appropriate MIME type based on file extension
func getContentTypeForFile(fileName string, isText bool) string {
	// Get extension
	ext := ""
	for i := len(fileName) - 1; i >= 0; i-- {
		if fileName[i] == '.' {
			ext = strings.ToLower(fileName[i+1:])
			break
		}
	}

	// Map common extensions to MIME types
	mimeTypes := map[string]string{
		// Text files
		"txt":    "text/plain; charset=utf-8",
		"log":    "text/plain; charset=utf-8",
		"out":    "text/plain; charset=utf-8",
		"err":    "text/plain; charset=utf-8",
		"stdout": "text/plain; charset=utf-8",
		"stderr": "text/plain; charset=utf-8",
		"md":     "text/markdown; charset=utf-8",

		// Config files
		"json":  "application/json",
		"yaml":  "application/x-yaml",
		"yml":   "application/x-yaml",
		"toml":  "application/toml",
		"xml":   "application/xml",
		"ini":   "text/plain; charset=utf-8",
		"conf":  "text/plain; charset=utf-8",
		"cfg":   "text/plain; charset=utf-8",
		"env":   "text/plain; charset=utf-8",
		"hcl":   "text/plain; charset=utf-8",
		"tf":    "text/plain; charset=utf-8",
		"nomad": "text/plain; charset=utf-8",

		// Scripts
		"sh":   "application/x-sh",
		"bash": "application/x-sh",
		"py":   "text/x-python",
		"rb":   "text/x-ruby",
		"js":   "application/javascript",
		"ts":   "application/typescript",
		"go":   "text/x-go",
		"rs":   "text/x-rust",

		// Web files
		"html": "text/html; charset=utf-8",
		"css":  "text/css; charset=utf-8",

		// Certificates
		"pem": "application/x-pem-file",
		"crt": "application/x-x509-ca-cert",
		"key": "application/x-pem-file",

		// Binary
		"db":  "application/octet-stream",
		"exe": "application/octet-stream",
		"bin": "application/octet-stream",
		"so":  "application/octet-stream",
		"dll": "application/octet-stream",
		"a":   "application/octet-stream",
	}

	if contentType, ok := mimeTypes[ext]; ok {
		return contentType
	}

	// Default based on text detection
	if isText {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// getTaskNames extracts task names from allocation detail
func (h *AllocationsHandler) getTaskNames(alloc allocationspartials.AllocationDetail) []string {
	tasks := make([]string, len(alloc.Tasks))
	for i, task := range alloc.Tasks {
		tasks[i] = task.Name
	}
	return tasks
}

// getTasksForAllocation returns the list of tasks for an allocation.
// Fetches from Nomad API and returns task names.
func (h *AllocationsHandler) getTasksForAllocation(allocID, token string) []string {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available for task list")
		return []string{}
	}

	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for task list", "allocID", allocID, "error", err)
		return []string{}
	}

	// Extract task names from TaskStates
	tasks := make([]string, 0, len(alloc.TaskStates))
	for taskName := range alloc.TaskStates {
		tasks = append(tasks, taskName)
	}

	// Sort for consistent display
	sort.Strings(tasks)
	return tasks
}

// getMockFiles returns mock file entries for testing
func (h *AllocationsHandler) getMockFiles(allocID, task, path string) []allocationspartials.FileEntry {
	now := time.Now()

	// Root level files
	if path == "/" {
		return []allocationspartials.FileEntry{
			{
				Name:    "alloc",
				Path:    "/alloc",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "local",
				Path:    "/local",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "secrets",
				Path:    "/secrets",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// /alloc directory
	if path == "/alloc" {
		return []allocationspartials.FileEntry{
			{
				Name:    "logs",
				Path:    "/alloc/logs",
				IsDir:   true,
				ModTime: now.Add(-1 * time.Hour),
			},
			{
				Name:    "data",
				Path:    "/alloc/data",
				IsDir:   true,
				ModTime: now.Add(-30 * time.Minute),
			},
		}
	}

	// /alloc/logs directory
	if path == "/alloc/logs" {
		return []allocationspartials.FileEntry{
			{
				Name:    task + ".stdout.0",
				Path:    "/alloc/logs/" + task + ".stdout.0",
				IsDir:   false,
				Size:    4523,
				ModTime: now.Add(-5 * time.Minute),
			},
			{
				Name:    task + ".stderr.0",
				Path:    "/alloc/logs/" + task + ".stderr.0",
				IsDir:   false,
				Size:    1256,
				ModTime: now.Add(-5 * time.Minute),
			},
		}
	}

	// /alloc/data directory
	if path == "/alloc/data" {
		return []allocationspartials.FileEntry{
			{
				Name:    "cache",
				Path:    "/alloc/data/cache",
				IsDir:   true,
				ModTime: now.Add(-15 * time.Minute),
			},
			{
				Name:    "state.json",
				Path:    "/alloc/data/state.json",
				IsDir:   false,
				Size:    892,
				ModTime: now.Add(-10 * time.Minute),
			},
		}
	}

	// /alloc/data/cache directory
	if path == "/alloc/data/cache" {
		return []allocationspartials.FileEntry{
			{
				Name:    "index.db",
				Path:    "/alloc/data/cache/index.db",
				IsDir:   false,
				Size:    102400,
				ModTime: now.Add(-8 * time.Minute),
			},
		}
	}

	// /local directory
	if path == "/local" {
		return []allocationspartials.FileEntry{
			{
				Name:    "config.json",
				Path:    "/local/config.json",
				IsDir:   false,
				Size:    2048,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "run.sh",
				Path:    "/local/run.sh",
				IsDir:   false,
				Size:    512,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// /secrets directory
	if path == "/secrets" {
		return []allocationspartials.FileEntry{
			{
				Name:    "api_key",
				Path:    "/secrets/api_key",
				IsDir:   false,
				Size:    64,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "cert.pem",
				Path:    "/secrets/cert.pem",
				IsDir:   false,
				Size:    1824,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// Empty for unknown paths
	return []allocationspartials.FileEntry{}
}

// getMockFileContent returns mock file content for testing
func (h *AllocationsHandler) getMockFileContent(path string) (content string, size int64, isText bool) {
	switch {
	case strings.HasSuffix(path, ".stdout.0"):
		return `[2026-01-26 10:00:00] Starting application...
[2026-01-26 10:00:01] Loading configuration from /local/config.json
[2026-01-26 10:00:01] Configuration loaded successfully
[2026-01-26 10:00:02] Initializing database connection
[2026-01-26 10:00:02] Database connected: localhost:5432
[2026-01-26 10:00:03] Starting HTTP server on :8080
[2026-01-26 10:00:03] Server is ready to accept connections
[2026-01-26 10:05:15] GET /health - 200 OK (2ms)
[2026-01-26 10:05:30] GET /api/v1/users - 200 OK (45ms)
[2026-01-26 10:06:00] POST /api/v1/tasks - 201 Created (123ms)`, 4523, true

	case strings.HasSuffix(path, ".stderr.0"):
		return `[WARN] 2026-01-26 10:00:02 - Connection pool size not specified, using default: 10
[WARN] 2026-01-26 10:05:45 - Slow query detected: SELECT * FROM tasks (took 234ms)
[ERROR] 2026-01-26 10:07:12 - Failed to parse request body: invalid JSON`, 1256, true

	case strings.HasSuffix(path, "config.json"):
		return `{
  "app": {
    "name": "web-frontend",
    "version": "1.2.3",
    "environment": "production"
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "timeout": "30s"
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "name": "appdb"
  }
}`, 2048, true

	case strings.HasSuffix(path, "run.sh"):
		return `#!/bin/bash
set -e

echo "Starting application..."
exec /usr/local/bin/app serve --config /local/config.json`, 512, true

	case strings.HasSuffix(path, "state.json"):
		return `{
  "status": "healthy",
  "last_check": "2026-01-26T10:05:00Z",
  "uptime_seconds": 7200
}`, 892, true

	case strings.HasSuffix(path, "api_key"):
		return `sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`, 64, true

	case strings.HasSuffix(path, "cert.pem"):
		return `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUXXXXXXXXXXXXXXXXXXXXXXXXXXXX
...certificate content...
-----END CERTIFICATE-----`, 1824, true

	case strings.HasSuffix(path, "index.db"):
		return "[binary data]", 102400, false

	default:
		return "File content not available", 0, true
	}
}

// Restart handles POST requests to restart an allocation.
// Route: POST /ui/allocations/{id}/restart
func (h *AllocationsHandler) Restart(w http.ResponseWriter, r *http.Request) {
	// Extract allocation ID from path: /ui/allocations/{id}/restart
	path := strings.TrimPrefix(r.URL.Path, "/ui/allocations/")
	path = strings.TrimSuffix(path, "/restart")
	allocID := path

	// Check permissions - need alloc-lifecycle capability to restart
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("default", "alloc-lifecycle") {
		h.handler.logger.Warn("restart permission denied", "allocID", allocID)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Permission Denied", "message": "You do not have the 'alloc-lifecycle' capability to restart allocations.", "duration": 5000}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Log the restart request
	h.handler.logger.Info("restart allocation request", "allocID", allocID)

	// TODO: In a real implementation, this would call the Nomad API:
	// client.Allocations().Restart(allocID, "", nil)

	// For now, simulate the restart operation
	// Send HX-Trigger header to show toast notification
	w.Header().Set("HX-Trigger", `{"showToast": {"variant": "success", "title": "Allocation Restarted", "message": "The allocation `+allocID[:8]+`... has been restarted successfully.", "duration": 5000}}`)
	// Trigger a page refresh after a short delay to show updated status
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Stop handles POST requests to stop an allocation.
// Route: POST /ui/allocations/{id}/stop
func (h *AllocationsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	// Extract allocation ID from path: /ui/allocations/{id}/stop
	path := strings.TrimPrefix(r.URL.Path, "/ui/allocations/")
	path = strings.TrimSuffix(path, "/stop")
	allocID := path

	// Check permissions - need alloc-lifecycle capability to stop
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("default", "alloc-lifecycle") {
		h.handler.logger.Warn("stop permission denied", "allocID", allocID)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Permission Denied", "message": "You do not have the 'alloc-lifecycle' capability to stop allocations.", "duration": 5000}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Log the stop request
	h.handler.logger.Info("stop allocation request", "allocID", allocID)

	// TODO: In a real implementation, this would call the Nomad API:
	// client.Allocations().Stop(allocID, nil)

	// For now, simulate the stop operation
	// Send HX-Trigger header to show toast notification
	w.Header().Set("HX-Trigger", `{"showToast": {"variant": "success", "title": "Allocation Stopped", "message": "The allocation `+allocID[:8]+`... has been stopped.", "duration": 5000}}`)
	// Trigger a page refresh after a short delay to show updated status
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
