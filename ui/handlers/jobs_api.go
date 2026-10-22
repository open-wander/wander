package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/open-wander/wander/api"
)

// JobsAPIHandler handles API requests for job actions.
type JobsAPIHandler struct {
	handler *Handler
}

// NewJobsAPIHandler creates a new jobs API handler.
func NewJobsAPIHandler(h *Handler) *JobsAPIHandler {
	return &JobsAPIHandler{
		handler: h,
	}
}

// ServeHTTP handles POST requests to job action endpoints.
func (h *JobsAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Extract job ID and action from path
	// Expected format: /api/jobs/{jobID}/{action} or /api/jobs/{jobID}/periodic/force
	parts := strings.Split(strings.TrimPrefix(path, "/api/jobs/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	jobID := parts[0]
	action := parts[1]

	// Log the request
	h.handler.logger.Debug("job action request", "jobID", jobID, "action", action)

	// Route to appropriate handler
	switch action {
	case "start":
		h.registerExistingJob(w, r, jobID, "started")
	case "stop":
		h.StopJob(w, r, jobID)
	case "run":
		h.registerExistingJob(w, r, jobID, "submitted")
	case "dispatch":
		h.DispatchJob(w, r, jobID)
	case "periodic":
		// Handle nested periodic actions
		if len(parts) > 2 && parts[2] == "force" {
			h.ForcePeriodicJob(w, r, jobID)
		} else {
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

// StopJob handles stopping a job.
func (h *JobsAPIHandler) StopJob(w http.ResponseWriter, r *http.Request, jobID string) {
	client, jobName, namespace, ok := h.prepareJobAction(w, r, jobID, "stop")
	if !ok {
		return
	}
	h.handler.logger.Info("stopping job", "jobID", jobName, "namespace", namespace)

	writeOpts := jobWriteOptions(r, namespace)

	// Stop the job (purge=false means just stop, don't remove from system)
	evalID, _, err := client.Jobs().Deregister(jobName, false, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to stop job", "error", err, "jobID", jobName, "namespace", namespace)
		setToast(w, "Failed to stop job: "+err.Error(), "error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("job stopped successfully", "jobID", jobName, "namespace", namespace, "evalID", evalID)

	// Return success with eval ID
	setToast(w, "Job stopped successfully (Eval: "+shortIdentifier(evalID)+")", "success")
	w.WriteHeader(http.StatusOK)
}

func (h *JobsAPIHandler) registerExistingJob(w http.ResponseWriter, r *http.Request, jobID, action string) {
	client, jobName, namespace, ok := h.prepareJobAction(w, r, jobID, action)
	if !ok {
		return
	}

	token := GetTokenFromRequest(r)
	queryOpts := &api.QueryOptions{Namespace: namespace, AuthToken: token}
	job, _, err := client.Jobs().Info(jobName, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to load job", "jobID", jobName, "namespace", namespace, "error", err)
		setToast(w, "Failed to load the job before submission", "error")
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	stopped := false
	job.Stop = &stopped
	response, _, err := client.Jobs().Register(job, &api.WriteOptions{Namespace: namespace, AuthToken: token})
	if err != nil {
		h.handler.logger.Error("failed to register job", "jobID", jobName, "namespace", namespace, "error", err)
		setToast(w, "Failed to submit the job", "error")
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	h.handler.logger.Info("job registered", "jobID", jobName, "namespace", namespace, "evalID", response.EvalID)
	setToast(w, fmt.Sprintf("Job %s successfully (Eval: %s)", action, shortIdentifier(response.EvalID)), "success")
	w.WriteHeader(http.StatusOK)
}

func splitJobReference(jobID, fallbackNamespace string) (string, string) {
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		return jobID[:idx], jobID[idx+1:]
	}
	return jobID, fallbackNamespace
}

func jobWriteOptions(r *http.Request, namespace string) *api.WriteOptions {
	return &api.WriteOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
}

func setToast(w http.ResponseWriter, message, toastType string) {
	payload, err := json.Marshal(map[string]any{
		"showToast": map[string]string{
			"message": message,
			"type":    toastType,
		},
	})
	if err == nil {
		w.Header().Set("HX-Trigger", string(payload))
	}
}

func (h *JobsAPIHandler) prepareJobAction(w http.ResponseWriter, r *http.Request, jobID, action string) (*api.Client, string, string, bool) {
	jobName, namespace := splitJobReference(jobID, GetNamespaceFromRequest(r))
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("job action permission denied", "action", action, "jobID", jobID, "namespace", namespace)
		setToast(w, "Permission denied: you do not have the 'submit-job' capability", "error")
		w.WriteHeader(http.StatusForbidden)
		return nil, "", "", false
	}

	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("Wander API client unavailable", "action", action)
		setToast(w, "Wander API client is unavailable", "error")
		w.WriteHeader(http.StatusServiceUnavailable)
		return nil, "", "", false
	}
	return client, jobName, namespace, true
}

// ForcePeriodicJob handles forcing a periodic job to run immediately.
func (h *JobsAPIHandler) ForcePeriodicJob(w http.ResponseWriter, r *http.Request, jobID string) {
	client, jobName, namespace, ok := h.prepareJobAction(w, r, jobID, "force periodic")
	if !ok {
		return
	}
	h.handler.logger.Info("forcing periodic job", "jobID", jobName, "namespace", namespace)

	writeOpts := jobWriteOptions(r, namespace)

	// Force the periodic job to run
	evalID, _, err := client.Jobs().PeriodicForce(jobName, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to force periodic job", "error", err, "jobID", jobName, "namespace", namespace)
		setToast(w, "Failed to force periodic job: "+err.Error(), "error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("periodic job forced successfully", "jobID", jobName, "namespace", namespace, "evalID", evalID)

	// Return success with eval ID
	setToast(w, "Periodic job forced successfully (Eval: "+shortIdentifier(evalID)+")", "success")
	w.WriteHeader(http.StatusOK)
}

// ScaleTaskGroupRequest represents the JSON payload for scaling a task group.
type ScaleTaskGroupRequest struct {
	Count   *int                   `json:"Count"`
	Message string                 `json:"Message"`
	Error   bool                   `json:"Error"`
	Meta    map[string]interface{} `json:"Meta"`
}

// ScaleTaskGroup handles scaling a task group in a job.
func (h *JobsAPIHandler) ScaleTaskGroup(w http.ResponseWriter, r *http.Request, jobID, taskGroup string) {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	// Check permissions - need scale-job capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanScaleJob(namespace) {
		h.handler.logger.Warn("scale permission denied", "jobID", jobID, "namespace", namespace, "taskGroup", taskGroup)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Permission denied: you do not have the 'scale-job' capability", "type": "error"}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Parse request body
	var req ScaleTaskGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handler.logger.Error("failed to parse scale request", "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Invalid request format", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate count is provided
	if req.Count == nil {
		h.handler.logger.Error("count not provided in scale request")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Count is required", "type": "error"}}`)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.handler.logger.Info("scaling task group", "jobID", jobName, "namespace", namespace, "taskGroup", taskGroup, "count", *req.Count)

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Wander client not available", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
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

	// Scale the task group
	resp, _, err := client.Jobs().Scale(jobName, taskGroup, req.Count, req.Message, req.Error, req.Meta, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to scale task group", "error", err, "jobID", jobName, "namespace", namespace, "taskGroup", taskGroup)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to scale task group: `+err.Error()+`", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("task group scaled successfully", "jobID", jobName, "namespace", namespace, "taskGroup", taskGroup, "evalID", resp.EvalID)

	// Return success with eval ID in JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"evalID": resp.EvalID,
	})
}

// DispatchJobRequest represents the request body for dispatch endpoint.
type DispatchJobRequest struct {
	Meta    map[string]string `json:"meta,omitempty"`
	Payload string            `json:"payload,omitempty"` // base64 encoded
}

// DispatchJobResponse represents the response from dispatch endpoint.
type DispatchJobResponse struct {
	DispatchedJobID string `json:"dispatchedJobId"`
	EvalID          string `json:"evalId"`
	Error           string `json:"error,omitempty"`
}

// DispatchJob handles dispatching a parameterized job.
// Route: POST /api/jobs/{id}/dispatch
func (h *JobsAPIHandler) DispatchJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := GetNamespaceFromRequest(r)
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	// Check permissions - need submit-job capability to dispatch jobs
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("dispatch permission denied", "jobID", jobID, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(DispatchJobResponse{
			Error: fmt.Sprintf("Permission denied: you do not have the 'submit-job' capability in the %s namespace", namespace),
		})
		return
	}

	h.handler.logger.Info("dispatching parameterized job", "jobID", jobName, "namespace", namespace)

	// Parse optional request body for meta and payload
	var req DispatchJobRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.handler.logger.Error("failed to decode dispatch request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(DispatchJobResponse{Error: "Invalid request body"})
			return
		}
	}

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(DispatchJobResponse{Error: "Wander client not available"})
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

	// Dispatch the job
	// idPrefixTemplate is empty string - let Nomad generate the ID
	dispatchResp, _, err := client.Jobs().Dispatch(jobName, req.Meta, []byte(req.Payload), "", writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to dispatch job", "error", err, "jobID", jobName, "namespace", namespace)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(DispatchJobResponse{
			Error: fmt.Sprintf("Failed to dispatch job: %v", err),
		})
		return
	}

	h.handler.logger.Info("job dispatched successfully",
		"jobID", jobName,
		"namespace", namespace,
		"dispatchedJobID", dispatchResp.DispatchedJobID,
		"evalID", dispatchResp.EvalID,
	)

	// Return success response with dispatch ID and eval ID
	response := DispatchJobResponse{
		DispatchedJobID: dispatchResp.DispatchedJobID,
		EvalID:          dispatchResp.EvalID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode dispatch response", "error", err)
	}
}
