# UI-HTMX Performance Metrics

## Overview

This document tracks performance metrics for the UI-HTMX implementation. We measure three key web vitals:

- **TTFB (Time To First Byte)**: Time from request to first byte received
- **FCP (First Contentful Paint)**: Time until first content is rendered
- **TTI (Time To Interactive)**: Time until page is fully interactive

## Performance Targets

| Metric | Target | Description |
|--------|--------|-------------|
| TTFB | < 100ms | Server response time |
| FCP | < 500ms | First visual content |
| TTI | < 1s | Fully interactive |

## Methodology

### TTFB Measurement (Automated)

TTFB is measured using curl's `time_starttransfer` metric, which represents the time from request start until the first byte is received from the server.

**Script**: `scripts/measure-performance.sh`

**Usage**:
```bash
cd ui-htmx
./scripts/measure-performance.sh [base_url]

# Example
./scripts/measure-performance.sh http://localhost:3000
```

**How it works**:
1. Makes 3 requests to each page/asset
2. Measures TTFB for each request
3. Reports average, minimum, and maximum TTFB
4. Records content size and HTTP status
5. Saves results to `test-results/performance-results-*.txt`

**Prerequisites**:
- Dev server must be running on the target URL
- `curl` and `bc` commands must be available

### FCP & TTI Measurement (Manual)

FCP and TTI require browser-based measurement tools:

#### Option 1: Chrome DevTools (Manual)

1. Open Chrome DevTools (F12)
2. Go to the **Performance** tab
3. Click **Record** (Ctrl+E)
4. Navigate to the page
5. Stop recording
6. Review the timeline for:
   - **FCP**: First Contentful Paint marker
   - **TTI**: Time to Interactive marker

#### Option 2: Lighthouse (Automated)

**Command-line**:
```bash
# Install Lighthouse
npm install -g lighthouse

# Run Lighthouse
lighthouse http://localhost:3000/ui/ \
  --output html \
  --output-path ./test-results/lighthouse-home.html \
  --chrome-flags="--headless"

# Test multiple pages
for page in "" "jobs" "allocations" "clients" "servers"; do
  lighthouse "http://localhost:3000/ui/$page" \
    --output html \
    --output-path "./test-results/lighthouse-$page.html" \
    --chrome-flags="--headless"
done
```

**Chrome DevTools UI**:
1. Open Chrome DevTools (F12)
2. Go to the **Lighthouse** tab
3. Select **Performance** category
4. Click **Analyze page load**

#### Option 3: Playwright Performance Test

Create automated performance tests using Playwright:

**File**: `tests/e2e/performance_test.go`

```go
package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

type PerformanceMetrics struct {
	TTFB        float64 `json:"ttfb"`
	FCP         float64 `json:"fcp"`
	LCP         float64 `json:"lcp"`
	TTI         float64 `json:"tti"`
	TBT         float64 `json:"tbt"`
	CLS         float64 `json:"cls"`
	LoadTime    float64 `json:"loadTime"`
	DOMComplete float64 `json:"domComplete"`
}

func TestPerformanceMetrics(t *testing.T) {
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	// Test pages
	pages := []string{
		"/ui/",
		"/ui/jobs",
		"/ui/allocations",
		"/ui/clients",
		"/ui/servers",
	}

	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			metrics := measurePagePerformance(t, browser, "http://localhost:3000"+page)

			t.Logf("Performance metrics for %s:", page)
			t.Logf("  TTFB: %.2fms", metrics.TTFB)
			t.Logf("  FCP: %.2fms", metrics.FCP)
			t.Logf("  TTI: %.2fms", metrics.TTI)
			t.Logf("  Load Time: %.2fms", metrics.LoadTime)

			// Assert performance targets
			if metrics.TTFB > 100 {
				t.Errorf("TTFB %.2fms exceeds target of 100ms", metrics.TTFB)
			}
			if metrics.FCP > 500 {
				t.Errorf("FCP %.2fms exceeds target of 500ms", metrics.FCP)
			}
			if metrics.TTI > 1000 {
				t.Errorf("TTI %.2fms exceeds target of 1000ms", metrics.TTI)
			}
		})
	}
}

func measurePagePerformance(t *testing.T, browser playwright.Browser, url string) *PerformanceMetrics {
	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("could not create page: %v", err)
	}
	defer page.Close()

	// Navigate and wait for load
	startTime := time.Now()
	if _, err := page.Goto(url); err != nil {
		t.Fatalf("could not goto: %v", err)
	}

	// Wait for page to be fully loaded
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateLoad); err != nil {
		t.Fatalf("page did not load: %v", err)
	}

	// Get Performance Navigation Timing
	perfData, err := page.Evaluate(`() => {
		const perfData = performance.getEntriesByType('navigation')[0];
		const paintData = performance.getEntriesByType('paint');

		let fcp = 0;
		for (const entry of paintData) {
			if (entry.name === 'first-contentful-paint') {
				fcp = entry.startTime;
				break;
			}
		}

		return {
			ttfb: perfData.responseStart - perfData.requestStart,
			fcp: fcp,
			loadTime: perfData.loadEventEnd - perfData.fetchStart,
			domComplete: perfData.domComplete - perfData.fetchStart,
		};
	}`)
	if err != nil {
		t.Fatalf("could not get performance data: %v", err)
	}

	// Parse performance data
	var metrics PerformanceMetrics
	data, _ := json.Marshal(perfData)
	if err := json.Unmarshal(data, &metrics); err != nil {
		t.Fatalf("could not parse performance data: %v", err)
	}

	// Rough TTI approximation (domComplete is a reasonable proxy)
	metrics.TTI = metrics.DOMComplete

	return &metrics
}
```

**Running Playwright tests**:
```bash
cd ui-htmx/tests/e2e
go test -v -run TestPerformanceMetrics
```

## Current Results

### TTFB Results

Last measured: [Run `scripts/measure-performance.sh` to populate]

| Page | Avg TTFB | Min TTFB | Max TTFB | Size | Status |
|------|----------|----------|----------|------|--------|
| /ui/ | - | - | - | - | - |
| /ui/jobs | - | - | - | - | - |
| /ui/allocations | - | - | - | - | - |
| /ui/clients | - | - | - | - | - |
| /ui/servers | - | - | - | - | - |
| /ui/static/css/output.css | - | - | - | - | - |
| /ui/static/js/htmx.min.js | - | - | - | - | - |
| /ui/static/js/alpine.min.js | - | - | - | - | - |

### Browser-Based Metrics

Last measured: [Manual measurement required]

| Page | FCP | TTI | LCP | Notes |
|------|-----|-----|-----|-------|
| /ui/ | - | - | - | - |
| /ui/jobs | - | - | - | - |
| /ui/allocations | - | - | - | - |
| /ui/clients | - | - | - | - |
| /ui/servers | - | - | - | - |

## Performance Analysis

### Target Comparison

| Metric | Target | Actual | Status | Notes |
|--------|--------|--------|--------|-------|
| TTFB | < 100ms | - | ⏳ Pending | Run performance test |
| FCP | < 500ms | - | ⏳ Pending | Use Lighthouse or Chrome DevTools |
| TTI | < 1s | - | ⏳ Pending | Use Lighthouse or Chrome DevTools |

**Legend**:
- ✅ **Pass**: Meets or exceeds target
- ⚠️ **Warning**: Within 2x of target
- ❌ **Fail**: Exceeds 2x target
- ⏳ **Pending**: Not yet measured

## Optimization Recommendations

### General Recommendations

1. **HTTP/2 Server Push**: Push critical CSS and JS files
2. **Resource Hints**: Add `preload` and `prefetch` directives for key assets
3. **CDN Integration**: Serve static assets from CDN for production
4. **Gzip/Brotli Compression**: Enable compression for text assets
5. **HTTP Caching**: Set appropriate cache headers for static assets
6. **Code Splitting**: Load JS modules on demand via HTMX

### HTMX-Specific Optimizations

1. **Lazy Loading**: Use `hx-trigger="revealed"` for below-fold content
2. **Request Queuing**: Use `hx-sync` to prevent request stampedes
3. **Prefetching**: Use `hx-boost` with `mousedown` trigger for instant navigation
4. **Response Caching**: Cache HTMX partial responses on the server
5. **Minimal Payloads**: Return only the HTML needed for updates

### Go Server Optimizations

1. **Template Caching**: Cache compiled Templ templates (already done)
2. **Response Pooling**: Reuse buffers for rendering
3. **Goroutine Pools**: Limit concurrent request handlers
4. **Database Connection Pooling**: Optimize Nomad API client
5. **Middleware Optimization**: Minimize middleware chain overhead

## Monitoring

### Production Monitoring

For production deployments, consider:

1. **Real User Monitoring (RUM)**: Track actual user experience
2. **Synthetic Monitoring**: Automated performance checks from multiple locations
3. **Server-Side Metrics**: Track Go server response times and resource usage
4. **Error Tracking**: Monitor failed requests and slow queries

### Recommended Tools

- **Prometheus + Grafana**: Server-side metrics
- **OpenTelemetry**: Distributed tracing
- **Sentry**: Error tracking and performance monitoring
- **SpeedCurve**: Performance monitoring and budgets
- **WebPageTest**: Detailed performance analysis

## Continuous Performance Testing

### CI/CD Integration

Add performance tests to CI pipeline:

```yaml
# .github/workflows/performance.yml
name: Performance Tests

on:
  pull_request:
    paths:
      - 'ui-htmx/**'

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25.5'

      - name: Start dev server
        run: |
          cd ui-htmx
          make setup
          go run cmd/dev/main.go &
          sleep 5

      - name: Run performance tests
        run: |
          cd ui-htmx
          ./scripts/measure-performance.sh http://localhost:3000

      - name: Check performance budget
        run: |
          cd ui-htmx
          # Add script to validate results against targets
          ./scripts/check-performance-budget.sh
```

## Performance Budget

Set performance budgets to prevent regression:

| Metric | Budget | Current | Headroom |
|--------|--------|---------|----------|
| TTFB (avg) | < 100ms | - | - |
| FCP | < 500ms | - | - |
| TTI | < 1s | - | - |
| Bundle Size (CSS) | < 50KB | - | - |
| Bundle Size (JS) | < 100KB | - | - |
| Total Page Weight | < 500KB | - | - |

## Benchmarking History

Track performance over time:

| Date | Version | TTFB | FCP | TTI | Notes |
|------|---------|------|-----|-----|-------|
| 2026-01-26 | Initial | - | - | - | Initial baseline needed |

## References

- [Web Vitals](https://web.dev/vitals/)
- [Lighthouse Performance Scoring](https://web.dev/performance-scoring/)
- [HTMX Performance Guide](https://htmx.org/essays/performance/)
- [Go Performance Tips](https://github.com/golang/go/wiki/Performance)
- [Templ Performance](https://templ.guide/performance/)
