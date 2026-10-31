package handlers

import (
	"context"
	"net/http"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Context keys for HTMX-related values
const (
	htmxContextKey     contextKey = "htmx_request"
	htmxCurrentURLKey  contextKey = "htmx_current_url"
	htmxTargetKey      contextKey = "htmx_target"
	htmxTriggerKey     contextKey = "htmx_trigger"
	htmxTriggerNameKey contextKey = "htmx_trigger_name"
	wanderTokenKey     contextKey = "wander_token"
	wanderNamespaceKey contextKey = "wander_namespace"
	wanderRegionKey    contextKey = "wander_region"
)

const (
	defaultNamespace = "default"
	defaultRegion    = "global"
)

// IsHTMXRequestFromContext retrieves the HTMX request status from the context.
// Returns true if the request is an HTMX AJAX request, false otherwise.
// This should be used after the HTMXMiddleware has been applied.
func IsHTMXRequestFromContext(ctx context.Context) bool {
	isHTMX, ok := ctx.Value(htmxContextKey).(bool)
	if !ok {
		return false
	}
	return isHTMX
}

// GetCurrentURLFromContext retrieves the current URL from the request context.
// Returns the URL string and true if found, empty string and false otherwise.
func GetCurrentURLFromContext(ctx context.Context) (string, bool) {
	url, ok := ctx.Value(htmxCurrentURLKey).(string)
	return url, ok
}

// GetTargetFromContext retrieves the target element ID from the request context.
// Returns the target ID and true if found, empty string and false otherwise.
func GetTargetFromContext(ctx context.Context) (string, bool) {
	target, ok := ctx.Value(htmxTargetKey).(string)
	return target, ok
}

// GetTriggerFromContext retrieves the triggering element ID from the request context.
// Returns the trigger ID and true if found, empty string and false otherwise.
func GetTriggerFromContext(ctx context.Context) (string, bool) {
	trigger, ok := ctx.Value(htmxTriggerKey).(string)
	return trigger, ok
}

// GetTriggerNameFromContext retrieves the triggering element name from the request context.
// Returns the trigger name and true if found, empty string and false otherwise.
func GetTriggerNameFromContext(ctx context.Context) (string, bool) {
	triggerName, ok := ctx.Value(htmxTriggerNameKey).(string)
	return triggerName, ok
}

// HTMXRequestInfo contains all HTMX-related information from the request context.
type HTMXRequestInfo struct {
	IsHTMX      bool
	CurrentURL  string
	Target      string
	Trigger     string
	TriggerName string
}

// GetHTMXInfo retrieves all HTMX information from the request context.
// Returns an HTMXRequestInfo struct with all available HTMX data.
func GetHTMXInfo(ctx context.Context) HTMXRequestInfo {
	info := HTMXRequestInfo{
		IsHTMX: IsHTMXRequestFromContext(ctx),
	}

	if url, ok := GetCurrentURLFromContext(ctx); ok {
		info.CurrentURL = url
	}

	if target, ok := GetTargetFromContext(ctx); ok {
		info.Target = target
	}

	if trigger, ok := GetTriggerFromContext(ctx); ok {
		info.Trigger = trigger
	}

	if triggerName, ok := GetTriggerNameFromContext(ctx); ok {
		info.TriggerName = triggerName
	}

	return info
}

// WithHTMXContext is a helper for testing that adds HTMX information to a context.
// This is useful for unit tests that need to simulate HTMX requests.
func WithHTMXContext(ctx context.Context, isHTMX bool) context.Context {
	return context.WithValue(ctx, htmxContextKey, isHTMX)
}

// IsHTMXFromRequest is a convenience function that checks both the request headers
// and the context for HTMX status. It first checks the context (if middleware was used),
// then falls back to checking the request headers directly.
func IsHTMXFromRequest(r *http.Request) bool {
	// First try to get from context (if middleware was applied)
	if isHTMX := IsHTMXRequestFromContext(r.Context()); isHTMX {
		return true
	}

	// Fall back to checking headers directly
	return IsHTMXRequest(r)
}

// GetTokenFromContext retrieves the ACL token from the request context.
// Returns the token string and true if found, empty string and false otherwise.
// The token is set by the ACLMiddleware from either the X-Nomad-Token header
// or the wander_token cookie.
func GetTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(wanderTokenKey).(string)
	return token, ok
}

// WithTokenContext is a helper for testing that adds an ACL token to a context.
// This is useful for unit tests that need to simulate authenticated requests.
func WithTokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, wanderTokenKey, token)
}

// GetTokenFromRequest is a convenience function that retrieves the token from
// either the request context (if ACLMiddleware was applied) or directly from
// the request headers/cookies.
func GetTokenFromRequest(r *http.Request) string {
	// First try to get from context (if middleware was applied)
	if token, ok := GetTokenFromContext(r.Context()); ok {
		return token
	}

	// Fall back to checking headers and cookies directly
	return extractToken(r)
}

// GetNamespaceFromRequest returns the namespace resolved by SelectionMiddleware.
// It falls back to the default namespace for directly-invoked handlers in tests.
func GetNamespaceFromRequest(r *http.Request) string {
	if namespace, ok := r.Context().Value(wanderNamespaceKey).(string); ok && namespace != "" {
		return namespace
	}
	return selectionValue(r, "namespace", WanderNamespaceCookie, defaultNamespace, true)
}

// GetRegionFromRequest returns the region resolved by SelectionMiddleware. It
// falls back to the default region for directly-invoked handlers in tests.
func GetRegionFromRequest(r *http.Request) string {
	if region, ok := r.Context().Value(wanderRegionKey).(string); ok && region != "" {
		return region
	}
	return selectionValue(r, "region", WanderRegionCookie, defaultRegion, false)
}

// GetACLTokenInfoFromRequest is a convenience function that retrieves the
// validated ACL token info from the request context.
// Returns nil if TokenValidationMiddleware has not been applied.
func GetACLTokenInfoFromRequest(r *http.Request) *ACLTokenInfo {
	info, _ := GetACLTokenInfoFromContext(r.Context())
	return info
}

// GetUserPermissionsFromRequest is a convenience function that retrieves the
// UserPermissions from the request context.
// Returns nil if TokenValidationMiddleware has not been applied.
func GetUserPermissionsFromRequest(r *http.Request) *UserPermissions {
	return GetPermissionsFromRequest(r)
}
