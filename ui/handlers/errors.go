package handlers

import (
	"net/http"
	"strings"
)

// ForbiddenError represents a 403 Forbidden error with details
type ForbiddenError struct {
	Message     string
	Capability  string
	Namespace   string
	Suggestions []string
}

// RenderForbidden renders a 403 Forbidden error response.
// For HTMX requests, it returns a partial error message.
// For full page requests, it returns a full error page.
func (h *Handler) RenderForbidden(w http.ResponseWriter, r *http.Request, err ForbiddenError) {
	w.WriteHeader(http.StatusForbidden)

	// For HTMX requests, return a simple error message
	if IsHTMXFromRequest(r) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderForbiddenPartial(err)))
		return
	}

	// For full page requests, render a complete error page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderForbiddenPage(err)))
}

// renderForbiddenPartial returns HTML for a partial error display (HTMX requests)
func renderForbiddenPartial(err ForbiddenError) string {
	message := err.Message
	if message == "" {
		message = "You do not have permission to perform this action"
	}

	html := `<div class="rounded-lg bg-red-50 border border-red-200 p-6">
	<div class="flex">
		<div class="flex-shrink-0">
			<svg class="h-6 w-6 text-red-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
			</svg>
		</div>
		<div class="ml-3">
			<h3 class="text-sm font-medium text-red-800">Access Denied</h3>
			<div class="mt-2 text-sm text-red-700">
				<p>` + message + `</p>`

	if err.Capability != "" {
		html += `<p class="mt-2"><strong>Required capability:</strong> <code class="px-1.5 py-0.5 bg-red-100 rounded">` + err.Capability + `</code>`
		if err.Namespace != "" {
			html += ` in namespace <code class="px-1.5 py-0.5 bg-red-100 rounded">` + err.Namespace + `</code>`
		}
		html += `</p>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-3">
					<p class="font-medium">What you can do:</p>
					<ul class="list-disc list-inside mt-1 space-y-1">`
		for _, suggestion := range err.Suggestions {
			html += `<li>` + suggestion + `</li>`
		}
		html += `</ul></div>`
	}

	html += `</div>
		</div>
	</div>
</div>`

	return html
}

// renderForbiddenPage returns HTML for a full error page
func renderForbiddenPage(err ForbiddenError) string {
	message := err.Message
	if message == "" {
		message = "You do not have permission to perform this action"
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>403 Forbidden - Wander</title>
	<script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-50">
	<div class="min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8">
		<div class="max-w-2xl w-full">
			<div class="text-center mb-8">
				<div class="inline-flex items-center justify-center w-16 h-16 bg-red-100 rounded-full mb-4">
					<svg class="h-8 w-8 text-red-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
					</svg>
				</div>
				<h1 class="text-4xl font-bold text-gray-900 mb-2">403 Forbidden</h1>
				<p class="text-xl text-gray-600">Access Denied</p>
			</div>

			<div class="bg-white shadow-lg rounded-lg p-8 mb-6">
				<p class="text-gray-700 text-lg mb-4">` + message + `</p>`

	if err.Capability != "" {
		html += `<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-4">
					<p class="text-sm font-medium text-gray-700 mb-2">Required Permission:</p>
					<div class="font-mono text-sm bg-white px-3 py-2 rounded border border-gray-300">
						<span class="text-blue-600">` + err.Capability + `</span>`
		if err.Namespace != "" {
			html += ` <span class="text-gray-500">in namespace</span> <span class="text-green-600">` + err.Namespace + `</span>`
		}
		html += `</div>
				</div>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-6">
					<p class="font-medium text-gray-900 mb-3">What you can do:</p>
					<ul class="space-y-2">`
		for _, suggestion := range err.Suggestions {
			html += `<li class="flex items-start">
						<svg class="h-5 w-5 text-blue-500 mr-2 mt-0.5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
						</svg>
						<span class="text-gray-700">` + suggestion + `</span>
					</li>`
		}
		html += `</ul></div>`
	}

	html += `</div>

			<div class="text-center">
				<a href="/ui/" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
					<svg class="mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12l8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
					</svg>
					Return to Home
				</a>
			</div>
		</div>
	</div>
</body>
</html>`

	return html
}

// CheckPermission is a helper function that checks a permission and renders
// a 403 error if the check fails. Returns true if permission check passed.
func (h *Handler) CheckPermission(w http.ResponseWriter, r *http.Request, check bool, err ForbiddenError) bool {
	if !check {
		h.RenderForbidden(w, r, err)
		return false
	}
	return true
}

// NotFoundError represents a 404 Not Found error with details
type NotFoundError struct {
	Message     string
	RequestPath string
	Suggestions []string
}

// RenderNotFound renders a 404 Not Found error response.
// For HTMX requests, it returns a partial error message.
// For full page requests, it returns a full error page.
func (h *Handler) RenderNotFound(w http.ResponseWriter, r *http.Request, err NotFoundError) {
	w.WriteHeader(http.StatusNotFound)

	// For HTMX requests, return a simple error message
	if IsHTMXFromRequest(r) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderNotFoundPartial(err)))
		return
	}

	// For full page requests, render a complete error page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderNotFoundPage(err)))
}

// renderNotFoundPartial returns HTML for a partial error display (HTMX requests)
func renderNotFoundPartial(err NotFoundError) string {
	message := err.Message
	if message == "" {
		message = "The page you are looking for could not be found"
	}

	html := `<div class="rounded-lg bg-yellow-50 border border-yellow-200 p-6">
	<div class="flex">
		<div class="flex-shrink-0">
			<svg class="h-6 w-6 text-yellow-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
			</svg>
		</div>
		<div class="ml-3">
			<h3 class="text-sm font-medium text-yellow-800">Page Not Found</h3>
			<div class="mt-2 text-sm text-yellow-700">
				<p>` + message + `</p>`

	if err.RequestPath != "" {
		html += `<p class="mt-2"><strong>Requested:</strong> <code class="px-1.5 py-0.5 bg-yellow-100 rounded">` + err.RequestPath + `</code></p>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-3">
					<p class="font-medium">You might want to:</p>
					<ul class="list-disc list-inside mt-1 space-y-1">`
		for _, suggestion := range err.Suggestions {
			html += `<li>` + suggestion + `</li>`
		}
		html += `</ul></div>`
	}

	html += `<div class="mt-4">
					<a href="/ui/" class="text-yellow-800 hover:text-yellow-900 underline">Return to Home</a>
				</div>
			</div>
		</div>
	</div>
</div>`

	return html
}

// renderNotFoundPage returns HTML for a full error page
func renderNotFoundPage(err NotFoundError) string {
	message := err.Message
	if message == "" {
		message = "The page you are looking for could not be found"
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>404 Not Found - Wander</title>
	<script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-50">
	<div class="min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8">
		<div class="max-w-2xl w-full">
			<div class="text-center mb-8">
				<div class="inline-flex items-center justify-center w-16 h-16 bg-yellow-100 rounded-full mb-4">
					<svg class="h-8 w-8 text-yellow-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
					</svg>
				</div>
				<h1 class="text-4xl font-bold text-gray-900 mb-2">404 Not Found</h1>
				<p class="text-xl text-gray-600">Page Not Found</p>
			</div>

			<div class="bg-white shadow-lg rounded-lg p-8 mb-6">
				<p class="text-gray-700 text-lg mb-4">` + message + `</p>`

	if err.RequestPath != "" {
		html += `<div class="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-4">
					<p class="text-sm font-medium text-gray-700 mb-2">Requested Path:</p>
					<div class="font-mono text-sm bg-white px-3 py-2 rounded border border-gray-300">
						<span class="text-gray-900">` + err.RequestPath + `</span>
					</div>
				</div>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-6">
					<p class="font-medium text-gray-900 mb-3">You might want to:</p>
					<ul class="space-y-2">`
		for _, suggestion := range err.Suggestions {
			html += `<li class="flex items-start">
						<svg class="h-5 w-5 text-blue-500 mr-2 mt-0.5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
						</svg>
						<span class="text-gray-700">` + suggestion + `</span>
					</li>`
		}
		html += `</ul></div>`
	}

	html += `</div>

			<div class="text-center">
				<a href="/ui/" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500">
					<svg class="mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12l8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
					</svg>
					Return to Home
				</a>
			</div>
		</div>
	</div>
</body>
</html>`

	return html
}

// ClusterUnavailableError represents cluster unavailability (no leader elected)
type ClusterUnavailableError struct {
	Message     string
	Details     string
	Suggestions []string
}

// RenderClusterUnavailable renders a 503 Service Unavailable error for cluster issues.
// For HTMX requests, it returns a partial error message.
// For full page requests, it returns a full error page with auto-refresh.
func (h *Handler) RenderClusterUnavailable(w http.ResponseWriter, r *http.Request, err ClusterUnavailableError) {
	w.WriteHeader(http.StatusServiceUnavailable)

	// For HTMX requests, return a simple error message
	if IsHTMXFromRequest(r) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(renderClusterUnavailablePartial(err)))
		return
	}

	// For full page requests, render a complete error page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderClusterUnavailablePage(err)))
}

// renderClusterUnavailablePartial returns HTML for a partial error display (HTMX requests)
func renderClusterUnavailablePartial(err ClusterUnavailableError) string {
	message := err.Message
	if message == "" {
		message = "The Nomad cluster is currently unavailable"
	}

	html := `<div class="rounded-lg bg-orange-50 border border-orange-200 p-6">
	<div class="flex">
		<div class="flex-shrink-0">
			<svg class="h-6 w-6 text-orange-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
			</svg>
		</div>
		<div class="ml-3">
			<h3 class="text-sm font-medium text-orange-800">Cluster Unavailable</h3>
			<div class="mt-2 text-sm text-orange-700">
				<p>` + message + `</p>`

	if err.Details != "" {
		html += `<p class="mt-2 text-orange-600 font-medium">` + err.Details + `</p>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-3">
					<p class="font-medium">What you can do:</p>
					<ul class="list-disc list-inside mt-1 space-y-1">`
		for _, suggestion := range err.Suggestions {
			html += `<li>` + suggestion + `</li>`
		}
		html += `</ul></div>`
	}

	html += `<div class="mt-4">
					<button onclick="location.reload()" class="text-sm px-3 py-1.5 bg-orange-600 text-white rounded hover:bg-orange-700">
						Retry
					</button>
				</div>
			</div>
		</div>
	</div>
</div>`

	return html
}

// renderClusterUnavailablePage returns HTML for a full error page with auto-refresh
func renderClusterUnavailablePage(err ClusterUnavailableError) string {
	message := err.Message
	if message == "" {
		message = "The Nomad cluster is currently unavailable"
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Cluster Unavailable - Wander</title>
	<script src="https://cdn.tailwindcss.com"></script>
	<meta http-equiv="refresh" content="10">
</head>
<body class="bg-gray-50">
	<div class="min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8">
		<div class="max-w-2xl w-full">
			<div class="text-center mb-8">
				<div class="inline-flex items-center justify-center w-16 h-16 bg-orange-100 rounded-full mb-4">
					<svg class="h-8 w-8 text-orange-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
					</svg>
				</div>
				<h1 class="text-4xl font-bold text-gray-900 mb-2">Cluster Unavailable</h1>
				<p class="text-xl text-gray-600">No Cluster Leader</p>
			</div>

			<div class="bg-white shadow-lg rounded-lg p-8 mb-6">
				<p class="text-gray-700 text-lg mb-4">` + message + `</p>`

	if err.Details != "" {
		html += `<div class="bg-orange-50 border border-orange-200 rounded-lg p-4 mb-4">
					<p class="text-sm font-medium text-orange-800 mb-2">Details:</p>
					<p class="text-sm text-orange-700">` + err.Details + `</p>
				</div>`
	}

	if len(err.Suggestions) > 0 {
		html += `<div class="mt-6">
					<p class="font-medium text-gray-900 mb-3">What you can do:</p>
					<ul class="space-y-2">`
		for _, suggestion := range err.Suggestions {
			html += `<li class="flex items-start">
						<svg class="h-5 w-5 text-blue-500 mr-2 mt-0.5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
						</svg>
						<span class="text-gray-700">` + suggestion + `</span>
					</li>`
		}
		html += `</ul></div>`
	}

	html += `</div>

			<div class="text-center space-y-3">
				<div class="flex items-center justify-center space-x-2 text-sm text-gray-500">
					<svg class="animate-spin h-4 w-4 text-orange-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					<span>Auto-refreshing in 10 seconds...</span>
				</div>
				<button onclick="location.reload()" class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-orange-600 hover:bg-orange-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500">
					<svg class="mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
					</svg>
					Retry Now
				</button>
			</div>
		</div>
	</div>
</body>
</html>`

	return html
}

// IsClusterUnavailableError checks if an error indicates cluster unavailability.
// Returns true if the error message contains phrases indicating no cluster leader.
func IsClusterUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "no cluster leader") ||
		strings.Contains(errMsg, "no leader") ||
		strings.Contains(errMsg, "cluster unavailable") ||
		strings.Contains(errMsg, "server unavailable") ||
		strings.Contains(errMsg, "leadership lost")
}
