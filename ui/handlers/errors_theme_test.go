package handlers

import (
	"strings"
	"testing"
)

func TestStandaloneErrorPagesApplyThemeBeforeStyles(t *testing.T) {
	pages := map[string]string{
		"forbidden":           renderForbiddenPage(ForbiddenError{}),
		"not found":           renderNotFoundPage(NotFoundError{}),
		"cluster unavailable": renderClusterUnavailablePage(ClusterUnavailableError{}),
	}

	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			themeScript := strings.Index(page, `src="/ui/static/js/theme.js"`)
			stylesheet := strings.Index(page, `href="/ui/static/css/output.css"`)
			if themeScript < 0 || stylesheet < 0 || themeScript > stylesheet {
				t.Fatal("error page must load the theme bootstrap before its stylesheet")
			}
			if !strings.Contains(page, "dark:bg-neutral-950") {
				t.Fatal("error page must provide a dark background")
			}
		})
	}
}
