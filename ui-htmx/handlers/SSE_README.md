# SSE Handler

Server-Sent Events (SSE) utility for real-time streaming in the Wander UI.

## Overview

The `SSEHandler` provides a robust, production-ready implementation of Server-Sent Events for streaming real-time updates to clients. It handles all the low-level details of the SSE protocol including proper headers, event formatting, flushing, heartbeats, and graceful shutdown.

## Features

- **Proper SSE Protocol**: Automatically sets required headers and formats events correctly
- **Heartbeat Support**: Configurable keep-alive mechanism to detect disconnections
- **Context Cancellation**: Integrates with Go's context for clean shutdown
- **Thread-Safe**: Safe for concurrent use with internal mutex protection
- **Flexible API**: Support for text, JSON, and io.Writer interfaces
- **Automatic Flushing**: Ensures immediate delivery of events to clients

## Quick Start

### Basic Streaming

```go
func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
    // Create SSE handler
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Send events
    sse.Write("message", "Hello from server")

    // Stream until client disconnects
    for {
        select {
        case <-sse.Done():
            return
        case log := <-logChannel:
            sse.Write("log", log)
        }
    }
}
```

### Streaming JSON

```go
type LogEntry struct {
    Level     string `json:"level"`
    Message   string `json:"message"`
    Timestamp string `json:"timestamp"`
}

func (h *Handler) StreamJSONLogs(w http.ResponseWriter, r *http.Request) {
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    log := LogEntry{
        Level:     "info",
        Message:   "Server started",
        Timestamp: time.Now().Format(time.RFC3339),
    }

    // Automatically marshals to JSON
    sse.WriteJSON("log", log)
}
```

### Custom Options

```go
func (h *Handler) StreamWithOptions(w http.ResponseWriter, r *http.Request) {
    opts := &handlers.SSEOptions{
        HeartbeatInterval: 15 * time.Second,  // Faster heartbeat
        RetryInterval:     5 * time.Second,   // Longer retry wait
    }

    sse, err := handlers.NewSSEHandlerWithOptions(w, r, opts)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Use SSE handler...
}
```

### As io.Writer

```go
func (h *Handler) StreamCommandOutput(w http.ResponseWriter, r *http.Request) {
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Create io.Writer that sends output as SSE events
    writer := handlers.NewSSEWriter(sse, "output")

    // Any standard io.Writer operations work
    cmd := exec.Command("ls", "-la")
    cmd.Stdout = writer
    cmd.Stderr = writer
    cmd.Run()
}
```

## Client-Side (HTMX)

### Basic EventSource

```html
<!-- Simple message display -->
<div id="messages"></div>

<script>
    const eventSource = new EventSource('/stream');

    eventSource.addEventListener('message', (e) => {
        const div = document.getElementById('messages');
        div.innerHTML += `<p>${e.data}</p>`;
    });

    eventSource.addEventListener('log', (e) => {
        const data = JSON.parse(e.data);
        console.log(`[${data.level}] ${data.message}`);
    });

    eventSource.onerror = (e) => {
        console.error('SSE error:', e);
        eventSource.close();
    };
</script>
```

### HTMX Integration

```html
<!-- Real-time log viewer -->
<div hx-ext="sse" sse-connect="/logs">
    <div sse-swap="log" hx-swap="beforeend">
        <!-- Log entries will be inserted here -->
    </div>
</div>
```

## API Reference

### Types

#### `SSEHandler`

Main handler for SSE streaming.

**Methods:**

- `Write(event, data string) error` - Send text event
- `WriteJSON(event string, data interface{}) error` - Send JSON event
- `WriteComment(comment string) error` - Send comment (for debugging/heartbeat)
- `Flush()` - Ensure buffered data is sent
- `Close() error` - End stream and cleanup resources
- `Done() <-chan struct{}` - Channel closed when handler is done
- `Err() error` - Get cancellation error

#### `SSEOptions`

Configuration for SSE handler.

**Fields:**

- `HeartbeatInterval time.Duration` - Interval for keep-alive heartbeats (0 = disabled)
- `RetryInterval time.Duration` - Client reconnection delay (0 = client default)

### Functions

#### `NewSSEHandler(w http.ResponseWriter, r *http.Request) (*SSEHandler, error)`

Create handler with default options:
- 30-second heartbeat interval
- 3-second retry interval

#### `NewSSEHandlerWithOptions(w, r, opts) (*SSEHandler, error)`

Create handler with custom options.

#### `DefaultSSEOptions() *SSEOptions`

Get default configuration.

#### `NewSSEWriter(handler *SSEHandler, eventName string) io.Writer`

Create io.Writer that sends data as SSE events.

## Event Format

SSE events follow this format:

```
event: eventName
data: content
<blank line>
```

For unnamed events:

```
data: content
<blank line>
```

Comments (heartbeats):

```
: heartbeat
<blank line>
```

## Best Practices

### 1. Always Close

Always defer `Close()` to ensure cleanup:

```go
sse, err := handlers.NewSSEHandler(w, r)
if err != nil {
    return err
}
defer sse.Close()  // Critical!
```

### 2. Check Done Channel

Use the `Done()` channel in select statements:

```go
for {
    select {
    case <-sse.Done():
        return  // Client disconnected
    case data := <-dataChannel:
        sse.Write("data", data)
    }
}
```

### 3. Handle Errors

Write operations can fail if the client disconnects:

```go
if err := sse.Write("event", "data"); err != nil {
    log.Printf("Failed to write: %v", err)
    return
}
```

### 4. Use Heartbeats

Enable heartbeats for long-lived connections to detect disconnections:

```go
opts := &handlers.SSEOptions{
    HeartbeatInterval: 30 * time.Second,
}
sse, err := handlers.NewSSEHandlerWithOptions(w, r, opts)
```

### 5. Structure Your Data

Use JSON for structured data:

```go
type Update struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
}

sse.WriteJSON("update", Update{
    Type:    "notification",
    Payload: notification,
})
```

## Common Patterns

### Progress Updates

```go
func (h *Handler) StreamProgress(w http.ResponseWriter, r *http.Request) {
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    for i := 0; i <= 100; i += 10 {
        progress := map[string]interface{}{
            "percent": i,
            "message": fmt.Sprintf("Processing... %d%%", i),
        }

        if err := sse.WriteJSON("progress", progress); err != nil {
            return
        }

        time.Sleep(500 * time.Millisecond)
    }

    sse.WriteJSON("complete", map[string]string{
        "status": "success",
    })
}
```

### Tailing Logs

```go
func (h *Handler) TailLogs(w http.ResponseWriter, r *http.Request) {
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Tail command output
    cmd := exec.CommandContext(sse.Done(), "tail", "-f", "/var/log/app.log")
    cmd.Stdout = handlers.NewSSEWriter(sse, "log")
    cmd.Stderr = handlers.NewSSEWriter(sse, "error")

    if err := cmd.Run(); err != nil && sse.Err() == nil {
        log.Printf("Command failed: %v", err)
    }
}
```

### Multiplexed Streams

```go
func (h *Handler) StreamMultiple(w http.ResponseWriter, r *http.Request) {
    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Subscribe to multiple channels
    logs := make(chan string)
    metrics := make(chan Metric)

    for {
        select {
        case <-sse.Done():
            return
        case log := <-logs:
            sse.Write("log", log)
        case metric := <-metrics:
            sse.WriteJSON("metric", metric)
        }
    }
}
```

## Troubleshooting

### Connection Drops Immediately

Check if the ResponseWriter supports flushing:

```go
sse, err := handlers.NewSSEHandler(w, r)
if err != nil {
    // ResponseWriter doesn't support flushing
    log.Printf("SSE not supported: %v", err)
}
```

### Events Not Arriving

1. Check browser console for connection errors
2. Verify proxy/load balancer doesn't buffer responses
3. Enable heartbeats to keep connection alive
4. Check `X-Accel-Buffering: no` header is set

### Memory Leaks

Always defer `Close()` to ensure cleanup:

```go
defer sse.Close()  // Stops heartbeat goroutine
```

### Testing

Use `httptest.ResponseRecorder` for testing:

```go
func TestStreamHandler(t *testing.T) {
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/stream", nil)

    handler.Stream(w, r)

    output := w.Body.String()
    // Assert on output
}
```

## SSE Connection Manager

The `SSEManager` provides centralized connection tracking and broadcasting capabilities for managing multiple SSE connections efficiently.

### Features

- **Connection Tracking**: Track all active SSE connections in one place
- **Topic Subscriptions**: Subscribe connections to specific topics for targeted broadcasting
- **Automatic Cleanup**: Periodically remove dead connections
- **Thread-Safe**: Safe for concurrent use across goroutines
- **Broadcasting**: Send events to all connections or specific topic subscribers

### Quick Start

```go
// Create a global manager
var sseManager = handlers.NewSSEManager()

// In your HTTP handler:
func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
    allocID := r.URL.Query().Get("alloc_id")
    topic := fmt.Sprintf("logs:%s", allocID)

    sse, err := handlers.NewSSEHandler(w, r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer sse.Close()

    // Register connection with manager
    sseManager.Register(sse, topic)
    defer sseManager.Unregister(sse)

    // Wait for disconnection
    <-sse.Done()
}

// Broadcast to topic from anywhere:
func (h *Handler) OnLogEvent(allocID string, log LogEntry) {
    topic := fmt.Sprintf("logs:%s", allocID)
    sseManager.BroadcastJSONToTopic(topic, "log", log)
}
```

### Broadcasting Patterns

#### Broadcast to All Connections

```go
// Send to all active SSE connections
sseManager.Broadcast("notification", "Server maintenance in 5 minutes")

// Send JSON to all connections
sseManager.BroadcastJSON("alert", map[string]string{
    "level": "warning",
    "message": "High memory usage detected",
})
```

#### Topic-Based Broadcasting

```go
// Register connections with topics
sseManager.Register(sse, "logs:alloc-123", "metrics:alloc-123")

// Broadcast to specific topic
sseManager.BroadcastToTopic("logs:alloc-123", "log", logMessage)
sseManager.BroadcastJSONToTopic("metrics:alloc-123", "metric", metricData)
```

#### Dynamic Subscriptions

```go
// Add topics after registration
err := sseManager.Subscribe(sse, "alerts", "notifications")

// Remove topics
err := sseManager.Unsubscribe(sse, "alerts")
```

### Manager Configuration

```go
config := &handlers.SSEManagerConfig{
    CleanupInterval: 2 * time.Minute,  // How often to check for dead connections
    EnableLogging:   true,              // Log cleanup operations
}

manager := handlers.NewSSEManagerWithConfig(config)
defer manager.Shutdown()  // Always shutdown on application exit
```

### Monitoring Connections

```go
// Get connection counts
total := sseManager.ConnectionCount()
topics := sseManager.TopicCount()
subscribers := sseManager.TopicSubscribers("logs:alloc-123")

// Get active topics
activeTopics := sseManager.Topics()
for _, topic := range activeTopics {
    count := sseManager.TopicSubscribers(topic)
    log.Printf("Topic %s has %d subscribers", topic, count)
}
```

### Topic Naming Conventions

Use consistent topic naming for better organization:

```go
// Resource-based topics
fmt.Sprintf("logs:%s", allocID)
fmt.Sprintf("metrics:%s", allocID)
fmt.Sprintf("events:%s", jobID)

// Category-based topics
"system:alerts"
"system:notifications"
"admin:audit"

// User-specific topics
fmt.Sprintf("user:%s:notifications", userID)
```

## Client-Side SSE Client

The `SSEClient` JavaScript class provides automatic reconnection and integration with HTMX.

### Features

- **Auto-Reconnection**: Exponential backoff on connection failures
- **State Management**: Track connection state (connecting, connected, disconnected, error)
- **Event Handlers**: Register handlers for custom events
- **HTMX Integration**: Declarative SSE via `hx-ext="sse"`
- **Alpine.js Support**: Magic property for Alpine integration

### Basic Usage

```html
<script src="/static/js/sse-client.js"></script>
<script>
    const client = new SSEClient('/stream/logs?alloc_id=123', {
        debug: true,
        maxRetries: 10,
        onConnect: () => console.log('Connected!'),
        onDisconnect: () => console.log('Disconnected'),
    });

    client.on('log', (data) => {
        console.log('Log:', data);
    });

    client.connect();
</script>
```

### HTMX Integration

```html
<!-- Include the SSE client -->
<script src="/static/js/sse-client.js"></script>

<!-- Declarative SSE connection -->
<div hx-ext="sse" sse-connect="/stream/logs?alloc_id=123">
    <div sse-swap="log" hx-swap="beforeend">
        <!-- Log entries will be inserted here -->
    </div>

    <div sse-swap="error" hx-swap="innerHTML" class="error-container">
        <!-- Errors will replace content here -->
    </div>
</div>
```

### Advanced Configuration

```javascript
const client = new SSEClient('/stream', {
    // Reconnection settings
    initialRetryDelay: 1000,        // Start with 1 second
    maxRetryDelay: 30000,           // Max 30 seconds between retries
    maxRetries: Infinity,           // Retry forever
    backoffMultiplier: 2,           // Double delay each retry

    // Lifecycle callbacks
    onConnect: () => {
        console.log('Stream connected');
        statusIndicator.classList.add('connected');
    },

    onDisconnect: () => {
        console.log('Stream disconnected');
        statusIndicator.classList.remove('connected');
    },

    onError: (error) => {
        console.error('Stream error:', error);
    },

    onMaxRetries: () => {
        alert('Failed to connect after maximum retries');
    },

    // Debug and auth
    debug: true,
    withCredentials: true,  // Send cookies with request
});

// Register event handlers
client
    .on('log', handleLog)
    .on('metric', handleMetric)
    .on('alert', handleAlert);

// Start connection
client.connect();

// Later: disconnect
client.disconnect();
```

### Alpine.js Integration

```html
<div x-data="{ logs: [] }"
     x-init="$sse('/stream/logs').on('log', (data) => logs.push(data))">
    <template x-for="log in logs">
        <div x-text="log.message"></div>
    </template>
</div>
```

### Connection State

```javascript
const client = new SSEClient('/stream');

// Check current state
console.log(client.getState()); // 'disconnected', 'connecting', 'connected', 'error'

// React to state changes in lifecycle callbacks
const client = new SSEClient('/stream', {
    onConnect: () => updateUI('connected'),
    onDisconnect: () => updateUI('disconnected'),
    onError: () => updateUI('error'),
});
```

## SSE vs HTMX Polling

### When to Use SSE

Use SSE when you need:

- **Real-time updates**: Immediate notification of events (logs, metrics, alerts)
- **Long-lived connections**: Continuous data streams (tailing logs, monitoring)
- **Server-pushed updates**: Server needs to push data without client requests
- **Multiple event types**: Different events on the same connection
- **Low latency**: Updates within milliseconds of occurrence

**Examples**:
- Live log streaming
- Real-time metrics dashboards
- Job status updates
- System notifications
- Chat/messaging features

```html
<!-- SSE for real-time logs -->
<div hx-ext="sse" sse-connect="/stream/logs?alloc_id=123">
    <div sse-swap="log" hx-swap="beforeend" class="log-container"></div>
</div>
```

### When to Use HTMX Polling

Use HTMX polling when you need:

- **Periodic updates**: Data changes infrequently (every few seconds+)
- **Simple requests**: Standard HTTP GET requests suffice
- **Resource efficiency**: Fewer concurrent connections
- **Fallback compatibility**: Works without SSE support
- **Controlled intervals**: Fixed refresh rates

**Examples**:
- Dashboard refresh every 30 seconds
- Status checks
- List updates
- Simple counters

```html
<!-- Polling for periodic updates -->
<div hx-get="/api/status"
     hx-trigger="load, every 30s"
     hx-swap="innerHTML">
    Initial content
</div>
```

### Comparison Table

| Feature | SSE | HTMX Polling |
|---------|-----|--------------|
| Latency | <100ms | Depends on interval |
| Server Load | 1 connection per client | 1 request per interval |
| Client Load | Minimal | Minimal |
| Browser Support | Modern browsers | All browsers |
| Complexity | Moderate | Simple |
| Best For | Real-time streams | Periodic updates |

### Hybrid Approach

Combine both for optimal UX:

```html
<!-- Real-time updates via SSE -->
<div hx-ext="sse" sse-connect="/stream/live-updates">
    <div sse-swap="update" hx-swap="beforeend"></div>
</div>

<!-- Periodic full refresh via polling -->
<div hx-get="/api/full-state"
     hx-trigger="load, every 5m"
     hx-swap="outerHTML">
    <!-- Fallback content -->
</div>
```

## Performance Considerations

- **Memory**: Each connection holds a goroutine for heartbeats
- **CPU**: Flushing after each write can be CPU-intensive under high load
- **Network**: Heartbeats add minimal bandwidth (~50 bytes per interval)

For high-volume streaming, consider:
- Batching events before writing
- Increasing heartbeat interval
- Using the SSEManager for efficient broadcasting
- Topic-based subscriptions to avoid unnecessary broadcasts

### SSE Manager Performance

The SSEManager is optimized for:
- **Concurrent Access**: Lock-free reads for broadcasting
- **Dead Connection Cleanup**: Periodic cleanup prevents memory leaks
- **Efficient Broadcasting**: Snapshot connections before sending to minimize lock time

```go
// Efficient broadcasting pattern
config := &handlers.SSEManagerConfig{
    CleanupInterval: 2 * time.Minute,  // Balance cleanup vs overhead
    EnableLogging:   false,             // Disable in production
}
```

## Related

- [HTMX SSE Extension](https://htmx.org/extensions/server-sent-events/)
- [MDN: Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [W3C EventSource Specification](https://html.spec.whatwg.org/multipage/server-sent-events.html)
