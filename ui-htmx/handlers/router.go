package handlers

import (
	"net/http"
)

// Router wraps http.ServeMux and provides route registration for the UI.
// It applies HTMX middleware to all routes and organizes routes under the /ui/ prefix.
type Router struct {
	mux     *http.ServeMux
	handler *Handler
}

// NewRouter creates a new Router with the provided handler dependencies.
// It sets up all UI routes and applies necessary middleware.
func NewRouter(h *Handler) *Router {
	r := &Router{
		mux:     http.NewServeMux(),
		handler: h,
	}

	// Register all routes
	r.registerRoutes()

	return r
}

// registerRoutes sets up all UI routes.
// Routes are organized by feature area and prefixed with /ui/
func (r *Router) registerRoutes() {
	// Static file serving - must be registered before other routes
	// to prevent conflicts with dynamic routes
	staticHandler := NewStaticHandler(r.handler)
	r.mux.Handle("/ui/static/", staticHandler.StripPrefix("/ui/static/"))

	// Home/Dashboard
	homeHandler := NewHomeHandler(r.handler)
	r.mux.Handle("/ui/", r.wrapMiddleware(homeHandler))

	// Jobs routes
	r.registerJobsRoutes()

	// Allocations routes
	r.registerAllocationsRoutes()

	// Clients routes
	r.registerClientsRoutes()

	// Servers routes
	r.registerServersRoutes()
}

// registerJobsRoutes sets up routes for the jobs section.
func (r *Router) registerJobsRoutes() {
	jobsHandler := NewJobsHandler(r.handler)

	// Jobs handler handles both list and detail pages
	r.mux.Handle("/ui/jobs", r.wrapMiddleware(jobsHandler))
	r.mux.Handle("/ui/jobs/", r.wrapMiddleware(jobsHandler))
}

// registerAllocationsRoutes sets up routes for the allocations section.
// Placeholder handlers will be replaced with actual implementations.
func (r *Router) registerAllocationsRoutes() {
	// Allocations list
	r.mux.Handle("/ui/allocations", r.wrapMiddleware(placeholderHandler("Allocations List")))

	// Allocation detail
	r.mux.Handle("/ui/allocations/", r.wrapMiddleware(placeholderHandler("Allocation Detail")))
}

// registerClientsRoutes sets up routes for the clients section.
// Placeholder handlers will be replaced with actual implementations.
func (r *Router) registerClientsRoutes() {
	// Clients list
	r.mux.Handle("/ui/clients", r.wrapMiddleware(placeholderHandler("Clients List")))

	// Client detail
	r.mux.Handle("/ui/clients/", r.wrapMiddleware(placeholderHandler("Client Detail")))
}

// registerServersRoutes sets up routes for the servers section.
// Placeholder handlers will be replaced with actual implementations.
func (r *Router) registerServersRoutes() {
	// Servers list
	r.mux.Handle("/ui/servers", r.wrapMiddleware(placeholderHandler("Servers List")))

	// Server detail
	r.mux.Handle("/ui/servers/", r.wrapMiddleware(placeholderHandler("Server Detail")))
}

// wrapMiddleware applies HTMX middleware and any other necessary middleware
// to the provided handler.
func (r *Router) wrapMiddleware(h http.Handler) http.Handler {
	// Apply HTMX middleware to add request context
	return HTMXMiddleware(h)
}

// ServeHTTP implements http.Handler, allowing Router to be used as an HTTP handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Handler returns the underlying http.ServeMux for advanced use cases.
// This allows the router to be composed with other routers or middleware.
func (r *Router) Handler() http.Handler {
	return r.mux
}

// placeholderHandler creates a simple placeholder handler for routes
// that haven't been implemented yet. It returns a basic HTML page with
// the route name.
func placeholderHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		isHTMX := IsHTMXFromRequest(r)

		if isHTMX {
			// Return just the content for HTMX requests
			if WantsHistoryPush(r) {
				SetHXPushURL(w, r.URL.String())
			}
			w.Write([]byte(`
				<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
					<div class="bg-white shadow rounded-lg p-6">
						<h2 class="text-2xl font-bold text-neutral-900 mb-4">` + name + `</h2>
						<p class="text-neutral-600">This page is coming soon.</p>
						<p class="text-sm text-neutral-500 mt-4">Path: ` + r.URL.Path + `</p>
					</div>
				</div>
			`))
		} else {
			// Return full page for non-HTMX requests
			w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>` + name + ` - Wander</title>
	<link rel="stylesheet" href="/ui/static/css/output.css">
	<script src="/ui/static/js/htmx.min.js"></script>
	<script src="/ui/static/js/alpine.min.js" defer></script>
</head>
<body class="bg-neutral-50">
	<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-2xl font-bold text-neutral-900 mb-4">` + name + `</h2>
			<p class="text-neutral-600">This page is coming soon.</p>
			<p class="text-sm text-neutral-500 mt-4">Path: ` + r.URL.Path + `</p>
			<div class="mt-4">
				<a href="/ui/" class="text-primary-600 hover:text-primary-700">← Back to Home</a>
			</div>
		</div>
	</div>
</body>
</html>`))
		}
	})
}
