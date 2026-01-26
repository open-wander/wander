package handlers

import (
	"net/http"
)

// NamespaceAPIHandler handles namespace-related API requests.
type NamespaceAPIHandler struct {
	handler *Handler
}

// NewNamespaceAPIHandler creates a new namespace API handler.
func NewNamespaceAPIHandler(h *Handler) *NamespaceAPIHandler {
	return &NamespaceAPIHandler{
		handler: h,
	}
}

// ServeHTTP handles namespace API requests.
func (h *NamespaceAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/namespace/select":
		h.SelectNamespace(w, r)
	default:
		http.NotFound(w, r)
	}
}

// SelectNamespace handles POST requests to select a namespace.
func (h *NamespaceAPIHandler) SelectNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ns := r.URL.Query().Get("ns")
	if ns == "" {
		http.Error(w, "Namespace parameter required", http.StatusBadRequest)
		return
	}

	// Update the current namespace in app context
	h.handler.SetCurrentNamespace(ns)

	h.handler.logger.Info("namespace selected", "namespace", ns)

	// Send HX-Trigger to show a toast notification and refresh the page
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Switched to namespace: `+ns+`", "type": "success"}}`)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
