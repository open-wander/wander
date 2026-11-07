package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestAllocationListProjectsInheritedFieldsAndPreservesScope(t *testing.T) {
	handler := newAllocationListTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		if filter := r.URL.Query().Get("filter"); filter != "" {
			t.Fatalf("allocation list unexpectedly hid filter choices with API filter %q", filter)
		}
		writeJSON(t, w, allocationListFixtures())
	})

	body := serveAllocationListRequest(t, handler, "/ui/allocations?namespace=team-a&region=west&type=system&task_group=daemon&status=running&scheduling=run&node=client-system&per_page=10&sort=version&dir=desc", http.StatusOK)
	assertContainsAll(t, body,
		"Short ID", "Job Version", "Client", "Scheduling", "View Logs",
		"system-job", "system", "daemon", "client-system", "placed by scheduler", ">3<",
		"service", "batch", "sysbatch",
		"namespace=team-a", "region=west",
		`name="namespace" value="team-a"`, `name="region" value="west"`,
		"/ui/allocations/alloc-system/logs", "/ui/clients/node-system", "/ui/jobs/system-job",
		"Rows per page", `value="10" selected`,
	)
	assertNotContainsAll(t, body, "service-job</a>", "batch-job</a>", "sysbatch-job</a>")
}

func TestAllocationListSortsAndPaginatesDeterministically(t *testing.T) {
	handler := newAllocationListTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, allocationListFixtures())
	})

	body := serveAllocationListRequest(t, handler, "/ui/allocations?namespace=team-a&region=west&per_page=2&page=2&sort=version&dir=desc", http.StatusOK)
	assertContainsAll(t, body, "batch-job", "service-job", "Showing", "results", `aria-current="page"`)
	assertNotContainsAll(t, body, "system-job</a>", "sysbatch-job</a>")
	assertOrderedHTML(t, body, "batch-job", "service-job")

	allocations := []AllocationSummary{{ID: "alloc-b", JobVersion: 1}, {ID: "alloc-a", JobVersion: 1}}
	NewAllocationsHandler(handler).sortAllocations(allocations, "version", "asc")
	if allocations[0].ID != "alloc-a" || allocations[1].ID != "alloc-b" {
		t.Fatalf("equal-version allocations are not deterministically ordered: %#v", allocations)
	}
	NewAllocationsHandler(handler).sortAllocations(allocations, "version", "desc")
	if allocations[0].ID != "alloc-b" || allocations[1].ID != "alloc-a" {
		t.Fatalf("descending equal-version allocations are not deterministically ordered: %#v", allocations)
	}
}

func TestAllocationListUsesEachAllocationsActualNamespace(t *testing.T) {
	handler := newAllocationListTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, []*api.AllocationListStub{{
			ID: "alloc-default", Namespace: "default", JobID: "default-job", NodeID: "node-a",
		}})
	})

	body := serveAllocationListRequest(t, handler, "/ui/allocations?namespace=team-a&region=west", http.StatusOK)
	assertContainsAll(t, body,
		"/ui/allocations/alloc-default?namespace=default&amp;region=west",
		"/ui/jobs/default-job?namespace=default&amp;region=west",
	)
}

func TestAllocationListPreservesEmptyDeniedAndUnavailableStates(t *testing.T) {
	for _, test := range []struct {
		name       string
		apiStatus  int
		wantStatus int
		wantBody   string
	}{
		{name: "empty", apiStatus: http.StatusOK, wantStatus: http.StatusOK, wantBody: "No allocations found"},
		{name: "denied", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden, wantBody: "Access Denied"},
		{name: "unavailable", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable, wantBody: "Cluster Unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := newAllocationListTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
				if test.apiStatus == http.StatusOK {
					writeJSON(t, w, []*api.AllocationListStub{})
					return
				}
				http.Error(w, fmt.Sprintf("backend status %d", test.apiStatus), test.apiStatus)
			})
			body := serveAllocationListRequest(t, handler, "/ui/allocations?namespace=team-a&region=west", test.wantStatus)
			if !strings.Contains(body, test.wantBody) {
				t.Fatalf("body does not contain %q: %s", test.wantBody, body)
			}
			if test.apiStatus != http.StatusOK && strings.Contains(body, "No allocations found") {
				t.Fatalf("backend failure rendered as an empty allocation list: %s", body)
			}
		})
	}
}

func newAllocationListTestHandler(t *testing.T, backendHandler http.HandlerFunc) *Handler {
	t.Helper()
	backend := httptest.NewServer(backendHandler)
	t.Cleanup(backend.Close)
	return newJobsTestHandler(t, backend.URL)
}

func serveAllocationListRequest(t *testing.T, handler *Handler, target string, wantStatus int) string {
	t.Helper()
	request := requestWithToken(http.MethodGet, target)
	response := httptest.NewRecorder()
	NewAllocationsHandler(handler).ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("GET %s status = %d, want %d; body: %s", target, response.Code, wantStatus, response.Body.String())
	}
	return response.Body.String()
}

func allocationListFixtures() []*api.AllocationListStub {
	return []*api.AllocationListStub{
		allocationListFixture("service", "service-job", api.JobTypeService, "web", api.AllocClientStatusRunning, api.AllocDesiredStatusRun, "placed by scheduler", 1),
		allocationListFixture("batch", "batch-job", api.JobTypeBatch, "worker", api.AllocClientStatusComplete, api.AllocDesiredStatusStop, "batch complete", 2),
		allocationListFixture("system", "system-job", api.JobTypeSystem, "daemon", api.AllocClientStatusRunning, api.AllocDesiredStatusRun, "placed by scheduler", 3),
		allocationListFixture("sysbatch", "sysbatch-job", api.JobTypeSysbatch, "maintenance", api.AllocClientStatusFailed, api.AllocDesiredStatusStop, "task failed", 4),
	}
}

func allocationListFixture(suffix, jobID, jobType, group, status, desired, description string, version uint64) *api.AllocationListStub {
	return &api.AllocationListStub{
		ID: "alloc-" + suffix, JobID: jobID, JobType: jobType, JobVersion: version,
		TaskGroup: group, NodeID: "node-" + suffix, NodeName: "client-" + suffix,
		ClientStatus: status, DesiredStatus: desired, DesiredDescription: description,
		CreateTime: int64(version) * 1_000_000_000,
	}
}

func assertNotContainsAll(t *testing.T, body string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(body, value) {
			t.Fatalf("body unexpectedly contains %q: %s", value, body)
		}
	}
}
