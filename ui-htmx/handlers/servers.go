package handlers

import (
	"net/http"

	"github.com/open-wander/wander/ui-htmx/templates/pages/servers"
	"github.com/open-wander/wander/ui-htmx/templates/partials/serverspartials"
)

// ServerSummary represents a summary view of a server for list display.
// This structure will eventually be populated from the Nomad API.
type ServerSummary struct {
	Name    string
	Address string
	Status  string
	Leader  bool
	Region  string
	Version string
}

// ServersHandler handles requests for the servers list page.
type ServersHandler struct {
	handler *Handler
}

// NewServersHandler creates a new servers handler.
func NewServersHandler(h *Handler) *ServersHandler {
	return &ServersHandler{
		handler: h,
	}
}

// ServeHTTP handles GET requests to the servers list page.
// It renders the servers list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *ServersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Handle /ui/servers exactly (list page)
	if path == "/ui/servers" {
		h.List(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the servers list page
func (h *ServersHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("servers list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock server data
	serversList := h.getServersList()

	// Parse sort parameters from query string
	sortColumn := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	// Default sort if not specified
	if sortColumn == "" {
		sortColumn = "name"
		sortDir = "asc"
	}

	// Render the servers list page
	err := Render(w, r,
		servers.List(serversList, sortColumn, sortDir),                // Full page with layout
		serverspartials.ListContent(serversList, sortColumn, sortDir), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render servers list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getServersList returns mock server data.
// This will be replaced with actual Nomad API calls in the future.
func (h *ServersHandler) getServersList() []ServerSummary {
	return []ServerSummary{
		{
			Name:    "nomad-server-1",
			Address: "10.0.1.10:4647",
			Status:  "alive",
			Leader:  true,
			Region:  "us-east-1",
			Version: "1.7.2",
		},
		{
			Name:    "nomad-server-2",
			Address: "10.0.1.11:4647",
			Status:  "alive",
			Leader:  false,
			Region:  "us-east-1",
			Version: "1.7.2",
		},
		{
			Name:    "nomad-server-3",
			Address: "10.0.1.12:4647",
			Status:  "alive",
			Leader:  false,
			Region:  "us-east-1",
			Version: "1.7.2",
		},
		{
			Name:    "nomad-server-4",
			Address: "10.0.2.10:4647",
			Status:  "alive",
			Leader:  false,
			Region:  "us-west-2",
			Version: "1.7.2",
		},
		{
			Name:    "nomad-server-5",
			Address: "10.0.2.11:4647",
			Status:  "failed",
			Leader:  false,
			Region:  "us-west-2",
			Version: "1.7.1",
		},
	}
}
