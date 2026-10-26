# UI Routes Test Report

## Summary

Created comprehensive route tests in `/ui-htmx/tests/routes_test.go` to verify all UI routes return appropriate status codes.

## Status: BLOCKED

Tests cannot run due to existing compilation errors in the codebase that must be fixed first.

## File Created

- `/Users/stokvis/dev/wander/wander/ui-htmx/tests/routes_test.go` (557 lines, 48 test cases)

## Routes Tested

### Main Routes (Expected 200 OK)

1. **Home/Dashboard**
   - `/ui/` - Dashboard home page

2. **Jobs Routes**
   - `/ui/jobs` - Jobs listing page
   - `/ui/jobs/example-job` - Job detail page
   - `/ui/jobs/example-job/overview` - Job overview tab
   - `/ui/jobs/example-job/definition` - Job definition tab
   - `/ui/jobs/example-job/versions` - Job versions tab
   - `/ui/jobs/example-job/allocations` - Job allocations tab
   - `/ui/jobs/example-job/evaluations` - Job evaluations tab

3. **Allocations Routes**
   - `/ui/allocations` - Allocations listing page
   - `/ui/allocations/abc123` - Allocation detail page
   - `/ui/allocations/abc123/overview` - Allocation overview tab
   - `/ui/allocations/abc123/logs` - Allocation logs tab
   - `/ui/allocations/abc123/exec` - Allocation exec tab

4. **Clients Routes**
   - `/ui/clients` - Clients listing page
   - `/ui/clients/node-1` - Client detail page
   - `/ui/clients/node-1/overview` - Client overview tab
   - `/ui/clients/node-1/allocations` - Client allocations tab
   - `/ui/clients/node-1/events` - Client events tab

5. **Servers Routes**
   - `/ui/servers` - Servers listing page
   - `/ui/servers/server-1` - Server detail page

6. **Topology Routes**
   - `/ui/topology` - Topology visualization page

7. **Variables Routes**
   - `/ui/variables` - Variables listing page
   - `/ui/variables/new` - New variable form
   - `/ui/variables/default/my-var` - Variable detail page

8. **Policies Routes**
   - `/ui/access-control` - ACL management overview
   - `/ui/access-control/policies` - ACL policies listing page
   - `/ui/access-control/policies/admin` - ACL policy detail page

9. **CSI Routes**
   - `/ui/csi/volumes` - CSI volumes listing page
   - `/ui/csi/volumes/vol-1` - CSI volume detail page
   - `/ui/csi/plugins` - CSI plugins listing page

10. **Settings Routes**
    - `/ui/settings` - Settings page

11. **Evaluations Routes**
    - `/ui/evaluations` - Recent evaluations panel

12. **Static Assets**
    - `/ui/static/css/output.css` - Tailwind CSS output file
    - `/ui/static/js/htmx.min.js` - HTMX JavaScript library
    - `/ui/static/js/alpine.min.js` - Alpine.js JavaScript library

### Error Response Tests

1. **Method Not Allowed (405)**
   - POST to `/ui/jobs` - POST method should not be allowed on list page
   - POST to `/ui/static/css/output.css` - POST method should not be allowed on static files

2. **Not Found (404)**
   - GET `/ui/static/nonexistent.css` - Non-existent static file
   - GET `/ui/jobs/test-job/unknown-tab` - Unknown tab

### API Routes (Expected 200 OK)

1. **Job Actions**
   - POST `/api/jobs/test-job/run` - Start/run a job
   - POST `/api/jobs/test-job/stop` - Stop a running job
   - POST `/api/jobs/test-job/purge` - Purge a job

## Test Functions

1. **TestRoutes** - Main test function that verifies all routes return correct status codes
2. **TestRoutesHTMXRequests** - Verifies HTMX requests work correctly and return partial content
3. **TestRoutesContentType** - Verifies correct Content-Type headers are set
4. **TestAPIRoutes** - Verifies API routes for HTMX actions
5. **TestRoutesConcurrency** - Verifies routes can handle concurrent requests

## Compilation Errors Blocking Test Execution

The following compilation errors must be fixed before tests can run:

### 1. ui-htmx/templates/pages/servers/list_templ.go:61
```
cannot use serversList (variable of type []ServerSummary) as []serverspartials.ServerSummary value
```

### 2. ui-htmx/templates/partials/topologypartials/list_content_templ.go:142
```
undefined: components.CircularProgress
undefined: components.CircularProgressProps
```

### 3. ui-htmx/templates/partials/policiespartials/list_content_templ.go:72,75
```
not enough arguments in call to components.Button
unknown field Href in struct literal of type components.ButtonProps
```

### 4. ui-htmx/templates/partials/taskgrouppartials/detail_templ.go:305
```
too many arguments in call to components.Badge
```

### 5. ui-htmx/templates/partials/variablespartials/detail_content_templ.go:15,120
```
"strings" imported and not used
not enough arguments in call to components.Button
```

### 6. ui-htmx/templates/partials/jobspartials/detail_templ.go:77
```
too many arguments in call to components.Badge
```

### 7. ui-htmx/templates/partials/clientspartials/list_content_templ.go:720,728,736,759
```
calculatePercentage redeclared
calculateMemoryPercentage redeclared
formatMemory redeclared
truncateID redeclared
```

### 8. ui-htmx/templates/partials/allocationspartials/detail_templ.go:116
```
too many arguments in call to components.Badge
```

### 9. ui-htmx/templates/partials/allocationspartials/files_templ.go:1119
```
declared and not used: lineStart
```

## Next Steps

1. Fix all compilation errors listed above
2. Run `go test ./ui-htmx/tests -run TestRoutes -v` to execute route tests
3. Verify all routes return expected status codes
4. Address any failing routes

## How to Run Tests (Once Compilation Errors Are Fixed)

```bash
cd ui-htmx
go test ./tests/... -run TestRoutes -v
```

Or run all tests:

```bash
cd ui-htmx
go test ./tests/... -v
```

## Expected Test Output

Once compilation errors are fixed, tests should:
- Verify 40+ routes return 200 OK
- Verify 2+ routes return 405 Method Not Allowed
- Verify 2+ routes return 404 Not Found
- Verify HTMX requests return partial content (no DOCTYPE)
- Verify correct Content-Type headers
- Verify API routes work correctly
- Verify concurrent request handling
