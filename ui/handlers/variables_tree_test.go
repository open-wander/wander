package handlers

import (
	"testing"

	"github.com/open-wander/wander/ui/templates/partials/variablespartials"
	"github.com/shoenig/test/must"
)

func TestBuildVariablePathTree(t *testing.T) {
	tree := buildVariablePathTree([]variablespartials.VariableSummary{
		{Path: "zeta"},
		{Path: "alpha/two"},
		{Path: "nomad/jobs/example/config"},
		{Path: "alpha"},
		{Path: "alpha/one/deep"},
		{Path: "alpha/one"},
	})

	must.Eq(t, []string{"alpha", "nomad", "zeta"}, variableNodeSegments(tree))
	must.NotNil(t, tree[0].Variable)
	must.Eq(t, "alpha", tree[0].Variable.Path)
	must.Eq(t, []string{"one", "two"}, variableNodeSegments(tree[0].Children))
	must.NotNil(t, tree[0].Children[0].Variable)
	must.Eq(t, "alpha/one", tree[0].Children[0].Variable.Path)
	must.Eq(t, "alpha/one/deep", tree[0].Children[0].Children[0].Path)
	must.Nil(t, tree[1].Variable)
	must.Eq(t, "nomad/jobs/example/config", tree[1].Children[0].Children[0].Children[0].Path)
}

func TestVariableListView(t *testing.T) {
	must.Eq(t, "tree", variableListView("tree"))
	must.Eq(t, "list", variableListView(""))
	must.Eq(t, "list", variableListView("unsupported"))
}

func variableNodeSegments(nodes []variablespartials.VariablePathNode) []string {
	segments := make([]string, 0, len(nodes))
	for _, node := range nodes {
		segments = append(segments, node.Segment)
	}
	return segments
}
