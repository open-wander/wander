package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
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

	// Check permissions - policies require management token
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.IsManagement() {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view ACL policies",
			Capability: "management token",
			Suggestions: []string{
				"ACL policy management requires a management token",
				"Contact your administrator to request management access",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get policy data - try real API first, fall back to mock
	var policiesList []policiespartials.PolicySummary
	if h.handler.HasNomadClient() {
		policiesList = h.getPoliciesFromAPI(token)
	}
	if len(policiesList) == 0 {
		policiesList = h.getPoliciesList()
	}

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

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"ACL Policies - Wander",
		"View and manage Nomad ACL policies",
		"/ui/policies",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Access Control", URL: ""},
			{Label: "Policies", URL: ""},
		},
	)

	// Render the policies list page
	err := Render(w, r,
		policies.List(layoutProps, policiesList, sortColumn, sortDir, searchQuery),                // Full page with layout
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

	// Check permissions - policies require management token
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.IsManagement() {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view ACL policy details",
			Capability: "management token",
			Suggestions: []string{
				"ACL policy management requires a management token",
				"Contact your administrator to request management access",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get policy data - try real API first, fall back to mock
	var policy policiespartials.PolicyDetail
	var exists bool
	if h.handler.HasNomadClient() {
		policy, exists = h.getPolicyFromAPI(policyName, token)
	}
	if !exists {
		policy, exists = h.getPolicyDetail(policyName)
	}
	if !exists {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		policy.Name+" - ACL Policy - Wander",
		"View and edit ACL policy: "+policy.Name,
		"/ui/policies/"+policyName,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "ACL Policies", URL: "/ui/policies"},
			{Label: policy.Name, URL: ""},
		},
	)

	// Render the policy detail page
	err := Render(w, r,
		policies.Detail(layoutProps, policy),                       // Full page with layout
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

	// Check permissions - policies require management token
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.IsManagement() {
		h.handler.logger.Warn("policy update permission denied", "policy", policyName)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Permission denied: management token required", "type": "error"}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Extract form values
	description := r.FormValue("description")
	rules := r.FormValue("rules")

	h.handler.logger.Info("policy update", "policy", policyName, "description", description, "rules_length", len(rules))

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Nomad client not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with token
	writeOpts := &api.WriteOptions{}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Build policy object for upsert
	policy := &api.ACLPolicy{
		Name:        policyName,
		Description: description,
		Rules:       rules,
	}

	// Upsert the policy via Nomad API
	_, err := client.ACLPolicies().Upsert(policy, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to upsert policy", "error", err, "policy", policyName)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to update policy: `+err.Error()+`", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("policy updated successfully", "policy", policyName)

	// Return success response (HTMX will show a toast)
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Policy updated successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// deletePolicy handles policy deletion requests
func (h *PoliciesHandler) deletePolicy(w http.ResponseWriter, r *http.Request, policyName string) {
	// Log the request
	h.handler.logger.Debug("policy delete request", "policy", policyName)

	// Check permissions - policies require management token
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.IsManagement() {
		h.handler.logger.Warn("policy delete permission denied", "policy", policyName)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Permission denied: management token required", "type": "error"}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	h.handler.logger.Info("policy delete", "policy", policyName)

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Nomad client not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with token
	writeOpts := &api.WriteOptions{}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Delete the policy via Nomad API
	_, err := client.ACLPolicies().Delete(policyName, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to delete policy", "error", err, "policy", policyName)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to delete policy: `+err.Error()+`", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("policy deleted successfully", "policy", policyName)

	// Return success response and redirect to policies list
	w.Header().Set("HX-Redirect", "/ui/policies")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Policy deleted successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// getPoliciesFromAPI fetches policies from the Nomad API.
func (h *PoliciesHandler) getPoliciesFromAPI(token string) []policiespartials.PolicySummary {
	client := h.handler.NomadClient()
	if client == nil {
		return nil
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching policies from API", "hasToken", token != "")

	// Fetch policies from the API
	policiesList, _, err := client.ACLPolicies().List(queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch policies from API", "error", err)
		return nil
	}

	h.handler.logger.Debug("fetched policies from API", "count", len(policiesList))

	result := make([]policiespartials.PolicySummary, 0, len(policiesList))
	for _, policy := range policiesList {
		// Note: ACLPolicyListStub doesn't include timestamps, only Raft indexes
		// We use zero time values which templates can handle gracefully
		result = append(result, policiespartials.PolicySummary{
			Name:        policy.Name,
			Description: policy.Description,
			CreateIndex: policy.CreateIndex,
			ModifyIndex: policy.ModifyIndex,
			CreatedAt:   time.Time{}, // ACL policies don't have timestamps in the API
			ModifiedAt:  time.Time{}, // ACL policies don't have timestamps in the API
		})
	}

	return result
}

// getPolicyFromAPI fetches a single policy from the Nomad API.
func (h *PoliciesHandler) getPolicyFromAPI(name string, token string) (policiespartials.PolicyDetail, bool) {
	client := h.handler.NomadClient()
	if client == nil {
		return policiespartials.PolicyDetail{}, false
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching policy from API", "policy", name, "hasToken", token != "")

	// Fetch policy from the API
	policy, _, err := client.ACLPolicies().Info(name, queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch policy from API", "policy", name, "error", err)
		return policiespartials.PolicyDetail{}, false
	}

	h.handler.logger.Debug("fetched policy from API", "policy", name)

	// Map API policy to template policy
	// Note: ACL policies don't have timestamps in the API, only Raft indexes
	return policiespartials.PolicyDetail{
		Name:        policy.Name,
		Description: policy.Description,
		Rules:       policy.Rules,
		CreateIndex: policy.CreateIndex,
		ModifyIndex: policy.ModifyIndex,
		CreatedAt:   time.Time{}, // ACL policies don't have timestamps in the API
		ModifiedAt:  time.Time{}, // ACL policies don't have timestamps in the API
	}, true
}

// getPoliciesList returns mock policy data.
// This will be replaced with actual Nomad API calls in the future.
func (h *PoliciesHandler) getPoliciesList() []policiespartials.PolicySummary {
	now := time.Now()

	return []policiespartials.PolicySummary{
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
func (h *PoliciesHandler) filterPolicies(policiesList []policiespartials.PolicySummary, query string) []policiespartials.PolicySummary {
	if query == "" {
		return policiesList
	}

	query = strings.ToLower(query)
	filtered := make([]policiespartials.PolicySummary, 0, len(policiesList))

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
func (h *PoliciesHandler) sortPolicies(policiesList []policiespartials.PolicySummary, sortColumn, sortDir string) {
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
