package policiespartials

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDetailContentKeepsRulesOutOfAlpineExpression(t *testing.T) {
	policy := PolicyDetail{
		Name:        "example",
		Description: "description with `quotes`",
		Rules:       "namespace \"default\" {\n  policy = \"read\"\n}",
	}
	var output bytes.Buffer
	if err := DetailContent(policy).Render(context.Background(), &output); err != nil {
		t.Fatalf("render policy detail: %v", err)
	}

	html := output.String()
	if !strings.Contains(html, `x-data="policyEditor"`) {
		t.Fatalf("expected CSP-safe Alpine component: %s", html)
	}
	if strings.Contains(html, "policyEditor(") {
		t.Fatalf("policy content must not be embedded in the Alpine expression: %s", html)
	}
	if !strings.Contains(html, `x-ref="rulesSource"`) || !strings.Contains(html, `x-ref="descriptionSource"`) {
		t.Fatalf("expected rules and description source fields: %s", html)
	}
}
