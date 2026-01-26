package handlers

import (
	"net/http"

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

	// Render the evaluations list partial
	err := evaluationspartials.List(evalsList).Render(r.Context(), w)

	if err != nil {
		h.handler.logger.Error("failed to render evaluations list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getEvaluationsList returns mock evaluation data.
// This will be replaced with actual Nomad API calls in the future.
func (h *EvaluationsHandler) getEvaluationsList() []evaluationspartials.EvaluationSummary {
	return []evaluationspartials.EvaluationSummary{
		{
			ID:          "a1b2c3d4-e5f6-7890-1234-567890abcdef",
			ShortID:     "a1b2c3d4",
			JobID:       "web-frontend",
			JobName:     "web-frontend",
			Status:      "complete",
			Type:        "service",
			TriggeredBy: "job-register",
			Priority:    50,
			TimeAgo:     "5m ago",
		},
		{
			ID:          "b2c3d4e5-f6a7-8901-2345-678901bcdefg",
			ShortID:     "b2c3d4e5",
			JobID:       "api-backend",
			JobName:     "api-backend",
			Status:      "pending",
			Type:        "service",
			TriggeredBy: "scaling",
			Priority:    50,
			TimeAgo:     "2m ago",
		},
		{
			ID:          "c3d4e5f6-a7b8-9012-3456-789012cdefgh",
			ShortID:     "c3d4e5f6",
			JobID:       "batch-processor",
			JobName:     "batch-processor",
			Status:      "complete",
			Type:        "batch",
			TriggeredBy: "periodic",
			Priority:    30,
			TimeAgo:     "15m ago",
		},
		{
			ID:          "d4e5f6a7-b8c9-0123-4567-890123defghi",
			ShortID:     "d4e5f6a7",
			JobID:       "data-pipeline",
			JobName:     "data-pipeline",
			Status:      "failed",
			Type:        "batch",
			TriggeredBy: "job-register",
			Priority:    40,
			TimeAgo:     "30m ago",
		},
		{
			ID:          "e5f6a7b8-c9d0-1234-5678-901234efghij",
			ShortID:     "e5f6a7b8",
			JobID:       "cache-service",
			JobName:     "cache-service",
			Status:      "blocked",
			Type:        "service",
			TriggeredBy: "node-update",
			Priority:    50,
			TimeAgo:     "1h ago",
		},
		{
			ID:          "f6a7b8c9-d0e1-2345-6789-012345fghijk",
			ShortID:     "f6a7b8c9",
			JobID:       "metrics-collector",
			JobName:     "metrics-collector",
			Status:      "complete",
			Type:        "system",
			TriggeredBy: "deployment-watcher",
			Priority:    50,
			TimeAgo:     "2h ago",
		},
		{
			ID:          "a7b8c9d0-e1f2-3456-7890-123456ghijkl",
			ShortID:     "a7b8c9d0",
			JobID:       "worker-pool",
			JobName:     "worker-pool",
			Status:      "complete",
			Type:        "service",
			TriggeredBy: "job-register",
			Priority:    50,
			TimeAgo:     "3h ago",
		},
		{
			ID:          "b8c9d0e1-f2a3-4567-8901-234567hijklm",
			ShortID:     "b8c9d0e1",
			JobID:       "log-aggregator",
			JobName:     "log-aggregator",
			Status:      "canceled",
			Type:        "service",
			TriggeredBy: "job-deregister",
			Priority:    50,
			TimeAgo:     "5h ago",
		},
		{
			ID:          "c9d0e1f2-a3b4-5678-9012-345678ijklmn",
			ShortID:     "c9d0e1f2",
			JobID:       "redis-cluster",
			JobName:     "redis-cluster",
			Status:      "complete",
			Type:        "service",
			TriggeredBy: "alloc-stop",
			Priority:    50,
			TimeAgo:     "6h ago",
		},
		{
			ID:          "d0e1f2a3-b4c5-6789-0123-456789jklmno",
			ShortID:     "d0e1f2a3",
			JobID:       "nginx-proxy",
			JobName:     "nginx-proxy",
			Status:      "complete",
			Type:        "service",
			TriggeredBy: "periodic",
			Priority:    50,
			TimeAgo:     "12h ago",
		},
		{
			ID:          "e1f2a3b4-c5d6-7890-1234-567890klmnop",
			ShortID:     "e1f2a3b4",
			JobID:       "db-backup",
			JobName:     "db-backup",
			Status:      "complete",
			Type:        "batch",
			TriggeredBy: "periodic",
			Priority:    20,
			TimeAgo:     "18h ago",
		},
		{
			ID:          "f2a3b4c5-d6e7-8901-2345-678901lmnopq",
			ShortID:     "f2a3b4c5",
			JobID:       "monitoring-agent",
			JobName:     "monitoring-agent",
			Status:      "pending",
			Type:        "system",
			TriggeredBy: "node-drain",
			Priority:    50,
			TimeAgo:     "1d ago",
		},
		{
			ID:          "a3b4c5d6-e7f8-9012-3456-789012mnopqr",
			ShortID:     "a3b4c5d6",
			JobID:       "etl-pipeline",
			JobName:     "etl-pipeline",
			Status:      "complete",
			Type:        "batch",
			TriggeredBy: "periodic",
			Priority:    30,
			TimeAgo:     "2d ago",
		},
	}
}
