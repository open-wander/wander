package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

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

	// Get current token info from actual validation
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
	ctx := r.Context()

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
			Path:   "/ui/",
			MaxAge: -1,
		})

		// Clear the token from cache if present
		cache := getTokenCache()
		if cookie, err := r.Cookie("nomad_token"); err == nil && cookie.Value != "" {
			cache.Delete(cookie.Value)
		}

		// Return updated token form (signed out state)
		tokenInfo := h.getTokenInfo(r) // This will detect ACL disabled if needed
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

	// Validate the token via Nomad API
	tokenInfo := h.validateToken(ctx, secretID)

	// Handle ACL disabled case
	if tokenInfo.ACLDisabled {
		err := RenderPartial(w, r, settingspartials.TokenError("ACL is disabled on this cluster"))
		if err != nil {
			h.handler.logger.Error("failed to render error", "error", err)
		}
		return
	}

	// Handle invalid token
	if !tokenInfo.IsAuthenticated {
		err := RenderPartial(w, r, settingspartials.TokenError("Token failed to authenticate. Please check your Secret ID and try again."))
		if err != nil {
			h.handler.logger.Error("failed to render error", "error", err)
		}
		return
	}

	// Set the token cookie
	// Path is /ui/ to match the UI routes
	http.SetCookie(w, &http.Cookie{
		Name:     "nomad_token",
		Value:    secretID,
		Path:     "/ui/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7, // 7 days
	})

	h.handler.logger.Info("token authenticated successfully",
		"name", tokenInfo.Name,
		"type", tokenInfo.Type,
		"has_expiration", tokenInfo.ExpirationTime != "")

	// Return success with updated token info
	err := RenderPartial(w, r, settingspartials.TokenSuccess(tokenInfo))
	if err != nil {
		h.handler.logger.Error("failed to render success", "error", err)
	}
}

// getTokenInfo retrieves token information from the request.
// This checks for a token in cookies and validates it via the Nomad API.
func (h *SettingsHandler) getTokenInfo(r *http.Request) settingspartials.TokenInfo {
	ctx := r.Context()

	// Try to get token from cookie
	cookie, err := r.Cookie("nomad_token")
	if err != nil || cookie.Value == "" {
		// No token provided - check if ACL is disabled
		// We can only know this if we try to validate with no token
		aclInfo := h.handler.ValidateToken(ctx, "")
		if !aclInfo.ACLEnabled {
			return settingspartials.TokenInfo{
				IsAuthenticated: false,
				ACLDisabled:     true,
			}
		}
		return settingspartials.TokenInfo{
			IsAuthenticated: false,
			ACLDisabled:     false,
		}
	}

	// Validate the token with Nomad API
	return h.validateToken(ctx, cookie.Value)
}

// validateToken validates a token secret and returns token information.
// This uses the actual Nomad API to validate tokens.
func (h *SettingsHandler) validateToken(ctx context.Context, secretID string) settingspartials.TokenInfo {
	// Validate the token with Nomad API
	aclInfo := h.handler.ValidateToken(ctx, secretID)

	// Handle ACL disabled case
	if !aclInfo.ACLEnabled {
		return settingspartials.TokenInfo{
			IsAuthenticated: true,
			ACLDisabled:     true,
		}
	}

	// Handle invalid token
	if !aclInfo.IsValid {
		return settingspartials.TokenInfo{
			IsAuthenticated: false,
			ACLDisabled:     false,
		}
	}

	// Handle valid token - extract token information
	token := aclInfo.Token
	if token == nil {
		// This shouldn't happen if IsValid is true, but handle defensively
		return settingspartials.TokenInfo{
			IsAuthenticated: false,
			ACLDisabled:     false,
		}
	}

	// Build policies list
	policies := make([]settingspartials.TokenPolicy, 0, len(token.Policies))
	for _, policyName := range token.Policies {
		policies = append(policies, settingspartials.TokenPolicy{
			Name:        policyName,
			Description: "", // Policy descriptions require separate API call
		})
	}

	// Format expiration time if present
	expirationTime := ""
	expiresSoon := false
	if token.ExpirationTime != nil {
		expirationTime = token.ExpirationTime.Format("2006-01-02 15:04:05 MST")
		// Check if token expires within 10 minutes (matching Ember UI)
		expiresSoon = IsTokenExpiringSoon(aclInfo, 10*time.Minute)
	}

	// Truncate accessor ID for display (show first 8 chars)
	accessorID := token.AccessorID
	if len(accessorID) > 8 {
		accessorID = accessorID[:8] + "..."
	}

	return settingspartials.TokenInfo{
		IsAuthenticated: true,
		ACLDisabled:     false,
		Name:            token.Name,
		AccessorID:      accessorID,
		SecretID:        secretID,
		Type:            token.Type,
		ExpirationTime:  expirationTime,
		ExpiresSoon:     expiresSoon,
		Policies:        policies,
	}
}
