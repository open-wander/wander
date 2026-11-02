# Client Detail Read Parity

## Context

The client detail page already renders live node resources, allocations,
events, attributes, metadata, and basic driver health. It still omits the
approved 1.6.5 client-detail read surfaces: allocation filtering, host-volume
details, and complete driver information. Several detail reads and links also
drop the selected namespace or region, and allocation read failures currently
look like an empty node.

This unit restores read-only detail parity. Dynamic metadata editing and the
remaining drain-strategy mutation behavior stay in separately testable units.

## Research Findings

- Approved black-box evidence records allocation filtering, attribute editing,
  and dynamic metadata creation on client detail; excluded UI source is not
  required.
- `api.Node` already exposes `HostVolumes`, full driver attributes and health
  descriptions, so no additional server API is needed.
- `Nodes().Allocations` returns namespace, desired/client status, task group,
  job identity, timestamps, and resource reservations in one scoped read.
- The current detail handler silently converts allocation-read failure to an
  empty slice and does not pass region/namespace to its API reads.

## Acceptance Criteria

- [x] Render sorted host-volume names, paths, and read-only state from live
      node data without inventing paths or volumes.
- [x] Render deterministic driver health descriptions and attributes in
      addition to detected/healthy/version state.
- [x] Filter client allocations by status, namespace, job, and task group,
      while deriving complete filter choices from the unfiltered response.
- [x] Preserve namespace and region through node, allocation, and stats reads,
      filter submissions, allocation links, job links, and client actions.
- [x] Distinguish allocation-read failure from a genuinely allocation-free
      client while retaining the rest of the client detail.
- [x] Cover success, empty, denied/unavailable allocation reads, filtering,
      host volumes, driver attributes, deterministic ordering, and scope.

## Technical Approach

1. Extend the detail projection with ordered host volumes, richer driver data,
   scoped URLs, filter state/options, and allocation availability.
2. Make node allocations return an error separately from their data and pass
   the request namespace/region/token to every detail read.
3. Apply local allocation filters after deriving options from the complete
   scoped response; keep resource totals based on all active allocations.
4. Add compact filter controls and truthful unavailable/empty states to the
   existing static allocations tab.
5. Add handler/template tests and extend the embedded Chromium smoke coverage.

## Testing Strategy

- Handler tests project and sort volume/driver data, propagate API scope, and
  distinguish empty from allocation-read failure.
- Pure tests cover composed allocation filters and complete option derivation.
- Embedded Chromium verifies client-detail filter controls and scoped links
  against the disposable live cluster.

## Risks and Mitigations

- Client allocation visibility is namespace-scoped. Keep the node read
  region-scoped and the allocation read both namespace- and region-scoped.
- Allocation failure must not hide the node itself. Render the detail with an
  explicit unavailable allocation panel and avoid false zero resource usage.
- Driver attribute maps are unordered. Sort drivers and attribute keys before
  rendering so output and tests remain deterministic.
