package jobspartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestJobOverviewRendersLiveSummaryRecentAllocationsAndMetadata(t *testing.T) {
	job := JobDetail{
		ID:        "example",
		Name:      "example",
		Type:      "batch",
		NodePool:  "workers",
		AllAtOnce: true,
		Allocations: AllocationSummary{
			Available: true,
			Running:   1,
			Complete:  2,
			Failed:    1,
		},
		RecentAllocationsAvailable: true,
		RecentAllocations: []JobAllocation{{
			ID: "allocation-123456", ShortID: "allocati", TaskGroup: "web", NodeName: "node-a", Status: "running", Created: time.Now(),
		}},
		PlacementFailuresAvailable: true,
		PlacementFailures:          []PlacementFailure{{TaskGroup: "web", Reason: "2 nodes exhausted"}},
		ChildJobs:                  ChildJobSummary{Applicable: true, Available: true, Pending: 1, Running: 2, Dead: 3},
		PeriodicDetails:            &PeriodicDetails{Enabled: true, Specs: []string{"0 * * * *"}, TimeZone: "UTC", ProhibitOverlap: true},
		Parameterized:              &ParameterizedJob{Payload: "required", MetaRequired: []string{"environment"}},
		Multiregion:                []JobRegion{{Name: "west", Count: 2, Datacenters: []string{"dc1"}, NodePool: "workers"}},
		Volumes:                    []JobVolume{{TaskGroup: "web", Name: "cache", Type: "host", Source: "cache-data", ReadOnly: true, PerAllocation: true}},
		LifecycleTasks:             []JobLifecycleTask{{TaskGroup: "web", Task: "setup", Hook: "prestart"}},
		Metadata:                   []JobMetadata{{Key: "alpha", Value: "first"}, {Key: "zeta", Value: "last"}},
	}
	html := renderJobDetail(t, job)
	for _, expected := range []string{
		"Allocation Summary", "4", "Recent Allocations", "/ui/allocations/allocation-123456",
		"node-a", "Placement Failures", "2 nodes exhausted", "Child Jobs", "Scheduling", "workers",
		"All At Once", "Periodic Enabled", "Periodic Schedule", "0 * * * *", "Required Dispatch Metadata", "environment",
		"west", "Volumes", "cache-data", "read-only, per allocation", "Task Lifecycle", "prestart",
		"Metadata", "alpha", "first", "zeta", "last",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered overview does not contain %q", expected)
		}
	}
	if strings.Index(html, "alpha") > strings.Index(html, "zeta") {
		t.Fatal("metadata did not render in deterministic order")
	}
}

func TestJobOverviewRendersTruthfulEmptyAndUnavailableStates(t *testing.T) {
	unavailable := renderJobDetail(t, JobDetail{ID: "unavailable", Name: "unavailable"})
	for _, expected := range []string{"Allocation summary is temporarily unavailable", "Recent allocations are temporarily unavailable", "Placement status is temporarily unavailable", "No metadata configured"} {
		if !strings.Contains(unavailable, expected) {
			t.Errorf("unavailable overview does not contain %q", expected)
		}
	}
	if strings.Contains(unavailable, "This section will display job constraints") {
		t.Fatal("overview still renders the fabricated constraints placeholder")
	}
	parentUnavailable := renderJobDetail(t, JobDetail{ID: "parent", Name: "parent", ChildJobs: ChildJobSummary{Applicable: true}})
	if !strings.Contains(parentUnavailable, "Child-job totals are temporarily unavailable") {
		t.Fatal("parent overview does not render the child-job unavailable state")
	}

	empty := renderJobDetail(t, JobDetail{
		ID: "empty", Name: "empty", Allocations: AllocationSummary{Available: true},
		RecentAllocationsAvailable: true, PlacementFailuresAvailable: true,
	})
	for _, expectation := range []struct {
		text    string
		present bool
	}{
		{text: "This job has no allocations", present: true},
		{text: "temporarily unavailable", present: false},
		{text: "Child Jobs", present: false},
		{text: "Scheduling", present: false},
		{text: "Task Lifecycle", present: false},
		{text: ">Volumes<", present: false},
	} {
		if strings.Contains(empty, expectation.text) != expectation.present {
			t.Errorf("empty overview presence of %q = %t, want %t", expectation.text, !expectation.present, expectation.present)
		}
	}
}

func TestParameterizedJobRendersDispatchContract(t *testing.T) {
	job := JobDetail{
		ID:     "example",
		Name:   "example",
		Status: "running",
		Parameterized: &ParameterizedJob{
			Payload:      "required",
			MetaRequired: []string{"environment"},
			MetaOptional: []string{"version"},
		},
	}
	html := renderJobDetail(t, job)
	for _, expected := range []string{
		`>Dispatch<`,
		`hx-post="/api/jobs/example/dispatch"`,
		`name="meta.environment" required`,
		`name="meta.version"`,
		`name="payload"`,
		`maxlength="16384"`,
		`hx-post="/api/jobs/example/purge"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered detail does not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, ">Run<") {
		t.Fatalf("parameterized parent should offer Dispatch instead of Run: %s", html)
	}
}

func TestForbiddenPayloadOmitsPayloadField(t *testing.T) {
	job := JobDetail{
		ID:            "example",
		Name:          "example",
		Parameterized: &ParameterizedJob{Payload: "forbidden"},
	}
	html := renderJobDetail(t, job)
	if strings.Contains(html, `name="payload"`) {
		t.Fatalf("forbidden payload job rendered a payload field: %s", html)
	}
}

func TestPeriodicJobRendersForceLaunchContract(t *testing.T) {
	job := JobDetail{ID: "periodic", Name: "periodic", Status: "running", Periodic: true}
	html := renderJobDetail(t, job)
	for _, expected := range []string{
		`>Force Launch<`,
		`hx-post="/api/jobs/periodic/periodic/force"`,
		`Launch <strong class="text-neutral-900">periodic</strong> immediately`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered periodic detail does not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, ">Run<") || strings.Contains(html, ">Dispatch<") {
		t.Fatalf("periodic parent should offer Force Launch instead of Run or Dispatch: %s", html)
	}
}

func renderJobDetail(t *testing.T, job JobDetail) string {
	t.Helper()
	var output bytes.Buffer
	if err := DetailContent(job).Render(context.Background(), &output); err != nil {
		t.Fatalf("render job detail: %v", err)
	}
	return output.String()
}
