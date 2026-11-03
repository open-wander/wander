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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		writeDeploymentMutationError(w, http.StatusServiceUnavailable, "Wander client not available")
		return
	}

	token := GetTokenFromRequest(r)
	deploy, err := h.deploymentForMutation(r, client, deploymentID, token)
	if err != nil {
		h.writeDeploymentBackendError(w, "load deployment before failing", deploymentID, err)
		return
	}
	if deploy == nil {
		writeDeploymentMutationError(w, http.StatusNotFound, "Deployment not found")
		return
	}
	namespace := deploy.Namespace

	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("fail deployment permission denied", "deploymentID", deploymentID, "namespace", namespace)
		writeDeploymentMutationError(w, http.StatusForbidden, "Permission denied: submit-job capability is required")
		return
	}

	h.handler.logger.Info("failing deployment", "deploymentID", deploymentID, "namespace", namespace)

	writeOpts := &api.WriteOptions{
		Namespace: namespace,
		Region:    GetRegionFromRequest(r),
		AuthToken: token,
	}

	resp, _, err := client.Deployments().Fail(deploymentID, writeOpts)
	if err != nil {
		h.writeDeploymentBackendError(w, "fail deployment", deploymentID, err)
		return
	}

	h.handler.logger.Info("deployment failed successfully", "deploymentID", deploymentID, "namespace", namespace, "evalID", resp.EvalID)

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
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		writeDeploymentMutationError(w, http.StatusServiceUnavailable, "Wander client not available")
		return
	}

	token := GetTokenFromRequest(r)
	deploy, err := h.deploymentForMutation(r, client, deploymentID, token)
	if err != nil {
		h.writeDeploymentBackendError(w, "load deployment before promoting", deploymentID, err)
		return
	}

	if deploy == nil {
		h.handler.logger.Warn("deployment not found", "deploymentID", deploymentID)
		writeDeploymentMutationError(w, http.StatusNotFound, "Deployment not found")
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

	writeOpts := &api.WriteOptions{
		Namespace: namespace,
		Region:    GetRegionFromRequest(r),
		AuthToken: token,
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
		h.writeDeploymentBackendError(w, "promote deployment", deploymentID, err)
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

func (h *DeploymentsAPIHandler) deploymentForMutation(r *http.Request, client *api.Client, deploymentID, token string) (*api.Deployment, error) {
	returnValue, _, err := client.Deployments().Info(deploymentID, &api.QueryOptions{
		Namespace: GetNamespaceFromRequest(r),
		Region:    GetRegionFromRequest(r),
		AuthToken: token,
	})
	return returnValue, err
}

func (h *DeploymentsAPIHandler) writeDeploymentBackendError(w http.ResponseWriter, action, deploymentID string, err error) {
	h.handler.logger.Error("failed to "+action, "error", err, "deploymentID", deploymentID)
	switch {
	case apiResponseHasStatus(err, http.StatusForbidden):
		writeDeploymentMutationError(w, http.StatusForbidden, "Permission denied: submit-job capability is required")
	case apiResponseHasStatus(err, http.StatusNotFound):
		writeDeploymentMutationError(w, http.StatusNotFound, "Deployment not found")
	default:
		writeDeploymentMutationError(w, http.StatusBadGateway, "The cluster rejected the deployment request")
	}
}

func writeDeploymentMutationError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(PromoteDeploymentResponse{Error: message})
}
