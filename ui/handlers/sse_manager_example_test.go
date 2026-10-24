package handlers_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/open-wander/wander/ui/handlers"
)

// Example demonstrates how to use SSEManager for connection tracking and broadcasting.
func Example_sseManager() {
	// Create a global SSE manager
	sseManager := handlers.NewSSEManager()
	defer sseManager.Shutdown()

	// Example HTTP handler for streaming logs
	streamLogsHandler := func(w http.ResponseWriter, r *http.Request) {
		allocID := r.URL.Query().Get("alloc_id")
		topic := fmt.Sprintf("logs:%s", allocID)

		// Create SSE handler
		sse, err := handlers.NewSSEHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		// Register with manager and subscribe to topic
		sseManager.Register(sse, topic)
		defer sseManager.Unregister(sse)

		// Wait for disconnection
		<-sse.Done()
	}

	// Example handler for broadcasting log events
	broadcastLogHandler := func(w http.ResponseWriter, r *http.Request) {
		allocID := r.URL.Query().Get("alloc_id")
		message := r.URL.Query().Get("message")

		topic := fmt.Sprintf("logs:%s", allocID)

		// Broadcast to all subscribers of this topic
		sseManager.BroadcastToTopic(topic, "log", message)

		w.WriteHeader(http.StatusOK)
	}

	// Register handlers
	http.HandleFunc("/stream/logs", streamLogsHandler)
	http.HandleFunc("/broadcast/log", broadcastLogHandler)

	fmt.Println("SSE Manager example setup complete")
	// Output: SSE Manager example setup complete
}

// Example demonstrates topic-based subscriptions.
func Example_sseManagerTopics() {
	sseManager := handlers.NewSSEManager()
	defer sseManager.Shutdown()

	// Handler that streams multiple topics
	streamHandler := func(w http.ResponseWriter, r *http.Request) {
		allocID := r.URL.Query().Get("alloc_id")

		sse, err := handlers.NewSSEHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer sse.Close()

		// Subscribe to multiple topics for this allocation
		topics := []string{
			fmt.Sprintf("logs:%s", allocID),
			fmt.Sprintf("metrics:%s", allocID),
			fmt.Sprintf("events:%s", allocID),
		}

		sseManager.Register(sse, topics...)
		defer sseManager.Unregister(sse)

		<-sse.Done()
	}

	// Later, dynamically add more topics
	_ = func(w http.ResponseWriter, r *http.Request, sse *handlers.SSEHandler) {
		// Subscribe to additional topics
		err := sseManager.Subscribe(sse, "system:alerts", "system:notifications")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}

	http.HandleFunc("/stream/multi", streamHandler)

	fmt.Println("Multi-topic example setup complete")
	// Output: Multi-topic example setup complete
}

// Example demonstrates monitoring active connections.
func Example_sseManagerMonitoring() {
	sseManager := handlers.NewSSEManager()
	defer sseManager.Shutdown()

	// Monitoring handler
	monitorHandler := func(w http.ResponseWriter, r *http.Request) {
		stats := map[string]interface{}{
			"total_connections": sseManager.ConnectionCount(),
			"total_topics":      sseManager.TopicCount(),
			"topics":            make(map[string]int),
		}

		// Get subscriber count for each topic
		for _, topic := range sseManager.Topics() {
			stats["topics"].(map[string]int)[topic] = sseManager.TopicSubscribers(topic)
		}

		fmt.Printf("Connections: %d\n", stats["total_connections"])
		fmt.Printf("Topics: %d\n", stats["total_topics"])
	}

	http.HandleFunc("/monitor/sse", monitorHandler)

	// Simulate monitoring
	fmt.Println("SSE monitoring example setup complete")
	// Output: SSE monitoring example setup complete
}

// Example demonstrates real-time log streaming with SSEManager.
func Example_sseManagerLogStreaming() {
	sseManager := handlers.NewSSEManager()
	defer sseManager.Shutdown()

	// Simulate a log event generator
	generateLogs := func(allocID string) {
		topic := fmt.Sprintf("logs:%s", allocID)

		for i := 0; i < 5; i++ {
			logEntry := map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "info",
				"message":   fmt.Sprintf("Log message %d", i),
			}

			// Broadcast to all subscribers of this topic
			sseManager.BroadcastJSONToTopic(topic, "log", logEntry)

			time.Sleep(100 * time.Millisecond)
		}
	}

	// Start log generation in background
	go generateLogs("alloc-123")

	fmt.Println("Log streaming example started")
	// Output: Log streaming example started
}
