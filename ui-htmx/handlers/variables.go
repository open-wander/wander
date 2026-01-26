package handlers

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/variables"
	"github.com/open-wander/wander/ui-htmx/templates/partials/variablespartials"
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

	// Get mock variable data
	variablesList := h.getVariablesList()

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
	pagination := PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	}

	// Paginate the variables list
	variablesList = paginateVariables(variablesList, page, perPage)

	// Render the variables list page
	err := Render(w, r,
		variables.List(variablesList, sortColumn, sortDir, searchQuery, pagination),                // Full page with layout
		variablespartials.ListContent(variablesList, sortColumn, sortDir, searchQuery, pagination), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render variables list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getVariablesList returns mock variable data.
// This will be replaced with actual Nomad API calls in the future.
func (h *VariablesHandler) getVariablesList() []variablespartials.VariableSummary {
	now := time.Now()

	return []variablespartials.VariableSummary{
		{
			Path:        "nomad/jobs",
			Namespace:   "default",
			CreateIndex: 1000,
			ModifyIndex: 1250,
			ItemCount:   12,
			Modified:    now.Add(-24 * time.Hour),
		},
		{
			Path:        "nomad/jobs/web",
			Namespace:   "default",
			CreateIndex: 1100,
			ModifyIndex: 1300,
			ItemCount:   8,
			Modified:    now.Add(-2 * time.Hour),
		},
		{
			Path:        "nomad/jobs/api",
			Namespace:   "default",
			CreateIndex: 1150,
			ModifyIndex: 1350,
			ItemCount:   6,
			Modified:    now.Add(-5 * time.Hour),
		},
		{
			Path:        "app/config",
			Namespace:   "default",
			CreateIndex: 2000,
			ModifyIndex: 2100,
			ItemCount:   15,
			Modified:    now.Add(-12 * time.Hour),
		},
		{
			Path:        "app/secrets",
			Namespace:   "default",
			CreateIndex: 2050,
			ModifyIndex: 2150,
			ItemCount:   4,
			Modified:    now.Add(-3 * 24 * time.Hour),
		},
		{
			Path:        "app/database",
			Namespace:   "default",
			CreateIndex: 2100,
			ModifyIndex: 2200,
			ItemCount:   7,
			Modified:    now.Add(-48 * time.Hour),
		},
		{
			Path:        "production/frontend",
			Namespace:   "production",
			CreateIndex: 3000,
			ModifyIndex: 3050,
			ItemCount:   10,
			Modified:    now.Add(-6 * time.Hour),
		},
		{
			Path:        "production/backend",
			Namespace:   "production",
			CreateIndex: 3100,
			ModifyIndex: 3150,
			ItemCount:   9,
			Modified:    now.Add(-8 * time.Hour),
		},
		{
			Path:        "staging/config",
			Namespace:   "staging",
			CreateIndex: 4000,
			ModifyIndex: 4050,
			ItemCount:   5,
			Modified:    now.Add(-72 * time.Hour),
		},
		{
			Path:        "shared/certificates",
			Namespace:   "default",
			CreateIndex: 5000,
			ModifyIndex: 5100,
			ItemCount:   3,
			Modified:    now.Add(-96 * time.Hour),
		},
	}
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

	// Render the create page
	err := Render(w, r,
		variables.Create(),                // Full page with layout
		variablespartials.CreateContent(), // Partial content for HTMX requests
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

	// Default namespace if not provided
	if namespace == "" {
		namespace = "default"
	}

	// TODO: In a real implementation, this would:
	// 1. Parse the key-value pairs from the form
	// 2. Call the Nomad API: client.Variables().Create(variable, nil)
	// 3. Return success/error response

	// For now, simulate success
	h.handler.logger.Info("variable created successfully", "path", path, "namespace", namespace)

	// Redirect to the detail page for the new variable
	w.Header().Set("HX-Redirect", "/ui/variables/"+path)
	w.WriteHeader(http.StatusCreated)
}

// Detail handles the variable detail/edit page
func (h *VariablesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract variable path from URL
	variablePath := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePath = strings.TrimSuffix(variablePath, "/")

	// Log the request
	h.handler.logger.Debug("variable detail request", "path", variablePath, "htmx", IsHTMXFromRequest(r))

	// Get mock variable detail data
	variableDetail := h.getVariableDetail(variablePath)

	// If not found, return 404
	if variableDetail == nil {
		http.NotFound(w, r)
		return
	}

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		variables.Detail(*variableDetail),                // Full page with layout
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
	// Extract variable path from URL
	variablePath := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePath = strings.TrimSuffix(variablePath, "/")

	// Log the request
	h.handler.logger.Info("variable update request", "path", variablePath)

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// TODO: In a real implementation, this would:
	// 1. Parse the key-value pairs from the form
	// 2. Call the Nomad API: client.Variables().Update(variable, nil)
	// 3. Return success/error response

	// For now, simulate success
	h.handler.logger.Info("variable updated successfully", "path", variablePath)

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
	// Extract variable path from URL
	variablePath := strings.TrimPrefix(r.URL.Path, "/ui/variables/")
	variablePath = strings.TrimSuffix(variablePath, "/")

	// Log the request
	h.handler.logger.Info("variable delete request", "path", variablePath)

	// TODO: In a real implementation, this would:
	// 1. Call the Nomad API: client.Variables().Delete(path, nil)
	// 2. Return success/error response

	// For now, simulate success
	h.handler.logger.Info("variable deleted successfully", "path", variablePath)

	// Return success response that redirects to list
	w.Header().Set("HX-Redirect", "/ui/variables")
	w.WriteHeader(http.StatusOK)
}

// getVariableDetail returns mock variable detail data.
// Returns nil if the variable is not found.
func (h *VariablesHandler) getVariableDetail(path string) *variablespartials.VariableDetail {
	now := time.Now()

	// Mock variable data - match some paths from the list
	variables := map[string]variablespartials.VariableDetail{
		"nomad/jobs": {
			Path:        "nomad/jobs",
			Namespace:   "default",
			CreateIndex: 1000,
			ModifyIndex: 1250,
			Modified:    now.Add(-24 * time.Hour),
			Items: map[string]string{
				"database_url":      "postgresql://localhost:5432/nomad",
				"redis_host":        "redis.service.consul",
				"api_timeout":       "30s",
				"max_connections":   "100",
				"enable_debugging":  "false",
			},
		},
		"nomad/jobs/web": {
			Path:        "nomad/jobs/web",
			Namespace:   "default",
			CreateIndex: 1100,
			ModifyIndex: 1300,
			Modified:    now.Add(-2 * time.Hour),
			Items: map[string]string{
				"port":             "8080",
				"log_level":        "info",
				"static_assets":    "/var/www/public",
				"session_timeout":  "3600",
			},
		},
		"nomad/jobs/api": {
			Path:        "nomad/jobs/api",
			Namespace:   "default",
			CreateIndex: 1150,
			ModifyIndex: 1350,
			Modified:    now.Add(-5 * time.Hour),
			Items: map[string]string{
				"api_key":          "test-key-12345",
				"rate_limit":       "1000",
				"cors_origins":     "*",
			},
		},
		"app/config": {
			Path:        "app/config",
			Namespace:   "default",
			CreateIndex: 2000,
			ModifyIndex: 2100,
			Modified:    now.Add(-12 * time.Hour),
			Items: map[string]string{
				"app_name":         "Wander",
				"environment":      "production",
				"feature_flags":    "new-ui,dark-mode",
				"maintenance_mode": "false",
				"version":          "1.0.0",
			},
		},
		"app/secrets": {
			Path:        "app/secrets",
			Namespace:   "default",
			CreateIndex: 2050,
			ModifyIndex: 2150,
			Modified:    now.Add(-3 * 24 * time.Hour),
			Items: map[string]string{
				"jwt_secret":       "super-secret-key",
				"encryption_key":   "aes-256-key",
				"api_token":        "Bearer token123",
			},
		},
	}

	if detail, ok := variables[path]; ok {
		return &detail
	}

	// Return a default variable for any other path (for testing)
	return &variablespartials.VariableDetail{
		Path:        path,
		Namespace:   "default",
		CreateIndex: 1000,
		ModifyIndex: 1000,
		Modified:    now,
		Items: map[string]string{
			"example_key": "example_value",
		},
	}
}
