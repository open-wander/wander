package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestClientAndTopologyResourcesUseLiveReservations(t *testing.T) {
	node := testResourceNode(1000, 2048, 10_000, 100, 128, 100)
	node.ID = "node-1"
	node.Name = "client-1"
	node.Address = "127.0.0.1"
	node.Datacenter = "dc1"
	node.NodePool = "batch"
	node.NodeClass = "gpu"
	node.Version = "1.6.5"
	node.HostVolumes = map[string]*api.HostVolumeInfo{"models": {Path: "/srv/models"}}
	node.Status = "ready"
	activeAllocation := testAllocation("alloc-running", node.ID, api.AllocClientStatusRunning, 50, 32, 10)
	terminalAllocation := testAllocation("alloc-complete", node.ID, api.AllocClientStatusComplete, 900, 900, 0)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("resources") != "true" {
			t.Errorf("%s request omitted resources=true", r.URL.Path)
		}
		if r.URL.Query().Get("region") != "region-1" {
			t.Errorf("%s request region = %q", r.URL.Path, r.URL.Query().Get("region"))
		}
		if r.Header.Get("X-Nomad-Token") != "client-token" {
			t.Errorf("%s request token = %q", r.URL.Path, r.Header.Get("X-Nomad-Token"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/nodes":
			_ = json.NewEncoder(w).Encode([]*api.NodeListStub{node})
		case "/v1/allocations":
			if r.URL.Query().Get("namespace") != "team-a" {
				t.Errorf("allocation namespace = %q", r.URL.Query().Get("namespace"))
			}
			_ = json.NewEncoder(w).Encode([]*api.AllocationListStub{activeAllocation, terminalAllocation})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	request := httptest.NewRequest(http.MethodGet, "/ui/clients?namespace=team-a&region=region-1", nil)
	clientList, err := NewClientsHandler(handler).getClientsList(request, "client-token")
	if err != nil {
		t.Fatalf("get clients list: %v", err)
	}
	if len(clientList) != 1 {
		t.Fatalf("client count = %d, want 1", len(clientList))
	}
	client := clientList[0]
	if client.NodePool != "batch" || client.Version != "1.6.5" || len(client.Volumes) != 1 || client.Volumes[0] != "models" {
		t.Fatalf("client list metadata = %#v", client)
	}
	if client.DetailURL != "/ui/clients/node-1?namespace=team-a&region=region-1" {
		t.Fatalf("client detail URL = %q", client.DetailURL)
	}
	if client.Allocations != 1 || client.CPUUsed != 50 || client.CPUTotal != 900 {
		t.Fatalf("unexpected client CPU/allocation summary: %#v", client)
	}
	if client.MemUsed != 32*bytesPerMegabyte || client.MemTotal != 1920*bytesPerMegabyte ||
		client.DiskUsed != 10*bytesPerMegabyte || client.DiskTotal != 9900*bytesPerMegabyte {
		t.Fatalf("unexpected client memory/disk summary: %#v", client)
	}
	if !client.ResourcesAvailable || !client.AllocationsAvailable {
		t.Fatalf("live client resources reported unavailable: %#v", client)
	}

	topology := NewTopologyHandler(handler).getTopologyData(request, "client-token")
	if topology.TotalNodes != 1 || topology.TotalAllocations != 1 || topology.UsedCPU != 50 || topology.TotalCPU != 900 {
		t.Fatalf("unexpected topology summary: %#v", topology)
	}
	if len(topology.Datacenters) != 1 || topology.Datacenters[0].Region != "region-1" {
		t.Fatalf("unexpected topology datacenter: %#v", topology.Datacenters)
	}
	topologyNode := topology.Datacenters[0].Nodes[0]
	if topologyNode.MemoryUsed != 32*bytesPerMegabyte || topologyNode.MemoryTotal != 1920*bytesPerMegabyte {
		t.Fatalf("unexpected topology node resources: %#v", topologyNode)
	}
}

func TestNodeCapacityNeverUnderflowsReservedResources(t *testing.T) {
	node := testResourceNode(100, 64, 10, 101, 65, 11)

	cpu, memory, disk, available := nodeCapacity(node)
	if !available || cpu != 0 || memory != 0 || disk != 0 {
		t.Fatalf("nodeCapacity = (%d, %d, %d, %t), want zero capacities and available", cpu, memory, disk, available)
	}
}

func testResourceNode(cpu, memoryMB, diskMB int64, reservedCPU, reservedMemoryMB, reservedDiskMB uint64) *api.NodeListStub {
	return &api.NodeListStub{
		NodeResources: &api.NodeResources{
			Cpu:    api.NodeCpuResources{CpuShares: cpu},
			Memory: api.NodeMemoryResources{MemoryMB: memoryMB},
			Disk:   api.NodeDiskResources{DiskMB: diskMB},
		},
		ReservedResources: &api.NodeReservedResources{
			Cpu:    api.NodeReservedCpuResources{CpuShares: reservedCPU},
			Memory: api.NodeReservedMemoryResources{MemoryMB: reservedMemoryMB},
			Disk:   api.NodeReservedDiskResources{DiskMB: reservedDiskMB},
		},
	}
}

func testAllocation(id, nodeID, status string, cpu, memoryMB, diskMB int64) *api.AllocationListStub {
	return &api.AllocationListStub{
		ID:           id,
		NodeID:       nodeID,
		ClientStatus: status,
		AllocatedResources: &api.AllocatedResources{
			Tasks: map[string]*api.AllocatedTaskResources{
				"fixture": {
					Cpu:    api.AllocatedCpuResources{CpuShares: cpu},
					Memory: api.AllocatedMemoryResources{MemoryMB: memoryMB},
				},
			},
			Shared: api.AllocatedSharedResources{DiskMB: diskMB},
		},
	}
}
