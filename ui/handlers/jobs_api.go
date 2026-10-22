package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/api"
)

const (
	maxDispatchPayloadSize = 16 * 1024
	maxDispatchRequestSize = 64 * 1024
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
	jobID, action, ok := splitJobActionPath(r.URL.Path)
	if !ok || !supportedJobAction(action) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	case "purge":
		h.PurgeJob(w, r, jobID)
	case "periodic/force":
		h.ForcePeriodicJob(w, r, jobID)
	default:
		http.NotFound(w, r)
	}
}

func supportedJobAction(action string) bool {
	switch action {
	case "start", "stop", "run", "dispatch", "purge", "periodic/force":
		return true
	default:
		return false
	}
}

func splitJobActionPath(path string) (string, string, bool) {
	remainder := strings.TrimPrefix(path, "/api/jobs/")
	if remainder == path || remainder == "" {
		return "", "", false
	}
	if strings.HasSuffix(remainder, "/periodic/force") {
		jobID := strings.TrimSuffix(remainder, "/periodic/force")
		return jobID, "periodic/force", jobID != ""
	}
	separator := strings.LastIndex(remainder, "/")
	if separator < 1 || separator == len(remainder)-1 {
		return "", "", false
	}
	return remainder[:separator], remainder[separator+1:], true
}

// StopJob handles stopping a job.
func (h *JobsAPIHandler) StopJob(w http.ResponseWriter, r *http.Request, jobID string) {
	h.deregisterJob(w, r, jobID, false)
}

// PurgeJob permanently removes a job and its history.
func (h *JobsAPIHandler) PurgeJob(w http.ResponseWriter, r *http.Request, jobID string) {
	h.deregisterJob(w, r, jobID, true)
}

func (h *JobsAPIHandler) deregisterJob(w http.ResponseWriter, r *http.Request, jobID string, purge bool) {
	action := "stop"
	if purge {
		action = "purge"
	}
	client, jobName, namespace, ok := h.prepareJobAction(w, r, jobID, action)
	if !ok {
		return
	}

	evalID, _, err := client.Jobs().Deregister(jobName, purge, jobWriteOptions(r, namespace))
	if err != nil {
		h.handler.logger.Error("failed to "+action+" job", "error", err, "jobID", jobName, "namespace", namespace)
		setToast(w, "Failed to "+action+" job: "+err.Error(), "error")
		status := http.StatusInternalServerError
		if purge {
			status = http.StatusBadGateway
		}
		w.WriteHeader(status)
		return
	}

	h.handler.logger.Info("job "+action+" successful", "jobID", jobName, "namespace", namespace, "evalID", evalID)
	if purge {
		setToast(w, "Job purged successfully", "success")
		w.Header().Set("HX-Redirect", scopedUIURL("/ui/jobs", namespace, GetRegionFromRequest(r)))
	} else {
		setToast(w, "Job stopped successfully (Eval: "+shortIdentifier(evalID)+")", "success")
	}
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
	Payload string            `json:"payload,omitempty"`
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
	jobName, namespace := splitJobReference(jobID, GetNamespaceFromRequest(r))
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.HasCapability(namespace, acl.NamespaceCapabilityDispatchJob) {
		h.handler.logger.Warn("dispatch permission denied", "jobID", jobID, "namespace", namespace)
		writeDispatchError(w, http.StatusForbidden, fmt.Sprintf("Permission denied: you do not have the 'dispatch-job' capability in the %s namespace", namespace))
		return
	}

	h.handler.logger.Info("dispatching parameterized job", "jobID", jobName, "namespace", namespace)

	req, err := decodeDispatchRequest(w, r)
	if err != nil {
		h.handler.logger.Warn("invalid dispatch request", "error", err, "jobID", jobName, "namespace", namespace)
		writeDispatchError(w, http.StatusBadRequest, "Invalid dispatch request: "+err.Error())
		return
	}

	// Get Nomad client
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available")
		writeDispatchError(w, http.StatusServiceUnavailable, "Wander client not available")
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	job, _, err := client.Jobs().Info(jobName, &api.QueryOptions{Namespace: namespace, AuthToken: token})
	if err != nil {
		h.handler.logger.Error("failed to load parameterized job", "error", err, "jobID", jobName, "namespace", namespace)
		writeDispatchError(w, http.StatusBadGateway, "Failed to load parameterized job")
		return
	}
	if err := validateDispatchInput(job, req); err != nil {
		writeDispatchError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Dispatch the job
	// idPrefixTemplate is empty string - let Nomad generate the ID
	dispatchResp, _, err := client.Jobs().Dispatch(jobName, req.Meta, []byte(req.Payload), "", jobWriteOptions(r, namespace))
	if err != nil {
		h.handler.logger.Error("failed to dispatch job", "error", err, "jobID", jobName, "namespace", namespace)
		writeDispatchError(w, http.StatusBadGateway, fmt.Sprintf("Failed to dispatch job: %v", err))
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
	w.Header().Set("HX-Redirect", scopedUIURL("/ui/jobs/"+url.PathEscape(dispatchResp.DispatchedJobID), namespace, GetRegionFromRequest(r)))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handler.logger.Error("failed to encode dispatch response", "error", err)
	}
}

func decodeDispatchRequest(w http.ResponseWriter, r *http.Request) (DispatchJobRequest, error) {
	var req DispatchJobRequest
	if r.Body == nil || r.ContentLength == 0 {
		return req, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDispatchRequestSize)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, err
		}
		return req, nil
	}
	if err := r.ParseForm(); err != nil {
		return req, err
	}
	req.Payload = r.Form.Get("payload")
	req.Meta = make(map[string]string)
	for key, values := range r.Form {
		if strings.HasPrefix(key, "meta.") && len(values) > 0 {
			req.Meta[strings.TrimPrefix(key, "meta.")] = values[0]
		}
	}
	return req, nil
}

func validateDispatchInput(job *api.Job, req DispatchJobRequest) error {
	if job == nil || !job.IsParameterized() {
		return fmt.Errorf("job is not a parameterized parent job")
	}
	config := job.ParameterizedJob
	if config.Payload == "required" && req.Payload == "" {
		return fmt.Errorf("payload is required")
	}
	if config.Payload == "forbidden" && req.Payload != "" {
		return fmt.Errorf("payload is forbidden")
	}
	if len(req.Payload) > maxDispatchPayloadSize {
		return fmt.Errorf("payload exceeds the 16 KiB limit")
	}

	allowed := make(map[string]bool, len(config.MetaRequired)+len(config.MetaOptional))
	for _, key := range config.MetaRequired {
		allowed[key] = true
		if _, ok := req.Meta[key]; !ok {
			return fmt.Errorf("metadata field %q is required", key)
		}
	}
	for _, key := range config.MetaOptional {
		allowed[key] = true
	}
	for key := range req.Meta {
		if !allowed[key] {
			return fmt.Errorf("metadata field %q is not allowed", key)
		}
	}
	return nil
}

func writeDispatchError(w http.ResponseWriter, status int, message string) {
	setToast(w, message, "error")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(DispatchJobResponse{Error: message})
}

func scopedUIURL(path, namespace, region string) string {
	query := url.Values{}
	if namespace != "" {
		query.Set("namespace", namespace)
	}
	if region != "" {
		query.Set("region", region)
	}
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}
