# Connection Status Indicator

## Overview

The connection status indicator provides real-time monitoring of SSE (Server-Sent Events) and WebSocket connections throughout the Wander UI. It displays in the header and shows visual states for connection health.

## Features

- **Real-time connection monitoring**: Tracks all SSE and WebSocket connections
- **Visual state indicators**: Color-coded status dots (green, red, yellow) with animations
- **Automatic reconnection detection**: Monitors reconnection attempts and retry counts
- **Health checks**: Periodic checks for stale connections based on heartbeat timestamps
- **Manual reconnection**: Users can trigger reconnection attempts via the indicator
- **Mobile-friendly**: Responsive design with tooltip for detailed status

## Visual States

| State | Color | Description |
|-------|-------|-------------|
| **Connected** | Green | All monitored connections are healthy |
| **Disconnected** | Red | No active connections or all disconnected |
| **Connecting** | Yellow (pulsing) | Initial connection attempt in progress |
| **Reconnecting** | Yellow (pulsing) | Reconnection attempt after disconnect |
| **Error** | Grey | Connection errors detected or stale connections |

## Architecture

### Components

1. **ConnectionStatus Component** (`ui-htmx/templates/components/connection_status.templ`)
   - Templ component that renders the indicator UI
   - Uses Alpine.js for reactive state management
   - Displays status dot, text, and optional tooltip

2. **connectionStatus Alpine Pattern** (`ui-htmx/static/js/alpine-patterns.js`)
   - Alpine.js data function that manages connection state
   - Tracks individual connections in a Map
   - Listens for global connection events
   - Performs periodic health checks

3. **SSEClient Event Dispatching** (`ui-htmx/static/js/sse-client.js`)
   - Enhanced SSEClient class dispatches global events
   - Events: `sse:connect`, `sse:disconnect`, `sse:error`
   - Enables global connection monitoring

4. **WebSocket Event Dispatching** (`ui-htmx/static/js/alpine-patterns.js`)
   - execTerminal pattern dispatches WebSocket events
   - Events: `ws:connect`, `ws:disconnect`, `ws:error`
   - Consistent with SSE event structure

### Event Flow

```
┌─────────────────┐
│  SSE/WebSocket  │
│   Connection    │
└────────┬────────┘
         │
         ├─ onopen  ──> dispatch('sse:connect')
         ├─ onerror ──> dispatch('sse:error')
         └─ onclose ──> dispatch('sse:disconnect')
                │
                ▼
    ┌───────────────────────┐
    │  connectionStatus()   │
    │  Alpine.js Pattern    │
    └───────────┬───────────┘
                │
                ├─ Tracks connections in Map
                ├─ Updates aggregate state
                └─ Renders indicator UI
                        │
                        ▼
            ┌──────────────────┐
            │  Header Display  │
            │  Green/Red/Yellow│
            └──────────────────┘
```

## Integration

### Header Integration

The connection status indicator is integrated into the header component:

```go
// ui-htmx/templates/components/header.templ
@ConnectionStatus(ConnectionStatusProps{
    ID: "global-connection-status",
})
```

### SSE Integration

Any component using the SSEClient automatically reports connection status:

```javascript
// In Alpine.js component
const client = new SSEClient('/stream/logs', {
    onConnect: () => { /* Your handler */ },
    // Events are automatically dispatched
});
client.connect();
```

### WebSocket Integration

The execTerminal pattern dispatches WebSocket events:

```javascript
// In alpine-patterns.js execTerminal
this.websocket.onopen = () => {
    // Dispatch global event for monitoring
    window.dispatchEvent(new CustomEvent('ws:connect', {
        detail: { id: url, url: url }
    }));
};
```

## Configuration

### Health Check Options

The connectionStatus pattern accepts configuration options:

```javascript
connectionStatus({
    checkInterval: 5000,      // Check health every 5 seconds
    connectionTimeout: 30000, // Mark stale after 30 seconds
    enableHealthCheck: true   // Enable periodic health checks
})
```

### SSE Retry Configuration

SSE connections have built-in retry configuration:

```go
// In handlers/sse.go
DefaultSSEOptions() *SSEOptions {
    return &SSEOptions{
        HeartbeatInterval: 30 * time.Second,
        RetryInterval:     3 * time.Second,
    }
}
```

## Manual Reconnection

Users can manually trigger reconnection:

1. Click the connection indicator (mobile)
2. Tap the info icon to open tooltip
3. Click "Reconnect Now" button

This dispatches a global event that all connections listen for:

```javascript
// Dispatched by connectionStatus.reconnect()
window.dispatchEvent(new CustomEvent('connection:reconnect-requested'));

// Handled by logViewer, execTerminal, etc.
window.addEventListener('connection:reconnect-requested', () => {
    if (this.connectionState === 'disconnected') {
        this.connect();
    }
});
```

## Testing

### Test Pages with SSE

1. **Allocation Logs** (`/ui/allocations/:id?tab=logs`)
   - Uses logViewer pattern with SSE
   - Streams stdout/stderr logs in real-time

2. **Allocation Exec** (`/ui/allocations/:id?tab=exec`)
   - Uses execTerminal pattern with WebSocket
   - Interactive terminal session

### Manual Testing

1. Navigate to allocation logs page
2. Observe green indicator when streaming logs
3. Stop the Nomad agent to simulate disconnect
4. Observe red indicator and reconnection attempts
5. Click reconnect button to trigger manual reconnection
6. Restart agent and observe automatic reconnection

### Network Simulation

Use browser DevTools to simulate network conditions:

```
Chrome DevTools > Network tab > Throttling
- Offline: Test disconnection behavior
- Slow 3G: Test reconnection delays
- Custom: Set specific latency
```

## Health Check Logic

The connection status performs periodic health checks:

1. **Heartbeat Monitoring**
   - SSE server sends heartbeat comments every 30s
   - Client tracks `lastHeartbeatAt` timestamp
   - Connection marked stale if no heartbeat for 30s

2. **State Aggregation**
   - Tracks multiple connections simultaneously
   - Aggregates to worst state for display
   - Priority: connected > error > disconnected > connecting

3. **Stale Detection**
   ```javascript
   checkConnectionHealth() {
       const now = new Date();
       for (const conn of this.connections.values()) {
           const timeSinceHeartbeat = now - conn.lastHeartbeatAt;
           if (timeSinceHeartbeat > config.connectionTimeout) {
               conn.state = 'error';
           }
       }
   }
   ```

## Troubleshooting

### Indicator Always Shows Disconnected

**Cause**: No SSE/WebSocket connections active on current page

**Solution**: Navigate to a page with streaming data (logs, exec)

### Indicator Shows Error After Connect

**Cause**: Heartbeat timeout or server not sending heartbeats

**Solution**: Check server SSE configuration, ensure heartbeats enabled

### Manual Reconnect Not Working

**Cause**: Connection listeners not properly set up

**Solution**: Check console for errors, verify event handlers registered

### Multiple Indicators Show Different States

**Cause**: Page-local vs. global indicators tracking different connections

**Solution**: Header indicator is global, page indicators are local

## Future Enhancements

Potential improvements for future iterations:

1. **Reconnection Strategy Tuning**
   - Configurable backoff parameters
   - Max retry limits per connection type
   - Circuit breaker pattern

2. **Connection Analytics**
   - Track connection uptime/downtime
   - Log reconnection frequency
   - Alert on excessive failures

3. **Multi-Region Support**
   - Track connections per Nomad region
   - Display region-specific status
   - Handle region failover

4. **Enhanced UI**
   - Detailed connection list in dropdown
   - Per-connection status and controls
   - Connection history timeline

## Related Files

- `ui-htmx/templates/components/connection_status.templ` - Component template
- `ui-htmx/templates/components/header.templ` - Header integration
- `ui-htmx/static/js/alpine-patterns.js` - connectionStatus pattern
- `ui-htmx/static/js/sse-client.js` - SSEClient with events
- `ui-htmx/handlers/sse.go` - Server-side SSE handler
- `ui-htmx/templates/layouts/base.templ` - Script loading

## References

- [Server-Sent Events Spec](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [Alpine.js Documentation](https://alpinejs.dev/)
- [MDN EventSource API](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)
- [WebSocket API](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
