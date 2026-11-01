package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/ui/templates/partials/allocationspartials"
)

func TestAllocationDetailProjectsPortsServicesAndLifecycle(t *testing.T) {
	detail := getAllocationRuntimeConfigurationForTest(t, allocationRuntimeFailures{})
	if len(detail.Ports) != 2 || detail.Ports[0].Label != "admin" || detail.Ports[1].Label != "http" {
		t.Fatalf("ports are not deterministically projected: %#v", detail.Ports)
	}
	if !detail.ServicesReady || detail.ServicesError != "" || len(detail.Services) != 2 {
		t.Fatalf("unexpected services state: %#v", detail)
	}
	if detail.Services[0].Name != "metrics" || detail.Services[1].Name != "web" {
		t.Fatalf("services are not deterministically projected: %#v", detail.Services)
	}
	if !strings.Contains(detail.Services[1].DetailURL, "namespace=team-a") || !strings.Contains(detail.Services[1].DetailURL, "region=west") {
		t.Fatalf("service detail link lost scope: %q", detail.Services[1].DetailURL)
	}
	if got, want := len(detail.Tasks), 2; got != want {
		t.Fatalf("task count = %d, want %d: %#v", got, want, detail.Tasks)
	}
	prepare := detail.Tasks[1]
	if detail.Tasks[0].Name != "app" || prepare.Name != "prepare" || prepare.LifecycleHook != "prestart" || !prepare.LifecycleSidecar {
		t.Fatalf("task lifecycle was not projected: %#v", detail.Tasks)
	}
	if detail.Stats == nil || detail.Stats.CPUPercent != 12.5 || detail.Stats.MemoryUsage != 16*1024*1024 {
		t.Fatalf("aggregate statistics were not projected: %#v", detail.Stats)
	}
	if detail.Tasks[0].Resources.LiveStats == nil || detail.Tasks[0].Resources.LiveStats.CPUPercent != 7.5 || detail.Tasks[1].Resources.LiveStats != nil {
		t.Fatalf("task statistics were not projected truthfully: %#v", detail.Tasks)
	}
	if detail.History.Previous.ID != "alloc-previous" || detail.History.Next.ID != "alloc-next" || detail.History.PreemptedBy.ID != "alloc-preemptor" {
		t.Fatalf("allocation relationships were not projected: %#v", detail.History)
	}
	if len(detail.History.Reschedules) != 2 || detail.History.Reschedules[0].PreviousAllocation.ID != "alloc-retry-new" || len(detail.History.PreemptedAllocations) != 2 || detail.History.PreemptedAllocations[0].ID != "alloc-preempted-a" {
		t.Fatalf("allocation history is not deterministically ordered: %#v", detail.History)
	}
	if !strings.Contains(detail.History.Reschedules[0].PreviousNode.URL, "namespace=team-a") || !strings.Contains(detail.History.Reschedules[0].PreviousNode.URL, "region=west") {
		t.Fatalf("history link lost scope: %q", detail.History.Reschedules[0].PreviousNode.URL)
	}

	html := renderAllocationTasks(t, detail)
	assertContainsAll(t, html,
		"Network Ports", "127.0.0.1:19000", "10.0.0.2:28080", "8080",
		"Live Services", "metrics", "web", "api, primary", "namespace=team-a", "region=west",
		"prepare", "prestart", "sidecar", "Live Resource Statistics", "12.5%", "16.0 MiB",
		"Live Resource Usage", "7.5%", "8.0 MiB", "Reserved Resources",
		"Allocation History", "Reschedule attempts", "Preempted by:", "alloc-pr", "alloc-ne",
	)
	if strings.Count(html, "Sampled") < 2 {
		t.Fatalf("aggregate and task sample timestamps were not rendered: %s", html)
	}
	assertOrderedHTML(t, html, "admin", "http")
}

func TestAllocationDetailPreservesServiceFailureState(t *testing.T) {
	detail := getAllocationRuntimeConfigurationForTest(t, allocationRuntimeFailures{services: true})
	if detail.ServicesReady || detail.ServicesError == "" || len(detail.Ports) != 2 || len(detail.Tasks) != 2 {
		t.Fatalf("service failure hid allocation data or became an empty state: %#v", detail)
	}
	html := renderAllocationTasks(t, detail)
	assertContainsAll(t, html, "Services unavailable", "Live service registrations are unavailable.", "Network Ports", "Tasks (2)")
}

func TestAllocationDetailPreservesStatisticsFailureState(t *testing.T) {
	detail := getAllocationRuntimeConfigurationForTest(t, allocationRuntimeFailures{stats: true})
	if detail.StatsReady || detail.StatsError == "" || !detail.ServicesReady || len(detail.Tasks) != 2 {
		t.Fatalf("statistics failure hid allocation data or became an empty state: %#v", detail)
	}
	if detail.Tasks[0].Resources.LiveStats != nil {
		t.Fatalf("failed statistics read fabricated task usage: %#v", detail.Tasks[0].Resources)
	}
	html := renderAllocationTasks(t, detail)
	assertContainsAll(t, html, "Statistics unavailable", "Live resource statistics are unavailable.", "Reserved Resources", "Allocation History")
}

func TestAllocationDetailRendersEmptyNetworkAndServices(t *testing.T) {
	html := renderAllocationTasks(t, allocationspartials.AllocationDetail{ServicesReady: true, StatsReady: true})
	assertContainsAll(t, html, "No allocated ports.", "No live service registrations.", "No live resource sample is available.", "No reschedule or preemption history.", "No tasks")
}

func TestAllocationDetailSkipsTaskRuntimeForOtherViews(t *testing.T) {
	runtimeReads := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		if r.URL.Path != "/v1/allocation/alloc-1" {
			runtimeReads++
			http.Error(w, "unexpected runtime read", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, json.RawMessage(`{"ID":"alloc-1","ClientStatus":"running"}`))
	}))
	t.Cleanup(backend.Close)
	handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/allocations/alloc-1/logs?namespace=team-a&region=west")
	if _, err := handler.getAllocationDetail(request, "alloc-1", GetTokenFromRequest(request), false); err != nil {
		t.Fatalf("get allocation detail without task runtime: %v", err)
	}
	if runtimeReads != 0 {
		t.Fatalf("non-task allocation view made %d task-runtime reads", runtimeReads)
	}
}

type allocationRuntimeFailures struct {
	services bool
	stats    bool
}

func getAllocationRuntimeConfigurationForTest(t *testing.T, failures allocationRuntimeFailures) allocationspartials.AllocationDetail {
	t.Helper()
	backend := newAllocationRuntimeConfigurationBackend(t, failures)
	t.Cleanup(backend.Close)
	handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/allocations/alloc-1?namespace=team-a&region=west")
	detail, err := handler.getAllocationDetail(request, "alloc-1", GetTokenFromRequest(request), true)
	if err != nil {
		t.Fatalf("get allocation detail: %v", err)
	}
	return detail
}

func newAllocationRuntimeConfigurationBackend(t *testing.T, failures allocationRuntimeFailures) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/allocation/alloc-1":
			writeJSON(t, w, json.RawMessage(`{
				"ID":"alloc-1","JobID":"example","TaskGroup":"web","NodeID":"node-1","NodeName":"client-1","ClientStatus":"running",
				"PreviousAllocation":"alloc-previous","NextAllocation":"alloc-next","PreemptedByAllocation":"alloc-preemptor",
				"PreemptedAllocations":["alloc-preempted-z","alloc-preempted-a"],
				"RescheduleTracker":{"Events":[
					{"RescheduleTime":3000000000,"PrevAllocID":"alloc-retry-old","PrevNodeID":"node-old"},null,
					{"RescheduleTime":4000000000,"PrevAllocID":"alloc-retry-new","PrevNodeID":"node-new"}
				]},
				"AllocatedResources":{"Shared":{"Ports":[
					{"Label":"http","Value":28080,"To":8080,"HostIP":"10.0.0.2"},
					{"Label":"admin","Value":19000,"HostIP":"127.0.0.1"}
				]},"Tasks":{"app":{"Cpu":{"CpuShares":50},"Memory":{"MemoryMB":32}},"prepare":{"Cpu":{"CpuShares":10},"Memory":{"MemoryMB":10}}}},
				"TaskStates":{"prepare":{"State":"running"},"app":{"State":"running"}},
				"Job":{"TaskGroups":[{"Name":"web","Tasks":[
					{"Name":"prepare","Lifecycle":{"Hook":"prestart","Sidecar":true}},
					{"Name":"app"}
				]}]}
			}`))
		case "/v1/allocation/alloc-1/services":
			writeAllocationServicesFixture(t, w, failures.services)
		case "/v1/client/allocation/alloc-1/stats":
			if failures.stats {
				http.Error(w, "client unavailable", http.StatusServiceUnavailable)
				return
			}
			writeJSON(t, w, json.RawMessage(`{"ResourceUsage":{"CpuStats":{"Percent":12.5},"MemoryStats":{"Usage":16777216,"RSS":12582912}},"Tasks":{"app":{"ResourceUsage":{"CpuStats":{"Percent":7.5},"MemoryStats":{"Usage":8388608,"RSS":6291456}},"Timestamp":2000000000}},"Timestamp":1000000000}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeAllocationServicesFixture(t *testing.T, w http.ResponseWriter, unavailable bool) {
	t.Helper()
	if unavailable {
		http.Error(w, "service registry unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(t, w, json.RawMessage(`[
		{"ServiceName":"web","Address":"10.0.0.2","Port":28080,"Tags":["api","primary"]},
		{"ServiceName":"metrics","Address":"127.0.0.1","Port":19000}
	]`))
}

func renderAllocationTasks(t *testing.T, detail allocationspartials.AllocationDetail) string {
	t.Helper()
	var output strings.Builder
	if err := allocationspartials.TasksTab(detail).Render(context.Background(), &output); err != nil {
		t.Fatalf("render allocation tasks: %v", err)
	}
	return output.String()
}
