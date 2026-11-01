package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllocationRestartTargetsAndScope(t *testing.T) {
	tests := []struct {
		name, target string
		wantTask     string
		wantAll      bool
	}{
		{name: "running tasks", target: ""},
		{name: "selected task", target: "&task=fixture", wantTask: "fixture"},
		{name: "all tasks", target: "&all_tasks=true", wantAll: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restarts := make(chan allocationRestartRequest, 1)
			backend := newAllocationRestartBackend(t, restarts)
			defer backend.Close()
			handler := NewAllocationsHandler(newJobsTestHandler(t, backend.URL))
			request := requestWithToken(http.MethodPost,
				"/ui/allocations/alloc-1/restart?namespace=team-a&region=west"+test.target)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("restart status=%d body=%q", response.Code, response.Body.String())
			}
			got := <-restarts
			if got.TaskName != test.wantTask || got.AllTasks != test.wantAll {
				t.Fatalf("restart request=%#v, want task=%q all=%t", got, test.wantTask, test.wantAll)
			}
		})
	}
}

func TestAllocationRestartRejectsConflictingTargets(t *testing.T) {
	handler := NewAllocationsHandler(newJobsTestHandler(t, "http://127.0.0.1:1"))
	request := requestWithToken(http.MethodPost,
		"/ui/allocations/alloc-1/restart?task=fixture&all_tasks=true")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("conflicting restart status=%d, want %d", response.Code, http.StatusBadRequest)
	}
}

type allocationRestartRequest struct {
	TaskName string
	AllTasks bool
}

func newAllocationRestartBackend(t *testing.T, restarts chan<- allocationRestartRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/allocation/alloc-1":
			writeJSON(t, w, map[string]any{"ID": "alloc-1"})
		case "/v1/client/allocation/alloc-1/restart":
			if r.Method != http.MethodPut {
				t.Fatalf("restart method=%s, want PUT", r.Method)
			}
			var request allocationRestartRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode restart request: %v", err)
			}
			restarts <- request
			writeJSON(t, w, map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
}
