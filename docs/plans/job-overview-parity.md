# Job overview parity

## Context

The job overview currently renders task groups and basic job identity, but it
does not expose the live allocation summary, recent allocations, or job
metadata required by the 0.1.0 parity checklist. It also contains a fabricated
constraints placeholder instead of a truthful live or empty state.

## Research findings

- The approved Nomad 1.6.5 black-box capture observed job-summary and
  job-allocation API reads on job detail routes.
- Wander's inherited client exposes `Jobs().Summary` and
  `Jobs().Allocations`; no new server API is required.
- The jobs list already converts `api.JobSummary` into the shared
  `AllocationSummary` projection.
- The allocations tab already defines the live allocation-row projection and
  newest-first ordering needed for a compact recent-allocation view.
- Overview-only reads must stay out of definition, version, deployment, and
  evaluation tab requests.

## Acceptance criteria

- [x] The initial job page and overview tab show live allocation totals.
- [x] The overview shows at most five recent allocations, newest first, with
      working allocation links.
- [x] Job metadata is sorted and rendered only from the live job response.
- [x] Missing metadata and missing allocations have truthful empty states.
- [x] Summary or allocation read failures have explicit unavailable states
      without hiding the job header and other tabs.
- [x] The fabricated constraints placeholder is removed.
- [x] Handler, template, and embedded Chromium tests cover the live behavior.
- [x] The route parity matrix and revival checklist record the restored scope.

## Technical approach

1. Extend `JobDetail` with allocation summary, recent allocation, metadata,
   and availability projections.
2. Keep the core job read in `getJobDetail`; add overview enrichment only for
   the initial overview and `/overview` HTMX route.
3. Reuse the existing allocation projection so overview and allocation-tab
   status, links, and timestamps cannot drift.
4. Render compact summary cards, a recent-allocation table, sorted metadata,
   and explicit empty/unavailable states.

## Testing strategy

- Handler test with independent job, summary, and allocations responses.
- Template tests for live, empty, and unavailable states.
- Chromium assertion against the existing `wander-ui-smoke` fixture.
- Focused UI Go tests, template generation, typecheck, and Deslop review.

## Risks and mitigations

- Overview reads can fail independently. Preserve the core job page and mark
  only the affected section unavailable.
- System and batch jobs may produce many allocations. Sort once and retain
  only the five newest rows in the overview projection.
- Metadata is a map and therefore unordered. Sort keys before rendering to
  keep browser output and tests deterministic.

## Verification

The embedded-cluster Chromium release suite passed on 2026-10-29 with 14 tests
passing and three environment-dependent tests skipped as expected. Rapid
Definition-to-Overview keyboard navigation also exposed and fixed a detached
editor initialization race without broadening shortcut behavior.
