package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestTopologyProjectsDeterministicMultiDatacenterGraph(t *testing.T) {
	backend := newTopologyBackend(t)
	defer backend.Close()
	handler := NewTopologyHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/topology?namespace=team-a&region=west")

	data, err := handler.getTopologyData(request, GetTokenFromRequest(request))
	if err != nil {
		t.Fatalf("get topology data: %v", err)
	}
	if data.TotalNodes != 3 || data.ReadyNodes != 2 || data.TotalAllocations != 3 {
		t.Fatalf("unexpected complete topology totals: %#v", data)
	}
	if data.VisibleNodes != 3 || data.VisibleAllocations != 3 {
		t.Fatalf("unexpected visible topology totals: %#v", data)
	}
	if len(data.Datacenters) != 2 || data.Datacenters[0].Name != "dc1" || data.Datacenters[1].Name != "dc2" {
		t.Fatalf("datacenters are not deterministic: %#v", data.Datacenters)
	}
	dc1 := data.Datacenters[0]
	if len(dc1.Nodes) != 2 || dc1.Nodes[0].Name != "alpha" || dc1.Nodes[1].Name != "beta" {
		t.Fatalf("dc1 nodes are not deterministic: %#v", dc1.Nodes)
	}
	allocations := dc1.Nodes[1].AllocationItems
	if len(allocations) != 2 || allocations[0].JobID != "alpha-job" || allocations[1].JobID != "beta-job" {
		t.Fatalf("allocation overlays are not deterministic: %#v", allocations)
	}
	if allocations[0].DetailURL != "/ui/allocations/alloc-pending?namespace=team-a&region=west" ||
		allocations[0].JobURL != "/ui/jobs/alpha-job?namespace=team-a&region=west" {
		t.Fatalf("allocation links lost scope: %#v", allocations[0])
	}
	if strings.Join(data.FilterOptions.Datacenters, ",") != "dc1,dc2" ||
		strings.Join(data.FilterOptions.AllocationStatuses, ",") != "complete,pending,running" {
		t.Fatalf("unexpected topology filter options: %#v", data.FilterOptions)
	}
}

func TestTopologyFiltersAndSelectsNodeAllocationRelationship(t *testing.T) {
	backend := newTopologyBackend(t)
	defer backend.Close()
	target := "/ui/topology?namespace=team-a&region=west&metric=memory&allocation_status=running" +
		"&node_pool=default&datacenter=dc1&class=general&state=ready&version=1.6.5" +
		"&selected_node=node-beta&selected_allocation=alloc-running"
	request := requestWithToken(http.MethodGet, target)
	response := httptest.NewRecorder()
	NewTopologyHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("topology status = %d, body: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		"Metrics", "Allocation Status", "Node Pool", "Datacenter", "Class", "State", "Version",
		`data-topology-node-id="node-beta"`, `data-topology-allocation-id="alloc-running"`,
		`data-selected="true"`, `aria-label="Allocations connected to beta"`,
		"beta-job", "web", "100 MHz", "32.0 MB", "Showing 1 client(s) and 1 allocation(s)",
		`data-topology-capacity-map="node-beta"`, `data-topology-resource="memory"`,
		`data-topology-resource-segment="alloc-running"`, "Allocation reservation map",
		"Segments show scheduler reservations", "32.0 MB memory reserved",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("topology body missing %q: %s", expected, body)
		}
	}
	for _, unexpected := range []string{"node-alpha", "node-zeta", "alloc-pending", "alloc-complete"} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("topology body unexpectedly contains %q", unexpected)
		}
	}
	if strings.Contains(body, "CPU allocated") || !strings.Contains(body, "Memory allocated") {
		t.Fatalf("metric selection did not control resource presentation: %s", body)
	}
	if strings.Contains(body, `data-topology-resource="cpu"`) {
		t.Fatalf("memory metric rendered a CPU capacity track: %s", body)
	}
}

func TestTopologyRendersTruthfulEmptyFilterState(t *testing.T) {
	backend := newTopologyBackend(t)
	defer backend.Close()
	request := requestWithToken(http.MethodGet, "/ui/topology?namespace=team-a&region=west&datacenter=missing")
	response := httptest.NewRecorder()
	NewTopologyHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "No clients match these filters") {
		t.Fatalf("unexpected empty topology response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func newTopologyBackend(t *testing.T) *httptest.Server {
	t.Helper()
	nodes := []*api.NodeListStub{
		topologyNodeFixture("node-zeta", "zeta", "dc2", "gpu", "compute", "1.6.5", api.NodeStatusReady),
		topologyNodeFixture("node-beta", "beta", "dc1", "default", "general", "1.6.5", api.NodeStatusReady),
		topologyNodeFixture("node-alpha", "alpha", "dc1", "default", "general", "1.6.4", api.NodeStatusDown),
	}
	allocations := []*api.AllocationListStub{
		topologyAllocationFixture("alloc-running", "node-beta", "beta-job", "web", api.AllocClientStatusRunning, 100, 32),
		topologyAllocationFixture("alloc-pending", "node-beta", "alpha-job", "worker", api.AllocClientStatusPending, 50, 16),
		topologyAllocationFixture("alloc-complete", "node-alpha", "old-job", "batch", api.AllocClientStatusComplete, 25, 8),
		topologyAllocationFixture("alloc-gpu", "node-zeta", "gpu-job", "trainer", api.AllocClientStatusRunning, 200, 64),
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/nodes":
			assertTopologyScope(t, r)
			writeJSON(t, w, nodes)
		case "/v1/allocations":
			assertTopologyScope(t, r)
			if r.URL.Query().Get("namespace") != "team-a" {
				t.Errorf("allocation namespace = %q", r.URL.Query().Get("namespace"))
			}
			writeJSON(t, w, allocations)
		case "/v1/namespaces", "/v1/regions":
			_, _ = w.Write([]byte("[]"))
		default:
			http.NotFound(w, r)
		}
	}))
}

func assertTopologyScope(t *testing.T, r *http.Request) {
	t.Helper()
	if r.URL.Query().Get("resources") != "true" {
		t.Errorf("%s omitted resources=true", r.URL.Path)
	}
	if r.URL.Query().Get("region") != "west" {
		t.Errorf("%s region = %q", r.URL.Path, r.URL.Query().Get("region"))
	}
	if r.Header.Get("X-Nomad-Token") != "browser-token" {
		t.Errorf("%s token = %q", r.URL.Path, r.Header.Get("X-Nomad-Token"))
	}
}

func topologyNodeFixture(id, name, datacenter, nodePool, nodeClass, version, status string) *api.NodeListStub {
	node := testResourceNode(1000, 2048, 10_000, 100, 128, 100)
	node.ID, node.Name, node.Datacenter = id, name, datacenter
	node.NodePool, node.NodeClass, node.Version = nodePool, nodeClass, version
	node.Status = status
	node.SchedulingEligibility = api.NodeSchedulingEligible
	return node
}

func topologyAllocationFixture(id, nodeID, jobID, taskGroup, status string, cpu, memoryMB int64) *api.AllocationListStub {
	allocation := testAllocation(id, nodeID, status, cpu, memoryMB, 10)
	allocation.Namespace = "team-a"
	allocation.JobID = jobID
	allocation.TaskGroup = taskGroup
	allocation.DesiredStatus = api.AllocDesiredStatusRun
	return allocation
}
