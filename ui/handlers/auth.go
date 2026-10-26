package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-wander/wander/api"
)

// Context key for ACL token information
const (
	aclTokenInfoKey contextKey = "acl_token_info" // #nosec G101 -- request-context key name, not a credential.
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

// Delete removes a specific token from the cache
func (c *TokenCache) Delete(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, token)
}

// Clear removes all entries from the cache
func (c *TokenCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*tokenCacheEntry)
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
//
// Token expiration is checked both at API validation time and when serving from cache.
// Tokens that are already expired will be marked as invalid.
func (h *Handler) ValidateToken(ctx context.Context, token string) *ACLTokenInfo {
	logger := h.logger.With("func", "ValidateToken")
	cacheKey := token
	if token == "" {
		address := ""
		if h.nomadClient != nil {
			address = h.nomadClient.Address()
		}
		cacheKey = "\x00anonymous:" + address
	}

	// Check cache first
	cache := getTokenCache()
	if cached, ok := cache.Get(cacheKey); ok {
		logger.Debug("token validation cache hit")

		// Double-check token expiration even for cached tokens
		// This ensures we don't serve expired tokens from cache
		if cached.Token != nil && cached.Token.ExpirationTime != nil {
			if time.Now().After(*cached.Token.ExpirationTime) {
				logger.Debug("cached token has expired",
					"expiration_time", cached.Token.ExpirationTime.Format(time.RFC3339))
				return &ACLTokenInfo{
					IsValid:    false,
					ACLEnabled: true,
					Error:      fmt.Errorf("token has expired"),
				}
			}
		}

		return cached
	}

	// If no Nomad client available, we can't validate
	if h.nomadClient == nil {
		logger.Warn("no Nomad client available for token validation")
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("Wander client not configured"),
		}
	}

	// Create a request-scoped client that retains the handler's API endpoint.
	// This avoids mutating the shared client configuration.
	tempClient, err := h.apiClientForToken(token)
	if err != nil {
		logger.Error("failed to create temporary client for token validation", "error", err)
		return &ACLTokenInfo{
			IsValid:    false,
			ACLEnabled: true,
			Error:      fmt.Errorf("failed to create API client: %w", err),
		}
	}

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
			cache.Set(cacheKey, info, 5*time.Minute)
			logger.Debug("ACL support is disabled on cluster")
			return info
		}

		// ACL token not found or invalid
		if strings.Contains(errMsg, "ACL token not found") ||
			strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "expired") ||
			(token == "" && (strings.Contains(errMsg, "Permission denied") ||
				strings.Contains(errMsg, "missing secret id"))) {
			info := &ACLTokenInfo{
				IsValid:    false,
				ACLEnabled: true,
				Error:      fmt.Errorf("a valid ACL token is required"),
			}
			// Cache negative results briefly to avoid repeated invalid token checks
			cache.Set(cacheKey, info, 30*time.Second)
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

	// Check if token has already expired
	if aclToken.ExpirationTime != nil {
		if time.Now().After(*aclToken.ExpirationTime) {
			logger.Debug("token has expired",
				"accessor_id", aclToken.AccessorID,
				"expiration_time", aclToken.ExpirationTime.Format(time.RFC3339))
			info := &ACLTokenInfo{
				IsValid:    false,
				ACLEnabled: true,
				Token:      aclToken, // Include token info for expiration time
				Error:      fmt.Errorf("token has expired"),
			}
			// Cache expired token briefly to avoid repeated checks
			cache.Set(token, info, 30*time.Second)
			return info
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

			// For very short-lived tokens, ensure minimum cache time
			if cacheTTL < 10*time.Second {
				cacheTTL = 10 * time.Second
			}
		}
	}

	cache.Set(token, info, cacheTTL)
	logger.Debug("token validated successfully",
		"accessor_id", aclToken.AccessorID,
		"name", aclToken.Name,
		"type", aclToken.Type,
		"cache_ttl", cacheTTL,
		"has_expiration", aclToken.ExpirationTime != nil)

	return info
}

func (h *Handler) apiClientForToken(token string) (*api.Client, error) {
	if h.nomadClient == nil {
		return nil, fmt.Errorf("Wander client not configured")
	}
	client := h.nomadClient.WithNamespaceAndRegion("", "")
	client.SetSecretID(token)
	return client, nil
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

// IsTokenExpired checks if the ACL token has expired.
// Returns true if the token has an expiration time and it has passed.
func IsTokenExpired(info *ACLTokenInfo) bool {
	if info == nil || info.Token == nil || info.Token.ExpirationTime == nil {
		return false
	}
	return time.Now().After(*info.Token.ExpirationTime)
}

// IsTokenExpiringSoon checks if the ACL token will expire within the specified duration.
// Returns true if the token has an expiration time and it will expire within the duration.
func IsTokenExpiringSoon(info *ACLTokenInfo, within time.Duration) bool {
	if info == nil || info.Token == nil || info.Token.ExpirationTime == nil {
		return false
	}
	timeUntilExpiry := time.Until(*info.Token.ExpirationTime)
	return timeUntilExpiry > 0 && timeUntilExpiry < within
}

// GetTimeUntilExpiry returns the time until the token expires.
// Returns 0 if the token has no expiration time or has already expired.
func GetTimeUntilExpiry(info *ACLTokenInfo) time.Duration {
	if info == nil || info.Token == nil || info.Token.ExpirationTime == nil {
		return 0
	}
	duration := time.Until(*info.Token.ExpirationTime)
	if duration < 0 {
		return 0
	}
	return duration
}

// ClearTokenCache is a handler that clears the token validation cache.
// This can be useful to force token re-validation, for example after
// a token has been renewed or when troubleshooting authentication issues.
//
// POST /api/v1/auth/clear-cache
func (h *Handler) ClearTokenCache(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cache := getTokenCache()

	token := GetTokenFromRequest(r)
	if token == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	cache.Delete(token)
	h.logger.Info("cleared token from cache",
		"method", r.Method,
		"path", r.URL.Path)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Token cache entry cleared"))
}

// GetTokenInfo is a handler that returns information about the current token.
// This includes expiration time, time until expiry, and other metadata.
//
// GET /api/v1/auth/token-info
func (h *Handler) GetTokenInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	// Get ACL token info from context
	tokenInfo, hasInfo := GetACLTokenInfoFromContext(ctx)
	if !hasInfo {
		http.Error(w, "No token information available", http.StatusUnauthorized)
		return
	}

	// Build response
	response := map[string]interface{}{
		"is_valid":    tokenInfo.IsValid,
		"acl_enabled": tokenInfo.ACLEnabled,
	}

	if tokenInfo.Error != nil {
		response["error"] = tokenInfo.Error.Error()
	}

	if tokenInfo.Token != nil {
		response["accessor_id"] = tokenInfo.Token.AccessorID
		response["name"] = tokenInfo.Token.Name
		response["type"] = tokenInfo.Token.Type
		response["global"] = tokenInfo.Token.Global

		if tokenInfo.Token.ExpirationTime != nil {
			response["expiration_time"] = tokenInfo.Token.ExpirationTime.Format(time.RFC3339)
			response["is_expired"] = IsTokenExpired(tokenInfo)
			response["expires_soon"] = IsTokenExpiringSoon(tokenInfo, 5*time.Minute)

			if timeUntilExpiry := GetTimeUntilExpiry(tokenInfo); timeUntilExpiry > 0 {
				response["time_until_expiry_seconds"] = int(timeUntilExpiry.Seconds())
			}
		}

		// Include policies
		if len(tokenInfo.Token.Policies) > 0 {
			response["policies"] = tokenInfo.Token.Policies
		}

		// Include roles
		if len(tokenInfo.Token.Roles) > 0 {
			roles := make([]map[string]string, len(tokenInfo.Token.Roles))
			for i, role := range tokenInfo.Token.Roles {
				roles[i] = map[string]string{
					"id":   role.ID,
					"name": role.Name,
				}
			}
			response["roles"] = roles
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode token info response", "error", err)
	}
}

// TokenValidationMiddleware is an optional middleware that validates ACL tokens.
// It extracts the token from the context (set by ACLMiddleware), validates it
// via the Nomad API, and stores the validation result in the context.
//
// This middleware does NOT block requests - it only validates and stores the result.
// Individual handlers can check the validation result and enforce their own
// authorization policies.
//
// Token expiration handling:
// - If token expires within 5 minutes, adds X-Nomad-Token-Expires-Soon header
// - Frontend can use this to prompt user to re-authenticate
// - Expired tokens are rejected during validation
func (h *Handler) TokenValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get token from context (set by ACLMiddleware)
		token, hasToken := GetTokenFromContext(ctx)

		// Validate the token (or check if ACL is disabled)
		var tokenInfo *ACLTokenInfo
		tokenInfo = h.ValidateToken(ctx, token)

		// Store validation result in context for handlers
		ctx = context.WithValue(ctx, aclTokenInfoKey, tokenInfo)

		// Create and store user permissions for easy capability checking
		permissions := NewUserPermissions(tokenInfo)
		if hasToken && tokenInfo.IsValid && !permissions.IsManagement() && tokenInfo.ACLEnabled {
			client, err := h.apiClientForToken(token)
			if err != nil {
				h.logger.Warn("failed to create ACL policy client", "error", err)
			} else if err := permissions.LoadPolicies(ctx, client); err != nil {
				h.logger.Warn("failed to load ACL policies", "error", err)
			}
		}
		ctx = WithPermissions(ctx, permissions)

		// Check for token expiration warnings
		// If token expires within 5 minutes, add warning header
		if tokenInfo.IsValid && tokenInfo.Token != nil && tokenInfo.Token.ExpirationTime != nil {
			timeUntilExpiry := time.Until(*tokenInfo.Token.ExpirationTime)
			if timeUntilExpiry > 0 && timeUntilExpiry < 5*time.Minute {
				w.Header().Set("X-Nomad-Token-Expires-Soon", "true")
				w.Header().Set("X-Nomad-Token-Expires-At", tokenInfo.Token.ExpirationTime.Format(time.RFC3339))
				h.logger.Debug("token expires soon",
					"method", r.Method,
					"path", r.URL.Path,
					"time_until_expiry", timeUntilExpiry.String())
			}
		}

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
// For expired tokens, it provides a clear error message suggesting re-authentication.
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

			// Provide specific error message for expired tokens
			if tokenInfo.Error != nil && strings.Contains(tokenInfo.Error.Error(), "expired") {
				http.Error(w, "Unauthorized: Your ACL token has expired. Please obtain a new token to continue.", http.StatusUnauthorized)
			} else {
				http.Error(w, "Unauthorized: Invalid or missing ACL token", http.StatusUnauthorized)
			}
			return
		}

		// Token is valid, continue
		next.ServeHTTP(w, r)
	})
}
