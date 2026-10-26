package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/servers"
	"github.com/open-wander/wander/ui/templates/partials/serverspartials"
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

// ServeHTTP handles requests to the servers endpoints.
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
	if isServerMonitorRoute(path) {
		h.handler.serveAgentMonitor(w, r, "/ui/servers/", "The server monitor page you requested could not be found.", h.serverMonitorPage, h.streamServerMonitor)
		return
	}

	// Handle /ui/servers/{name} (detail page)
	if strings.HasPrefix(path, "/ui/servers/") {
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the servers list page
func (h *ServersHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("servers list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get live server data.
	serversList := h.getServersList(r)

	sortColumn, sortDir := serverSortOptions(r.URL.Query())
	sortServers(serversList, sortColumn, sortDir)
	page := parsePage(r.URL.Query().Get("page"))
	perPage := 8
	if r.URL.Query().Has("per_page") {
		perPage = parsePerPage(r.URL.Query().Get("per_page"))
	}
	page = boundedServerPage(page, len(serversList), perPage)
	pagination := serverspartials.PaginationMeta{Page: page, PerPage: perPage, Total: len(serversList)}
	serversList = paginateServers(serversList, page, perPage)
	listURL := serverListURL(GetRegionFromRequest(r))

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Servers - Wander",
		"View Wander server nodes",
		"/ui/servers",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Servers", URL: ""},
		},
	)

	// Render the servers list page
	err := Render(w, r,
		servers.List(layoutProps, serversList, sortColumn, sortDir, pagination, listURL),
		serverspartials.ListContent(serversList, sortColumn, sortDir, pagination, listURL),
	)

	if err != nil {
		h.handler.logger.Error("failed to render servers list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getServersList fetches server members from the Nomad API.
func (h *ServersHandler) getServersList(r *http.Request) []serverspartials.ServerSummary {
	client := h.handler.NomadClientForRequest(r)
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
		// Get values from tags with defaults
		dc := member.Tags["dc"]
		if dc == "" {
			dc = serverMembers.ServerDC
		}
		version := member.Tags["build"]

		// Check if this member is the leader
		isLeader := false
		if leaderAddr != "" {
			isLeader = serverRPCAddress(member) == leaderAddr
		}

		result = append(result, serverspartials.ServerSummary{
			Name:       member.Name,
			Address:    member.Addr,
			Port:       strconv.Itoa(int(member.Port)),
			Status:     member.Status,
			Leader:     isLeader,
			Datacenter: dc,
			Version:    version,
		})
	}

	h.handler.logger.Debug("fetched server members from Nomad API", "count", len(result))
	return result
}

// Detail handles the server detail page
func (h *ServersHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract server name from path
	serverName := strings.TrimPrefix(r.URL.Path, "/ui/servers/")
	serverName = strings.TrimSuffix(serverName, "/")

	// Log the request
	h.handler.logger.Debug("server detail request", "serverName", serverName, "htmx", IsHTMXFromRequest(r))

	// Get server detail data from Nomad API
	serverDetail, found := h.getServerDetail(r, serverName)
	if !found {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		serverDetail.Name+" - Server Detail - Wander",
		"Server detail for "+serverDetail.Name,
		"/ui/servers/"+serverName,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Servers", URL: "/ui/servers"},
			{Label: serverDetail.Name, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		servers.Detail(layoutProps, serverDetail),   // Full page with layout
		serverspartials.DetailContent(serverDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render server detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getServerDetail fetches detailed information for a specific server by name.
// It uses both Members() to get the server info and Self() if it's the connected server.
func (h *ServersHandler) getServerDetail(r *http.Request, serverName string) (serverspartials.ServerDetail, bool) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty server detail")
		return serverspartials.ServerDetail{}, false
	}

	// Fetch server members from the API
	serverMembers, err := client.Agent().Members()
	if err != nil {
		h.handler.logger.Error("failed to fetch server members from Nomad API", "error", err)
		return serverspartials.ServerDetail{}, false
	}

	// Find the requested server in the members list
	var targetMember *api.AgentMember
	for _, member := range serverMembers.Members {
		if member.Name == serverName {
			targetMember = member
			break
		}
	}

	if targetMember == nil {
		h.handler.logger.Debug("server not found in members list", "serverName", serverName)
		return serverspartials.ServerDetail{}, false
	}

	// Get the current leader to check if this server is the leader
	leaderAddr := ""
	if leaderInfo, err := client.Status().Leader(); err == nil {
		leaderAddr = leaderInfo
	}

	// Build the full address
	addr := serverRPCAddress(targetMember)

	// Check if this member is the leader
	isLeader := false
	if leaderAddr != "" {
		isLeader = addr == leaderAddr || targetMember.Addr+":4647" == leaderAddr
	}

	// Extract values from tags
	region := targetMember.Tags["region"]
	if region == "" {
		region = serverMembers.ServerRegion
	}
	dc := targetMember.Tags["dc"]
	if dc == "" {
		dc = serverMembers.ServerDC
	}
	version := targetMember.Tags["build"]
	raftVersion := targetMember.Tags["raft_vsn"]

	// Build protocol version string
	protocolVersion := fmt.Sprintf("%d (min: %d, max: %d)",
		targetMember.ProtocolCur, targetMember.ProtocolMin, targetMember.ProtocolMax)

	// Try to get additional stats from Self() if we're looking at the connected server
	var stats map[string]map[string]string
	selfInfo, err := client.Agent().Self()
	if err == nil && selfInfo.Member.Name == serverName {
		stats = selfInfo.Stats
	}

	return serverspartials.ServerDetail{
		Name:            targetMember.Name,
		Address:         addr,
		Status:          targetMember.Status,
		Leader:          isLeader,
		Region:          region,
		Datacenter:      dc,
		Version:         version,
		ProtocolVersion: protocolVersion,
		RaftVersion:     raftVersion,
		BuildDate:       targetMember.Tags["build_date"],
		Revision:        targetMember.Tags["revision"],
		Tags:            targetMember.Tags,
		SortedTags:      sortedServerTags(targetMember.Tags),
		Stats:           stats,
	}, true
}

func serverSortOptions(query url.Values) (string, string) {
	column := query.Get("sort")
	if column == "isLeader" {
		column = "leader"
	}
	validColumns := map[string]bool{"name": true, "status": true, "leader": true, "address": true, "port": true, "datacenter": true, "version": true}
	if !validColumns[column] {
		return "leader", "desc"
	}
	direction := strings.ToLower(query.Get("dir"))
	if direction == "" && query.Has("desc") {
		if legacyDesc, err := strconv.ParseBool(query.Get("desc")); err == nil && legacyDesc {
			direction = "desc"
		} else {
			direction = "asc"
		}
	}
	if direction != "desc" {
		direction = "asc"
	}
	return column, direction
}

func sortServers(servers []serverspartials.ServerSummary, column, direction string) {
	sort.SliceStable(servers, func(i, j int) bool {
		left, right := servers[i], servers[j]
		var less bool
		switch column {
		case "status":
			less = left.Status < right.Status
		case "leader":
			less = !left.Leader && right.Leader
		case "address":
			less = left.Address < right.Address
		case "port":
			leftPort, _ := strconv.Atoi(left.Port)
			rightPort, _ := strconv.Atoi(right.Port)
			less = leftPort < rightPort
		case "datacenter":
			less = left.Datacenter < right.Datacenter
		case "version":
			less = left.Version < right.Version
		default:
			less = left.Name < right.Name
		}
		if direction == "desc" {
			return !less && !serverSortEqual(left, right, column)
		}
		return less
	})
}

func serverSortEqual(left, right serverspartials.ServerSummary, column string) bool {
	switch column {
	case "status":
		return left.Status == right.Status
	case "leader":
		return left.Leader == right.Leader
	case "address":
		return left.Address == right.Address
	case "port":
		return left.Port == right.Port
	case "datacenter":
		return left.Datacenter == right.Datacenter
	case "version":
		return left.Version == right.Version
	default:
		return left.Name == right.Name
	}
}

func paginateServers(servers []serverspartials.ServerSummary, page, perPage int) []serverspartials.ServerSummary {
	start := (page - 1) * perPage
	if start >= len(servers) {
		return []serverspartials.ServerSummary{}
	}
	end := min(start+perPage, len(servers))
	return servers[start:end]
}

func boundedServerPage(page, total, perPage int) int {
	totalPages := max(1, (total+perPage-1)/perPage)
	return min(page, totalPages)
}

func serverRPCAddress(member *api.AgentMember) string {
	port := member.Tags["port"]
	if port == "" {
		port = strconv.Itoa(int(member.Port))
	}
	return net.JoinHostPort(member.Addr, port)
}

func sortedServerTags(tags map[string]string) []serverspartials.ServerTag {
	result := make([]serverspartials.ServerTag, 0, len(tags))
	for name, value := range tags {
		result = append(result, serverspartials.ServerTag{Name: name, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func serverListURL(region string) string {
	query := url.Values{}
	query.Set("region", region)
	return "/ui/servers?" + query.Encode()
}
