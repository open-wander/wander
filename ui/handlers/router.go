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

	// Services routes
	r.registerServicesRoutes()

	// Topology routes
	r.registerTopologyRoutes()

	// Variables routes
	r.registerVariablesRoutes()

	// Namespaces routes
	r.registerNamespacesRoutes()

	// ACL overview, policy, token, and role routes
	r.registerAccessControlRoutes()

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
		// WebSocket connections skip HTMX middleware, but still need ACL token
		// extraction from the request cookie/header.
		if strings.Contains(path, "/exec/ws") {
			ACLMiddleware(execHandler).ServeHTTP(w, req)
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

// registerServicesRoutes sets up read-only service registration routes.
func (r *Router) registerServicesRoutes() {
	servicesHandler := &ServicesHandler{handler: r.handler}
	r.mux.Handle("/ui/services", r.wrapMiddleware(servicesHandler))
	r.mux.Handle("/ui/services/", r.wrapMiddleware(servicesHandler))
}

// registerTopologyRoutes sets up routes for the topology section.
func (r *Router) registerTopologyRoutes() {
	topologyHandler := NewTopologyHandler(r.handler)

	// Topology handler handles the visualization page and its JSON data feed.
	r.mux.Handle("/ui/topology", r.wrapMiddleware(topologyHandler))
	r.mux.Handle("/ui/topology/data", r.wrapMiddleware(topologyHandler))
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

// registerAccessControlRoutes sets up the Nomad 1.6.5-compatible ACL route
// hierarchy and preserves Wander's earlier top-level paths as aliases.
func (r *Router) registerAccessControlRoutes() {
	accessControlHandler := NewAccessControlHandler(r.handler)
	r.mux.Handle(aclBasePath, r.wrapMiddleware(accessControlHandler))
	r.mux.Handle(aclBasePath+"/", r.wrapMiddleware(accessControlHandler))

	for legacyPath, canonicalPath := range map[string]string{
		legacyPoliciesPath: aclPoliciesPath,
		legacyTokensPath:   aclTokensPath,
		legacyRolesPath:    aclRolesPath,
	} {
		alias := rewritePathPrefix(legacyPath, canonicalPath, accessControlHandler)
		r.mux.Handle(legacyPath, r.wrapMiddleware(alias))
		r.mux.Handle(legacyPath+"/", r.wrapMiddleware(alias))
	}
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
	selectionAPIHandler := NewSelectionAPIHandler(r.handler)
	clientsAPIHandler := NewClientsAPIHandler(r.handler)
	deploymentsAPIHandler := NewDeploymentsAPIHandler(r.handler)

	// Job action endpoints
	r.mux.Handle("/api/jobs/", r.wrapMiddleware(jobsAPIHandler))

	// Namespace API endpoints
	r.mux.Handle("/api/namespace/", r.wrapMiddleware(selectionAPIHandler))

	// Region API endpoints
	r.mux.Handle("/api/region/", r.wrapMiddleware(selectionAPIHandler))

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

	r.mux.Handle("/api/v1/auth/clear-cache", ACLMiddleware(http.HandlerFunc(r.handler.ClearTokenCache)))
}

// wrapMiddleware applies HTMX middleware and any other necessary middleware
// to the provided handler.
//
// Current middleware chain:
// 1. ACLMiddleware - extracts and stores token from headers/cookies
// 2. TokenValidationMiddleware - validates tokens and resolves permissions
// 3. HTMXMiddleware - adds HTMX request context
//
// Optional middleware available but not enabled by default:
// - RequireToken - enforces authentication (blocks unauthenticated requests)
//
// To enforce authentication on specific routes, wrap them with:
//
//	r.handler.RequireToken(handler)
func (r *Router) wrapMiddleware(h http.Handler) http.Handler {
	// Apply middleware in order:
	// 1. ACL middleware extracts and stores token from headers/cookies
	// 2. Token validation resolves ACL mode and permissions
	// 3. HTMX middleware adds HTMX request context
	return ACLMiddleware(r.handler.TokenValidationMiddleware(SelectionMiddleware(HTMXMiddleware(h))))
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
