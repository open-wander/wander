# User Permissions System

This document describes the user permissions system for the HTMX UI handlers.

## Overview

The permissions system provides easy capability checking for ACL-protected operations in the Wander UI. It extracts and stores user permissions from validated ACL tokens, making it simple to check if a user has permission to perform specific actions.

## Architecture

### Components

1. **ACLTokenInfo** (`auth.go`) - Contains validated ACL token information from Nomad API
2. **UserPermissions** (`permissions.go`) - Provides capability checking methods
3. **Context Integration** - Stores permissions in request context for easy access
4. **Middleware** - TokenValidationMiddleware populates permissions automatically

### Data Flow

```
Request → ACLMiddleware → TokenValidationMiddleware → UserPermissions → Handler
            (extract)         (validate + create)        (in context)    (check perms)
```

## Usage

### Basic Permission Checks

```go
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
    // Get permissions from context (set by TokenValidationMiddleware)
    perms := GetPermissionsFromRequest(r)
    if perms == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    namespace := r.URL.Query().Get("namespace")
    if namespace == "" {
        namespace = "default"
    }

    // Check permission
    if !perms.CanListJobs(namespace) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // User has permission, proceed...
}
```

### Available Check Methods

#### Token Type Checks
- `IsAnonymous() bool` - No token provided
- `IsManagement() bool` - Management token (has all permissions)
- `ACLEnabled() bool` - Is ACL enabled on cluster

#### Job Capabilities
- `CanListJobs(namespace string) bool`
- `CanReadJob(namespace string) bool`
- `CanSubmitJob(namespace string) bool`
- `CanScaleJob(namespace string) bool`

#### Allocation Capabilities
- `CanReadLogs(namespace string) bool`
- `CanReadFS(namespace string) bool`
- `CanExec(namespace string) bool`

#### Variable Capabilities
- `CanListVariables(namespace, path string) bool`
- `CanReadVariables(namespace, path string) bool`
- `CanWriteVariables(namespace, path string) bool`
- `CanDestroyVariables(namespace, path string) bool`

#### CSI Capabilities
- `CanReadCSI(namespace string) bool`
- `CanWriteCSI(namespace string) bool`
- `CanListCSI(namespace string) bool`

#### Generic Capability Check
- `HasCapability(namespace, capability string) bool`

### Advanced Usage: Loading Full Policies

For more detailed permission checks, you can load the full policy rules:

```go
perms := GetPermissionsFromRequest(r)
if perms != nil && !perms.IsManagement() {
    // Load full policies from Nomad API
    err := perms.LoadPolicies(r.Context(), h.nomadClient)
    if err != nil {
        h.logger.Error("failed to load policies", "error", err)
    }
}

// Now capability checks will use the parsed policy rules
canSubmit := perms.CanSubmitJob("production")
```

**Note:** Loading policies is expensive (requires API calls), so only do this when needed. For most operations, the API's own ACL checks are sufficient.

## Special Cases

### ACL Disabled

When ACL is disabled on the Nomad cluster:
- `ACLEnabled()` returns `false`
- All capability checks return `true`
- No token validation is performed

### Management Tokens

Management tokens have full access:
- `IsManagement()` returns `true`
- All capability checks return `true`
- No policy parsing is needed

### Anonymous Users

When no token is provided:
- `IsAnonymous()` returns `true`
- All capability checks return `false` (unless ACL is disabled)
- The anonymous policy applies (if configured in Nomad)

## Namespace Matching

The permissions system supports namespace patterns with wildcards:

- `"default"` - Exact match for namespace "default"
- `"prod-*"` - Matches "prod-app", "prod-api", etc.
- `"*"` - Matches all namespaces

The matching algorithm follows Nomad's namespace rules:
1. Exact match takes precedence
2. Most specific wildcard match wins
3. Falls back to "default" namespace if no match

## Variable Path Matching

Variable permissions support path patterns:

- `"app/config"` - Exact path match
- `"app/*"` - Matches all paths under "app/"
- `"*"` - Matches all paths

## Context Helpers

### Adding Permissions to Context

```go
perms := NewUserPermissions(tokenInfo)
ctx = WithPermissions(ctx, perms)
```

### Retrieving from Context

```go
perms, ok := GetPermissionsFromContext(ctx)
if !ok {
    // No permissions in context
}
```

### Convenience Helpers

```go
// From request
perms := GetPermissionsFromRequest(r)
perms := GetUserPermissionsFromRequest(r)

// Get underlying token
token := perms.GetToken()
tokenInfo := perms.GetTokenInfo()
```

## Middleware Setup

The permissions system is automatically enabled when using `TokenValidationMiddleware`:

```go
router.Use(h.ACLMiddleware)              // Extract token
router.Use(h.TokenValidationMiddleware)  // Validate + create permissions
```

After these middlewares run, all handlers can access permissions via the request context.

## Implementation Notes

### Design Decisions

1. **Deferred Policy Loading**: We don't load full policies by default to avoid latency. The token validation gives us the token type and basic info, which is sufficient for most checks. Full policies can be loaded on-demand when needed.

2. **Management Token Shortcut**: Management tokens bypass all capability checks, returning `true` immediately. This matches Nomad's behavior.

3. **ACL Disabled Shortcut**: When ACL is disabled, all checks return `true`. This maintains backward compatibility with non-ACL clusters.

4. **Context Storage**: Permissions are stored in request context, making them available throughout the request lifecycle without passing them explicitly.

### Future Enhancements

1. **Policy Caching**: Cache parsed policies to reduce API calls
2. **Roles Support**: Add support for ACL roles (already in token struct)
3. **Batch Capability Checks**: Check multiple capabilities at once
4. **Permission Bundles**: Pre-defined permission sets for common operations

## Testing

See `permissions_test.go` and `permissions_example_test.go` for comprehensive examples and test cases.

## References

- Nomad ACL Documentation: https://openwander.org/wander/docs/access-control
- Policy Syntax: `/acl/policy.go`
- Old Ember UI Implementation: `wander-old/ui/app/abilities/abstract.js`
