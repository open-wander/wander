package handlers

import (
	"sort"
	"strings"

	"github.com/open-wander/wander/ui/templates/partials/variablespartials"
)

type variablePathTreeNode struct {
	segment  string
	path     string
	variable *variablespartials.VariableSummary
	children map[string]*variablePathTreeNode
}

func variableListView(view string) string {
	if view == "tree" {
		return view
	}
	return "list"
}

func buildVariablePathTree(variables []variablespartials.VariableSummary) []variablespartials.VariablePathNode {
	root := &variablePathTreeNode{children: make(map[string]*variablePathTreeNode)}
	for i := range variables {
		segments := strings.Split(strings.Trim(variables[i].Path, "/"), "/")
		parent := root
		pathSegments := make([]string, 0, len(segments))
		for _, segment := range segments {
			if segment == "" {
				continue
			}
			pathSegments = append(pathSegments, segment)
			node, exists := parent.children[segment]
			if !exists {
				node = &variablePathTreeNode{
					segment:  segment,
					path:     strings.Join(pathSegments, "/"),
					children: make(map[string]*variablePathTreeNode),
				}
				parent.children[segment] = node
			}
			parent = node
		}
		parent.variable = &variables[i]
	}
	return projectVariablePathNodes(root.children)
}

func projectVariablePathNodes(nodes map[string]*variablePathTreeNode) []variablespartials.VariablePathNode {
	segments := make([]string, 0, len(nodes))
	for segment := range nodes {
		segments = append(segments, segment)
	}
	sort.Strings(segments)

	result := make([]variablespartials.VariablePathNode, 0, len(segments))
	for _, segment := range segments {
		node := nodes[segment]
		result = append(result, variablespartials.VariablePathNode{
			Segment:  node.segment,
			Path:     node.path,
			Variable: node.variable,
			Children: projectVariablePathNodes(node.children),
		})
	}
	return result
}
