# PRD: UI Modernization - Ember.js to HTMX/Templ/Tailwind Migration

## Introduction

Migrate the Wander UI from Ember.js 3.28 + HashiCorp Design System (HDS) + Bulma to a modern server-side rendering stack using Go Templ templates, HTMX for dynamic interactions, Alpine.js for client-side state, and Tailwind CSS for styling.

This is a complete rewrite (Big Bang approach) that maintains the same layout and functionality while allowing for a modern styling refresh. The migration addresses performance concerns, removes problematic HashiCorp dependencies, improves developer experience with server-side rendering, and eliminates technical debt from the outdated Ember 3.28 framework.

### Current State

| Aspect | Current Stack |
|--------|---------------|
| Framework | Ember.js 3.28 (Octane) |
| CSS | Bulma 0.9.3 + HDS Components |
| Components | 123+ custom components |
| Templates | 197 Handlebars templates |
| Routes | 64 route files |
| Charts | D3.js (d3-array, d3-scale, d3-shape, etc.) |
| Code Editor | CodeMirror 5.58.2 |
| Terminal | xterm 4.6.0 |
| Build | Ember CLI + Broccoli |
| Testing | QUnit + Mirage.js + Percy |

### Target State

| Aspect | New Stack |
|--------|-----------|
| Templates | Go Templ |
| Interactivity | HTMX (~16kb) |
| Client State | Alpine.js (~15kb) |
| CSS | Tailwind CSS |
| Charts | D3.js (keep existing) |
| Code Editor | CodeMirror 6 (upgrade from v5) |
| Terminal | xterm (keep) |
| Icons | Heroicons |
| Real-time | Hybrid: SSE for logs/events, HTMX polling for lists |
| Build | Go build + Templ generate + Tailwind CLI |
| Testing | Go testing + Playwright e2e |
| Browser Support | Chrome/Edge 80+, Firefox 78+, Safari 14+ (2020+) |

## Goals

- Remove Ember.js framework dependency entirely
- Remove HashiCorp Design System dependency
- Reduce JavaScript bundle size by 60%+ (target: <100kb total JS)
- Maintain feature parity with current UI
- Improve server response times with SSR
- Enable Go developers to contribute to UI without learning Ember
- Establish full test coverage (unit, integration, e2e)
- Modernize styling while preserving layout and UX patterns

## User Stories

### Phase 1: Infrastructure Setup

#### US-001: Set up Templ templating infrastructure
**Description:** As a developer, I need Templ configured in the Go project so I can write type-safe HTML templates.

**Acceptance Criteria:**
- [ ] Install Templ CLI (`go install github.com/a-h/templ/cmd/templ@latest`)
- [ ] Create `/ui-htmx/` directory structure for new UI
- [ ] Configure `templ generate` in build process
- [ ] Create base layout template with HTML head, body structure
- [ ] Verify templates compile to Go code successfully
- [ ] Add Makefile targets: `make ui-generate`, `make ui-watch`

#### US-002: Integrate HTMX into Go server
**Description:** As a developer, I need HTMX available so pages can make dynamic requests without full page reloads.

**Acceptance Criteria:**
- [ ] Add HTMX library (CDN or embedded)
- [ ] Create middleware for HTMX request detection (`HX-Request` header)
- [ ] Implement partial vs full page response logic
- [ ] Add `HX-Push-Url` support for browser history
- [ ] Create example endpoint returning HTML fragment
- [ ] Verify HTMX swaps work correctly

#### US-003: Set up Tailwind CSS build pipeline
**Description:** As a developer, I need Tailwind CSS configured so I can style components with utility classes.

**Acceptance Criteria:**
- [ ] Install Tailwind CLI
- [ ] Create `tailwind.config.js` with custom theme (colors matching current UI)
- [ ] Configure content paths for Templ files
- [ ] Set up CSS build: `tailwind -i input.css -o output.css --watch`
- [ ] Create base styles (typography, forms, buttons)
- [ ] Integrate CSS output into Go embed

#### US-004: Configure Alpine.js for client-side state
**Description:** As a developer, I need Alpine.js for components requiring client-side interactivity (dropdowns, modals, tabs).

**Acceptance Criteria:**
- [ ] Add Alpine.js library (CDN or embedded)
- [ ] Create reusable Alpine component patterns (dropdown, modal, tabs)
- [ ] Document when to use Alpine vs HTMX
- [ ] Verify Alpine components work with HTMX swaps (`x-init` reinitializes)

#### US-005: Create new HTTP handlers for UI routes
**Description:** As a developer, I need new HTTP handlers that serve Templ-rendered HTML instead of the Ember SPA.

**Acceptance Criteria:**
- [ ] Create `/ui-htmx/handlers/` package
- [ ] Implement handler pattern that renders Templ templates
- [ ] Add route registration separate from API routes
- [ ] Support both full page and HTMX partial responses
- [ ] Add request context for user session/token

#### US-006: Set up development workflow
**Description:** As a developer, I need hot-reload during development for rapid iteration.

**Acceptance Criteria:**
- [ ] Configure `air` or similar for Go hot-reload
- [ ] Templ watch mode generates on file change
- [ ] Tailwind watch mode rebuilds CSS on change
- [ ] Browser auto-refresh on changes (via HTMX or livereload)
- [ ] Document development setup in README

#### US-006b: Implement Server-Sent Events infrastructure
**Description:** As a developer, I need SSE support for real-time log streaming and event updates.

**Acceptance Criteria:**
- [ ] Create SSE handler utility for Go
- [ ] Implement connection management (client tracking, cleanup)
- [ ] Add SSE endpoint pattern for streaming data
- [ ] Create client-side SSE helper (EventSource wrapper)
- [ ] Handle reconnection logic on connection drop
- [ ] Integrate with HTMX via `hx-ext="sse"` extension
- [ ] Document when to use SSE vs HTMX polling

---

### Phase 2: Core Component Library

#### US-007: Create base Templ components (buttons, badges, links)
**Description:** As a developer, I need reusable base components so I can build consistent UI elements.

**Acceptance Criteria:**
- [ ] `Button` component with variants: primary, secondary, danger, ghost
- [ ] `Badge` component with color variants (success, warning, error, info)
- [ ] `Link` component with internal/external handling
- [ ] `Icon` component using Heroicons (inline SVG)
- [ ] Create icon helper to embed Heroicons as SVG in templates
- [ ] All components use Tailwind utility classes
- [ ] Components accept common HTML attributes (class, id, data-*)

#### US-008: Create form components
**Description:** As a developer, I need form components for user input throughout the application.

**Acceptance Criteria:**
- [ ] `Input` component (text, number, password, search)
- [ ] `Textarea` component with auto-resize option
- [ ] `Select` component with Alpine.js for custom styling
- [ ] `Checkbox` and `Radio` components
- [ ] `Toggle` switch component
- [ ] `FormField` wrapper with label and error message
- [ ] Form validation patterns documented

#### US-009: Create layout components
**Description:** As a developer, I need layout components to structure pages consistently.

**Acceptance Criteria:**
- [ ] `PageLayout` with header, sidebar, main content areas
- [ ] `Card` component with header, body, footer slots
- [ ] `Table` component with sortable columns support
- [ ] `Modal` component with Alpine.js
- [ ] `Dropdown` menu component with Alpine.js
- [ ] `Tabs` component with HTMX lazy-loading support
- [ ] `Breadcrumbs` component

#### US-010: Create feedback components
**Description:** As a developer, I need components for user feedback and notifications.

**Acceptance Criteria:**
- [ ] `Alert` component (info, success, warning, error)
- [ ] `Toast` notification system with Alpine.js
- [ ] `EmptyState` component for no-data scenarios
- [ ] `LoadingSpinner` component
- [ ] `Skeleton` loader components for HTMX loading states
- [ ] `ProgressBar` component

#### US-011: Create navigation components
**Description:** As a developer, I need navigation components matching the current sidebar and header patterns.

**Acceptance Criteria:**
- [ ] `Sidebar` component with collapsible sections
- [ ] `NavItem` component with active state
- [ ] `Header` component with user menu, region selector
- [ ] `SearchBar` component with HTMX autocomplete
- [ ] Mobile-responsive navigation (hamburger menu)

---

### Phase 3: Page Migration - Jobs Section

#### US-012: Migrate Jobs list page
**Description:** As a user, I want to see all jobs in my namespace so I can monitor workloads.

**Acceptance Criteria:**
- [ ] Jobs table with columns: Name, Status, Type, Priority, Submitted
- [ ] Status badges with correct colors (running=green, pending=yellow, dead=gray)
- [ ] Pagination with HTMX (no full page reload)
- [ ] Search/filter with HTMX debounced requests
- [ ] Namespace selector in header
- [ ] Sort by column headers
- [ ] Click row navigates to job detail
- [ ] e2e test: list jobs, filter, paginate

#### US-013: Migrate Job detail page
**Description:** As a user, I want to see job details so I can understand its configuration and status.

**Acceptance Criteria:**
- [ ] Job header with name, status, type, priority
- [ ] Tabs: Overview, Definition, Versions, Allocations, Evaluations
- [ ] Overview tab: Task groups summary, constraints, datacenters
- [ ] HTMX tab switching (lazy load tab content)
- [ ] Action buttons: Start, Stop, Run (with confirmation modals)
- [ ] e2e test: view job, switch tabs, stop job

#### US-014: Migrate Job definition/editor page
**Description:** As a user, I want to view and edit job specifications in HCL/JSON format.

**Acceptance Criteria:**
- [ ] CodeMirror 6 integration for HCL/JSON editing
- [ ] Install @codemirror/lang-json and create HCL language support
- [ ] Syntax highlighting for HCL and JSON
- [ ] "Plan" button to dry-run changes
- [ ] "Run" button to submit job
- [ ] Diff view for job updates (evaluate @codemirror/merge)
- [ ] Error display for invalid job specs
- [ ] e2e test: edit job spec, plan, run

#### US-015: Migrate Job versions page
**Description:** As a user, I want to see job version history so I can track changes over time.

**Acceptance Criteria:**
- [ ] Version list with version number, stable flag, submit time
- [ ] Click version to view diff against previous
- [ ] "Revert to this version" action
- [ ] HTMX infinite scroll or pagination
- [ ] e2e test: view versions, view diff, revert

#### US-016: Migrate Job allocations tab
**Description:** As a user, I want to see all allocations for a job so I can monitor task placement.

**Acceptance Criteria:**
- [ ] Allocations table: ID, Node, Status, Created, Modified
- [ ] Status with health indicators
- [ ] Filter by task group
- [ ] Click allocation navigates to allocation detail
- [ ] Real-time updates via HTMX polling or SSE
- [ ] e2e test: view allocations, filter by task group

#### US-017: Migrate Task group detail page
**Description:** As a user, I want to see task group details including scaling and placement.

**Acceptance Criteria:**
- [ ] Task group header with count, scaling info
- [ ] Tasks list with resource requirements
- [ ] Placement failures display
- [ ] Scaling controls (if autoscaling enabled)
- [ ] e2e test: view task group, scale count

---

### Phase 4: Page Migration - Allocations Section

#### US-018: Migrate Allocations list page
**Description:** As a user, I want to see all allocations so I can monitor task execution.

**Acceptance Criteria:**
- [ ] Allocations table with job, task group, node, status
- [ ] Status badges with health colors
- [ ] Filter by status, job, node
- [ ] Search by allocation ID prefix
- [ ] Pagination with HTMX
- [ ] e2e test: list allocations, filter, search

#### US-019: Migrate Allocation detail page
**Description:** As a user, I want to see allocation details including tasks and resources.

**Acceptance Criteria:**
- [ ] Allocation header with ID, job, node, status
- [ ] Tabs: Tasks, Logs, Files, Raw
- [ ] Tasks list with individual task status
- [ ] Resource utilization (CPU, memory)
- [ ] Events timeline
- [ ] Action buttons: Restart, Stop
- [ ] e2e test: view allocation, restart task

#### US-020: Migrate Task logs viewer
**Description:** As a user, I want to stream task logs so I can debug issues.

**Acceptance Criteria:**
- [ ] Log streaming via Server-Sent Events (SSE)
- [ ] SSE endpoint wraps Nomad log streaming API
- [ ] Toggle between stdout and stderr
- [ ] Auto-scroll with pause on manual scroll
- [ ] Download logs button
- [ ] ANSI color support (use anser library or similar)
- [ ] e2e test: view logs, switch streams, download

#### US-021: Migrate File system browser
**Description:** As a user, I want to browse allocation files so I can inspect artifacts.

**Acceptance Criteria:**
- [ ] Directory tree navigation
- [ ] File content viewer with syntax highlighting
- [ ] File download capability
- [ ] Breadcrumb navigation
- [ ] File size and modification time display
- [ ] HTMX navigation (no full page reload)
- [ ] e2e test: navigate directories, view file, download

#### US-022: Migrate Exec terminal
**Description:** As a user, I want to execute commands in running tasks for debugging.

**Acceptance Criteria:**
- [ ] xterm.js integration for terminal emulation
- [ ] WebSocket connection to allocation exec endpoint
- [ ] Task selector dropdown
- [ ] Terminal resize handling
- [ ] Copy/paste support
- [ ] e2e test: open exec, run command, see output

---

### Phase 5: Page Migration - Clients & Servers

#### US-023: Migrate Clients list page
**Description:** As a user, I want to see all client nodes so I can monitor cluster capacity.

**Acceptance Criteria:**
- [ ] Clients table: Name, Status, Datacenter, Allocations, Resources
- [ ] Status indicators (ready, down, draining)
- [ ] Resource bars (CPU, memory, disk)
- [ ] Filter by status, datacenter
- [ ] Click navigates to client detail
- [ ] e2e test: list clients, filter

#### US-024: Migrate Client detail page
**Description:** As a user, I want to see client node details and resource utilization.

**Acceptance Criteria:**
- [ ] Client header with name, status, address
- [ ] Resource utilization charts (CPU, memory over time)
- [ ] Allocations running on this client
- [ ] Node attributes and metadata
- [ ] Drain controls
- [ ] e2e test: view client, view allocations

#### US-025: Migrate Servers list page
**Description:** As a user, I want to see server agents so I can monitor cluster leadership.

**Acceptance Criteria:**
- [ ] Servers table: Name, Address, Status, Leader indicator
- [ ] Leader badge highlighting
- [ ] Raft peer status
- [ ] Click navigates to server detail
- [ ] e2e test: list servers, identify leader

---

### Phase 6: Page Migration - Additional Sections

#### US-026: Migrate Topology page
**Description:** As a user, I want a visual topology view of the cluster.

**Acceptance Criteria:**
- [ ] Evaluate: D3.js visualization vs simpler HTMX/Tailwind grid
- [ ] Datacenter groupings
- [ ] Node status indicators
- [ ] Allocation placement visualization
- [ ] Interactive zoom/pan if using D3
- [ ] e2e test: view topology, interact with nodes

#### US-027: Migrate CSI Volumes pages
**Description:** As a user, I want to manage Container Storage Interface volumes.

**Acceptance Criteria:**
- [ ] Volumes list with name, plugin, capacity, status
- [ ] Volume detail page with attachments
- [ ] Plugins list page
- [ ] Create/delete volume actions
- [ ] e2e test: list volumes, view detail

#### US-028: Migrate Variables pages
**Description:** As a user, I want to manage secure variables for jobs.

**Acceptance Criteria:**
- [ ] Variables list with path, namespace
- [ ] Variable detail/edit page with key-value editor
- [ ] Create new variable form
- [ ] Delete with confirmation
- [ ] Path-based navigation (folder structure)
- [ ] e2e test: create variable, edit, delete

#### US-029: Migrate Policies pages (ACL)
**Description:** As a user, I want to manage ACL policies for access control.

**Acceptance Criteria:**
- [ ] Policies list with name, description
- [ ] Policy editor with HCL syntax highlighting
- [ ] Create/edit/delete policies
- [ ] Policy rules validation
- [ ] e2e test: create policy, edit rules

#### US-030: Migrate Settings/Token page
**Description:** As a user, I want to manage my authentication token.

**Acceptance Criteria:**
- [ ] Token input field
- [ ] Token validation display
- [ ] Token permissions summary
- [ ] Clear token action
- [ ] e2e test: enter token, verify access

#### US-031: Migrate Evaluations sidebar
**Description:** As a user, I want to view evaluation details for scheduling decisions.

**Acceptance Criteria:**
- [ ] Evaluation detail panel (slide-out or modal)
- [ ] Status, triggered by, job link
- [ ] Placement failures explanation
- [ ] Related evaluations
- [ ] e2e test: view evaluation detail

---

### Phase 7: Charts & Visualizations

#### US-032: Implement D3 chart components with Templ
**Description:** As a developer, I need to integrate D3.js charts with the new Templ-based UI.

**Acceptance Criteria:**
- [ ] Create chart container Templ components with proper sizing
- [ ] Implement `LineChart` component for time-series data
- [ ] Implement `GaugeChart` component for resource utilization
- [ ] Implement `BarChart` component for comparisons
- [ ] Charts initialize correctly after HTMX swaps (mutation observer or htmx:afterSwap)
- [ ] Charts responsive to container size
- [ ] e2e test: verify chart renders with data

#### US-033: Implement resource utilization charts
**Description:** As a user, I want to see resource usage over time for clients and allocations.

**Acceptance Criteria:**
- [ ] CPU utilization line chart
- [ ] Memory utilization line chart
- [ ] Time range selector (1h, 6h, 24h, 7d)
- [ ] Auto-refresh with HTMX polling
- [ ] Hover tooltips with exact values
- [ ] e2e test: view chart, change time range

---

### Phase 8: Testing Infrastructure

#### US-034: Set up Go unit tests for handlers
**Description:** As a developer, I need unit tests for all HTTP handlers.

**Acceptance Criteria:**
- [ ] Test utilities for rendering Templ templates in tests
- [ ] Mock data layer for testing without database
- [ ] Handler tests for each route (happy path + errors)
- [ ] Test coverage target: 80%+
- [ ] CI integration: `go test ./...`

#### US-035: Set up Playwright e2e tests
**Description:** As a developer, I need end-to-end tests to verify full user flows.

**Acceptance Criteria:**
- [ ] Playwright configured with Go test server
- [ ] Test fixtures for consistent data
- [ ] Page object pattern for reusable selectors
- [ ] Tests for all major user flows documented in acceptance criteria
- [ ] Screenshot on failure for debugging
- [ ] CI integration with test artifacts

#### US-036: Implement visual regression testing
**Description:** As a developer, I need to catch unintended visual changes.

**Acceptance Criteria:**
- [ ] Evaluate: Percy, Playwright screenshots, or Chromatic
- [ ] Baseline screenshots for key pages
- [ ] CI integration with visual diff reports
- [ ] Process for approving intentional changes

---

### Phase 9: Migration Cutover

#### US-037: Prepare release cutover
**Description:** As a developer, I need to prepare the new UI for hard cutover release.

**Acceptance Criteria:**
- [ ] New UI serves at `/ui/` path (replacing Ember)
- [ ] All routes return 200 or appropriate error codes
- [ ] Verify no broken links or missing assets
- [ ] Update CHANGELOG with migration notes
- [ ] Document any breaking changes for users

#### US-038: Remove Ember.js and dependencies
**Description:** As a developer, I need to remove the old UI after successful migration.

**Acceptance Criteria:**
- [ ] Remove `/ui/` directory (Ember app)
- [ ] Remove `package.json` and node_modules
- [ ] Remove go-bindata asset generation for old UI
- [ ] Update CI to not build Ember
- [ ] Update documentation
- [ ] Remove feature flag, make HTMX UI the default

#### US-039: Performance validation
**Description:** As a developer, I need to verify the new UI meets performance goals.

**Acceptance Criteria:**
- [ ] Measure JavaScript bundle size: target <100kb
- [ ] Measure Time to First Byte (TTFB): target <100ms
- [ ] Measure First Contentful Paint (FCP): target <500ms
- [ ] Measure Time to Interactive (TTI): target <1s
- [ ] Load test with concurrent users
- [ ] Document performance comparison vs old UI

---

## Functional Requirements

### Templates & Rendering
- FR-1: All UI pages must be rendered server-side using Go Templ templates
- FR-2: HTMX must be used for all dynamic content updates without full page reloads
- FR-3: Alpine.js must be used only for client-side state (dropdowns, modals, form validation)
- FR-4: Templates must support partial rendering for HTMX requests (`HX-Request` header detection)
- FR-5: All templates must be type-safe with Go structs for data

### Styling
- FR-6: Tailwind CSS must be used for all styling (no custom CSS except for third-party integration)
- FR-7: Color scheme must match current UI branding (customizable via Tailwind config)
- FR-8: UI must be responsive (mobile, tablet, desktop breakpoints)
- FR-9: Dark mode support (optional, future consideration)

### Navigation & Routing
- FR-10: Browser history must work correctly (back/forward buttons)
- FR-11: Direct URL access must work for all pages (no JavaScript required for initial render)
- FR-12: HTMX must update URL via `hx-push-url` for client-side navigation
- FR-13: Breadcrumbs must reflect current location

### Data & API
- FR-14: UI handlers must call existing Nomad RPC/API internally (no new API endpoints)
- FR-15: Real-time updates must use HTMX polling or Server-Sent Events
- FR-16: Blocking queries must be supported for efficient polling
- FR-17: Error responses must render user-friendly error pages

### Authentication
- FR-18: ACL token must be read from cookie or header
- FR-19: Unauthenticated users must see appropriate limited UI
- FR-20: Token management page must allow setting/clearing token

### Accessibility
- FR-21: All interactive elements must be keyboard accessible
- FR-22: ARIA labels must be present for screen readers
- FR-23: Focus management must work correctly with HTMX swaps
- FR-24: Color contrast must meet WCAG AA standards

---

## Non-Goals (Out of Scope)

- **Mobile native app** - This is web UI only
- **Offline support** - Requires server connection
- **GraphQL API** - Continue using existing REST/RPC
- **WebSocket for all real-time** - HTMX polling is sufficient for most cases
- **Custom design system documentation site** - Use code comments and examples
- **Internationalization (i18n)** - English only initially
- **Plugin/extension system** - Core UI only
- **Storybook replacement** - Document components in code

---

## Technical Considerations

### Directory Structure
```
/ui-htmx/
├── handlers/           # HTTP handlers for each route
│   ├── jobs.go
│   ├── allocations.go
│   └── ...
├── templates/          # Templ templates
│   ├── layouts/        # Base layouts
│   ├── components/     # Reusable components
│   ├── pages/          # Page-specific templates
│   └── partials/       # HTMX partial responses
├── static/             # Static assets
│   ├── css/            # Tailwind output
│   ├── js/             # HTMX, Alpine, CodeMirror
│   └── icons/          # SVG icons
└── tests/              # Test files
    ├── unit/
    └── e2e/
```

### Technology Decisions
| Component | Decision | Rationale |
|-----------|----------|-----------|
| Charts | **D3.js** (keep) | Existing investment, complex visualizations needed for topology |
| Code Editor | **CodeMirror 6** (upgrade) | Modern architecture, better performance, tree-shaking |
| Terminal | **xterm** (keep) | Stable, works well |
| Icons | **Heroicons** | Tailwind ecosystem, popular, 300+ icons, MIT license |
| Real-time | **Hybrid SSE + Polling** | SSE for logs/events (low latency), HTMX polling for lists (simpler) |
| Browser Support | **2020+ browsers** | Chrome/Edge 80+, Firefox 78+, Safari 14+ |
| Rollout | **Hard cutover** | Clean break, no maintenance of two UIs |

### Build Process
1. `templ generate` - Compile templates to Go
2. `tailwindcss -i input.css -o static/css/output.css` - Build CSS
3. `go build` - Compile Go binary with embedded assets
4. Single binary deployment (no node_modules in production)

### Performance Targets
- JavaScript total: <100kb (gzipped)
- CSS total: <50kb (gzipped, after Tailwind purge)
- Initial page load: <1s TTI
- HTMX partial swap: <200ms

---

## Success Metrics

- **Bundle Size:** JavaScript <100kb (vs current ~500kb+ with Ember)
- **Build Time:** <30s full build (vs current ~2min with Ember)
- **Test Coverage:** >80% handler coverage, >90% critical path e2e coverage
- **Performance:** First Contentful Paint <500ms on average connection
- **Developer Velocity:** New pages can be added in <1 day
- **Zero HashiCorp Dependencies:** Complete removal of HDS packages

---

## Recommended Agents & Skills

When implementing this PRD, use the following specialized agents:

| Phase/Task | Agent | When to Use |
|------------|-------|-------------|
| Go code (handlers, templates) | `golang-pro` | Writing idiomatic Go, Templ integration, HTTP handlers |
| Frontend design | `frontend-design` | Creating Tailwind components, layout decisions |
| JavaScript (Alpine, D3, CodeMirror) | `javascript-pro` | Client-side code, Alpine components, D3 charts |
| TypeScript (if any) | `typescript-pro` | Type definitions, CodeMirror 6 setup |
| Testing | `test-automator` | Setting up Playwright, Go tests, test patterns |
| Code review | `code-reviewer` | After completing each user story |
| Architecture decisions | `architect-reviewer` | After structural changes, new packages |
| Security | `security-auditor` | ACL/token handling, input validation |
| CSS/UI verification | `ui-visual-validator` | Verifying UI matches requirements |
| Debugging | `debugger` | When encountering errors or test failures |
| Documentation | `docs-architect` | README updates, architecture docs |

### Workflow for Each User Story

1. **Start**: Read the user story and acceptance criteria
2. **Plan**: Use `Plan` agent for complex stories (3+ files)
3. **Implement**: Use appropriate specialized agent (e.g., `golang-pro` for Go code)
4. **Review**: Run `code-reviewer` agent after implementation
5. **Test**: Use `test-automator` to write/run tests
6. **Verify UI**: Use `ui-visual-validator` for any visual changes
7. **Complete**: Mark acceptance criteria as done

### Skills to Use

- `/commit` - After completing each user story
- `/lint` - Before committing to catch issues
- `/test` - Run test suite after changes
- `agent-browser` - For manual UI verification (screenshots, interaction testing)

### Browser Automation for UI Verification

For stories with UI components, use `agent-browser` to verify:
```bash
agent-browser open http://localhost:4646/ui/
agent-browser snapshot -i          # Get interactive elements
agent-browser click @e1            # Test interactions
agent-browser screenshot verify.png # Capture for review
```

---

## Decisions Log

All open questions have been resolved:

| Question | Decision | Date |
|----------|----------|------|
| Chart library | **Keep D3.js** - Complex visualizations justify the investment | 2026-01-25 |
| Code Editor | **Upgrade to CodeMirror 6** - Modern architecture, better tree-shaking | 2026-01-25 |
| Real-time strategy | **Hybrid: SSE + Polling** - SSE for logs/events, HTMX polling for lists | 2026-01-25 |
| Icon library | **Heroicons** - Tailwind ecosystem, well-maintained | 2026-01-25 |
| Rollout strategy | **Hard cutover** - Clean release, no parallel UI maintenance | 2026-01-25 |
| Browser support | **2020+ browsers** - Chrome/Edge 80+, Firefox 78+, Safari 14+ | 2026-01-25 |

---

## Appendix: Component Mapping

### Current HDS Components → New Implementation

| HDS Component | Usage Count | New Implementation |
|---------------|-------------|-------------------|
| Hds::Button | ~20 | Templ `Button` component |
| Hds::Badge | ~15 | Templ `Badge` component |
| Hds::Alert | ~10 | Templ `Alert` component |
| Hds::Toast | ~5 | Alpine.js toast system |
| Hds::Modal | ~8 | Alpine.js + Templ modal |
| Hds::Table | ~12 | Templ `Table` component |
| Hds::Form::* | ~25 | Templ form components |
| Hds::Link | ~30 | Templ `Link` component |

### Current Bulma Components → Tailwind

| Bulma Class | Tailwind Equivalent |
|-------------|---------------------|
| `.button` | `btn btn-primary` (custom) |
| `.card` | `rounded-lg shadow bg-white` |
| `.table` | `table-auto w-full` |
| `.navbar` | `flex items-center` |
| `.columns` | `grid grid-cols-*` |
| `.is-flex` | `flex` |
| `.has-text-*` | `text-*` |
