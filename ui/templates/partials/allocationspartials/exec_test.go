package allocationspartials

import (
	"context"
	"strings"
	"testing"
)

func TestExecTabReliesOnAlpineInitLifecycle(t *testing.T) {
	var output strings.Builder
	if err := ExecTabContent(ExecTabProps{
		AllocID:      "allocation-id",
		Tasks:        []string{"web"},
		SelectedTask: "web",
		Command:      "/bin/sh",
	}).Render(context.Background(), &output); err != nil {
		t.Fatalf("render exec tab: %v", err)
	}

	html := output.String()
	if !strings.Contains(html, `x-data="execTerminal(`) {
		t.Fatalf("exec tab is missing its Alpine component: %s", html)
	}
	if strings.Contains(html, `x-init="init()"`) {
		t.Fatal("exec tab must not invoke init explicitly; Alpine invokes object init methods automatically")
	}
}
