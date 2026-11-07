package handlers

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	pathpkg "path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/allocations"
	"github.com/open-wander/wander/ui/templates/partials/allocationspartials"
)

// AllocationSummary represents live allocation-list data used for filtering,
// sorting, and projection into the rendered list.
type AllocationSummary struct {
	ID                 string
	JobID              string
	JobType            string
	JobVersion         uint64
	TaskGroup          string
	NodeID             string
	NodeName           string
	Status             string
	DesiredStatus      string
	DesiredDescription string
	Created            time.Time
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

	h.handler.RenderNotFound(w, r, NotFoundError{
		Message:     "The allocation page you requested could not be found.",
		RequestPath: path,
		Suggestions: []string{
			`Go to <a href="/ui/allocations" class="underline">Allocations List</a>`,
			`Return to <a href="/ui/" class="underline">Home</a>`,
		},
	})
}

// List handles the allocations list page
func (h *AllocationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("allocations list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters first to get namespace
	query := r.URL.Query()
	namespace := GetNamespaceFromRequest(r)
	region := GetRegionFromRequest(r)

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
	typeFilter := query.Get("type")
	taskGroupFilter := query.Get("task_group")
	nodeFilter := query.Get("node")
	desiredStatusFilter := query.Get("scheduling")
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")
	pageStr := query.Get("page")
	perPageStr := query.Get("per_page")

	// Default sort
	if sortColumn == "" {
		sortColumn = "created"
	}
	if !validAllocationSortColumn(sortColumn) {
		sortColumn = "created"
	}
	if sortDir == "" {
		sortDir = "desc"
	}
	if sortDir != "asc" && sortDir != "desc" {
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

	// Read the complete scoped list once so filter choices do not disappear as
	// filters are composed.
	allAllocations, err := h.getAllocationsList(r, namespace, token)
	if err != nil {
		h.handler.renderJobScopedReadError(w, r, namespace, "list allocations", "allocation", false, err)
		return
	}

	// Extract filter options from all allocations (before filtering)
	filterOptions := h.extractFilterOptions(allAllocations)

	filteredAllocations := h.filterAllocations(allAllocations, allocationListFilters{
		status: statusFilter, job: jobFilter, jobType: typeFilter,
		taskGroup: taskGroupFilter, node: nodeFilter,
		desiredStatus: desiredStatusFilter, search: searchQuery,
	})

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
			ID:                 a.ID,
			JobID:              a.JobID,
			JobType:            a.JobType,
			JobVersion:         a.JobVersion,
			TaskGroup:          a.TaskGroup,
			NodeID:             a.NodeID,
			NodeName:           a.NodeName,
			Status:             a.Status,
			DesiredStatus:      a.DesiredStatus,
			DesiredDescription: a.DesiredDescription,
			Created:            a.Created,
			IDPrefixMatch:      idPrefixMatch,
			MatchedPrefix:      matchedPrefix,
			DetailURL:          scopedUIURL("/ui/allocations/"+url.PathEscape(a.ID), namespace, region),
			JobURL:             scopedUIURL("/ui/jobs/"+url.PathEscape(a.JobID), namespace, region),
			ClientURL:          scopedUIURL("/ui/clients/"+url.PathEscape(a.NodeID), namespace, region),
			LogsURL:            scopedUIURL("/ui/allocations/"+url.PathEscape(a.ID)+"/logs", namespace, region),
		}
	}

	// Build props
	props := allocationspartials.ListProps{
		Allocations: allocationsList,
		FilterOptions: allocationspartials.FilterOptions{
			Statuses:        filterOptions.Statuses,
			Jobs:            filterOptions.Jobs,
			JobTypes:        filterOptions.JobTypes,
			TaskGroups:      filterOptions.TaskGroups,
			Nodes:           filterOptions.Nodes,
			DesiredStatuses: filterOptions.DesiredStatuses,
		},
		Filters: allocationspartials.ListFilters{
			Namespace:     namespace,
			Region:        region,
			Status:        statusFilter,
			Job:           jobFilter,
			JobType:       typeFilter,
			TaskGroup:     taskGroupFilter,
			Node:          nodeFilter,
			DesiredStatus: desiredStatusFilter,
			Search:        searchQuery,
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
	layoutProps := h.handler.BaseLayoutProps(r,
		"Allocations - Wander",
		"View and manage Wander allocations",
		"/ui/allocations",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Allocations", URL: ""},
		},
	)

	// Render the allocations list page
	err = Render(w, r,
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
	Statuses        []string
	Jobs            []string
	JobTypes        []string
	TaskGroups      []string
	Nodes           []string
	DesiredStatuses []string
}

// extractFilterOptions extracts unique values for filter dropdowns
func (h *AllocationsHandler) extractFilterOptions(allocs []AllocationSummary) FilterOptions {
	statusSet := make(map[string]bool)
	jobSet := make(map[string]bool)
	typeSet := make(map[string]bool)
	taskGroupSet := make(map[string]bool)
	nodeSet := make(map[string]bool)
	desiredStatusSet := make(map[string]bool)

	for _, a := range allocs {
		addAllocationFilterValue(statusSet, a.Status)
		addAllocationFilterValue(jobSet, a.JobID)
		addAllocationFilterValue(typeSet, a.JobType)
		addAllocationFilterValue(taskGroupSet, a.TaskGroup)
		addAllocationFilterValue(nodeSet, a.NodeName)
		addAllocationFilterValue(desiredStatusSet, a.DesiredStatus)
	}

	return FilterOptions{
		Statuses:        sortedAllocationFilterValues(statusSet),
		Jobs:            sortedAllocationFilterValues(jobSet),
		JobTypes:        sortedAllocationFilterValues(typeSet),
		TaskGroups:      sortedAllocationFilterValues(taskGroupSet),
		Nodes:           sortedAllocationFilterValues(nodeSet),
		DesiredStatuses: sortedAllocationFilterValues(desiredStatusSet),
	}
}

func addAllocationFilterValue(values map[string]bool, value string) {
	if value != "" {
		values[value] = true
	}
}

func sortedAllocationFilterValues(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type allocationListFilters struct {
	status        string
	job           string
	jobType       string
	taskGroup     string
	node          string
	desiredStatus string
	search        string
}

// filterAllocations applies filters to the allocations list.
// When searching, ID prefix matches are prioritized over other matches.
func (h *AllocationsHandler) filterAllocations(allocs []AllocationSummary, filters allocationListFilters) []AllocationSummary {
	// Apply non-search filters first
	preFiltered := make([]AllocationSummary, 0, len(allocs))

	for _, a := range allocs {
		// Status filter
		if filters.status != "" && a.Status != filters.status {
			continue
		}

		// Job filter
		if filters.job != "" && a.JobID != filters.job {
			continue
		}
		if filters.jobType != "" && a.JobType != filters.jobType {
			continue
		}
		if filters.taskGroup != "" && a.TaskGroup != filters.taskGroup {
			continue
		}

		// Node filter
		if filters.node != "" && a.NodeName != filters.node && a.NodeID != filters.node {
			continue
		}
		if filters.desiredStatus != "" && a.DesiredStatus != filters.desiredStatus {
			continue
		}

		preFiltered = append(preFiltered, a)
	}

	// If no search query, return pre-filtered results
	if strings.TrimSpace(filters.search) == "" {
		return preFiltered
	}

	// Apply search filter with ID prefix prioritization
	searchLower := strings.ToLower(strings.TrimSpace(filters.search))

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
			strings.Contains(strings.ToLower(a.JobType), searchLower) ||
			strings.Contains(strings.ToLower(a.TaskGroup), searchLower) ||
			strings.Contains(strings.ToLower(a.NodeName), searchLower) ||
			strings.Contains(strings.ToLower(a.NodeID), searchLower) ||
			strings.Contains(strings.ToLower(a.DesiredStatus), searchLower) {
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
		comparison := 0

		switch column {
		case "id":
			comparison = cmp.Compare(allocs[i].ID, allocs[j].ID)
		case "job":
			comparison = cmp.Compare(allocs[i].JobID, allocs[j].JobID)
		case "type":
			comparison = cmp.Compare(allocs[i].JobType, allocs[j].JobType)
		case "taskgroup":
			comparison = cmp.Compare(allocs[i].TaskGroup, allocs[j].TaskGroup)
		case "version":
			comparison = cmp.Compare(allocs[i].JobVersion, allocs[j].JobVersion)
		case "node":
			comparison = cmp.Compare(allocs[i].NodeName, allocs[j].NodeName)
		case "status":
			comparison = cmp.Compare(allocs[i].Status, allocs[j].Status)
		case "scheduling":
			comparison = cmp.Compare(allocs[i].DesiredStatus, allocs[j].DesiredStatus)
		case "created":
			comparison = allocs[i].Created.Compare(allocs[j].Created)
		default:
			comparison = allocs[i].Created.Compare(allocs[j].Created)
		}

		if comparison == 0 {
			comparison = cmp.Compare(allocs[i].ID, allocs[j].ID)
		}
		if dir == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func validAllocationSortColumn(column string) bool {
	switch column {
	case "id", "job", "type", "taskgroup", "version", "node", "status", "scheduling", "created":
		return true
	default:
		return false
	}
}

// getAllocationsList fetches allocations from the Nomad API.
func (h *AllocationsHandler) getAllocationsList(r *http.Request, namespace, token string) ([]AllocationSummary, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available for allocation list")
		return nil, errNomadClientUnavailable
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
		Region:    GetRegionFromRequest(r),
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching allocations from API",
		"namespace", namespace,
		"region", queryOpts.Region,
		"hasToken", token != "",
	)

	// Fetch allocations from the API
	allocs, _, err := client.Allocations().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocations from Nomad API", "error", err)
		return nil, err
	}

	h.handler.logger.Debug("fetched allocations from API", "count", len(allocs))

	// Convert API response to our summary type
	result := make([]AllocationSummary, 0, len(allocs))
	for _, alloc := range allocs {
		if alloc == nil {
			continue
		}
		created := time.Time{}
		if alloc.CreateTime > 0 {
			created = time.Unix(0, alloc.CreateTime)
		}

		result = append(result, AllocationSummary{
			ID:                 alloc.ID,
			JobID:              alloc.JobID,
			JobType:            alloc.JobType,
			JobVersion:         alloc.JobVersion,
			TaskGroup:          alloc.TaskGroup,
			NodeID:             alloc.NodeID,
			NodeName:           alloc.NodeName,
			Status:             alloc.ClientStatus,
			DesiredStatus:      alloc.DesiredStatus,
			DesiredDescription: alloc.DesiredDescription,
			Created:            created,
		})
	}

	return result, nil
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
	legacyFSRoute := false
	if len(parts) > 1 && parts[1] == "fs" {
		legacyFSRoute = true
		tabName = "files"
		setAllocationFSRouteQuery(r, "", parts[2:])
	} else if len(parts) > 2 && parts[2] == "fs" {
		legacyFSRoute = true
		tabName = "files"
		setAllocationFSRouteQuery(r, parts[1], parts[3:])
	} else if len(parts) > 1 {
		tabName = parts[1]
	}
	if !legacyFSRoute && len(parts) > 2 {
		subAction = parts[2]
	}
	if !legacyFSRoute && (len(parts) > 3 || (len(parts) > 2 && tabName != "files")) {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "The allocation page you requested could not be found.",
			RequestPath: r.URL.Path,
		})
		return
	}
	if tabName != "" && tabName != "tasks" && tabName != "exec" && tabName != "files" && tabName != "raw" {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "The allocation tab you requested could not be found.",
			RequestPath: r.URL.Path,
		})
		return
	}

	// Log the request
	h.handler.logger.Debug("allocation detail request", "allocID", actualAllocID, "tab", tabName, "subAction", subAction, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read-job capability to view allocation details
	namespace := GetNamespaceFromRequest(r)
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadJob(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view this allocation",
			Capability: "read-job",
			Namespace:  namespace,
			Suggestions: []string{
				"Allocations inherit job permissions. You need the 'read-job' capability",
				"Contact your administrator to request appropriate permissions",
			},
		})
		return
	}
	if !h.authorizeAllocationTab(w, r, tabName) {
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
		case "content":
			h.FilesContentHandler(w, r, actualAllocID)
			return
		case "stream":
			h.FilesStreamHandler(w, r, actualAllocID)
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
	loadTaskRuntime := tabName == "" || tabName == "tasks"
	allocDetail, fetchErr := h.getAllocationDetail(r, actualAllocID, token, loadTaskRuntime)
	if fetchErr != nil {
		h.renderAllocationFetchError(w, r, actualAllocID, fetchErr)
		return
	}
	if tabName != "" && tabName != "tasks" {
		content, contentErr := h.allocationTabContent(r, actualAllocID, tabName, token, allocDetail)
		if contentErr != nil {
			h.renderAllocationTabError(w, r, actualAllocID, tabName, contentErr)
			return
		}
		allocDetail.ActiveTab = tabName
		allocDetail.ActiveTabContent = content
	}

	// Build layout props - use short ID for display
	shortID := actualAllocID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	// Build breadcrumbs - include job in the hierarchy if available
	breadcrumbs := []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
	}
	if allocDetail.JobID != "" {
		breadcrumbs = append(breadcrumbs,
			components.Breadcrumb{Label: "Jobs", URL: "/ui/jobs"},
			components.Breadcrumb{Label: allocDetail.JobID, URL: "/ui/jobs/" + allocDetail.JobID},
			components.Breadcrumb{Label: shortID, URL: ""},
		)
	} else {
		// Fallback if no job info
		breadcrumbs = append(breadcrumbs,
			components.Breadcrumb{Label: "Allocations", URL: "/ui/allocations"},
			components.Breadcrumb{Label: shortID, URL: ""},
		)
	}

	layoutProps := h.handler.BaseLayoutProps(r,
		"Allocation "+shortID+" - Wander",
		"Allocation detail for "+actualAllocID,
		"/ui/allocations/"+actualAllocID,
		breadcrumbs,
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		allocations.Detail(layoutProps, allocDetail),   // Full page with layout
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
func (h *AllocationsHandler) getAllocationDetail(r *http.Request, allocID, token string, loadTaskRuntime bool) (allocationspartials.AllocationDetail, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return allocationspartials.AllocationDetail{}, fmt.Errorf("Wander API client is unavailable")
	}

	// Scope all allocation reads to the browser's selected cluster context.
	queryOpts := &api.QueryOptions{
		Namespace: GetNamespaceFromRequest(r),
		Region:    GetRegionFromRequest(r),
	}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocation from the API
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		return allocationspartials.AllocationDetail{}, err
	}

	h.handler.logger.Debug("fetched allocation from API", "allocID", allocID, "status", alloc.ClientStatus)

	ports := make([]allocationspartials.AllocationPort, 0)
	if alloc.AllocatedResources != nil {
		for _, port := range alloc.AllocatedResources.Shared.Ports {
			ports = append(ports, allocationspartials.AllocationPort{
				Label:    port.Label,
				HostIP:   port.HostIP,
				HostPort: port.Value,
				To:       port.To,
			})
		}
		sort.Slice(ports, func(i, j int) bool {
			if ports[i].Label != ports[j].Label {
				return ports[i].Label < ports[j].Label
			}
			if ports[i].HostIP != ports[j].HostIP {
				return ports[i].HostIP < ports[j].HostIP
			}
			return ports[i].HostPort < ports[j].HostPort
		})
	}

	lifecycles := allocationTaskLifecycles(alloc)

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

		lifecycle := lifecycles[taskName]
		tasks = append(tasks, allocationspartials.TaskStatus{
			Name:             taskName,
			State:            taskState.State,
			Failed:           taskState.Failed,
			StartedAt:        taskState.StartedAt,
			FinishedAt:       taskState.FinishedAt,
			Restarts:         taskState.Restarts,
			Resources:        taskResources,
			Events:           events,
			LifecycleHook:    lifecycle.Hook,
			LifecycleSidecar: lifecycle.Sidecar,
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

	detail := allocationspartials.AllocationDetail{
		ID:        alloc.ID,
		JobID:     alloc.JobID,
		TaskGroup: alloc.TaskGroup,
		NodeID:    alloc.NodeID,
		NodeName:  alloc.NodeName,
		Status:    alloc.ClientStatus,
		Created:   created,
		Modified:  modified,
		Tasks:     tasks,
		Ports:     ports,
		History:   allocationHistory(alloc, queryOpts.Namespace, queryOpts.Region),
		Resources: resources,
		Events:    allocEvents,
	}
	if loadTaskRuntime {
		h.enrichAllocationTaskRuntime(client, alloc, queryOpts, &detail)
	}
	return detail, nil
}

func (h *AllocationsHandler) enrichAllocationTaskRuntime(client *api.Client, alloc *api.Allocation, queryOpts *api.QueryOptions, detail *allocationspartials.AllocationDetail) {
	detail.ServicesReady = true
	registrations, _, err := client.Allocations().Services(alloc.ID, queryOpts)
	if err != nil {
		h.markAllocationRuntimeUnavailable(&detail.ServicesReady, &detail.ServicesError, "Live service registrations are unavailable.", "services", alloc.ID, err)
	} else {
		detail.Services = projectAllocationServices(registrations, queryOpts.Namespace, queryOpts.Region)
	}

	detail.StatsReady = true
	stats, err := client.Allocations().Stats(alloc, queryOpts)
	if err != nil {
		h.markAllocationRuntimeUnavailable(&detail.StatsReady, &detail.StatsError, "Live resource statistics are unavailable.", "statistics", alloc.ID, err)
		return
	}
	if stats == nil {
		return
	}
	detail.Stats = projectLiveAllocationStats(stats.ResourceUsage, stats.Timestamp)
	taskIndexes := make(map[string]int, len(detail.Tasks))
	for index := range detail.Tasks {
		taskIndexes[detail.Tasks[index].Name] = index
	}
	for taskName, taskStats := range stats.Tasks {
		index, ok := taskIndexes[taskName]
		if !ok || taskStats == nil {
			continue
		}
		timestamp := taskStats.Timestamp
		if timestamp == 0 {
			timestamp = stats.Timestamp
		}
		detail.Tasks[index].Resources.LiveStats = projectLiveResourceStats(taskStats.ResourceUsage, timestamp)
	}
}

func (h *AllocationsHandler) markAllocationRuntimeUnavailable(ready *bool, message *string, userMessage, resource, allocID string, err error) {
	*ready = false
	*message = userMessage
	h.handler.logger.Warn("failed to fetch allocation "+resource, "allocID", allocID, "error", err)
}

func projectAllocationServices(registrations []*api.ServiceRegistration, namespace, region string) []allocationspartials.AllocationService {
	services := make([]allocationspartials.AllocationService, 0, len(registrations))
	for _, registration := range registrations {
		if registration == nil {
			continue
		}
		services = append(services, allocationspartials.AllocationService{
			Name:      registration.ServiceName,
			Address:   registration.Address,
			Port:      registration.Port,
			Tags:      strings.Join(registration.Tags, ", "),
			DetailURL: scopedUIURL("/ui/services/"+url.PathEscape(registration.ServiceName), namespace, region),
		})
	}
	sort.Slice(services, func(i, j int) bool {
		left, right := services[i], services[j]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Address != right.Address {
			return left.Address < right.Address
		}
		return left.Port < right.Port
	})
	return services
}

func projectLiveAllocationStats(usage *api.ResourceUsage, timestamp int64) *allocationspartials.AllocationStats {
	live := projectLiveResourceStats(usage, timestamp)
	if live == nil {
		return nil
	}
	return &allocationspartials.AllocationStats{LiveResourceStats: *live}
}

func projectLiveResourceStats(usage *api.ResourceUsage, timestamp int64) *allocationspartials.LiveResourceStats {
	if usage == nil {
		return nil
	}
	result := &allocationspartials.LiveResourceStats{}
	if usage.CpuStats != nil {
		result.CPUPercent = usage.CpuStats.Percent
	}
	if usage.MemoryStats != nil {
		result.MemoryUsage = usage.MemoryStats.Usage
		result.MemoryRSS = usage.MemoryStats.RSS
	}
	if timestamp > 0 {
		result.Timestamp = time.Unix(0, timestamp)
	}
	return result
}

func allocationHistory(alloc *api.Allocation, namespace, region string) allocationspartials.AllocationHistory {
	history := allocationspartials.AllocationHistory{
		Previous:    allocationHistoryLink("allocations", alloc.PreviousAllocation, namespace, region),
		Next:        allocationHistoryLink("allocations", alloc.NextAllocation, namespace, region),
		PreemptedBy: allocationHistoryLink("allocations", alloc.PreemptedByAllocation, namespace, region),
	}
	for _, allocID := range alloc.PreemptedAllocations {
		if allocID != "" {
			history.PreemptedAllocations = append(history.PreemptedAllocations, allocationHistoryLink("allocations", allocID, namespace, region))
		}
	}
	slices.SortFunc(history.PreemptedAllocations, func(left, right allocationspartials.AllocationLink) int {
		return strings.Compare(left.ID, right.ID)
	})
	if alloc.RescheduleTracker != nil {
		for _, event := range alloc.RescheduleTracker.Events {
			if event == nil {
				continue
			}
			reschedule := allocationspartials.AllocationRescheduleEvent{
				PreviousAllocation: allocationHistoryLink("allocations", event.PrevAllocID, namespace, region),
				PreviousNode:       allocationHistoryLink("clients", event.PrevNodeID, namespace, region),
			}
			if event.RescheduleTime > 0 {
				reschedule.Time = time.Unix(0, event.RescheduleTime)
			}
			history.Reschedules = append(history.Reschedules, reschedule)
		}
		sort.Slice(history.Reschedules, func(i, j int) bool {
			left, right := history.Reschedules[i], history.Reschedules[j]
			if !left.Time.Equal(right.Time) {
				return left.Time.After(right.Time)
			}
			return left.PreviousAllocation.ID < right.PreviousAllocation.ID
		})
	}
	return history
}

func allocationHistoryLink(resource, id, namespace, region string) allocationspartials.AllocationLink {
	if id == "" {
		return allocationspartials.AllocationLink{}
	}
	return allocationspartials.AllocationLink{
		ID:  id,
		URL: scopedUIURL("/ui/"+resource+"/"+url.PathEscape(id), namespace, region),
	}
}

func allocationTaskLifecycles(alloc *api.Allocation) map[string]api.TaskLifecycle {
	lifecycles := make(map[string]api.TaskLifecycle)
	if alloc == nil || alloc.Job == nil {
		return lifecycles
	}
	for _, group := range alloc.Job.TaskGroups {
		if group == nil || group.Name == nil || *group.Name != alloc.TaskGroup {
			continue
		}
		for _, task := range group.Tasks {
			if task != nil && task.Lifecycle != nil && !task.Lifecycle.Empty() {
				lifecycles[task.Name] = *task.Lifecycle
			}
		}
		break
	}
	return lifecycles
}

func (h *AllocationsHandler) renderAllocationFetchError(w http.ResponseWriter, r *http.Request, allocID string, err error) {
	if apiResponseHasStatus(err, http.StatusNotFound) {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     fmt.Sprintf("Allocation %q was not found.", allocID),
			RequestPath: r.URL.Path,
			Suggestions: []string{`Go to <a href="/ui/allocations" class="underline">Allocations</a>`},
		})
		return
	}
	if h.handler.CheckClusterError(w, r, err) {
		return
	}
	h.handler.logger.Error("failed to fetch allocation from Wander API", "allocID", allocID, "error", err)
	http.Error(w, "Failed to fetch allocation from Wander API", http.StatusBadGateway)
}

// renderTabContent handles rendering individual tab content
func (h *AllocationsHandler) renderTabContent(w http.ResponseWriter, r *http.Request, allocID, tabName string) {
	// Get ACL token and allocation detail data
	token := GetTokenFromRequest(r)
	allocDetail, fetchErr := h.getAllocationDetail(r, allocID, token, tabName == "tasks")
	if fetchErr != nil {
		h.renderAllocationFetchError(w, r, allocID, fetchErr)
		return
	}

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
		tasks := h.getTasksForAllocation(r, allocID, token)
		task = selectAllocationTask(task, tasks)
		props := allocationspartials.LogsTabProps{
			AllocID:      allocID,
			Tasks:        tasks,
			SelectedTask: task,
			LogType:      logType,
		}
		err = RenderPartial(w, r, allocationspartials.LogsTabContent(props))
	case "exec", "files", "raw":
		content, contentErr := h.allocationTabContent(r, allocID, tabName, token, allocDetail)
		if contentErr != nil {
			h.renderAllocationTabError(w, r, allocID, tabName, contentErr)
			return
		}
		err = RenderPartial(w, r, content)
	default:
		http.NotFound(w, r)
		return
	}

	if err != nil {
		h.handler.logger.Error("failed to render tab content", "tab", tabName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *AllocationsHandler) renderAllocationTabError(w http.ResponseWriter, r *http.Request, allocID, tabName string, err error) {
	if tabName == "files" {
		h.renderFileReadError(w, r, allocID, r.URL.Query().Get("path"), err)
		return
	}
	if h.handler.CheckClusterError(w, r, err) {
		return
	}
	h.handler.logger.Error("failed to build allocation tab", "allocID", allocID, "tab", tabName, "error", err)
	http.Error(w, "Failed to fetch allocation", http.StatusBadGateway)
}

func (h *AllocationsHandler) authorizeAllocationTab(w http.ResponseWriter, r *http.Request, tabName string) bool {
	perms := GetPermissionsFromRequest(r)
	if perms == nil {
		return true
	}

	namespace := GetNamespaceFromRequest(r)
	var forbidden *ForbiddenError
	switch {
	case tabName == "logs" && !perms.CanReadLogs(namespace):
		forbidden = &ForbiddenError{Message: "You do not have permission to view allocation logs", Capability: "read-logs", Namespace: namespace}
	case tabName == "exec" && !perms.CanExec(namespace):
		forbidden = &ForbiddenError{Message: "You do not have permission to execute commands in allocations", Capability: "alloc-exec", Namespace: namespace}
	case tabName == "files" && !perms.CanReadFS(namespace):
		forbidden = &ForbiddenError{Message: "You do not have permission to browse allocation filesystem", Capability: "read-fs", Namespace: namespace}
	}
	if forbidden == nil {
		return true
	}

	forbidden.Suggestions = []string{"Contact your administrator to request the required capability", "You may still be able to view other allocation details"}
	h.handler.RenderForbidden(w, r, *forbidden)
	return false
}

func (h *AllocationsHandler) allocationTabContent(r *http.Request, allocID, tabName, token string, allocDetail allocationspartials.AllocationDetail) (templ.Component, error) {
	switch tabName {
	case "exec":
		tasks := h.getTasksForAllocation(r, allocID, token)
		task := selectAllocationTask(r.URL.Query().Get("task"), tasks)
		command := r.URL.Query().Get("command")
		if command == "" {
			command = "/bin/sh"
		}
		return allocationspartials.ExecTabContent(allocationspartials.ExecTabProps{
			AllocID: allocID, Tasks: tasks, SelectedTask: task, Command: command,
		}), nil
	case "files":
		tasks := h.getTaskNames(allocDetail)
		selectedTask := r.URL.Query().Get("task")
		if !isAllocationFSRoute(r.URL.Path) {
			selectedTask = selectAllocationTask(selectedTask, tasks)
		}
		initialPath := r.URL.Query().Get("path")
		if initialPath == "" {
			initialPath = "/"
		}
		files, err := h.getFiles(r, allocID, selectedTask, initialPath, token)
		if err != nil {
			return nil, err
		}
		return allocationspartials.FilesTab(allocID, tasks, selectedTask, files), nil
	case "raw":
		raw, err := h.getRawAllocation(r, allocID, token)
		if err != nil {
			return nil, err
		}
		return allocationspartials.RawTab(raw), nil
	default:
		return nil, fmt.Errorf("unknown allocation tab %q", tabName)
	}
}

func selectAllocationTask(selected string, tasks []string) string {
	if selected == "" && len(tasks) > 0 {
		return tasks[0]
	}
	return selected
}

func (h *AllocationsHandler) getRawAllocation(r *http.Request, allocID, token string) (string, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return "", fmt.Errorf("Wander API client is unavailable")
	}

	queryOpts := &api.QueryOptions{AuthToken: token}
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		return "", fmt.Errorf("fetch allocation: %w", err)
	}

	raw, err := json.MarshalIndent(alloc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode allocation: %w", err)
	}
	return string(raw), nil
}

// FilesListHandler handles requests for listing directory contents
func (h *AllocationsHandler) FilesListHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	h.handler.logger.Debug("files list request", "allocID", allocID, "task", task, "path", path)

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	// Get directory contents from API
	files, fetchErr := h.getFiles(r, allocID, task, path, token)
	if fetchErr != nil {
		h.renderFileReadError(w, r, allocID, path, fetchErr)
		return
	}

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

// FilePreviewSize matches the inherited large-file Head and Tail window.
const FilePreviewSize int64 = 50000

// FilesPreviewHandler handles requests for previewing file contents
func (h *AllocationsHandler) FilesPreviewHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "head"
	}
	if mode != "head" && mode != "tail" {
		http.Error(w, "Invalid file preview mode: expected head or tail", http.StatusBadRequest)
		return
	}
	if path == "" {
		http.Error(w, "Missing required parameter: path", http.StatusBadRequest)
		return
	}

	h.handler.logger.Debug("files preview request", "allocID", allocID, "task", task, "path", path)

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	client, alloc, fileInfo, queryOpts, allocationPath, err := h.getAllocationFile(r, allocID, task, path, token)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}

	// Get file name from path
	fileName := pathpkg.Base(path)
	isText := isTextContentType(fileInfo.ContentType, path)
	isImage := isPreviewableImage(fileInfo.ContentType)
	baseProps := allocationspartials.FilePreviewProps{
		AllocID:   allocID,
		Task:      task,
		FileName:  fileName,
		Path:      path,
		IsText:    isText,
		IsImage:   isImage,
		Size:      fileInfo.Size,
		MaxSize:   FilePreviewSize,
		Mode:      mode,
		ImageURL:  allocationspartials.FileActionURL(allocID, "content", task, path),
		StreamURL: allocationspartials.FileActionURL(allocID, "stream", task, path),
	}

	// Check for binary files
	if !isText {
		err := RenderPartial(w, r, allocationspartials.FilePreviewEnhanced(baseProps))
		if err != nil {
			h.handler.logger.Error("failed to render file preview", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	offset, limit := filePreviewRange(fileInfo.Size, mode)
	reader, err := client.AllocFS().ReadAt(alloc, allocationPath, offset, limit, queryOpts)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}

	baseProps.Content = string(content)
	baseProps.IsTruncated = fileInfo.Size > int64(len(content))

	err = RenderPartial(w, r, allocationspartials.FilePreviewEnhanced(baseProps))
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

	fileName := pathpkg.Base(path)

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	client, alloc, fileInfo, queryOpts, allocationPath, err := h.getAllocationFile(r, allocID, task, path, token)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	reader, err := client.AllocFS().Cat(alloc, allocationPath, queryOpts)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	defer reader.Close()

	// Determine Content-Type based on file extension
	contentType := getContentTypeForFile(fileName, isTextContentType(fileInfo.ContentType, path))

	// Set headers for download
	w.Header().Set("Content-Disposition", downloadContentDisposition(fileName))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := io.Copy(w, reader); err != nil {
		h.handler.logger.Warn("file download interrupted", "allocID", allocID, "path", path, "error", err)
	}
}

// FilesContentHandler serves allowlisted images for the same-origin preview.
func (h *AllocationsHandler) FilesContentHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing required parameter: path", http.StatusBadRequest)
		return
	}

	client, alloc, fileInfo, queryOpts, allocationPath, err := h.getAllocationFile(r, allocID, task, path, GetTokenFromRequest(r))
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	if !isPreviewableImage(fileInfo.ContentType) {
		http.Error(w, "Inline preview is not available for this file type", http.StatusUnsupportedMediaType)
		return
	}
	reader, err := client.AllocFS().Cat(alloc, allocationPath, queryOpts)
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", fileInfo.ContentType)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, reader); err != nil {
		h.handler.logger.Warn("image preview interrupted", "allocID", allocID, "path", path, "error", err)
	}
}

// FilesStreamHandler follows a growing text file from its latest preview window.
func (h *AllocationsHandler) FilesStreamHandler(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing required parameter: path", http.StatusBadRequest)
		return
	}

	client, alloc, fileInfo, queryOpts, allocationPath, err := h.getAllocationFile(r, allocID, task, path, GetTokenFromRequest(r))
	if err != nil {
		h.renderFileReadError(w, r, allocID, path, err)
		return
	}
	if !isTextContentType(fileInfo.ContentType, path) {
		http.Error(w, "Live streaming is only available for text files", http.StatusUnsupportedMediaType)
		return
	}

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		h.handler.logger.Error("failed to create allocation file stream", "allocID", allocID, "path", path, "error", err)
		http.Error(w, "Unable to start the file stream", http.StatusInternalServerError)
		return
	}
	defer sse.Close()

	cancel := make(chan struct{})
	go func() {
		select {
		case <-r.Context().Done():
		case <-sse.Done():
		}
		close(cancel)
	}()
	frames, streamErrors := client.AllocFS().Stream(alloc, allocationPath, "end", FilePreviewSize, cancel, queryOpts)
	for frames != nil || streamErrors != nil {
		select {
		case <-r.Context().Done():
			return
		case <-sse.Done():
			return
		case frame, ok := <-frames:
			if !ok {
				frames = nil
				continue
			}
			if frame == nil || frame.IsHeartbeat() {
				continue
			}
			encoded := base64.StdEncoding.EncodeToString(frame.Data)
			if err := sse.WriteJSON("file", map[string]string{"data": encoded}); err != nil {
				return
			}
		case streamErr, ok := <-streamErrors:
			if !ok {
				streamErrors = nil
				continue
			}
			if streamErr == nil || streamErr == io.EOF {
				continue
			}
			_ = sse.WriteJSON("error", map[string]string{"message": streamErr.Error()})
			return
		}
	}
}

func downloadContentDisposition(fileName string) string {
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = "download"
	}
	return mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
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
func (h *AllocationsHandler) getTasksForAllocation(r *http.Request, allocID, token string) []string {
	client := h.handler.NomadClientForRequest(r)
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

// getFiles fetches file entries from the Nomad API for a given allocation and path.
// Returns a list of FileEntry objects representing files and directories.
func (h *AllocationsHandler) getFiles(r *http.Request, allocID, task, path, token string) ([]allocationspartials.FileEntry, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, fmt.Errorf("Wander API client is unavailable")
	}

	queryOpts := allocationQueryOptions(r, token)

	// Fetch allocation info to get the allocation object
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		return nil, fmt.Errorf("fetch allocation for file listing: %w", err)
	}

	allocationPath, err := allocationFilePath(task, path)
	if err != nil {
		return nil, err
	}

	// Call AllocFS().List to get file listing
	files, _, err := client.AllocFS().List(alloc, allocationPath, queryOpts)
	if err != nil {
		return nil, fmt.Errorf("list allocation files: %w", err)
	}

	h.handler.logger.Debug("fetched files from API", "allocID", allocID, "path", path, "count", len(files))

	// Convert API response to FileEntry format
	result := make([]allocationspartials.FileEntry, 0, len(files))
	for _, file := range files {
		// Build full path by joining parent path with file name
		fullPath := path
		if !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}
		fullPath += file.Name

		result = append(result, allocationspartials.FileEntry{
			Name:    file.Name,
			Path:    fullPath,
			IsDir:   file.IsDir,
			Size:    file.Size,
			ModTime: file.ModTime,
		})
	}

	return result, nil
}

func (h *AllocationsHandler) getAllocationFile(r *http.Request, allocID, task, path, token string) (*api.Client, *api.Allocation, *api.AllocFileInfo, *api.QueryOptions, string, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, nil, nil, nil, "", fmt.Errorf("Wander API client is unavailable")
	}

	queryOpts := allocationQueryOptions(r, token)

	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("fetch allocation: %w", err)
	}

	allocationPath, err := allocationFilePath(task, path)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	fileInfo, _, err := client.AllocFS().Stat(alloc, allocationPath, queryOpts)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("stat allocation file: %w", err)
	}
	if fileInfo.IsDir {
		return nil, nil, nil, nil, "", fmt.Errorf("%w: %s", errAllocationFileIsDirectory, path)
	}

	return client, alloc, fileInfo, queryOpts, allocationPath, nil
}

func allocationFilePath(task, logicalPath string) (string, error) {
	if logicalPath == "" {
		logicalPath = "/"
	}
	for _, segment := range strings.Split(strings.ReplaceAll(logicalPath, "\\", "/"), "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: parent traversal", errInvalidAllocationFilePath)
		}
	}
	logicalPath = pathpkg.Clean("/" + strings.TrimPrefix(logicalPath, "/"))
	if task == "" {
		return logicalPath, nil
	}
	if task == "." || task == ".." || strings.ContainsAny(task, "/\\\x00") {
		return "", fmt.Errorf("%w: invalid task name", errInvalidAllocationFilePath)
	}
	if logicalPath == "/" {
		return task + "/", nil
	}
	return task + logicalPath, nil
}

func setAllocationFSRouteQuery(r *http.Request, task string, pathParts []string) {
	query := r.URL.Query()
	if task == "" {
		query.Del("task")
	} else {
		query.Set("task", task)
	}
	logicalPath := "/"
	if len(pathParts) > 0 {
		logicalPath += strings.Join(pathParts, "/")
	}
	query.Set("path", logicalPath)
	r.URL.RawQuery = query.Encode()
}

func isAllocationFSRoute(requestPath string) bool {
	parts := strings.Split(strings.TrimPrefix(requestPath, "/ui/allocations/"), "/")
	return len(parts) > 1 && parts[1] == "fs"
}

func filePreviewRange(size int64, mode string) (offset, limit int64) {
	limit = min(size, FilePreviewSize)
	if mode == "tail" {
		offset = max(0, size-limit)
	}
	return offset, limit
}

func isPreviewableImage(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch mediaType {
	case "image/avif", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

var (
	errInvalidAllocationFilePath = errors.New("invalid allocation file path")
	errAllocationFileIsDirectory = errors.New("allocation file path is a directory")
)

func (h *AllocationsHandler) renderFileReadError(w http.ResponseWriter, r *http.Request, allocID, path string, err error) {
	h.handler.logger.Error("allocation file read failed", "allocID", allocID, "path", path, "error", err)
	switch {
	case errors.Is(err, errInvalidAllocationFilePath), errors.Is(err, errAllocationFileIsDirectory):
		http.Error(w, "The requested allocation file path is invalid", http.StatusBadRequest)
	case apiResponseHasStatus(err, http.StatusNotFound):
		h.handler.RenderNotFound(w, r, NotFoundError{Message: "The requested allocation file was not found.", RequestPath: r.URL.Path})
	case apiResponseHasStatus(err, http.StatusForbidden):
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message: "You do not have permission to read the allocation filesystem", Capability: "read-fs", Namespace: GetNamespaceFromRequest(r),
		})
	case h.handler.CheckClusterError(w, r, err):
		return
	default:
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{Message: "Wander could not read the allocation file from the cluster."})
	}
}

// isTextContentType determines if a file is text based on ContentType and filename.
func isTextContentType(contentType, fileName string) bool {
	// Check ContentType first
	if strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "javascript") {
		return true
	}
	if contentType != "" {
		return false
	}

	// Fallback to filename extension check
	textExts := map[string]bool{
		"txt": true, "log": true, "md": true, "json": true,
		"yaml": true, "yml": true, "toml": true, "xml": true,
		"html": true, "css": true, "js": true, "ts": true,
		"go": true, "py": true, "rb": true, "sh": true,
		"bash": true, "conf": true, "cfg": true, "ini": true,
		"env": true, "out": true, "err": true, "stdout": true,
		"stderr": true, "hcl": true, "tf": true, "nomad": true,
		"pem": true, "crt": true, "key": true,
	}

	// Get extension
	ext := ""
	for i := len(fileName) - 1; i >= 0; i-- {
		if fileName[i] == '.' {
			ext = fileName[i+1:]
			break
		}
	}

	// Check for stdout/stderr patterns
	if strings.Contains(fileName, ".stdout.") || strings.Contains(fileName, ".stderr.") {
		return true
	}

	return textExts[ext]
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
	if perms != nil && !perms.HasCapability(GetNamespaceFromRequest(r), "alloc-lifecycle") {
		h.handler.logger.Warn("restart permission denied", "allocID", allocID)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Permission Denied", "message": "You do not have the 'alloc-lifecycle' capability to restart allocations.", "duration": 5000}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Log the restart request
	h.handler.logger.Info("restart allocation request", "allocID", allocID)

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available for restart operation")
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Service Unavailable", "message": "Unable to connect to the Wander API.", "duration": 5000}}`)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Get ACL token from request
	token := GetTokenFromRequest(r)
	taskName := strings.TrimSpace(r.FormValue("task"))
	restartAll := r.FormValue("all_tasks") == "true"
	if taskName != "" && restartAll {
		http.Error(w, "Choose either a task restart or all tasks, not both", http.StatusBadRequest)
		return
	}

	queryOpts := allocationQueryOptions(r, token)

	// Fetch allocation info to get the allocation object
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for restart", "allocID", allocID, "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Error", "message": "Failed to fetch allocation details.", "duration": 5000}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if restartAll {
		err = client.Allocations().RestartAllTasks(alloc, queryOpts)
	} else {
		err = client.Allocations().Restart(alloc, taskName, queryOpts)
	}
	if err != nil {
		h.handler.logger.Error("failed to restart allocation", "allocID", allocID, "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Restart Failed", "message": "Failed to restart allocation.", "duration": 5000}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("allocation restarted successfully", "allocID", allocID, "task", taskName, "allTasks", restartAll)

	// Send HX-Trigger header to show toast notification
	restartTarget := "running tasks in allocation " + shortIdentifier(allocID)
	if taskName != "" {
		restartTarget = "task " + taskName
	} else if restartAll {
		restartTarget = "all tasks in allocation " + shortIdentifier(allocID)
	}
	w.Header().Set("HX-Trigger", `{"showToast": {"variant": "success", "title": "Restart Requested", "message": "Restarted `+restartTarget+`.", "duration": 5000}}`)
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
	if perms != nil && !perms.HasCapability(GetNamespaceFromRequest(r), "alloc-lifecycle") {
		h.handler.logger.Warn("stop permission denied", "allocID", allocID)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Permission Denied", "message": "You do not have the 'alloc-lifecycle' capability to stop allocations.", "duration": 5000}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Log the stop request
	h.handler.logger.Info("stop allocation request", "allocID", allocID)

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available for stop operation")
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Service Unavailable", "message": "Unable to connect to the Wander API.", "duration": 5000}}`)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocation info first to get the allocation object
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for stop", "allocID", allocID, "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"variant": "error", "title": "Allocation Not Found", "message": "Unable to find allocation `+shortIdentifier(allocID)+`", "duration": 5000}}`)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Call Allocations().Stop() to stop the allocation
	stopResp, err := client.Allocations().Stop(alloc, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to stop allocation", "allocID", allocID, "error", err)
		setToast(w, "The cluster rejected the allocation stop request", "error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Log success with eval ID
	h.handler.logger.Info("allocation stopped successfully", "allocID", allocID, "evalID", stopResp.EvalID)

	// Send HX-Trigger header to show toast notification with eval ID
	w.Header().Set("HX-Trigger", `{"showToast": {"variant": "success", "title": "Allocation Stopped", "message": "The allocation `+shortIdentifier(allocID)+` has been stopped. Evaluation ID: `+shortIdentifier(stopResp.EvalID)+`", "duration": 5000}}`)
	// Trigger a page refresh after a short delay to show updated status
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
