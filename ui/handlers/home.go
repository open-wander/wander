package handlers

import (
	"net/http"
)

// HomeHandler handles the home/dashboard page at /ui/
type HomeHandler struct {
	handler *Handler
}

// NewHomeHandler creates a new home page handler.
func NewHomeHandler(h *Handler) *HomeHandler {
	return &HomeHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the home page.
// It redirects to the jobs page which is the default landing page.
func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle exact /ui/ path - other paths are handled by other handlers
	if r.URL.Path != "/ui/" && r.URL.Path != "/ui" {
		http.NotFound(w, r)
		return
	}

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Redirect to jobs page (the default landing page)
	http.Redirect(w, r, "/ui/jobs", http.StatusFound)
}
