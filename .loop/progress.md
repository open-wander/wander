# Progress Log

## Patterns Discovered
(Add reusable patterns here as you find them)

---

## Execution Log

### 2026-01-26 - Project Setup (Feature Parity)
- Created new task list with 70 tasks across 6 phases
- Previous UI modernization project marked complete (127/127 tasks)
- Key focus areas:
  1. ACL middleware (foundation for secure operations)
  2. API integration (replace mock data with real Nomad API)
  3. Write operations (job actions, variable CRUD, etc.)
  4. Missing pages (job submission, deployments)
  5. Real-time updates (polling, SSE)
  6. UX polish (keyboard shortcuts, search, error pages)
- Decided to keep current stack (HTMX + Alpine.js + Templ + Tailwind)
- No React/Vue migration needed - current architecture is sound
---

### 2026-01-26 - Task 1: ACL middleware for token extraction
- Added token extraction from X-Nomad-Token header and nomad_token cookie
- Header takes precedence over cookie (matches Nomad standard)
- Token stored in request context via typed contextKey
- Added helpers: GetTokenFromContext(), WithTokenContext(), GetTokenFromRequest()
- ACL middleware wired into router via wrapMiddleware()
- Added 4 test functions for all extraction scenarios
- Pattern: Non-validating middleware - just extracts, doesn't validate
- Files: context.go, middleware.go, middleware_test.go, router.go
---

### 2026-01-26 - Task 2: Token validation with Nomad ACL API
- Created auth.go with ValidateToken() calling client.ACLTokens().Self()
- Added TokenCache with 2-min TTL, background cleanup goroutine
- Handles: no token (anon), valid, invalid/expired, ACL disabled
- Created TokenValidationMiddleware (optional) and RequireToken (enforcing)
- Uses temporary client per request to avoid concurrency issues
- Added comprehensive auth_test.go with all scenarios
- Pattern: Lazy init with sync.Once, temporary client for validation
- Files: auth.go (368 lines), auth_test.go (186 lines), context.go
---

### 2026-01-26 - Task 3: Store permissions in request context
- Created UserPermissions struct with capability checking methods
- Fast paths: Management tokens and ACL-disabled bypass all checks
- Job capabilities: CanListJobs, CanReadJob, CanSubmitJob, CanScaleJob
- Alloc capabilities: CanReadLogs, CanReadFS, CanExec
- Variable capabilities: CanListVariables, CanReadVariables, CanWriteVariables, CanDestroyVariables
- CSI capabilities: CanReadCSI, CanWriteCSI, CanListCSI
- Namespace pattern matching with wildcard support (prod-*, *)
- Lazy policy loading via LoadPolicies() to avoid latency
- Pattern: Most specific wildcard wins, "deny" takes precedence
- Files: permissions.go (386 lines), permissions_test.go, permissions_example_test.go, PERMISSIONS.md
---

### 2026-01-26 - Task 4: Per-handler permission checks with 403 responses
- Created errors.go with RenderForbidden(), CheckPermission() helpers
- Three response types: full page 403, HTMX partial, JSON error
- Jobs: CanListJobs, CanReadJob, CanSubmitJob, CanScaleJob checks
- Allocations: read-job, read-logs, alloc-exec, read-fs, alloc-lifecycle checks
- Variables: Path-based list/read/write/destroy checks
- Policies: Management token required for all operations
- Clients: node:read and node:write capability checks
- CSI: csi-list-volume and csi-read-volume checks
- Pattern: HTMX partial errors vs full page errors based on request type
- Files: errors.go (new), jobs.go, allocations.go, variables.go, policies.go, clients.go, csi.go
---
