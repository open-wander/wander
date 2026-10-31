package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/ui/templates/partials/taskgrouppartials"
)

func TestTaskGroupRuntimeFiltersSortsAndProjectsScaleEvents(t *testing.T) {
	detail := getTaskGroupRuntimeForTest(t, false, "&status=running&sort=node&order=asc")
	if !detail.AllocationsReady || detail.AllocationsError != "" || len(detail.Allocations) != 2 {
		t.Fatalf("unexpected allocations state: %#v", detail)
	}
	if detail.Allocations[0].NodeName != "alpha" || detail.Allocations[1].NodeName != "zeta" {
		t.Fatalf("allocations are not sorted by node ascending: %#v", detail.Allocations)
	}
	if !strings.Contains(detail.Allocations[0].DetailURL, "namespace=team-a") || !strings.Contains(detail.Allocations[0].DetailURL, "region=west") {
		t.Fatalf("allocation link lost scope: %q", detail.Allocations[0].DetailURL)
	}
	if detail.ScaleStatus == nil || detail.ScaleStatus.Desired != 2 || detail.ScaleStatus.Running != 2 || len(detail.ScaleStatus.Events) != 2 {
		t.Fatalf("unexpected scale status: %#v", detail.ScaleStatus)
	}
	if detail.ScaleStatus.Events[0].Target != "2" || detail.ScaleStatus.Events[0].Evaluation != "eval-new" {
		t.Fatalf("scaling events are not newest-first: %#v", detail.ScaleStatus.Events)
	}

	html := renderTaskGroupDetail(t, detail)
	assertContainsAll(t, html, "Runtime Status", "Desired", "Healthy", "Scaling Events", "scaled up", "Allocations", "alloc-alpha", "namespace=team-a", "region=west")
	assertOrderedHTML(t, html, "alpha", "zeta")
}

func TestTaskGroupRuntimePreservesIndependentFailureStates(t *testing.T) {
	detail := getTaskGroupRuntimeForTest(t, true, "")
	if detail.AllocationsReady || detail.AllocationsError == "" || detail.ScaleStatusReady || detail.ScaleStatusError == "" {
		t.Fatalf("runtime failures were rendered as successful empty states: %#v", detail)
	}
	html := renderTaskGroupDetail(t, detail)
	assertContainsAll(t, html, "Allocations unavailable", "Scale status unavailable", "Configured Volumes")
}

func getTaskGroupRuntimeForTest(t *testing.T, failRuntime bool, query string) taskgrouppartials.TaskGroupDetail {
	t.Helper()
	backend := newTaskGroupRuntimeBackend(t, failRuntime)
	t.Cleanup(backend.Close)
	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/jobs/example@team-a/taskgroups/web?namespace=team-a&region=west"+query)
	detail, err := handler.getLiveTaskGroupDetail(request, "example@team-a", "web")
	if err != nil {
		t.Fatalf("get task group runtime: %v", err)
	}
	return detail
}

func newTaskGroupRuntimeBackend(t *testing.T, failRuntime bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/job/example":
			writeJSON(t, w, json.RawMessage(`{"ID":"example","TaskGroups":[{"Name":"web","Count":2,"Tasks":[{"Name":"app","Driver":"raw_exec"}]}]}`))
		case "/v1/job/example/allocations":
			if failRuntime {
				http.Error(w, "no leader", http.StatusServiceUnavailable)
				return
			}
			assertMonitorQueryValue(t, r, "all", "true")
			writeJSON(t, w, json.RawMessage(`[
				{"ID":"alloc-zeta","Name":"example.web[1]","TaskGroup":"web","NodeName":"zeta","ClientStatus":"running","JobVersion":3,"CreateTime":300},
				{"ID":"alloc-complete","Name":"example.web[2]","TaskGroup":"web","NodeName":"middle","ClientStatus":"complete","JobVersion":2,"CreateTime":200},
				{"ID":"alloc-alpha","Name":"example.web[0]","TaskGroup":"web","NodeName":"alpha","ClientStatus":"running","JobVersion":3,"CreateTime":100},
				{"ID":"alloc-other","Name":"example.worker[0]","TaskGroup":"worker","NodeName":"other","ClientStatus":"running","CreateTime":400}
			]`))
		case "/v1/job/example/scale":
			if failRuntime {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			writeJSON(t, w, json.RawMessage(`{"TaskGroups":{"web":{"Desired":2,"Placed":2,"Running":2,"Healthy":1,"Unhealthy":1,"Events":[
				{"Count":1,"PreviousCount":0,"Message":"initial count","Time":100,"CreateIndex":10},
				{"Count":2,"PreviousCount":1,"Message":"scaled up","EvalID":"eval-new","Time":200,"CreateIndex":20}
			]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}
