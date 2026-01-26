# UI Link Check Results

Generated: 2026-01-26 14:21:00

## Status

⚠️ **BLOCKED** - Cannot run link checker due to compilation errors

## Issue

The link checker cannot be run because there are compilation errors in the UI templates:

1. **templates/partials/topologypartials/list_content_templ.go**: Undefined `components.CircularProgress`
2. **templates/partials/policiespartials/list_content_templ.go**: Incorrect `components.Button` call signature
3. **templates/partials/taskgrouppartials/detail_templ.go**: Incorrect `components.Badge` call signature
4. **templates/partials/variablespartials/detail_content_templ.go**: Unused import and incorrect `components.Button` call
5. **templates/partials/clientspartials/list_content_templ.go**: Function redeclaration issues
6. **templates/partials/jobspartials/detail_templ.go**: Incorrect `components.Badge` call signature
7. **templates/partials/allocationspartials/detail_templ.go**: Incorrect `components.Badge` call signature and unused variable
8. **templates/pages/servers/list_templ.go**: Type mismatch for `ServerSummary`

## Link Checker Implementation

Two implementations have been created:

### 1. Standalone Script (`ui-htmx/scripts/check-links.go`)

Run with:
```bash
# Start dev server first
cd ui-htmx && make dev

# In another terminal:
cd ui-htmx
go run scripts/check-links.go -base-url http://localhost:3000 -output LINK_CHECK_RESULTS.md
```

### 2. Test Suite (`ui-htmx/tests/link_check_test.go`)

Run with:
```bash
cd ui-htmx
go test -v -run TestLinkChecker ./tests/
```

The test automatically starts a test server and crawls all UI routes.

## Features

Both implementations:
- Start from `/ui/` and recursively crawl all internal links
- Check all `href` and `src` attributes
- Report broken links (404s and other errors)
- Report missing assets (CSS, JS, images)
- Track visited URLs to avoid duplicates
- Skip external links, anchors, and WebSocket URLs
- Generate detailed markdown report with:
  - Total pages crawled
  - Total links checked
  - Total assets checked
  - List of broken links (if any)
  - List of missing assets (if any)
  - Summary status

## Next Steps

1. Fix the compilation errors in the templates
2. Run the link checker: `cd ui-htmx && go test -v -run TestLinkChecker ./tests/`
3. Review results in this file
4. Fix any broken links or missing assets found
