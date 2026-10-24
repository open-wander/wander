# Cluster Unavailability Error Handling

## Overview

This document describes the cluster unavailability error handling implementation for the Wander UI. When a Nomad cluster has no elected leader, API calls will fail with specific error messages. The UI now detects these errors and displays a user-friendly error page.

## Implementation

### Error Detection

The `IsClusterUnavailableError(err error) bool` function detects cluster unavailability by checking for common error message patterns:

- "no cluster leader"
- "no leader"
- "cluster unavailable"
- "server unavailable"
- "leadership lost"

### Error Types

**ClusterUnavailableError** - Represents cluster unavailability state:
```go
type ClusterUnavailableError struct {
    Message     string   // Main error message
    Details     string   // Additional technical details
    Suggestions []string // Helpful suggestions for users
}
```

### Rendering

**RenderClusterUnavailable** - Renders appropriate error pages:
- **HTMX requests**: Returns a partial error display with retry button
- **Full page requests**: Returns a complete error page with auto-refresh (10 seconds)

The full page includes:
- Clear "Cluster Unavailable" header with server icon
- Explanation of the issue
- Helpful suggestions for users
- Auto-refresh countdown and manual retry button

### Helper Method

**CheckClusterError** - Convenient method to check and handle cluster errors:

```go
func (h *Handler) CheckClusterError(w http.ResponseWriter, r *http.Request, err error) bool
```

Returns `true` if error was handled, `false` otherwise.

## Usage

### In Handlers

```go
// Fetch data from Nomad API
jobs, _, err := client.Jobs().List(opts)
if err != nil {
    // Check cluster unavailability first
    if h.handler.CheckClusterError(w, r, err) {
        return // Error page rendered
    }
    // Handle other errors
    http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
    return
}
```

### Current Integration

The cluster error check has been integrated into:

1. **Job List Handler** (`jobs.go`) - When fetching jobs list
2. **Job Detail Handler** (`jobs.go`) - When fetching job details
3. **Namespace Refresh** (`handlers.go`) - Silently handles cluster unavailability during namespace fetch

## Error Page Features

### Full Page View
- Orange-themed design to indicate transient issue (not critical error)
- Server/cluster icon
- Clear "Cluster Unavailable" title with "No Cluster Leader" subtitle
- Detailed explanation message
- Optional technical details section
- Helpful suggestions list with icons
- Auto-refresh indicator with spinning icon
- Manual "Retry Now" button
- Auto-refreshes every 10 seconds

### HTMX Partial View
- Compact orange alert box
- Warning icon
- Brief message
- Optional details
- Suggestions list
- Manual "Retry" button

## Common Scenarios

### When This Error Appears

1. **Cluster Startup** - During initial cluster formation
2. **Leader Election** - When a leader crashes and a new one is being elected
3. **Network Issues** - When server nodes cannot communicate
4. **Quorum Loss** - When majority of servers are unreachable
5. **Configuration Issues** - After adding/removing server nodes

### User Guidance

The error page provides these suggestions:
- Wait for leader election to complete (usually quick)
- Check that all server nodes are running
- Verify network connectivity between servers
- Check server logs for leadership issues
- Contact cluster administrator if persistent

## Files Modified

- `/ui-htmx/handlers/errors.go` - Added ClusterUnavailableError type and rendering functions
- `/ui-htmx/handlers/handlers.go` - Added CheckClusterError helper method
- `/ui-htmx/handlers/jobs.go` - Integrated cluster error checks
- `/ui-htmx/handlers/CLUSTER_ERROR_HANDLING.md` - This documentation

## Testing

To test the error page:

1. Stop all Nomad server nodes to simulate no leader
2. Access any UI page that makes API calls (e.g., /ui/jobs)
3. Verify the cluster unavailable error page displays
4. Verify auto-refresh works
5. Start server nodes and verify page recovers automatically

## Future Enhancements

Consider adding cluster error checks to:
- Allocation handlers
- Client/Server handlers (where errors are currently swallowed)
- Deployment handlers
- CSI volume handlers
- Variable handlers
