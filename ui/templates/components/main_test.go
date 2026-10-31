package components

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestMainAllowsFlexContentToScroll(t *testing.T) {
	for name, component := range map[string]templ.Component{
		"contained":  Main(MainProps{}, templ.NopComponent),
		"full-width": MainFullWidth(MainProps{}, templ.NopComponent),
	} {
		t.Run(name, func(t *testing.T) {
			var output strings.Builder
			if err := component.Render(context.Background(), &output); err != nil {
				t.Fatalf("render main component: %v", err)
			}

			html := output.String()
			if !strings.Contains(html, "min-h-0 flex-1 overflow-y-auto") {
				t.Fatalf("main scroll container must be allowed to shrink: %s", html)
			}
		})
	}
}
