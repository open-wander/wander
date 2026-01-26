package handlers

import (
	"context"
	"net/http"
)

// HTMX request headers
const (
	// HXRequest indicates this is an HTMX AJAX request
	HXRequest = "HX-Request"

	// HXCurrentURL is the current URL of the browser
	HXCurrentURL = "HX-Current-URL"

	// HXHistoryRestoreRequest indicates this is a history restore request
	HXHistoryRestoreRequest = "HX-History-Restore-Request"

	// HXPrompt is the user response to an hx-prompt
	HXPrompt = "HX-Prompt"

	// HXTarget is the id of the target element if it exists
	HXTarget = "HX-Target"

	// HXTriggerName is the name of the triggered element if it exists
	HXTriggerName = "HX-Trigger-Name"

	// HXTrigger is the id of the triggered element if it exists
	HXTrigger = "HX-Trigger"
)

// Nomad ACL headers and cookies
const (
	// XNomadToken is the HTTP header for Nomad ACL tokens
	XNomadToken = "X-Nomad-Token"

	// NomadTokenCookie is the cookie name for storing Nomad ACL tokens
	NomadTokenCookie = "nomad_token"
)

// IsHTMXRequest checks if the request is an HTMX AJAX request by checking
// for the HX-Request header. Returns true if the header is set to "true".
func IsHTMXRequest(r *http.Request) bool {
	return r.Header.Get(HXRequest) == "true"
}

// WantsHistoryPush checks if the request wants history push functionality.
// This is determined by checking if it's an HTMX request but NOT a history restore request.
func WantsHistoryPush(r *http.Request) bool {
	if !IsHTMXRequest(r) {
		return false
	}
	return r.Header.Get(HXHistoryRestoreRequest) != "true"
}

// GetCurrentURL retrieves the current URL from the HX-Current-URL header.
// Returns empty string if the header is not present.
func GetCurrentURL(r *http.Request) string {
	return r.Header.Get(HXCurrentURL)
}

// GetTarget retrieves the target element ID from the HX-Target header.
// Returns empty string if the header is not present.
func GetTarget(r *http.Request) string {
	return r.Header.Get(HXTarget)
}

// GetTriggerName retrieves the name of the triggering element from the HX-Trigger-Name header.
// Returns empty string if the header is not present.
func GetTriggerName(r *http.Request) string {
	return r.Header.Get(HXTriggerName)
}

// GetTrigger retrieves the ID of the triggering element from the HX-Trigger header.
// Returns empty string if the header is not present.
func GetTrigger(r *http.Request) string {
	return r.Header.Get(HXTrigger)
}

// GetPromptResponse retrieves the user's response to an hx-prompt from the HX-Prompt header.
// Returns empty string if the header is not present.
func GetPromptResponse(r *http.Request) string {
	return r.Header.Get(HXPrompt)
}

// HTMXMiddleware is a middleware that adds HTMX detection to the request context.
// It checks for the HX-Request header and stores the result in the context,
// making it available to handlers via the context helpers.
func HTMXMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an HTMX request
		isHTMX := IsHTMXRequest(r)

		// Add HTMX status to context
		ctx := context.WithValue(r.Context(), htmxContextKey, isHTMX)

		// Add current URL to context if available
		if currentURL := GetCurrentURL(r); currentURL != "" {
			ctx = context.WithValue(ctx, htmxCurrentURLKey, currentURL)
		}

		// Add target to context if available
		if target := GetTarget(r); target != "" {
			ctx = context.WithValue(ctx, htmxTargetKey, target)
		}

		// Add trigger info to context if available
		if trigger := GetTrigger(r); trigger != "" {
			ctx = context.WithValue(ctx, htmxTriggerKey, trigger)
		}
		if triggerName := GetTriggerName(r); triggerName != "" {
			ctx = context.WithValue(ctx, htmxTriggerNameKey, triggerName)
		}

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken extracts the Nomad ACL token from the request.
// It first checks the X-Nomad-Token header, then falls back to checking
// the nomad_token cookie. Returns empty string if no token is found.
func extractToken(r *http.Request) string {
	// Check X-Nomad-Token header first (preferred method for API calls)
	if token := r.Header.Get(XNomadToken); token != "" {
		return token
	}

	// Fall back to cookie (used for browser-based authentication)
	cookie, err := r.Cookie(NomadTokenCookie)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// ACLMiddleware extracts the Nomad ACL token from the request and stores it
// in the request context for downstream handlers to use.
// The token is extracted from either:
// 1. The X-Nomad-Token HTTP header (preferred for API calls)
// 2. The nomad_token cookie (used for browser-based authentication)
//
// Note: This middleware does NOT validate the token - it only extracts and stores it.
// Token validation should be performed by downstream handlers or a separate middleware.
func ACLMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract token from headers or cookies
		if token := extractToken(r); token != "" {
			// Store token in context for downstream handlers
			ctx = context.WithValue(ctx, nomadTokenKey, token)
		}

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
