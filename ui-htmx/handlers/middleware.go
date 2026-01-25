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
