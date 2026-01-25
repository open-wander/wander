package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/open-wander/wander/ui-htmx/templates/pages"
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

// ServeHTTP handles GET requests to the home page.
// It renders the home page using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Log the request
	h.handler.logger.Debug("home page request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Render the home page
	// For now, we'll use the example page as a placeholder
	// In the future, this will be replaced with a proper dashboard
	err := Render(w, r,
		pages.ExamplePage(),  // Full page with layout
		homeContent(),        // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render home page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// homeContent renders the main content for the home page.
// This is a placeholder that will be replaced with an actual dashboard.
func homeContent() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				<div class="bg-white shadow rounded-lg p-6">
					<h2 class="text-2xl font-bold text-neutral-900 mb-4">
						Welcome to Wander
					</h2>
					<p class="text-neutral-600 mb-4">
						This is the home page. The dashboard will be implemented here.
					</p>
					<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
						<div class="border border-neutral-200 rounded-lg p-4">
							<h3 class="text-lg font-semibold text-neutral-900 mb-2">Jobs</h3>
							<p class="text-neutral-600 text-sm">View and manage jobs</p>
							<a href="/ui/jobs" class="text-primary-600 hover:text-primary-700 text-sm mt-2 inline-block">
								Go to Jobs →
							</a>
						</div>
						<div class="border border-neutral-200 rounded-lg p-4">
							<h3 class="text-lg font-semibold text-neutral-900 mb-2">Allocations</h3>
							<p class="text-neutral-600 text-sm">View task allocations</p>
							<a href="/ui/allocations" class="text-primary-600 hover:text-primary-700 text-sm mt-2 inline-block">
								Go to Allocations →
							</a>
						</div>
						<div class="border border-neutral-200 rounded-lg p-4">
							<h3 class="text-lg font-semibold text-neutral-900 mb-2">Clients</h3>
							<p class="text-neutral-600 text-sm">View client nodes</p>
							<a href="/ui/clients" class="text-primary-600 hover:text-primary-700 text-sm mt-2 inline-block">
								Go to Clients →
							</a>
						</div>
					</div>
				</div>
			</div>
		`)
		return err
	})
}
