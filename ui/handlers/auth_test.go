package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
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

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name     string
		info     *ACLTokenInfo
		expected bool
	}{
		{
			name:     "nil info",
			info:     nil,
			expected: false,
		},
		{
			name: "no token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
			},
			expected: false,
		},
		{
			name: "no expiration time",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID: "test-accessor",
				},
			},
			expected: false,
		},
		{
			name: "expired token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(-1 * time.Hour)),
				},
			},
			expected: true,
		},
		{
			name: "valid token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(1 * time.Hour)),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTokenExpired(tt.info)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsTokenExpiringSoon(t *testing.T) {
	tests := []struct {
		name     string
		info     *ACLTokenInfo
		within   time.Duration
		expected bool
	}{
		{
			name:     "nil info",
			info:     nil,
			within:   5 * time.Minute,
			expected: false,
		},
		{
			name: "no token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
			},
			within:   5 * time.Minute,
			expected: false,
		},
		{
			name: "expires in 2 minutes (within 5)",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(2 * time.Minute)),
				},
			},
			within:   5 * time.Minute,
			expected: true,
		},
		{
			name: "expires in 10 minutes (not within 5)",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(10 * time.Minute)),
				},
			},
			within:   5 * time.Minute,
			expected: false,
		},
		{
			name: "already expired",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(-1 * time.Hour)),
				},
			},
			within:   5 * time.Minute,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTokenExpiringSoon(tt.info, tt.within)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetTimeUntilExpiry(t *testing.T) {
	tests := []struct {
		name     string
		info     *ACLTokenInfo
		validate func(time.Duration) bool
	}{
		{
			name: "nil info",
			info: nil,
			validate: func(d time.Duration) bool {
				return d == 0
			},
		},
		{
			name: "no token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
			},
			validate: func(d time.Duration) bool {
				return d == 0
			},
		},
		{
			name: "expired token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(-1 * time.Hour)),
				},
			},
			validate: func(d time.Duration) bool {
				return d == 0
			},
		},
		{
			name: "valid token",
			info: &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: true,
				Token: &api.ACLToken{
					AccessorID:     "test-accessor",
					ExpirationTime: timePtr(time.Now().Add(1 * time.Hour)),
				},
			},
			validate: func(d time.Duration) bool {
				// Should be approximately 1 hour (with some tolerance for execution time)
				return d > 59*time.Minute && d <= 60*time.Minute
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeUntilExpiry(tt.info)
			if !tt.validate(result) {
				t.Errorf("time until expiry validation failed: got %v", result)
			}
		})
	}
}

func TestTokenCacheDelete(t *testing.T) {
	cache := getTokenCache()

	token := "test-token-delete"
	info := &ACLTokenInfo{
		IsValid:    true,
		ACLEnabled: true,
	}

	// Add to cache
	cache.Set(token, info, 5*time.Minute)

	// Verify it's in cache
	_, ok := cache.Get(token)
	if !ok {
		t.Fatal("expected token to be in cache")
	}

	// Delete it
	cache.Delete(token)

	// Verify it's gone
	_, ok = cache.Get(token)
	if ok {
		t.Error("expected token to be deleted from cache")
	}
}

func TestTokenCacheClear(t *testing.T) {
	cache := getTokenCache()

	// Add multiple tokens
	for i := 0; i < 5; i++ {
		token := "test-token-" + string(rune('0'+i))
		info := &ACLTokenInfo{
			IsValid:    true,
			ACLEnabled: true,
		}
		cache.Set(token, info, 5*time.Minute)
	}

	// Clear all
	cache.Clear()

	// Verify all are gone
	for i := 0; i < 5; i++ {
		token := "test-token-" + string(rune('0'+i))
		_, ok := cache.Get(token)
		if ok {
			t.Errorf("expected token %s to be cleared from cache", token)
		}
	}
}

func TestRequireTokenWithExpiredToken(t *testing.T) {
	handler := NewHandler(hclog.NewNullLogger())

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.RequireToken(testHandler)

	t.Run("expired token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		// Add expired token info to context
		info := &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("token has expired"),
		}
		ctx := WithACLTokenInfo(req.Context(), info)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		// Check for specific expired token message
		body := w.Body.String()
		if !strings.Contains(body, "expired") {
			t.Errorf("expected error message to mention expiration, got: %s", body)
		}
	})
}

// Helper function to create a time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
