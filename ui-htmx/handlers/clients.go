package handlers

import (
	"net/http"
	"strings"

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

// ServeHTTP handles GET requests to the clients list page.
// It renders the clients list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *ClientsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Handle /ui/clients exactly (list page)
	if path == "/ui/clients" {
		h.List(w, r)
		return
	}

	// Handle /ui/clients/{id} (detail page)
	if strings.HasPrefix(path, "/ui/clients/") {
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the clients list page
func (h *ClientsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("clients list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock client data
	clientsList := h.getClientsList()

	// Render the clients list page
	err := Render(w, r,
		clients.List(clientsList),                       // Full page with layout
		clientspartials.ListContent(clientsList), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render clients list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Detail handles the client detail page
func (h *ClientsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	clientID := strings.TrimPrefix(r.URL.Path, "/ui/clients/")
	clientID = strings.TrimSuffix(clientID, "/")

	// Log the request
	h.handler.logger.Debug("client detail request", "clientID", clientID, "htmx", IsHTMXFromRequest(r))

	// Mock client detail data
	clientDetail := h.getClientDetail(clientID)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		clients.Detail(clientDetail),                       // Full page with layout
		clientspartials.DetailContent(clientDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render client detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getClientsList returns mock client data.
// This will be replaced with actual Nomad API calls in the future.
func (h *ClientsHandler) getClientsList() []clientspartials.ClientSummary {
	return []clientspartials.ClientSummary{
		{
			ID:          "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o",
			Name:        "client-node-1",
			Address:     "192.168.1.100:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "compute",
			Allocations: 5,
			CPUUsed:     2400,
			CPUTotal:    8000,
			MemUsed:     8589934592,   // 8 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    32212254720,  // 30 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "d2e3f4g5-h6i7-8j9k-0l1m-2n3o4p5q6r7s",
			Name:        "client-node-2",
			Address:     "192.168.1.101:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "compute",
			Allocations: 3,
			CPUUsed:     4200,
			CPUTotal:    8000,
			MemUsed:     12884901888,  // 12 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    53687091200,  // 50 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "e3f4g5h6-i7j8-9k0l-1m2n-3o4p5q6r7s8t",
			Name:        "client-node-3",
			Address:     "192.168.2.100:4646",
			Status:      "down",
			Datacenter:  "dc2",
			NodeClass:   "storage",
			Allocations: 0,
			CPUUsed:     0,
			CPUTotal:    8000,
			MemUsed:     0,
			MemTotal:    17179869184, // 16 GB in bytes
			DiskUsed:    0,
			DiskTotal:   214748364800, // 200 GB in bytes
		},
		{
			ID:          "f4g5h6i7-j8k9-0l1m-2n3o-4p5q6r7s8t9u",
			Name:        "client-node-4",
			Address:     "192.168.1.102:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "high-memory",
			Allocations: 8,
			CPUUsed:     6800,
			CPUTotal:    16000,
			MemUsed:     25769803776,  // 24 GB in bytes
			MemTotal:    34359738368,  // 32 GB in bytes
			DiskUsed:    96636764160,  // 90 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "g5h6i7j8-k9l0-1m2n-3o4p-5q6r7s8t9u0v",
			Name:        "client-node-5",
			Address:     "192.168.2.101:4646",
			Status:      "draining",
			Datacenter:  "dc2",
			NodeClass:   "compute",
			Allocations: 2,
			CPUUsed:     1200,
			CPUTotal:    8000,
			MemUsed:     4294967296,  // 4 GB in bytes
			MemTotal:    17179869184, // 16 GB in bytes
			DiskUsed:    21474836480, // 20 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "h6i7j8k9-l0m1-2n3o-4p5q-6r7s8t9u0v1w",
			Name:        "client-node-6",
			Address:     "192.168.2.102:4646",
			Status:      "ready",
			Datacenter:  "dc2",
			NodeClass:   "gpu",
			Allocations: 6,
			CPUUsed:     5400,
			CPUTotal:    8000,
			MemUsed:     10737418240,  // 10 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    80530636800,  // 75 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
	}
}

// getClientDetail returns mock client detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *ClientsHandler) getClientDetail(clientID string) clientspartials.ClientDetail {
	return clientspartials.ClientDetail{
		ID:         clientID,
		Name:       "client-node-1",
		Status:     "ready",
		Datacenter: "dc1",
		Address:    "192.168.1.100:4646",
		Version:    "1.7.2",
		Uptime:     "5d 12h 30m",
		Resources: clientspartials.ClientResources{
			CPUUsed:  2400,
			CPUTotal: 8000,
			MemUsed:  8589934592,  // 8 GB in bytes
			MemTotal: 17179869184, // 16 GB in bytes
			DiskUsed: 53687091200, // 50 GB in bytes
			DiskTotal: 107374182400, // 100 GB in bytes
		},
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
			"kernel.name":           "linux",
			"kernel.version":        "5.15.0-91-generic",
			"os.name":               "ubuntu",
			"os.version":            "22.04",
			"cpu.arch":              "amd64",
			"cpu.numcores":          "8",
			"cpu.totalcompute":      "8000",
			"memory.totalbytes":     "17179869184",
			"unique.hostname":       "client-node-1",
			"unique.network.ip-address": "192.168.1.100",
			"driver.docker":         "1",
			"driver.docker.version": "24.0.7",
			"driver.raw_exec":       "1",
		},
		Meta: map[string]string{
			"environment": "production",
			"region":      "us-west-2",
			"zone":        "us-west-2a",
			"instance_type": "m5.2xlarge",
			"cloud_provider": "aws",
		},
	}
}
