package handlers

import (
	"net/http"
	"strings"
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
	// TODO: Implement actual Nomad API call to stop the job
	h.handler.logger.Info("stopping job", "jobID", jobID)

	// For now, just return a success toast
	// In production, this would call the Nomad API and handle errors
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Job stopped successfully", "type": "success"}}`)
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
