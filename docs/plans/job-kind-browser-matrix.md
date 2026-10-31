# Job-kind browser matrix

## Context

Wander 0.1.0 must preserve the behavior of service, batch, system, sysbatch,
periodic, parameterized, and dispatched jobs. The release browser suite already
covers a service job and parameterized dispatch, but it has no durable fixtures
for the remaining kinds. The periodic force API exists, yet the detail page
does not expose it.

## Research findings

- Approved Nomad 1.6.5 black-box evidence records service, batch, system,
  periodic, and parameterized states without consulting excluded UI source.
- The existing smoke fixture covers a running service job.
- The existing parameterized fixture and Chromium test cover its parent and a
  dispatched child, including purge.
- `JobsAPIHandler.ForcePeriodicJob` already performs the scoped live mutation;
  only the periodic marker and detail control are missing from the UI.
- The disposable client supports the inherited `raw_exec` driver, so all job
  kinds can be represented without external images or services.

## Acceptance criteria

- [x] Disposable release tests register batch, system, sysbatch, and periodic
      fixtures in addition to the existing service and parameterized fixtures.
- [x] The jobs list truthfully reports every fixture's live type.
- [x] Detail pages render for every kind without browser errors.
- [x] Running service/system jobs expose stop, completed batch/sysbatch jobs
      expose start and purge, periodic parents expose force launch, and
      parameterized parents expose dispatch.
- [x] Force launch calls the existing live API and reports success.
- [x] The route parity matrix and revival checklist record the completed scope.

## Technical approach

1. Add small independently authored `raw_exec` fixtures for each missing kind
   and register them through the existing release-cluster setup.
2. Project `Job.IsPeriodic()` into `JobDetail`, render a force-launch action and
   confirmation modal, and post to `/api/jobs/{id}/periodic/force`.
3. Add template coverage for periodic-only action visibility and a live
   Chromium matrix for list/detail/action behavior.

## Testing

- Template test for periodic force-launch controls.
- Existing handler tests for the force endpoint plus the live mutation.
- Focused and full Chromium release coverage against the disposable cluster.
- Full UI Go tests, generation, lint, and Deslop review.

The embedded-cluster Chromium release suite passed on 2026-10-29 with 14 tests
passing and three environment-dependent tests skipped as expected.

## Risks

- Batch jobs may still be finishing when the browser opens. Use polling
  assertions rather than sleeps.
- Periodic schedules must not launch unexpectedly during unrelated tests. Use
  an annual schedule and exercise launches explicitly through the force API.
- System fixtures remain active for the suite lifetime; keep their resource
  requests and output minimal.
