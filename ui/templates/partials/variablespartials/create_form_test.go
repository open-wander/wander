package variablespartials

import (
	"testing"

	"github.com/open-wander/wander/ui/templates/components"
	"github.com/shoenig/test/must"
)

func TestVariableNamespaceOptions(t *testing.T) {
	options := variableNamespaceOptions(CreateProps{
		CurrentNamespace: "team-a",
		AvailableNamespaces: []components.NamespaceInfo{
			{Name: "default"},
			{Name: "team-a"},
			{Name: "team-a"},
			{},
		},
	})

	must.Eq(t, []components.SelectOption{
		{Value: "default", Label: "default"},
		{Value: "team-a", Label: "team-a"},
	}, options)
}

func TestVariableNamespaceOptionsRetainsCurrentFallback(t *testing.T) {
	options := variableNamespaceOptions(CreateProps{CurrentNamespace: "team-a"})
	must.Eq(t, []components.SelectOption{{Value: "team-a", Label: "team-a"}}, options)
}
