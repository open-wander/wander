package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/clients"
	"github.com/open-wander/wander/ui-htmx/templates/partials/clientspartials"
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
	if perms != nil && !perms.HasCapability("", "node:read") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view client nodes",
			Capability: "node:read",
			Suggestions: []string{
				"Contact your administrator to request the 'node:read' capability",
				"This capability allows viewing Nomad client node information",
			},
		})
		return
	}

	// Parse filter query parameters
	filters := clientspartials.ClientFilters{
		Status:     r.URL.Query().Get("status"),
		Datacenter: r.URL.Query().Get("datacenter"),
		NodeClass:  r.URL.Query().Get("nodeclass"),
		Search:     r.URL.Query().Get("search"),
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get client data from Nomad API
	allClients := h.getClientsList(token)

	// Extract filter options from all clients (before filtering)
	filterOptions := h.extractFilterOptions(allClients)

	// Apply filters
	filteredClients := h.applyFilters(allClients, filters)

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Clients - Wander",
		"View and manage Nomad client nodes",
		"/ui/clients",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Clients", URL: ""},
		},
	)

	// Render the clients list page
	err := Render(w, r,
		clients.List(layoutProps, filteredClients, filters, filterOptions),      // Full page with layout
		clientspartials.ListContent(filteredClients, filters, filterOptions), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render clients list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// extractFilterOptions extracts unique datacenters and node classes from the client list.
func (h *ClientsHandler) extractFilterOptions(clientsList []clientspartials.ClientSummary) clientspartials.ClientFilterOptions {
	dcSet := make(map[string]struct{})
	ncSet := make(map[string]struct{})

	for _, client := range clientsList {
		if client.Datacenter != "" {
			dcSet[client.Datacenter] = struct{}{}
		}
		if client.NodeClass != "" {
			ncSet[client.NodeClass] = struct{}{}
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

	return clientspartials.ClientFilterOptions{
		Datacenters: datacenters,
		NodeClasses: nodeClasses,
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

		// Search filter (matches name, address, datacenter, or ID)
		if filters.Search != "" {
			searchLower := strings.ToLower(filters.Search)
			if !strings.Contains(strings.ToLower(client.Name), searchLower) &&
				!strings.Contains(strings.ToLower(client.Address), searchLower) &&
				!strings.Contains(strings.ToLower(client.Datacenter), searchLower) &&
				!strings.Contains(strings.ToLower(client.ID), searchLower) {
				continue
			}
		}

		filtered = append(filtered, client)
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
	if perms != nil && !perms.HasCapability("", "node:read") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view client node details",
			Capability: "node:read",
			Suggestions: []string{
				"Contact your administrator to request the 'node:read' capability",
				"This capability allows viewing Nomad client node information",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get client detail data from Nomad API
	clientDetail, found := h.getClientDetail(clientID, token)
	if !found {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
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
	err := Render(w, r,
		clients.Detail(layoutProps, clientDetail),                // Full page with layout
		clientspartials.DetailContent(clientDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render client detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
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

	deadline := r.FormValue("deadline")
	ignoreSystem := r.FormValue("ignore_system") == "true"

	// Log the drain request
	h.handler.logger.Info("drain node request",
		"clientID", clientID,
		"deadline", deadline,
		"ignoreSystem", ignoreSystem,
	)

	// TODO: Implement actual Nomad API call to drain node
	// For now, just log and return success

	// Return HX-Redirect header to refresh the page
	w.Header().Set("HX-Redirect", "/ui/clients/"+clientID)
	w.WriteHeader(http.StatusOK)
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

	// Log the cancel drain request
	h.handler.logger.Info("cancel drain request", "clientID", clientID)

	// TODO: Implement actual Nomad API call to cancel drain
	// For now, just log and return success

	// Return HX-Redirect header to refresh the page
	w.Header().Set("HX-Redirect", "/ui/clients/"+clientID)
	w.WriteHeader(http.StatusOK)
}

// getClientsList fetches client nodes from the Nomad API.
// Falls back to empty list if API is unavailable.
func (h *ClientsHandler) getClientsList(token string) []clientspartials.ClientSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty client list")
		return []clientspartials.ClientSummary{}
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching nodes from API", "hasToken", token != "")

	// Fetch nodes from the API
	nodes, _, err := client.Nodes().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch nodes from Nomad API", "error", err)
		return []clientspartials.ClientSummary{}
	}

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

		// Get resource info
		var cpuTotal, memTotal, diskTotal int64
		if node.NodeResources != nil {
			if node.NodeResources.Cpu.CpuShares > 0 {
				cpuTotal = node.NodeResources.Cpu.CpuShares
			}
			if node.NodeResources.Memory.MemoryMB > 0 {
				memTotal = node.NodeResources.Memory.MemoryMB * 1024 * 1024
			}
			if node.NodeResources.Disk.DiskMB > 0 {
				diskTotal = node.NodeResources.Disk.DiskMB * 1024 * 1024
			}
		}

		// Get reserved/used resources
		var cpuUsed, memUsed, diskUsed int64
		if node.ReservedResources != nil {
			if node.ReservedResources.Cpu.CpuShares > 0 {
				cpuUsed = int64(node.ReservedResources.Cpu.CpuShares)
			}
			if node.ReservedResources.Memory.MemoryMB > 0 {
				memUsed = int64(node.ReservedResources.Memory.MemoryMB) * 1024 * 1024
			}
			if node.ReservedResources.Disk.DiskMB > 0 {
				diskUsed = int64(node.ReservedResources.Disk.DiskMB) * 1024 * 1024
			}
		}

		result = append(result, clientspartials.ClientSummary{
			ID:          node.ID,
			Name:        node.Name,
			Address:     node.Address,
			Status:      status,
			Datacenter:  node.Datacenter,
			NodeClass:   node.NodeClass,
			Allocations: 0, // Would need separate API call to get allocation count
			CPUUsed:     int(cpuUsed),
			CPUTotal:    int(cpuTotal),
			MemUsed:     memUsed,
			MemTotal:    memTotal,
			DiskUsed:    diskUsed,
			DiskTotal:   diskTotal,
		})
	}

	return result
}

// getClientDetail fetches client node detail from the Nomad API.
// Returns the client detail and a boolean indicating if the node was found.
func (h *ClientsHandler) getClientDetail(clientID string, token string) (clientspartials.ClientDetail, bool) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty client detail")
		return clientspartials.ClientDetail{ID: clientID}, false
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch node from the API
	node, _, err := client.Nodes().Info(clientID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch node from Nomad API", "clientID", clientID, "error", err)
		return clientspartials.ClientDetail{ID: clientID}, false
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
		memTotal = node.NodeResources.Memory.MemoryMB * 1024 * 1024
		diskTotal = node.NodeResources.Disk.DiskMB * 1024 * 1024
	}

	// Get reserved/used resources
	var cpuUsed, memUsed, diskUsed int64
	if node.ReservedResources != nil {
		cpuUsed = int64(node.ReservedResources.Cpu.CpuShares)
		memUsed = int64(node.ReservedResources.Memory.MemoryMB) * 1024 * 1024
		diskUsed = int64(node.ReservedResources.Disk.DiskMB) * 1024 * 1024
	}

	// Convert drivers
	drivers := make(map[string]clientspartials.DriverInfo)
	if node.Drivers != nil {
		for name, driver := range node.Drivers {
			version := ""
			// Extract version from attributes if available
			if driver.Attributes != nil {
				if v, ok := driver.Attributes["driver.version"]; ok {
					version = v
				}
			}
			drivers[name] = clientspartials.DriverInfo{
				Detected: driver.Detected,
				Healthy:  driver.Healthy,
				Version:  version,
			}
		}
	}

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

	// Fetch allocations for this node
	allocations := h.getNodeAllocations(clientID, token)

	// Fetch real-time stats from the node
	stats := h.getNodeStats(clientID, token)

	// Generate mock resource history (TODO: implement real metrics from Nomad)
	now := time.Now()
	cpuHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	memoryHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	for i := 47; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * 30 * time.Minute)
		// Mock data - in the future, this would come from metrics API
		cpuValue := 25.0 + 15.0*float64((i+7)%10)/10.0 + 5.0*float64(i%5)
		if cpuValue > 50 {
			cpuValue = 50
		}
		cpuHistory = append(cpuHistory, clientspartials.ResourceDataPoint{
			Time:  t,
			Value: cpuValue,
		})
		memValue := 45.0 + 10.0*float64((i+3)%8)/8.0 + 5.0*float64(i%4)
		if memValue > 60 {
			memValue = 60
		}
		memoryHistory = append(memoryHistory, clientspartials.ResourceDataPoint{
			Time:  t,
			Value: memValue,
		})
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
			CPUUsed:   int(cpuUsed),
			CPUTotal:  int(cpuTotal),
			MemUsed:   memUsed,
			MemTotal:  memTotal,
			DiskUsed:  diskUsed,
			DiskTotal: diskTotal,
		},
		Stats:         stats,
		CPUHistory:    cpuHistory,
		MemoryHistory: memoryHistory,
		Allocations:   allocations,
		Attributes:    node.Attributes,
		Meta:          node.Meta,
		Drivers:       drivers,
		Events:        events,
	}, true
}

// getNodeAllocations fetches all allocations running on a specific node.
func (h *ClientsHandler) getNodeAllocations(nodeID string, token string) []clientspartials.AllocationSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty allocations list")
		return []clientspartials.AllocationSummary{}
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocations for this specific node using the more efficient Nodes API
	allocs, _, err := client.Nodes().Allocations(nodeID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch node allocations from Nomad API", "nodeID", nodeID, "error", err)
		return []clientspartials.AllocationSummary{}
	}

	// Convert to summary format
	result := make([]clientspartials.AllocationSummary, 0, len(allocs))
	for _, alloc := range allocs {
		// Get resource info from allocation - sum across all tasks
		cpuUsed := 0
		var memUsed int64
		if alloc.AllocatedResources != nil && alloc.AllocatedResources.Tasks != nil {
			for _, taskRes := range alloc.AllocatedResources.Tasks {
				if taskRes != nil {
					cpuUsed += int(taskRes.Cpu.CpuShares)
					memUsed += int64(taskRes.Memory.MemoryMB) * 1024 * 1024
				}
			}
		}

		result = append(result, clientspartials.AllocationSummary{
			ID:        alloc.ID,
			JobID:     alloc.JobID,
			TaskGroup: alloc.TaskGroup,
			Status:    alloc.ClientStatus,
			CPUUsed:   cpuUsed,
			MemUsed:   memUsed,
		})
	}

	return result
}

// getNodeStats fetches real-time resource stats for a specific node.
// Returns nil if stats are unavailable (e.g., node is down or unreachable).
func (h *ClientsHandler) getNodeStats(nodeID string, token string) *clientspartials.ClientStats {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, cannot fetch node stats")
		return nil
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch stats from the API
	stats, err := client.Nodes().Stats(nodeID, queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch node stats from Nomad API", "nodeID", nodeID, "error", err)
		return nil
	}

	// Convert API stats to our template type
	result := &clientspartials.ClientStats{
		Uptime: stats.Uptime,
	}

	// Process memory stats
	if stats.Memory != nil {
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
			result.CPUUtilization = totalUtilization / float64(len(result.CPUCores))
		}
	}

	// Process disk stats
	if len(stats.DiskStats) > 0 {
		result.DiskStats = make([]clientspartials.DiskStat, 0, len(stats.DiskStats))
		for _, disk := range stats.DiskStats {
			if disk != nil {
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
	}

	return result
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
