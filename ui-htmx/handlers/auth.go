package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-wander/wander/api"
)

// Context key for ACL token information
const (
	aclTokenInfoKey contextKey = "acl_token_info"
)

// ACLTokenInfo contains validated ACL token information
type ACLTokenInfo struct {
	Token      *api.ACLToken
	IsValid    bool
	ACLEnabled bool
	Error      error
}

// TokenCache provides simple in-memory caching for validated tokens
type TokenCache struct {
	mu      sync.RWMutex
	entries map[string]*tokenCacheEntry
}

type tokenCacheEntry struct {
	info      *ACLTokenInfo
	expiresAt time.Time
}

// Global token cache - shared across all requests
var (
	tokenCache     *TokenCache
	tokenCacheOnce sync.Once
)

// getTokenCache returns the singleton token cache instance
func getTokenCache() *TokenCache {
	tokenCacheOnce.Do(func() {
		tokenCache = &TokenCache{
			entries: make(map[string]*tokenCacheEntry),
		}
		// Start background cleanup goroutine
		go tokenCache.cleanup()
	})
	return tokenCache
}

// Get retrieves a cached token validation result
func (c *TokenCache) Get(token string) (*ACLTokenInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[token]
	if !ok {
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.info, true
}

// Set stores a token validation result in the cache
func (c *TokenCache) Set(token string, info *ACLTokenInfo, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[token] = &tokenCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(ttl),
	}
}

// cleanup periodically removes expired entries from the cache
func (c *TokenCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for token, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, token)
			}
		}
		c.mu.Unlock()
	}
}

// ValidateToken validates a Nomad ACL token by calling the Nomad API.
// It handles the following cases:
// - No token provided: Returns ACLTokenInfo with IsValid=false, ACLEnabled=unknown
// - Valid token: Returns ACLTokenInfo with the token details
// - Invalid/expired token: Returns ACLTokenInfo with IsValid=false and error
// - ACL disabled: Returns ACLTokenInfo with ACLEnabled=false, IsValid=true
//
// The function uses a temporary client to avoid concurrent access issues with
// the shared client instance. Results are cached briefly to reduce API calls.
func (h *Handler) ValidateToken(ctx context.Context, token string) *ACLTokenInfo {
	logger := h.logger.With("func", "ValidateToken")

	// If no token provided, return default info
	if token == "" {
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true, // Assume ACL is enabled until we know otherwise
		}
	}

	// Check cache first
	cache := getTokenCache()
	if cached, ok := cache.Get(token); ok {
		logger.Debug("token validation cache hit")
		return cached
	}

	// If no Nomad client available, we can't validate
	if h.nomadClient == nil {
		logger.Warn("no Nomad client available for token validation")
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("Nomad client not configured"),
		}
	}

	// Create a new client with default config
	// This avoids concurrency issues with the shared client
	config := api.DefaultConfig()
	tempClient, err := api.NewClient(config)
	if err != nil {
		logger.Error("failed to create temporary client for token validation", "error", err)
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("failed to create API client: %w", err),
		}
	}

	// Set the token on the temporary client
	tempClient.SetSecretID(token)

	// Call the Self endpoint to validate the token
	aclToken, _, err := tempClient.ACLTokens().Self(nil)
	if err != nil {
		// Check for specific error messages
		errMsg := err.Error()

		// ACL support disabled - this is actually OK, allow the request
		if strings.Contains(errMsg, "ACL support disabled") {
			info := &ACLTokenInfo{
				IsValid:    true,
				ACLEnabled: false,
			}
			// Cache this result with a longer TTL since ACL state rarely changes
			cache.Set(token, info, 5*time.Minute)
			logger.Debug("ACL support is disabled on cluster")
			return info
		}

		// ACL token not found or invalid
		if strings.Contains(errMsg, "ACL token not found") ||
			strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "expired") {
			info := &ACLTokenInfo{
				IsValid:    false,
				ACLEnabled: true,
				Error:      fmt.Errorf("invalid or expired token"),
			}
			// Cache negative results briefly to avoid repeated invalid token checks
			cache.Set(token, info, 30*time.Second)
			logger.Debug("token validation failed", "error", errMsg)
			return info
		}

		// Other errors (network, etc.)
		logger.Warn("token validation error", "error", err)
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("token validation failed: %w", err),
		}
	}

	// Token is valid
	info := &ACLTokenInfo{
		Token:      aclToken,
		IsValid:    true,
		ACLEnabled: true,
	}

	// Determine cache TTL based on token expiration
	cacheTTL := 2 * time.Minute // Default TTL
	if aclToken.ExpirationTime != nil {
		// If token expires soon, use shorter cache TTL
		timeUntilExpiry := time.Until(*aclToken.ExpirationTime)
		if timeUntilExpiry > 0 && timeUntilExpiry < cacheTTL {
			cacheTTL = timeUntilExpiry / 2 // Cache for half the remaining time
		}
	}

	cache.Set(token, info, cacheTTL)
	logger.Debug("token validated successfully",
		"accessor_id", aclToken.AccessorID,
		"name", aclToken.Name,
		"type", aclToken.Type,
		"cache_ttl", cacheTTL)

	return info
}

// GetACLTokenInfoFromContext retrieves the validated ACL token info from the request context.
// Returns the ACLTokenInfo and true if found, nil and false otherwise.
func GetACLTokenInfoFromContext(ctx context.Context) (*ACLTokenInfo, bool) {
	info, ok := ctx.Value(aclTokenInfoKey).(*ACLTokenInfo)
	return info, ok
}

// WithACLTokenInfo adds ACL token info to the context.
// This is primarily used for testing.
func WithACLTokenInfo(ctx context.Context, info *ACLTokenInfo) context.Context {
	return context.WithValue(ctx, aclTokenInfoKey, info)
}

// TokenValidationMiddleware is an optional middleware that validates ACL tokens.
// It extracts the token from the context (set by ACLMiddleware), validates it
// via the Nomad API, and stores the validation result in the context.
//
// This middleware does NOT block requests - it only validates and stores the result.
// Individual handlers can check the validation result and enforce their own
// authorization policies.
func (h *Handler) TokenValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get token from context (set by ACLMiddleware)
		token, hasToken := GetTokenFromContext(ctx)

		// Validate the token (or check if ACL is disabled)
		var tokenInfo *ACLTokenInfo
		if hasToken {
			tokenInfo = h.ValidateToken(ctx, token)
		} else {
			// No token provided - create default info
			tokenInfo = &ACLTokenInfo{
				IsValid:    false,
				ACLEnabled: true, // Assume enabled until proven otherwise
			}
		}

		// Store validation result in context for handlers
		ctx = context.WithValue(ctx, aclTokenInfoKey, tokenInfo)

		// Create and store user permissions for easy capability checking
		permissions := NewUserPermissions(tokenInfo)
		ctx = WithPermissions(ctx, permissions)

		// Log validation result for debugging
		if tokenInfo.IsValid {
			if tokenInfo.Token != nil {
				h.logger.Debug("request authenticated",
					"method", r.Method,
					"path", r.URL.Path,
					"token_name", tokenInfo.Token.Name,
					"token_type", tokenInfo.Token.Type)
			} else {
				h.logger.Debug("request authenticated (ACL disabled)",
					"method", r.Method,
					"path", r.URL.Path)
			}
		} else if !hasToken {
			h.logger.Debug("unauthenticated request (no token)",
				"method", r.Method,
				"path", r.URL.Path)
		} else if tokenInfo.Error != nil {
			h.logger.Debug("authentication failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", tokenInfo.Error)
		}

		// Continue to next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireToken is a middleware that enforces token authentication.
// It checks the ACL token info from context and returns 401 if:
// - ACL is enabled and no valid token is present
// - ACL is enabled and token validation failed
//
// This middleware should be applied to routes that require authentication.
func (h *Handler) RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get ACL token info from context (set by TokenValidationMiddleware)
		tokenInfo, hasInfo := GetACLTokenInfoFromContext(ctx)

		// If no token info, validation middleware wasn't run
		if !hasInfo {
			h.logger.Warn("RequireToken middleware used without TokenValidationMiddleware",
				"method", r.Method,
				"path", r.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// If ACL is disabled, allow through
		if !tokenInfo.ACLEnabled {
			next.ServeHTTP(w, r)
			return
		}

		// If token is not valid, return 401
		if !tokenInfo.IsValid {
			h.logger.Debug("rejecting request: token validation failed",
				"method", r.Method,
				"path", r.URL.Path,
				"error", tokenInfo.Error)

			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, "Unauthorized: Invalid or missing ACL token", http.StatusUnauthorized)
			return
		}

		// Token is valid, continue
		next.ServeHTTP(w, r)
	})
}
