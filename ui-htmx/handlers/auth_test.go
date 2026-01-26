package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
)

func TestTokenCache(t *testing.T) {
	cache := getTokenCache()

	// Test Set and Get
	token := "test-token-123"
	info := &ACLTokenInfo{
		IsValid:    true,
		ACLEnabled: true,
	}

	cache.Set(token, info, 1*time.Second)

	// Should retrieve the cached entry
	retrieved, ok := cache.Get(token)
	if !ok {
		t.Fatal("expected to retrieve cached token")
	}
	if !retrieved.IsValid {
		t.Error("expected cached token to be valid")
	}

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Should not retrieve expired entry
	_, ok = cache.Get(token)
	if ok {
		t.Error("expected cached token to be expired")
	}
}

func TestGetACLTokenInfoFromContext(t *testing.T) {
	ctx := context.Background()

	// Test with no info in context
	_, ok := GetACLTokenInfoFromContext(ctx)
	if ok {
		t.Error("expected no token info in empty context")
	}

	// Test with info in context
	info := &ACLTokenInfo{
		IsValid:    true,
		ACLEnabled: true,
	}
	ctx = WithACLTokenInfo(ctx, info)

	retrieved, ok := GetACLTokenInfoFromContext(ctx)
	if !ok {
		t.Fatal("expected to retrieve token info from context")
	}
	if !retrieved.IsValid {
		t.Error("expected token to be valid")
	}
}

func TestTokenValidationMiddleware(t *testing.T) {
	handler := NewHandler(hclog.NewNullLogger())

	// Create a test handler that checks the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := GetACLTokenInfoFromContext(r.Context())
		if !ok {
			t.Error("expected ACL token info in context")
			return
		}

		// Without a real Nomad cluster, we expect validation to fail
		// but the info should still be present
		if info == nil {
			t.Error("expected non-nil ACL token info")
		}

		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	middleware := handler.TokenValidationMiddleware(testHandler)

	// Create test request with token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(XNomadToken, "test-token")

	// Add ACLMiddleware context first (simulating the middleware chain)
	ctx := WithTokenContext(req.Context(), "test-token")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRequireTokenMiddleware(t *testing.T) {
	handler := NewHandler(hclog.NewNullLogger())

	// Create a test handler that should only be called if auth passes
	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with RequireToken middleware
	middleware := handler.RequireToken(testHandler)

	t.Run("no token info in context", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
		if handlerCalled {
			t.Error("handler should not have been called")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)

		// Add invalid token info to context
		info := &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
		}
		ctx := WithACLTokenInfo(req.Context(), info)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
		if handlerCalled {
			t.Error("handler should not have been called")
		}
	})

	t.Run("valid token", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)

		// Add valid token info to context
		info := &ACLTokenInfo{
			IsValid:    true,
			ACLEnabled: true,
		}
		ctx := WithACLTokenInfo(req.Context(), info)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if !handlerCalled {
			t.Error("handler should have been called")
		}
	})

	t.Run("ACL disabled", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest("GET", "/test", nil)

		// Add token info with ACL disabled
		info := &ACLTokenInfo{
			IsValid:    true,
			ACLEnabled: false,
		}
		ctx := WithACLTokenInfo(req.Context(), info)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if !handlerCalled {
			t.Error("handler should have been called when ACL is disabled")
		}
	})
}

func TestGetACLTokenInfoFromRequest(t *testing.T) {
	// Test with no context
	req := httptest.NewRequest("GET", "/test", nil)
	info := GetACLTokenInfoFromRequest(req)
	if info != nil {
		t.Error("expected nil info from request without context")
	}

	// Test with context
	tokenInfo := &ACLTokenInfo{
		IsValid:    true,
		ACLEnabled: true,
	}
	ctx := WithACLTokenInfo(req.Context(), tokenInfo)
	req = req.WithContext(ctx)

	info = GetACLTokenInfoFromRequest(req)
	if info == nil {
		t.Fatal("expected non-nil info from request with context")
	}
	if !info.IsValid {
		t.Error("expected token to be valid")
	}
}
