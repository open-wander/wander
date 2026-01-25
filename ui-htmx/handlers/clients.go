package handlers

import (
	"net/http"

	"github.com/open-wander/wander/ui-htmx/templates/pages/clients"
	"github.com/open-wander/wander/ui-htmx/templates/partials/clientspartials"
)

// ClientSummary represents a summary view of a client for list display.
// This structure will eventually be populated from the Nomad API.
type ClientSummary struct {
	ID          string
	Name        string
	Status      string
	Datacenter  string
	Allocations int
	CPUUsed     int
	CPUTotal    int
	MemUsed     int64
	MemTotal    int64
}

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

	// For now, only handle the list page
	// Detail pages will be implemented later
	if r.URL.Path == "/ui/clients" {
		h.List(w, r)
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

// getClientsList returns mock client data.
// This will be replaced with actual Nomad API calls in the future.
func (h *ClientsHandler) getClientsList() []ClientSummary {
	return []ClientSummary{
		{
			ID:          "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o",
			Name:        "client-node-1",
			Status:      "ready",
			Datacenter:  "dc1",
			Allocations: 5,
			CPUUsed:     2400,
			CPUTotal:    8000,
			MemUsed:     8589934592,  // 8 GB in bytes
			MemTotal:    17179869184, // 16 GB in bytes
		},
		{
			ID:          "d2e3f4g5-h6i7-8j9k-0l1m-2n3o4p5q6r7s",
			Name:        "client-node-2",
			Status:      "ready",
			Datacenter:  "dc1",
			Allocations: 3,
			CPUUsed:     4200,
			CPUTotal:    8000,
			MemUsed:     12884901888, // 12 GB in bytes
			MemTotal:    17179869184, // 16 GB in bytes
		},
		{
			ID:          "e3f4g5h6-i7j8-9k0l-1m2n-3o4p5q6r7s8t",
			Name:        "client-node-3",
			Status:      "down",
			Datacenter:  "dc2",
			Allocations: 0,
			CPUUsed:     0,
			CPUTotal:    8000,
			MemUsed:     0,
			MemTotal:    17179869184, // 16 GB in bytes
		},
		{
			ID:          "f4g5h6i7-j8k9-0l1m-2n3o-4p5q6r7s8t9u",
			Name:        "client-node-4",
			Status:      "ready",
			Datacenter:  "dc1",
			Allocations: 8,
			CPUUsed:     6800,
			CPUTotal:    16000,
			MemUsed:     25769803776, // 24 GB in bytes
			MemTotal:    34359738368, // 32 GB in bytes
		},
		{
			ID:          "g5h6i7j8-k9l0-1m2n-3o4p-5q6r7s8t9u0v",
			Name:        "client-node-5",
			Status:      "draining",
			Datacenter:  "dc2",
			Allocations: 2,
			CPUUsed:     1200,
			CPUTotal:    8000,
			MemUsed:     4294967296,  // 4 GB in bytes
			MemTotal:    17179869184, // 16 GB in bytes
		},
		{
			ID:          "h6i7j8k9-l0m1-2n3o-4p5q-6r7s8t9u0v1w",
			Name:        "client-node-6",
			Status:      "ready",
			Datacenter:  "dc2",
			Allocations: 6,
			CPUUsed:     5400,
			CPUTotal:    8000,
			MemUsed:     10737418240, // 10 GB in bytes
			MemTotal:    17179869184, // 16 GB in bytes
		},
	}
}
