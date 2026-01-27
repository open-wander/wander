package handlers

import (
	"net/http"
	"strings"
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

	// Topology routes
	r.registerTopologyRoutes()

	// Variables routes
	r.registerVariablesRoutes()

	// Namespaces routes
	r.registerNamespacesRoutes()

	// Policies routes
	r.registerPoliciesRoutes()

	// CSI routes
	r.registerCSIRoutes()

	// Settings routes
	r.registerSettingsRoutes()

	// Evaluations routes
	r.registerEvaluationsRoutes()

	// Deployments routes
	r.registerDeploymentsRoutes()

	// Search routes
	r.registerSearchRoutes()

	// API routes
	r.registerAPIRoutes()
}

// registerJobsRoutes sets up routes for the jobs section.
func (r *Router) registerJobsRoutes() {
	jobsHandler := NewJobsHandler(r.handler)

	// Jobs handler handles both list and detail pages
	r.mux.Handle("/ui/jobs", r.wrapMiddleware(jobsHandler))
	r.mux.Handle("/ui/jobs/", r.wrapMiddleware(jobsHandler))
}

// registerAllocationsRoutes sets up routes for the allocations section.
func (r *Router) registerAllocationsRoutes() {
	allocationsHandler := NewAllocationsHandler(r.handler)
	logsHandler := NewAllocationLogsHandler(r.handler)
	execHandler := NewExecHandler(r.handler)

	// Allocations handler handles both list and detail pages
	r.mux.Handle("/ui/allocations", r.wrapMiddleware(allocationsHandler))

	// Log streaming and exec WebSocket routes must be registered before the general allocations route
	// to ensure they take precedence for their respective paths
	r.mux.HandleFunc("/ui/allocations/", func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		// Route exec WebSocket requests to the exec handler
		// WebSocket connections should NOT have HTMX middleware applied
		if strings.Contains(path, "/exec/ws") {
			execHandler.ServeHTTP(w, req)
			return
		}
		// Route log-related requests to the logs handler
		if strings.Contains(path, "/logs") {
			r.wrapMiddleware(logsHandler).ServeHTTP(w, req)
			return
		}
		// All other allocation requests go to the main handler
		r.wrapMiddleware(allocationsHandler).ServeHTTP(w, req)
	})
}

// registerClientsRoutes sets up routes for the clients section.
func (r *Router) registerClientsRoutes() {
	clientsHandler := NewClientsHandler(r.handler)

	// Clients handler handles both list and detail pages
	r.mux.Handle("/ui/clients", r.wrapMiddleware(clientsHandler))
	r.mux.Handle("/ui/clients/", r.wrapMiddleware(clientsHandler))
}

// registerServersRoutes sets up routes for the servers section.
func (r *Router) registerServersRoutes() {
	serversHandler := NewServersHandler(r.handler)

	// Servers handler handles both list and detail pages
	r.mux.Handle("/ui/servers", r.wrapMiddleware(serversHandler))
	r.mux.Handle("/ui/servers/", r.wrapMiddleware(serversHandler))
}

// registerTopologyRoutes sets up routes for the topology section.
func (r *Router) registerTopologyRoutes() {
	topologyHandler := NewTopologyHandler(r.handler)

	// Topology handler handles the topology visualization page
	r.mux.Handle("/ui/topology", r.wrapMiddleware(topologyHandler))
}

// registerVariablesRoutes sets up routes for the variables section.
func (r *Router) registerVariablesRoutes() {
	variablesHandler := NewVariablesHandler(r.handler)

	// Variables handler handles both list and detail pages
	r.mux.Handle("/ui/variables", r.wrapMiddleware(variablesHandler))
	r.mux.Handle("/ui/variables/", r.wrapMiddleware(variablesHandler))
}

// registerNamespacesRoutes sets up routes for the namespaces section.
func (r *Router) registerNamespacesRoutes() {
	namespacesHandler := NewNamespacesHandler(r.handler)

	// Namespaces handler handles the namespaces list page
	r.mux.Handle("/ui/namespaces", r.wrapMiddleware(namespacesHandler))
}

// registerPoliciesRoutes sets up routes for the ACL policies section.
func (r *Router) registerPoliciesRoutes() {
	policiesHandler := NewPoliciesHandler(r.handler)

	// Policies handler handles both list and detail pages
	r.mux.Handle("/ui/policies", r.wrapMiddleware(policiesHandler))
	r.mux.Handle("/ui/policies/", r.wrapMiddleware(policiesHandler))
}

// registerCSIRoutes sets up routes for the CSI section.
func (r *Router) registerCSIRoutes() {
	csiHandler := NewCSIHandler(r.handler)

	// CSI handler handles plugins list, volumes list and detail pages
	r.mux.Handle("/ui/csi/plugins", r.wrapMiddleware(csiHandler))
	r.mux.Handle("/ui/csi/volumes", r.wrapMiddleware(csiHandler))
	r.mux.Handle("/ui/csi/volumes/", r.wrapMiddleware(csiHandler))
}

// registerSettingsRoutes sets up routes for the settings section.
func (r *Router) registerSettingsRoutes() {
	settingsHandler := NewSettingsHandler(r.handler)

	// Settings handler handles settings page and token management
	r.mux.Handle("/ui/settings", r.wrapMiddleware(settingsHandler))
	r.mux.Handle("/ui/settings/", r.wrapMiddleware(settingsHandler))
}

// registerEvaluationsRoutes sets up routes for the evaluations panel.
func (r *Router) registerEvaluationsRoutes() {
	evaluationsHandler := NewEvaluationsHandler(r.handler)

	// Evaluations handler handles both list and detail pages
	r.mux.Handle("/ui/evaluations", r.wrapMiddleware(evaluationsHandler))
	r.mux.Handle("/ui/evaluations/", r.wrapMiddleware(evaluationsHandler))
}

// registerDeploymentsRoutes sets up routes for the deployments panel.
func (r *Router) registerDeploymentsRoutes() {
	deploymentsHandler := NewDeploymentsHandler(r.handler)

	// Deployments handler handles both list and detail pages
	r.mux.Handle("/ui/deployments", r.wrapMiddleware(deploymentsHandler))
	r.mux.Handle("/ui/deployments/", r.wrapMiddleware(deploymentsHandler))
}

// registerSearchRoutes sets up routes for global search functionality.
func (r *Router) registerSearchRoutes() {
	searchHandler := NewSearchHandler(r.handler)

	// Search endpoint for HTMX autocomplete
	r.mux.Handle("/ui/search", r.wrapMiddleware(searchHandler))
}

// registerAPIRoutes sets up API routes for HTMX actions.
func (r *Router) registerAPIRoutes() {
	jobsAPIHandler := NewJobsAPIHandler(r.handler)
	namespaceAPIHandler := NewNamespaceAPIHandler(r.handler)
	regionAPIHandler := NewRegionAPIHandler(r.handler)
	clientsAPIHandler := NewClientsAPIHandler(r.handler)
	deploymentsAPIHandler := NewDeploymentsAPIHandler(r.handler)

	// Job action endpoints
	r.mux.Handle("/api/jobs/", r.wrapMiddleware(jobsAPIHandler))

	// Namespace API endpoints
	r.mux.Handle("/api/namespace/", r.wrapMiddleware(namespaceAPIHandler))

	// Region API endpoints
	r.mux.Handle("/api/region/", r.wrapMiddleware(regionAPIHandler))

	// Client node action endpoints
	r.mux.Handle("/api/clients/", r.wrapMiddleware(clientsAPIHandler))

	// Deployment action endpoints
	r.mux.Handle("/api/deployments/", r.wrapMiddleware(deploymentsAPIHandler))

	// Auth endpoints for token management
	r.mux.HandleFunc("/api/v1/auth/token-info", func(w http.ResponseWriter, req *http.Request) {
		// Apply middleware manually for this route
		middleware := r.handler.TokenValidationMiddleware(http.HandlerFunc(r.handler.GetTokenInfo))
		middleware.ServeHTTP(w, req)
	})

	r.mux.HandleFunc("/api/v1/auth/clear-cache", r.handler.ClearTokenCache)
}

// wrapMiddleware applies HTMX middleware and any other necessary middleware
// to the provided handler.
//
// Current middleware chain:
// 1. ACLMiddleware - extracts and stores token from headers/cookies
// 2. HTMXMiddleware - adds HTMX request context
//
// Optional middleware available but not enabled by default:
// - TokenValidationMiddleware - validates tokens via Nomad API (adds latency)
// - RequireToken - enforces authentication (blocks unauthenticated requests)
//
// To enable token validation, modify this function to:
//
//	return ACLMiddleware(r.handler.TokenValidationMiddleware(HTMXMiddleware(h)))
//
// To enforce authentication on specific routes, wrap them with:
//
//	r.handler.RequireToken(handler)
func (r *Router) wrapMiddleware(h http.Handler) http.Handler {
	// Apply middleware in order:
	// 1. ACL middleware extracts and stores token from headers/cookies
	// 2. HTMX middleware adds HTMX request context
	return ACLMiddleware(HTMXMiddleware(h))
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
