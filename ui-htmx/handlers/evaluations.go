package handlers

import (
	"fmt"
	"net/http"
	"time"

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
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.List(w, r)
}

// List handles the evaluations list request and returns the partial content
func (h *EvaluationsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("evaluations list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get mock evaluation data
	evalsList := h.getEvaluationsList()

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

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		evaluations.List(layoutProps, evalsList),                    // Full page with layout
		evaluationspartials.ListContent(evalsList),    // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render evaluations list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getEvaluationsList fetches evaluations from the Nomad API.
func (h *EvaluationsHandler) getEvaluationsList() []evaluationspartials.EvaluationSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty evaluations list")
		return []evaluationspartials.EvaluationSummary{}
	}

	// Fetch evaluations from the API
	evals, _, err := client.Evaluations().List(nil)
	if err != nil {
		h.handler.logger.Error("failed to fetch evaluations from Nomad API", "error", err)
		return []evaluationspartials.EvaluationSummary{}
	}

	if len(evals) == 0 {
		h.handler.logger.Debug("no evaluations returned from Nomad API")
		return []evaluationspartials.EvaluationSummary{}
	}

	result := make([]evaluationspartials.EvaluationSummary, 0, len(evals))
	for _, eval := range evals {
		// Get short ID (first 8 characters)
		shortID := eval.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Calculate time ago from CreateTime (in nanoseconds)
		timeAgo := formatTimeAgo(time.Unix(0, eval.CreateTime))

		result = append(result, evaluationspartials.EvaluationSummary{
			ID:          eval.ID,
			ShortID:     shortID,
			JobID:       eval.JobID,
			JobName:     eval.JobID, // JobName is same as JobID in the API
			Status:      eval.Status,
			Type:        eval.Type,
			TriggeredBy: eval.TriggeredBy,
			Priority:    eval.Priority,
			TimeAgo:     timeAgo,
		})
	}

	h.handler.logger.Debug("fetched evaluations from Nomad API", "count", len(result))
	return result
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
