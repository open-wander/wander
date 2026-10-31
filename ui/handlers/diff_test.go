package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

func TestProjectVersionDiffCountsOnlySemanticChanges(t *testing.T) {
	diff := projectVersionDiff("example", 5, 2, &api.JobDiff{
		Type: "Edited",
		ID:   "example",
		Fields: []*api.FieldDiff{
			unchangedField("ID", "example"),
			{Type: "Edited", Name: "Priority", Old: "50", New: "75"},
			{Type: "Added", Name: "Meta[owner]", New: "platform"},
		},
		Objects: []*api.ObjectDiff{
			{Type: "None", Name: "Datacenters", Fields: []*api.FieldDiff{unchangedField("dc1", "dc1")}},
			{Type: "Added", Name: "Periodic"},
			{Type: "Edited", Name: "Multiregion", Fields: []*api.FieldDiff{{Type: "Edited", Name: "Strategy", Old: "failover", New: "all-at-once"}}},
		},
		TaskGroups: []*api.TaskGroupDiff{{
			Type: "Edited", Name: "web",
			Fields:  []*api.FieldDiff{unchangedField("Count", "1")},
			Objects: []*api.ObjectDiff{{Type: "Deleted", Name: "Service"}},
			Tasks: []*api.TaskDiff{{
				Type: "Edited", Name: "app",
				Fields: []*api.FieldDiff{{Type: "Edited", Name: "Config[image]", Old: "app:v1", New: "app:v2", Annotations: []string{"forces replacement"}}},
			}},
		}},
	})

	if diff.OldVersion != 2 || diff.NewVersion != 5 {
		t.Fatalf("unexpected version pair: %#v", diff)
	}
	if diff.Counts.Added != 2 || diff.Counts.Edited != 3 || diff.Counts.Deleted != 1 || diff.Counts.Total() != 6 {
		t.Fatalf("unexpected semantic counts: %#v", diff.Counts)
	}
	if len(diff.Changes) != 6 {
		t.Fatalf("changes = %d, want 6: %#v", len(diff.Changes), diff.Changes)
	}
	joined := make([]string, 0, len(diff.Changes))
	for _, change := range diff.Changes {
		joined = append(joined, change.Type+" "+change.Scope+" "+change.Name+" "+change.Old+" "+change.New)
	}
	output := strings.Join(joined, "\n")
	for _, expected := range []string{
		"Edited Job Priority 50 75",
		"Added Job Periodic",
		`Deleted Task group "web" Service`,
		`Edited Task group "web" / Task "app" Config[image] app:v1 app:v2`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("projected changes do not contain %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "Datacenters") || strings.Contains(output, "Count") {
		t.Fatalf("context-only records leaked into changes:\n%s", output)
	}
}

func TestJobVersionsUseStructuredAPIDifferences(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/job/example/versions" {
			t.Errorf("unexpected API path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("diffs") != "true" || r.URL.Query().Get("namespace") != "team-a" {
			t.Errorf("unexpected versions query: %s", r.URL.RawQuery)
		}
		assertRequestHeaderValue(t, r, "X-Nomad-Token", "browser-token")
		writeJSON(t, w, map[string]any{
			"Versions": []map[string]any{
				{"ID": "example", "Version": 5, "Stable": true, "SubmitTime": 500},
				{"ID": "example", "Version": 2, "Stable": false, "SubmitTime": 200},
			},
			"Diffs": []map[string]any{{
				"Type": "Edited", "ID": "example",
				"Fields": []map[string]any{{"Type": "Edited", "Name": "Priority", "Old": "50", "New": "75"}},
			}},
		})
	}))
	defer backend.Close()

	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	request := httptest.NewRequest(http.MethodGet, "/ui/jobs/example/versions?namespace=team-a", nil)
	request = request.WithContext(WithTokenContext(request.Context(), "browser-token"))
	versions, err := handler.getJobVersions(request, "example")
	if err != nil {
		t.Fatalf("get structured versions: %v", err)
	}
	if len(versions) != 2 || versions[0].Version != 5 || versions[0].Diff == nil || versions[0].Diff.OldVersion != 2 {
		t.Fatalf("unexpected projected versions: %#v", versions)
	}
	if versions[0].Diff.Counts.Edited != 1 || versions[1].Diff != nil {
		t.Fatalf("unexpected projected differences: %#v", versions)
	}
	var versionsHTML strings.Builder
	if err := jobspartials.VersionsTab(versions, "example").Render(context.Background(), &versionsHTML); err != nil {
		t.Fatalf("render versions tab: %v", err)
	}
	for _, expected := range []string{"1 changes", "Initial version", "/ui/jobs/example/versions/5/diff/2"} {
		if !strings.Contains(versionsHTML.String(), expected) {
			t.Errorf("versions tab does not contain %q", expected)
		}
	}

	response := httptest.NewRecorder()
	handler.VersionDiff(response, requestWithToken(http.MethodGet, "/ui/jobs/example/versions/5/diff/2?namespace=team-a"))
	assertHTTPResponse(t, response, http.StatusOK, "Priority")
	body := response.Body.String()
	for _, expected := range []string{"Version 5 vs Version 2", "1 changes", "Previous value", "50", "75"} {
		if !strings.Contains(body, expected) {
			t.Errorf("version diff response does not contain %q", expected)
		}
	}
}

func TestJobVersionsRejectInconsistentDiffResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Versions": []map[string]any{{"Version": 2}, {"Version": 1}},
			"Diffs":    []any{},
		})
	}))
	defer backend.Close()

	request := httptest.NewRequest(http.MethodGet, "/ui/jobs/example/versions", nil)
	_, err := NewJobsHandler(newJobsTestHandler(t, backend.URL)).getJobVersions(request, "example")
	if err == nil || !strings.Contains(err.Error(), "2 versions and 0 differences") {
		t.Fatalf("unexpected inconsistent-response error: %v", err)
	}
}

func requestWithToken(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	return request.WithContext(WithTokenContext(request.Context(), "browser-token"))
}

func unchangedField(name, value string) *api.FieldDiff {
	return &api.FieldDiff{Type: "None", Name: name, Old: value, New: value}
}
