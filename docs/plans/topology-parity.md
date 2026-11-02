# Topology Parity

## Context

The topology page currently projects live node capacity and active allocation
reservations, but it only renders an unfiltered collection of node cards. The
approved Nomad 1.6.5 black-box evidence records Metrics and Allocation Status
controls together with Node Pool, Datacenter, Class, State, and Version
controls. The revival checklist also requires allocation overlays, selection,
edge visualization, allocation-level granularity, and deterministic coverage
for multiple datacenters and nodes.

Wander already reads resource-bearing node and allocation records. This unit
will project that existing data into a server-rendered, accessible topology;
it will not copy excluded UI source, introduce a new JavaScript dependency, or
claim that allocation reservations are host utilization.

## Research Findings

- `ui/handlers/topology.go` already performs region/token-scoped node and
  allocation reads and calculates active allocation reservations.
- `ui/templates/partials/topologypartials/list_content.templ` renders the
  current node cards but has no filtering, selection, or allocation detail.
- The unused D3 component is not mounted by the topology page and depends on a
  global D3 runtime. The shipped page can express the required relationships
  with semantic HTML and CSS without adding runtime or npm dependencies.
- Existing jobs, clients, and allocations pages provide the project pattern
  for GET filters, HTMX replacement, scoped URLs, deterministic options, and
  truthful empty states.
- Allocation list stubs contain the job, task group, namespace, client status,
  scheduling status, resource reservation, and node relationship needed for
  the overlay without additional API reads.

## Acceptance Criteria

- [x] Expose Metrics, Allocation Status, Node Pool, Datacenter, Class, State,
      and Version controls with URL-persistent, composable GET filters.
- [x] Preserve region, namespace, and ACL scope on API reads and rendered
      navigation.
- [x] Render deterministic datacenter, node, and allocation ordering for
      multiple datacenters and multiple nodes.
- [x] Show allocation-level job/task-group/status/resource data beneath its
      client with an explicit visual and semantic node-to-allocation edge.
- [x] Allow a client or allocation to be selected through the URL, highlight
      the selection, and expose scoped detail navigation.
- [x] Keep cluster metrics based on the complete scoped data set while clearly
      labelling filtered result counts and reservation metrics.
- [x] Render truthful empty-filter and no-allocation states.
- [x] Cover projection, filtering, ordering, scope, selection, and the browser
      workflow with handler/template and embedded Chromium tests.

## Technical Approach

1. Extend the topology projection with filter state/options, allocation
   summaries, selection state, scoped links, and visible-result counts.
2. Build filter options from the complete scoped node/allocation reads before
   applying filters. Validate the metric value against the supported set.
3. Filter nodes by node metadata and allocation overlays by client status.
   Sort datacenters and nodes by stable names/IDs and allocations by job,
   task group, then ID.
4. Add a GET/HTMX filter form and server-rendered node-to-allocation graph.
   Use borders and accessible labels for edges rather than a canvas-only
   representation.
5. Retain selection and all filter/scope parameters in topology links. Keep
   separate links to open client and allocation details.
6. Add a multi-datacenter fixture and browser assertions, then update the
   revival checklist and parity matrix after all gates pass.

## Testing Strategy

- Handler tests: multi-datacenter projection, deterministic ordering, filter
  composition, allocation status, metric validation, selection, scope, and
  JSON data projection.
- Template tests: filter labels, empty states, relationship labels, selected
  styling markers, and allocation-level links.
- Browser smoke: exercise the baseline filter controls and select a live node
  and allocation without console errors.
- Gates: templ generation/formatting, focused and complete UI Go tests,
  golangci-lint, gosec, govulncheck, Deslop, and embedded Chromium.

## Risks and Mitigations

- Filtering allocations must not silently change cluster totals. Keep summary
  metrics tied to the complete scoped reads and label the visible counts.
- Terminal allocations can overwhelm a busy node. The allocation-status
  control defaults to active (`pending` and `running`) overlays while allowing
  an explicit all or exact-status view.
- A selected entity may be excluded by another filter. Do not fabricate it;
  render the filtered empty state and retain the URL selection for correction.
- Resource-bearing list calls can fail independently. Preserve the existing
  all-or-nothing behavior in this unit and address richer partial-read states
  under the global UI error-state checklist.
