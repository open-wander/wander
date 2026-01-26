package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

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

	http.NotFound(w, r)
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

	// Get mock client data
	allClients := h.getClientsList()

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

	// Mock client detail data
	clientDetail := h.getClientDetail(clientID)

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
func (h *ClientsHandler) getClientsList() []clientspartials.ClientSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty client list")
		return []clientspartials.ClientSummary{}
	}

	// Fetch nodes from the API
	nodes, _, err := client.Nodes().List(nil)
	if err != nil {
		h.handler.logger.Error("failed to fetch nodes from Nomad API", "error", err)
		return []clientspartials.ClientSummary{}
	}

	// Convert API response to our summary type
	result := make([]clientspartials.ClientSummary, 0, len(nodes))
	for _, node := range nodes {
		// Determine status
		status := node.Status
		if node.SchedulingEligibility == "ineligible" {
			status = "ineligible"
		}
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

// getClientDetail returns mock client detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *ClientsHandler) getClientDetail(clientID string) clientspartials.ClientDetail {
	now := time.Now()

	// Generate mock CPU history data (last 24 hours, every 30 minutes)
	cpuHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	memoryHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	for i := 47; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * 30 * time.Minute)
		// Simulate varying CPU usage between 20-50%
		cpuValue := 25.0 + 15.0*float64((i+7)%10)/10.0 + 5.0*float64(i%5)
		if cpuValue > 50 {
			cpuValue = 50
		}
		cpuHistory = append(cpuHistory, clientspartials.ResourceDataPoint{
			Time:  t,
			Value: cpuValue,
		})
		// Simulate varying memory usage between 40-60%
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
		ID:         clientID,
		Name:       "client-node-1",
		Status:     "ready",
		Datacenter: "dc1",
		Address:    "192.168.1.100:4646",
		Version:    "1.7.2",
		Uptime:     "5d 12h 30m",
		NodeClass:  "compute",
		Drain:      false,
		Draining:   false,
		Eligible:   true,
		Resources: clientspartials.ClientResources{
			CPUUsed:   2400,
			CPUTotal:  8000,
			MemUsed:   8589934592,   // 8 GB in bytes
			MemTotal:  17179869184,  // 16 GB in bytes
			DiskUsed:  53687091200,  // 50 GB in bytes
			DiskTotal: 107374182400, // 100 GB in bytes
		},
		CPUHistory:    cpuHistory,
		MemoryHistory: memoryHistory,
		Allocations: []clientspartials.AllocationSummary{
			{
				ID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				JobID:     "web-frontend",
				TaskGroup: "web",
				Status:    "running",
				CPUUsed:   500,
				MemUsed:   1073741824, // 1 GB
			},
			{
				ID:        "b2c3d4e5-f6g7-8901-bcde-f12345678901",
				JobID:     "api-gateway",
				TaskGroup: "api",
				Status:    "running",
				CPUUsed:   800,
				MemUsed:   2147483648, // 2 GB
			},
			{
				ID:        "c3d4e5f6-g7h8-9012-cdef-123456789012",
				JobID:     "cache-service",
				TaskGroup: "cache",
				Status:    "running",
				CPUUsed:   400,
				MemUsed:   1610612736, // 1.5 GB
			},
			{
				ID:        "d4e5f6g7-h8i9-0123-def1-234567890123",
				JobID:     "data-processor",
				TaskGroup: "processor",
				Status:    "running",
				CPUUsed:   600,
				MemUsed:   2684354560, // 2.5 GB
			},
			{
				ID:        "e5f6g7h8-i9j0-1234-ef12-345678901234",
				JobID:     "metrics-collector",
				TaskGroup: "metrics",
				Status:    "running",
				CPUUsed:   100,
				MemUsed:   536870912, // 512 MB
			},
		},
		Attributes: map[string]string{
			"kernel.name":               "linux",
			"kernel.version":            "5.15.0-91-generic",
			"os.name":                   "ubuntu",
			"os.version":                "22.04",
			"cpu.arch":                  "amd64",
			"cpu.numcores":              "8",
			"cpu.totalcompute":          "8000",
			"memory.totalbytes":         "17179869184",
			"unique.hostname":           "client-node-1",
			"unique.network.ip-address": "192.168.1.100",
			"driver.docker":             "1",
			"driver.docker.version":     "24.0.7",
			"driver.raw_exec":           "1",
		},
		Meta: map[string]string{
			"environment":    "production",
			"region":         "us-west-2",
			"zone":           "us-west-2a",
			"instance_type":  "m5.2xlarge",
			"cloud_provider": "aws",
		},
		Drivers: map[string]clientspartials.DriverInfo{
			"docker": {
				Detected: true,
				Healthy:  true,
				Version:  "24.0.7",
			},
			"raw_exec": {
				Detected: true,
				Healthy:  true,
				Version:  "",
			},
			"exec": {
				Detected: true,
				Healthy:  true,
				Version:  "",
			},
			"java": {
				Detected: false,
				Healthy:  false,
				Version:  "",
			},
		},
		Events: []clientspartials.ClientEvent{
			{
				Time:    now.Add(-5 * time.Minute),
				Type:    "Allocation placed",
				Message: "Allocation e5f6g7h8 placed on node",
			},
			{
				Time:    now.Add(-2 * time.Hour),
				Type:    "Allocation placed",
				Message: "Allocation d4e5f6g7 placed on node",
			},
			{
				Time:    now.Add(-1 * 24 * time.Hour),
				Type:    "Node eligibility changed",
				Message: "Node eligibility set to eligible",
			},
			{
				Time:    now.Add(-3 * 24 * time.Hour),
				Type:    "Node drain complete",
				Message: "Drain completed successfully",
			},
			{
				Time:    now.Add(-3*24*time.Hour - 30*time.Minute),
				Type:    "Node drain started",
				Message: "Drain initiated for maintenance",
			},
			{
				Time:    now.Add(-5 * 24 * time.Hour),
				Type:    "Node registered",
				Message: "Node successfully registered with cluster",
			},
		},
	}
}
