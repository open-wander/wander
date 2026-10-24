# JavaScript Bundle Size Analysis

**Date:** 2026-01-26
**Target:** < 100 KB (gzipped)
**Current Total:** 157.18 KB (gzipped)
**Status:** ❌ FAILED - 57.18 KB (57.2%) over budget

## Bundle Composition

| File | Raw Size | Gzipped | Compression | Notes |
|------|----------|---------|-------------|-------|
| **xterm.min.js** | 282.92 KB | 66.30 KB | 23.4% | Terminal emulator (largest file) |
| **htmx.min.js** | 50.05 KB | 16.23 KB | 32.4% | Core HTMX framework |
| **alpine.min.js** | 44.71 KB | 16.14 KB | 36.1% | Alpine.js framework |
| **alpine-patterns.js** | 43.37 KB | 10.61 KB | 24.5% | Custom Alpine patterns |
| **codemirror-bundle.js** | 34.51 KB | 7.39 KB | 21.4% | Code editor |
| **topology-viz.js** | 23.85 KB | 5.21 KB | 21.8% | Topology visualization |
| **memory-chart.js** | 22.16 KB | 5.29 KB | 23.9% | Memory chart component |
| **bar-chart.js** | 17.96 KB | 4.03 KB | 22.4% | Bar chart component |
| **cpu-chart.js** | 17.04 KB | 4.45 KB | 26.1% | CPU chart component |
| **line-chart.js** | 16.85 KB | 4.10 KB | 24.3% | Line chart component |
| **chart-utils.js** | 14.02 KB | 3.95 KB | 28.2% | Chart utilities |
| **sse-client.js** | 12.34 KB | 3.18 KB | 25.8% | Server-sent events client |
| **gauge-chart.js** | 10.61 KB | 2.71 KB | 25.6% | Gauge chart component |
| **time-range-selector.js** | 8.82 KB | 2.53 KB | 28.7% | Time range selector |
| **chart-time-range-integration.js** | 7.78 KB | 2.24 KB | 28.8% | Chart time range integration |
| **ansi.js** | 6.11 KB | 2.00 KB | 32.8% | ANSI parser for terminal |
| **xterm-addon-fit.min.js** | 1.74 KB | 0.82 KB | 46.9% | XTerm fit addon |

**TOTAL** | **614.84 KB** | **157.18 KB** | **25.6%** | Overall compression

## Analysis by Category

### 1. Vendor Libraries (99.49 KB gzipped, 63.3% of total)

- **xterm.js + addon:** 67.12 KB (42.7% of bundle)
  - Only needed on Exec/Terminal pages
  - Strong candidate for lazy loading

- **htmx:** 16.23 KB (10.3% of bundle)
  - Core framework, needed on all pages

- **alpine.js:** 16.14 KB (10.3% of bundle)
  - Core framework, needed on all pages

- **codemirror:** 7.39 KB (4.7% of bundle)
  - Only needed on Job editor pages
  - Should be lazy loaded

### 2. Custom Chart Components (27.74 KB gzipped, 17.6% of total)

- chart-utils.js: 3.95 KB
- cpu-chart.js: 4.45 KB
- memory-chart.js: 5.29 KB
- line-chart.js: 4.10 KB
- bar-chart.js: 4.03 KB
- gauge-chart.js: 2.71 KB
- chart-time-range-integration.js: 2.24 KB
- time-range-selector.js: 2.53 KB

Charts are used on:
- Dashboard
- Client detail pages
- Job detail pages

### 3. Custom Application Code (29.95 KB gzipped, 19.1% of total)

- alpine-patterns.js: 10.61 KB
- topology-viz.js: 5.21 KB
- sse-client.js: 3.18 KB
- ansi.js: 2.00 KB

## Recommendations

### Priority 1: Lazy Load Terminal & Editor (SAVES ~75 KB)

**Impact:** Would reduce bundle to 82.18 KB gzipped (18% under budget)

```html
<!-- Only load on pages with terminal/exec functionality -->
<script>
  if (document.querySelector('[data-terminal]')) {
    Promise.all([
      import('/static/js/xterm.min.js'),
      import('/static/js/xterm-addon-fit.min.js'),
      import('/static/js/ansi.js')
    ]);
  }

  if (document.querySelector('[data-code-editor]')) {
    import('/static/js/codemirror-bundle.js');
  }
</script>
```

### Priority 2: Lazy Load Charts by Context (SAVES ~28 KB)

Not all pages need all chart types. Load charts on-demand:

- **Dashboard:** Load only line-chart, gauge-chart
- **Client detail:** Load all charts
- **Job pages:** Load only relevant charts

```html
<!-- Load charts based on page context -->
<script>
  const pageType = document.body.dataset.pageType;
  const chartModules = {
    dashboard: ['line-chart', 'gauge-chart'],
    client: ['line-chart', 'bar-chart', 'gauge-chart', 'cpu-chart', 'memory-chart'],
    // ...
  };

  chartModules[pageType]?.forEach(chart => {
    import(`/static/js/${chart}.js`);
  });
</script>
```

### Priority 3: Review alpine-patterns.js (SAVES ~5-10 KB)

The custom Alpine patterns file is large (43.37 KB raw, 10.61 KB gzipped). Review if:
- All patterns are necessary
- Code can be optimized
- Some patterns can be extracted to separate lazy-loaded files

### Priority 4: Consider Alternative Terminal Library

xterm.js is powerful but heavy (66 KB gzipped). Alternatives:
- **xterm-headless:** Lighter version without DOM rendering
- **Custom terminal:** Build minimal ANSI-capable terminal for read-only logs
- Keep xterm only for interactive exec sessions

### Priority 5: Minify Custom JavaScript

Files that appear to be unminified:
- alpine-patterns.js (43.37 KB → ~11 KB gzipped suggests room for improvement)
- Chart components (could benefit from minification)
- topology-viz.js

Consider adding a build step with:
- Terser for minification
- Tree-shaking for unused code
- Dead code elimination

## Proposed Bundle Strategy

### Core Bundle (32.32 KB gzipped, 68% under budget)
Load on every page:
- htmx.min.js: 16.23 KB
- alpine.min.js: 16.14 KB

### Feature Bundles (Lazy Loaded)

**Terminal Bundle (67.12 KB):**
- xterm.min.js
- xterm-addon-fit.min.js
- ansi.js

**Editor Bundle (7.39 KB):**
- codemirror-bundle.js

**Dashboard Bundle (~15 KB):**
- chart-utils.js
- line-chart.js
- gauge-chart.js
- time-range-selector.js

**Client Detail Bundle (~28 KB):**
- All chart components
- topology-viz.js

**Utilities (Always Loaded) (~14 KB):**
- alpine-patterns.js (optimize first)
- sse-client.js

## Implementation Steps

1. **Immediate (Week 1):** Implement lazy loading for xterm and codemirror
   - Expected result: ~82 KB total bundle

2. **Short-term (Week 2):** Implement chart lazy loading by page type
   - Expected result: ~60 KB core bundle + page-specific bundles

3. **Medium-term (Week 3-4):** Optimize alpine-patterns.js
   - Review and refactor custom patterns
   - Split large patterns into separate files

4. **Long-term:** Add minification build step
   - Set up esbuild or similar
   - Integrate into build process

## Measurement Script

A measurement script is available at `scripts/measure-bundle.sh` to track progress:

```bash
./ui-htmx/scripts/measure-bundle.sh
```

This script:
- Measures raw and gzipped size of all JS files
- Compares against 100 KB target
- Shows detailed breakdown by file
- Returns exit code 0 if under budget, 1 if over

## Success Metrics

- **Current:** 157.18 KB (57.2% over budget)
- **After Priority 1:** ~82 KB (18% under budget) ✓
- **After Priority 2:** ~60 KB core + lazy bundles ✓✓
- **Target:** < 100 KB ✓

With lazy loading of terminal and editor features, we can meet the target while maintaining full functionality.
