package handlers

import (
	"net/http"
)

// RegionAPIHandler handles region-related API requests.
type RegionAPIHandler struct {
	handler *Handler
}

// NewRegionAPIHandler creates a new region API handler.
func NewRegionAPIHandler(h *Handler) *RegionAPIHandler {
	return &RegionAPIHandler{
		handler: h,
	}
}

// ServeHTTP handles region API requests.
func (h *RegionAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/region/select":
		h.SelectRegion(w, r)
	default:
		http.NotFound(w, r)
	}
}

// SelectRegion handles POST requests to select a region.
func (h *RegionAPIHandler) SelectRegion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	region := r.URL.Query().Get("region")
	if region == "" {
		http.Error(w, "Region parameter required", http.StatusBadRequest)
		return
	}

	// Update the current region in app context
	h.handler.SetCurrentRegion(region)

	h.handler.logger.Info("region selected", "region", region)

	// Send HX-Trigger to show a toast notification and refresh the page
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Switched to region: `+region+`", "type": "success"}}`)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
