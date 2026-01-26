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

	// Handle /ui/variables exactly (list page)
	if path == "/ui/variables" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/variables/{path} (detail page - to be implemented)
	if strings.HasPrefix(path, "/ui/variables/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// TODO: Implement detail page
		http.NotFound(w, r)
		return
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
