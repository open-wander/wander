package handlers

import (
	"context"
	"net/http"
	"strings"
	"unicode"
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

// ACL transport names
const (
	// XNomadToken is retained because it is part of the inherited HTTP API.
	XNomadToken = "X-Nomad-Token"

	// WanderTokenCookie is Wander UI state and is not part of the inherited API.
	WanderTokenCookie = "wander_token"

	// WanderNamespaceCookie and WanderRegionCookie keep cluster selection scoped
	// to one browser rather than shared by every UI user in the server process.
	WanderNamespaceCookie = "wander_namespace"
	WanderRegionCookie    = "wander_region"
)

// SelectionMiddleware resolves namespace and region independently for every
// request. URL values have priority so links remain explicit and shareable;
// browser cookies provide continuity for links that omit the selection.
func SelectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace := selectionValue(r, "namespace", WanderNamespaceCookie, defaultNamespace, true)
		region := selectionValue(r, "region", WanderRegionCookie, defaultRegion, false)

		ctx := context.WithValue(r.Context(), wanderNamespaceKey, namespace)
		ctx = context.WithValue(ctx, wanderRegionKey, region)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func selectionValue(r *http.Request, queryName, cookieName, fallback string, allowWildcard bool) string {
	if value := r.URL.Query().Get(queryName); validSelectionValue(value, allowWildcard) {
		return value
	}
	if cookie, err := r.Cookie(cookieName); err == nil && validSelectionValue(cookie.Value, allowWildcard) {
		return cookie.Value
	}
	return fallback
}

func validSelectionValue(value string, allowWildcard bool) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	if allowWildcard && value == "*" {
		return true
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.')
	}) == -1
}

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

// extractToken extracts the ACL token from the request.
// It first checks the X-Nomad-Token header, then falls back to checking
// the wander_token cookie. Returns empty string if no token is found.
func extractToken(r *http.Request) string {
	// Check X-Nomad-Token header first (preferred method for API calls)
	if token := r.Header.Get(XNomadToken); token != "" {
		return token
	}

	// Fall back to cookie (used for browser-based authentication)
	cookie, err := r.Cookie(WanderTokenCookie)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// ACLMiddleware extracts the ACL token from the request and stores it
// in the request context for downstream handlers to use.
// The token is extracted from either:
// 1. The X-Nomad-Token HTTP header (preferred for API calls)
// 2. The wander_token cookie (used for browser-based authentication)
//
// Note: This middleware does NOT validate the token - it only extracts and stores it.
// Token validation should be performed by downstream handlers or a separate middleware.
func ACLMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract token from headers or cookies
		if token := extractToken(r); token != "" {
			// Store token in context for downstream handlers
			ctx = context.WithValue(ctx, wanderTokenKey, token)
		}

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
