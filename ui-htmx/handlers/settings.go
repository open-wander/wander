package handlers

import (
	"net/http"
	"strings"

	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/settings"
	"github.com/open-wander/wander/ui-htmx/templates/partials/settingspartials"
)

// SettingsHandler handles requests for the settings pages.
type SettingsHandler struct {
	handler *Handler
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(h *Handler) *SettingsHandler {
	return &SettingsHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the settings pages.
// It renders the settings page using the page layout, supporting both
// full page loads and HTMX partial updates.
func (h *SettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/settings exactly (main settings page)
	if path == "/ui/settings" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Show(w, r)
		return
	}

	// Handle /ui/settings/token (token update)
	if strings.HasSuffix(path, "/token") {
		if r.Method == http.MethodPost {
			h.UpdateToken(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.NotFound(w, r)
}

// Show handles the settings page
func (h *SettingsHandler) Show(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("settings page request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Get current token info from cookie/session if exists
	// For now, we'll use mock data
	tokenInfo := h.getTokenInfo(r)

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Settings - Wander",
		"Configure Wander settings",
		"/ui/settings",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Settings", URL: ""},
		},
	)

	// Render the settings page
	err := Render(w, r,
		settings.Settings(layoutProps, tokenInfo),                // Full page with layout
		settingspartials.SettingsContent(tokenInfo), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render settings page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// UpdateToken handles POST requests to update the ACL token
// Route: POST /ui/settings/token
func (h *SettingsHandler) UpdateToken(w http.ResponseWriter, r *http.Request) {
	// Parse the form data
	if err := r.ParseForm(); err != nil {
		h.handler.logger.Error("failed to parse form", "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		RenderPartial(w, r, settingspartials.TokenError("Invalid form data"))
		return
	}

	// Get the action (signin or signout)
	action := r.FormValue("action")
	secretID := r.FormValue("secret_id")

	h.handler.logger.Info("token action",
		"action", action,
		"hasSecret", secretID != "",
	)

	if action == "signout" {
		// Clear the token cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "nomad_token",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		// Return updated token form (signed out state)
		tokenInfo := settingspartials.TokenInfo{
			IsAuthenticated: false,
		}
		err := RenderPartial(w, r, settingspartials.TokenFormContent(tokenInfo))
		if err != nil {
			h.handler.logger.Error("failed to render token form", "error", err)
		}
		return
	}

	// Validate secret ID is provided
	if secretID == "" {
		h.handler.logger.Warn("empty secret ID provided")
		err := RenderPartial(w, r, settingspartials.TokenError("Secret ID is required"))
		if err != nil {
			h.handler.logger.Error("failed to render error", "error", err)
		}
		return
	}

	// TODO: In a real implementation, this would:
	// 1. Call the Nomad API: client.ACLTokens().Self(&api.QueryOptions{AuthToken: secretID})
	// 2. Validate the token and get its policies
	// 3. Store the token in a secure cookie/session

	// Mock token validation for demonstration
	tokenInfo := h.validateToken(secretID)

	if !tokenInfo.IsAuthenticated {
		err := RenderPartial(w, r, settingspartials.TokenError("Token failed to authenticate"))
		if err != nil {
			h.handler.logger.Error("failed to render error", "error", err)
		}
		return
	}

	// Set the token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "nomad_token",
		Value:    secretID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 7, // 7 days
	})

	// Return success with updated token info
	err := RenderPartial(w, r, settingspartials.TokenSuccess(tokenInfo))
	if err != nil {
		h.handler.logger.Error("failed to render success", "error", err)
	}
}

// getTokenInfo retrieves token information from the request.
// This checks for a token in cookies and validates it.
func (h *SettingsHandler) getTokenInfo(r *http.Request) settingspartials.TokenInfo {
	// Try to get token from cookie
	cookie, err := r.Cookie("nomad_token")
	if err != nil || cookie.Value == "" {
		return settingspartials.TokenInfo{
			IsAuthenticated: false,
		}
	}

	// TODO: In a real implementation, validate the token with Nomad API
	// For now, return mock authenticated state
	return h.validateToken(cookie.Value)
}

// validateToken validates a token secret and returns token information.
// This is a mock implementation that will be replaced with actual Nomad API calls.
func (h *SettingsHandler) validateToken(secretID string) settingspartials.TokenInfo {
	// Mock validation - any token starting with "valid-" is considered valid
	if !strings.HasPrefix(secretID, "valid-") {
		return settingspartials.TokenInfo{
			IsAuthenticated: false,
		}
	}

	// Mock successful authentication
	tokenInfo := settingspartials.TokenInfo{
		IsAuthenticated: true,
		Name:            "Development Token",
		AccessorID:      "12345678-1234-1234-1234-123456789012",
		SecretID:        secretID,
		Type:            "client",
		Policies: []settingspartials.TokenPolicy{
			{
				Name:        "read-only",
				Description: "Read-only access to all resources",
			},
			{
				Name:        "submit-job",
				Description: "Ability to submit and manage jobs",
			},
		},
	}

	// Mock management token
	if strings.Contains(secretID, "management") {
		tokenInfo.Type = "management"
		tokenInfo.Name = "Management Token"
		tokenInfo.Policies = nil
	}

	return tokenInfo
}
