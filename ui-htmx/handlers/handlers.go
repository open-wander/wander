package handlers

import (
	"github.com/hashicorp/go-hclog"
)

// Handler holds shared dependencies for all HTTP handlers.
// This follows the standard Go pattern of centralizing dependencies
// for easier testing and dependency injection.
type Handler struct {
	logger hclog.Logger
	// Additional dependencies can be added here as needed:
	// - config *Config
	// - nomadClient *api.Client
	// - db *sql.DB
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
func NewHandler(logger hclog.Logger) *Handler {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	return &Handler{
		logger: logger,
	}
}

// Logger returns the handler's logger instance.
// This allows handlers to access the logger for request logging.
func (h *Handler) Logger() hclog.Logger {
	return h.logger
}
