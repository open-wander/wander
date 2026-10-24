package jobspartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDefinitionTabKeepsContentOutOfAlpineExpression(t *testing.T) {
	definition := "job `quoted` {\n  meta { value = \"${literal}\" }\n}"
	var output bytes.Buffer
	err := DefinitionTab(JobDefinition{
		JobID:       "example",
		Content:     definition,
		HCLContent:  definition,
		JSONContent: `{"ID":"example"}`,
		Format:      "hcl",
	}).Render(context.Background(), &output)
	if err != nil {
		t.Fatalf("render definition tab: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`x-data="jobDefinitionEditor"`,
		`x-ref="definitionSource"`,
		`x-ref="hclSource"`,
		`x-ref="jsonSource"`,
		"${literal}",
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered definition does not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, "jobDefinitionEditor(") {
		t.Fatalf("job content must not be embedded in the Alpine expression: %s", html)
	}
}
