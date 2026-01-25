package handlers

import (
	"net/http"
)

// ExampleHandler demonstrates the pattern for handling both HTMX and full page requests.
//
// This example shows:
// - Using the Render helper for automatic partial vs full page selection
// - Setting HTMX response headers
// - Handling different HTTP methods
//
// Note: This is a documentation example. The actual template components would need
// to be created in the templates directory.
type ExampleHandler struct {
	// Add any dependencies like database, logger, etc.
}

// NewExampleHandler creates a new example handler instance.
func NewExampleHandler() *ExampleHandler {
	return &ExampleHandler{}
}

// Index handles GET requests to the index page.
// It demonstrates the basic Render pattern.
//
// Example route setup:
//
//	router.HandleFunc("/example", handler.Index).Methods("GET")
//
// For HTMX requests (HX-Request: true), this will return just the partial content.
// For full page requests, this will return the complete page with layout.
func (h *ExampleHandler) Index(w http.ResponseWriter, r *http.Request) {
	// Fetch your data
	data := struct {
		Title string
		Items []string
	}{
		Title: "Example Page",
		Items: []string{"Item 1", "Item 2", "Item 3"},
	}

	// Render using the helper - it automatically chooses full page or partial
	// based on the HX-Request header
	//
	// In a real implementation, you would use actual templ components like:
	// err := Render(w, r,
	//     pages.ExamplePage(data),       // Full page with layout
	//     partials.ExampleContent(data), // Just the content portion
	// )
	//
	// For this example, we'll just write a simple response
	if IsHTMXFromRequest(r) {
		// HTMX request - return partial
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if WantsHistoryPush(r) {
			SetHXPushURL(w, r.URL.String())
		}
		w.Write([]byte("<div><h2>" + data.Title + "</h2><p>Partial content</p></div>"))
	} else {
		// Full page request
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!DOCTYPE html><html><head><title>" + data.Title + "</title></head><body><div><h2>" + data.Title + "</h2><p>Full page content</p></div></body></html>"))
	}
}

// Update handles POST requests to update data.
// It demonstrates setting HTMX response headers and returning updated content.
func (h *ExampleHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Process the update
	// ... do your business logic ...

	// For HTMX requests, we can use various response headers
	if IsHTMXFromRequest(r) {
		// Trigger a client-side event
		SetHXTrigger(w, "itemUpdated")

		// Return the updated content
		// In a real app, you'd render a templ component:
		// err := RenderPartial(w, r, partials.UpdatedContent(data))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<div>Updated content</div>"))
		return
	}

	// For non-HTMX requests, redirect to the index page
	http.Redirect(w, r, "/example", http.StatusSeeOther)
}

// Delete handles DELETE requests.
// It demonstrates using HX-Redirect for post-delete navigation.
func (h *ExampleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL or form
	// id := r.URL.Query().Get("id")
	// ... perform deletion ...

	// For HTMX requests, we can either:
	// 1. Return empty content (item will be removed from DOM)
	// 2. Redirect to another page
	if IsHTMXFromRequest(r) {
		// Option 1: Return empty content with success trigger
		SetHXTrigger(w, "itemDeleted")
		w.WriteHeader(http.StatusOK)
		return

		// Option 2: Redirect to index (commented out)
		// SetHXRedirect(w, "/example")
		// w.WriteHeader(http.StatusOK)
		// return
	}

	// For non-HTMX requests, redirect to index
	http.Redirect(w, r, "/example", http.StatusSeeOther)
}

// Refresh demonstrates triggering a full page refresh.
// This is useful after operations that affect the entire page.
func (h *ExampleHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Perform some operation that requires a full refresh
	// ... do work ...

	if IsHTMXFromRequest(r) {
		// Tell HTMX to refresh the entire page
		SetHXRefresh(w, true)
		w.WriteHeader(http.StatusOK)
		return
	}

	// For non-HTMX requests, redirect to trigger refresh
	http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
}

// SearchResults demonstrates rendering only partial content for
// search-as-you-type functionality.
func (h *ExampleHandler) SearchResults(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	// Fetch search results
	results := []string{
		"Result 1 for " + query,
		"Result 2 for " + query,
	}

	// Always render partial for this endpoint
	// In a real app: err := RenderPartial(w, r, partials.SearchResults(results))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<ul>"))
	for _, result := range results {
		w.Write([]byte("<li>" + result + "</li>"))
	}
	w.Write([]byte("</ul>"))
}

// Common patterns demonstrated:
//
// 1. Basic dual rendering (full page vs partial):
//    Render(w, r, fullPage, partial)
//
// 2. Always partial (for dynamic updates):
//    RenderPartial(w, r, partial)
//
// 3. Always full page (for errors):
//    RenderFullPage(w, r, fullPage)
//
// 4. Trigger client-side events:
//    SetHXTrigger(w, "eventName")
//
// 5. Redirect after mutation:
//    SetHXRedirect(w, "/new-url")
//
// 6. Refresh entire page:
//    SetHXRefresh(w, true)
//
// 7. Prevent history push:
//    SetHXPushURL(w, "false")
//
// 8. Custom target/swap:
//    SetHXRetarget(w, "#new-target")
//    SetHXReswap(w, "outerHTML")
