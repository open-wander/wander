# Task-group scaling-policy bounds

## Context

The task-group page already renders autoscaling minimum and maximum values and its Alpine component rejects invalid counts. The 0.1.0 revival checklist still lacks browser evidence using a real job whose task group has an enabled scaling policy.

## Research findings

- `getLiveTaskGroupDetail` maps the submitted job's scaling policy into the rendered task-group model.
- `ScalingControls` exposes the live bounds as component data and applies them to the number input.
- `scaleControls` disables decrement/increment and confirmation outside the inclusive range.
- The inherited server independently rejects scale requests below the policy minimum or above its maximum.

## Acceptance criteria

- [x] The disposable smoke job has an enabled scaling policy with deterministic bounds.
- [x] The browser displays the live minimum and maximum.
- [x] The count input exposes matching native `min` and `max` attributes.
- [x] Decrement is disabled at the minimum and increment is disabled at the maximum.
- [x] Direct input below or above the policy bounds cannot open the confirmation dialog.
- [x] A count inside the bounds retains the existing confirmation behavior.

## Technical approach

1. Add an enabled `min = 1`, `max = 2` scaling block to the existing smoke fixture.
2. Extend the supported Chromium release test to exercise both inclusive boundaries and invalid direct input.
3. Keep handler and Alpine behavior unchanged unless the live test exposes a defect.

## Testing strategy

- Format and validate the updated HCL fixture by registering it with the local development cluster.
- Run UI Go tests and the Playwright test discovery/type-loading check.
- Run the focused browser scenario against a live embedded cluster where practical.
- Scan changed files for new duplication before committing.

## Risks and mitigations

- The smoke fixture is shared by several tests; keep its initial count inside the new bounds and avoid requiring an external autoscaler.
- Browser-only validation is bypassable, but the server remains authoritative and already enforces the same policy bounds.
