package handlers

import (
	"net/http"
	"net/url"
	"sort"
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("global-search"))
	}
	if query == "" {
		h.renderResults(w, r, nil, query, "", false)
		return
	}

	if len([]rune(query)) < 2 {
		h.renderResults(w, r, nil, query, "", true)
		return
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("Nomad client not available for search")
		h.renderResults(w, r, nil, query, "Search is temporarily unavailable.", false)
		return
	}

	resp, _, err := client.Search().FuzzySearch(query, contexts.All, &api.QueryOptions{
		Namespace: GetNamespaceFromRequest(r),
		Region:    GetRegionFromRequest(r),
		AuthToken: GetTokenFromRequest(r),
	})
	if err != nil {
		h.handler.logger.Error("failed to search", "error", err, "query", query)
		h.renderResults(w, r, nil, query, "Search is temporarily unavailable.", false)
		return
	}

	h.renderResults(w, r, resp, query, "", false)
}

var searchContextOrder = []contexts.Context{
	contexts.Jobs,
	contexts.Allocs,
	contexts.Nodes,
	contexts.Evals,
	contexts.Deployments,
	contexts.Services,
	contexts.Variables,
	contexts.Volumes,
	contexts.Plugins,
	contexts.Namespaces,
	contexts.NodePools,
	contexts.Groups,
	contexts.Tasks,
}

func (h *SearchHandler) renderResults(
	w http.ResponseWriter,
	r *http.Request,
	resp *api.FuzzySearchResponse,
	query, errorMessage string,
	tooShort bool,
) {
	var results []searchpartials.SearchResultGroup
	if resp != nil {
		for _, ctx := range searchContextOrder {
			items := projectSearchMatches(ctx, resp.Matches[ctx], GetNamespaceFromRequest(r), GetRegionFromRequest(r))
			if len(items) == 0 {
				continue
			}
			group := searchpartials.SearchResultGroup{
				Type:       formatContextType(ctx),
				Truncated:  resp.Truncations[ctx],
				TotalCount: len(items),
				Items:      items,
			}
			results = append(results, group)
		}
	}

	props := searchpartials.SearchResultsProps{
		Query:        query,
		Results:      results,
		ErrorMessage: errorMessage,
		TooShort:     tooShort,
	}

	err := searchpartials.SearchResults(props).Render(r.Context(), w)
	if err != nil {
		h.handler.logger.Error("failed to render search results", "error", err)
		http.Error(w, "Failed to render results", http.StatusInternalServerError)
		return
	}
}

func projectSearchMatches(
	ctx contexts.Context,
	matches []api.FuzzyMatch,
	requestNamespace, region string,
) []searchpartials.SearchResultItem {
	items := make([]searchpartials.SearchResultItem, 0, len(matches))
	for _, match := range matches {
		resultURL := buildResultURL(ctx, match, requestNamespace, region)
		if resultURL == "" {
			continue
		}
		items = append(items, searchpartials.SearchResultItem{ID: match.ID, Scope: match.Scope, URL: resultURL})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return strings.Join(items[i].Scope, "\x00") < strings.Join(items[j].Scope, "\x00")
		}
		return items[i].ID < items[j].ID
	})
	return items
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
func buildResultURL(ctx contexts.Context, match api.FuzzyMatch, requestNamespace, region string) string {
	namespace := searchMatchNamespace(match, requestNamespace)
	escapedID := url.PathEscape(match.ID)
	switch ctx {
	case contexts.Jobs:
		return scopedSearchResultURL("/ui/jobs/"+escapedID, namespace, region, "", "")
	case contexts.Allocs:
		return scopedSearchResultURL("/ui/allocations/"+escapedID, namespace, region, "", "")
	case contexts.Nodes:
		return scopedSearchResultURL("/ui/clients/"+escapedID, "", region, "", "")
	case contexts.Evals:
		return scopedSearchResultURL("/ui/evaluations/"+escapedID, namespace, region, "", "")
	case contexts.Deployments:
		return scopedSearchResultURL("/ui/deployments/"+escapedID, namespace, region, "", "")
	case contexts.Services:
		return scopedSearchResultURL("/ui/services/"+escapedID, namespace, region, "", "")
	case contexts.Namespaces:
		return scopedSearchResultURL("/ui/namespaces", match.ID, region, "", "")
	case contexts.NodePools:
		return scopedSearchResultURL("/ui/clients", "", region, "node_pool", match.ID)
	case contexts.Volumes:
		return scopedSearchResultURL("/ui/csi/volumes/"+escapedID, namespace, region, "", "")
	case contexts.Plugins:
		return scopedSearchResultURL("/ui/csi/plugins/"+escapedID, "", region, "", "")
	case contexts.Variables:
		return scopedSearchResultURL("/ui/variables/"+escapedID, namespace, region, "", "")
	case contexts.Groups:
		if len(match.Scope) >= 2 && match.Scope[1] != "" {
			path := "/ui/jobs/" + url.PathEscape(match.Scope[1]) + "/taskgroups/" + escapedID
			return scopedSearchResultURL(path, namespace, region, "", "")
		}
		return ""
	case contexts.Tasks:
		if len(match.Scope) >= 3 && match.Scope[1] != "" && match.Scope[2] != "" {
			path := "/ui/jobs/" + url.PathEscape(match.Scope[1]) + "/taskgroups/" + url.PathEscape(match.Scope[2])
			return scopedSearchResultURL(path, namespace, region, "", "")
		}
		return ""
	default:
		return ""
	}
}

func searchMatchNamespace(match api.FuzzyMatch, fallback string) string {
	if len(match.Scope) > 0 && match.Scope[0] != "" {
		return match.Scope[0]
	}
	return fallback
}

func scopedSearchResultURL(path, namespace, region, extraKey, extraValue string) string {
	query := url.Values{}
	if namespace != "" {
		query.Set("namespace", namespace)
	}
	if region != "" {
		query.Set("region", region)
	}
	if extraKey != "" && extraValue != "" {
		query.Set(extraKey, extraValue)
	}
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}
