package jobspartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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

func renderJobDetail(t *testing.T, job JobDetail) string {
	t.Helper()
	var output bytes.Buffer
	if err := DetailContent(job).Render(context.Background(), &output); err != nil {
		t.Fatalf("render job detail: %v", err)
	}
	return output.String()
}
