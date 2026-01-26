package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/policies"
	"github.com/open-wander/wander/ui-htmx/templates/partials/policiespartials"
)

// PoliciesHandler handles requests for the ACL policies pages.
type PoliciesHandler struct {
	handler *Handler
}

// NewPoliciesHandler creates a new policies handler.
func NewPoliciesHandler(h *Handler) *PoliciesHandler {
	return &PoliciesHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the policies pages.
// It renders the policies list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *PoliciesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/policies exactly (list page)
	if path == "/ui/policies" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/policies/{name} (detail page - stub for now)
	if strings.HasPrefix(path, "/ui/policies/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// List handles the policies list page
func (h *PoliciesHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("policies list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock policy data
	policiesList := h.getPoliciesList()

	// Parse search query from query string
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))

	// Filter policies by search query if provided
	if searchQuery != "" {
		policiesList = h.filterPolicies(policiesList, searchQuery)
	}

	// Parse sort parameters from query string
	sortColumn := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	// Default sort if not specified
	if sortColumn == "" {
		sortColumn = "modified"
		sortDir = "desc"
	}

	// Sort the policies list based on parameters
	h.sortPolicies(policiesList, sortColumn, sortDir)

	// Render the policies list page
	err := Render(w, r,
		policies.List(policiesList, sortColumn, sortDir, searchQuery),                // Full page with layout
		policiespartials.ListContent(policiesList, sortColumn, sortDir, searchQuery), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render policies list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Detail handles the policy detail page (stub for now)
func (h *PoliciesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract policy name from path
	policyName := strings.TrimPrefix(r.URL.Path, "/ui/policies/")
	policyName = strings.TrimSuffix(policyName, "/")

	// Log the request
	h.handler.logger.Debug("policy detail request", "policy", policyName, "htmx", IsHTMXFromRequest(r))

	// TODO: Implement policy detail page with editor
	// For now, return a placeholder
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
		<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
			<div class="bg-white dark:bg-neutral-900 shadow rounded-lg p-6">
				<h2 class="text-2xl font-bold text-neutral-900 dark:text-neutral-100 mb-4">Policy: ` + policyName + `</h2>
				<p class="text-neutral-600 dark:text-neutral-400">Policy editor coming soon.</p>
			</div>
		</div>
	`))
}

// getPoliciesList returns mock policy data.
// This will be replaced with actual Nomad API calls in the future.
func (h *PoliciesHandler) getPoliciesList() []policies.PolicySummary {
	now := time.Now()

	return []policies.PolicySummary{
		{
			Name:        "admin",
			Description: "Full administrative access to all Nomad resources",
			CreateIndex: 1000,
			ModifyIndex: 1000,
			CreatedAt:   now.Add(-720 * time.Hour), // 30 days ago
			ModifiedAt:  now.Add(-720 * time.Hour),
		},
		{
			Name:        "developer",
			Description: "Read and write access to jobs, allocations, and evaluations",
			CreateIndex: 1001,
			ModifyIndex: 1015,
			CreatedAt:   now.Add(-600 * time.Hour), // 25 days ago
			ModifiedAt:  now.Add(-48 * time.Hour),  // 2 days ago
		},
		{
			Name:        "operator",
			Description: "Access to manage nodes, deployments, and scaling operations",
			CreateIndex: 1002,
			ModifyIndex: 1008,
			CreatedAt:   now.Add(-480 * time.Hour), // 20 days ago
			ModifiedAt:  now.Add(-120 * time.Hour), // 5 days ago
		},
		{
			Name:        "read-only",
			Description: "Read-only access to all Nomad resources",
			CreateIndex: 1003,
			ModifyIndex: 1003,
			CreatedAt:   now.Add(-360 * time.Hour), // 15 days ago
			ModifiedAt:  now.Add(-360 * time.Hour),
		},
		{
			Name:        "ci-deploy",
			Description: "CI/CD pipeline policy for automated deployments",
			CreateIndex: 1004,
			ModifyIndex: 1020,
			CreatedAt:   now.Add(-240 * time.Hour), // 10 days ago
			ModifiedAt:  now.Add(-24 * time.Hour),  // 1 day ago
		},
		{
			Name:        "monitoring",
			Description: "Read access for monitoring and observability tools",
			CreateIndex: 1005,
			ModifyIndex: 1005,
			CreatedAt:   now.Add(-168 * time.Hour), // 7 days ago
			ModifiedAt:  now.Add(-168 * time.Hour),
		},
	}
}

// filterPolicies filters the policies list based on search query.
// The search is case-insensitive and matches against policy name and description.
func (h *PoliciesHandler) filterPolicies(policiesList []policies.PolicySummary, query string) []policies.PolicySummary {
	if query == "" {
		return policiesList
	}

	query = strings.ToLower(query)
	filtered := make([]policies.PolicySummary, 0, len(policiesList))

	for _, policy := range policiesList {
		// Match against Name or Description (case-insensitive)
		if strings.Contains(strings.ToLower(policy.Name), query) ||
			strings.Contains(strings.ToLower(policy.Description), query) {
			filtered = append(filtered, policy)
		}
	}

	return filtered
}

// sortPolicies sorts the policies list based on the specified column and direction.
func (h *PoliciesHandler) sortPolicies(policiesList []policies.PolicySummary, sortColumn, sortDir string) {
	sort.Slice(policiesList, func(i, j int) bool {
		var less bool

		switch sortColumn {
		case "name":
			less = policiesList[i].Name < policiesList[j].Name
		case "description":
			less = policiesList[i].Description < policiesList[j].Description
		case "modified":
			less = policiesList[i].ModifiedAt.Before(policiesList[j].ModifiedAt)
		default:
			// Default to sorting by modified time descending
			less = policiesList[i].ModifiedAt.After(policiesList[j].ModifiedAt)
		}

		// Reverse if descending
		if sortDir == "desc" {
			return !less
		}
		return less
	})
}
