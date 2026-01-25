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

## Performance Considerations

- **Memory**: Each connection holds a goroutine for heartbeats
- **CPU**: Flushing after each write can be CPU-intensive under high load
- **Network**: Heartbeats add minimal bandwidth (~50 bytes per interval)

For high-volume streaming, consider:
- Batching events before writing
- Increasing heartbeat interval
- Using a dedicated streaming infrastructure (Redis Streams, etc.)

## Related

- [HTMX SSE Extension](https://htmx.org/extensions/server-sent-events/)
- [MDN: Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [W3C EventSource Specification](https://html.spec.whatwg.org/multipage/server-sent-events.html)
