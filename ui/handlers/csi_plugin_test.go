package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

func TestCSIPluginDetailRoutesRenderLiveData(t *testing.T) {
	plugin := testCSIPlugin()
	handler := csiTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plugin/csi/example" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("resources") != "true" {
			t.Errorf("resources query = %q, want true", r.URL.Query().Get("resources"))
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, plugin)
	})
	t.Run("detail full page", func(t *testing.T) {
		body := serveCSIRequest(t, handler, "/ui/csi/plugins/example", false, http.StatusOK)
		assertContainsAll(t, body, "<!doctype html", "example", "org.example.csi", "v1.2.3", "Controller Allocations", "alloc-01")
	})
	t.Run("detail HTMX", func(t *testing.T) {
		body := serveCSIRequest(t, handler, "/ui/csi/plugins/example", true, http.StatusOK)
		assertContainsAll(t, body, "example", "Node Allocations")
		if strings.Contains(strings.ToLower(body), "<!doctype html") {
			t.Error("HTMX response unexpectedly contains the document shell")
		}
	})
	t.Run("filtered allocations", func(t *testing.T) {
		body := serveCSIRequest(t, handler, "/ui/csi/plugins/example/allocations?type=node&healthy=false", false, http.StatusOK)
		assertContainsAll(t, body, "Plugin Allocations", "alloc-02", "Unhealthy", "1 matching allocations")
	})
}

func TestCSIPluginsListDoesNotPresentBackendFailureAsEmpty(t *testing.T) {
	handler := csiTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	})
	body := serveCSIRequest(t, handler, "/ui/csi/plugins", false, http.StatusServiceUnavailable)
	if strings.Contains(body, "No plugins found") {
		t.Fatal("backend failure was rendered as an empty plugin list")
	}
}

func TestCSIPluginDetailRoutesPreserveTruthfulErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		apiStatus  int
		wantStatus int
	}{
		{name: "missing", apiStatus: http.StatusNotFound, wantStatus: http.StatusNotFound},
		{name: "denied", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
		{name: "backend failure", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := csiTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, http.StatusText(test.apiStatus), test.apiStatus)
			})
			serveCSIRequest(t, handler, "/ui/csi/plugins/missing", false, test.wantStatus)
		})
	}
}

func TestCSIPluginsListPaginatesWithoutLosingFilters(t *testing.T) {
	plugins := make([]*api.CSIPluginListStub, 26)
	for index := range plugins {
		plugins[index] = &api.CSIPluginListStub{ID: fmt.Sprintf("plugin-%02d", index), Provider: "org.example.csi"}
	}
	handler := csiTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/plugins" || r.URL.Query().Get("type") != "csi" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, plugins)
	})
	body := serveCSIRequest(t, handler, "/ui/csi/plugins?page=2&per_page=25&sort=id&dir=asc", false, http.StatusOK)
	if count := strings.Count(body, "data-plugin-id="); count != 1 {
		t.Fatalf("plugin row count = %d, want 1", count)
	}
	for _, expected := range []string{"plugin-25", "sort=id", "dir=asc", "per_page=25"} {
		if !strings.Contains(body, expected) {
			t.Errorf("response does not contain %q", expected)
		}
	}
}

func TestBuildPluginAllocationsPageFiltersSortsAndPaginates(t *testing.T) {
	plugin := PluginDetail{
		Controllers: []PluginAllocation{
			testPluginAllocation("controller-old", "controller", true, 1),
			testPluginAllocation("controller-new", "controller", false, 3),
		},
		Nodes: []PluginAllocation{
			testPluginAllocation("node-middle", "node", false, 2),
		},
	}

	page := buildPluginAllocationsPage(plugin, mapValues("type", "controller", "healthy", "false", "per_page", "1"))
	if page.Total != 1 || len(page.Allocations) != 1 || page.Allocations[0].AllocationID != "controller-new" {
		t.Fatalf("unexpected filtered page: %#v", page)
	}

	page = buildPluginAllocationsPage(plugin, mapValues("per_page", "1", "page", "2"))
	if page.Total != 3 || page.TotalPages != 3 || len(page.Allocations) != 1 || page.Allocations[0].AllocationID != "node-middle" {
		t.Fatalf("unexpected paginated page: %#v", page)
	}
}

func testCSIPlugin() *api.CSIPlugin {
	created := time.Date(2026, time.October, 27, 10, 0, 0, 0, time.UTC)
	modified := created.Add(time.Minute)
	return &api.CSIPlugin{
		ID: "example", Provider: "org.example.csi", Version: "v1.2.3", ControllerRequired: true,
		ControllersHealthy: 1, ControllersExpected: 1, NodesHealthy: 0, NodesExpected: 1,
		Controllers: map[string]*api.CSIInfo{
			"node-01": {AllocID: "alloc-01-controller", Healthy: true, UpdateTime: modified},
		},
		Nodes: map[string]*api.CSIInfo{
			"node-02": {AllocID: "alloc-02-node", Healthy: false, HealthDescription: "probe failed", UpdateTime: modified},
		},
		Allocations: []*api.AllocationListStub{
			{
				ID: "alloc-01-controller", NodeID: "node-01", NodeName: "client-one", JobID: "csi-controller",
				Namespace: "default", TaskGroup: "controller", JobVersion: 4, ClientStatus: api.AllocClientStatusRunning,
				CreateTime: created.UnixNano(), ModifyTime: modified.UnixNano(),
				AllocatedResources: &api.AllocatedResources{Tasks: map[string]*api.AllocatedTaskResources{
					"plugin": {Cpu: api.AllocatedCpuResources{CpuShares: 500}, Memory: api.AllocatedMemoryResources{MemoryMB: 256}},
				}},
			},
			{
				ID: "alloc-02-node", NodeID: "node-02", NodeName: "client-two", JobID: "csi-node",
				Namespace: "default", TaskGroup: "node", JobVersion: 2, ClientStatus: api.AllocClientStatusRunning,
				CreateTime: created.UnixNano(), ModifyTime: modified.UnixNano(),
			},
		},
	}
}

func mapValues(pairs ...string) url.Values {
	values := make(url.Values, len(pairs)/2)
	for index := 0; index < len(pairs); index += 2 {
		values[pairs[index]] = []string{pairs[index+1]}
	}
	return values
}

func serveCSIRequest(t *testing.T, handler *CSIHandler, path string, htmx bool, wantStatus int) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if htmx {
		request.Header.Set(HXRequest, "true")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, wantStatus, response.Body.String())
	}
	return response.Body.String()
}

func assertContainsAll(t *testing.T, body string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(body, value) {
			t.Errorf("response does not contain %q", value)
		}
	}
}

func csiTestHandler(t *testing.T, backend http.HandlerFunc) *CSIHandler {
	t.Helper()
	server := httptest.NewServer(backend)
	t.Cleanup(server.Close)
	return NewCSIHandler(newJobsTestHandler(t, server.URL))
}

func testPluginAllocation(id, kind string, healthy bool, updatedUnix int64) PluginAllocation {
	return PluginAllocation{AllocationID: id, Type: kind, Healthy: healthy, Updated: time.Unix(updatedUnix, 0)}
}
