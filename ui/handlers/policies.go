package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/policies"
	"github.com/open-wander/wander/ui/templates/partials/policiespartials"
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

	// Handle the policies list page.
	if path == aclPoliciesPath {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle policy detail and mutation routes.
	if strings.HasPrefix(path, aclPoliciesPath+"/") {
		if strings.TrimSuffix(path, "/") == aclPoliciesPath+"/new" {
			switch r.Method {
			case http.MethodGet:
				h.showNew(w, r, policiespartials.NewPolicyForm{})
			case http.MethodPost:
				h.createPolicy(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *PoliciesHandler) showNew(w http.ResponseWriter, r *http.Request, form policiespartials.NewPolicyForm) {
	perms := GetPermissionsFromRequest(r)
	if perms == nil || !perms.IsManagement() {
		h.handler.RenderForbidden(w, r, ForbiddenError{Message: "You do not have permission to create ACL policies", Capability: "management token"})
		return
	}
	props := h.handler.BaseLayoutProps(r, "New ACL Policy - Wander", "Create a Wander ACL policy", aclPoliciesPath+"/new", []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
		{Label: "Access Control", URL: aclBasePath},
		{Label: "ACL Policies", URL: aclPoliciesPath},
		{Label: "New Policy"},
	})
	if err := Render(w, r, policies.New(props, form), policiespartials.NewContent(form)); err != nil {
		h.handler.logger.Error("failed to render new policy form", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *PoliciesHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	perms := GetPermissionsFromRequest(r)
	if perms == nil || !perms.IsManagement() {
		h.handler.RenderForbidden(w, r, ForbiddenError{Message: "You do not have permission to create ACL policies", Capability: "management token"})
		return
	}
	if err := r.ParseForm(); err != nil {
		h.showNew(w, r, policiespartials.NewPolicyForm{Error: "The submitted form is invalid."})
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	rules := strings.TrimSpace(r.FormValue("rules"))
	form := policiespartials.NewPolicyForm{Name: name, Description: description, Rules: rules}
	if name == "" || strings.Contains(name, "/") {
		form.Error = "Policy name is required and cannot contain a slash."
		h.showNew(w, r, form)
		return
	}
	if rules == "" {
		form.Error = "Policy rules are required."
		h.showNew(w, r, form)
		return
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{Message: "The Wander API client is unavailable."})
		return
	}
	_, err := client.ACLPolicies().Upsert(&api.ACLPolicy{Name: name, Description: description, Rules: rules}, &api.WriteOptions{Region: GetRegionFromRequest(r), AuthToken: GetTokenFromRequest(r)})
	if err != nil {
		h.handler.logger.Warn("failed to create ACL policy", "policy", name, "error", err)
		status := http.StatusInternalServerError
		if apiResponseHasStatus(err, http.StatusBadRequest) {
			status = http.StatusBadRequest
		} else if apiResponseHasStatus(err, http.StatusForbidden) {
			status = http.StatusForbidden
		}
		http.Error(w, "Unable to create ACL policy", status)
		return
	}
	target := aclPoliciesPath + "/" + url.PathEscape(name)
	if IsHTMXFromRequest(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
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

	// Never substitute fabricated policies when the cluster is unavailable.
	policiesList := h.getPoliciesFromAPI(r, token)

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
	layoutProps := h.handler.BaseLayoutProps(r,
		"ACL Policies - Wander",
		"View and manage Wander ACL policies",
		aclPoliciesPath,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Access Control", URL: aclBasePath},
			{Label: "Policies", URL: ""},
		},
	)

	// Render the policies list page
	err := Render(w, r,
		policies.List(layoutProps, policiesList, sortColumn, sortDir, searchQuery),   // Full page with layout
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
	policyName := strings.TrimPrefix(r.URL.Path, aclPoliciesPath+"/")
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

	policy, err := h.getPolicyFromAPI(r, policyName, token)
	if err != nil {
		if apiResponseHasStatus(err, http.StatusNotFound) {
			http.NotFound(w, r)
			return
		}
		h.handler.logger.Warn("failed to load ACL policy detail", "policy", policyName, "error", err)
		http.Error(w, "Unable to load ACL policy", http.StatusInternalServerError)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		policy.Name+" - ACL Policy - Wander",
		"View and edit ACL policy: "+policy.Name,
		aclPoliciesPath+"/"+policyName,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Access Control", URL: aclBasePath},
			{Label: "ACL Policies", URL: aclPoliciesPath},
			{Label: policy.Name, URL: ""},
		},
	)

	// Render the policy detail page
	err = Render(w, r,
		policies.Detail(layoutProps, policy),   // Full page with layout
		policiespartials.DetailContent(policy), // Partial content for HTMX requests
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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Wander client not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with token
	writeOpts := &api.WriteOptions{Region: GetRegionFromRequest(r)}
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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Wander client not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with token
	writeOpts := &api.WriteOptions{Region: GetRegionFromRequest(r)}
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
	w.Header().Set("HX-Redirect", aclPoliciesPath)
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Policy deleted successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// getPoliciesFromAPI fetches policies from the Nomad API.
func (h *PoliciesHandler) getPoliciesFromAPI(r *http.Request, token string) []policiespartials.PolicySummary {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{Region: GetRegionFromRequest(r)}
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
func (h *PoliciesHandler) getPolicyFromAPI(r *http.Request, name string, token string) (policiespartials.PolicyDetail, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return policiespartials.PolicyDetail{}, errors.New("Wander API client unavailable")
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{Region: GetRegionFromRequest(r)}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching policy from API", "policy", name, "hasToken", token != "")

	// Fetch policy from the API
	policy, _, err := client.ACLPolicies().Info(name, queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch policy from API", "policy", name, "error", err)
		return policiespartials.PolicyDetail{}, err
	}
	roles, _, err := client.ACLRoles().List(queryOpts)
	if err != nil {
		return policiespartials.PolicyDetail{}, err
	}
	tokens, _, err := client.ACLTokens().List(queryOpts)
	if err != nil {
		return policiespartials.PolicyDetail{}, err
	}

	h.handler.logger.Debug("fetched policy from API", "policy", name)

	// Map API policy to template policy
	// Note: ACL policies don't have timestamps in the API, only Raft indexes
	detail := policiespartials.PolicyDetail{
		Name:        policy.Name,
		Description: policy.Description,
		Rules:       policy.Rules,
		CreateIndex: policy.CreateIndex,
		ModifyIndex: policy.ModifyIndex,
		CreatedAt:   time.Time{}, // ACL policies don't have timestamps in the API
		ModifiedAt:  time.Time{}, // ACL policies don't have timestamps in the API
	}
	detail.RelatedTokens, detail.RelatedRoles = policyRelationships(name, tokens, roles)
	return detail, nil
}

func policyRelationships(policyName string, tokens []*api.ACLTokenListStub, roles []*api.ACLRoleListStub) ([]policiespartials.PolicyTokenBinding, []policiespartials.PolicyRoleBinding) {
	relatedRoleNames := make(map[string]string)
	relatedRoles := make([]policiespartials.PolicyRoleBinding, 0)
	for _, role := range roles {
		for _, policy := range role.Policies {
			if policy.Name == policyName {
				relatedRoleNames[role.ID] = role.Name
				relatedRoles = append(relatedRoles, policiespartials.PolicyRoleBinding{ID: role.ID, Name: role.Name})
				break
			}
		}
	}

	relatedTokens := make([]policiespartials.PolicyTokenBinding, 0)
	for _, token := range tokens {
		bindings := make([]string, 0, len(token.Roles)+1)
		for _, policy := range token.Policies {
			if policy == policyName {
				bindings = append(bindings, "direct")
				break
			}
		}
		for _, role := range token.Roles {
			if roleName, ok := relatedRoleNames[role.ID]; ok {
				bindings = append(bindings, "role "+roleName)
			}
		}
		if len(bindings) != 0 {
			relatedTokens = append(relatedTokens, policiespartials.PolicyTokenBinding{
				AccessorID: token.AccessorID,
				Name:       token.Name,
				Via:        strings.Join(bindings, ", "),
			})
		}
	}
	sort.Slice(relatedRoles, func(i, j int) bool { return relatedRoles[i].Name < relatedRoles[j].Name })
	sort.Slice(relatedTokens, func(i, j int) bool { return relatedTokens[i].Name < relatedTokens[j].Name })
	return relatedTokens, relatedRoles
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
