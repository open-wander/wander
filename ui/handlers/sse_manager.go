package handlers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// SSEManager manages multiple SSE connections and provides broadcasting capabilities.
// It tracks active connections, handles topic-based subscriptions, and automatically
// cleans up dead connections.
//
// Example usage:
//
//	manager := NewSSEManager()
//	defer manager.Shutdown()
//
//	// In your HTTP handler:
//	func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
//	    allocID := r.URL.Query().Get("alloc_id")
//	    topic := fmt.Sprintf("logs:%s", allocID)
//
//	    sse, err := NewSSEHandler(w, r)
//	    if err != nil {
//	        http.Error(w, err.Error(), http.StatusInternalServerError)
//	        return
//	    }
//	    defer sse.Close()
//
//	    // Register connection with manager
//	    manager.Register(sse, topic)
//	    defer manager.Unregister(sse)
//
//	    // Wait for disconnection
//	    <-sse.Done()
//	}
//
//	// Broadcast to all subscribers of a topic:
//	manager.BroadcastToTopic("logs:alloc-123", "log", logData)
type SSEManager struct {
	mu          sync.RWMutex
	connections map[*SSEHandler]*connectionInfo
	topics      map[string]map[*SSEHandler]bool // topic -> connections
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// connectionInfo stores metadata about an SSE connection.
type connectionInfo struct {
	topics    map[string]bool
	createdAt time.Time
}

// SSEManagerConfig configures the SSE manager behavior.
type SSEManagerConfig struct {
	// CleanupInterval sets how often to check for dead connections.
	// If zero, cleanup is disabled. Recommended: 1-5 minutes.
	CleanupInterval time.Duration

	// EnableLogging enables debug logging for connection lifecycle.
	EnableLogging bool
}

// DefaultSSEManagerConfig returns sensible defaults for the SSE manager.
func DefaultSSEManagerConfig() *SSEManagerConfig {
	return &SSEManagerConfig{
		CleanupInterval: 2 * time.Minute,
		EnableLogging:   false,
	}
}

// NewSSEManager creates a new SSE connection manager with default configuration.
func NewSSEManager() *SSEManager {
	return NewSSEManagerWithConfig(DefaultSSEManagerConfig())
}

// NewSSEManagerWithConfig creates a new SSE connection manager with custom configuration.
func NewSSEManagerWithConfig(config *SSEManagerConfig) *SSEManager {
	if config == nil {
		config = DefaultSSEManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &SSEManager{
		connections: make(map[*SSEHandler]*connectionInfo),
		topics:      make(map[string]map[*SSEHandler]bool),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start cleanup goroutine if configured
	if config.CleanupInterval > 0 {
		m.wg.Add(1)
		go m.cleanupLoop(config.CleanupInterval, config.EnableLogging)
	}

	return m
}

// Register adds a new SSE connection to the manager.
// Topics can be provided to subscribe the connection to specific channels.
// If no topics are provided, the connection can still receive global broadcasts.
func (m *SSEManager) Register(handler *SSEHandler, topics ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := &connectionInfo{
		topics:    make(map[string]bool),
		createdAt: time.Now(),
	}

	// Store connection
	m.connections[handler] = info

	// Subscribe to topics
	for _, topic := range topics {
		info.topics[topic] = true

		// Add to topic map
		if m.topics[topic] == nil {
			m.topics[topic] = make(map[*SSEHandler]bool)
		}
		m.topics[topic][handler] = true
	}
}

// Unregister removes an SSE connection from the manager.
// This should be called when the connection is closed.
func (m *SSEManager) Unregister(handler *SSEHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unregisterLocked(handler)
}

// unregisterLocked removes a connection without acquiring the lock.
// Must be called with m.mu held.
func (m *SSEManager) unregisterLocked(handler *SSEHandler) {
	info, exists := m.connections[handler]
	if !exists {
		return
	}

	// Remove from all topics
	for topic := range info.topics {
		if conns, exists := m.topics[topic]; exists {
			delete(conns, handler)
			// Clean up empty topic maps
			if len(conns) == 0 {
				delete(m.topics, topic)
			}
		}
	}

	// Remove from connections
	delete(m.connections, handler)
}

// Subscribe adds a connection to one or more topics.
// The connection must already be registered with the manager.
func (m *SSEManager) Subscribe(handler *SSEHandler, topics ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.connections[handler]
	if !exists {
		return fmt.Errorf("handler not registered with manager")
	}

	for _, topic := range topics {
		// Add to connection's topic list
		info.topics[topic] = true

		// Add to topic map
		if m.topics[topic] == nil {
			m.topics[topic] = make(map[*SSEHandler]bool)
		}
		m.topics[topic][handler] = true
	}

	return nil
}

// Unsubscribe removes a connection from one or more topics.
func (m *SSEManager) Unsubscribe(handler *SSEHandler, topics ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.connections[handler]
	if !exists {
		return fmt.Errorf("handler not registered with manager")
	}

	for _, topic := range topics {
		// Remove from connection's topic list
		delete(info.topics, topic)

		// Remove from topic map
		if conns, exists := m.topics[topic]; exists {
			delete(conns, handler)
			// Clean up empty topic maps
			if len(conns) == 0 {
				delete(m.topics, topic)
			}
		}
	}

	return nil
}

// Broadcast sends an event to all registered connections.
// Dead connections are automatically removed.
func (m *SSEManager) Broadcast(event, data string) {
	m.mu.RLock()
	// Create a snapshot of connections
	handlers := make([]*SSEHandler, 0, len(m.connections))
	for handler := range m.connections {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Send to all connections outside the lock
	var deadHandlers []*SSEHandler
	for _, handler := range handlers {
		if err := handler.Write(event, data); err != nil {
			deadHandlers = append(deadHandlers, handler)
		}
	}

	// Clean up dead connections
	if len(deadHandlers) > 0 {
		m.mu.Lock()
		for _, handler := range deadHandlers {
			m.unregisterLocked(handler)
		}
		m.mu.Unlock()
	}
}

// BroadcastJSON sends a JSON event to all registered connections.
// Dead connections are automatically removed.
func (m *SSEManager) BroadcastJSON(event string, data interface{}) {
	m.mu.RLock()
	// Create a snapshot of connections
	handlers := make([]*SSEHandler, 0, len(m.connections))
	for handler := range m.connections {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Send to all connections outside the lock
	var deadHandlers []*SSEHandler
	for _, handler := range handlers {
		if err := handler.WriteJSON(event, data); err != nil {
			deadHandlers = append(deadHandlers, handler)
		}
	}

	// Clean up dead connections
	if len(deadHandlers) > 0 {
		m.mu.Lock()
		for _, handler := range deadHandlers {
			m.unregisterLocked(handler)
		}
		m.mu.Unlock()
	}
}

// BroadcastToTopic sends an event to all connections subscribed to a specific topic.
// Dead connections are automatically removed.
func (m *SSEManager) BroadcastToTopic(topic, event, data string) {
	m.mu.RLock()
	// Create a snapshot of topic subscribers
	topicConns := m.topics[topic]
	handlers := make([]*SSEHandler, 0, len(topicConns))
	for handler := range topicConns {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Send to all topic subscribers outside the lock
	var deadHandlers []*SSEHandler
	for _, handler := range handlers {
		if err := handler.Write(event, data); err != nil {
			deadHandlers = append(deadHandlers, handler)
		}
	}

	// Clean up dead connections
	if len(deadHandlers) > 0 {
		m.mu.Lock()
		for _, handler := range deadHandlers {
			m.unregisterLocked(handler)
		}
		m.mu.Unlock()
	}
}

// BroadcastJSONToTopic sends a JSON event to all connections subscribed to a specific topic.
// Dead connections are automatically removed.
func (m *SSEManager) BroadcastJSONToTopic(topic, event string, data interface{}) {
	m.mu.RLock()
	// Create a snapshot of topic subscribers
	topicConns := m.topics[topic]
	handlers := make([]*SSEHandler, 0, len(topicConns))
	for handler := range topicConns {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Send to all topic subscribers outside the lock
	var deadHandlers []*SSEHandler
	for _, handler := range handlers {
		if err := handler.WriteJSON(event, data); err != nil {
			deadHandlers = append(deadHandlers, handler)
		}
	}

	// Clean up dead connections
	if len(deadHandlers) > 0 {
		m.mu.Lock()
		for _, handler := range deadHandlers {
			m.unregisterLocked(handler)
		}
		m.mu.Unlock()
	}
}

// ConnectionCount returns the total number of active connections.
func (m *SSEManager) ConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// TopicCount returns the number of active topics.
func (m *SSEManager) TopicCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.topics)
}

// TopicSubscribers returns the number of connections subscribed to a topic.
func (m *SSEManager) TopicSubscribers(topic string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.topics[topic])
}

// Topics returns a list of all active topics.
func (m *SSEManager) Topics() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topics := make([]string, 0, len(m.topics))
	for topic := range m.topics {
		topics = append(topics, topic)
	}
	return topics
}

// cleanupLoop periodically checks for and removes dead connections.
func (m *SSEManager) cleanupLoop(interval time.Duration, enableLogging bool) {
	defer m.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup(enableLogging)
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanup removes dead connections by attempting to send a comment.
func (m *SSEManager) cleanup(enableLogging bool) {
	m.mu.RLock()
	// Create a snapshot of connections
	handlers := make([]*SSEHandler, 0, len(m.connections))
	for handler := range m.connections {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Test each connection
	var deadHandlers []*SSEHandler
	for _, handler := range handlers {
		// Try to write a comment to test if connection is alive
		if err := handler.WriteComment("ping"); err != nil {
			deadHandlers = append(deadHandlers, handler)
		}
	}

	// Remove dead connections
	if len(deadHandlers) > 0 {
		if enableLogging {
			log.Printf("SSEManager: Cleaning up %d dead connections", len(deadHandlers))
		}

		m.mu.Lock()
		for _, handler := range deadHandlers {
			m.unregisterLocked(handler)
			handler.Close()
		}
		m.mu.Unlock()
	}
}

// Shutdown gracefully shuts down the manager and closes all connections.
func (m *SSEManager) Shutdown() {
	// Stop cleanup loop
	m.cancel()
	m.wg.Wait()

	// Close all connections
	m.mu.Lock()
	defer m.mu.Unlock()

	for handler := range m.connections {
		handler.Close()
	}

	// Clear all data
	m.connections = make(map[*SSEHandler]*connectionInfo)
	m.topics = make(map[string]map[*SSEHandler]bool)
}
