package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/open-wander/wander/api"
)

func TestNewUserPermissions_Anonymous(t *testing.T) {
	perms := NewUserPermissions(nil)

	if !perms.IsAnonymous() {
		t.Error("Expected anonymous user")
	}

	if perms.IsManagement() {
		t.Error("Anonymous user should not be management")
	}

	if !perms.ACLEnabled() {
		t.Error("Missing token information must fail closed and assume ACL is enabled")
	}

	if perms.CanListJobs("default") {
		t.Error("Anonymous user must not receive capabilities when ACL state is unknown")
	}
}

func TestNewUserPermissions_ACLDisabled(t *testing.T) {
	tokenInfo := &ACLTokenInfo{
		Token:      nil,
		IsValid:    true,
		ACLEnabled: false,
	}

	perms := NewUserPermissions(tokenInfo)

	if !perms.ACLEnabled() {
		// ACL is disabled, so all capabilities should be allowed
		if !perms.CanListJobs("default") {
			t.Error("Expected all capabilities when ACL is disabled")
		}
		if !perms.CanSubmitJob("default") {
			t.Error("Expected all capabilities when ACL is disabled")
		}
	}
}

func TestNewUserPermissions_ManagementToken(t *testing.T) {
	token := &api.ACLToken{
		AccessorID: "test-accessor",
		SecretID:   "test-secret",
		Name:       "Test Management Token",
		Type:       "management",
		Policies:   []string{},
		Global:     true,
		CreateTime: time.Now(),
	}

	tokenInfo := &ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}

	perms := NewUserPermissions(tokenInfo)

	if perms.IsAnonymous() {
		t.Error("Management token should not be anonymous")
	}

	if !perms.IsManagement() {
		t.Error("Expected management token")
	}

	// Management tokens have all capabilities
	if !perms.CanListJobs("default") {
		t.Error("Management token should have list-jobs capability")
	}

	if !perms.CanSubmitJob("default") {
		t.Error("Management token should have submit-job capability")
	}

	if !perms.CanReadVariables("default", "test/path") {
		t.Error("Management token should have read variables capability")
	}
}

func TestNewUserPermissions_ClientToken(t *testing.T) {
	token := &api.ACLToken{
		AccessorID: "test-accessor",
		SecretID:   "test-secret",
		Name:       "Test Client Token",
		Type:       "client",
		Policies:   []string{"default-read"},
		Global:     false,
		CreateTime: time.Now(),
	}

	tokenInfo := &ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}

	perms := NewUserPermissions(tokenInfo)

	if perms.IsAnonymous() {
		t.Error("Client token should not be anonymous")
	}

	if perms.IsManagement() {
		t.Error("Client token should not be management")
	}

	// Client tokens don't have capabilities until policies are loaded
	// (we defer policy fetching to avoid latency)
	if perms.CanListJobs("default") {
		t.Error("Client token should not have capabilities until policies are loaded")
	}
}

func TestMatchesNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		pattern   string
		expected  bool
	}{
		{"default", "default", true},
		{"default", "prod", false},
		{"prod-app", "prod-*", true},
		{"prod-api", "prod-*", true},
		{"dev-app", "prod-*", false},
		{"test", "*", true},
		{"anything", "*", true},
		{"prod-app-v1", "prod-app-*", true},
		{"prod-app-v1", "prod-*", true},
	}

	for _, tt := range tests {
		result := matchesNamespace(tt.namespace, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchesNamespace(%q, %q) = %v, want %v",
				tt.namespace, tt.pattern, result, tt.expected)
		}
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		path     string
		pattern  string
		expected bool
	}{
		{"test/path", "test/path", true},
		{"test/path", "other/path", false},
		{"test/path/sub", "test/*", true},
		{"test/path/sub/deep", "test/*", true},
		{"other/path", "test/*", false},
		{"anything", "*", true},
	}

	for _, tt := range tests {
		result := matchesPath(tt.path, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchesPath(%q, %q) = %v, want %v",
				tt.path, tt.pattern, result, tt.expected)
		}
	}
}

func TestGetPermissionsFromContext(t *testing.T) {
	ctx := context.Background()

	// No permissions in context
	perms, ok := GetPermissionsFromContext(ctx)
	if ok {
		t.Error("Expected no permissions in empty context")
	}
	if perms != nil {
		t.Error("Expected nil permissions")
	}

	// Add permissions to context
	testPerms := NewUserPermissions(nil)
	ctx = WithPermissions(ctx, testPerms)

	// Retrieve permissions
	perms, ok = GetPermissionsFromContext(ctx)
	if !ok {
		t.Error("Expected permissions in context")
	}
	if perms == nil {
		t.Error("Expected non-nil permissions")
	}
	if perms != testPerms {
		t.Error("Expected same permissions instance")
	}
}

func TestUserPermissions_GetToken(t *testing.T) {
	// Anonymous user
	perms := NewUserPermissions(nil)
	if perms.GetToken() != nil {
		t.Error("Anonymous user should have nil token")
	}

	// User with token
	token := &api.ACLToken{
		AccessorID: "test-accessor",
		SecretID:   "test-secret",
		Name:       "Test Token",
		Type:       "client",
	}

	tokenInfo := &ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}

	perms = NewUserPermissions(tokenInfo)
	if perms.GetToken() == nil {
		t.Error("Expected non-nil token")
	}
	if perms.GetToken() != token {
		t.Error("Expected same token instance")
	}
}
