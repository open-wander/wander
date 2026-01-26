package handlers

import (
	"fmt"
	"net/http"

	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/servers"
	"github.com/open-wander/wander/ui-htmx/templates/partials/serverspartials"
)

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

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Servers - Wander",
		"View Nomad server nodes",
		"/ui/servers",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Servers", URL: ""},
		},
	)

	// Render the servers list page
	err := Render(w, r,
		servers.List(layoutProps, serversList, sortColumn, sortDir),                // Full page with layout
		serverspartials.ListContent(serversList, sortColumn, sortDir), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render servers list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getServersList fetches server members from the Nomad API.
func (h *ServersHandler) getServersList() []serverspartials.ServerSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty server list")
		return []serverspartials.ServerSummary{}
	}

	// Fetch server members from the API
	serverMembers, err := client.Agent().Members()
	if err != nil {
		h.handler.logger.Error("failed to fetch server members from Nomad API", "error", err)
		return []serverspartials.ServerSummary{}
	}

	if serverMembers == nil || len(serverMembers.Members) == 0 {
		h.handler.logger.Debug("no server members returned from Nomad API")
		return []serverspartials.ServerSummary{}
	}

	// Get the current leader to mark it in the list
	// The leader can be determined from the status endpoint or by checking agent self
	leaderAddr := ""
	if leaderInfo, err := client.Status().Leader(); err == nil {
		leaderAddr = leaderInfo
	}

	result := make([]serverspartials.ServerSummary, 0, len(serverMembers.Members))
	for _, member := range serverMembers.Members {
		// Build the full address
		addr := fmt.Sprintf("%s:%d", member.Addr, member.Port)

		// Get values from tags with defaults
		region := member.Tags["region"]
		if region == "" {
			region = serverMembers.ServerRegion
		}
		dc := member.Tags["dc"]
		if dc == "" {
			dc = serverMembers.ServerDC
		}
		version := member.Tags["build"]

		// Check if this member is the leader
		isLeader := false
		if leaderAddr != "" {
			// Leader addr format is typically "ip:port"
			isLeader = addr == leaderAddr || member.Addr+":4647" == leaderAddr
		}

		// Check if this is a raft voter (non-voting servers have "nonvoter" in tags)
		isRaftVoter := member.Tags["raft_vsn"] != "" // Presence of raft_vsn indicates server participation

		result = append(result, serverspartials.ServerSummary{
			Name:       member.Name,
			Address:    addr,
			Status:     member.Status,
			Leader:     isLeader,
			Region:     region,
			Datacenter: dc,
			RaftVoter:  isRaftVoter,
			Version:    version,
		})
	}

	h.handler.logger.Debug("fetched server members from Nomad API", "count", len(result))
	return result
}
