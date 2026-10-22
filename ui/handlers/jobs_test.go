package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
)

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
