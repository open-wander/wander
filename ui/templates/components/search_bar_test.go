package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSearchBarSupportsIndependentNameAndID(t *testing.T) {
	var output bytes.Buffer
	err := SearchBar(SearchBarProps{
		Name:      "q",
		ID:        "global-search",
		SearchURL: "/ui/search",
		ResultsID: "global-search-results",
	}).Render(context.Background(), &output)
	if err != nil {
		t.Fatalf("render search bar: %v", err)
	}
	rendered := output.String()
	for _, expected := range []string{`name="q"`, `id="global-search"`, `global-search-indicator`} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("search bar missing %q: %s", expected, rendered)
		}
	}
}
