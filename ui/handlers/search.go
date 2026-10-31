package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/api/contexts"
	"github.com/open-wander/wander/ui/templates/partials/searchpartials"
)

// SearchHandler handles requests for global search functionality.
type SearchHandler struct {
	handler *Handler
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(h *Handler) *SearchHandler {
	return &SearchHandler{
		handler: h,
	}
}

// ServeHTTP handles search requests using Nomad's fuzzy search API.
// It supports HTMX requests for live search updates.
func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get search query from URL parameter
	query := r.URL.Query().Get("q")
	if query == "" {
		// Empty query - return empty results
		h.renderResults(w, r, nil, query)
		return
	}

	// If query is too short, don't search
	if len(query) < 2 {
		h.renderResults(w, r, nil, query)
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("Nomad client not available for search")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Call Nomad's fuzzy search API
	// Context "all" searches across all object types
	resp, _, err := client.Search().FuzzySearch(query, contexts.All, &api.QueryOptions{})
	if err != nil {
		h.handler.logger.Error("failed to search", "error", err, "query", query)
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Render search results
	h.renderResults(w, r, resp, query)
}

// renderResults renders the search results partial
func (h *SearchHandler) renderResults(w http.ResponseWriter, r *http.Request, resp *api.FuzzySearchResponse, query string) {
	// Convert API response to template props
	var results []searchpartials.SearchResultGroup

	if resp != nil {
		// Process each context type that has matches
		for ctx, matches := range resp.Matches {
			if len(matches) == 0 {
				continue
			}

			group := searchpartials.SearchResultGroup{
				Type:       formatContextType(ctx),
				Truncated:  resp.Truncations[ctx],
				TotalCount: len(matches),
				Items:      make([]searchpartials.SearchResultItem, 0, len(matches)),
			}

			// Convert matches to items
			for _, match := range matches {
				item := searchpartials.SearchResultItem{
					ID:    match.ID,
					Scope: match.Scope,
					URL:   buildResultURL(ctx, match),
				}
				group.Items = append(group.Items, item)
			}

			results = append(results, group)
		}
	}

	// Render the search results partial
	props := searchpartials.SearchResultsProps{
		Query:   query,
		Results: results,
	}

	err := searchpartials.SearchResults(props).Render(r.Context(), w)
	if err != nil {
		h.handler.logger.Error("failed to render search results", "error", err)
		http.Error(w, "Failed to render results", http.StatusInternalServerError)
		return
	}
}

// formatContextType converts a context.Context to a human-readable label
func formatContextType(ctx contexts.Context) string {
	switch ctx {
	case contexts.Jobs:
		return "Jobs"
	case contexts.Allocs:
		return "Allocations"
	case contexts.Nodes:
		return "Nodes"
	case contexts.Evals:
		return "Evaluations"
	case contexts.Deployments:
		return "Deployments"
	case contexts.Namespaces:
		return "Namespaces"
	case contexts.NodePools:
		return "Node Pools"
	case contexts.Quotas:
		return "Quotas"
	case contexts.Plugins:
		return "CSI Plugins"
	case contexts.Volumes:
		return "CSI Volumes"
	case contexts.Variables:
		return "Variables"
	case contexts.ScalingPolicies:
		return "Scaling Policies"
	case contexts.Groups:
		return "Task Groups"
	case contexts.Tasks:
		return "Tasks"
	case contexts.Services:
		return "Services"
	case contexts.Images:
		return "Images"
	case contexts.Commands:
		return "Commands"
	case contexts.Classes:
		return "Node Classes"
	default:
		// Capitalize first letter
		s := string(ctx)
		if len(s) > 0 {
			return strings.ToUpper(s[:1]) + s[1:]
		}
		return s
	}
}

// buildResultURL constructs the UI URL for a search result based on its type and scope
func buildResultURL(ctx contexts.Context, match api.FuzzyMatch) string {
	// Build URL based on context type
	switch ctx {
	case contexts.Jobs:
		// Jobs: /ui/jobs/{id}
		if len(match.Scope) > 0 {
			// Job is in a namespace: scope[0] is namespace, ID is job name
			return fmt.Sprintf("/ui/jobs/%s@%s", match.ID, match.Scope[0])
		}
		return fmt.Sprintf("/ui/jobs/%s", match.ID)

	case contexts.Allocs:
		// Allocations: /ui/allocations/{id}
		return fmt.Sprintf("/ui/allocations/%s", match.ID)

	case contexts.Nodes:
		// Nodes/Clients: /ui/clients/{id}
		return fmt.Sprintf("/ui/clients/%s", match.ID)

	case contexts.Evals:
		// Evaluations: /ui/evaluations/{id}
		return fmt.Sprintf("/ui/evaluations/%s", match.ID)

	case contexts.Deployments:
		// Deployments: /ui/deployments/{id}
		return fmt.Sprintf("/ui/deployments/%s", match.ID)

	case contexts.Namespaces:
		// Namespaces: /ui/namespaces
		return "/ui/namespaces"

	case contexts.Volumes:
		// CSI Volumes: /ui/csi/volumes/{id}
		return fmt.Sprintf("/ui/csi/volumes/%s", match.ID)

	case contexts.Plugins:
		// CSI Plugins: /ui/csi/plugins
		return "/ui/csi/plugins"

	case contexts.Variables:
		// Variables: /ui/variables/{path}
		return fmt.Sprintf("/ui/variables/%s", match.ID)

	case contexts.Groups:
		// Task Groups: /ui/jobs/{job}/taskgroups/{group}
		// Scope: [namespace, job]
		if len(match.Scope) >= 2 {
			return fmt.Sprintf("/ui/jobs/%s@%s/taskgroups/%s", match.Scope[1], match.Scope[0], match.ID)
		}
		return ""

	case contexts.Tasks:
		// Tasks: /ui/jobs/{job}/taskgroups/{group}
		// Scope: [namespace, job, group]
		if len(match.Scope) >= 3 {
			return fmt.Sprintf("/ui/jobs/%s@%s/taskgroups/%s", match.Scope[1], match.Scope[0], match.Scope[2])
		}
		return ""

	default:
		// For other types, try to link to job if scope indicates it
		if len(match.Scope) >= 2 {
			return fmt.Sprintf("/ui/jobs/%s@%s", match.Scope[1], match.Scope[0])
		}
		return ""
	}
}
