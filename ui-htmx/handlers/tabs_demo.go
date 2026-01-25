package handlers

import (
	"net/http"

	"github.com/open-wander/wander/ui-htmx/templates/pages"
	"github.com/open-wander/wander/ui-htmx/templates/partials"
)

// TabsDemoHandler handles the tabs demo page and HTMX tab endpoints.
type TabsDemoHandler struct{}

// NewTabsDemoHandler creates a new tabs demo handler instance.
func NewTabsDemoHandler() *TabsDemoHandler {
	return &TabsDemoHandler{}
}

// Index renders the tabs demo page.
//
// Route: GET /ui/demo/tabs
func (h *TabsDemoHandler) Index(w http.ResponseWriter, r *http.Request) {
	// For the demo page, we always render the full page
	err := RenderFullPage(w, r, pages.TabsDemoPage())
	if err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

// JobsTab returns the jobs tab content via HTMX.
//
// Route: GET /ui/demo/tabs/jobs
func (h *TabsDemoHandler) JobsTab(w http.ResponseWriter, r *http.Request) {
	// Always render partial for tab content
	err := RenderPartial(w, r, partials.TabsDemo_JobsTab())
	if err != nil {
		http.Error(w, "Failed to render tab", http.StatusInternalServerError)
		return
	}
}

// AllocationsTab returns the allocations tab content via HTMX.
//
// Route: GET /ui/demo/tabs/allocations
func (h *TabsDemoHandler) AllocationsTab(w http.ResponseWriter, r *http.Request) {
	// Simulate slight delay for demonstration
	// time.Sleep(500 * time.Millisecond)

	err := RenderPartial(w, r, partials.TabsDemo_AllocationsTab())
	if err != nil {
		http.Error(w, "Failed to render tab", http.StatusInternalServerError)
		return
	}
}

// HistoryTab returns the history tab content via HTMX.
//
// Route: GET /ui/demo/tabs/history
func (h *TabsDemoHandler) HistoryTab(w http.ResponseWriter, r *http.Request) {
	// Simulate slight delay for demonstration
	// time.Sleep(500 * time.Millisecond)

	err := RenderPartial(w, r, partials.TabsDemo_HistoryTab())
	if err != nil {
		http.Error(w, "Failed to render tab", http.StatusInternalServerError)
		return
	}
}
