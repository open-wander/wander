package handlers

import (
	"net/http"

	"github.com/a-h/templ"
)

// HTMX response headers
const (
	// HXPushURL pushes a new URL into the browser location bar
	HXPushURL = "HX-Push-Url"

	// HXRedirect triggers a client-side redirect
	HXRedirect = "HX-Redirect"

	// HXRefresh triggers a full page refresh
	HXRefresh = "HX-Refresh"

	// HXTriggerResponse triggers client-side events
	HXTriggerResponse = "HX-Trigger"
)

// Render renders either a full page or partial content based on whether
// the request is an HTMX AJAX request. For HTMX requests, it renders the
// partial content. For full page requests, it renders the complete page.
//
// If the request wants history push (HTMX request but not a history restore),
// it automatically sets the HX-Push-Url header to the current request URL.
//
// Example usage:
//
//	func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
//	    data := fetchData()
//	    Render(w, r,
//	        pages.IndexPage(data),        // Full page
//	        partials.IndexContent(data),  // Partial for HTMX
//	    )
//	}
func Render(w http.ResponseWriter, r *http.Request, fullPage, partial templ.Component) error {
	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check if this is an HTMX request
	isHTMX := IsHTMXFromRequest(r)

	// If HTMX request and wants history push, set the push URL header
	if isHTMX && WantsHistoryPush(r) {
		SetHXPushURL(w, r.URL.String())
	}

	// Render the appropriate component
	if isHTMX {
		return partial.Render(r.Context(), w)
	}

	return fullPage.Render(r.Context(), w)
}

// RenderPartial always renders the partial content, regardless of request type.
// This is useful when you always want to return a fragment (e.g., for dynamic content updates).
func RenderPartial(w http.ResponseWriter, r *http.Request, partial templ.Component) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return partial.Render(r.Context(), w)
}

// RenderFullPage always renders the full page, regardless of request type.
// This is useful for error pages or when you want to force a full page render.
func RenderFullPage(w http.ResponseWriter, r *http.Request, fullPage templ.Component) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return fullPage.Render(r.Context(), w)
}

// SetHXPushURL sets the HX-Push-Url header to push a new URL into the browser
// location bar. This creates a new history entry.
//
// Pass "false" as the url to prevent pushing a URL into the history.
func SetHXPushURL(w http.ResponseWriter, url string) {
	w.Header().Set(HXPushURL, url)
}

// SetHXRedirect sets the HX-Redirect header to trigger a client-side redirect
// to the specified URL. This is a full page redirect.
func SetHXRedirect(w http.ResponseWriter, url string) {
	w.Header().Set(HXRedirect, url)
}

// SetHXRefresh sets the HX-Refresh header to trigger a full page refresh on
// the client side. Pass true to refresh, false to not refresh.
func SetHXRefresh(w http.ResponseWriter, refresh bool) {
	if refresh {
		w.Header().Set(HXRefresh, "true")
	} else {
		w.Header().Set(HXRefresh, "false")
	}
}

// SetHXTrigger sets the HX-Trigger header to trigger a client-side event.
// Multiple events can be triggered by calling this multiple times, or by
// passing a JSON object as the event string.
//
// Simple event: SetHXTrigger(w, "myEvent")
// Event with data: SetHXTrigger(w, `{"myEvent": {"key": "value"}}`)
func SetHXTrigger(w http.ResponseWriter, event string) {
	w.Header().Set(HXTriggerResponse, event)
}

// SetHXTriggerAfterSettle sets the HX-Trigger-After-Settle header to trigger
// a client-side event after the settle step.
func SetHXTriggerAfterSettle(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger-After-Settle", event)
}

// SetHXTriggerAfterSwap sets the HX-Trigger-After-Swap header to trigger
// a client-side event after the swap step.
func SetHXTriggerAfterSwap(w http.ResponseWriter, event string) {
	w.Header().Set("HX-Trigger-After-Swap", event)
}

// SetHXRetarget sets the HX-Retarget header to change the target element
// for the response content.
func SetHXRetarget(w http.ResponseWriter, target string) {
	w.Header().Set("HX-Retarget", target)
}

// SetHXReswap sets the HX-Reswap header to change the swap strategy
// for the response content.
func SetHXReswap(w http.ResponseWriter, strategy string) {
	w.Header().Set("HX-Reswap", strategy)
}
