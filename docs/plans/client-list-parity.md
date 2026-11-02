# Client List Parity

## Context

The Wander client list renders live nodes and allocated resources, but its
sortable headers do not currently affect server-rendered order and the page
has no pagination. The approved Nomad 1.6.5 black-box evidence also records
State, Node Pool, Class, Datacenter, Version, Volume, and page-size controls
that Wander does not yet expose completely.

This unit restores the list contract only. Client-detail drain strategy,
allocation filtering, host-volume detail, driver attributes, and metadata
editing remain separately testable units.

## Research Findings

- The node list response already contains node pool, version, drivers, and
  host-volume data on the server-side `NodeListStub`.
- Wander's public API `NodeListStub` omitted `HostVolumes`, so the field was
  silently discarded during JSON decoding even though `HostVolumeInfo` already
  exists in the API package.
- `ui/handlers/clients.go` already makes one resource-bearing node-list read
  and one allocation-list read; no per-node request is needed for list parity.
- The allocation-list unit established the compact filter, deterministic sort,
  pagination, truthful error, and scoped-link patterns to reuse here.
- Approved behavior evidence is recorded in
  `docs/plans/nomad-1.6.5-ui-blackbox-evidence.md`; excluded UI source is not
  required.

## Acceptance Criteria

- [x] Decode host-volume metadata from the existing node-list response.
- [x] Render state, node pool, class, datacenter, version, volumes, running
      allocations, and allocated capacity from live data.
- [x] Filter by state, node pool, class, datacenter, version, and volume.
- [x] Apply deterministic sorting and 10/25/50/100 row pagination.
- [x] Preserve namespace, region, and ACL scope through reads, filters,
      sorting, pagination, clearing, and detail navigation.
- [x] Keep node-list failures distinct from a genuinely empty cluster while
      retaining truthful unavailable allocation-resource cells.
- [x] Cover filter option completeness, composed filters, sorting, pagination,
      host volumes, scoping, and empty/denied/unavailable states.

## Technical Approach

1. Add the missing `HostVolumes` decode field to the public node-list stub and
   protect it with an API JSON regression test.
2. Project list fields and scoped detail URLs from the two existing API reads.
3. Derive complete filter choices before applying local filters, sorting, and
   pagination.
4. Replace cosmetic per-control HTMX filtering with one compact GET/HTMX form
   and render the inherited columns plus shared pagination controls.
5. Add handler/rendering and embedded Chromium coverage, then update the
   parity inventory without marking the remaining detail mutations complete.

## Testing Strategy

- API test: host-volume JSON survives `NodeListStub` decoding.
- Handler/rendering tests: projection, composed filters, deterministic order,
  page size, scope, empty state, permission denial, and cluster failure.
- Chromium: live node-pool/version controls, columns, scoped navigation, and
  filter submission in the disposable cluster.

## Risks and Mitigations

- Allocation reads can fail independently of node reads. Keep the node rows
  visible and label allocation/resource cells unavailable, matching current
  truthful partial-failure behavior.
- A node may advertise many volumes. Render a compact sorted name list and
  preserve full names in filter options rather than exposing host paths.
- Node lists are cluster-scoped while allocation visibility can be namespace-
  scoped. Preserve both selected region and namespace on interactions and use
  the same scope for the allocation summary read.
