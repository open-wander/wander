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
		Variables:   `image = "busybox:stable"`,
		Format:      "hcl",
		ModifyIndex: 42,
		CanEdit:     true,
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
		`x-ref="variablesSource"`,
		`data-job-modify-index="42"`,
		`Job Spec`,
		`Full Definition`,
		`Review changes`,
		`image = &#34;busybox:stable&#34;`,
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

func TestDefinitionTabHidesEditControlsWithoutSubmitPermission(t *testing.T) {
	var output bytes.Buffer
	err := DefinitionTab(JobDefinition{
		JobID:       "example",
		Content:     `{"ID":"example"}`,
		JSONContent: `{"ID":"example"}`,
		Format:      "json",
		CanEdit:     false,
	}).Render(context.Background(), &output)
	if err != nil {
		t.Fatalf("render definition tab: %v", err)
	}

	html := output.String()
	if strings.Contains(html, `@click="toggleEdit"`) || strings.Contains(html, `@click="planJob"`) {
		t.Fatalf("definition rendered edit controls without submit permission: %s", html)
	}
}
