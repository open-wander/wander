package handlers

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/topology"
	"github.com/open-wander/wander/ui/templates/partials/topologypartials"
)

// TopologyHandler handles requests for the topology visualization page.
type TopologyHandler struct {
	handler *Handler
}

// NewTopologyHandler creates a new topology handler.
func NewTopologyHandler(h *Handler) *TopologyHandler {
	return &TopologyHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the topology endpoints.
func (h *TopologyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/topology exactly (topology visualization page)
	if path == "/ui/topology" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/topology/data (JSON data endpoint for D3 visualization)
	if path == "/ui/topology/data" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Data(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the topology visualization page.
// This provides a visual representation of the cluster topology including
// datacenters, nodes, and allocations.
func (h *TopologyHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("topology request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get topology data from Nomad API
	topologyData := h.getTopologyData(r, token)

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Topology - Wander",
		"View cluster topology visualization",
		"/ui/topology",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Cluster", URL: ""},
			{Label: "Topology", URL: ""},
		},
	)

	// Render the topology page
	err := Render(w, r,
		topology.List(layoutProps, topologyData),   // Full page with layout
		topologypartials.ListContent(topologyData), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render topology", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Data handles JSON data endpoint for the D3 topology visualization.
// Returns topology data in a format suitable for the D3.js visualization.
func (h *TopologyHandler) Data(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("topology data request", "path", r.URL.Path)

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get topology data from Nomad API
	topologyData := h.getTopologyData(r, token)

	// Convert to JSON response format
	response := struct {
		Datacenters []topologypartials.Datacenter `json:"datacenters"`
	}{
		Datacenters: topologyData.Datacenters,
	}

	// Set content type and write JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode topology data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getTopologyData fetches topology data from the Nomad API.
// It combines data from Nodes().List() and Allocations().List().
func (h *TopologyHandler) getTopologyData(r *http.Request, token string) topologypartials.TopologyData {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty topology data")
		return topologypartials.TopologyData{}
	}

	// Build query options with ACL token
	queryOpts := resourceQueryOptions(token)

	// Fetch all nodes from the API
	nodeStubs, _, err := client.Nodes().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch nodes from Nomad API", "error", err)
		return topologypartials.TopologyData{}
	}

	h.handler.logger.Debug("fetched nodes from API", "count", len(nodeStubs))

	// Fetch all allocations from the API
	allocStubs, _, err := client.Allocations().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocations from Nomad API", "error", err)
		return topologypartials.TopologyData{}
	}

	h.handler.logger.Debug("fetched allocations from API", "count", len(allocStubs))

	allocsByNode := summarizeNodeAllocations(allocStubs)

	// Group nodes by datacenter
	nodesByDC := make(map[string][]topologypartials.Node)
	region := GetRegionFromRequest(r)

	// Build node data
	for _, nodeStub := range nodeStubs {
		// Determine composite status
		status := nodeStub.Status // "ready", "down", "initializing", "disconnected"
		if nodeStub.Drain {
			status = "draining"
		} else if nodeStub.SchedulingEligibility == "ineligible" {
			status = "ineligible"
		}

		cpuTotal, memTotal, _, resourcesAvailable := nodeCapacity(nodeStub)
		nodeAllocs := allocsByNode[nodeStub.ID]

		node := topologypartials.Node{
			ID:                 nodeStub.ID,
			Name:               nodeStub.Name,
			Status:             status,
			Datacenter:         nodeStub.Datacenter,
			NodeClass:          nodeStub.NodeClass,
			Allocations:        nodeAllocs.Count,
			CPUUsed:            int(nodeAllocs.CPU),
			CPUTotal:           int(cpuTotal),
			MemoryUsed:         nodeAllocs.MemoryBytes,
			MemoryTotal:        memTotal,
			ResourcesAvailable: resourcesAvailable,
		}

		nodesByDC[nodeStub.Datacenter] = append(nodesByDC[nodeStub.Datacenter], node)

	}

	// Build datacenter list
	datacenters := make([]topologypartials.Datacenter, 0, len(nodesByDC))
	for dcName, nodes := range nodesByDC {
		datacenters = append(datacenters, topologypartials.Datacenter{
			Name:   dcName,
			Region: region,
			Nodes:  nodes,
		})
	}

	// Sort datacenters by name for consistent ordering
	sort.Slice(datacenters, func(i, j int) bool {
		return datacenters[i].Name < datacenters[j].Name
	})

	// Calculate cluster-wide statistics
	totalNodes := 0
	readyNodes := 0
	totalAllocations := 0
	totalCPU := 0
	usedCPU := 0
	totalMemory := int64(0)
	usedMemory := int64(0)

	for _, dc := range datacenters {
		for _, node := range dc.Nodes {
			totalNodes++
			if node.Status == "ready" {
				readyNodes++
			}
			totalAllocations += node.Allocations
			totalCPU += node.CPUTotal
			usedCPU += node.CPUUsed
			totalMemory += node.MemoryTotal
			usedMemory += node.MemoryUsed
		}
	}

	return topologypartials.TopologyData{
		Datacenters:      datacenters,
		TotalNodes:       totalNodes,
		ReadyNodes:       readyNodes,
		TotalAllocations: totalAllocations,
		TotalCPU:         totalCPU,
		UsedCPU:          usedCPU,
		TotalMemory:      totalMemory,
		UsedMemory:       usedMemory,
	}
}
