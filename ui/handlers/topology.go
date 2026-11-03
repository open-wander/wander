package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/open-wander/wander/api"
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
	topologyData, err := h.getTopologyData(r, token)
	if err != nil {
		h.handler.renderScopedReadError(w, r, GetNamespaceFromRequest(r), "load topology", "topology", "node:read", false, err)
		return
	}

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
	err = Render(w, r,
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
	topologyData, err := h.getTopologyData(r, token)
	if err != nil {
		h.handler.renderScopedReadError(w, r, GetNamespaceFromRequest(r), "load topology", "topology", "node:read", false, err)
		return
	}

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
func (h *TopologyHandler) getTopologyData(r *http.Request, token string) (topologypartials.TopologyData, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return topologypartials.TopologyData{}, fmt.Errorf("Wander API client is unavailable")
	}

	// Build query options with ACL token
	queryOpts := resourceQueryOptions(token)

	// Fetch all nodes from the API
	nodeStubs, _, err := client.Nodes().List(queryOpts)
	if err != nil {
		return topologypartials.TopologyData{}, fmt.Errorf("list topology nodes: %w", err)
	}

	h.handler.logger.Debug("fetched nodes from API", "count", len(nodeStubs))

	// Fetch all allocations from the API
	allocStubs, _, err := client.Allocations().List(queryOpts)
	if err != nil {
		return topologypartials.TopologyData{}, fmt.Errorf("list topology allocations: %w", err)
	}

	h.handler.logger.Debug("fetched allocations from API", "count", len(allocStubs))

	allocsByNode := summarizeNodeAllocations(allocStubs)
	filters := parseTopologyFilters(r)
	filterOptions := topologyFilterOptions(nodeStubs, allocStubs)
	allocationsByNode := projectTopologyAllocations(allocStubs, filters, GetNamespaceFromRequest(r), GetRegionFromRequest(r))

	// Group nodes by datacenter
	nodesByDC := make(map[string][]topologypartials.Node)
	region := GetRegionFromRequest(r)

	// Build node data
	for _, nodeStub := range nodeStubs {
		if nodeStub == nil || !topologyNodeMatches(nodeStub, filters) {
			continue
		}
		// Determine composite status
		status := topologyNodeStatus(nodeStub)

		cpuTotal, memTotal, _, resourcesAvailable := nodeCapacity(nodeStub)
		nodeAllocs := allocsByNode[nodeStub.ID]

		node := topologypartials.Node{
			ID:                 nodeStub.ID,
			Name:               nodeStub.Name,
			Status:             status,
			Datacenter:         nodeStub.Datacenter,
			NodeClass:          nodeStub.NodeClass,
			NodePool:           nodeStub.NodePool,
			Version:            nodeStub.Version,
			Allocations:        nodeAllocs.Count,
			AllocationItems:    allocationsByNode[nodeStub.ID],
			CPUUsed:            int(nodeAllocs.CPU),
			CPUTotal:           int(cpuTotal),
			MemoryUsed:         nodeAllocs.MemoryBytes,
			MemoryTotal:        memTotal,
			ResourcesAvailable: resourcesAvailable,
			DetailURL:          scopedUIURL("/ui/clients/"+url.PathEscape(nodeStub.ID), GetNamespaceFromRequest(r), region),
			Selected:           filters.SelectedNode == nodeStub.ID,
		}
		for _, allocation := range node.AllocationItems {
			if allocation.Selected {
				node.Selected = true
				break
			}
		}

		nodesByDC[nodeStub.Datacenter] = append(nodesByDC[nodeStub.Datacenter], node)

	}

	// Build datacenter list
	datacenters := make([]topologypartials.Datacenter, 0, len(nodesByDC))
	for dcName, nodes := range nodesByDC {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Name != nodes[j].Name {
				return nodes[i].Name < nodes[j].Name
			}
			return nodes[i].ID < nodes[j].ID
		})
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

	// Keep summary metrics tied to the complete scoped reads. Filters only
	// change the topology graph and its visible-result counts.
	totalNodes, readyNodes, totalAllocations := 0, 0, 0
	totalCPU, usedCPU := 0, 0
	totalMemory, usedMemory := int64(0), int64(0)
	for _, nodeStub := range nodeStubs {
		if nodeStub == nil {
			continue
		}
		totalNodes++
		if topologyNodeStatus(nodeStub) == "ready" {
			readyNodes++
		}
		cpuTotal, memoryTotal, _, _ := nodeCapacity(nodeStub)
		nodeAllocs := allocsByNode[nodeStub.ID]
		totalAllocations += nodeAllocs.Count
		totalCPU += int(cpuTotal)
		usedCPU += int(nodeAllocs.CPU)
		totalMemory += memoryTotal
		usedMemory += nodeAllocs.MemoryBytes
	}
	visibleNodes := 0
	visibleAllocations := 0

	for _, dc := range datacenters {
		for _, node := range dc.Nodes {
			visibleNodes++
			visibleAllocations += len(node.AllocationItems)
		}
	}

	return topologypartials.TopologyData{
		Datacenters:        datacenters,
		TotalNodes:         totalNodes,
		ReadyNodes:         readyNodes,
		TotalAllocations:   totalAllocations,
		TotalCPU:           totalCPU,
		UsedCPU:            usedCPU,
		TotalMemory:        totalMemory,
		UsedMemory:         usedMemory,
		VisibleNodes:       visibleNodes,
		VisibleAllocations: visibleAllocations,
		Filters:            filters,
		FilterOptions:      filterOptions,
		Namespace:          GetNamespaceFromRequest(r),
		Region:             region,
	}, nil
}

func parseTopologyFilters(r *http.Request) topologypartials.Filters {
	query := r.URL.Query()
	metric := query.Get("metric")
	switch metric {
	case "allocations", "cpu", "memory":
	default:
		metric = "all"
	}
	allocationStatus := strings.ToLower(strings.TrimSpace(query.Get("allocation_status")))
	if allocationStatus == "" {
		allocationStatus = "active"
	}
	return topologypartials.Filters{
		Metric: metric, AllocationStatus: allocationStatus,
		NodePool: query.Get("node_pool"), Datacenter: query.Get("datacenter"),
		NodeClass: query.Get("class"), State: query.Get("state"), Version: query.Get("version"),
		SelectedNode: query.Get("selected_node"), SelectedAllocation: query.Get("selected_allocation"),
	}
}

func topologyFilterOptions(nodes []*api.NodeListStub, allocations []*api.AllocationListStub) topologypartials.FilterOptions {
	nodePools := map[string]struct{}{}
	datacenters := map[string]struct{}{}
	nodeClasses := map[string]struct{}{}
	states := map[string]struct{}{}
	versions := map[string]struct{}{}
	allocationStatuses := map[string]struct{}{}
	for _, node := range nodes {
		if node == nil {
			continue
		}
		addNonEmpty(nodePools, node.NodePool)
		addNonEmpty(datacenters, node.Datacenter)
		addNonEmpty(nodeClasses, node.NodeClass)
		addNonEmpty(states, topologyNodeStatus(node))
		addNonEmpty(versions, node.Version)
	}
	for _, allocation := range allocations {
		if allocation != nil {
			addNonEmpty(allocationStatuses, allocation.ClientStatus)
		}
	}
	return topologypartials.FilterOptions{
		NodePools: sortedKeys(nodePools), Datacenters: sortedKeys(datacenters),
		NodeClasses: sortedKeys(nodeClasses), States: sortedKeys(states), Versions: sortedKeys(versions),
		AllocationStatuses: sortedKeys(allocationStatuses),
	}
}

func topologyNodeMatches(node *api.NodeListStub, filters topologypartials.Filters) bool {
	return (filters.NodePool == "" || node.NodePool == filters.NodePool) &&
		(filters.Datacenter == "" || node.Datacenter == filters.Datacenter) &&
		(filters.NodeClass == "" || node.NodeClass == filters.NodeClass) &&
		(filters.State == "" || topologyNodeStatus(node) == filters.State) &&
		(filters.Version == "" || node.Version == filters.Version)
}

func topologyNodeStatus(node *api.NodeListStub) string {
	if node.Drain {
		return "draining"
	}
	if node.SchedulingEligibility == "ineligible" {
		return "ineligible"
	}
	return node.Status
}

func projectTopologyAllocations(allocations []*api.AllocationListStub, filters topologypartials.Filters, namespace, region string) map[string][]topologypartials.Allocation {
	byNode := make(map[string][]topologypartials.Allocation)
	for _, allocation := range allocations {
		if allocation == nil || !topologyAllocationMatches(allocation.ClientStatus, filters.AllocationStatus) {
			continue
		}
		cpu, memoryBytes, _ := allocationReservations(allocation)
		allocationNamespace := allocation.Namespace
		if allocationNamespace == "" {
			allocationNamespace = namespace
		}
		byNode[allocation.NodeID] = append(byNode[allocation.NodeID], topologypartials.Allocation{
			ID: allocation.ID, JobID: allocation.JobID, TaskGroup: allocation.TaskGroup,
			Namespace: allocation.Namespace, ClientStatus: allocation.ClientStatus,
			DesiredStatus: allocation.DesiredStatus, CPU: cpu, MemoryBytes: memoryBytes,
			DetailURL: scopedUIURL("/ui/allocations/"+url.PathEscape(allocation.ID), allocationNamespace, region),
			JobURL:    scopedUIURL("/ui/jobs/"+url.PathEscape(allocation.JobID), allocationNamespace, region),
			Selected:  filters.SelectedAllocation == allocation.ID,
		})
	}
	for nodeID := range byNode {
		sort.Slice(byNode[nodeID], func(i, j int) bool {
			left, right := byNode[nodeID][i], byNode[nodeID][j]
			if left.JobID != right.JobID {
				return left.JobID < right.JobID
			}
			if left.TaskGroup != right.TaskGroup {
				return left.TaskGroup < right.TaskGroup
			}
			return left.ID < right.ID
		})
	}
	return byNode
}

func topologyAllocationMatches(status, filter string) bool {
	switch filter {
	case "all":
		return true
	case "active":
		return isActiveAllocation(status)
	default:
		return strings.EqualFold(status, filter)
	}
}
