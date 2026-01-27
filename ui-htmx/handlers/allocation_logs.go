package handlers

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/allocations"
	"github.com/open-wander/wander/ui-htmx/templates/partials/allocationspartials"
)

// AllocationLogsHandler handles requests for allocation log streaming.
type AllocationLogsHandler struct {
	handler *Handler
}

// NewAllocationLogsHandler creates a new allocation logs handler.
func NewAllocationLogsHandler(h *Handler) *AllocationLogsHandler {
	return &AllocationLogsHandler{
		handler: h,
	}
}

// LogLine represents a single log line to send via SSE.
type LogLine struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level,omitempty"` // error, warn, info, debug
}

// ServeHTTP handles log-related requests.
// Routes:
// - GET /ui/allocations/{id}/logs - Returns logs tab partial
// - GET /ui/allocations/{id}/logs/stream - SSE stream for real-time logs
func (h *AllocationLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Extract allocation ID from path
	// Path format: /ui/allocations/{id}/logs or /ui/allocations/{id}/logs/stream
	if !strings.HasPrefix(path, "/ui/allocations/") {
		http.NotFound(w, r)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(path, "/ui/allocations/"), "/")
	if len(pathParts) < 2 {
		http.NotFound(w, r)
		return
	}

	allocID := pathParts[0]

	// Check if this is a stream request
	if len(pathParts) >= 3 && pathParts[2] == "stream" {
		h.StreamLogs(w, r, allocID)
		return
	}

	// Return the logs tab partial
	h.LogsTab(w, r, allocID)
}

// LogsTab renders the logs tab.
// For HTMX requests: returns just the partial (for tab switching)
// For full page loads: returns the allocation detail page with logs tab active
func (h *AllocationLogsHandler) LogsTab(w http.ResponseWriter, r *http.Request, allocID string) {
	h.handler.logger.Debug("logs tab request", "allocID", allocID, "htmx", IsHTMXFromRequest(r))

	// Get task from query params or use first task
	task := r.URL.Query().Get("task")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "stdout"
	}

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	// Get allocation details to get task list from Nomad API
	tasks := h.getTasksForAllocation(allocID, token)

	if task == "" && len(tasks) > 0 {
		task = tasks[0]
	}

	props := allocationspartials.LogsTabProps{
		AllocID:      allocID,
		Tasks:        tasks,
		SelectedTask: task,
		LogType:      logType,
	}

	// For HTMX requests, return just the partial
	if IsHTMXFromRequest(r) {
		err := RenderPartial(w, r, allocationspartials.LogsTabContent(props))
		if err != nil {
			h.handler.logger.Error("failed to render logs tab partial", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// For full page loads, render the allocation detail page with logs tab active
	allocDetail := h.getAllocationDetailForLogs(allocID, token)
	allocDetail.ActiveTab = "logs"

	// Build layout props
	shortID := allocID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	breadcrumbs := []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
	}
	if allocDetail.JobID != "" {
		breadcrumbs = append(breadcrumbs,
			components.Breadcrumb{Label: "Jobs", URL: "/ui/jobs"},
			components.Breadcrumb{Label: allocDetail.JobID, URL: "/ui/jobs/" + allocDetail.JobID},
			components.Breadcrumb{Label: shortID, URL: "/ui/allocations/" + allocID},
			components.Breadcrumb{Label: "Logs", URL: ""},
		)
	} else {
		breadcrumbs = append(breadcrumbs,
			components.Breadcrumb{Label: "Allocations", URL: "/ui/allocations"},
			components.Breadcrumb{Label: shortID, URL: "/ui/allocations/" + allocID},
			components.Breadcrumb{Label: "Logs", URL: ""},
		)
	}

	layoutProps := h.handler.BaseLayoutProps(
		"Logs - Allocation "+shortID+" - Wander",
		"Allocation logs for "+allocID,
		"/ui/allocations/"+allocID+"/logs",
		breadcrumbs,
	)

	err := Render(w, r,
		allocations.Detail(layoutProps, allocDetail),
		allocationspartials.LogsTabContent(props),
	)
	if err != nil {
		h.handler.logger.Error("failed to render logs page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// StreamLogs handles SSE streaming of allocation logs.
func (h *AllocationLogsHandler) StreamLogs(w http.ResponseWriter, r *http.Request, allocID string) {
	task := r.URL.Query().Get("task")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "stdout"
	}

	// Parse follow parameter (default: true for streaming)
	follow := true
	if followStr := r.URL.Query().Get("follow"); followStr != "" {
		follow = followStr == "true"
	}

	// Parse offset parameter (default: 0)
	offset := int64(0)
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			offset = parsedOffset
		}
	}

	// Parse origin parameter (default: "end" for tailing)
	origin := "end"
	if originStr := r.URL.Query().Get("origin"); originStr != "" {
		origin = originStr
	}

	h.handler.logger.Debug("starting log stream",
		"allocID", allocID,
		"task", task,
		"type", logType,
		"follow", follow,
		"offset", offset,
		"origin", origin,
	)

	// Validate required parameters
	if task == "" {
		h.handler.logger.Warn("log stream missing task parameter", "allocID", allocID)
		http.Error(w, "Missing required parameter: task", http.StatusBadRequest)
		return
	}

	// Create SSE handler
	sse, err := NewSSEHandler(w, r)
	if err != nil {
		h.handler.logger.Error("failed to create SSE handler", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sse.Close()

	// Get ACL token from request
	token := GetTokenFromRequest(r)

	// Stream logs from Nomad API
	h.streamLogsFromNomad(sse, r, allocID, task, logType, follow, offset, origin, token)
}

// streamLogsFromNomad streams logs from the Nomad API via SSE.
func (h *AllocationLogsHandler) streamLogsFromNomad(sse *SSEHandler, r *http.Request, allocID, task, logType string, follow bool, offset int64, origin string, token string) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Error("no Nomad client available for log streaming")
		sse.WriteJSON("error", map[string]string{
			"message": "Nomad client not available",
		})
		return
	}

	// Build query options with ACL token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Fetch allocation info
	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for log streaming", "allocID", allocID, "error", err)
		sse.WriteJSON("error", map[string]string{
			"message": "Failed to fetch allocation: " + err.Error(),
		})
		return
	}

	// Create cancel channel from request context
	ctx := r.Context()
	cancel := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(cancel)
	}()

	// Stream logs from Nomad API
	// AllocFS().Logs returns two channels: frames and errors
	frames, errs := client.AllocFS().Logs(alloc, follow, task, logType, origin, offset, cancel, queryOpts)

	h.handler.logger.Debug("log stream connected to Nomad API",
		"allocID", allocID,
		"task", task,
		"type", logType,
		"follow", follow,
	)

	// Process frames and errors
	for {
		select {
		case <-ctx.Done():
			h.handler.logger.Debug("log stream client disconnected", "allocID", allocID)
			return

		case <-sse.Done():
			h.handler.logger.Debug("log stream SSE handler closed", "allocID", allocID)
			return

		case frame, ok := <-frames:
			if !ok {
				// Frames channel closed, stream ended
				h.handler.logger.Debug("log stream frames channel closed", "allocID", allocID)
				return
			}

			if frame == nil {
				continue
			}

			// Skip heartbeat frames (empty frames sent to keep connection alive)
			if frame.IsHeartbeat() {
				continue
			}

			// Decode frame data - Nomad sends log data as raw bytes
			// We need to handle potential binary data by base64 encoding it
			message := string(frame.Data)

			// Try to detect if data is binary (contains non-printable characters)
			// If binary, encode as base64
			isBinary := false
			for _, b := range frame.Data {
				if b < 32 && b != '\n' && b != '\r' && b != '\t' {
					isBinary = true
					break
				}
			}

			if isBinary {
				message = base64.StdEncoding.EncodeToString(frame.Data)
			}

			// Send log line to client
			// Frame.Offset contains the byte offset of this data
			// Frame.File contains the log file name
			line := LogLine{
				Timestamp: "", // Nomad doesn't provide timestamps in log frames
				Message:   message,
				Level:     h.detectLogLevel(message),
			}

			if err := sse.WriteJSON("log", line); err != nil {
				h.handler.logger.Debug("log stream write error", "error", err, "allocID", allocID)
				return
			}

		case err, ok := <-errs:
			if !ok {
				// Errors channel closed
				h.handler.logger.Debug("log stream errors channel closed", "allocID", allocID)
				return
			}

			if err != nil {
				h.handler.logger.Error("log stream error from Nomad", "allocID", allocID, "error", err)
				sse.WriteJSON("error", map[string]string{
					"message": err.Error(),
				})
				return
			}
		}
	}
}

// detectLogLevel attempts to detect log level from message content.
func (h *AllocationLogsHandler) detectLogLevel(msg string) string {
	msgLower := strings.ToLower(msg)
	if strings.Contains(msgLower, "[error]") || strings.Contains(msgLower, "error") {
		return "error"
	}
	if strings.Contains(msgLower, "[warn]") || strings.Contains(msgLower, "warning") {
		return "warn"
	}
	if strings.Contains(msgLower, "[debug]") {
		return "debug"
	}
	if strings.Contains(msgLower, "[info]") {
		return "info"
	}
	return ""
}

// getAllocationDetailForLogs fetches allocation details needed for the logs page.
// Returns a minimal allocation detail structure for template rendering.
func (h *AllocationLogsHandler) getAllocationDetailForLogs(allocID, token string) allocationspartials.AllocationDetail {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available for allocation detail")
		return allocationspartials.AllocationDetail{ID: allocID}
	}

	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for logs page", "allocID", allocID, "error", err)
		return allocationspartials.AllocationDetail{ID: allocID}
	}

	// Build task list
	tasks := make([]allocationspartials.TaskStatus, 0, len(alloc.TaskStates))
	for taskName, taskState := range alloc.TaskStates {
		tasks = append(tasks, allocationspartials.TaskStatus{
			Name:      taskName,
			State:     taskState.State,
			Failed:    taskState.Failed,
			StartedAt: taskState.StartedAt,
			Restarts:  int(taskState.Restarts),
		})
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name < tasks[j].Name
	})

	return allocationspartials.AllocationDetail{
		ID:        alloc.ID,
		JobID:     alloc.JobID,
		TaskGroup: alloc.TaskGroup,
		NodeID:    alloc.NodeID,
		NodeName:  alloc.NodeName,
		Status:    alloc.ClientStatus,
		Tasks:     tasks,
	}
}

// getTasksForAllocation returns the list of tasks for an allocation.
// Fetches from Nomad API and returns task names.
func (h *AllocationLogsHandler) getTasksForAllocation(allocID, token string) []string {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available for task list")
		return []string{}
	}

	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for task list", "allocID", allocID, "error", err)
		return []string{}
	}

	// Extract task names from TaskStates
	tasks := make([]string, 0, len(alloc.TaskStates))
	for taskName := range alloc.TaskStates {
		tasks = append(tasks, taskName)
	}

	// Sort for consistent display
	sort.Strings(tasks)

	return tasks
}
