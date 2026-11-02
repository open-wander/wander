package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/clientspartials"
)

func TestClientDetailProjectsAndFiltersScopedLiveData(t *testing.T) {
	backend := newClientDetailBackend(t, http.StatusOK)
	defer backend.Close()

	target := "/ui/clients/node-1?namespace=*&region=west&tab=allocations" +
		"&allocation_status=running&allocation_namespace=team-a" +
		"&allocation_job=alpha-job&allocation_task_group=web"
	response := serveClientRequest(t, backend.URL, http.MethodGet, target, "")

	assertHTTPResponse(t, response, http.StatusOK, "alpha-job")
	body := response.Body.String()
	for _, value := range []string{
		"Host Volumes", "models", "/srv/models", "Read only",
		"healthy and ready", "driver.version", "2.1.0",
		"Node Metadata", "Static metadata", "rack", "r1",
		"Dynamic metadata", "runtime.note", "ready", "Save metadata",
		"Default — force remaining allocations after 1 hour", "No deadline — wait indefinitely",
		"Force immediately", "Operator message", "Keep the node ineligible after cancelling",
		`name="allocation_status"`, `name="allocation_namespace"`,
		`name="allocation_job"`, `name="allocation_task_group"`,
		"namespace=team-a", "region=west",
	} {
		if !strings.Contains(body, value) {
			t.Errorf("client detail does not contain %q", value)
		}
	}
	if strings.Contains(body, `data-alloc-id="alloc-b"`) {
		t.Fatal("client detail includes an allocation excluded by composed filters")
	}
}

func TestClientMetadataSetAndUnsetPreserveScope(t *testing.T) {
	tests := []struct {
		name      string
		form      string
		wantValue *string
	}{
		{name: "set", form: "operation=set&key=runtime.note&value=ready", wantValue: stringPointer("ready")},
		{name: "unset", form: "operation=unset&key=runtime.note", wantValue: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/client/metadata" {
					t.Fatalf("unexpected metadata request: %s %s", r.Method, r.URL.String())
				}
				assertClientDetailScope(t, r, "", "west")
				var request api.NodeMetaApplyRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode metadata request: %v", err)
				}
				if request.NodeID != "node-1" {
					t.Fatalf("metadata node ID = %q", request.NodeID)
				}
				value, ok := request.Meta["runtime.note"]
				if !ok || (test.wantValue == nil && value != nil) || (test.wantValue != nil && (value == nil || *value != *test.wantValue)) {
					t.Fatalf("metadata update = %#v, want %#v", request.Meta, test.wantValue)
				}
				writeJSON(t, w, &api.NodeMetaResponse{})
			}))
			defer backend.Close()

			response := serveClientRequest(t, backend.URL, http.MethodPost, "/ui/clients/node-1/metadata?namespace=team-a&region=west", test.form)
			if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/ui/clients/node-1?namespace=team-a&region=west" {
				t.Fatalf("metadata response = %d redirect %q: %s", response.Code, response.Header().Get("Location"), response.Body.String())
			}
		})
	}
}

func TestClientMetadataRejectsMalformedAndUpstreamFailures(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer backend.Close()
	handler := NewClientsHandler(newJobsTestHandler(t, backend.URL))

	for _, test := range []struct {
		name       string
		form       string
		wantStatus int
	}{
		{name: "missing key", form: "operation=set&value=x", wantStatus: http.StatusBadRequest},
		{name: "unknown operation", form: "operation=replace&key=x", wantStatus: http.StatusBadRequest},
		{name: "upstream denied", form: "operation=set&key=x&value=y", wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := requestWithToken(http.MethodPost, "/ui/clients/node-1/metadata?region=west")
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Body = io.NopCloser(strings.NewReader(test.form))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("metadata response status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestClientDetailDistinguishesDeniedAndUnavailableAllocations(t *testing.T) {
	assertClientDetailAllocationFailure(t, http.StatusForbidden, "do not have permission")
	assertClientDetailAllocationFailure(t, http.StatusServiceUnavailable, "could not read")
}

func TestClientDetailKeepsMergedMetadataWhenDynamicReadIsUnavailable(t *testing.T) {
	backend := newClientDetailBackend(t, http.StatusOK, http.StatusServiceUnavailable)
	defer backend.Close()
	response := serveClientRequest(t, backend.URL, http.MethodGet, "/ui/clients/node-1?namespace=*&region=west", "")
	assertHTTPResponse(t, response, http.StatusOK, "Dynamic metadata is unavailable")
	if !strings.Contains(response.Body.String(), "Merged metadata") {
		t.Fatal("client detail did not retain the merged metadata fallback")
	}
}

func TestClientDetailDistinguishesMissingDeniedAndUnavailableNode(t *testing.T) {
	assertClientDetailNodeFailure(t, http.StatusNotFound, http.StatusNotFound, "not found")
	assertClientDetailNodeFailure(t, http.StatusForbidden, http.StatusForbidden, "Access Denied")
	assertClientDetailNodeFailure(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "Cluster Unavailable")
}

func TestClientAllocationFiltersKeepCompleteOptions(t *testing.T) {
	allocations := clientDetailAllocationFixtures()
	options := extractClientAllocationFilterOptions(allocations)
	if len(options.Statuses) != 2 || len(options.Namespaces) != 2 || len(options.Jobs) != 2 || len(options.TaskGroups) != 2 {
		t.Fatalf("incomplete allocation filter options: %#v", options)
	}
	filtered := filterClientAllocations(allocations, clientspartials.ClientAllocationFilters{
		Status: "running", Namespace: "team-a", Job: "alpha-job", TaskGroup: "web",
	})
	if len(filtered) != 1 || filtered[0].ID != "alloc-a" {
		t.Fatalf("filtered allocations = %#v", filtered)
	}
}

func TestClientDrainPreservesRegionScope(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/node/node-1/drain" {
			t.Fatalf("unexpected drain request: %s %s", r.Method, r.URL.String())
		}
		assertClientDetailScope(t, r, "", "west")
		var request api.NodeUpdateDrainRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode drain request: %v", err)
		}
		if request.DrainSpec == nil || request.DrainSpec.Deadline.String() != "30m0s" || !request.DrainSpec.IgnoreSystemJobs {
			t.Fatalf("unexpected drain strategy: %#v", request.DrainSpec)
		}
		if request.Meta["message"] != "maintenance" {
			t.Fatalf("drain metadata = %#v", request.Meta)
		}
		writeJSON(t, w, &api.NodeDrainUpdateResponse{})
	}))
	defer backend.Close()

	response := serveClientRequest(t, backend.URL, http.MethodPost, "/ui/clients/node-1/drain?namespace=team-a&region=west", "deadline=30m&ignore_system=true&message=maintenance")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/ui/clients/node-1?namespace=team-a&region=west" {
		t.Fatalf("drain response = %d redirect %q", response.Code, response.Header().Get("Location"))
	}
}

func TestParseClientDrainStrategies(t *testing.T) {
	tests := []struct {
		name         string
		form         url.Values
		wantDeadline time.Duration
		wantError    bool
	}{
		{name: "default", form: url.Values{}, wantDeadline: time.Hour},
		{name: "legacy custom", form: url.Values{"deadline": {"30m"}}, wantDeadline: 30 * time.Minute},
		{name: "custom", form: url.Values{"deadline_mode": {"custom"}, "deadline": {"2h30m"}}, wantDeadline: 150 * time.Minute},
		{name: "none", form: url.Values{"deadline_mode": {"none"}}, wantDeadline: 0},
		{name: "force", form: url.Values{"deadline_mode": {"force"}}, wantDeadline: -time.Second},
		{name: "invalid custom", form: url.Values{"deadline_mode": {"custom"}, "deadline": {"soon"}}, wantError: true},
		{name: "invalid mode", form: url.Values{"deadline_mode": {"later"}}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec, _, err := parseDrainStrategy(test.form)
			if test.wantError {
				if err == nil {
					t.Fatalf("parseDrainStrategy(%v) unexpectedly succeeded", test.form)
				}
				return
			}
			if err != nil || spec.Deadline != test.wantDeadline {
				t.Fatalf("parseDrainStrategy(%v) = %#v, %v", test.form, spec, err)
			}
		})
	}
}

func TestClientCancelDrainCanKeepNodeIneligible(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertClientDetailScope(t, r, "", "west")
		var request api.NodeUpdateDrainRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode cancel drain request: %v", err)
		}
		if request.DrainSpec != nil || request.MarkEligible {
			t.Fatalf("cancel drain strategy = %#v", request)
		}
		if request.Meta["cancel_message"] != "maintenance postponed" {
			t.Fatalf("cancel metadata = %#v", request.Meta)
		}
		writeJSON(t, w, &api.NodeDrainUpdateResponse{})
	}))
	defer backend.Close()

	response := serveClientRequest(t, backend.URL, http.MethodPost, "/ui/clients/node-1/drain/cancel?namespace=team-a&region=west", "keep_ineligible=true&cancel_message=maintenance+postponed")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/ui/clients/node-1?namespace=team-a&region=west" {
		t.Fatalf("cancel response = %d redirect %q", response.Code, response.Header().Get("Location"))
	}
}

func assertClientDetailAllocationFailure(t *testing.T, status int, wantText string) {
	t.Helper()
	backend := newClientDetailBackend(t, status)
	defer backend.Close()
	response := serveClientRequest(t, backend.URL, http.MethodGet, "/ui/clients/node-1?namespace=*&region=west&tab=allocations", "")
	assertHTTPResponse(t, response, http.StatusOK, "worker-1")
	if !strings.Contains(response.Body.String(), wantText) {
		t.Fatalf("allocation failure does not contain %q: %s", wantText, response.Body.String())
	}
}

func assertClientDetailNodeFailure(t *testing.T, backendStatus, wantStatus int, wantText string) {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(backendStatus)
	}))
	defer backend.Close()
	response := serveClientRequest(t, backend.URL, http.MethodGet, "/ui/clients/node-1?region=west", "")
	assertHTTPResponse(t, response, wantStatus, wantText)
}

func newClientDetailBackend(t *testing.T, allocationStatus int, metadataStatuses ...int) *httptest.Server {
	t.Helper()
	metadataStatus := http.StatusOK
	if len(metadataStatuses) > 0 {
		metadataStatus = metadataStatuses[0]
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/node/node-1":
			assertClientDetailScope(t, r, "", "west")
			writeJSON(t, w, &api.Node{
				ID: "node-1", Name: "worker-1", Datacenter: "dc1", HTTPAddr: "127.0.0.1:4646",
				Status: api.NodeStatusReady, SchedulingEligibility: api.NodeSchedulingEligible,
				Attributes: map[string]string{"nomad.version": "1.6.5"}, Meta: map[string]string{"rack": "r1"},
				HostVolumes: map[string]*api.HostVolumeInfo{
					"scratch": {Path: "/tmp/scratch"},
					"models":  {Path: "/srv/models", ReadOnly: true},
				},
				Drivers: map[string]*api.DriverInfo{
					"raw_exec": {Detected: true, Healthy: true, HealthDescription: "healthy and ready", Attributes: map[string]string{"driver.version": "2.1.0", "zeta": "last"}},
				},
			})
		case "/v1/node/node-1/allocations":
			assertClientDetailScope(t, r, "*", "west")
			if allocationStatus != http.StatusOK {
				w.WriteHeader(allocationStatus)
				return
			}
			writeJSON(t, w, []*api.Allocation{
				{ID: "alloc-b", Namespace: "team-b", JobID: "beta-job", TaskGroup: "batch", ClientStatus: api.AllocClientStatusComplete},
				{ID: "alloc-a", Namespace: "team-a", JobID: "alpha-job", TaskGroup: "web", ClientStatus: api.AllocClientStatusRunning},
			})
		case "/v1/client/stats":
			assertClientDetailScope(t, r, "", "west")
			writeJSON(t, w, &api.HostStats{})
		case "/v1/client/metadata":
			assertClientDetailScope(t, r, "", "west")
			if metadataStatus != http.StatusOK {
				w.WriteHeader(metadataStatus)
				return
			}
			if r.URL.Query().Get("node_id") != "node-1" {
				t.Errorf("metadata node ID = %q", r.URL.Query().Get("node_id"))
			}
			value := "ready"
			writeJSON(t, w, &api.NodeMetaResponse{
				Meta:    map[string]string{"rack": "r1", "runtime.note": value},
				Static:  map[string]string{"rack": "r1"},
				Dynamic: map[string]*string{"runtime.note": &value},
			})
		case "/v1/namespaces":
			writeJSON(t, w, []*api.Namespace{{Name: "default"}, {Name: "team-a"}})
		case "/v1/regions":
			writeJSON(t, w, []string{"global", "west"})
		default:
			t.Errorf("unexpected client detail API request: %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
}

func stringPointer(value string) *string {
	return &value
}

func serveClientRequest(t *testing.T, backendURL, method, target, form string) *httptest.ResponseRecorder {
	t.Helper()
	request := requestWithToken(method, target)
	if form != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Body = io.NopCloser(strings.NewReader(form))
	}
	response := httptest.NewRecorder()
	NewClientsHandler(newJobsTestHandler(t, backendURL)).ServeHTTP(response, request)
	return response
}

func assertClientDetailScope(t *testing.T, r *http.Request, namespace, region string) {
	t.Helper()
	if r.Header.Get("X-Nomad-Token") != "browser-token" {
		t.Errorf("%s token = %q", r.URL.Path, r.Header.Get("X-Nomad-Token"))
	}
	if r.URL.Query().Get("region") != region {
		t.Errorf("%s region = %q, want %q", r.URL.Path, r.URL.Query().Get("region"), region)
	}
	if namespace != "" && r.URL.Query().Get("namespace") != namespace {
		t.Errorf("%s namespace = %q, want %q", r.URL.Path, r.URL.Query().Get("namespace"), namespace)
	}
}

func clientDetailAllocationFixtures() []clientspartials.AllocationSummary {
	return []clientspartials.AllocationSummary{
		{ID: "alloc-a", Namespace: "team-a", JobID: "alpha-job", TaskGroup: "web", Status: "running"},
		{ID: "alloc-b", Namespace: "team-b", JobID: "beta-job", TaskGroup: "batch", Status: "complete"},
	}
}
