package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/handlers"
)

// Example_permissions demonstrates how to use UserPermissions for authorization checks.
func Example_permissions() {
	// Create a sample ACL token (this would come from ValidateToken in real usage)
	token := &api.ACLToken{
		AccessorID: "test-accessor-123",
		SecretID:   "test-secret-456",
		Name:       "Example User Token",
		Type:       "client",
		Policies:   []string{"developer-policy"},
		Global:     false,
		CreateTime: time.Now(),
	}

	tokenInfo := &handlers.ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}

	// Create user permissions
	perms := handlers.NewUserPermissions(tokenInfo)

	// Check various capabilities
	fmt.Printf("Is Anonymous: %v\n", perms.IsAnonymous())
	fmt.Printf("Is Management: %v\n", perms.IsManagement())
	fmt.Printf("ACL Enabled: %v\n", perms.ACLEnabled())

	// Output:
	// Is Anonymous: false
	// Is Management: false
	// ACL Enabled: true
}

// Example_permissionsManagement demonstrates management token behavior.
func Example_permissionsManagement() {
	// Management token has full access
	token := &api.ACLToken{
		AccessorID: "mgmt-accessor",
		SecretID:   "mgmt-secret",
		Name:       "Management Token",
		Type:       "management",
		Global:     true,
		CreateTime: time.Now(),
	}

	tokenInfo := &handlers.ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}

	perms := handlers.NewUserPermissions(tokenInfo)

	// Management tokens have all capabilities
	fmt.Printf("Can List Jobs: %v\n", perms.CanListJobs("default"))
	fmt.Printf("Can Submit Jobs: %v\n", perms.CanSubmitJob("default"))
	fmt.Printf("Can Read Variables: %v\n", perms.CanReadVariables("default", "app/config"))
	fmt.Printf("Can Write Variables: %v\n", perms.CanWriteVariables("default", "app/config"))

	// Output:
	// Can List Jobs: true
	// Can Submit Jobs: true
	// Can Read Variables: true
	// Can Write Variables: true
}

// Example_permissionsACLDisabled demonstrates behavior when ACL is disabled.
func Example_permissionsACLDisabled() {
	// When ACL is disabled, all operations are allowed
	tokenInfo := &handlers.ACLTokenInfo{
		Token:      nil,
		IsValid:    true,
		ACLEnabled: false,
	}

	perms := handlers.NewUserPermissions(tokenInfo)

	// All capabilities are granted when ACL is disabled
	fmt.Printf("ACL Enabled: %v\n", perms.ACLEnabled())
	fmt.Printf("Can List Jobs: %v\n", perms.CanListJobs("default"))
	fmt.Printf("Can Submit Jobs: %v\n", perms.CanSubmitJob("default"))

	// Output:
	// ACL Enabled: false
	// Can List Jobs: true
	// Can Submit Jobs: true
}

// Example_permissionsMiddleware demonstrates using permissions in HTTP handlers.
func Example_permissionsMiddleware() {
	// This is how you would use permissions in a handler

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get permissions from context (set by TokenValidationMiddleware)
		perms := handlers.GetPermissionsFromRequest(r)
		if perms == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get namespace from request (from query param, path, etc.)
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		// Check if user can list jobs in this namespace
		if !perms.CanListJobs(namespace) {
			http.Error(w, "Forbidden: Cannot list jobs in this namespace", http.StatusForbidden)
			return
		}

		// User has permission, proceed with listing jobs
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Listing jobs in namespace: %s", namespace)
	})

	// Example request with permissions in context
	token := &api.ACLToken{
		Type: "management",
	}
	tokenInfo := &handlers.ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}
	perms := handlers.NewUserPermissions(tokenInfo)

	req, _ := http.NewRequest("GET", "/api/jobs?namespace=default", nil)
	ctx := handlers.WithPermissions(context.Background(), perms)
	req = req.WithContext(ctx)

	// Serve the request (would normally use httptest.ResponseRecorder)
	_ = handler
	_ = req

	fmt.Println("Handler would process request with permissions")

	// Output:
	// Handler would process request with permissions
}

// Example_permissionsContext demonstrates working with permissions in context.
func Example_permissionsContext() {
	ctx := context.Background()

	// Create and add permissions to context
	token := &api.ACLToken{
		Name: "Test Token",
		Type: "client",
	}
	tokenInfo := &handlers.ACLTokenInfo{
		Token:      token,
		IsValid:    true,
		ACLEnabled: true,
	}
	perms := handlers.NewUserPermissions(tokenInfo)
	ctx = handlers.WithPermissions(ctx, perms)

	// Retrieve permissions from context
	retrieved, ok := handlers.GetPermissionsFromContext(ctx)
	fmt.Printf("Found permissions: %v\n", ok)
	fmt.Printf("Token name: %s\n", retrieved.GetToken().Name)

	// Output:
	// Found permissions: true
	// Token name: Test Token
}
