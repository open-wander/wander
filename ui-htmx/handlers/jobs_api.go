package handlers

import (
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
	// Expected format: /api/jobs/{jobID}/{action}
	parts := strings.Split(strings.TrimPrefix(path, "/api/jobs/"), "/")
	if len(parts) != 2 {
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
		h.StartJob(w, r, jobID)
	case "stop":
		h.StopJob(w, r, jobID)
	case "run":
		h.RunJob(w, r, jobID)
	default:
		http.NotFound(w, r)
	}
}

// StartJob handles starting a job.
func (h *JobsAPIHandler) StartJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// TODO: Implement actual Nomad API call to start the job
	h.handler.logger.Info("starting job", "jobID", jobID)

	// For now, just return a success toast
	// In production, this would call the Nomad API and handle errors
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Job started successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// StopJob handles stopping a job.
func (h *JobsAPIHandler) StopJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// Parse namespace from job ID if provided (format: jobID@namespace)
	namespace := "default"
	jobName := jobID
	if idx := strings.LastIndex(jobID, "@"); idx != -1 {
		jobName = jobID[:idx]
		namespace = jobID[idx+1:]
	}

	// Check permissions - need submit-job capability to stop jobs
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanSubmitJob(namespace) {
		h.handler.logger.Warn("stop permission denied", "jobID", jobID, "namespace", namespace)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Permission denied: you do not have the 'submit-job' capability", "type": "error"}}`)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	h.handler.logger.Info("stopping job", "jobID", jobName, "namespace", namespace)

	// Get Nomad client
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Nomad client not available", "type": "error"}}`)
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

	// Stop the job (purge=false means just stop, don't remove from system)
	evalID, _, err := client.Jobs().Deregister(jobName, false, writeOpts)
	if err != nil {
		h.handler.logger.Error("failed to stop job", "error", err, "jobID", jobName, "namespace", namespace)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to stop job: `+err.Error()+`", "type": "error"}}`)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.handler.logger.Info("job stopped successfully", "jobID", jobName, "namespace", namespace, "evalID", evalID)

	// Return success with eval ID
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Job stopped successfully (Eval: `+evalID[:8]+`...)", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

// RunJob handles submitting a new version of a job.
func (h *JobsAPIHandler) RunJob(w http.ResponseWriter, r *http.Request, jobID string) {
	// TODO: Implement actual Nomad API call to run/resubmit the job
	h.handler.logger.Info("running job", "jobID", jobID)

	// For now, just return a success toast
	// In production, this would call the Nomad API and handle errors
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Job submitted successfully", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}
