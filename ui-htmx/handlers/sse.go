package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SSEHandler manages Server-Sent Events (SSE) streaming to a client.
// It handles the low-level details of SSE protocol including proper headers,
// event formatting, flushing, and context cancellation.
//
// Example usage:
//
//	func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
//	    sse, err := NewSSEHandler(w, r)
//	    if err != nil {
//	        http.Error(w, err.Error(), http.StatusInternalServerError)
//	        return
//	    }
//	    defer sse.Close()
//
//	    // Stream log events
//	    for {
//	        select {
//	        case <-r.Context().Done():
//	            return
//	        case log := <-logChannel:
//	            if err := sse.WriteJSON("log", log); err != nil {
//	                return
//	            }
//	        }
//	    }
//	}
type SSEHandler struct {
	w       http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool

	// Heartbeat configuration
	heartbeatInterval time.Duration
	heartbeatStop     chan struct{}
	heartbeatDone     chan struct{}
}

// SSEOptions configures the SSE handler behavior.
type SSEOptions struct {
	// HeartbeatInterval sets the interval for sending heartbeat comments.
	// If zero, heartbeats are disabled. Recommended: 15-30 seconds.
	HeartbeatInterval time.Duration

	// RetryInterval tells the client how long to wait before reconnecting
	// after a connection drop. If zero, client default is used.
	RetryInterval time.Duration
}

// DefaultSSEOptions returns sensible defaults for SSE streaming.
func DefaultSSEOptions() *SSEOptions {
	return &SSEOptions{
		HeartbeatInterval: 30 * time.Second,
		RetryInterval:     3 * time.Second,
	}
}

// NewSSEHandler creates a new SSE handler for the given response writer.
// It sets the required SSE headers and verifies the writer supports flushing.
//
// Returns an error if the response writer doesn't support flushing, which is
// required for SSE to work properly.
func NewSSEHandler(w http.ResponseWriter, r *http.Request) (*SSEHandler, error) {
	return NewSSEHandlerWithOptions(w, r, DefaultSSEOptions())
}

// NewSSEHandlerWithOptions creates a new SSE handler with custom options.
func NewSSEHandlerWithOptions(w http.ResponseWriter, r *http.Request, opts *SSEOptions) (*SSEHandler, error) {
	if opts == nil {
		opts = DefaultSSEOptions()
	}

	// Verify the response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	// Set SSE headers
	setSSEHeaders(w, opts)

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(r.Context())

	h := &SSEHandler{
		w:       w,
		flusher: flusher,
		ctx:     ctx,
		cancel:  cancel,
		closed:  false,
	}

	// Send initial retry configuration if specified
	if opts.RetryInterval > 0 {
		if err := h.writeRetry(int(opts.RetryInterval.Milliseconds())); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to write retry interval: %w", err)
		}
	}

	// Start heartbeat if configured
	if opts.HeartbeatInterval > 0 {
		h.startHeartbeat(opts.HeartbeatInterval)
	}

	return h, nil
}

// setSSEHeaders sets the required HTTP headers for SSE streaming.
func setSSEHeaders(w http.ResponseWriter, opts *SSEOptions) {
	// Required SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Disable buffering in nginx and similar proxies
	w.Header().Set("X-Accel-Buffering", "no")
}

// Write sends an SSE event with the given event name and data.
// The data will be sent as-is, so ensure it doesn't contain newlines
// unless you want to send multi-line data (which is valid SSE).
//
// For structured data, use WriteJSON instead.
func (h *SSEHandler) Write(event, data string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("SSE handler is closed")
	}

	// Format: event: name\ndata: content\n\n
	if event != "" {
		if _, err := fmt.Fprintf(h.w, "event: %s\n", event); err != nil {
			return fmt.Errorf("failed to write event name: %w", err)
		}
	}

	if _, err := fmt.Fprintf(h.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	h.flusher.Flush()
	return nil
}

// WriteJSON sends an SSE event with JSON-encoded data.
// This is a convenience method that marshals the data to JSON before sending.
func (h *SSEHandler) WriteJSON(event string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return h.Write(event, string(jsonData))
}

// WriteComment sends a comment line. Comments are ignored by the client
// but can be used for debugging or as heartbeats to keep the connection alive.
func (h *SSEHandler) WriteComment(comment string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("SSE handler is closed")
	}

	if _, err := fmt.Fprintf(h.w, ": %s\n\n", comment); err != nil {
		return fmt.Errorf("failed to write comment: %w", err)
	}

	h.flusher.Flush()
	return nil
}

// writeRetry sends a retry field to tell the client how long to wait
// before reconnecting after a connection drop (in milliseconds).
func (h *SSEHandler) writeRetry(milliseconds int) error {
	if _, err := fmt.Fprintf(h.w, "retry: %d\n\n", milliseconds); err != nil {
		return fmt.Errorf("failed to write retry: %w", err)
	}

	h.flusher.Flush()
	return nil
}

// Flush ensures any buffered data is sent immediately to the client.
// This is called automatically after each Write, but can be called
// manually if needed.
func (h *SSEHandler) Flush() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.closed {
		h.flusher.Flush()
	}
}

// Close ends the SSE stream and releases resources.
// It's safe to call Close multiple times.
func (h *SSEHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true
	h.cancel()

	// Stop heartbeat if running
	h.stopHeartbeat()

	return nil
}

// Done returns a channel that's closed when the SSE handler is closed
// or the context is cancelled. This can be used in select statements.
func (h *SSEHandler) Done() <-chan struct{} {
	return h.ctx.Done()
}

// Err returns the error that caused the context to be cancelled.
func (h *SSEHandler) Err() error {
	return h.ctx.Err()
}

// startHeartbeat starts sending periodic heartbeat comments to keep
// the connection alive and detect client disconnections.
func (h *SSEHandler) startHeartbeat(interval time.Duration) {
	h.heartbeatStop = make(chan struct{})
	h.heartbeatDone = make(chan struct{})

	go func() {
		defer close(h.heartbeatDone)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send heartbeat comment
				if err := h.WriteComment("heartbeat"); err != nil {
					// Connection likely closed, stop heartbeat
					return
				}
			case <-h.heartbeatStop:
				return
			case <-h.ctx.Done():
				return
			}
		}
	}()
}

// stopHeartbeat stops the heartbeat goroutine if it's running.
func (h *SSEHandler) stopHeartbeat() {
	if h.heartbeatStop != nil {
		close(h.heartbeatStop)
		<-h.heartbeatDone // Wait for goroutine to finish
	}
}

// SSEWriter is a simple io.Writer implementation that sends each Write
// as an SSE event. This is useful for streaming command output or logs.
type SSEWriter struct {
	handler   *SSEHandler
	eventName string
}

// NewSSEWriter creates an io.Writer that sends data as SSE events.
// Each Write call will send one SSE event with the given event name.
func NewSSEWriter(handler *SSEHandler, eventName string) io.Writer {
	return &SSEWriter{
		handler:   handler,
		eventName: eventName,
	}
}

// Write implements io.Writer by sending the data as an SSE event.
func (w *SSEWriter) Write(p []byte) (n int, err error) {
	if err := w.handler.Write(w.eventName, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
