package components

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestThemeSelectorsExposeAllAccessibleChoices(t *testing.T) {
	for name, component := range map[string]templ.Component{
		"menu":     ThemeMenu(),
		"settings": ThemeSettings(),
	} {
		t.Run(name, func(t *testing.T) {
			var output strings.Builder
			if err := component.Render(context.Background(), &output); err != nil {
				t.Fatalf("render theme selector: %v", err)
			}

			html := output.String()
			for _, preference := range []string{"system", "light", "dark"} {
				if !strings.Contains(html, `data-theme-choice="`+preference+`"`) {
					t.Errorf("theme selector missing %q choice", preference)
				}
			}
			if !strings.Contains(html, `aria-checked="false"`) {
				t.Error("theme choices must expose an ARIA selection state")
			}
		})
	}
}
