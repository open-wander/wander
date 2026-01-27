package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockResponseWriter implements http.ResponseWriter and http.Flusher for testing
type mockResponseWriter struct {
	headers http.Header
	body    []byte
	mu      sync.Mutex
	flushed int
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		headers: make(http.Header),
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	// Not needed for SSE tests
}

func (m *mockResponseWriter) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushed++
}

func (m *mockResponseWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return string(m.body)
}

func (m *mockResponseWriter) FlushCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushed
}

func TestSSEManager_Register(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Test basic registration
	manager.Register(sse)

	if count := manager.ConnectionCount(); count != 1 {
		t.Errorf("Expected 1 connection, got %d", count)
	}

	// Test unregistration
	manager.Unregister(sse)

	if count := manager.ConnectionCount(); count != 0 {
		t.Errorf("Expected 0 connections after unregister, got %d", count)
	}
}

func TestSSEManager_RegisterWithTopics(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Register with topics
	manager.Register(sse, "topic1", "topic2")

	if count := manager.TopicCount(); count != 2 {
		t.Errorf("Expected 2 topics, got %d", count)
	}

	if count := manager.TopicSubscribers("topic1"); count != 1 {
		t.Errorf("Expected 1 subscriber to topic1, got %d", count)
	}

	if count := manager.TopicSubscribers("topic2"); count != 1 {
		t.Errorf("Expected 1 subscriber to topic2, got %d", count)
	}

	// Verify topics list
	topics := manager.Topics()
	topicMap := make(map[string]bool)
	for _, topic := range topics {
		topicMap[topic] = true
	}

	if !topicMap["topic1"] || !topicMap["topic2"] {
		t.Errorf("Expected topics [topic1, topic2], got %v", topics)
	}
}

func TestSSEManager_Subscribe(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Register without topics
	manager.Register(sse)

	if count := manager.TopicCount(); count != 0 {
		t.Errorf("Expected 0 topics initially, got %d", count)
	}

	// Subscribe to topics
	err = manager.Subscribe(sse, "logs", "metrics")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	if count := manager.TopicCount(); count != 2 {
		t.Errorf("Expected 2 topics after subscribe, got %d", count)
	}

	if count := manager.TopicSubscribers("logs"); count != 1 {
		t.Errorf("Expected 1 subscriber to logs, got %d", count)
	}
}

func TestSSEManager_Unsubscribe(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Register with topics
	manager.Register(sse, "logs", "metrics")

	// Unsubscribe from one topic
	err = manager.Unsubscribe(sse, "logs")
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	if count := manager.TopicSubscribers("logs"); count != 0 {
		t.Errorf("Expected 0 subscribers to logs after unsubscribe, got %d", count)
	}

	if count := manager.TopicSubscribers("metrics"); count != 1 {
		t.Errorf("Expected 1 subscriber to metrics, got %d", count)
	}

	// Topic should be removed from topic map
	if count := manager.TopicCount(); count != 1 {
		t.Errorf("Expected 1 topic after unsubscribe, got %d", count)
	}
}

func TestSSEManager_Broadcast(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	// Create multiple connections
	var handlers []*SSEHandler
	var writers []*mockResponseWriter

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/stream", nil)
		w := newMockResponseWriter()

		sse, err := NewSSEHandler(w, req)
		if err != nil {
			t.Fatalf("Failed to create SSE handler %d: %v", i, err)
		}
		defer sse.Close()

		manager.Register(sse)
		handlers = append(handlers, sse)
		writers = append(writers, w)
	}

	// Broadcast to all connections
	manager.Broadcast("test", "hello world")

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// Verify all connections received the message
	for i, w := range writers {
		output := w.String()
		if output == "" {
			t.Errorf("Connection %d received no data", i)
			continue
		}

		// Check for event and data
		if !contains(output, "event: test") {
			t.Errorf("Connection %d missing event name, got: %s", i, output)
		}
		if !contains(output, "data: hello world") {
			t.Errorf("Connection %d missing data, got: %s", i, output)
		}
	}
}

func TestSSEManager_BroadcastJSON(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	manager.Register(sse)

	// Broadcast JSON data
	data := map[string]string{"message": "test"}
	manager.BroadcastJSON("event", data)

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	output := w.String()
	if !contains(output, "event: event") {
		t.Errorf("Missing event name in output: %s", output)
	}
	if !contains(output, `"message":"test"`) {
		t.Errorf("Missing JSON data in output: %s", output)
	}
}

func TestSSEManager_BroadcastToTopic(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	// Create connections with different topics
	req1 := httptest.NewRequest("GET", "/stream", nil)
	w1 := newMockResponseWriter()
	sse1, _ := NewSSEHandler(w1, req1)
	defer sse1.Close()
	manager.Register(sse1, "logs")

	req2 := httptest.NewRequest("GET", "/stream", nil)
	w2 := newMockResponseWriter()
	sse2, _ := NewSSEHandler(w2, req2)
	defer sse2.Close()
	manager.Register(sse2, "metrics")

	req3 := httptest.NewRequest("GET", "/stream", nil)
	w3 := newMockResponseWriter()
	sse3, _ := NewSSEHandler(w3, req3)
	defer sse3.Close()
	manager.Register(sse3, "logs", "metrics")

	// Broadcast to logs topic only
	manager.BroadcastToTopic("logs", "log", "test message")

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// sse1 should receive (subscribed to logs)
	if !contains(w1.String(), "test message") {
		t.Errorf("sse1 (logs subscriber) did not receive message")
	}

	// sse2 should not receive (subscribed to metrics only)
	if contains(w2.String(), "test message") {
		t.Errorf("sse2 (metrics subscriber) incorrectly received message")
	}

	// sse3 should receive (subscribed to both)
	if !contains(w3.String(), "test message") {
		t.Errorf("sse3 (logs+metrics subscriber) did not receive message")
	}
}

func TestSSEManager_BroadcastJSONToTopic(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	manager.Register(sse, "logs")

	// Broadcast JSON to topic
	data := map[string]interface{}{
		"level":   "info",
		"message": "test log",
	}
	manager.BroadcastJSONToTopic("logs", "log", data)

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	output := w.String()
	if !contains(output, `"level":"info"`) {
		t.Errorf("Missing level in output: %s", output)
	}
	if !contains(output, `"message":"test log"`) {
		t.Errorf("Missing message in output: %s", output)
	}
}

func TestSSEManager_ConcurrentAccess(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	var wg sync.WaitGroup
	connectionCount := 10

	// Register connections concurrently
	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/stream", nil)
			w := newMockResponseWriter()

			sse, err := NewSSEHandler(w, req)
			if err != nil {
				t.Errorf("Failed to create SSE handler %d: %v", id, err)
				return
			}
			defer sse.Close()

			manager.Register(sse, "topic1", "topic2")

			// Simulate some activity
			time.Sleep(10 * time.Millisecond)

			manager.Unregister(sse)
		}(i)
	}

	// Broadcast concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				manager.Broadcast("test", "concurrent message")
				manager.BroadcastToTopic("topic1", "test", "topic message")
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify clean state
	if count := manager.ConnectionCount(); count != 0 {
		t.Errorf("Expected 0 connections after concurrent test, got %d", count)
	}

	if count := manager.TopicCount(); count != 0 {
		t.Errorf("Expected 0 topics after concurrent test, got %d", count)
	}
}

func TestSSEManager_DeadConnectionCleanup(t *testing.T) {
	// Use a short cleanup interval for testing
	config := &SSEManagerConfig{
		CleanupInterval: 100 * time.Millisecond,
		EnableLogging:   false,
	}
	manager := NewSSEManagerWithConfig(config)
	defer manager.Shutdown()

	// Create a connection and close it to make it "dead"
	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}

	manager.Register(sse)

	if count := manager.ConnectionCount(); count != 1 {
		t.Errorf("Expected 1 connection, got %d", count)
	}

	// Close the handler to make it dead
	sse.Close()

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Connection should be removed by cleanup
	if count := manager.ConnectionCount(); count != 0 {
		t.Errorf("Expected 0 connections after cleanup, got %d", count)
	}
}

func TestSSEManager_Shutdown(t *testing.T) {
	manager := NewSSEManager()

	// Create some connections
	var handlers []*SSEHandler
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/stream", nil)
		w := newMockResponseWriter()

		sse, err := NewSSEHandler(w, req)
		if err != nil {
			t.Fatalf("Failed to create SSE handler %d: %v", i, err)
		}

		manager.Register(sse, "topic1")
		handlers = append(handlers, sse)
	}

	if count := manager.ConnectionCount(); count != 3 {
		t.Errorf("Expected 3 connections, got %d", count)
	}

	// Shutdown should clean up everything
	manager.Shutdown()

	if count := manager.ConnectionCount(); count != 0 {
		t.Errorf("Expected 0 connections after shutdown, got %d", count)
	}

	if count := manager.TopicCount(); count != 0 {
		t.Errorf("Expected 0 topics after shutdown, got %d", count)
	}

	// Verify all handlers were closed
	for i, sse := range handlers {
		select {
		case <-sse.Done():
			// Good, handler is closed
		default:
			t.Errorf("Handler %d was not closed during shutdown", i)
		}
	}
}

func TestSSEManager_SubscribeUnregistered(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Try to subscribe without registering first
	err = manager.Subscribe(sse, "topic1")
	if err == nil {
		t.Error("Expected error when subscribing unregistered handler, got nil")
	}
}

func TestSSEManager_UnsubscribeUnregistered(t *testing.T) {
	manager := NewSSEManager()
	defer manager.Shutdown()

	req := httptest.NewRequest("GET", "/stream", nil)
	w := newMockResponseWriter()

	sse, err := NewSSEHandler(w, req)
	if err != nil {
		t.Fatalf("Failed to create SSE handler: %v", err)
	}
	defer sse.Close()

	// Try to unsubscribe without registering first
	err = manager.Unsubscribe(sse, "topic1")
	if err == nil {
		t.Error("Expected error when unsubscribing unregistered handler, got nil")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
