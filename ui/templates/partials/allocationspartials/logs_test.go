package allocationspartials

import (
	"bytes"
	"context"
	"regexp"
	"testing"
)

func TestLogsTabExposesInheritedNavigationAndWrappingControls(t *testing.T) {
	var output bytes.Buffer
	props := LogsTabProps{AllocID: "alloc-1", Tasks: []string{"web"}, SelectedTask: "web", LogType: "stdout"}
	if err := LogsTabContent(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("render logs tab: %v", err)
	}

	html := output.String()
	contract := regexp.MustCompile(`(?s)data-log-action="head".*data-log-action="live".*data-log-action="tail".*data-log-wrap-toggle.*data-log-line`)
	if !contract.MatchString(html) {
		t.Fatalf("rendered logs tab is missing navigation or wrapping controls: %s", html)
	}
	wrapping := regexp.MustCompile(`(?s)'whitespace-pre-wrap break-all': wrapLines.*'whitespace-pre': !wrapLines`)
	if !wrapping.MatchString(html) {
		t.Fatalf("rendered log lines do not switch wrapping classes: %s", html)
	}
}
