package handlers

import (
	"net/http"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/topology"
	"github.com/open-wander/wander/ui-htmx/templates/partials/topologypartials"
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

	http.NotFound(w, r)
}

// List handles the topology visualization page.
// This provides a visual representation of the cluster topology including
// datacenters, nodes, and allocations.
func (h *TopologyHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("topology request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock topology data
	topologyData := h.getTopologyData()

	// Render the topology page
	err := Render(w, r,
		topology.List(topologyData),                // Full page with layout
		topologypartials.ListContent(topologyData), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render topology", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getTopologyData returns mock topology data.
// This will be replaced with actual Nomad API calls in the future.
func (h *TopologyHandler) getTopologyData() topologypartials.TopologyData {
	now := time.Now()

	// Mock datacenters with nodes
	datacenters := []topologypartials.Datacenter{
		{
			Name:   "dc1",
			Region: "us-west-2",
			Nodes: []topologypartials.Node{
				{
					ID:           "node-1a2b3c4d",
					Name:         "worker-01.dc1",
					Status:       "ready",
					Datacenter:   "dc1",
					NodeClass:    "compute",
					Allocations:  5,
					CPUUsed:      2400,
					CPUTotal:     8000,
					MemoryUsed:   8589934592,  // 8 GB
					MemoryTotal:  17179869184, // 16 GB
					LastHeartbeat: now.Add(-30 * time.Second),
				},
				{
					ID:           "node-2b3c4d5e",
					Name:         "worker-02.dc1",
					Status:       "ready",
					Datacenter:   "dc1",
					NodeClass:    "compute",
					Allocations:  3,
					CPUUsed:      4200,
					CPUTotal:     8000,
					MemoryUsed:   12884901888, // 12 GB
					MemoryTotal:  17179869184, // 16 GB
					LastHeartbeat: now.Add(-15 * time.Second),
				},
				{
					ID:           "node-4d5e6f7g",
					Name:         "worker-04.dc1",
					Status:       "ready",
					Datacenter:   "dc1",
					NodeClass:    "high-memory",
					Allocations:  8,
					CPUUsed:      6800,
					CPUTotal:     16000,
					MemoryUsed:   25769803776, // 24 GB
					MemoryTotal:  34359738368, // 32 GB
					LastHeartbeat: now.Add(-45 * time.Second),
				},
			},
		},
		{
			Name:   "dc2",
			Region: "us-east-1",
			Nodes: []topologypartials.Node{
				{
					ID:           "node-3c4d5e6f",
					Name:         "worker-03.dc2",
					Status:       "down",
					Datacenter:   "dc2",
					NodeClass:    "storage",
					Allocations:  0,
					CPUUsed:      0,
					CPUTotal:     8000,
					MemoryUsed:   0,
					MemoryTotal:  17179869184, // 16 GB
					LastHeartbeat: now.Add(-5 * time.Minute),
				},
				{
					ID:           "node-5e6f7g8h",
					Name:         "worker-05.dc2",
					Status:       "draining",
					Datacenter:   "dc2",
					NodeClass:    "compute",
					Allocations:  2,
					CPUUsed:      1200,
					CPUTotal:     8000,
					MemoryUsed:   4294967296,  // 4 GB
					MemoryTotal:  17179869184, // 16 GB
					LastHeartbeat: now.Add(-20 * time.Second),
				},
				{
					ID:           "node-6f7g8h9i",
					Name:         "worker-06.dc2",
					Status:       "ready",
					Datacenter:   "dc2",
					NodeClass:    "gpu",
					Allocations:  6,
					CPUUsed:      5400,
					CPUTotal:     8000,
					MemoryUsed:   10737418240, // 10 GB
					MemoryTotal:  17179869184, // 16 GB
					LastHeartbeat: now.Add(-10 * time.Second),
				},
			},
		},
	}

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
