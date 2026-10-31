# Job overview configuration parity

## Context

The first job-overview parity slice restored live allocation totals, recent
allocations, and metadata. The remaining 0.1.0 overview gap is configuration
and scheduler context already present in the Nomad 1.6.5 API: current
placement failures, child-job totals, volumes, task lifecycle hooks, and
job-type-specific scheduling details.

## Acceptance criteria

- [x] Current placement failures are projected from the newest evaluation,
      with an explicit unavailable state when that read fails.
- [x] Periodic and parameterized parents show live child-job totals from the
      job summary, including zero and unavailable states.
- [x] Configured task-group volumes and task lifecycle hooks are visible and
      deterministic.
- [x] Scheduling details show applicable node-pool, batch, parent/dispatched,
      periodic, parameterized, and multiregion information without fabricated
      defaults.
- [x] Ordinary jobs do not receive irrelevant parent/type-specific panels.
- [x] Handler and template tests cover populated, empty, and unavailable
      states.
- [x] The route parity matrix and revival checklist record the restored scope.

## Technical approach

1. Extend the core job projection with configuration that is already returned
   by `Jobs().Info`; do not add server endpoints.
2. Preserve the child summary returned by the existing `Jobs().Summary` read.
3. Share the evaluation projection between the overview and Evaluations tab,
   while allowing the overview to distinguish an empty result from a failed
   read.
4. Render compact, conditional sections so job kinds only show fields that
   apply to them.

## Testing strategy

- Handler projection tests for volumes, lifecycle, job-kind configuration,
  child totals, placement failures, and independent read failure.
- Template tests for populated and irrelevant/empty states.
- Existing embedded-browser job-kind coverage as the regression boundary.
- Focused Go tests, template generation, typecheck, and Deslop review.

## Risks and mitigations

- Historical placement failures can mislead operators. Only use the newest
  evaluation, matching current scheduler context rather than aggregating old
  failures.
- Map-backed volumes and evaluation metrics are unordered. Sort every
  projection before rendering.
- Overview reads may fail independently. Preserve the core job page and mark
  only the affected section unavailable.

## Verification

The focused handler, template, and E2E Go packages pass. The embedded-cluster
Chromium release suite passed on 2026-10-29 with 14 applicable tests passing
and three environment-dependent tests skipped. The live suite covers ordinary,
periodic, and parameterized overview states.
