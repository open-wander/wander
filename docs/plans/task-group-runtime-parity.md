# Task-group Runtime Parity

## Context

Task-group detail now covers inherited configuration and scaling controls, but
does not show the group's live allocations, scheduler scale status, or scaling
event history. These are required to complete the remaining task-group item in
the 0.1.0 revival checklist.

This work uses inherited API endpoints and Wander-owned templates only. It does
not use later Nomad UI source.

## Research Findings

- The existing request-scoped job read returns the API client and query options
  needed for `Jobs().Allocations` and `Jobs().ScaleStatus`.
- Job allocation results contain every task group and must be filtered by an
  exact task-group name before presentation.
- Scale status provides desired, placed, running, healthy, and unhealthy counts
  plus events per task group.
- Runtime reads can fail independently of the job definition, so false empty
  states would be misleading.

## Acceptance Criteria

- [x] Task-group detail lists only that group's allocations with scoped links.
- [x] Status filtering and created/status/node sorting are deterministic and
      preserve namespace and region.
- [x] Scheduler desired/placed/running/healthy/unhealthy counts are shown.
- [x] Scaling events are newest-first and report target, previous count,
      message, error state, evaluation, and time when available.
- [x] Allocation and scale-status failures render truthful section errors while
      the task-group definition remains usable.
- [x] Focused tests and the live Chromium task-group scenario pass. The full UI
      Go/vet/lint/TypeScript gates pass; two complete-suite browser reruns each
      reached a separate pre-existing parallel-test flake after the task-group
      scenario itself passed.
- [x] Deslop reports no newly added duplicate cluster for this change.

## Technical Approach

1. Enrich `TaskGroupDetail` with scoped runtime presentation models.
2. Fetch allocations and scale status after the task group definition is found.
3. Filter and sort allocations from validated query parameters and sort events
   deterministically by scheduler time/index.
4. Render filter/sort controls, allocation rows, scale metrics, event history,
   and distinct empty/error states.
5. Exercise the real smoke allocation, scheduler counts, and filter/sort scope
   in Chromium; cover deterministic event history with the API fixture.

## Testing Strategy

- API fixtures cover exact group filtering, scope, sorting, filters, events,
  and independent failures.
- Template assertions cover controls, links, metrics, empty and error states.
- The disposable-cluster Chromium test covers the running allocation and live
  scheduler status.
- Run full UI tests, vet, lint, TypeScript checking, and Deslop.

## Risks and Mitigations

- Historical allocation volume can be large. This unit uses the inherited job
  allocation endpoint and deterministic client-side filtering without adding a
  second per-row request.
- Scheduler event timestamps are unsigned nanoseconds. Reject values that do
  not fit `int64` instead of overflowing them into incorrect dates.
- One failed runtime endpoint must not hide configuration. Track availability
  independently for allocations and scale status.
