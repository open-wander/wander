package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

func TestEvaluationRoutesPreserveLiveStatesAndSelections(t *testing.T) {
	created := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)
	handler := stateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "west" {
			t.Errorf("selection query = %q, want namespace=team-a and region=west", r.URL.RawQuery)
		}
		switch r.URL.Path {
		case "/v1/evaluations":
			writeJSON(t, w, []*api.Evaluation{{
				ID: "blocked-evaluation", JobID: "example", Namespace: "team-a", Status: api.EvalStatusBlocked,
				StatusDescription: "waiting for capacity", Type: "service", TriggeredBy: "job-register", CreateTime: created.UnixNano(),
			}})
		case "/v1/evaluation/blocked-evaluation":
			cpu := 500
			writeJSON(t, w, &api.Evaluation{
				ID: "blocked-evaluation", JobID: "example", Namespace: "team-a", Status: api.EvalStatusBlocked,
				StatusDescription: "waiting for capacity", Type: "service", TriggeredBy: "job-register", CreateTime: created.UnixNano(),
				FailedTGAllocs: map[string]*api.AllocationMetric{"web": {NodesEvaluated: 1, ResourcesExhausted: map[string]*api.Resources{"cpu": {CPU: &cpu}}}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	listBody := serveStateRequest(t, handler, "/ui/evaluations?namespace=team-a&region=west", http.StatusOK)
	assertContainsAll(t, listBody, "blocked-evaluation", "blocked")
	detailBody := serveStateRequest(t, handler, "/ui/evaluations/blocked-evaluation?namespace=team-a&region=west", http.StatusOK)
	assertContainsAll(t, detailBody, "waiting for capacity", "Placement Failures", "web")
}

func TestEvaluationDetailRendersTerminalStatuses(t *testing.T) {
	for _, status := range []string{api.EvalStatusComplete, api.EvalStatusFailed, api.EvalStatusBlocked} {
		t.Run(status, func(t *testing.T) {
			handler := stateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, &api.Evaluation{ID: status + "-evaluation", JobID: "example", Status: status, StatusDescription: status + " state"})
			})
			body := serveStateRequest(t, handler, "/ui/evaluations/"+status+"-evaluation", http.StatusOK)
			assertContainsAll(t, body, status, status+" state")
		})
	}
}

func TestDeploymentDetailRendersSupportedStatusesAndNilTaskGroups(t *testing.T) {
	for _, status := range []string{
		api.DeploymentStatusRunning, api.DeploymentStatusSuccessful, api.DeploymentStatusFailed,
		api.DeploymentStatusCancelled, api.DeploymentStatusBlocked,
	} {
		t.Run(status, func(t *testing.T) {
			handler := stateTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "west" {
					t.Errorf("selection query = %q, want namespace=team-a and region=west", r.URL.RawQuery)
				}
				writeJSON(t, w, &api.Deployment{
					ID: status + "-deployment", JobID: "example", Namespace: "team-a", Status: status,
					StatusDescription: status + " state", TaskGroups: map[string]*api.DeploymentState{"missing": nil, "web": {DesiredTotal: 2, PlacedAllocs: 2}},
				})
			})
			body := serveStateRequest(t, handler, "/ui/deployments/"+status+"-deployment?namespace=team-a&region=west", http.StatusOK)
			assertContainsAll(t, body, status, status+" state", "web")
		})
	}
}

func TestEvaluationAndDeploymentRoutesPreserveTruthfulErrors(t *testing.T) {
	for _, route := range []string{"evaluations", "deployments"} {
		for _, test := range []struct {
			name       string
			suffix     string
			apiStatus  int
			wantStatus int
		}{
			{name: "empty", apiStatus: http.StatusOK, wantStatus: http.StatusOK},
			{name: "list denied", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
			{name: "list unavailable", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
			{name: "detail missing", suffix: "/missing", apiStatus: http.StatusNotFound, wantStatus: http.StatusNotFound},
			{name: "detail denied", suffix: "/missing", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden},
			{name: "detail unavailable", suffix: "/missing", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusServiceUnavailable},
		} {
			t.Run(route+" "+test.name, func(t *testing.T) {
				handler := stateTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
					if test.apiStatus == http.StatusOK {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte("[]"))
						return
					}
					http.Error(w, fmt.Sprintf("backend status %d", test.apiStatus), test.apiStatus)
				})
				body := serveStateRequest(t, handler, "/ui/"+route+test.suffix, test.wantStatus)
				if test.apiStatus != http.StatusOK && strings.Contains(body, "No "+route) {
					t.Fatalf("backend failure was rendered as an empty %s list", route)
				}
			})
		}
	}
}

func TestEvaluationAndDeploymentRoutesRejectInventedSubroutes(t *testing.T) {
	handler := stateTestHandler(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid nested route reached the backend")
	})
	serveStateRequest(t, handler, "/ui/evaluations/example/allocations", http.StatusNotFound)
	serveStateRequest(t, handler, "/ui/deployments/example/allocations", http.StatusNotFound)
}

func stateTestHandler(t *testing.T, backend http.HandlerFunc) http.Handler {
	t.Helper()
	server := httptest.NewServer(backend)
	t.Cleanup(server.Close)
	base := newJobsTestHandler(t, server.URL)
	evaluationsHandler := NewEvaluationsHandler(base)
	deploymentsHandler := NewDeploymentsHandler(base)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ui/evaluations"):
			evaluationsHandler.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/ui/deployments"):
			deploymentsHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func serveStateRequest(t *testing.T, handler http.Handler, path string, wantStatus int) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, wantStatus, response.Body.String())
	}
	return response.Body.String()
}
