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
	// App context contains immutable presentation defaults shared by requests.
	appContext *AppContext
	// nomadClient is an immutable template. Request handlers use a scoped copy.
	nomadClient *api.Client
}

// AppContext holds shared presentation state that is not request-specific.
type AppContext struct {
	User *components.UserInfo
}

// Config contains configuration options for the handler.
type Config struct {
	// StaticPrefix is the URL prefix for static assets
	StaticPrefix string
	// DevMode enables development features like live reload
	DevMode bool
	// Additional config fields will be added as needed
}

// NewHandler creates a new Handler with the provided dependencies and the
// default Wander API configuration from environment variables.
// Note: API calls are deferred until first request to avoid blocking during server startup.
func NewHandler(logger hclog.Logger) *Handler {
	return NewHandlerWithAPIConfig(logger, api.DefaultConfig())
}

// NewHandlerWithAPIConfig creates a handler whose internal API calls use the
// supplied Wander endpoint. Embedded servers use this to target their own
// listener instead of assuming the default port.
func NewHandlerWithAPIConfig(logger hclog.Logger, wanderConfig *api.Config) *Handler {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	if wanderConfig == nil {
		wanderConfig = api.DefaultConfig()
	}
	nomadClient, err := api.NewClient(wanderConfig)
	if err != nil {
		logger.Error("failed to create Nomad API client", "error", err)
		// Continue without a client so handlers can report unavailability.
		nomadClient = nil
	} else {
		logger.Info("Wander API client initialized", "address", wanderConfig.Address)
	}

	h := &Handler{
		logger:      logger,
		nomadClient: nomadClient,
		appContext: &AppContext{
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

func (h *Handler) availableNamespaces(r *http.Request, client *api.Client) []components.NamespaceInfo {
	fallback := []components.NamespaceInfo{{Name: defaultNamespace, Description: "Default namespace"}}
	if client == nil {
		return fallback
	}

	namespaces, _, err := client.Namespaces().List(&api.QueryOptions{AuthToken: GetTokenFromRequest(r)})
	if err != nil {
		// Don't warn about cluster unavailability - it's expected during startup/elections
		if !IsClusterUnavailableError(err) {
			h.logger.Warn("failed to fetch namespaces from Nomad", "error", err)
		}
		return fallback
	}

	available := make([]components.NamespaceInfo, 0, len(namespaces))
	for _, ns := range namespaces {
		available = append(available, components.NamespaceInfo{
			Name:        ns.Name,
			Description: ns.Description,
		})
	}

	h.logger.Debug("fetched namespaces from Nomad", "count", len(namespaces))
	return available
}

func (h *Handler) availableRegions(client *api.Client) []components.RegionInfo {
	fallback := []components.RegionInfo{{Name: defaultRegion}}
	if client == nil {
		return fallback
	}

	regions, err := client.Regions().List()
	if err != nil {
		// Don't warn about cluster unavailability - it's expected during startup/elections
		if !IsClusterUnavailableError(err) {
			h.logger.Warn("failed to fetch regions from Nomad", "error", err)
		}
		return fallback
	}

	available := make([]components.RegionInfo, 0, len(regions))
	for _, region := range regions {
		available = append(available, components.RegionInfo{
			Name: region,
		})
	}

	h.logger.Debug("fetched regions from Nomad", "count", len(regions))
	return available
}

// Logger returns the handler's logger instance.
// This allows handlers to access the logger for request logging.
func (h *Handler) Logger() hclog.Logger {
	return h.logger
}

// NomadClientForRequest returns an isolated client configuration for the
// request while retaining the shared client's connection pool.
func (h *Handler) NomadClientForRequest(r *http.Request) *api.Client {
	if h.nomadClient == nil {
		return nil
	}
	client := h.nomadClient.WithNamespaceAndRegion(
		GetNamespaceFromRequest(r),
		GetRegionFromRequest(r),
	)
	client.SetSecretID(GetTokenFromRequest(r))
	return client
}

// BaseLayoutProps returns PageLayoutProps with common request-scoped fields.
func (h *Handler) BaseLayoutProps(r *http.Request, title, description, currentPath string, breadcrumbs []components.Breadcrumb) layouts.PageLayoutProps {
	client := h.NomadClientForRequest(r)
	return layouts.PageLayoutProps{
		Title:               title,
		Description:         description,
		CurrentPath:         currentPath,
		Breadcrumbs:         breadcrumbs,
		User:                h.appContext.User,
		CurrentNamespace:    GetNamespaceFromRequest(r),
		AvailableNamespaces: h.availableNamespaces(r, client),
		CurrentRegion:       GetRegionFromRequest(r),
		AvailableRegions:    h.availableRegions(client),
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
			Message: "The Wander cluster is currently unavailable. No cluster leader has been elected.",
			Details: "This usually happens when the cluster is starting up, during a leader election, or when there are network issues between server nodes.",
			Suggestions: []string{
				"Wait a few moments for leader election to complete",
				"Check that all Wander server nodes are running and can communicate",
				"Verify network connectivity between server nodes",
				"Check server logs for leadership election issues",
				"Contact your cluster administrator if the issue persists",
			},
		})
		return true
	}

	return false
}
