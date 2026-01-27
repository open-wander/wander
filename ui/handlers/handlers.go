package handlers

import (
	"net/http"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/layouts"
	"github.com/open-wander/wander/version"
)

// Handler holds shared dependencies for all HTTP handlers.
// This follows the standard Go pattern of centralizing dependencies
// for easier testing and dependency injection.
type Handler struct {
	logger hclog.Logger
	// App context for shared state
	appContext *AppContext
	// Nomad API client for making API calls
	nomadClient *api.Client
	// Track if we've done the initial namespace fetch
	namespacesRefreshed bool
	// Track if we've done the initial regions fetch
	regionsRefreshed bool
}

// AppContext holds shared application state like current namespace, region and user info
type AppContext struct {
	CurrentNamespace    string
	AvailableNamespaces []components.NamespaceInfo
	CurrentRegion       string
	AvailableRegions    []components.RegionInfo
	User                *components.UserInfo
}

// Config contains configuration options for the handler.
type Config struct {
	// StaticPrefix is the URL prefix for static assets
	StaticPrefix string
	// DevMode enables development features like live reload
	DevMode bool
	// Additional config fields will be added as needed
}

// NewHandler creates a new Handler with the provided dependencies.
// This constructor ensures all handlers have consistent initialization.
// It creates a Nomad API client using default configuration from environment variables.
// Note: API calls are deferred until first request to avoid blocking during server startup.
func NewHandler(logger hclog.Logger) *Handler {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	// Create Nomad API client with default config (reads from WANDER_* or NOMAD_* env vars)
	nomadConfig := api.DefaultConfig()
	nomadClient, err := api.NewClient(nomadConfig)
	if err != nil {
		logger.Error("failed to create Nomad API client", "error", err)
		// Continue without client - handlers will use mock data
		nomadClient = nil
	} else {
		logger.Info("Nomad API client initialized", "address", nomadConfig.Address)
	}

	h := &Handler{
		logger:      logger,
		nomadClient: nomadClient,
		appContext: &AppContext{
			CurrentNamespace: "default",
			AvailableNamespaces: []components.NamespaceInfo{
				{Name: "default", Description: "Default namespace"},
			},
			CurrentRegion: "global",
			AvailableRegions: []components.RegionInfo{
				{Name: "global"},
			},
			User: &components.UserInfo{
				Name:  "Anonymous",
				Email: "",
			},
		},
	}

	// NOTE: Do NOT call refreshNamespaces() or refreshRegions() here - it would block during
	// server startup because the HTTP server isn't ready yet. These refresh lazily on first request.

	return h
}

// refreshNamespaces fetches the list of namespaces from Nomad and updates the app context.
func (h *Handler) refreshNamespaces() {
	if h.nomadClient == nil {
		return
	}

	namespaces, _, err := h.nomadClient.Namespaces().List(nil)
	if err != nil {
		// Don't warn about cluster unavailability - it's expected during startup/elections
		if !IsClusterUnavailableError(err) {
			h.logger.Warn("failed to fetch namespaces from Nomad", "error", err)
		}
		return
	}

	h.appContext.AvailableNamespaces = make([]components.NamespaceInfo, 0, len(namespaces))
	for _, ns := range namespaces {
		h.appContext.AvailableNamespaces = append(h.appContext.AvailableNamespaces, components.NamespaceInfo{
			Name:        ns.Name,
			Description: ns.Description,
		})
	}

	h.logger.Debug("fetched namespaces from Nomad", "count", len(namespaces))
}

// refreshRegions fetches the list of regions from Nomad and updates the app context.
func (h *Handler) refreshRegions() {
	if h.nomadClient == nil {
		return
	}

	regions, err := h.nomadClient.Regions().List()
	if err != nil {
		// Don't warn about cluster unavailability - it's expected during startup/elections
		if !IsClusterUnavailableError(err) {
			h.logger.Warn("failed to fetch regions from Nomad", "error", err)
		}
		return
	}

	h.appContext.AvailableRegions = make([]components.RegionInfo, 0, len(regions))
	for _, region := range regions {
		h.appContext.AvailableRegions = append(h.appContext.AvailableRegions, components.RegionInfo{
			Name: region,
		})
	}

	h.logger.Debug("fetched regions from Nomad", "count", len(regions))
}

// Logger returns the handler's logger instance.
// This allows handlers to access the logger for request logging.
func (h *Handler) Logger() hclog.Logger {
	return h.logger
}

// NomadClient returns the Nomad API client.
// Returns nil if the client is not configured or failed to initialize.
func (h *Handler) NomadClient() *api.Client {
	return h.nomadClient
}

// HasNomadClient returns true if a Nomad API client is available.
func (h *Handler) HasNomadClient() bool {
	return h.nomadClient != nil
}

// AppContext returns the shared application context.
func (h *Handler) AppContext() *AppContext {
	return h.appContext
}

// SetCurrentNamespace updates the current namespace in the app context.
func (h *Handler) SetCurrentNamespace(ns string) {
	h.appContext.CurrentNamespace = ns
	// Also update the Nomad client's namespace
	if h.nomadClient != nil {
		h.nomadClient.SetNamespace(ns)
	}
}

// SetCurrentRegion updates the current region in the app context.
func (h *Handler) SetCurrentRegion(region string) {
	h.appContext.CurrentRegion = region
	// Also update the Nomad client's region
	if h.nomadClient != nil {
		h.nomadClient.SetRegion(region)
	}
}

// BaseLayoutProps returns PageLayoutProps with common fields pre-filled from app context.
// Handlers should use this and then fill in page-specific fields.
// On first call, it lazily fetches namespaces and regions from Nomad if a client is available.
func (h *Handler) BaseLayoutProps(title, description, currentPath string, breadcrumbs []components.Breadcrumb) layouts.PageLayoutProps {
	// Lazy refresh namespaces on first request
	if !h.namespacesRefreshed && h.nomadClient != nil {
		h.refreshNamespaces()
		h.namespacesRefreshed = true
	}

	// Lazy refresh regions on first request
	if !h.regionsRefreshed && h.nomadClient != nil {
		h.refreshRegions()
		h.regionsRefreshed = true
	}

	return layouts.PageLayoutProps{
		Title:               title,
		Description:         description,
		CurrentPath:         currentPath,
		Breadcrumbs:         breadcrumbs,
		User:                h.appContext.User,
		CurrentNamespace:    h.appContext.CurrentNamespace,
		AvailableNamespaces: h.appContext.AvailableNamespaces,
		CurrentRegion:       h.appContext.CurrentRegion,
		AvailableRegions:    h.appContext.AvailableRegions,
		Version:             "v" + version.GetVersion().VersionNumber(),
	}
}

// CheckClusterError checks if an error indicates cluster unavailability and renders
// an appropriate error page if so. Returns true if error was handled, false otherwise.
// Handlers should check this before handling other types of errors.
//
// Example usage:
//
//	jobs, _, err := client.Jobs().List(opts)
//	if err != nil {
//	    if h.handler.CheckClusterError(w, r, err) {
//	        return // Error page rendered
//	    }
//	    // Handle other errors
//	}
func (h *Handler) CheckClusterError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	if IsClusterUnavailableError(err) {
		h.logger.Warn("cluster unavailable error detected", "error", err)
		h.RenderClusterUnavailable(w, r, ClusterUnavailableError{
			Message: "The Nomad cluster is currently unavailable. No cluster leader has been elected.",
			Details: "This usually happens when the cluster is starting up, during a leader election, or when there are network issues between server nodes.",
			Suggestions: []string{
				"Wait a few moments for leader election to complete",
				"Check that all Nomad server nodes are running and can communicate",
				"Verify network connectivity between server nodes",
				"Check server logs for leadership election issues",
				"Contact your cluster administrator if the issue persists",
			},
		})
		return true
	}

	return false
}
