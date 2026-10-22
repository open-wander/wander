package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/open-wander/wander/api"
)

// DeploymentsAPIHandler handles API requests for deployment actions.
type DeploymentsAPIHandler struct {
	handler *Handler
}

// NewDeploymentsAPIHandler creates a new deployments API handler.
func NewDeploymentsAPIHandler(h *Handler) *DeploymentsAPIHandler {
	return &DeploymentsAPIHandler{
		handler: h,
	}
}

// ServeHTTP handles POST requests to the deployments API endpoint.
func (h *DeploymentsAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Expected format: /api/deployments/{deploymentID}/{action}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/deployments/"), "/")
	if len(parts) != 2 || parts[0] == "" || (parts[1] != "fail" && parts[1] != "promote") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deploymentID := parts[0]
	action := parts[1]

	switch action {
	case "fail":
		h.FailDeployment(w, r, deploymentID)
	case "promote":
		h.PromoteDeployment(w, r, deploymentID)
	}
}

// FailDeployment handles failing a deployment.
// Route: POST /api/deployments/{id}/fail
func (h *DeploymentsAPIHandler) FailDeployment(w http.ResponseWriter, r *http.Request, deploymentID string) {
	// Parse namespace from query param or use default
	namespace := GetNamespaceFromRequest(r)

	// Check permissions - need submit-job capability to fail deployments
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("fail deployment permission denied", "deploymentID", deploymentID, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Permission denied: you do not have the 'submit-job' capability in the " + namespace + " namespace",
		})
		return
	}

	h.handler.logger.Info("failing deployment", "deploymentID", deploymentID, "namespace", namespace)

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Wander client not available",
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build write options with namespace and token
	writeOpts := &api.WriteOptions{
		Namespace: namespace,
	}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Fail the deployment
	resp, _, err := client.Deployments().Fail(deploymentID, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to fail deployment", "error", err, "deploymentID", deploymentID, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to fail deployment: " + err.Error(),
		})
		return
	}

	h.handler.logger.Info("deployment failed successfully", "deploymentID", deploymentID, "namespace", namespace, "evalID", resp.EvalID)

	// Return success response with eval ID
	if IsHTMXFromRequest(r) {
		w.Header().Set("HX-Refresh", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"evalID":  resp.EvalID,
		"message": fmt.Sprintf("Deployment failed successfully (Eval: %s)", shortIdentifier(resp.EvalID)),
	})
}

// PromoteDeploymentRequest represents the JSON payload for promoting a deployment.
type PromoteDeploymentRequest struct {
	All    bool     `json:"All,omitempty"`
	Groups []string `json:"Groups,omitempty"`
}

// PromoteDeploymentResponse represents the response from promote endpoint.
type PromoteDeploymentResponse struct {
	EvalID string `json:"evalId"`
	Error  string `json:"error,omitempty"`
}

// PromoteDeployment handles promoting canaries in a deployment.
// Route: POST /api/deployments/{id}/promote
func (h *DeploymentsAPIHandler) PromoteDeployment(w http.ResponseWriter, r *http.Request, deploymentID string) {
	perms := GetPermissionsFromRequest(r)
	if perms != nil && perms.IsAnonymous() {
		handlerError := PromoteDeploymentResponse{Error: "Permission denied: a valid ACL token is required"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(handlerError)
		return
	}

	// Get the deployment to determine its namespace
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{Error: "Wander client not available"})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Build query options to fetch deployment
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch deployment to get namespace
	deploy, _, err := client.Deployments().Info(deploymentID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch deployment", "error", err, "deploymentID", deploymentID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{
			Error: fmt.Sprintf("Failed to fetch deployment: %v", err),
		})
		return
	}

	if deploy == nil {
		h.handler.logger.Warn("deployment not found", "deploymentID", deploymentID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{Error: "Deployment not found"})
		return
	}

	namespace := deploy.Namespace

	// Check permissions - need submit-job capability to promote deployments
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("promote permission denied", "deploymentID", deploymentID, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{
			Error: fmt.Sprintf("Permission denied: you do not have the 'submit-job' capability in the %s namespace", namespace),
		})
		return
	}

	h.handler.logger.Info("promoting deployment", "deploymentID", deploymentID, "namespace", namespace)

	// Parse optional request body for Groups/All
	var req PromoteDeploymentRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.handler.logger.Error("failed to decode promote request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{Error: "Invalid request body"})
			return
		}
	} else {
		// Default to promoting all if no body provided
		req.All = true
	}

	// Build write options with namespace and token
	writeOpts := &api.WriteOptions{
		Namespace: namespace,
	}
	if token != "" {
		writeOpts.AuthToken = token
	}

	// Call appropriate promote method based on request
	var promoteResp *api.DeploymentUpdateResponse
	var writeMeta *api.WriteMeta

	if req.All || len(req.Groups) == 0 {
		// Promote all groups
		promoteResp, writeMeta, err = client.Deployments().PromoteAll(deploymentID, writeOpts)
	} else {
		// Promote specific groups
		promoteResp, writeMeta, err = client.Deployments().PromoteGroups(deploymentID, req.Groups, writeOpts)
	}

	if err != nil {
		h.handler.logger.Error("failed to promote deployment", "error", err, "deploymentID", deploymentID, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{
			Error: fmt.Sprintf("Failed to promote deployment: %v", err),
		})
		return
	}

	h.handler.logger.Info("deployment promoted successfully",
		"deploymentID", deploymentID,
		"namespace", namespace,
		"evalID", promoteResp.EvalID,
		"index", writeMeta.LastIndex,
	)

	// Return success response with eval ID
	response := PromoteDeploymentResponse{
		EvalID: promoteResp.EvalID,
	}

	if IsHTMXFromRequest(r) {
		w.Header().Set("HX-Refresh", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode promote response", "error", err)
	}
}
