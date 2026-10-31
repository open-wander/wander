package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

func TestJobHistoryTabsPreserveScopeOrderAndSchedulerState(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Nomad-Token") != "browser-token" {
			t.Errorf("token header = %q, want browser-token", r.Header.Get("X-Nomad-Token"))
		}
		assertMonitorQueryValue(t, r, "namespace", "team-a")
		assertMonitorQueryValue(t, r, "region", "west")

		switch r.URL.Path {
		case "/v1/job/example/deployments":
			assertMonitorQueryValue(t, r, "all", "true")
			writeJSON(t, w, json.RawMessage(`[
				{"ID":"deployment-old","JobVersion":1,"Status":"failed","StatusDescription":"old failure","CreateIndex":10},
				{"ID":"deployment-new","JobVersion":2,"Status":"successful","StatusDescription":"healthy rollout","CreateIndex":20,
				 "TaskGroups":{"zeta":{"DesiredTotal":2,"PlacedAllocs":2,"HealthyAllocs":2},"alpha":{"DesiredCanaries":1,"DesiredTotal":1,"PlacedAllocs":1,"HealthyAllocs":1,"Promoted":true}}}
			]`))
		case "/v1/job/example/evaluations":
			writeJSON(t, w, json.RawMessage(`[
				{"ID":"evaluation-old","Priority":40,"Status":"complete","TriggeredBy":"job-register","CreateIndex":10,"CreateTime":100},
				{"ID":"evaluation-new","Priority":75,"Status":"blocked","StatusDescription":"waiting for capacity","TriggeredBy":"job-register","CreateIndex":20,"CreateTime":200,
				 "FailedTGAllocs":{"web":{"NodesExhausted":2,"NodesAvailable":{"dc1":0}}}}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(backend.Close)

	handler, request := newJobHistoryTestRequest(t, backend.URL)

	deployments, err := handler.getJobDeployments(request, "example")
	if err != nil {
		t.Fatalf("get deployments: %v", err)
	}
	if len(deployments) != 2 || deployments[0].ID != "deployment-new" || deployments[1].ID != "deployment-old" {
		t.Fatalf("deployments are not newest first: %#v", deployments)
	}
	if got := deployments[0].TaskGroups; len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("deployment task groups are not sorted: %#v", got)
	}

	evaluations, err := handler.getJobEvaluations(request, "example")
	if err != nil {
		t.Fatalf("get evaluations: %v", err)
	}
	if len(evaluations) != 2 || evaluations[0].ID != "evaluation-new" || evaluations[0].Priority != 75 || evaluations[0].Status != "blocked" {
		t.Fatalf("unexpected evaluations: %#v", evaluations)
	}
	if len(evaluations[0].PlacementFailures) != 1 || !strings.Contains(evaluations[0].PlacementFailures[0].Reason, "2 nodes exhausted") {
		t.Fatalf("placement failure was not projected: %#v", evaluations[0].PlacementFailures)
	}

	var deploymentHTML strings.Builder
	if err := jobspartials.DeploymentsTab(deployments, "example").Render(context.Background(), &deploymentHTML); err != nil {
		t.Fatalf("render deployment tab: %v", err)
	}
	assertOrderedHTML(t, deploymentHTML.String(), "deployment-new", "deployment-old")
	assertOrderedHTML(t, deploymentHTML.String(), "alpha", "zeta")
	for _, expected := range []string{"/ui/deployments/deployment-new", "namespace=team-a", "region=west", "successful", "Time unavailable"} {
		if !strings.Contains(deploymentHTML.String(), expected) {
			t.Errorf("deployment tab does not contain %q", expected)
		}
	}

	var evaluationHTML strings.Builder
	if err := jobspartials.EvaluationsTab(evaluations, "example").Render(context.Background(), &evaluationHTML); err != nil {
		t.Fatalf("render evaluation tab: %v", err)
	}
	assertOrderedHTML(t, evaluationHTML.String(), "evaluation-new", "evaluation-old")
	for _, expected := range []string{"/ui/evaluations/evaluation-new", "namespace=team-a", "region=west", "Priority: 75", "blocked", "Placement Failures"} {
		if !strings.Contains(evaluationHTML.String(), expected) {
			t.Errorf("evaluation tab does not contain %q", expected)
		}
	}
}

func TestJobHistoryTabFailuresAreNotRenderedAsEmptyHistory(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "scheduler unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(backend.Close)

	handler, request := newJobHistoryTestRequest(t, backend.URL)
	jobDetail := &jobspartials.JobDetail{ID: "example", Name: "example"}

	for _, tab := range []string{"deployments", "evaluations"} {
		t.Run(tab, func(t *testing.T) {
			component, err := handler.jobTabContent(request, "example", tab, jobDetail)
			if err == nil || component != nil {
				t.Fatalf("jobTabContent(%q) = (%v, %v), want nil component and error", tab, component, err)
			}
		})
	}
}

func assertOrderedHTML(t *testing.T, body, first, second string) {
	t.Helper()
	firstIndex := strings.Index(body, first)
	secondIndex := strings.Index(body, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("expected %q before %q in rendered HTML", first, second)
	}
}

func newJobHistoryTestRequest(t *testing.T, address string) (*JobsHandler, *http.Request) {
	t.Helper()
	return NewJobsHandler(newJobsTestHandler(t, address)), requestWithToken(
		http.MethodGet,
		"/ui/jobs/example?namespace=team-a&region=west",
	)
}
