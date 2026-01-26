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

// Detail handles the policy detail page
func (h *PoliciesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract policy name from path
	policyName := strings.TrimPrefix(r.URL.Path, "/ui/policies/")
	policyName = strings.TrimSuffix(policyName, "/")

	// Handle different HTTP methods
	switch r.Method {
	case http.MethodGet:
		h.showDetail(w, r, policyName)
	case http.MethodPost:
		h.updatePolicy(w, r, policyName)
	case http.MethodDelete:
		h.deletePolicy(w, r, policyName)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// showDetail renders the policy detail page
func (h *PoliciesHandler) showDetail(w http.ResponseWriter, r *http.Request, policyName string) {
	// Log the request
	h.handler.logger.Debug("policy detail request", "policy", policyName, "htmx", IsHTMXFromRequest(r))

	// Get policy data (mock for now)
	policy, exists := h.getPolicyDetail(policyName)
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Render the policy detail page
	err := Render(w, r,
		policies.Detail(policy),                       // Full page with layout
		policiespartials.DetailContent(policy),        // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render policy detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// updatePolicy handles policy update requests
func (h *PoliciesHandler) updatePolicy(w http.ResponseWriter, r *http.Request, policyName string) {
	// Log the request
	h.handler.logger.Debug("policy update request", "policy", policyName)

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Extract form values
	description := r.FormValue("description")
	rules := r.FormValue("rules")

	// Log the update (in real implementation, this would call Nomad API)
	h.handler.logger.Info("policy update", "policy", policyName, "description", description, "rules_length", len(rules))

	// Return success response (HTMX will show a toast)
	w.Header().Set("HX-Trigger", `{"toast": {"message": "Policy updated successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// deletePolicy handles policy deletion requests
func (h *PoliciesHandler) deletePolicy(w http.ResponseWriter, r *http.Request, policyName string) {
	// Log the request
	h.handler.logger.Debug("policy delete request", "policy", policyName)

	// In real implementation, this would call Nomad API
	h.handler.logger.Info("policy delete", "policy", policyName)

	// Return success response and redirect to policies list
	w.Header().Set("HX-Redirect", "/ui/policies")
	w.Header().Set("HX-Trigger", `{"toast": {"message": "Policy deleted successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
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

// getPolicyDetail returns mock policy detail data.
// This will be replaced with actual Nomad API calls in the future.
func (h *PoliciesHandler) getPolicyDetail(name string) (policiespartials.PolicyDetail, bool) {
	now := time.Now()

	// Mock policy data with HCL rules
	mockPolicies := map[string]policiespartials.PolicyDetail{
		"admin": {
			Name:        "admin",
			Description: "Full administrative access to all Nomad resources",
			Rules: `# Allow full access to all namespaces
namespace "*" {
  policy       = "write"
  capabilities = ["alloc-lifecycle", "submit-job", "dispatch-job", "read-logs"]
}

# Allow full access to agent operations
agent {
  policy = "write"
}

# Allow full access to operator commands
operator {
  policy = "write"
}

# Allow full access to quota specifications
quota {
  policy = "write"
}

# Allow full access to node operations
node {
  policy = "write"
}`,
			CreateIndex: 1000,
			ModifyIndex: 1000,
			CreatedAt:   now.Add(-720 * time.Hour),
			ModifiedAt:  now.Add(-720 * time.Hour),
		},
		"developer": {
			Name:        "developer",
			Description: "Read and write access to jobs, allocations, and evaluations",
			Rules: `# Allow read-write access to default namespace
namespace "default" {
  policy       = "write"
  capabilities = ["submit-job", "dispatch-job", "read-logs", "alloc-exec"]
}

# Allow read-write access to development namespace
namespace "dev" {
  policy       = "write"
  capabilities = ["submit-job", "dispatch-job", "read-logs", "alloc-exec"]
}

# Allow read access to all other namespaces
namespace "*" {
  policy = "read"
}

# Allow read access to agent
agent {
  policy = "read"
}

# Allow read access to nodes
node {
  policy = "read"
}`,
			CreateIndex: 1001,
			ModifyIndex: 1015,
			CreatedAt:   now.Add(-600 * time.Hour),
			ModifiedAt:  now.Add(-48 * time.Hour),
		},
		"operator": {
			Name:        "operator",
			Description: "Access to manage nodes, deployments, and scaling operations",
			Rules: `# Allow read-write access to all namespaces
namespace "*" {
  policy       = "write"
  capabilities = ["alloc-lifecycle", "submit-job"]
}

# Allow write access to nodes
node {
  policy = "write"
}

# Allow read access to agent
agent {
  policy = "read"
}

# Allow write access to operator commands (drain, etc.)
operator {
  policy = "write"
}`,
			CreateIndex: 1002,
			ModifyIndex: 1008,
			CreatedAt:   now.Add(-480 * time.Hour),
			ModifiedAt:  now.Add(-120 * time.Hour),
		},
		"read-only": {
			Name:        "read-only",
			Description: "Read-only access to all Nomad resources",
			Rules: `# Allow read access to all namespaces
namespace "*" {
  policy = "read"
}

# Allow read access to agent
agent {
  policy = "read"
}

# Allow read access to nodes
node {
  policy = "read"
}

# Allow read access to operator
operator {
  policy = "read"
}

# Allow read access to quota
quota {
  policy = "read"
}`,
			CreateIndex: 1003,
			ModifyIndex: 1003,
			CreatedAt:   now.Add(-360 * time.Hour),
			ModifiedAt:  now.Add(-360 * time.Hour),
		},
		"ci-deploy": {
			Name:        "ci-deploy",
			Description: "CI/CD pipeline policy for automated deployments",
			Rules: `# Allow write access to production namespace for deployments
namespace "production" {
  policy       = "write"
  capabilities = ["submit-job", "dispatch-job", "read-logs"]
}

# Allow write access to staging namespace
namespace "staging" {
  policy       = "write"
  capabilities = ["submit-job", "dispatch-job", "read-logs"]
}

# Allow read access to all namespaces for monitoring
namespace "*" {
  policy = "read"
}

# Allow read access to nodes for health checks
node {
  policy = "read"
}`,
			CreateIndex: 1004,
			ModifyIndex: 1020,
			CreatedAt:   now.Add(-240 * time.Hour),
			ModifiedAt:  now.Add(-24 * time.Hour),
		},
		"monitoring": {
			Name:        "monitoring",
			Description: "Read access for monitoring and observability tools",
			Rules: `# Allow read access to all namespaces for metrics collection
namespace "*" {
  policy       = "read"
  capabilities = ["read-logs"]
}

# Allow read access to agent for health metrics
agent {
  policy = "read"
}

# Allow read access to nodes for resource metrics
node {
  policy = "read"
}

# Allow read access to operator for cluster state
operator {
  policy = "read"
}`,
			CreateIndex: 1005,
			ModifyIndex: 1005,
			CreatedAt:   now.Add(-168 * time.Hour),
			ModifiedAt:  now.Add(-168 * time.Hour),
		},
	}

	policy, exists := mockPolicies[name]
	return policy, exists
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
