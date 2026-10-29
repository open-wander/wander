# Job list columns and filters parity

## Context

Wander 0.1.0 must retain the Nomad 1.6.5 OSS job-list capabilities while
remaining an independently implemented UI. The current Wander page renders
only name, status, type, priority, and submission time, and exposes only a
free-text search. This leaves the revival checklist's job-list parity item
incomplete.

## Research findings

- Approved Nomad 1.6.5 black-box evidence records Type, Node Pool, Status,
  Datacenter, Prefix, and page-size controls, plus sortable name, status, type,
  node-pool, and priority fields.
- The existing clean-room comparison inventory additionally records namespace,
  task-group count, allocation summary, pack indication, and multi-value type
  and status filtering as missing.
- `api.JobListStub` already carries namespace, datacenters, status, type,
  priority, allocation summaries, and arbitrary job metadata in one list
  response. No per-job request is needed.
- The server-side list stub emits `NodePool`, but the public Go API list stub in
  this branch does not decode it. Adding the matching field restores data the
  server already returns.
- Nomad Pack metadata uses dotted keys including `pack.name` and
  `pack.version`. A Pack marker is applicable only when `pack.name` is present;
  the UI must not infer or fabricate it.
- Prefix filtering maps to job-ID prefix matching. Status and type accept
  repeated query values so callers can select more than one value.

## Acceptance criteria

- [x] The jobs list renders namespace, datacenters, status, type, node pool,
      task-group count, an allocation-state summary, and truthful Pack metadata.
- [x] Status and type support multiple selected values.
- [x] Datacenter, job-ID prefix, and node-pool filters work and compose with
      search, sorting, and pagination.
- [x] Filter choices are derived from the unfiltered live response and no
      unavailable value is fabricated.
- [x] The list path still performs a single jobs-list API request.
- [x] Unit/render tests cover field conversion, summary aggregation, composed
      filtering, query preservation, and Pack/no-Pack states.
- [x] A live Chromium check covers the controls and representative row data.
- [x] The revival checklist and route parity matrix record the completed scope
      and any explicit limitation.

## Technical approach

1. Add `NodePool` to the API client list stub so the existing server response
   is decoded.
2. Expand the UI `JobSummary` projection with the live list fields and aggregate
   allocation counts from `JobSummary.Summary`.
3. Introduce explicit job-list filter and option structs. Apply all UI filters
   locally to the single scoped list response; retain the existing raw API
   `filter` parameter for compatibility.
4. Render a GET/HTMX filter form with repeated status/type values, single
   datacenter and node-pool selectors, prefix and search inputs, and a clear
   action. Preserve filter state in sort and pagination URLs.
5. Use the existing horizontally scrollable table component for the expanded
   columns and compact row presentation.

## Testing

- Focused API client decoding test for `NodePool`.
- Handler tests using one fake `/v1/jobs` response with several jobs and
  combined query parameters.
- Template/render assertions for expanded columns, summaries, Pack marker, and
  selected filter values.
- `go test` for the affected API and UI packages.
- Supported live Chromium test against the disposable development cluster.
- Deslop scan of every changed source file before handoff.

## Risks

- A wide parity table can overflow smaller viewports. Keep it inside the shared
  horizontal-scroll table wrapper and use compact cells.
- Empty or older responses may omit summaries, metadata, datacenters, or node
  pool. Render an em dash or zero counts rather than guessing.
- Repeated query values must survive every sort and pagination transition;
  centralize URL construction around `url.Values`.
