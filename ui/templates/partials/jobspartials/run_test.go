package jobspartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunContentExposesHCLVariablesInput(t *testing.T) {
	var output bytes.Buffer
	if err := RunContent("team-a", `job "example" {}`).Render(context.Background(), &output); err != nil {
		t.Fatalf("render run content: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		"HCL Variables",
		`id="job-variables"`,
		`x-model="variables"`,
		`x-show="format === 'hcl'"`,
		"HCL2 variable-file content",
		`data-namespace="team-a"`,
		`job &#34;example&#34; {}`,
		`/ui/jobs/run/templates?namespace=team-a`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered run content does not contain %q: %s", expected, html)
		}
	}
}

func TestRunContentDoesNotExposeBrowserOnlyTemplates(t *testing.T) {
	var output bytes.Buffer
	if err := RunContent("default", "").Render(context.Background(), &output); err != nil {
		t.Fatalf("render run content: %v", err)
	}
	for _, removed := range []string{"Service Job", "Batch Job", "Parameterized Job", `x-for="template in getTemplates()`} {
		if strings.Contains(output.String(), removed) {
			t.Fatalf("rendered run content still contains browser-only template %q", removed)
		}
	}
}
