# Job-linked Variables Parity

## Context

The approved Nomad 1.6.5 black-box capture shows a Variables tab on job detail
pages. Wander already supports live variable CRUD and the inherited reserved
`nomad/jobs/{jobID}` path contract, but job pages do not expose those linked
variables. HCL submission variables are a separate concern and remain in the
Definition editor.

This work uses the inherited Wander API client, the recorded black-box route,
and Wander-owned template patterns. It does not use later Nomad UI source.

## Research Findings

- The inherited Variables API supports metadata listing by prefix.
- Job-scoped variables may exist at `nomad/jobs/{jobID}` or below it for task
  groups and tasks.
- A raw prefix query can also return a different job whose ID merely starts
  with the requested ID, so the UI must enforce an exact path-segment boundary.
- Global variable detail routes already provide read/edit/delete behavior.
- Existing job tabs propagate cluster API failures through a shared truthful
  error renderer and preserve namespace, region, and ACL-token scope.

## Acceptance Criteria

- [x] Job detail includes a Variables tab and supports direct full-page and
      HTMX navigation at `/ui/jobs/{id}/variables`.
- [x] The read carries request namespace, region, and ACL token scope.
- [x] Only the exact `nomad/jobs/{jobID}` path and descendants are shown; job
      IDs sharing a string prefix are excluded.
- [x] Results are deterministic, link to existing variable detail routes, and
      preserve namespace and region in those links.
- [x] Empty, unavailable, and denied states remain truthful; API failures are
      never rendered as an empty variable list.
- [x] Focused handler/template tests and a live disposable-cluster Chromium
      fixture cover the behavior.
- [x] Full UI tests, vet/lint, TypeScript checks, Chromium, and Deslop pass.

## Technical Approach

1. Add the job Variables tab to path parsing, tab dispatch, and job detail
   navigation.
2. Query variable metadata using the reserved job prefix and request-scoped
   query options, then enforce the exact path boundary locally.
3. Project metadata into a deterministic presentation model with scoped detail
   URLs and render it using the existing table components.
4. Add focused API-contract and rendering tests for scope, ordering, prefix
   collisions, links, empty results, and propagated errors.
5. Register a real job-linked variable in the disposable cluster and exercise
   the tab in the release Chromium suite.

## Testing Strategy

- Handler tests with an HTTP API fixture for query scope and failure behavior.
- Template assertions for ordering, empty state, and scoped detail links.
- Full UI Go tests, UI vet/lint, Playwright typecheck, and supported Chromium
  release smoke.
- Deslop review over each changed source file.

## Risks and Mitigations

- Prefix collisions could disclose another job's metadata. Enforce either an
  exact path match or a following slash before projecting results.
- Variable values are not required for this list and may be sensitive. Use
  metadata listing only and leave item reads to the existing detail route.
- Scope can be lost on navigation from a direct URL. Build detail links with
  the selected namespace and region in the handler.
