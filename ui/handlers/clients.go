package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/clients"
	"github.com/open-wander/wander/ui/templates/partials/clientspartials"
)

// ClientsHandler handles requests for the clients list page.
type ClientsHandler struct {
	handler *Handler
}

// NewClientsHandler creates a new clients handler.
func NewClientsHandler(h *Handler) *ClientsHandler {
	return &ClientsHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the clients endpoints.
// It renders the clients list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *ClientsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/clients exactly (list page)
	if path == "/ui/clients" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle drain cancel endpoint: /ui/clients/{id}/drain/cancel
	if strings.HasSuffix(path, "/drain/cancel") && strings.HasPrefix(path, "/ui/clients/") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.CancelDrain(w, r)
		return
	}

	// Handle drain endpoint: /ui/clients/{id}/drain
	if strings.HasSuffix(path, "/drain") && strings.HasPrefix(path, "/ui/clients/") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Drain(w, r)
		return
	}

	// Handle dynamic metadata endpoint: /ui/clients/{id}/metadata
	if strings.HasSuffix(path, "/metadata") && strings.HasPrefix(path, "/ui/clients/") {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.UpdateMetadata(w, r)
		return
	}

	// Handle client agent monitor routes before the general detail route.
	if isClientMonitorRoute(path) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handler.serveAgentMonitor(w, r, "/ui/clients/", "The client monitor page you requested could not be found.", h.clientMonitorPage, h.streamClientMonitor)
		return
	}

	// Handle /ui/clients/{id} (detail page)
	if strings.HasPrefix(path, "/ui/clients/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Detail(w, r)
		return
	}

	h.handler.RenderNotFound(w, r, NotFoundError{
		Message:     "The client page you requested could not be found.",
		RequestPath: path,
		Suggestions: []string{
			`Go to <a href="/ui/clients" class="underline">Clients List</a>`,
			`Return to <a href="/ui/" class="underline">Home</a>`,
		},
	})
}

// List handles the clients list page
func (h *ClientsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("clients list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read-nodes capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadNode() {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view client nodes",
			Capability: "node:read",
			Suggestions: []string{
				"Contact your administrator to request the 'node:read' capability",
				"This capability allows viewing Wander client node information",
			},
		})
		return
	}

	query := r.URL.Query()
	view := parseListViewOptions(query, "name", "asc")
	if !validClientSortColumn(view.sort) {
		view.sort = "name"
	}
	if view.dir != "asc" && view.dir != "desc" {
		view.dir = "asc"
	}

	filters := clientspartials.ClientFilters{
		Namespace:  GetNamespaceFromRequest(r),
		Region:     GetRegionFromRequest(r),
		Status:     query.Get("status"),
		Datacenter: query.Get("datacenter"),
		NodePool:   query.Get("nodepool"),
		NodeClass:  query.Get("nodeclass"),
		Version:    query.Get("version"),
		Volume:     query.Get("volume"),
		Search:     view.search,
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get client data from Nomad API
	allClients, err := h.getClientsList(r, token)
	if err != nil {
		h.renderClientListError(w, r, err)
		return
	}

	// Extract filter options from all clients (before filtering)
	filterOptions := h.extractFilterOptions(allClients)

	// Apply filters
	filteredClients := h.applyFilters(allClients, filters)
	h.sortClients(filteredClients, view.sort, view.dir)

	total := len(filteredClients)
	totalPages := int(math.Ceil(float64(total) / float64(view.perPage)))
	if totalPages > 0 && view.page > totalPages {
		view.page = totalPages
	}
	paginatedClients := paginateClients(filteredClients, view.page, view.perPage)
	props := clientspartials.ListProps{
		Clients:       paginatedClients,
		FilterOptions: filterOptions,
		Filters:       filters,
		Pagination: clientspartials.PaginationMeta{
			Page: view.page, PerPage: view.perPage, Total: total, TotalPages: totalPages,
		},
		SortColumn: view.sort,
		SortDir:    view.dir,
	}
	if IsHTMXFromRequest(r) && r.Header.Get("HX-Target") == "clients-table-container" {
		if err := RenderPartial(w, r, clientspartials.ClientsTable(props)); err != nil {
			h.handler.logger.Error("failed to render clients table", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Clients - Wander",
		"View and manage Wander client nodes",
		"/ui/clients",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Clients", URL: ""},
		},
	)

	// Render the clients list page
	err = Render(w, r,
		clients.List(layoutProps, props),
		clientspartials.ListContent(props),
	)

	if err != nil {
		h.handler.logger.Error("failed to render clients list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func paginateClients(clientsList []clientspartials.ClientSummary, page, perPage int) []clientspartials.ClientSummary {
	start := (page - 1) * perPage
	if start > len(clientsList) {
		start = len(clientsList)
	}
	end := start + perPage
	if end > len(clientsList) {
		end = len(clientsList)
	}
	return clientsList[start:end]
}

// extractFilterOptions extracts unique datacenters and node classes from the client list.
func (h *ClientsHandler) extractFilterOptions(clientsList []clientspartials.ClientSummary) clientspartials.ClientFilterOptions {
	dcSet := make(map[string]struct{})
	ncSet := make(map[string]struct{})
	npSet := make(map[string]struct{})
	versionSet := make(map[string]struct{})
	volumeSet := make(map[string]struct{})

	for _, client := range clientsList {
		if client.Datacenter != "" {
			dcSet[client.Datacenter] = struct{}{}
		}
		if client.NodeClass != "" {
			ncSet[client.NodeClass] = struct{}{}
		}
		if client.NodePool != "" {
			npSet[client.NodePool] = struct{}{}
		}
		if client.Version != "" {
			versionSet[client.Version] = struct{}{}
		}
		for _, volume := range client.Volumes {
			volumeSet[volume] = struct{}{}
		}
	}

	// Convert sets to sorted slices
	datacenters := make([]string, 0, len(dcSet))
	for dc := range dcSet {
		datacenters = append(datacenters, dc)
	}
	sort.Strings(datacenters)

	nodeClasses := make([]string, 0, len(ncSet))
	for nc := range ncSet {
		nodeClasses = append(nodeClasses, nc)
	}
	sort.Strings(nodeClasses)
	nodePools := sortedSetValues(npSet)
	versions := sortedSetValues(versionSet)
	volumes := sortedSetValues(volumeSet)

	return clientspartials.ClientFilterOptions{
		Datacenters: datacenters,
		NodeClasses: nodeClasses,
		NodePools:   nodePools,
		Versions:    versions,
		Volumes:     volumes,
	}
}

// applyFilters filters the client list based on the provided filters.
func (h *ClientsHandler) applyFilters(clientsList []clientspartials.ClientSummary, filters clientspartials.ClientFilters) []clientspartials.ClientSummary {
	filtered := make([]clientspartials.ClientSummary, 0, len(clientsList))

	for _, client := range clientsList {
		// Status filter
		if filters.Status != "" && client.Status != filters.Status {
			continue
		}

		// Datacenter filter
		if filters.Datacenter != "" && client.Datacenter != filters.Datacenter {
			continue
		}

		// Node class filter
		if filters.NodeClass != "" && client.NodeClass != filters.NodeClass {
			continue
		}
		if filters.NodePool != "" && client.NodePool != filters.NodePool {
			continue
		}
		if filters.Version != "" && client.Version != filters.Version {
			continue
		}
		if filters.Volume != "" && !clientContainsString(client.Volumes, filters.Volume) {
			continue
		}

		// Search filter (matches name, address, datacenter, or ID)
		if filters.Search != "" {
			searchLower := strings.ToLower(filters.Search)
			if !strings.Contains(strings.ToLower(client.Name), searchLower) &&
				!strings.Contains(strings.ToLower(client.Address), searchLower) &&
				!strings.Contains(strings.ToLower(client.Datacenter), searchLower) &&
				!strings.Contains(strings.ToLower(client.ID), searchLower) &&
				!strings.Contains(strings.ToLower(client.NodePool), searchLower) &&
				!strings.Contains(strings.ToLower(client.Version), searchLower) &&
				!strings.Contains(strings.ToLower(strings.Join(client.Volumes, " ")), searchLower) {
				continue
			}
		}

		filtered = append(filtered, client)
	}

	return filtered
}

func sortedSetValues(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func clientContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func extractClientAllocationFilterOptions(allocations []clientspartials.AllocationSummary) clientspartials.ClientAllocationFilterOptions {
	statuses := make(map[string]struct{})
	namespaces := make(map[string]struct{})
	jobs := make(map[string]struct{})
	taskGroups := make(map[string]struct{})
	for _, allocation := range allocations {
		if allocation.Status != "" {
			statuses[allocation.Status] = struct{}{}
		}
		if allocation.Namespace != "" {
			namespaces[allocation.Namespace] = struct{}{}
		}
		if allocation.JobID != "" {
			jobs[allocation.JobID] = struct{}{}
		}
		if allocation.TaskGroup != "" {
			taskGroups[allocation.TaskGroup] = struct{}{}
		}
	}
	return clientspartials.ClientAllocationFilterOptions{
		Statuses: sortedSetValues(statuses), Namespaces: sortedSetValues(namespaces),
		Jobs: sortedSetValues(jobs), TaskGroups: sortedSetValues(taskGroups),
	}
}

func filterClientAllocations(allocations []clientspartials.AllocationSummary, filters clientspartials.ClientAllocationFilters) []clientspartials.AllocationSummary {
	filtered := make([]clientspartials.AllocationSummary, 0, len(allocations))
	for _, allocation := range allocations {
		if filters.Status != "" && allocation.Status != filters.Status {
			continue
		}
		if filters.Namespace != "" && allocation.Namespace != filters.Namespace {
			continue
		}
		if filters.Job != "" && allocation.JobID != filters.Job {
			continue
		}
		if filters.TaskGroup != "" && allocation.TaskGroup != filters.TaskGroup {
			continue
		}
		filtered = append(filtered, allocation)
	}
	return filtered
}

// Detail handles the client detail page
func (h *ClientsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	clientID := strings.TrimPrefix(r.URL.Path, "/ui/clients/")
	clientID = strings.TrimSuffix(clientID, "/")

	// Log the request
	h.handler.logger.Debug("client detail request", "clientID", clientID, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read-nodes capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadNode() {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view client node details",
			Capability: "node:read",
			Suggestions: []string{
				"Contact your administrator to request the 'node:read' capability",
				"This capability allows viewing Wander client node information",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get client detail data from Nomad API
	clientDetail, found, err := h.getClientDetail(r, clientID, token)
	if err != nil {
		if apiResponseHasStatus(err, http.StatusForbidden) {
			h.handler.RenderForbidden(w, r, ForbiddenError{Message: "You do not have permission to view client node details", Capability: "node:read"})
			return
		}
		h.handler.logger.Error("failed to read client node detail", "clientID", clientID, "error", err)
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{Message: "Wander could not read the client node from the cluster."})
		return
	}
	if !found {
		h.handler.RenderNotFound(w, r, NotFoundError{Message: "The requested client node was not found.", RequestPath: r.URL.Path})
		return
	}
	clientDetail.ActiveTab = r.URL.Query().Get("tab")
	if clientDetail.ActiveTab != "allocations" && clientDetail.ActiveTab != "events" {
		clientDetail.ActiveTab = "overview"
	}
	clientDetail.AllocationFilters = clientspartials.ClientAllocationFilters{
		Status:    r.URL.Query().Get("allocation_status"),
		Namespace: r.URL.Query().Get("allocation_namespace"),
		Job:       r.URL.Query().Get("allocation_job"),
		TaskGroup: r.URL.Query().Get("allocation_task_group"),
	}
	clientDetail.AllocationFilterOptions = extractClientAllocationFilterOptions(clientDetail.Allocations)
	clientDetail.Allocations = filterClientAllocations(clientDetail.Allocations, clientDetail.AllocationFilters)
	perms = GetPermissionsFromRequest(r)
	clientDetail.CanWriteNode = perms == nil || perms.HasCapability("", "node:write")

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		clientDetail.Name+" - Client Detail - Wander",
		"Client detail for "+clientDetail.Name,
		"/ui/clients/"+clientID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Clients", URL: "/ui/clients"},
			{Label: clientDetail.Name, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err = Render(w, r,
		clients.Detail(layoutProps, clientDetail),   // Full page with layout
		clientspartials.DetailContent(clientDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render client detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// UpdateMetadata creates, updates, or unsets one dynamic metadata key.
func (h *ClientsHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/clients/"), "/metadata")
	if clientID == "" || strings.Contains(clientID, "/") {
		http.Error(w, "invalid client ID", http.StatusBadRequest)
		return
	}

	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("", "node:write") {
		http.Error(w, "Permission denied: you do not have the 'node:write' capability", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid metadata form", http.StatusBadRequest)
		return
	}

	key := strings.TrimSpace(r.FormValue("key"))
	operation := r.FormValue("operation")
	if key == "" {
		http.Error(w, "metadata key is required", http.StatusBadRequest)
		return
	}
	if operation != "set" && operation != "unset" {
		http.Error(w, "metadata operation must be set or unset", http.StatusBadRequest)
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander API client is unavailable", http.StatusServiceUnavailable)
		return
	}
	updates := map[string]*string{key: nil}
	if operation == "set" {
		value := r.FormValue("value")
		updates[key] = &value
	}
	_, err := client.Nodes().Meta().Apply(&api.NodeMetaApplyRequest{NodeID: clientID, Meta: updates}, &api.QueryOptions{
		AuthToken: GetTokenFromRequest(r),
		Region:    GetRegionFromRequest(r),
	})
	if err != nil {
		h.handler.logger.Error("failed to update client metadata", "clientID", clientID, "operation", operation, "key", key, "error", err)
		switch {
		case apiResponseHasStatus(err, http.StatusBadRequest):
			http.Error(w, "The metadata update was invalid", http.StatusBadRequest)
		case apiResponseHasStatus(err, http.StatusForbidden):
			http.Error(w, "Permission denied: node:write is required", http.StatusForbidden)
		case apiResponseHasStatus(err, http.StatusNotFound):
			http.Error(w, "The client is unavailable for metadata updates", http.StatusNotFound)
		default:
			http.Error(w, "Failed to update client metadata", http.StatusBadGateway)
		}
		return
	}

	http.Redirect(w, r, scopedUIURL("/ui/clients/"+url.PathEscape(clientID), GetNamespaceFromRequest(r), GetRegionFromRequest(r)), http.StatusSeeOther)
}

// Drain handles the drain node request
func (h *ClientsHandler) Drain(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	clientID := strings.TrimPrefix(r.URL.Path, "/ui/clients/")
	clientID = strings.TrimSuffix(clientID, "/drain")

	// Check permissions - need node-write capability to drain nodes
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("", "node:write") {
		h.handler.logger.Warn("drain permission denied", "clientID", clientID)
		http.Error(w, "Permission denied: you do not have the 'node:write' capability", http.StatusForbidden)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse drain form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	drainSpec, metadata, err := parseDrainStrategy(r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log the drain request
	h.handler.logger.Info("drain node request",
		"clientID", clientID,
		"deadline", drainSpec.Deadline,
		"ignoreSystem", drainSpec.IgnoreSystemJobs,
	)

	if !h.updateNodeDrain(w, r, clientID, &api.DrainOptions{DrainSpec: drainSpec, Meta: metadata}, "start") {
		return
	}

	redirectToClient(w, r, clientID)
}

// CancelDrain handles the cancel drain request
func (h *ClientsHandler) CancelDrain(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	clientID := strings.TrimPrefix(r.URL.Path, "/ui/clients/")
	clientID = strings.TrimSuffix(clientID, "/drain/cancel")

	// Check permissions - need node-write capability to cancel drain
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("", "node:write") {
		h.handler.logger.Warn("cancel drain permission denied", "clientID", clientID)
		http.Error(w, "Permission denied: you do not have the 'node:write' capability", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	keepIneligible := r.FormValue("keep_ineligible") == "true"
	metadata := make(map[string]string)
	if message := strings.TrimSpace(r.FormValue("cancel_message")); message != "" {
		metadata["cancel_message"] = message
	}

	// Log the cancel drain request
	h.handler.logger.Info("cancel drain request", "clientID", clientID)

	if !h.updateNodeDrain(w, r, clientID, &api.DrainOptions{MarkEligible: !keepIneligible, Meta: metadata}, "cancel") {
		return
	}

	redirectToClient(w, r, clientID)
}

func redirectToClient(w http.ResponseWriter, r *http.Request, clientID string) {
	target := scopedUIURL("/ui/clients/"+url.PathEscape(clientID), GetNamespaceFromRequest(r), GetRegionFromRequest(r))
	if IsHTMXFromRequest(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func parseDrainStrategy(form url.Values) (*api.DrainSpec, map[string]string, error) {
	mode := form.Get("deadline_mode")
	deadline := strings.TrimSpace(form.Get("deadline"))
	if mode == "" {
		if deadline != "" {
			mode = "custom"
		} else {
			mode = "default"
		}
	}

	var duration time.Duration
	switch mode {
	case "default":
		duration = time.Hour
	case "custom":
		parsed, err := time.ParseDuration(deadline)
		if err != nil || parsed <= 0 {
			return nil, nil, fmt.Errorf("custom deadline must be a positive duration such as 1h or 30m")
		}
		duration = parsed
	case "none":
		duration = 0
	case "force":
		duration = -time.Second
	default:
		return nil, nil, fmt.Errorf("unknown drain deadline mode %q", mode)
	}

	metadata := make(map[string]string)
	if message := strings.TrimSpace(form.Get("message")); message != "" {
		metadata["message"] = message
	}
	return &api.DrainSpec{Deadline: duration, IgnoreSystemJobs: form.Get("ignore_system") == "true"}, metadata, nil
}

func (h *ClientsHandler) updateNodeDrain(w http.ResponseWriter, r *http.Request, clientID string, opts *api.DrainOptions, action string) bool {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander API client is unavailable", http.StatusServiceUnavailable)
		return false
	}

	writeOpts := &api.WriteOptions{AuthToken: GetTokenFromRequest(r), Region: GetRegionFromRequest(r)}
	if _, err := client.Nodes().UpdateDrainOpts(clientID, opts, writeOpts); err != nil {
		h.handler.logger.Error("failed to update node drain", "action", action, "clientID", clientID, "error", err)
		http.Error(w, "Failed to "+action+" node drain", http.StatusBadGateway)
		return false
	}
	return true
}

// getClientsList fetches client nodes from the Wander API.
func (h *ClientsHandler) getClientsList(r *http.Request, token string) ([]clientspartials.ClientSummary, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, fmt.Errorf("Wander API client is unavailable")
	}

	queryOpts := resourceQueryOptions(token)
	queryOpts.Region = GetRegionFromRequest(r)

	// Log the request for debugging
	h.handler.logger.Debug("fetching nodes from API", "hasToken", token != "")

	// Fetch nodes from the API
	nodes, _, err := client.Nodes().List(queryOpts)
	if err != nil {
		return nil, err
	}
	allocationOpts := &api.QueryOptions{
		AuthToken: token,
		Namespace: GetNamespaceFromRequest(r),
		Region:    GetRegionFromRequest(r),
		Params:    map[string]string{"resources": "true"},
	}
	allocations, _, allocationsErr := client.Allocations().List(allocationOpts)
	if allocationsErr != nil {
		h.handler.logger.Warn("failed to fetch allocation resources for clients", "error", allocationsErr)
	}
	allocationsByNode := summarizeNodeAllocations(allocations)

	// Convert API response to our summary type
	result := make([]clientspartials.ClientSummary, 0, len(nodes))
	for _, node := range nodes {
		// Determine composite status
		// Prioritize draining status, then check eligibility, then use base status
		status := node.Status // "ready", "down", "initializing", "disconnected"

		// Override with ineligible if node is not eligible for scheduling
		if node.SchedulingEligibility == "ineligible" && !node.Drain {
			status = "ineligible"
		}

		// Override with draining if node is actively draining
		if node.Drain {
			status = "draining"
		}

		cpuTotal, memTotal, diskTotal, capacityAvailable := nodeCapacity(node)
		allocated := allocationsByNode[node.ID]
		volumes := make([]string, 0, len(node.HostVolumes))
		for name := range node.HostVolumes {
			volumes = append(volumes, name)
		}
		sort.Strings(volumes)

		result = append(result, clientspartials.ClientSummary{
			ID:                   node.ID,
			Name:                 node.Name,
			Address:              node.Address,
			Status:               status,
			Datacenter:           node.Datacenter,
			NodePool:             node.NodePool,
			NodeClass:            node.NodeClass,
			Version:              node.Version,
			Volumes:              volumes,
			DetailURL:            scopedUIURL("/ui/clients/"+url.PathEscape(node.ID), GetNamespaceFromRequest(r), GetRegionFromRequest(r)),
			Allocations:          allocated.Count,
			CPUUsed:              int(allocated.CPU),
			CPUTotal:             int(cpuTotal),
			MemUsed:              allocated.MemoryBytes,
			MemTotal:             memTotal,
			DiskUsed:             allocated.DiskBytes,
			DiskTotal:            diskTotal,
			ResourcesAvailable:   capacityAvailable && allocationsErr == nil,
			AllocationsAvailable: allocationsErr == nil,
		})
	}

	return result, nil
}

func (h *ClientsHandler) renderClientListError(w http.ResponseWriter, r *http.Request, err error) {
	if apiResponseHasStatus(err, http.StatusForbidden) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message: "You do not have permission to list client nodes", Capability: "node:read",
		})
		return
	}
	h.handler.logger.Error("failed to list client nodes", "error", err)
	h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{
		Message: "Wander could not list client nodes from the cluster.",
	})
}

func validClientSortColumn(column string) bool {
	switch column {
	case "name", "address", "status", "nodepool", "nodeclass", "datacenter", "version", "volumes", "allocations":
		return true
	default:
		return false
	}
}

func (h *ClientsHandler) sortClients(clientsList []clientspartials.ClientSummary, column, direction string) {
	sort.SliceStable(clientsList, func(i, j int) bool {
		left, right := clientsList[i], clientsList[j]
		comparison := compareClientSummaries(left, right, column)
		if comparison == 0 {
			comparison = strings.Compare(left.ID, right.ID)
		}
		if direction == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareClientSummaries(left, right clientspartials.ClientSummary, column string) int {
	var leftValue, rightValue string
	switch column {
	case "address":
		leftValue, rightValue = left.Address, right.Address
	case "status":
		leftValue, rightValue = left.Status, right.Status
	case "nodepool":
		leftValue, rightValue = left.NodePool, right.NodePool
	case "nodeclass":
		leftValue, rightValue = left.NodeClass, right.NodeClass
	case "datacenter":
		leftValue, rightValue = left.Datacenter, right.Datacenter
	case "version":
		leftValue, rightValue = left.Version, right.Version
	case "volumes":
		leftValue, rightValue = strings.Join(left.Volumes, "\x00"), strings.Join(right.Volumes, "\x00")
	case "allocations":
		if left.Allocations < right.Allocations {
			return -1
		}
		if left.Allocations > right.Allocations {
			return 1
		}
		return 0
	default:
		leftValue, rightValue = left.Name, right.Name
	}
	return strings.Compare(strings.ToLower(leftValue), strings.ToLower(rightValue))
}

// getClientDetail fetches client node detail from the Nomad API.
// Returns the client detail and a boolean indicating if the node was found.
func (h *ClientsHandler) getClientDetail(r *http.Request, clientID string, token string) (clientspartials.ClientDetail, bool, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty client detail")
		return clientspartials.ClientDetail{ID: clientID}, false, fmt.Errorf("Wander API client is unavailable")
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{Region: GetRegionFromRequest(r)}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch node from the API
	node, _, err := client.Nodes().Info(clientID, queryOpts)
	if err != nil {
		if apiResponseHasStatus(err, http.StatusNotFound) {
			return clientspartials.ClientDetail{ID: clientID}, false, nil
		}
		return clientspartials.ClientDetail{ID: clientID}, false, err
	}

	h.handler.logger.Debug("fetched node from API", "clientID", clientID, "status", node.Status)

	// Determine composite status
	status := node.Status // "ready", "down", "initializing", "disconnected"
	draining := node.Drain
	eligible := node.SchedulingEligibility != "ineligible"

	// Override status based on draining/eligibility
	if node.Drain {
		status = "draining"
	} else if node.SchedulingEligibility == "ineligible" {
		status = "ineligible"
	}

	// Get version from attributes
	version := ""
	if node.Attributes != nil {
		if v, ok := node.Attributes["nomad.version"]; ok {
			version = v
		}
	}

	// Calculate uptime from StatusUpdatedAt
	uptime := ""
	if node.StatusUpdatedAt > 0 {
		statusTime := time.Unix(node.StatusUpdatedAt, 0)
		duration := time.Since(statusTime)
		uptime = formatUptime(duration)
	}

	// Get resource info
	var cpuTotal, memTotal, diskTotal int64
	if node.NodeResources != nil {
		cpuTotal = node.NodeResources.Cpu.CpuShares
		memoryMB := node.NodeResources.Memory.MemoryMB
		diskMB := node.NodeResources.Disk.DiskMB
		if node.ReservedResources != nil {
			cpuTotal = subtractReserved(cpuTotal, node.ReservedResources.Cpu.CpuShares)
			memoryMB = subtractReserved(memoryMB, node.ReservedResources.Memory.MemoryMB)
			diskMB = subtractReserved(diskMB, node.ReservedResources.Disk.DiskMB)
		}
		memTotal = memoryMB * bytesPerMegabyte
		diskTotal = diskMB * bytesPerMegabyte
	}

	// Convert drivers
	drivers := make([]clientspartials.DriverInfo, 0, len(node.Drivers))
	if node.Drivers != nil {
		for name, driver := range node.Drivers {
			version := ""
			// Extract version from attributes if available
			if driver.Attributes != nil {
				if v, ok := driver.Attributes["driver.version"]; ok {
					version = v
				}
			}
			drivers = append(drivers, clientspartials.DriverInfo{
				Name: name, Detected: driver.Detected, Healthy: driver.Healthy,
				Version: version, HealthDescription: driver.HealthDescription,
				Attributes: driver.Attributes,
			})
		}
	}
	sort.Slice(drivers, func(i, j int) bool { return drivers[i].Name < drivers[j].Name })
	hostVolumes := make([]clientspartials.HostVolumeInfo, 0, len(node.HostVolumes))
	for name, volume := range node.HostVolumes {
		if volume == nil {
			continue
		}
		hostVolumes = append(hostVolumes, clientspartials.HostVolumeInfo{Name: name, Path: volume.Path, ReadOnly: volume.ReadOnly})
	}
	sort.Slice(hostVolumes, func(i, j int) bool { return hostVolumes[i].Name < hostVolumes[j].Name })

	// Convert node events
	events := make([]clientspartials.ClientEvent, 0, len(node.Events))
	if node.Events != nil {
		for _, event := range node.Events {
			events = append(events, clientspartials.ClientEvent{
				Time:    event.Timestamp,
				Type:    event.Subsystem,
				Message: event.Message,
			})
		}
	}

	metadata, metadataErr := client.Nodes().Meta().Read(clientID, &api.QueryOptions{
		AuthToken: token,
		Region:    GetRegionFromRequest(r),
	})
	metadataError := ""
	var staticMetadata map[string]string
	var dynamicMetadata map[string]*string
	if metadataErr != nil {
		metadataError = "Dynamic metadata is unavailable because Wander could not contact this client."
		if apiResponseHasStatus(metadataErr, http.StatusForbidden) {
			metadataError = "Dynamic metadata is unavailable because you do not have permission to read it."
		}
	} else if metadata != nil {
		staticMetadata = metadata.Static
		dynamicMetadata = metadata.Dynamic
	}

	// Fetch allocations for this node
	allocations, allocationsErr := h.getNodeAllocations(r, clientID, token)
	allocationError := ""
	if allocationsErr != nil {
		allocationError = "Allocations are unavailable. Wander could not read this client's allocations from the cluster."
		if apiResponseHasStatus(allocationsErr, http.StatusForbidden) {
			allocationError = "Allocations are unavailable because you do not have permission to read them in this namespace."
		}
	}
	var cpuUsed int
	var memUsed, diskUsed int64
	for _, allocation := range allocations {
		if !isActiveAllocation(allocation.Status) {
			continue
		}
		cpuUsed += allocation.CPUUsed
		memUsed += allocation.MemUsed
		diskUsed += allocation.DiskUsed
	}

	// Fetch the current host statistics sample independently from node detail.
	stats, statsErr := h.getNodeStats(r, clientID, token)
	statsError := ""
	if statsErr != nil {
		statsError = "Current host statistics are unavailable because Wander could not contact this client."
		if apiResponseHasStatus(statsErr, http.StatusForbidden) {
			statsError = "Current host statistics are unavailable because you do not have permission to read them."
		}
	}

	return clientspartials.ClientDetail{
		ID:         node.ID,
		Name:       node.Name,
		Status:     status,
		Datacenter: node.Datacenter,
		Address:    node.HTTPAddr,
		Version:    version,
		Uptime:     uptime,
		NodeClass:  node.NodeClass,
		Drain:      node.Drain,
		Draining:   draining,
		Eligible:   eligible,
		Resources: clientspartials.ClientResources{
			CPUUsed:   cpuUsed,
			CPUTotal:  int(cpuTotal),
			MemUsed:   memUsed,
			MemTotal:  memTotal,
			DiskUsed:  diskUsed,
			DiskTotal: diskTotal,
		},
		Stats:                stats,
		StatsAvailable:       statsErr == nil,
		StatsError:           statsError,
		Allocations:          allocations,
		Attributes:           node.Attributes,
		Meta:                 node.Meta,
		StaticMeta:           staticMetadata,
		DynamicMeta:          dynamicMetadata,
		MetadataAvailable:    metadataErr == nil,
		MetadataError:        metadataError,
		Drivers:              drivers,
		HostVolumes:          hostVolumes,
		Events:               events,
		AllocationsAvailable: allocationsErr == nil,
		AllocationError:      allocationError,
		Namespace:            GetNamespaceFromRequest(r),
		Region:               GetRegionFromRequest(r),
	}, true, nil
}

// getNodeAllocations fetches all allocations running on a specific node.
func (h *ClientsHandler) getNodeAllocations(r *http.Request, nodeID string, token string) ([]clientspartials.AllocationSummary, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty allocations list")
		return nil, fmt.Errorf("Wander API client is unavailable")
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{Namespace: GetNamespaceFromRequest(r), Region: GetRegionFromRequest(r)}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocations for this specific node using the more efficient Nodes API
	allocs, _, err := client.Nodes().Allocations(nodeID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch node allocations from Nomad API", "nodeID", nodeID, "error", err)
		return nil, err
	}

	// Convert to summary format
	result := make([]clientspartials.AllocationSummary, 0, len(allocs))
	for _, alloc := range allocs {
		// Get resource info from allocation - sum across all tasks
		cpuUsed := 0
		var memUsed, diskUsed int64
		if alloc.AllocatedResources != nil && alloc.AllocatedResources.Tasks != nil {
			for _, taskRes := range alloc.AllocatedResources.Tasks {
				if taskRes != nil {
					cpuUsed += int(taskRes.Cpu.CpuShares)
					memUsed += int64(taskRes.Memory.MemoryMB) * 1024 * 1024
				}
			}
		}
		if alloc.AllocatedResources != nil {
			diskUsed = alloc.AllocatedResources.Shared.DiskMB * bytesPerMegabyte
		}

		result = append(result, clientspartials.AllocationSummary{
			ID:            alloc.ID,
			JobID:         alloc.JobID,
			Namespace:     alloc.Namespace,
			TaskGroup:     alloc.TaskGroup,
			Status:        alloc.ClientStatus,
			DesiredStatus: alloc.DesiredStatus,
			DetailURL:     scopedUIURL("/ui/allocations/"+url.PathEscape(alloc.ID), GetNamespaceFromRequest(r), GetRegionFromRequest(r)),
			JobURL:        scopedUIURL("/ui/jobs/"+url.PathEscape(alloc.JobID), alloc.Namespace, GetRegionFromRequest(r)),
			CPUUsed:       cpuUsed,
			MemUsed:       memUsed,
			DiskUsed:      diskUsed,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].JobID != result[j].JobID {
			return result[i].JobID < result[j].JobID
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// getNodeStats fetches real-time resource stats for a specific node.
func (h *ClientsHandler) getNodeStats(r *http.Request, nodeID string, token string) (*clientspartials.ClientStats, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, cannot fetch node stats")
		return nil, fmt.Errorf("Wander API client is unavailable")
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{Region: GetRegionFromRequest(r)}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch stats from the API
	stats, err := client.Nodes().Stats(nodeID, queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch node stats from Nomad API", "nodeID", nodeID, "error", err)
		return nil, err
	}

	// Convert API stats to our template type
	result := &clientspartials.ClientStats{
		Uptime: stats.Uptime,
	}

	// Process memory stats
	if stats.Memory != nil {
		result.HasMemory = true
		result.MemoryTotal = stats.Memory.Total
		result.MemoryUsed = stats.Memory.Used
		result.MemoryAvailable = stats.Memory.Available
	}

	// Process CPU stats and calculate average utilization
	if len(stats.CPU) > 0 {
		result.CPUCores = make([]clientspartials.CPUCoreStat, 0, len(stats.CPU))
		totalUtilization := 0.0
		for _, cpu := range stats.CPU {
			if cpu != nil {
				coreStat := clientspartials.CPUCoreStat{
					Core:   cpu.CPU,
					User:   cpu.User,
					System: cpu.System,
					Idle:   cpu.Idle,
				}
				result.CPUCores = append(result.CPUCores, coreStat)
				// Calculate utilization as 100 - idle
				totalUtilization += (100.0 - cpu.Idle)
			}
		}
		// Average across all cores
		if len(result.CPUCores) > 0 {
			result.HasCPU = true
			result.CPUUtilization = totalUtilization / float64(len(result.CPUCores))
		}
		sort.Slice(result.CPUCores, func(i, j int) bool { return result.CPUCores[i].Core < result.CPUCores[j].Core })
	}

	// Process disk stats
	if len(stats.DiskStats) > 0 {
		result.DiskStats = make([]clientspartials.DiskStat, 0, len(stats.DiskStats))
		for _, disk := range stats.DiskStats {
			if disk != nil {
				result.HasDisk = true
				diskStat := clientspartials.DiskStat{
					Device:      disk.Device,
					Mountpoint:  disk.Mountpoint,
					Size:        disk.Size,
					Used:        disk.Used,
					Available:   disk.Available,
					UsedPercent: disk.UsedPercent,
				}
				result.DiskStats = append(result.DiskStats, diskStat)
			}
		}
		sort.Slice(result.DiskStats, func(i, j int) bool {
			if result.DiskStats[i].Mountpoint != result.DiskStats[j].Mountpoint {
				return result.DiskStats[i].Mountpoint < result.DiskStats[j].Mountpoint
			}
			return result.DiskStats[i].Device < result.DiskStats[j].Device
		})
	}

	return result, nil
}

// formatUptime formats a duration into a human-readable uptime string.
func formatUptime(d time.Duration) string {
	if d < 0 {
		return "unknown"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
		}
		return fmt.Sprintf("%dd %dm", days, minutes)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}
