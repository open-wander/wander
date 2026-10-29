package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
)

func TestGetJobOverviewProjectsLiveSummaryAllocationsAndMetadata(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertMonitorQueryValue(t, r, "namespace", "team-a")
		if r.Header.Get("X-Nomad-Token") != "browser-token" {
			t.Errorf("token header = %q, want browser-token", r.Header.Get("X-Nomad-Token"))
		}
		switch r.URL.Path {
		case "/v1/job/example":
			writeJSON(t, w, json.RawMessage(`{
				"ID":"example","Name":"Example","Status":"running","Type":"service","NodePool":"workers",
				"Meta":{"zeta":"last","alpha":"first"},
				"ParameterizedJob":{"Payload":"required","MetaRequired":["environment"]},
				"TaskGroups":[{"Name":"web","Count":1,
					"Volumes":{"cache":{"Type":"host","Source":"cache-data","ReadOnly":true,"PerAlloc":true}},
					"Tasks":[{"Name":"setup","Lifecycle":{"Hook":"prestart","Sidecar":false}}]
				}]
			}`))
		case "/v1/job/example/summary":
			writeJSON(t, w, json.RawMessage(`{"JobID":"example","Summary":{"web":{"Running":1,"Complete":2,"Failed":1}},"Children":{"Pending":1,"Running":2,"Dead":3}}`))
		case "/v1/job/example/allocations":
			writeJSON(t, w, json.RawMessage(`[
				{"ID":"allocation-1","TaskGroup":"web","NodeName":"node-a","ClientStatus":"complete","CreateTime":100},
				{"ID":"allocation-2","TaskGroup":"web","NodeName":"node-b","ClientStatus":"running","CreateTime":200},
				{"ID":"allocation-3","TaskGroup":"web","NodeName":"node-c","ClientStatus":"complete","CreateTime":300},
				{"ID":"allocation-4","TaskGroup":"web","NodeName":"node-d","ClientStatus":"complete","CreateTime":400},
				{"ID":"allocation-5","TaskGroup":"web","NodeName":"node-e","ClientStatus":"complete","CreateTime":500},
				{"ID":"allocation-6","TaskGroup":"web","NodeName":"node-f","ClientStatus":"running","CreateTime":600}
			]`))
		case "/v1/job/example/evaluations":
			writeJSON(t, w, json.RawMessage(`[
				{"ID":"evaluation-new","CreateTime":200,"FailedTGAllocs":{"web":{"NodesExhausted":2,"NodesAvailable":{"dc1":0}}}},
				{"ID":"evaluation-old","CreateTime":100,"FailedTGAllocs":{"old":{"NodesExhausted":9}}}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	detail, err := handler.getJobOverview(
		httptest.NewRequest(http.MethodGet, "/ui/jobs/example?namespace=team-a", nil),
		"example", "team-a", "browser-token",
	)
	if err != nil {
		t.Fatalf("get job overview: %v", err)
	}
	if detail.Allocations.Total() != 4 || detail.Allocations.Running != 1 || detail.Allocations.Unhealthy() != 1 {
		t.Fatalf("unexpected allocation summary: %#v", detail.Allocations)
	}
	if len(detail.Metadata) != 2 || detail.Metadata[0].Key != "alpha" || detail.Metadata[1].Key != "zeta" {
		t.Fatalf("metadata is not sorted: %#v", detail.Metadata)
	}
	checks := []struct {
		name  string
		valid bool
		value any
	}{
		{name: "recent allocations", valid: detail.RecentAllocationsAvailable && len(detail.RecentAllocations) == 5 && detail.RecentAllocations[0].ID == "allocation-6" && detail.RecentAllocations[4].ID == "allocation-2", value: detail.RecentAllocations},
		{name: "child-job summary", valid: detail.ChildJobs.Available && detail.ChildJobs.Total() == 6 && detail.ChildJobs.Running == 2, value: detail.ChildJobs},
		{name: "current placement failures", valid: detail.PlacementFailuresAvailable && len(detail.PlacementFailures) == 1 && detail.PlacementFailures[0].TaskGroup == "web" && !strings.Contains(detail.PlacementFailures[0].Reason, "9"), value: detail.PlacementFailures},
		{name: "scheduling and volumes", valid: detail.NodePool == "workers" && len(detail.Volumes) == 1 && detail.Volumes[0].Name == "cache" && detail.Volumes[0].ReadOnly && detail.Volumes[0].PerAllocation, value: detail},
		{name: "lifecycle", valid: len(detail.LifecycleTasks) == 1 && detail.LifecycleTasks[0].Task == "setup" && detail.LifecycleTasks[0].Hook == "prestart", value: detail.LifecycleTasks},
	}
	for _, check := range checks {
		if !check.valid {
			t.Errorf("unexpected %s projection: %#v", check.name, check.value)
		}
	}
}

func TestGetJobOverviewKeepsCoreDetailWhenOverviewReadsFail(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/job/example" {
			writeJSON(t, w, json.RawMessage(`{"ID":"example","Name":"Example","Status":"running","Type":"service","ParameterizedJob":{"Payload":"optional"}}`))
			return
		}
		http.Error(w, "overview unavailable", http.StatusServiceUnavailable)
	}))
	defer backend.Close()

	detail, err := NewJobsHandler(newJobsTestHandler(t, backend.URL)).getJobOverview(
		httptest.NewRequest(http.MethodGet, "/ui/jobs/example", nil), "example", "default", "",
	)
	if err != nil {
		t.Fatalf("overview read failures should not hide core detail: %v", err)
	}
	if detail == nil || detail.Name != "Example" {
		t.Fatalf("core detail was not preserved: %#v", detail)
	}
	if detail.Allocations.Available || detail.RecentAllocationsAvailable || detail.PlacementFailuresAvailable || detail.ChildJobs.Available {
		t.Fatalf("failed reads were reported as available: %#v", detail)
	}
}

func TestJobsRouteAPIStates(t *testing.T) {
	tests := []struct {
		name        string
		apiStatus   int
		apiBody     string
		permissions *UserPermissions
		wantStatus  int
		wantBody    string
	}{
		{
			name:       "success uses API job",
			apiStatus:  http.StatusOK,
			apiBody:    `[{"ID":"api-backed-job","Name":"API Backed Job","Status":"running","Type":"service","Priority":50}]`,
			wantStatus: http.StatusOK,
			wantBody:   "API Backed Job",
		},
		{
			name:       "empty API response renders empty state",
			apiStatus:  http.StatusOK,
			apiBody:    `[]`,
			wantStatus: http.StatusOK,
			wantBody:   "No jobs found",
		},
		{
			name:       "leaderless API renders unavailable state",
			apiStatus:  http.StatusInternalServerError,
			apiBody:    "No cluster leader",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "Cluster Unavailable",
		},
		{
			name:      "missing capability renders permission denied",
			apiStatus: http.StatusOK,
			apiBody:   `[]`,
			permissions: NewUserPermissions(&ACLTokenInfo{
				Token:      &api.ACLToken{Type: "client"},
				IsValid:    true,
				ACLEnabled: true,
			}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Access Denied",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/jobs" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.apiStatus)
				_, _ = w.Write([]byte(test.apiBody))
			}))
			defer backend.Close()

			handler := newJobsTestHandler(t, backend.URL)
			request := httptest.NewRequest(http.MethodGet, "/ui/jobs", nil)
			if test.permissions != nil {
				request = request.WithContext(WithPermissions(request.Context(), test.permissions))
			}
			response := httptest.NewRecorder()

			NewJobsHandler(handler).ServeHTTP(response, request)

			assertHTTPResponse(t, response, test.wantStatus, test.wantBody)
		})
	}
}

func TestJobsListRendersAndFiltersParityFieldsFromOneAPIRead(t *testing.T) {
	apiCalls := 0
	requestedMetadata := ""
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/jobs":
			apiCalls++
			requestedMetadata = r.URL.Query().Get("meta")
			_, _ = w.Write([]byte(`[
			{"ID":"web-api","Name":"Web API","Namespace":"default","Datacenters":["dc1"],"NodePool":"default","Status":"running","Type":"service","Priority":50,"Meta":{"pack.name":"web-pack","pack.version":"1.2.3"},"JobSummary":{"Summary":{"web":{"Running":2,"Starting":1,"Failed":1}}}},
			{"ID":"web-batch","Name":"Web Batch","Namespace":"default","Datacenters":["dc1"],"NodePool":"default","Status":"pending","Type":"batch","Priority":40,"JobSummary":{"Summary":{"batch":{"Queued":1}}}},
			{"ID":"other-job","Name":"Other Job","Namespace":"default","Datacenters":["dc2"],"NodePool":"gpu","Status":"dead","Type":"system","Priority":30}
		]`))
		case "/v1/namespaces":
			_, _ = w.Write([]byte(`[{"Name":"default"}]`))
		case "/v1/regions":
			_, _ = w.Write([]byte(`["global"]`))
		default:
			t.Errorf("unexpected API path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	target := "/ui/jobs?prefix=web&status=running&status=pending&type=service&type=batch&datacenter=dc1&node_pool=default&sort=name&dir=asc&per_page=25"
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()

	NewJobsHandler(handler).ServeHTTP(response, request)

	if apiCalls != 1 {
		t.Fatalf("jobs list API calls = %d, want 1", apiCalls)
	}
	if requestedMetadata != "true" {
		t.Fatalf("jobs list metadata field = %q, want true", requestedMetadata)
	}
	assertHTTPResponse(t, response, http.StatusOK, "Web API")
	body := response.Body.String()
	for _, value := range []string{
		"Web Batch", "Namespace", "Datacenter", "Node Pool", "Task Groups",
		"Allocations", "2 running / 4 total", "1 pending · 0 complete · 1 unhealthy",
		"Pack · web-pack", "Nomad Pack: web-pack 1.2.3",
		"status=running", "status=pending", "type=service", "type=batch",
		"Rows per page", `name="per_page"`, `value="25" selected`, "per_page=25",
	} {
		if !strings.Contains(body, value) {
			t.Errorf("response does not contain %q", value)
		}
	}
	if strings.Contains(body, "Other Job") {
		t.Fatal("response includes job excluded by composed filters")
	}
	if strings.Count(body, "Pack ·") != 1 {
		t.Fatalf("Pack markers = %d, want only the metadata-backed job", strings.Count(body, "Pack ·"))
	}
	if !containsURLValues(body, "per_page=25", "sort=name") {
		t.Fatal("job sort URL does not preserve per_page=25")
	}
}

func containsURLValues(body string, values ...string) bool {
	for _, attribute := range strings.Split(body, `hx-get="`) {
		url, _, _ := strings.Cut(attribute, `"`)
		if strings.HasPrefix(url, "/ui/jobs?") {
			matches := true
			for _, value := range values {
				matches = matches && strings.Contains(url, value)
			}
			if matches {
				return true
			}
		}
	}
	return false
}

func TestJobListFilterHelpers(t *testing.T) {
	filters := parseJobListFilters(url.Values{
		"status": {"running,pending", "running"},
		"type":   {"service", "batch"},
	})
	if strings.Join(filters.Statuses, ",") != "running,pending" {
		t.Fatalf("unexpected normalized statuses: %#v", filters.Statuses)
	}
	if strings.Join(filters.Types, ",") != "service,batch" {
		t.Fatalf("unexpected normalized types: %#v", filters.Types)
	}

	summary := summarizeJobAllocations(&api.JobSummary{Summary: map[string]api.TaskGroupSummary{
		"api":    {Running: 2, Starting: 1},
		"worker": {Complete: 3, Lost: 1},
	}})
	if !summary.Available || summary.TaskGroups != 2 || summary.Total() != 7 || summary.Unhealthy() != 1 {
		t.Fatalf("unexpected allocation summary: %#v", summary)
	}
	if summarizeJobAllocations(nil).Available {
		t.Fatal("missing allocation summary reported as available")
	}
}

func TestGetLiveJobDefinitionProvidesHCLAndJSON(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/job/example":
			_, _ = w.Write([]byte(`{"ID":"example","Name":"example","Version":3}`))
		case "/v1/job/example/submission":
			_, _ = w.Write([]byte(`{"Source":"job \"example\" {}","Format":"hcl2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	definition, err := NewJobsHandler(handler).getLiveJobDefinition(
		httptest.NewRequest(http.MethodGet, "/ui/jobs/example/definition", nil),
		"example",
	)
	if err != nil {
		t.Fatalf("get live job definition: %v", err)
	}
	if definition.Format != "hcl" || definition.HCLContent != `job "example" {}` {
		t.Fatalf("unexpected HCL definition: %#v", definition)
	}
	if !strings.Contains(definition.JSONContent, `"ID": "example"`) {
		t.Fatalf("JSON definition does not contain the live job: %s", definition.JSONContent)
	}
}

func TestScaleTaskGroupOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		count       string
		apiStatus   int
		permissions *UserPermissions
		wantStatus  int
		wantBody    string
		wantAPICall bool
	}{
		{
			name:        "success",
			count:       "2",
			apiStatus:   http.StatusOK,
			wantStatus:  http.StatusOK,
			wantBody:    "scaled to 2 instance(s)",
			wantAPICall: true,
		},
		{
			name:        "cluster rejects request",
			count:       "2",
			apiStatus:   http.StatusInternalServerError,
			wantStatus:  http.StatusBadGateway,
			wantBody:    "cluster rejected the scale request",
			wantAPICall: true,
		},
		{
			name:       "invalid lower bound",
			count:      "-1",
			apiStatus:  http.StatusOK,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid count value",
		},
		{
			name:      "permission denied",
			count:     "2",
			apiStatus: http.StatusOK,
			permissions: NewUserPermissions(&ACLTokenInfo{
				Token:      &api.ACLToken{Type: "client"},
				IsValid:    true,
				ACLEnabled: true,
			}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Permission denied",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apiCalled := false
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalled = true
				if r.Method != http.MethodPut || r.URL.Path != "/v1/job/example/scale" {
					t.Errorf("unexpected API request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.apiStatus)
				if test.apiStatus == http.StatusOK {
					_, _ = w.Write([]byte(`{"EvalID":"eval-1"}`))
				} else {
					_, _ = w.Write([]byte(`{"error":"scale failed"}`))
				}
			}))
			defer backend.Close()

			handler := newJobsTestHandler(t, backend.URL)
			request := httptest.NewRequest(
				http.MethodPost,
				"/ui/jobs/example/taskgroups/web/scale",
				strings.NewReader("count="+test.count),
			)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if test.permissions != nil {
				request = request.WithContext(WithPermissions(request.Context(), test.permissions))
			}
			response := httptest.NewRecorder()

			NewJobsHandler(handler).ServeHTTP(response, request)

			assertHTTPResponse(t, response, test.wantStatus, test.wantBody)
			if apiCalled != test.wantAPICall {
				t.Fatalf("API called = %t, want %t", apiCalled, test.wantAPICall)
			}
		})
	}
}

func assertHTTPResponse(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, wantStatus, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), wantBody) {
		t.Fatalf("body does not contain %q: %s", wantBody, response.Body.String())
	}
}

func TestCheckedUint64ToInt(t *testing.T) {
	if got, ok := checkedUint64ToInt(42); !ok || got != 42 {
		t.Fatalf("checkedUint64ToInt(42) = (%d, %t)", got, ok)
	}
	tooLarge := uint64(^uint(0)>>1) + 1
	if _, ok := checkedUint64ToInt(tooLarge); ok {
		t.Fatalf("checkedUint64ToInt(%d) unexpectedly succeeded", tooLarge)
	}
}

func newJobsTestHandler(t *testing.T, address string) *Handler {
	t.Helper()
	config := api.DefaultConfig()
	config.Address = address
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	handler := NewHandler(hclog.NewNullLogger())
	handler.nomadClient = client
	return handler
}
