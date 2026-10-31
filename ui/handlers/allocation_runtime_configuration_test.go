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
	detail := getAllocationRuntimeConfigurationForTest(t, false)
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

	html := renderAllocationTasks(t, detail)
	assertContainsAll(t, html,
		"Network Ports", "127.0.0.1:19000", "10.0.0.2:28080", "8080",
		"Live Services", "metrics", "web", "api, primary", "namespace=team-a", "region=west",
		"prepare", "prestart", "sidecar",
	)
	assertOrderedHTML(t, html, "admin", "http")
}

func TestAllocationDetailPreservesServiceFailureState(t *testing.T) {
	detail := getAllocationRuntimeConfigurationForTest(t, true)
	if detail.ServicesReady || detail.ServicesError == "" || len(detail.Ports) != 2 || len(detail.Tasks) != 2 {
		t.Fatalf("service failure hid allocation data or became an empty state: %#v", detail)
	}
	html := renderAllocationTasks(t, detail)
	assertContainsAll(t, html, "Services unavailable", "Live service registrations are unavailable.", "Network Ports", "Tasks (2)")
}

func TestAllocationDetailRendersEmptyNetworkAndServices(t *testing.T) {
	html := renderAllocationTasks(t, allocationspartials.AllocationDetail{ServicesReady: true})
	assertContainsAll(t, html, "No allocated ports.", "No live service registrations.", "No tasks")
}

func getAllocationRuntimeConfigurationForTest(t *testing.T, failServices bool) allocationspartials.AllocationDetail {
	t.Helper()
	backend := newAllocationRuntimeConfigurationBackend(t, failServices)
	t.Cleanup(backend.Close)
	handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/allocations/alloc-1?namespace=team-a&region=west")
	detail, err := handler.getAllocationDetail(request, "alloc-1", GetTokenFromRequest(request))
	if err != nil {
		t.Fatalf("get allocation detail: %v", err)
	}
	return detail
}

func newAllocationRuntimeConfigurationBackend(t *testing.T, failServices bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/allocation/alloc-1":
			writeJSON(t, w, json.RawMessage(`{
				"ID":"alloc-1","JobID":"example","TaskGroup":"web","NodeID":"node-1","NodeName":"client-1","ClientStatus":"running",
				"AllocatedResources":{"Shared":{"Ports":[
					{"Label":"http","Value":28080,"To":8080,"HostIP":"10.0.0.2"},
					{"Label":"admin","Value":19000,"HostIP":"127.0.0.1"}
				]},"Tasks":{}},
				"TaskStates":{"prepare":{"State":"running"},"app":{"State":"running"}},
				"Job":{"TaskGroups":[{"Name":"web","Tasks":[
					{"Name":"prepare","Lifecycle":{"Hook":"prestart","Sidecar":true}},
					{"Name":"app"}
				]}]}
			}`))
		case "/v1/allocation/alloc-1/services":
			writeAllocationServicesFixture(t, w, failServices)
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
