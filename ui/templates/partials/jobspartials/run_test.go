package jobspartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunContentExposesHCLVariablesInput(t *testing.T) {
	var output bytes.Buffer
	if err := RunContent("team-a").Render(context.Background(), &output); err != nil {
		t.Fatalf("render run content: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		"HCL Variables",
		`id="job-variables"`,
		`x-model="variables"`,
		`x-show="format === 'hcl'"`,
		"HCL2 variable-file content",
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered run content does not contain %q: %s", expected, html)
		}
	}
}
