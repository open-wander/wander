package handlers

import (
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/variables"
	"github.com/open-wander/wander/ui/templates/partials/variablespartials"
)

// VariablesHandler handles requests for the variables list page.
type VariablesHandler struct {
	handler *Handler
}

// NewVariablesHandler creates a new variables handler.
func NewVariablesHandler(h *Handler) *VariablesHandler {
	return &VariablesHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the variables pages.
// It renders the variables list using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *VariablesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/variables exactly
	if path == "/ui/variables" {
		switch r.Method {
		case http.MethodGet:
			h.List(w, r)
			return
		case http.MethodPost:
			h.Create(w, r)
			return
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	// Handle /ui/variables/new (create page)
	if path == "/ui/variables/new" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.ShowCreate(w, r)
		return
	}

	// Handle /ui/variables/{path} (detail page)
	if strings.HasPrefix(path, "/ui/variables/") {
		switch r.Method {
		case http.MethodGet:
			h.Detail(w, r)
			return
		case http.MethodPost, http.MethodPut:
			h.Update(w, r)
			return
		case http.MethodDelete:
			h.Delete(w, r)
			return
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	http.NotFound(w, r)
}

// List handles the variables list page
func (h *VariablesHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("variables list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	namespace := GetNamespaceFromRequest(r)

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Check permissions - need list capability for variables
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListVariables(namespace, "*") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to list variables",
			Capability: "variables: list",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'list' capability for variables",
				"Variables require specific path-based permissions",
			},
		})
		return
	}

	// Get variable data from the API.
	variablesList := h.getVariablesList(r, namespace, token)

	// Parse search query from query string
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))

	// Filter variables by search query if provided
	if searchQuery != "" {
		variablesList = h.filterVariables(variablesList, searchQuery)
	}

	// Parse sort parameters from query string
	sortColumn := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	// Default sort if not specified
	if sortColumn == "" {
		sortColumn = "path"
		sortDir = "asc"
	}

	// Sort the variables list based on parameters
	h.sortVariables(variablesList, sortColumn, sortDir)

	// Parse pagination parameters
	page := parsePage(r.URL.Query().Get("page"))
	perPage := parsePerPage(r.URL.Query().Get("per_page"))

	// Calculate total before pagination
	total := len(variablesList)

	// Calculate pagination metadata
	pagination := variablespartials.PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	}

	// Paginate the variables list
	variablesList = paginateVariables(variablesList, page, perPage)

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Variables - Wander",
		"View and manage Wander variables",
		"/ui/variables",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Variables", URL: ""},
		},
	)

	// Render the variables list page
	err := Render(w, r,
		variables.List(layoutProps, variablesList, sortColumn, sortDir, searchQuery, pagination),   // Full page with layout
		variablespartials.ListContent(variablesList, sortColumn, sortDir, searchQuery, pagination), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render variables list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getVariablesList fetches variables from the Nomad API.
func (h *VariablesHandler) getVariablesList(r *http.Request, namespace, token string) []variablespartials.VariableSummary {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return nil
	}

	// Build query options with namespace and token
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching variables from API",
		"namespace", namespace,
		"hasToken", token != "",
	)

	// Fetch variables from the API
	varsList, _, err := client.Variables().List(queryOpts)
	if err != nil {
		h.handler.logger.Warn("failed to fetch variables from API", "error", err)
		return nil
	}

	h.handler.logger.Debug("fetched variables from API", "count", len(varsList))

	// Convert API response to template format
	result := make([]variablespartials.VariableSummary, 0, len(varsList))
	for _, v := range varsList {
		// Convert ModifyTime (unix nano) to time.Time
		modifiedTime := time.Unix(0, v.ModifyTime)

		// Extract JobID from path if it follows the pattern nomad/jobs/{jobid}
		jobID := ""
		if strings.HasPrefix(v.Path, "nomad/jobs/") {
			parts := strings.Split(v.Path, "/")
			if len(parts) >= 3 {
				jobID = parts[2]
			}
		}

		// Note: The API returns VariableMetadata which doesn't include item count
		// We would need to fetch the full variable to get item count, but that
		// would be expensive for a list view. For now, set to 0.
		// TODO: Consider adding item count to VariableMetadata in the API
		itemCount := 0

		result = append(result, variablespartials.VariableSummary{
			Path:        v.Path,
			Namespace:   v.Namespace,
			CreateIndex: v.CreateIndex,
			ModifyIndex: v.ModifyIndex,
			ItemCount:   itemCount,
			Modified:    modifiedTime,
			JobID:       jobID,
		})
	}

	return result
}

// filterVariables filters the variables list based on search query.
// The search is case-insensitive and matches against path and namespace.
func (h *VariablesHandler) filterVariables(variablesList []variablespartials.VariableSummary, query string) []variablespartials.VariableSummary {
	if query == "" {
		return variablesList
	}

	query = strings.ToLower(query)
	filtered := make([]variablespartials.VariableSummary, 0, len(variablesList))

	for _, variable := range variablesList {
		// Match against Path or Namespace (case-insensitive)
		if strings.Contains(strings.ToLower(variable.Path), query) ||
			strings.Contains(strings.ToLower(variable.Namespace), query) {
			filtered = append(filtered, variable)
		}
	}

	return filtered
}

// sortVariables sorts the variables list based on the specified column and direction.
func (h *VariablesHandler) sortVariables(variablesList []variablespartials.VariableSummary, sortColumn, sortDir string) {
	sort.Slice(variablesList, func(i, j int) bool {
		var less bool

		switch sortColumn {
		case "path":
			less = variablesList[i].Path < variablesList[j].Path
		case "namespace":
			less = variablesList[i].Namespace < variablesList[j].Namespace
		case "items":
			less = variablesList[i].ItemCount < variablesList[j].ItemCount
		case "modified":
			less = variablesList[i].Modified.Before(variablesList[j].Modified)
		default:
			// Default to sorting by path ascending
			less = variablesList[i].Path < variablesList[j].Path
		}

		// Reverse if descending
		if sortDir == "desc" {
			return !less
		}
		return less
	})
}

// paginateVariables returns a slice of variables for the requested page.
// Returns an empty slice if the page is beyond available data.
func paginateVariables(variables []variablespartials.VariableSummary, page, perPage int) []variablespartials.VariableSummary {
	start := (page - 1) * perPage
	if start >= len(variables) {
		return []variablespartials.VariableSummary{}
	}

	end := start + perPage
	if end > len(variables) {
		end = len(variables)
	}

	return variables[start:end]
}

// ShowCreate handles the variable create page
func (h *VariablesHandler) ShowCreate(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("variable create page request", "htmx", IsHTMXFromRequest(r))

	// Check permissions - need write capability to create variables
	namespace := GetNamespaceFromRequest(r)
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, "*") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to create variables",
			Capability: "variables: write",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'write' capability for variables",
				"Variables require specific path-based permissions",
			},
		})
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		"Create Variable - Wander",
		"Create a new Wander variable",
		"/ui/variables/new",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Variables", URL: "/ui/variables"},
			{Label: "Create", URL: ""},
		},
	)
	createProps := variablespartials.CreateProps{
		CurrentNamespace:    layoutProps.CurrentNamespace,
		AvailableNamespaces: layoutProps.AvailableNamespaces,
	}

	// Render the create page
	err := Render(w, r,
		variables.Create(layoutProps, createProps),   // Full page with layout
		variablespartials.CreateContent(createProps), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render variable create page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Create handles POST requests to create a new variable
func (h *VariablesHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Info("variable create request")

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get form values
	path := strings.TrimSpace(r.FormValue("path"))
	namespace := strings.TrimSpace(r.FormValue("namespace"))

	// Default namespace if not provided
	if namespace == "" {
		namespace = GetNamespaceFromRequest(r)
	}

	// Check permissions - need write capability to create variables
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, path) {
		h.handler.logger.Warn("create variable permission denied", "namespace", namespace, "path", path)
		http.Error(w, "Permission denied: you do not have the 'write' capability for variables", http.StatusForbidden)
		return
	}

	// Validate path (required)
	if path == "" {
		h.handler.logger.Error("path is required")
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Validate path format (basic validation)
	if strings.Contains(path, " ") {
		h.handler.logger.Error("path contains spaces", "path", path)
		http.Error(w, "Path cannot contain spaces", http.StatusBadRequest)
		return
	}

	client, writeOpts, _, items, ok := h.variableItemsWriteContext(w, r, namespace)
	if !ok {
		return
	}

	// Build the variable
	variable := &api.Variable{
		Namespace: namespace,
		Path:      path,
		Items:     items,
	}

	// Create the variable via Nomad API
	createdVar, _, err := client.Variables().Create(variable, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to create variable", "error", err, "path", path, "namespace", namespace)
		http.Error(w, "Failed to create variable: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("variable created successfully", "path", createdVar.Path, "namespace", createdVar.Namespace, "itemCount", len(createdVar.Items))

	// Redirect to the detail page for the new variable
	redirectQuery := url.Values{"namespace": {createdVar.Namespace}}
	w.Header().Set("HX-Redirect", "/ui/variables/"+url.PathEscape(createdVar.Path)+"?"+redirectQuery.Encode())
	w.WriteHeader(http.StatusCreated)
}

// Detail handles the variable detail/edit page
func (h *VariablesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract variable path from URL (URL-encoded)
	variablePathEncoded := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePathEncoded = strings.TrimSuffix(variablePathEncoded, "/")

	// URL decode the path
	variablePath, err := url.PathUnescape(variablePathEncoded)
	if err != nil {
		h.handler.logger.Error("failed to decode variable path", "error", err, "path", variablePathEncoded)
		http.Error(w, "Invalid variable path", http.StatusBadRequest)
		return
	}

	// Get namespace from query param
	namespace := GetNamespaceFromRequest(r)

	// Log the request
	h.handler.logger.Debug("variable detail request", "path", variablePath, "namespace", namespace, "htmx", IsHTMXFromRequest(r))

	// Check permissions - need read capability to view variables
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadVariables(namespace, variablePath) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to read this variable",
			Capability: "variables: read",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'read' capability for this variable path",
				"Variable permissions are path-specific",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get variable detail from the API.
	variableDetail := h.getVariableDetail(r, variablePath, namespace, token)

	// If not found, return 404
	if variableDetail == nil {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
		variableDetail.Path+" - Variable - Wander",
		"View and edit variable "+variableDetail.Path,
		"/ui/variables/"+variablePathEncoded,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Variables", URL: "/ui/variables"},
			{Label: variableDetail.Path, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err = Render(w, r,
		variables.Detail(layoutProps, *variableDetail),   // Full page with layout
		variablespartials.DetailContent(*variableDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render variable detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Update handles POST/PUT requests to update a variable
func (h *VariablesHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Extract variable path from URL (URL-encoded)
	variablePathEncoded := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePathEncoded = strings.TrimSuffix(variablePathEncoded, "/")

	// URL decode the path
	variablePath, err := url.PathUnescape(variablePathEncoded)
	if err != nil {
		h.handler.logger.Error("failed to decode variable path", "error", err, "path", variablePathEncoded)
		http.Error(w, "Invalid variable path", http.StatusBadRequest)
		return
	}

	// Log the request
	h.handler.logger.Info("variable update request", "path", variablePath)

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get namespace from query param or form
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = strings.TrimSpace(r.FormValue("namespace"))
	}
	if namespace == "" {
		namespace = GetNamespaceFromRequest(r)
	}

	// Check permissions - need write capability to update variables
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, variablePath) {
		h.handler.logger.Warn("update variable permission denied", "path", variablePath, "namespace", namespace)
		http.Error(w, "Permission denied: you do not have the 'write' capability for this variable path", http.StatusForbidden)
		return
	}

	client, writeOpts, token, items, ok := h.variableItemsWriteContext(w, r, namespace)
	if !ok {
		return
	}

	// First, read the existing variable to get its ModifyIndex (for proper CAS updates)
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}
	if token != "" {
		queryOpts.AuthToken = token
	}

	existingVar, _, err := client.Variables().Read(variablePath, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to read existing variable", "error", err, "path", variablePath, "namespace", namespace)
		http.Error(w, "Failed to read existing variable: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build the updated variable with the existing ModifyIndex
	variable := &api.Variable{
		Namespace:   namespace,
		Path:        variablePath,
		Items:       items,
		ModifyIndex: existingVar.ModifyIndex,
	}

	// Update the variable via Nomad API
	updatedVar, _, err := client.Variables().Update(variable, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to update variable", "error", err, "path", variablePath, "namespace", namespace)
		http.Error(w, "Failed to update variable: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("variable updated successfully", "path", updatedVar.Path, "namespace", updatedVar.Namespace, "itemCount", len(updatedVar.Items))

	// Return success response for HTMX
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<div class="rounded-lg bg-green-50 border border-green-200 p-4">
		<div class="flex items-center">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-green-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>
			</div>
			<div class="ml-3">
				<p class="text-sm font-medium text-green-800">Variable saved successfully</p>
			</div>
		</div>
	</div>`))
}

// Delete handles DELETE requests to delete a variable
func (h *VariablesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Extract variable path from URL (URL-encoded)
	variablePathEncoded := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePathEncoded = strings.TrimSuffix(variablePathEncoded, "/")

	// URL decode the path
	variablePath, err := url.PathUnescape(variablePathEncoded)
	if err != nil {
		h.handler.logger.Error("failed to decode variable path", "error", err, "path", variablePathEncoded)
		http.Error(w, "Invalid variable path", http.StatusBadRequest)
		return
	}

	// Get namespace from query param
	namespace := GetNamespaceFromRequest(r)

	// Log the request
	h.handler.logger.Info("variable delete request", "path", variablePath, "namespace", namespace)

	// Check permissions - need destroy capability to delete variables
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanDestroyVariables(namespace, variablePath) {
		h.handler.logger.Warn("delete variable permission denied", "path", variablePath, "namespace", namespace)
		http.Error(w, "Permission denied: you do not have the 'destroy' capability for this variable path", http.StatusForbidden)
		return
	}

	client, writeOpts, _, ok := h.variableWriteContext(w, r, namespace)
	if !ok {
		return
	}

	// Delete the variable via Nomad API
	_, err = client.Variables().Delete(variablePath, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to delete variable", "error", err, "path", variablePath, "namespace", namespace)
		http.Error(w, "Failed to delete variable: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("variable deleted successfully", "path", variablePath, "namespace", namespace)

	// Return success response that redirects to list
	w.Header().Set("HX-Redirect", "/ui/variables")
	w.WriteHeader(http.StatusOK)
}

func (h *VariablesHandler) variableWriteContext(
	w http.ResponseWriter,
	r *http.Request,
	namespace string,
) (*api.Client, *api.WriteOptions, string, bool) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		http.Error(w, "Wander client not available", http.StatusInternalServerError)
		return nil, nil, "", false
	}

	token := GetTokenFromRequest(r)
	writeOpts := &api.WriteOptions{Namespace: namespace, AuthToken: token}
	return client, writeOpts, token, true
}

func (h *VariablesHandler) variableItemsWriteContext(
	w http.ResponseWriter,
	r *http.Request,
	namespace string,
) (*api.Client, *api.WriteOptions, string, map[string]string, bool) {
	client, writeOpts, token, ok := h.variableWriteContext(w, r, namespace)
	if !ok {
		return nil, nil, "", nil, false
	}
	return client, writeOpts, token, parseVariableItems(r.Form), true
}

// getVariableDetail fetches a variable by path from the Nomad API.
func (h *VariablesHandler) getVariableDetail(r *http.Request, path, namespace, token string) *variablespartials.VariableDetail {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return nil
	}

	// Build query options with namespace and token
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching variable from API",
		"path", path,
		"namespace", namespace,
		"hasToken", token != "",
	)

	// Fetch the variable from the API
	variable, _, err := client.Variables().Read(path, queryOpts)
	if err != nil {
		// Check if it's a 404 (not found) error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			h.handler.logger.Debug("variable not found", "path", path, "namespace", namespace)
			return nil
		}
		h.handler.logger.Warn("failed to fetch variable from API", "error", err, "path", path)
		return nil
	}

	h.handler.logger.Debug("fetched variable from API", "path", path, "itemCount", len(variable.Items))

	// Convert ModifyTime (unix nano) to time.Time
	modifiedTime := time.Unix(0, variable.ModifyTime)

	// Convert api.Variable to template format
	return &variablespartials.VariableDetail{
		Path:        variable.Path,
		Namespace:   variable.Namespace,
		CreateIndex: variable.CreateIndex,
		ModifyIndex: variable.ModifyIndex,
		Modified:    modifiedTime,
		Items:       variable.Items,
	}
}

func parseVariableItems(form url.Values) map[string]string {
	items := make(map[string]string)
	for field, values := range form {
		if !strings.HasPrefix(field, "key_") || len(values) == 0 {
			continue
		}

		itemKey := strings.TrimSpace(values[0])
		itemValues := form["value_"+strings.TrimPrefix(field, "key_")]
		if itemKey != "" && len(itemValues) > 0 {
			items[itemKey] = itemValues[0]
		}
	}
	return items
}
