package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/evaluations"
	"github.com/open-wander/wander/ui-htmx/templates/partials/evaluationspartials"
)

// EvaluationsHandler handles requests for the evaluations panel.
type EvaluationsHandler struct {
	handler *Handler
}

// NewEvaluationsHandler creates a new evaluations handler.
func NewEvaluationsHandler(h *Handler) *EvaluationsHandler {
	return &EvaluationsHandler{
		handler: h,
	}
}

// ServeHTTP handles GET requests to the evaluations endpoint.
func (h *EvaluationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/evaluations exactly (list page)
	if path == "/ui/evaluations" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.List(w, r)
		return
	}

	// Handle /ui/evaluations/{id} (detail page)
	if strings.HasPrefix(path, "/ui/evaluations/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// EvaluationSummary represents a summary view of an evaluation for processing.
type EvaluationSummary struct {
	ID          string
	JobID       string
	Namespace   string
	Status      string
	Type        string
	TriggeredBy string
	Priority    int
	CreateTime  time.Time
	NodeID      string
}

// List handles the evaluations list request and returns the partial content
func (h *EvaluationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("evaluations list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	query := r.URL.Query()
	namespace := query.Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	statusFilter := query.Get("status")
	typeFilter := query.Get("type")
	jobFilter := query.Get("job")
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")
	pageStr := query.Get("page")
	perPageStr := query.Get("per_page")

	// Default sorting
	if sortColumn == "" {
		sortColumn = "created"
	}
	if sortDir == "" {
		sortDir = "desc"
	}

	// Default pagination
	page := 1
	perPage := 25
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all evaluations with server-side filtering
	allEvaluations := h.getEvaluationsList(namespace, token, statusFilter, typeFilter, jobFilter)

	// Apply client-side search filter (status/type/job already filtered server-side)
	filteredEvaluations := h.filterEvaluations(allEvaluations, "", "", "", searchQuery)

	// Apply sorting
	h.sortEvaluations(filteredEvaluations, sortColumn, sortDir)

	// Calculate pagination
	total := len(filteredEvaluations)
	totalPages := (total + perPage - 1) / perPage // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// Apply pagination
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > total {
		endIdx = total
	}
	if startIdx > total {
		startIdx = total
	}

	paginatedEvaluations := filteredEvaluations[startIdx:endIdx]

	// Convert to template format
	evalsList := make([]evaluationspartials.EvaluationSummary, len(paginatedEvaluations))
	for i, eval := range paginatedEvaluations {
		// Get short ID (first 8 characters)
		shortID := eval.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Calculate time ago from CreateTime
		timeAgo := formatTimeAgo(eval.CreateTime)

		evalsList[i] = evaluationspartials.EvaluationSummary{
			ID:          eval.ID,
			ShortID:     shortID,
			JobID:       eval.JobID,
			JobName:     eval.JobID, // JobName is same as JobID in the API
			Status:      eval.Status,
			Type:        eval.Type,
			TriggeredBy: eval.TriggeredBy,
			Priority:    eval.Priority,
			TimeAgo:     timeAgo,
		}
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Evaluations - Wander",
		"View Nomad evaluations",
		"/ui/evaluations",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Evaluations", URL: ""},
		},
	)

	// Check if this is the evaluations panel request (should not push URL)
	// The panel targets #panel-content-evaluations
	hxTarget := r.Header.Get("HX-Target")
	isPanelRequest := strings.Contains(hxTarget, "panel-content")

	if isPanelRequest {
		// Panel requests should never push URL - use RenderPartial
		err := RenderPartial(w, r, evaluationspartials.ListContent(evalsList))
		if err != nil {
			h.handler.logger.Error("failed to render evaluations panel", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		evaluations.List(layoutProps, evalsList),       // Full page with layout
		evaluationspartials.ListContent(evalsList),     // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render evaluations list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Detail handles the evaluation detail request
func (h *EvaluationsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract evaluation ID from path
	evalID := strings.TrimPrefix(r.URL.Path, "/ui/evaluations/")
	evalID = strings.TrimSuffix(evalID, "/")

	// Log the request
	h.handler.logger.Debug("evaluation detail request", "evalID", evalID, "htmx", IsHTMXFromRequest(r))

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get evaluation detail
	evalDetail, found := h.getEvaluationDetail(evalID, token)
	if !found {
		http.Error(w, "Evaluation not found", http.StatusNotFound)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Evaluation "+evalDetail.ShortID+" - Wander",
		"View Nomad evaluation details",
		"/ui/evaluations/"+evalID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Evaluations", URL: "/ui/evaluations"},
			{Label: evalDetail.ShortID, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		evaluations.Detail(layoutProps, evalDetail),           // Full page with layout
		evaluationspartials.DetailContent(evalDetail),         // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render evaluation detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getEvaluationDetail fetches a single evaluation from the Nomad API.
func (h *EvaluationsHandler) getEvaluationDetail(evalID string, token string) (evaluationspartials.EvaluationDetail, bool) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty evaluation detail")
		return evaluationspartials.EvaluationDetail{ID: evalID}, false
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch evaluation from the API
	eval, _, err := client.Evaluations().Info(evalID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch evaluation from Nomad API", "error", err, "evalID", evalID)
		return evaluationspartials.EvaluationDetail{ID: evalID}, false
	}

	// Get short ID (first 8 characters)
	shortID := evalID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Parse timestamps
	createTime := time.Time{}
	if eval.CreateTime > 0 {
		createTime = time.Unix(0, eval.CreateTime)
	}
	modifyTime := time.Time{}
	if eval.ModifyTime > 0 {
		modifyTime = time.Unix(0, eval.ModifyTime)
	}

	// Convert failed task group allocations
	failedTGAllocs := make(map[string]evaluationspartials.AllocationMetric)
	if eval.FailedTGAllocs != nil {
		for tgName, metrics := range eval.FailedTGAllocs {
			failedTGAllocs[tgName] = evaluationspartials.AllocationMetric{
				NodesEvaluated:     metrics.NodesEvaluated,
				NodesFiltered:      metrics.NodesFiltered,
				NodesAvailable:     metrics.NodesAvailable,
				NodesExhausted:     metrics.NodesExhausted,
				ClassFiltered:      metrics.ClassFiltered,
				ConstraintFiltered: metrics.ConstraintFiltered,
				ClassExhausted:     metrics.ClassExhausted,
				DimensionExhausted: metrics.DimensionExhausted,
				QuotaExhausted:     metrics.QuotaExhausted,
				ResourcesExhausted: convertResourcesMap(metrics.ResourcesExhausted),
				Scores:             metrics.Scores,
				ScoreMetaData:      convertScoreMetaData(metrics.ScoreMetaData),
			}
		}
	}

	// Build detail struct
	detail := evaluationspartials.EvaluationDetail{
		ID:                eval.ID,
		ShortID:           shortID,
		JobID:             eval.JobID,
		Namespace:         eval.Namespace,
		Status:            eval.Status,
		StatusDescription: eval.StatusDescription,
		Type:              eval.Type,
		TriggeredBy:       eval.TriggeredBy,
		Priority:          eval.Priority,
		CreateTime:        createTime,
		CreateTimeAgo:     formatTimeAgo(createTime),
		ModifyTime:        modifyTime,
		ModifyTimeAgo:     formatTimeAgo(modifyTime),
		NodeID:            eval.NodeID,
		DeploymentID:      eval.DeploymentID,
		NextEval:          eval.NextEval,
		PreviousEval:      eval.PreviousEval,
		BlockedEval:       eval.BlockedEval,
		FailedTGAllocs:    failedTGAllocs,
		QueuedAllocations: eval.QueuedAllocations,
	}

	return detail, true
}

// convertResourcesMap converts API Resources to template Resources
func convertResourcesMap(resources map[string]*api.Resources) map[string]*evaluationspartials.Resources {
	if resources == nil {
		return nil
	}
	result := make(map[string]*evaluationspartials.Resources)
	for k, v := range resources {
		if v != nil {
			result[k] = &evaluationspartials.Resources{
				CPU:      *v.CPU,
				MemoryMB: *v.MemoryMB,
				DiskMB:   *v.DiskMB,
			}
		}
	}
	return result
}

// convertScoreMetaData converts API NodeScoreMeta to template NodeScoreMeta
func convertScoreMetaData(scores []*api.NodeScoreMeta) []*evaluationspartials.NodeScoreMeta {
	if scores == nil {
		return nil
	}
	result := make([]*evaluationspartials.NodeScoreMeta, len(scores))
	for i, score := range scores {
		if score != nil {
			result[i] = &evaluationspartials.NodeScoreMeta{
				NodeID:    score.NodeID,
				Scores:    score.Scores,
				NormScore: score.NormScore,
			}
		}
	}
	return result
}

// getEvaluationsList fetches evaluations from the Nomad API with server-side filtering.
// Falls back to empty list if API is unavailable.
func (h *EvaluationsHandler) getEvaluationsList(namespace, token, statusFilter, typeFilter, jobFilter string) []EvaluationSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty evaluations list")
		return []EvaluationSummary{}
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Build Nomad filter expression for server-side filtering
	var filters []string
	if statusFilter != "" {
		filters = append(filters, fmt.Sprintf("Status == %q", statusFilter))
	}
	if typeFilter != "" {
		filters = append(filters, fmt.Sprintf("Type == %q", typeFilter))
	}
	if jobFilter != "" {
		filters = append(filters, fmt.Sprintf("JobID == %q", jobFilter))
	}
	if len(filters) > 0 {
		queryOpts.Filter = strings.Join(filters, " and ")
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching evaluations from API",
		"namespace", namespace,
		"filter", queryOpts.Filter,
		"hasToken", token != "",
	)

	// Fetch evaluations from the API
	evals, _, err := client.Evaluations().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch evaluations from Nomad API", "error", err)
		return []EvaluationSummary{}
	}

	h.handler.logger.Debug("fetched evaluations from API", "count", len(evals))

	// Convert API response to our summary type
	result := make([]EvaluationSummary, 0, len(evals))
	for _, eval := range evals {
		createTime := time.Time{}
		if eval.CreateTime > 0 {
			createTime = time.Unix(0, eval.CreateTime)
		}

		result = append(result, EvaluationSummary{
			ID:          eval.ID,
			JobID:       eval.JobID,
			Namespace:   eval.Namespace,
			Status:      eval.Status,
			Type:        eval.Type,
			TriggeredBy: eval.TriggeredBy,
			Priority:    eval.Priority,
			CreateTime:  createTime,
			NodeID:      eval.NodeID,
		})
	}

	return result
}

// filterEvaluations applies client-side filtering to the evaluations list.
// Note: status, type, and job filters are already applied server-side.
func (h *EvaluationsHandler) filterEvaluations(evals []EvaluationSummary, status, evalType, job, search string) []EvaluationSummary {
	if search == "" && status == "" && evalType == "" && job == "" {
		return evals
	}

	searchLower := strings.ToLower(strings.TrimSpace(search))
	result := make([]EvaluationSummary, 0, len(evals))

	for _, eval := range evals {
		// Apply status filter if provided (usually already handled server-side)
		if status != "" && eval.Status != status {
			continue
		}

		// Apply type filter if provided (usually already handled server-side)
		if evalType != "" && eval.Type != evalType {
			continue
		}

		// Apply job filter if provided (usually already handled server-side)
		if job != "" && eval.JobID != job {
			continue
		}

		// Apply search filter across multiple fields
		if searchLower != "" {
			idLower := strings.ToLower(eval.ID)
			jobLower := strings.ToLower(eval.JobID)
			statusLower := strings.ToLower(eval.Status)
			typeLower := strings.ToLower(eval.Type)
			triggeredByLower := strings.ToLower(eval.TriggeredBy)
			nodeLower := strings.ToLower(eval.NodeID)

			if !strings.Contains(idLower, searchLower) &&
				!strings.Contains(jobLower, searchLower) &&
				!strings.Contains(statusLower, searchLower) &&
				!strings.Contains(typeLower, searchLower) &&
				!strings.Contains(triggeredByLower, searchLower) &&
				!strings.Contains(nodeLower, searchLower) {
				continue
			}
		}

		result = append(result, eval)
	}

	return result
}

// sortEvaluations sorts the evaluations list by the specified column
func (h *EvaluationsHandler) sortEvaluations(evals []EvaluationSummary, column, dir string) {
	sort.Slice(evals, func(i, j int) bool {
		var less bool

		switch column {
		case "id":
			less = evals[i].ID < evals[j].ID
		case "job":
			less = evals[i].JobID < evals[j].JobID
		case "status":
			less = evals[i].Status < evals[j].Status
		case "type":
			less = evals[i].Type < evals[j].Type
		case "triggered":
			less = evals[i].TriggeredBy < evals[j].TriggeredBy
		case "priority":
			less = evals[i].Priority < evals[j].Priority
		case "created":
			less = evals[i].CreateTime.Before(evals[j].CreateTime)
		default:
			// Default to sorting by created time
			less = evals[i].CreateTime.Before(evals[j].CreateTime)
		}

		if dir == "desc" {
			return !less
		}
		return less
	})
}

// formatTimeAgo formats a time as a human-readable "time ago" string.
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	if d < 0 {
		return "just now"
	}

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return formatDuration(mins, "m") + " ago"
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return formatDuration(hours, "h") + " ago"
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return formatDuration(days, "d") + " ago"
	}
}

func formatDuration(n int, unit string) string {
	return fmt.Sprintf("%d%s", n, unit)
}
