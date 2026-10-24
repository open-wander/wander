package handlers

import (
	"net/http"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/namespaces"
	"github.com/open-wander/wander/ui/templates/partials/namespacespartials"
)

// NamespacesHandler handles requests for the namespaces list page.
type NamespacesHandler struct {
	handler *Handler
}

// NewNamespacesHandler creates a new namespaces handler.
func NewNamespacesHandler(h *Handler) *NamespacesHandler {
	return &NamespacesHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the namespaces endpoints.
// It renders the namespaces list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *NamespacesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests for now
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.List(w, r)
}

// List handles the namespaces list page
func (h *NamespacesHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("namespaces list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Never substitute fabricated namespaces when the cluster is unavailable.
	namespacesList := h.getNamespacesFromAPI(r, token)

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Namespaces - Wander",
		"View and manage Wander namespaces",
		"/ui/namespaces",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Cluster", URL: ""},
			{Label: "Namespaces", URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		namespaces.List(layoutProps, namespacesList),   // Full page with layout
		namespacespartials.ListContent(namespacesList), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render namespaces list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getNamespacesFromAPI fetches namespaces from the Nomad API.
func (h *NamespacesHandler) getNamespacesFromAPI(r *http.Request, token string) []namespacespartials.NamespaceSummary {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching namespaces from API", "hasToken", token != "")

	// Fetch namespaces from the API
	nsList, _, err := client.Namespaces().List(queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch namespaces from API", "error", err)
		return nil
	}

	h.handler.logger.Debug("fetched namespaces from API", "count", len(nsList))

	result := make([]namespacespartials.NamespaceSummary, 0, len(nsList))
	for _, ns := range nsList {
		// Build query options for job count with the same token
		jobQueryOpts := &api.QueryOptions{
			Namespace: ns.Name,
		}
		if token != "" {
			jobQueryOpts.AuthToken = token
		}

		// Get job count for this namespace
		jobCount := 0
		if jobs, _, err := client.Jobs().List(jobQueryOpts); err == nil {
			jobCount = len(jobs)
		}

		quota := ""
		if ns.Quota != "" {
			quota = ns.Quota
		}

		result = append(result, namespacespartials.NamespaceSummary{
			Name:        ns.Name,
			Description: ns.Description,
			Quota:       quota,
			JobCount:    jobCount,
			CreateIndex: ns.CreateIndex,
			ModifyIndex: ns.ModifyIndex,
		})
	}

	return result
}
