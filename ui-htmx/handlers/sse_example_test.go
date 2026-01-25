package handlers_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/open-wander/wander/ui-htmx/handlers"
)

// ExampleSSEHandler demonstrates basic SSE streaming usage.
func ExampleSSEHandler() {
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		// Create SSE handler with default options (30s heartbeat, 3s retry)
		sse, err := handlers.NewSSEHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		// Send initial message
		if err := sse.Write("connected", "Stream started"); err != nil {
			return
		}

		// Stream messages until client disconnects
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-sse.Done():
				// Client disconnected or context cancelled
				return
			case <-ticker.C:
				count++
				msg := fmt.Sprintf("Message %d", count)
				if err := sse.Write("message", msg); err != nil {
					return
				}
				if count >= 5 {
					return
				}
			}
		}
	})
}

// ExampleSSEHandler_json demonstrates streaming JSON data.
func ExampleSSEHandler_json() {
	type LogEntry struct {
		Level     string `json:"level"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		sse, err := handlers.NewSSEHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		logs := []LogEntry{
			{Level: "info", Message: "Server started", Timestamp: time.Now().Format(time.RFC3339)},
			{Level: "warn", Message: "High memory usage", Timestamp: time.Now().Format(time.RFC3339)},
			{Level: "error", Message: "Connection failed", Timestamp: time.Now().Format(time.RFC3339)},
		}

		for _, log := range logs {
			if err := sse.WriteJSON("log", log); err != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// ExampleSSEHandler_withOptions demonstrates custom SSE options.
func ExampleSSEHandler_withOptions() {
	http.HandleFunc("/stream-custom", func(w http.ResponseWriter, r *http.Request) {
		// Custom options: faster heartbeat, longer retry
		opts := &handlers.SSEOptions{
			HeartbeatInterval: 10 * time.Second,
			RetryInterval:     5 * time.Second,
		}

		sse, err := handlers.NewSSEHandlerWithOptions(w, r, opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		// Send data...
		sse.Write("start", "Custom configuration active")
	})
}

// ExampleNewSSEWriter demonstrates using SSE as an io.Writer.
func ExampleNewSSEWriter() {
	http.HandleFunc("/command-output", func(w http.ResponseWriter, r *http.Request) {
		sse, err := handlers.NewSSEHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		// Create an io.Writer that sends output as SSE events
		writer := handlers.NewSSEWriter(sse, "output")

		// Any writes to this writer will be sent as SSE events
		fmt.Fprintln(writer, "Starting command...")
		fmt.Fprintln(writer, "Processing...")
		fmt.Fprintln(writer, "Complete!")
	})
}
