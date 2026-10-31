# Vault token retry test repair

## Context

`TestTaskRunner_DeriveToken_Retry` reliably times out because it waits for its
mock batch task to stop, but the test does not give the mock driver the
zero-duration configuration used by the adjacent Vault lifecycle tests. The
production retry path succeeds and starts the task; the stale fixture then
leaves the task running until cleanup kills it.

## Plan

- Pin the test task's mock-driver `run_for` value to `0s` so the fixture
  matches the lifecycle asserted by the test.
- Preserve assertions that the recoverable error is retried, the derived token
  is written, the task exits successfully, and the token is revoked.
- Run the isolated test repeatedly, then the Vault-token lifecycle group and
  task-runner package vet. Record unrelated package-wide prerequisites rather
  than treating them as failures of this repair.
- Mark the revival checklist blocker complete only after those checks pass.

## Acceptance criteria

- The retry test passes repeatedly without changing production task-runner
  behavior.
- The Vault-token lifecycle tests and affected package vet pass.
- Deslop reports no new duplication caused by the change.

## Validation outcome

- The isolated retry test passed five consecutive runs, followed by three
  more after the final fixture form was selected.
- The four Vault-token lifecycle tests passed twice together, followed by one
  final group run.
- `go vet ./client/allocrunner/taskrunner` passed.
- After Consul 2.1.0 was installed, the package exposed a separate
  `TestTaskRunner_CheckWatcher_Restart` timeout: its three intentional restart
  cycles exceed the generic five-second state wait. The test now uses the
  established 15-second task-runner deadline.
- The corrected check-watcher test passed five consecutive runs. The complete
  `client/allocrunner/taskrunner` package and package vet then passed with
  Consul 2.1.0 available.
