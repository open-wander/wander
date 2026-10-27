package layouts

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestBaseLayoutExposesResolvedSelectionToScopeHelper(t *testing.T) {
	var output strings.Builder
	err := BaseLayout(BaseLayoutProps{
		Title:            "Scoped page",
		CurrentNamespace: `team-a`,
		CurrentRegion:    `region-a`,
	}, templ.NopComponent).Render(context.Background(), &output)
	if err != nil {
		t.Fatalf("render base layout: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`data-wander-namespace="team-a"`,
		`data-wander-region="region-a"`,
		`src="/ui/static/js/selection-scope.js"`,
		`src="/ui/static/js/keyboard-shortcuts.js"`,
		`data-keyboard-shortcuts-dialog`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered layout missing %s", expected)
		}
	}
}
