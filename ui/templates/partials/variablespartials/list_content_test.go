package variablespartials

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestVariableListURLRetainsScopeAndView(t *testing.T) {
	props := ListProps{
		Namespace:   "team-a",
		Region:      "region-a",
		SearchQuery: "service key",
	}

	must.Eq(t, "/ui/variables?namespace=team-a&region=region-a&search=service+key&view=tree", variableWorkspaceURL(props, "tree"))
	must.Eq(t, "/ui/variables?namespace=team-a&region=region-a&search=service+key", variableWorkspaceURL(props, "list"))
	must.Eq(t, "/ui/variables/new?namespace=team-a&region=region-a", variableScopedURL("/ui/variables/new", props.Namespace, props.Region))
}

func TestBuildVariablesURLRetainsScope(t *testing.T) {
	props := ListProps{
		Namespace:   "team-a",
		Region:      "region-a",
		SortColumn:  "path",
		SortDir:     "asc",
		SearchQuery: "service",
	}

	must.Eq(t, "/ui/variables?dir=asc&namespace=team-a&region=region-a&search=service&sort=path&", buildVariablesURL(props))
}
