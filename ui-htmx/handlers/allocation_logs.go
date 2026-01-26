package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

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

// LogsTab renders the logs tab partial.
func (h *AllocationLogsHandler) LogsTab(w http.ResponseWriter, r *http.Request, allocID string) {
	h.handler.logger.Debug("logs tab request", "allocID", allocID)

	// Get task from query params or use first task
	task := r.URL.Query().Get("task")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "stdout"
	}

	// Get allocation details to get task list
	// In a real implementation, this would query Nomad API
	tasks := h.getTasksForAllocation(allocID)

	if task == "" && len(tasks) > 0 {
		task = tasks[0]
	}

	props := allocationspartials.LogsTabProps{
		AllocID:      allocID,
		Tasks:        tasks,
		SelectedTask: task,
		LogType:      logType,
	}

	err := RenderPartial(w, r, allocationspartials.LogsTabContent(props))
	if err != nil {
		h.handler.logger.Error("failed to render logs tab", "error", err)
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

	h.handler.logger.Debug("starting log stream", "allocID", allocID, "task", task, "type", logType)

	// Create SSE handler
	sse, err := NewSSEHandler(w, r)
	if err != nil {
		h.handler.logger.Error("failed to create SSE handler", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sse.Close()

	// In a real implementation, this would:
	// 1. Connect to Nomad's log streaming API
	// 2. Forward log frames to the SSE connection
	// For now, we simulate log output

	h.simulateLogStream(sse, r, allocID, task, logType)
}

// simulateLogStream generates mock log data for demonstration.
// In production, this would be replaced with actual Nomad API calls.
func (h *AllocationLogsHandler) simulateLogStream(sse *SSEHandler, r *http.Request, allocID, task, logType string) {
	// Sample log messages for simulation
	stdoutMessages := []string{
		"Starting application...",
		"Configuration loaded successfully",
		"Connecting to database...",
		"Database connection established",
		"Listening on port 8080",
		"Request received: GET /health",
		"Response sent: 200 OK (12ms)",
		"Request received: GET /api/users",
		"Response sent: 200 OK (45ms)",
		"Cache hit for key: user:123",
		"Request received: POST /api/orders",
		"Order created: #12345",
		"Response sent: 201 Created (156ms)",
		"Garbage collection completed in 23ms",
		"Health check passed",
		"Metrics exported to /metrics",
		"Connection pool: 10 active, 5 idle",
		"Memory usage: 256MB / 512MB",
		"CPU usage: 35%",
		"Processing batch job...",
		"Batch job completed: 1000 items processed",
	}

	stderrMessages := []string{
		"[WARN] High memory usage detected",
		"[ERROR] Connection timeout to external service",
		"[WARN] Retry attempt 1/3 for database query",
		"[ERROR] Failed to parse JSON payload",
		"[WARN] Rate limit approaching for API key",
		"[ERROR] Permission denied for file /var/log/app.log",
		"[WARN] Deprecated API endpoint called",
		"[DEBUG] Stack trace for debugging",
		"[ERROR] Unhandled exception in worker thread",
		"[WARN] SSL certificate expires in 7 days",
	}

	// Choose message set based on log type
	var messages []string
	switch logType {
	case "stderr":
		messages = stderrMessages
	case "both":
		// Combine both stdout and stderr messages for "both" mode
		messages = append(messages, stdoutMessages...)
		messages = append(messages, stderrMessages...)
	default: // stdout
		messages = stdoutMessages
	}

	// Send initial historical logs (simulating log tail)
	for i := 0; i < 20; i++ {
		msg := messages[rand.Intn(len(messages))]
		timestamp := time.Now().Add(time.Duration(-20+i) * time.Second)

		line := LogLine{
			Timestamp: timestamp.Format("2006-01-02 15:04:05"),
			Message:   msg,
			Level:     h.detectLogLevel(msg),
		}

		if err := sse.WriteJSON("log", line); err != nil {
			return
		}
	}

	// Stream new logs periodically
	ticker := time.NewTicker(time.Duration(500+rand.Intn(2000)) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			h.handler.logger.Debug("log stream client disconnected", "allocID", allocID)
			return
		case <-sse.Done():
			h.handler.logger.Debug("log stream SSE handler closed", "allocID", allocID)
			return
		case <-ticker.C:
			msg := messages[rand.Intn(len(messages))]

			line := LogLine{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Message:   fmt.Sprintf("[%s/%s] %s", task, logType, msg),
				Level:     h.detectLogLevel(msg),
			}

			data, _ := json.Marshal(line)
			if err := sse.Write("log", string(data)); err != nil {
				h.handler.logger.Debug("log stream write error", "error", err)
				return
			}

			// Vary the interval for realistic feel
			ticker.Reset(time.Duration(500+rand.Intn(2000)) * time.Millisecond)
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

// getTasksForAllocation returns the list of tasks for an allocation.
// In production, this would query the Nomad API.
func (h *AllocationLogsHandler) getTasksForAllocation(allocID string) []string {
	// Mock task list - in production, query Nomad API
	return []string{"nginx", "redis", "sidecar"}
}
