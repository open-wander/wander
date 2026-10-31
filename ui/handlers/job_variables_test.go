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

func TestJobVariablesPreserveScopeAndEnforceJobPathBoundary(t *testing.T) {
	handler, request := newJobVariablesTestContext(t, "example")

	variableList := mustGetJobVariables(t, handler, request, "example@team-a", 3)
	if variableList[0].Path != "nomad/jobs/example" || variableList[1].Path != "nomad/jobs/example/group/api" || variableList[2].Path != "nomad/jobs/example/group/web" {
		t.Fatalf("job variables are not deterministically sorted: %#v", variableList)
	}
	if variableList[0].RelativePath != "" || variableList[1].RelativePath != "group/api" {
		t.Fatalf("unexpected relative paths: %#v", variableList)
	}

	html := renderJobVariablesTab(t, variableList)
	for _, expected := range []string{
		"Job root",
		"group/api",
		"nomad/jobs/example/group/web",
		"/ui/variables/nomad%2Fjobs%2Fexample%2Fgroup%2Fapi",
		"namespace=team-a",
		"region=west",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("variables tab does not contain %q", expected)
		}
	}
	for _, excluded := range []string{"example-other", "unrelated"} {
		if strings.Contains(html, excluded) {
			t.Fatalf("variables tab crossed the job path boundary with %q: %s", excluded, html)
		}
	}
	assertOrderedHTML(t, html, "group/api", "group/web")

	if _, tab := splitJobDetailPath("example@team-a/variables"); tab != "variables" {
		t.Fatalf("job variables deep link parsed as tab %q", tab)
	}
}

func TestJobVariablesRenderTruthfulEmptyDeniedAndUnavailableStates(t *testing.T) {
	handler, emptyRequest := newJobVariablesTestContext(t, "empty")
	empty := mustGetJobVariables(t, handler, emptyRequest, "empty", 0)
	emptyHTML := renderJobVariablesTab(t, empty)
	if !strings.Contains(emptyHTML, "No linked variables") {
		t.Fatalf("empty variables state missing: %s", emptyHTML)
	}

	for _, test := range []struct {
		job        string
		wantStatus int
		wantBody   string
	}{
		{job: "denied", wantStatus: http.StatusForbidden, wantBody: "variables: list"},
		{job: "unavailable", wantStatus: http.StatusServiceUnavailable, wantBody: "Cluster Unavailable"},
	} {
		t.Run(test.job, func(t *testing.T) {
			request := requestWithToken(http.MethodGet, "/ui/jobs/"+test.job+"/variables?namespace=team-a&region=west")
			_, fetchErr := handler.getJobVariables(request, test.job)
			if fetchErr == nil {
				t.Fatal("getJobVariables unexpectedly succeeded")
			}
			response := httptest.NewRecorder()
			handler.renderJobTabError(response, request, fetchErr, "variables", test.job)
			assertHTTPResponse(t, response, test.wantStatus, test.wantBody)
		})
	}
}

func newJobVariablesTestContext(t *testing.T, job string) (*JobsHandler, *http.Request) {
	t.Helper()
	backend := newJobVariablesBackend(t)
	t.Cleanup(backend.Close)
	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/jobs/"+job+"/variables?namespace=team-a&region=west")
	return handler, request
}

func renderJobVariablesTab(t *testing.T, variables []jobspartials.JobVariable) string {
	t.Helper()
	var body strings.Builder
	if err := jobspartials.VariablesTab(variables).Render(context.Background(), &body); err != nil {
		t.Fatalf("render variables tab: %v", err)
	}
	return body.String()
}

func mustGetJobVariables(t *testing.T, handler *JobsHandler, request *http.Request, job string, wantCount int) []jobspartials.JobVariable {
	t.Helper()
	variables, err := handler.getJobVariables(request, job)
	if err != nil {
		t.Fatalf("get job variables: %v", err)
	}
	if len(variables) != wantCount {
		t.Fatalf("job variables = %#v, want %d entries", variables, wantCount)
	}
	return variables
}

func newJobVariablesBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vars" {
			http.NotFound(w, r)
			return
		}
		assertScopedBrowserRequest(t, r)
		prefix := r.URL.Query().Get("prefix")
		switch prefix {
		case "nomad/jobs/example":
			writeJSON(t, w, json.RawMessage(`[
				{"Path":"nomad/jobs/example/group/web","Namespace":"team-a","CreateIndex":30,"ModifyIndex":31,"ModifyTime":300},
				{"Path":"nomad/jobs/example-other","Namespace":"team-a","CreateIndex":40,"ModifyIndex":41,"ModifyTime":400},
				{"Path":"nomad/jobs/example","Namespace":"team-a","CreateIndex":10,"ModifyIndex":11,"ModifyTime":100},
				null,
				{"Path":"unrelated","Namespace":"team-a","CreateIndex":50,"ModifyIndex":51,"ModifyTime":500},
				{"Path":"nomad/jobs/example/group/api","Namespace":"team-a","CreateIndex":20,"ModifyIndex":21,"ModifyTime":200}
			]`))
		case "nomad/jobs/empty":
			writeJSON(t, w, json.RawMessage(`[]`))
		case "nomad/jobs/denied":
			http.Error(w, "permission denied", http.StatusForbidden)
		case "nomad/jobs/unavailable":
			http.Error(w, "no cluster leader", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
}
