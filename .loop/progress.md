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

### 2026-01-26 - Task 5: Token expiration handling and refresh
- Enhanced ValidateToken() to check expiration before returning cached entries
- Added expiration warning headers: X-Nomad-Token-Expires-Soon, X-Nomad-Token-Expires-At
- Cache TTL respects token expiration (never caches beyond expiry)
- Added helper functions: IsTokenExpired(), IsTokenExpiringSoon(), GetTimeUntilExpiry()
- Added TokenCache.Delete() and TokenCache.Clear() methods
- Created /api/v1/auth/token-info endpoint for token metadata
- Created /api/v1/auth/clear-cache endpoint for cache management
- Expired tokens get specific error message vs invalid tokens
- Pattern: 5-minute warning threshold, cache half remaining lifetime
- Files: auth.go, auth_test.go, router.go
---

### 2026-01-26 - Task 6: Settings page real token management
- Wired Show() handler to use real ValidateToken() from auth.go
- Sign in validates token via Nomad API before setting cookie
- Sign out clears cookie AND clears token from cache
- Cookie: nomad_token, path=/ui/, HttpOnly, SameSite=Lax
- Three-state display: ACL disabled, authenticated, unauthenticated
- Shows token name, accessor ID (truncated), type, policies, expiration
- Expiration warning banner when token expires within 10 minutes
- Pattern: Cookie path matches UI routes for proper scoping
- Files: settings.go, settingspartials/token_form.templ
---

### 2026-01-26 - Task 7: Jobs list real API
- Replaced mock data with client.Jobs().List(queryOpts)
- Query params: namespace, filter, status, type, search, sort, dir, page, per_page
- Builds Nomad filter expressions from status/type params
- Maps api.JobListStub to jobspartials.JobSummary
- Token from context via GetTokenFromRequest()
- Pagination/sorting client-side (Nomad API doesn't paginate jobs)
- Pattern: Filter expression building with "and" combinator
- Files: jobs.go
---

### 2026-01-26 - Task 8: Job detail real API
- Replaced mock data with client.Jobs().Info(jobID, queryOpts)
- Namespace parsing from job ID format (jobID@namespace)
- Maps api.Job to jobspartials.JobDetail with pointer-safe helpers
- Extracts task groups and task names from job structure
- Returns 404 for job not found, 500 for API errors
- Tab content updated to use real API call
- Pattern: Safe pointer dereferencing with safeString()/safeInt()
- Files: jobs.go
---

### 2026-01-26 - Task 9: Job versions tab real API
- Replaced mock getJobVersions with client.Jobs().Versions()
- Extracts version number, stable flag, submit time from api.Job pointers
- Determines active version as highest version number
- Sorts versions descending (newest first)
- Version 0 correctly excludes diff link (no previous)
- ACL token passed via QueryOptions.AuthToken
- Pattern: diffs=false for performance, compute on-demand
- Files: jobs.go
---

### 2026-01-26 - Task 10: Job allocations tab real API
- Replaced mock getJobAllocations with client.Jobs().Allocations()
- Maps api.AllocationListStub to jobspartials.JobAllocation
- Extracts: ID (short), task group, node, client status, resources
- Sums CPU/Memory from AllocatedResources.Tasks map
- Sorts by creation time (newest first)
- Removed mock generation functions (generateMockAllocations, hashString)
- Pattern: allAllocs=true to include completed allocations
- Files: jobs.go
---

### 2026-01-26 - Task 11: Job deployments tab real API
- Implemented getJobDeployments() with client.Jobs().Deployments()
- Maps api.Deployment to jobspartials.JobDeployment
- Shows: ID, status, version, task group states, canaries
- Task group breakdown: desired/placed/healthy/unhealthy counts
- Status badges: running=blue, successful=green, failed=red
- Added Deployments tab to job detail navigation
- Pattern: Only jobs with update stanza have deployments
- Files: jobs.go, tabs.templ, detail.templ
---

### 2026-01-26 - Task 12: Job evaluations tab real API
- Implemented getJobEvaluations() with client.Jobs().Evaluations()
- Maps api.Evaluation to jobspartials.JobEvaluation
- Shows: ID (short), status, triggered by, created time
- Processes placement failures from FailedTGAllocs map
- Status badges: pending=warning, complete=success, failed/blocked=danger
- Formats trigger codes to readable text (job-register, alloc-stop, etc.)
- Sorts evaluations by creation time (newest first)
- Files: jobs.go, tabs.templ
---

### 2026-01-26 - Task 13: Allocations list real API
- Replaced mock data with client.Allocations().List()
- Server-side filtering via Nomad filter expressions
- Filters: status (ClientStatus), job (JobID), node (NodeID/NodeName)
- Namespace and ACL token passed via QueryOptions
- Maps api.AllocationListStub to template structure
- Client-side: search (ID prefix), sort, pagination
- Pattern: Filter expressions combined with "and" operators
- Files: allocations.go
---

### 2026-01-26 - Task 14: Allocation detail real API
- Replaced mock getAllocationDetail with client.Allocations().Info()
- Maps api.Allocation to template with task states and events
- Extracts resources from TaskResources and AllocatedResources.Tasks
- Converts task events with Type, Time, DisplayMessage, FailsTask
- Updated getTasksForAllocation() to use real API
- ACL token passed via QueryOptions.AuthToken
- Note: CPU/Mem usage % requires separate stats endpoint
- Files: allocations.go
---

### 2026-01-26 - Task 15: Allocation file browser real API
- Replaced mock file listing with client.AllocFS().List()
- Added getFileContent() with AllocFS().Stat() and ReadAt()
- File preview limited to 100KB (MaxPreviewSize)
- Detects text vs binary via ContentType and extensions
- Handles .stdout.0/.stderr.0 log file detection
- Directory navigation with HTMX lazy loading
- ACL token passed for authenticated requests
- Files: allocations.go
---

### 2026-01-27 - Task 16: Allocation file content real API
- VERIFIED: Already fully implemented in Task 15
- AllocFS().Stat() for file metadata (size, content type)
- AllocFS().ReadAt() for file content reading (not Cat())
- Binary vs text detection via ContentType and file extensions
- MaxPreviewSize = 100KB for large file truncation
- Log file detection: .stdout.0, .stderr.0
- File download handler with proper MIME types
- ACL token authentication via GetTokenFromRequest()
- No changes needed - task requirement already satisfied
- Pattern: ReadAt() preferred over Cat() for partial reads
- Files: allocations.go (verified, no changes)
---

### 2026-01-27 - Task 17: Allocation log streaming real API
- Replaced mock streamLogsFromNomad() with real client.AllocFS().Logs()
- Updated getTasksForAllocation() to fetch real task names from API
- SSE endpoint: GET /ui/allocations/{id}/logs/stream
- Query params: task (required), type (stdout/stderr), follow, offset, origin
- Uses StreamFrame channels pattern from Nomad SDK
- Heartbeat frame filtering via IsHeartbeat() helper
- Binary data detection with base64 encoding for non-text logs
- Context cancellation via request context for cleanup on disconnect
- ACL token passed via QueryOptions.AuthToken
- Pattern: Nomad streaming uses two channels (frames, errs)
- Files: allocation_logs.go
---

### 2026-01-27 - Task 18: Clients list real API
- Added ACL token integration via GetTokenFromRequest(r)
- QueryOptions.AuthToken for authenticated API calls
- Enhanced composite status from Status + SchedulingEligibility + Drain
- Statuses: ready, down, initializing, disconnected, draining, ineligible
- Template filter dropdown updated with all status options
- Client-side filtering: status, datacenter, node class, search
- Maps NodeListStub to ClientSummary with resource conversion
- Note: Allocations count still 0 (would need separate API call)
- Pattern: Composite status derived by prioritizing special states
- Files: clients.go, list_content.templ, clients_test.go
---

### 2026-01-27 - Tasks 19, 22, 24, 26: Parallel API Integration

**Task 19: Client detail real API**
- Wired client.Nodes().Info(nodeID, queryOpts) for node details
- Added getNodeAllocations() helper for allocations on node
- Maps: attributes, drivers, events, resources, meta
- Version from node.Attributes["nomad.version"]
- Uptime calculated from StatusUpdatedAt timestamp
- Pattern: AllocatedResources.Tasks map for CPU/Memory sums
- Files: clients.go

**Task 22: Servers list (verified already complete)**
- Already uses client.Agent().Members() and Status().Leader()
- Maps member tags: region, datacenter, build, raft_vsn
- Leader detection by comparing addr:port with leader
- Pattern: Agent endpoints don't require ACL tokens
- Files: servers.go (no changes needed)

**Task 24: Evaluations list real API**
- Added ACL token via GetTokenFromRequest + QueryOptions.AuthToken
- Server-side filters: status, type, job via Filter expressions
- Client-side: search across ID, JobID, Status, Type, TriggeredBy
- Sorting by: id, job, status, type, triggered, priority, created
- Pagination: 25 default, max 100 per page
- Time formatting: Unix nano to "time ago" strings
- Pattern: Two-stage filtering (server + client side)
- Files: evaluations.go

**Task 26: Namespaces list real API**
- Added ACL token via GetTokenFromRequest + QueryOptions.AuthToken
- Maps: Name, Description, Quota, CreateIndex, ModifyIndex
- Job count via Jobs().List() per namespace (N+1 pattern)
- Maintains mock fallback when API unavailable
- Pattern: Job counts need separate API calls per namespace
- Files: namespaces.go
---
