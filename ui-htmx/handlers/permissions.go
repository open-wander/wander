package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/api"
)

const (
	permissionsContextKey contextKey = "user_permissions"
)

// UserPermissions provides easy permission checking for the current user.
// It parses ACL token policies and provides methods to check capabilities.
type UserPermissions struct {
	// Raw token info from the API
	tokenInfo *ACLTokenInfo

	// Parsed policies for efficient capability lookups
	// Map structure: namespace -> capability -> bool
	namespaceCapabilities map[string]map[string]bool

	// Parsed variable policies for path-based permissions
	// Map structure: namespace -> path -> capabilities
	variableCapabilities map[string]map[string][]string

	// Reference to Nomad client for fetching policies
	client *api.Client
}

// NewUserPermissions creates a UserPermissions instance from validated ACL token info.
// The client parameter is optional and only needed if you want to fetch full policy details.
// For now, we create the structure but defer policy fetching to avoid latency.
func NewUserPermissions(tokenInfo *ACLTokenInfo) *UserPermissions {
	if tokenInfo == nil {
		return &UserPermissions{
			tokenInfo:             nil,
			namespaceCapabilities: make(map[string]map[string]bool),
			variableCapabilities:  make(map[string]map[string][]string),
		}
	}

	perms := &UserPermissions{
		tokenInfo:             tokenInfo,
		namespaceCapabilities: make(map[string]map[string]bool),
		variableCapabilities:  make(map[string]map[string][]string),
	}

	// If ACL is disabled, no need to parse policies - everything is allowed
	if !tokenInfo.ACLEnabled {
		return perms
	}

	// Note: We don't parse policies here because:
	// 1. The ACLToken only has policy names, not the actual policy rules
	// 2. Fetching policies would require additional API calls and add latency
	// 3. For most UI operations, we can rely on the API's own ACL checks
	// 4. When we need detailed permissions, we can fetch policies on-demand

	return perms
}

// LoadPolicies fetches and parses the policy rules from the Nomad API.
// This is an expensive operation and should be called sparingly.
// Most UI operations should rely on the API's own ACL checks instead.
func (p *UserPermissions) LoadPolicies(ctx context.Context, client *api.Client) error {
	if p.tokenInfo == nil || p.tokenInfo.Token == nil || client == nil {
		return nil
	}

	// Management tokens don't need policy parsing
	if p.IsManagement() {
		return nil
	}

	p.client = client

	// Fetch each policy by name
	for _, policyName := range p.tokenInfo.Token.Policies {
		policy, _, err := client.ACLPolicies().Info(policyName, nil)
		if err != nil {
			// Log error but continue with other policies
			continue
		}

		// Parse the HCL rules
		if err := p.parsePolicy(policy); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// parsePolicy parses a single ACL policy and updates the capability maps.
func (p *UserPermissions) parsePolicy(policy *api.ACLPolicy) error {
	if policy == nil || policy.Rules == "" {
		return nil
	}

	// Parse the HCL rules using Nomad's ACL parser
	parsed, err := acl.Parse(policy.Rules)
	if err != nil {
		return err
	}

	// Process namespace policies
	for _, ns := range parsed.Namespaces {
		if p.namespaceCapabilities[ns.Name] == nil {
			p.namespaceCapabilities[ns.Name] = make(map[string]bool)
		}

		// Add all capabilities for this namespace
		for _, cap := range ns.Capabilities {
			p.namespaceCapabilities[ns.Name][cap] = true
		}

		// Process variable policies within this namespace
		if ns.Variables != nil {
			if p.variableCapabilities[ns.Name] == nil {
				p.variableCapabilities[ns.Name] = make(map[string][]string)
			}

			for _, pathPolicy := range ns.Variables.Paths {
				p.variableCapabilities[ns.Name][pathPolicy.PathSpec] = pathPolicy.Capabilities
			}
		}
	}

	return nil
}

// IsAnonymous returns true if no ACL token was provided.
func (p *UserPermissions) IsAnonymous() bool {
	return p.tokenInfo == nil || p.tokenInfo.Token == nil
}

// IsManagement returns true if the token is a management token (full access).
func (p *UserPermissions) IsManagement() bool {
	if p.tokenInfo == nil || p.tokenInfo.Token == nil {
		return false
	}
	return p.tokenInfo.Token.Type == "management"
}

// ACLEnabled returns true if ACL is enabled on the cluster.
func (p *UserPermissions) ACLEnabled() bool {
	if p.tokenInfo == nil {
		return true // Default to assuming ACL is enabled
	}
	return p.tokenInfo.ACLEnabled
}

// HasCapability checks if the user has a specific capability in a namespace.
// Management tokens always have all capabilities.
// When ACL is disabled, all capabilities are granted.
func (p *UserPermissions) HasCapability(namespace, capability string) bool {
	// If ACL is disabled, allow everything
	if !p.ACLEnabled() {
		return true
	}

	// Management tokens have all capabilities
	if p.IsManagement() {
		return true
	}

	// Anonymous users have no capabilities
	if p.IsAnonymous() {
		return false
	}

	// Check if capability exists in the namespace
	if nsCaps, ok := p.namespaceCapabilities[namespace]; ok {
		if nsCaps[capability] {
			return true
		}
	}

	// Check wildcard namespaces
	for nsPattern, nsCaps := range p.namespaceCapabilities {
		if matchesNamespace(namespace, nsPattern) {
			if nsCaps[capability] {
				return true
			}
		}
	}

	// Check if deny capability is set (denies override everything)
	if nsCaps, ok := p.namespaceCapabilities[namespace]; ok {
		if nsCaps[acl.NamespaceCapabilityDeny] {
			return false
		}
	}

	return false
}

// matchesNamespace checks if a namespace matches a pattern (supporting wildcards).
// Implements the same matching logic as the old Ember UI.
func matchesNamespace(namespace, pattern string) bool {
	// Exact match
	if namespace == pattern {
		return true
	}

	// No wildcard in pattern
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Convert wildcard pattern to regex
	regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
	matched, err := regexp.MatchString(regexPattern, namespace)
	if err != nil {
		return false
	}

	return matched
}

// HasVariableCapability checks if the user has a specific capability for a variable path.
func (p *UserPermissions) HasVariableCapability(namespace, path, capability string) bool {
	// If ACL is disabled, allow everything
	if !p.ACLEnabled() {
		return true
	}

	// Management tokens have all capabilities
	if p.IsManagement() {
		return true
	}

	// Anonymous users have no capabilities
	if p.IsAnonymous() {
		return false
	}

	// Check variable capabilities for the namespace
	if nsPaths, ok := p.variableCapabilities[namespace]; ok {
		// Find the most specific matching path
		for pathPattern, caps := range nsPaths {
			if matchesPath(path, pathPattern) {
				for _, cap := range caps {
					if cap == capability {
						return true
					}
				}
			}
		}
	}

	return false
}

// matchesPath checks if a path matches a pattern (supporting wildcards).
func matchesPath(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}

	// No wildcard in pattern
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Convert wildcard pattern to regex
	regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
	matched, err := regexp.MatchString(regexPattern, path)
	if err != nil {
		return false
	}

	return matched
}

// Convenience methods for common capability checks

// CanListJobs returns true if the user can list jobs in the namespace.
func (p *UserPermissions) CanListJobs(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityListJobs)
}

// CanReadJob returns true if the user can read job details in the namespace.
func (p *UserPermissions) CanReadJob(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityReadJob)
}

// CanSubmitJob returns true if the user can submit/modify jobs in the namespace.
func (p *UserPermissions) CanSubmitJob(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilitySubmitJob)
}

// CanScaleJob returns true if the user can scale jobs in the namespace.
func (p *UserPermissions) CanScaleJob(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityScaleJob)
}

// CanReadLogs returns true if the user can read allocation logs in the namespace.
func (p *UserPermissions) CanReadLogs(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityReadLogs)
}

// CanReadFS returns true if the user can read allocation filesystem in the namespace.
func (p *UserPermissions) CanReadFS(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityReadFS)
}

// CanExec returns true if the user can execute commands in allocations in the namespace.
func (p *UserPermissions) CanExec(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityAllocExec)
}

// CanListVariables returns true if the user can list variables in the namespace.
func (p *UserPermissions) CanListVariables(namespace, path string) bool {
	return p.HasVariableCapability(namespace, path, acl.VariablesCapabilityList)
}

// CanReadVariables returns true if the user can read variables in the namespace.
func (p *UserPermissions) CanReadVariables(namespace, path string) bool {
	return p.HasVariableCapability(namespace, path, acl.VariablesCapabilityRead)
}

// CanWriteVariables returns true if the user can write variables in the namespace.
func (p *UserPermissions) CanWriteVariables(namespace, path string) bool {
	return p.HasVariableCapability(namespace, path, acl.VariablesCapabilityWrite)
}

// CanDestroyVariables returns true if the user can destroy variables in the namespace.
func (p *UserPermissions) CanDestroyVariables(namespace, path string) bool {
	return p.HasVariableCapability(namespace, path, acl.VariablesCapabilityDestroy)
}

// CanReadCSI returns true if the user can read CSI volumes in the namespace.
func (p *UserPermissions) CanReadCSI(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityCSIReadVolume)
}

// CanWriteCSI returns true if the user can write CSI volumes in the namespace.
func (p *UserPermissions) CanWriteCSI(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityCSIWriteVolume)
}

// CanListCSI returns true if the user can list CSI volumes in the namespace.
func (p *UserPermissions) CanListCSI(namespace string) bool {
	return p.HasCapability(namespace, acl.NamespaceCapabilityCSIListVolume)
}

// GetToken returns the underlying ACL token, if available.
func (p *UserPermissions) GetToken() *api.ACLToken {
	if p.tokenInfo == nil {
		return nil
	}
	return p.tokenInfo.Token
}

// GetTokenInfo returns the underlying ACL token info.
func (p *UserPermissions) GetTokenInfo() *ACLTokenInfo {
	return p.tokenInfo
}

// GetPermissionsFromContext retrieves the UserPermissions from the request context.
// Returns the UserPermissions and true if found, nil and false otherwise.
func GetPermissionsFromContext(ctx context.Context) (*UserPermissions, bool) {
	perms, ok := ctx.Value(permissionsContextKey).(*UserPermissions)
	return perms, ok
}

// WithPermissions adds UserPermissions to the context.
func WithPermissions(ctx context.Context, perms *UserPermissions) context.Context {
	return context.WithValue(ctx, permissionsContextKey, perms)
}

// GetPermissionsFromRequest is a convenience function that retrieves the
// UserPermissions from the request context.
// Returns nil if TokenValidationMiddleware has not been applied.
func GetPermissionsFromRequest(r *http.Request) *UserPermissions {
	perms, _ := GetPermissionsFromContext(r.Context())
	return perms
}
