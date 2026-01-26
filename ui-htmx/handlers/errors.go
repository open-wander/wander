package handlers

import (
	"net/http"
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
