# Ember Token Service vs Go Implementation Comparison

## Overview
This document compares the old Ember.js token validation approach with the new Go implementation.

## Ember.js Implementation

### Token Storage
- **Location**: `window.localStorage.nomadTokenSecret`
- **Access**: Via computed property getter/setter
- **Persistence**: Browser local storage

### Token Validation
- **Method**: `fetchSelfToken` task
- **Endpoint**: `/v1/acl/token/self` via `TokenAdapter.findSelf()`
- **Error Handling**:
  - "ACL support disabled" → Sets `aclEnabled = false`
  - "ACL token not found" → Sets `tokenNotFound = true`
  - Other errors → Returns null

### Token Expiration
- **Monitoring**: `monitorTokenTime` task runs every 1 second
- **Warning**: Shows notification at 10 minutes before expiration
- **Actions**: Allows user to re-authenticate via settings

### Request Authentication
```javascript
authorizedRequest(url, options) {
  const token = this.secret;
  if (token) {
    headers['X-Nomad-Token'] = token;
  }
  return fetch(url, { headers, credentials: 'include' });
}
```

## Go Implementation

### Token Storage
- **Location**: Request context (per-request)
- **Access**: Via context helpers
- **Persistence**: Not handled by backend (client responsibility)

### Token Validation
- **Method**: `Handler.ValidateToken(ctx, token)`
- **Endpoint**: `client.ACLTokens().Self(nil)`
- **Error Handling**:
  - "ACL support disabled" → `ACLEnabled = false, IsValid = true`
  - "ACL token not found" → `IsValid = false, Error = "invalid or expired token"`
  - Other errors → `IsValid = false` with error details

### Token Caching
- **Storage**: In-memory `TokenCache`
- **TTL**: 2 minutes default, adjusted based on token expiration
- **Cleanup**: Background goroutine removes expired entries every minute

### Request Authentication
```go
// Extract from header or cookie
token := extractToken(r)

// Validate (with caching)
tokenInfo := h.ValidateToken(ctx, token)

// Store in context for handlers
ctx = context.WithValue(ctx, aclTokenInfoKey, tokenInfo)
```

## Key Differences

### Architecture
| Aspect | Ember.js | Go |
|--------|----------|-----|
| Storage | Browser local storage | Request context |
| Validation | On-demand task | Middleware + cache |
| Expiration | Active monitoring (1s polls) | Passive (checked during validation) |
| Scope | Global service | Per-request |

### Error Handling
| Error | Ember.js | Go |
|-------|----------|-----|
| ACL disabled | Set flag, continue | Set flag in ACLTokenInfo, allow request |
| Invalid token | Set flag, return null | Return ACLTokenInfo with error |
| Network error | Return null | Return ACLTokenInfo with error |
| Missing token | No validation | Return default ACLTokenInfo |

### Performance
| Feature | Ember.js | Go |
|---------|----------|-----|
| Caching | Model cache | Custom TTL cache |
| Validation | Every route transition | Per-request (with cache) |
| Monitoring | Active (1s interval) | Passive (on validation) |
| API calls | Multiple (token + policies) | Single (token only) |

## Feature Parity

### Implemented ✓
- [x] Token extraction from headers/cookies
- [x] Token validation via Nomad API
- [x] ACL disabled detection
- [x] Invalid/expired token handling
- [x] Caching validation results
- [x] Context-based token passing

### Not Implemented (Future)
- [ ] Token expiration monitoring (UI concern)
- [ ] Policy fetching (separate task)
- [ ] Token persistence (client-side concern)
- [ ] One-time token exchange
- [ ] JWT login integration
- [ ] Active expiration warnings

## Migration Notes

### What Changed
1. **Storage**: Token no longer stored server-side (stateless)
2. **Validation**: Happens in middleware, not in handlers
3. **Monitoring**: Client must handle expiration warnings
4. **Caching**: Server-side in-memory cache instead of model cache

### What Stayed the Same
1. **API endpoint**: Still uses `/v1/acl/token/self`
2. **Token header**: Still uses `X-Nomad-Token`
3. **Cookie fallback**: Still supports `nomad_token` cookie
4. **ACL detection**: Still detects "ACL support disabled"
5. **Error messages**: Similar error handling patterns

### Integration Points

#### Client-Side Responsibilities
- Store token in local storage or cookie
- Send token in `X-Nomad-Token` header or `nomad_token` cookie
- Monitor token expiration (check `ExpirationTime` field)
- Show expiration warnings to user
- Handle 401 responses and redirect to login

#### Server-Side Responsibilities
- Extract token from request
- Validate token via Nomad API (optional)
- Store validation result in context
- Enforce authentication on protected routes (optional)
- Return 401 for invalid tokens (when enforcing)

## Recommendations

### For UI Developers
1. Store token in cookie for automatic inclusion in requests
2. Check `ExpirationTime` from token info and show warnings
3. Handle 401 responses and redirect to token settings
4. Use WebSocket for long-lived connections with token refresh

### For Backend Developers
1. Enable `TokenValidationMiddleware` for production
2. Use `RequireToken` on sensitive routes (jobs CRUD, variables, etc.)
3. Consider namespace-level authorization checks
4. Add audit logging for authentication failures
5. Monitor cache hit rate and adjust TTL if needed

### For Operators
1. Enable ACL in Nomad configuration
2. Configure token TTL appropriately (not too short)
3. Monitor authentication failures
4. Set up token rotation policies
5. Use management tokens for admin operations
