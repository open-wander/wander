# Project: UI Modernization - Ember.js to HTMX/Templ/Tailwind

> Complete UI rewrite from Ember.js 3.28 + HDS + Bulma to Go Templ + HTMX + Alpine.js + Tailwind CSS

Started: 2026-01-25
Status: in-progress

---

## Tasks

### Phase 1: Infrastructure Setup

- [x] 1. Create ui-htmx directory structure and install Templ CLI
- [x] 2. Configure templ generate in Makefile with watch mode (needs: 1)
- [x] 3. Create base layout template with HTML head/body structure (needs: 2)
- [x] 4. Add HTMX library and create request detection middleware (needs: 1)
- [x] 5. Implement partial vs full page response logic (needs: 4)
- [x] 6. Install Tailwind CLI and create tailwind.config.js (needs: 1)
- [x] 7. Set up Tailwind CSS build pipeline with content paths (needs: 6)
- [x] 8. Create base Tailwind styles (typography, forms, buttons) (needs: 7)
- [x] 9. Add Alpine.js and create component patterns (dropdown, modal, tabs) (needs: 1)
- [x] 10. Create ui-htmx handlers package with route registration (needs: 3, 5)
- [x] 11. Configure air for Go hot-reload development workflow (needs: 10)
- [x] 12. Create SSE handler utility for real-time log streaming (needs: 10)
- [x] 13. Implement SSE connection management and reconnection logic (needs: 12)

### Phase 2: Core Component Library

- [x] 14. Create Button component with variants (primary, secondary, danger, ghost) (needs: 8)
- [x] 15. Create Badge component with color variants (needs: 8)
- [x] 16. Create Link component with internal/external handling (needs: 8)
- [x] 17. Create Icon component with Heroicons SVG helper (needs: 8)
- [x] 18. Create Input component (text, number, password, search) (needs: 8)
- [x] 19. Create Textarea component with auto-resize option (needs: 8)
- [x] 20. Create Select component with Alpine.js styling (needs: 9, 8)
- [x] 21. Create Checkbox, Radio, and Toggle components (needs: 8)
- [x] 22. Create FormField wrapper with label and error message (needs: 18, 19, 20, 21)
- [x] 23. Create PageLayout component with header, sidebar, main areas (needs: 3)
- [x] 24. Create Card component with header, body, footer slots (needs: 8)
- [x] 25. Create Table component with sortable columns support (needs: 8)
- [x] 26. Create Modal component with Alpine.js (needs: 9, 8)
- [x] 27. Create Dropdown menu component with Alpine.js (needs: 9, 8)
- [x] 28. Create Tabs component with HTMX lazy-loading support (needs: 5, 9)
- [x] 29. Create Breadcrumbs component (needs: 16)
- [x] 30. Create Alert component (info, success, warning, error) (needs: 8)
- [x] 31. Create Toast notification system with Alpine.js (needs: 9, 8)
- [x] 32. Create EmptyState component (needs: 8)
- [x] 33. Create LoadingSpinner and Skeleton loader components (needs: 8)
- [x] 34. Create ProgressBar component (needs: 8)
- [x] 35. Create Sidebar component with collapsible sections (needs: 9, 23)
- [x] 36. Create NavItem component with active state (needs: 16)
- [x] 37. Create Header component with user menu, region selector (needs: 27, 23)
- [x] 38. Create SearchBar component with HTMX autocomplete (needs: 5, 18)

### Phase 3: Jobs Section

- [x] 39. Create Jobs list handler and route (needs: 10)
- [x] 40. Create Jobs list page template with table (needs: 25, 39)
- [x] 41. Add Jobs list pagination with HTMX (needs: 40)
- [x] 42. Add Jobs list search/filter with debounced HTMX (needs: 38, 40)
- [x] 43. Add Jobs list sorting by column headers (needs: 40)
- [x] 44. Create Job detail handler and route (needs: 10)
- [x] 45. Create Job detail page with header and tabs (needs: 28, 44)
- [x] 46. Create Job Overview tab with task groups summary (needs: 45)
- [x] 47. Create Job Definition tab with CodeMirror 6 editor (needs: 45)
- [x] 48. Set up CodeMirror 6 with HCL/JSON syntax highlighting (needs: 47)
- [x] 49. Add Plan and Run buttons to Job editor (needs: 48)
- [x] 50. Create Job Versions tab with version list (needs: 45)
- [x] 51. Add version diff view (needs: 50)
- [x] 52. Add revert to version action (needs: 51)
- [x] 53. Create Job Allocations tab with table (needs: 45)
- [x] 54. Add real-time allocation updates via HTMX polling (needs: 53)
- [x] 55. Create Task group detail page (needs: 44)
- [x] 56. Add scaling controls to Task group page (needs: 55)
- [x] 57. Add Job action buttons (Start, Stop, Run) with confirmation modals (needs: 26, 45)

### Phase 4: Allocations Section

- [x] 58. Create Allocations list handler and route (needs: 10)
- [x] 59. Create Allocations list page with filters (needs: 25, 58)
- [x] 60. Add Allocations search by ID prefix (needs: 59)
- [x] 61. Create Allocation detail handler and route (needs: 10)
- [x] 62. Create Allocation detail page with tabs (needs: 28, 61)
- [x] 63. Create Allocation Tasks tab with status list (needs: 62)
- [x] 64. Create Allocation Logs tab with SSE streaming (needs: 12, 13, 62)
- [x] 65. Add stdout/stderr toggle to logs viewer (needs: 64)
- [x] 66. Add ANSI color support to logs viewer (needs: 64)
- [x] 67. Add download logs button (needs: 64)
- [x] 68. Create Allocation Files tab with directory tree (needs: 62)
- [x] 69. Add file content viewer with syntax highlighting (needs: 68)
- [x] 70. Add file download capability (needs: 68)
- [x] 71. Create Allocation Exec terminal with xterm.js (needs: 62)
- [x] 72. Implement WebSocket connection for exec (needs: 71)
- [x] 73. Add terminal resize handling (needs: 71)
- [x] 74. Add Allocation action buttons (Restart, Stop) (needs: 26, 62)

### Phase 5: Clients & Servers

- [x] 75. Create Clients list handler and route (needs: 10)
- [x] 76. Create Clients list page with resource bars (needs: 25, 34, 75)
- [x] 77. Add Clients filtering by status, datacenter (needs: 76)
- [x] 78. Create Client detail handler and route (needs: 10)
- [x] 79. Create Client detail page (needs: 78)
- [x] 80. Add resource utilization charts to Client detail (needs: 79)
- [x] 81. Add allocations list to Client detail (needs: 79)
- [x] 82. Add drain controls to Client detail (needs: 79)
- [x] 83. Create Servers list handler and route (needs: 10)
- [x] 84. Create Servers list page with leader badge (needs: 25, 15, 83)

### Phase 6: Additional Sections

- [x] 85. Create Topology page handler (needs: 10)
- [x] 86. Implement Topology visualization (D3 or grid-based) (needs: 85)
- [x] 87. Create CSI Volumes list handler and page (needs: 10, 25)
- [x] 88. Create CSI Volume detail page (needs: 87)
- [x] 89. Create CSI Plugins list page (needs: 87)
- [x] 90. Create Variables list handler and page (needs: 10, 25)
- [x] 91. Create Variable detail/edit page with key-value editor (needs: 90)
- [x] 92. Add create/delete variable functionality (needs: 91)
- [x] 93. Create Policies list handler and page (needs: 10, 25)
- [x] 94. Create Policy editor with HCL highlighting (needs: 48, 93)
- [x] 95. Create Settings/Token page (needs: 10, 22)
- [x] 96. Create Evaluations sidebar panel (needs: 26)

### Phase 7: Charts & Visualizations

- [x] 97. Create D3 chart container Templ component (needs: 8)
- [x] 98. Implement LineChart component for time-series data (needs: 97)
- [x] 99. Implement GaugeChart component for resource utilization (needs: 97)
- [x] 100. Implement BarChart component for comparisons (needs: 97)
- [x] 101. Add chart initialization after HTMX swaps (needs: 97)
- [x] 102. Implement CPU utilization chart (needs: 98)
- [x] 103. Implement Memory utilization chart (needs: 98)
- [x] 104. Add time range selector to resource charts (needs: 102, 103)
- [x] 105. Add auto-refresh to charts via HTMX polling (needs: 104)

### Phase 8: Testing Infrastructure

- [x] 106. Create Templ template test utilities (needs: 3)
- [x] 107. Create mock data layer for handler testing (needs: 10)
- [x] 108. Write handler unit tests for Jobs routes (needs: 39, 44, 107)
- [x] 109. Write handler unit tests for Allocations routes (needs: 58, 61, 107)
- [x] 110. Write handler unit tests for Clients/Servers routes (needs: 75, 78, 83, 107)
- [x] 111. Configure Playwright with Go test server (needs: 10)
- [x] 112. Create Playwright page object patterns (needs: 111)
- [ ] 113. Write e2e tests for Jobs list and detail (needs: 112, 45)
- [ ] 114. Write e2e tests for Allocations and logs (needs: 112, 64)
- [ ] 115. Write e2e tests for Clients and Servers (needs: 112, 79, 84)
- [ ] 116. Set up visual regression testing baseline (needs: 113)

### Phase 9: Migration Cutover

- [x] 117. Configure new UI to serve at /ui/ path (needs: all Phase 3-6 tasks)
- [x] 118. Verify all routes return 200 or appropriate error codes (needs: 117)
- [x] 119. Run full link checker for broken links/assets (needs: 117)
- [x] 120. Measure JavaScript bundle size (target <100kb) (needs: 117)
- [ ] 121. Measure TTFB, FCP, TTI performance metrics (needs: 117)
- [x] 122. Run load test with concurrent users (needs: 117)
- [x] 123. Update CHANGELOG with migration notes (needs: 117)
- [ ] 124. Remove Ember.js /ui/ directory (needs: 122, 116)
- [ ] 125. Remove package.json and node_modules (needs: 124)
- [ ] 126. Update CI to not build Ember (needs: 124)
- [ ] 127. Update documentation (needs: 126)

---

## Notes

### Technology Stack
- **Templates:** Go Templ (type-safe, compiles to Go)
- **Interactivity:** HTMX (~16kb) for dynamic updates
- **Client State:** Alpine.js (~15kb) for dropdowns, modals, tabs
- **Styling:** Tailwind CSS (utility-first)
- **Charts:** D3.js (keep existing)
- **Code Editor:** CodeMirror 6 (upgrade from v5)
- **Terminal:** xterm.js (keep)
- **Icons:** Heroicons
- **Real-time:** SSE for logs/events, HTMX polling for lists

### Performance Targets
- JavaScript bundle: <100kb gzipped
- CSS bundle: <50kb gzipped (after Tailwind purge)
- Time to First Byte: <100ms
- First Contentful Paint: <500ms
- Time to Interactive: <1s

### Rollout Strategy
- Hard cutover (no feature flag)
- New UI replaces Ember UI at /ui/ path
- Single binary deployment (no node_modules in production)

### Directory Structure
```
/ui-htmx/
  handlers/       # HTTP handlers
  templates/
    layouts/      # Base layouts
    components/   # Reusable components
    pages/        # Page templates
    partials/     # HTMX partial responses
  static/
    css/          # Tailwind output
    js/           # HTMX, Alpine, CodeMirror
    icons/        # Heroicons SVGs
  tests/
    unit/
    e2e/
```

### Agent Recommendations
- Go code: `golang-pro`
- Frontend design: `frontend-design`
- JavaScript: `javascript-pro`
- Testing: `test-automator`
- Code review: `code-reviewer`
- Security: `security-auditor`
