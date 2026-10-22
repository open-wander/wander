package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/open-wander/wander/api"
)

// ClientsAPIHandler handles API requests for client node actions.
type ClientsAPIHandler struct {
	handler *Handler
}

// NewClientsAPIHandler creates a new clients API handler.
func NewClientsAPIHandler(h *Handler) *ClientsAPIHandler {
	return &ClientsAPIHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to client action endpoints.
func (h *ClientsAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Extract client ID and action from path
	// Expected format: /api/clients/{clientID}/{action}
	parts := strings.Split(strings.TrimPrefix(path, "/api/clients/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	clientID := parts[0]
	action := parts[1]

	// Log the request
	h.handler.logger.Debug("client action request", "clientID", clientID, "action", action)

	// Route to appropriate handler
	switch action {
	case "eligibility":
		h.ToggleEligibility(w, r, clientID)
	default:
		http.NotFound(w, r)
	}
}

// ToggleEligibilityRequest represents the JSON payload for toggling eligibility.
type ToggleEligibilityRequest struct {
	Eligible bool `json:"Eligible"`
}

// ToggleEligibilityResponse represents the response from the eligibility endpoint.
type ToggleEligibilityResponse struct {
	EvalIDs []string `json:"evalIds,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ToggleEligibility handles toggling a node's scheduling eligibility.
// Route: POST /api/clients/{id}/eligibility
func (h *ClientsAPIHandler) ToggleEligibility(w http.ResponseWriter, r *http.Request, clientID string) {
	// Check permissions - need node:write capability to change eligibility
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability("", "node:write") {
		h.handler.logger.Warn("toggle eligibility permission denied", "clientID", clientID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(ToggleEligibilityResponse{
			Error: "Permission denied: you do not have the 'node:write' capability",
		})
		return
	}

	// Parse request body
	var req ToggleEligibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to decode toggle eligibility request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ToggleEligibilityResponse{Error: "Invalid request body"})
		return
	}

	h.handler.logger.Info("toggling node eligibility", "clientID", clientID, "eligible", req.Eligible)

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ToggleEligibilityResponse{Error: "Wander client not available"})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with token
	writeOpts := &api.WriteOptions{}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Toggle node eligibility
	resp, err := client.Nodes().ToggleEligibility(clientID, req.Eligible, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to toggle node eligibility", "error", err, "clientID", clientID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ToggleEligibilityResponse{
			Error: err.Error(),
		})
		return
	}

	h.handler.logger.Info("node eligibility toggled successfully",
		"clientID", clientID,
		"eligible", req.Eligible,
		"evalIDs", resp.EvalIDs,
	)

	// Return success response with eval IDs
	response := ToggleEligibilityResponse{
		EvalIDs: resp.EvalIDs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode toggle eligibility response", "error", err)
	}
}
