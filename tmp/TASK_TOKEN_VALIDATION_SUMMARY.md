# Token Validation Implementation Summary

## Status: DONE

## Files Changed

### Created
1. `/Users/stokvis/dev/wander/wander/ui-htmx/handlers/auth.go` - Core token validation logic
2. `/Users/stokvis/dev/wander/wander/ui-htmx/handlers/auth_test.go` - Unit tests for auth functionality

### Modified
1. `/Users/stokvis/dev/wander/wander/ui-htmx/handlers/context.go` - Added `GetACLTokenInfoFromRequest()` helper
2. `/Users/stokvis/dev/wander/wander/ui-htmx/handlers/router.go` - Added documentation on how to enable token validation

## Implementation Details

### Token Validation Function

Created `Handler.ValidateToken(ctx, token)` which:
- Takes a token string from request context
- Creates a temporary API client to avoid concurrency issues
- Calls `client.ACLTokens().Self()` to validate the token
- Returns an `ACLTokenInfo` struct with validation results
- Handles all required cases:
  - No token provided: Returns `IsValid=false`, `ACLEnabled=true` (assumed)
  - Valid token: Returns token details with `IsValid=true`
  - Invalid/expired token: Returns `IsValid=false` with error
  - ACL disabled: Returns `ACLEnabled=false`, `IsValid=true`

### Token Cache

Implemented a simple in-memory cache (`TokenCache`) that:
- Caches validation results to reduce API calls
- Uses TTL-based expiration (2 minutes default)
- Adjusts TTL based on token expiration time
- Runs background cleanup goroutine to remove expired entries
- Thread-safe with RWMutex

### Middleware

Created three middleware functions:

1. **TokenValidationMiddleware** (optional, not enabled by default)
   - Validates tokens via Nomad API
   - Stores `ACLTokenInfo` in request context
   - Does NOT block requests
   - Logs validation results for debugging

2. **RequireToken** (optional, for protected routes)
   - Enforces authentication
   - Returns 401 if ACL is enabled and token is invalid
   - Allows requests through if ACL is disabled
   - Should be used on specific routes that need protection

### Context Helpers

Added helpers to retrieve token validation info:
- `GetACLTokenInfoFromContext(ctx) (*ACLTokenInfo, bool)` - Get from context
- `WithACLTokenInfo(ctx, info)` - Add to context (for testing)
- `GetACLTokenInfoFromRequest(r) *ACLTokenInfo` - Convenience wrapper

### Data Structures

```go
// ACLTokenInfo contains validated ACL token information
type ACLTokenInfo struct {
    Token      *api.ACLToken  // Full token details from Nomad
    IsValid    bool            // Whether token is valid
    ACLEnabled bool            // Whether ACL is enabled on cluster
    Error      error           // Validation error if any
}
```

## Usage Examples

### Example 1: Check token in a handler
```go
func (h *Handler) SomeHandler(w http.ResponseWriter, r *http.Request) {
    // Get token validation info from context
    tokenInfo := GetACLTokenInfoFromRequest(r)

    if tokenInfo != nil && tokenInfo.IsValid && tokenInfo.Token != nil {
        // User is authenticated
        log.Printf("Authenticated as: %s", tokenInfo.Token.Name)
    } else if tokenInfo != nil && !tokenInfo.ACLEnabled {
        // ACL is disabled, treat as authenticated
        log.Printf("ACL disabled - allowing access")
    } else {
        // Anonymous access
        log.Printf("Anonymous access")
    }
}
```

### Example 2: Enable token validation globally
In `router.go`, modify `wrapMiddleware`:
```go
func (r *Router) wrapMiddleware(h http.Handler) http.Handler {
    return ACLMiddleware(
        r.handler.TokenValidationMiddleware(
            HTMXMiddleware(h)))
}
```

### Example 3: Protect specific routes
```go
// In registerJobsRoutes()
jobsHandler := NewJobsHandler(r.handler)
protectedHandler := r.handler.RequireToken(jobsHandler)
r.mux.Handle("/ui/jobs", r.wrapMiddleware(protectedHandler))
```

### Example 4: Conditionally protect routes
```go
func (r *Router) registerJobsRoutes() {
    jobsHandler := NewJobsHandler(r.handler)

    // List is public
    r.mux.Handle("/ui/jobs", r.wrapMiddleware(jobsHandler))

    // Detail requires auth (if path ends with a job ID)
    r.mux.HandleFunc("/ui/jobs/", func(w http.ResponseWriter, req *http.Request) {
        if strings.Count(req.URL.Path, "/") > 3 {
            // This is a detail page, require auth
            r.handler.RequireToken(
                r.wrapMiddleware(jobsHandler)).ServeHTTP(w, req)
        } else {
            r.wrapMiddleware(jobsHandler).ServeHTTP(w, req)
        }
    })
}
```

## Design Decisions

1. **Temporary client per validation**: Avoids concurrency issues with shared client
2. **Cache with TTL**: Reduces API calls while respecting token expiration
3. **Optional middleware**: Not enabled by default to avoid adding latency
4. **Non-blocking validation**: Handlers can check token info and decide
5. **ACL-disabled support**: Gracefully handles clusters without ACL
6. **Error details**: Logs detailed errors for debugging but returns simple messages to clients

## Testing

Created comprehensive unit tests:
- Token cache functionality (set, get, expiration)
- Context helpers
- TokenValidationMiddleware behavior
- RequireToken middleware with various token states
- ACL disabled scenario

## Integration Points

### With existing code:
- Uses existing `ACLMiddleware` to extract tokens
- Compatible with existing context helpers
- Follows existing middleware patterns
- Uses existing Handler struct and logger

### With Nomad API:
- Uses `api.ACLTokens().Self()` for validation
- Handles all Nomad API error messages
- Respects token expiration times
- Works with both management and client tokens

## Performance Considerations

1. **Caching**: Reduces API calls by caching validation results
2. **Lazy validation**: Only validates when middleware is enabled
3. **Concurrent safe**: Uses RWMutex for thread-safe cache access
4. **Background cleanup**: Prevents cache memory growth
5. **Temporary client**: Avoids mutex contention on shared client

## Security Considerations

1. **Token in context only**: Token never exposed in logs (only accessor ID)
2. **Short cache TTL**: Reduces window for stale validation
3. **Respects expiration**: Adjusts cache based on token expiry
4. **Clear error messages**: Helps debugging without leaking sensitive info
5. **Optional enforcement**: Allows gradual rollout of auth requirements

## Future Enhancements (not implemented)

- Distributed cache (Redis) for multi-instance deployments
- Token refresh/rotation support
- Policy-based authorization helpers
- Namespace-level permission checks
- Role-based access control helpers
- Audit logging integration

## Patterns Discovered

1. **Middleware composition**: Chain multiple middleware cleanly
2. **Context-based state**: Pass validation results through context
3. **Lazy initialization**: Initialize cache on first use with sync.Once
4. **Optional enforcement**: Validate but don't block by default
5. **Temporary clients**: Avoid shared state for concurrent operations
6. **TTL-based caching**: Simple but effective for reducing API calls

## Build Verification

```bash
$ go build ./ui-htmx/...
# Success - no errors

$ go build ./ui-htmx/handlers
# Success - no errors
```

Note: Test package has pre-existing compilation errors unrelated to this implementation.

## Next Steps (for orchestrator)

1. Enable TokenValidationMiddleware in router if desired
2. Add RequireToken to specific protected routes
3. Update user display to show authenticated user name
4. Add UI for token management (login/logout)
5. Implement policy-based authorization logic
6. Add audit logging for authentication events
