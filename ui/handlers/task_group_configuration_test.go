package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/ui/templates/partials/taskgrouppartials"
)

func TestTaskGroupDetailProjectsVolumesLifecycleAndRequestScope(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/job/example" {
			http.NotFound(w, r)
			return
		}
		assertScopedBrowserRequest(t, r)
		writeJSON(t, w, json.RawMessage(`{
			"ID":"example",
			"Namespace":"team-a",
			"TaskGroups":[{
				"Name":"web",
				"Count":2,
				"Volumes":{
					"cache":{"Type":"host","Source":"cache-data","ReadOnly":true,"PerAlloc":true},
					"archive":{"Type":"csi","Source":"archive-volume","AttachmentMode":"file-system","AccessMode":"single-node-reader-only"},
					"ignored":null
				},
				"Tasks":[
					{"Name":"prepare","Driver":"raw_exec","Lifecycle":{"Hook":"prestart","Sidecar":false},"Resources":{"CPU":10,"MemoryMB":8}},
					{"Name":"app","Driver":"raw_exec","Resources":{"CPU":50,"MemoryMB":32}}
				]
			}]
		}`))
	}))
	t.Cleanup(backend.Close)

	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	request := requestWithToken(http.MethodGet, "/ui/jobs/example@team-a/taskgroups/web?namespace=team-a&region=west")
	detail, err := handler.getLiveTaskGroupDetail(request, "example@team-a", "web")
	if err != nil {
		t.Fatalf("get task group detail: %v", err)
	}
	if len(detail.Volumes) != 2 || detail.Volumes[0].Name != "archive" || detail.Volumes[1].Name != "cache" {
		t.Fatalf("task group volumes are not deterministically projected: %#v", detail.Volumes)
	}
	if !detail.Volumes[1].ReadOnly || !detail.Volumes[1].PerAllocation {
		t.Fatalf("task group volume mode was not projected: %#v", detail.Volumes[1])
	}
	if len(detail.Tasks) != 2 || detail.Tasks[0].LifecycleHook != "prestart" || detail.Tasks[0].LifecycleSidecar {
		t.Fatalf("task lifecycle was not projected: %#v", detail.Tasks)
	}

	html := renderTaskGroupDetail(t, detail)
	for _, expected := range []string{
		"Configured Volumes",
		"archive-volume",
		"file-system",
		"single-node-reader-only",
		"Read-only, per allocation",
		`data-task-lifecycle="prepare"`,
		"Lifecycle:</span> <span class=\"font-medium text-neutral-900\">prestart",
		`data-task-lifecycle="app"`,
		"Lifecycle:</span> <span class=\"font-medium text-neutral-900\">Normal",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("task group detail does not contain %q", expected)
		}
	}
	assertOrderedHTML(t, html, "archive", "cache")
}

func TestTaskGroupDetailRendersTruthfulEmptyVolumes(t *testing.T) {
	html := renderTaskGroupDetail(t, taskgrouppartials.TaskGroupDetail{
		Name:  "web",
		Count: 1,
		Tasks: []taskgrouppartials.TaskDetail{{Name: "app", Driver: "raw_exec"}},
	})
	if !strings.Contains(html, "No configured volumes") {
		t.Fatalf("empty volume state missing: %s", html)
	}
	if strings.Contains(html, "prestart") || strings.Contains(html, "poststart") || strings.Contains(html, "poststop") {
		t.Fatalf("normal task invented a lifecycle hook: %s", html)
	}
}

func renderTaskGroupDetail(t *testing.T, detail taskgrouppartials.TaskGroupDetail) string {
	t.Helper()
	var output strings.Builder
	if err := taskgrouppartials.DetailContent("example@team-a", detail).Render(context.Background(), &output); err != nil {
		t.Fatalf("render task group detail: %v", err)
	}
	return output.String()
}
