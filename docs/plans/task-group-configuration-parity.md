# Task-group Configuration Parity

## Context

The Nomad 1.6.5 black-box capture establishes task-group detail as part of the
release browser contract. Wander's task-group page currently renders scaling
controls and task resource requirements, but omits volume requests and task
lifecycle hooks that are already available in the inherited job definition.

This unit restores those read-only configuration details using the inherited
API client and Wander-owned templates. It does not use later Nomad UI source.

## Research Findings

- `getLiveTaskGroupDetail` already reads the complete task group through the
  request-scoped job endpoint, so no additional cluster request is required.
- The job overview already projects the same `VolumeRequest` and
  `TaskLifecycle` fields, confirming their inherited API semantics.
- Volume requests are stored in a map and therefore require explicit sorting
  for deterministic output.
- Tasks without a lifecycle block must remain distinguishable from prestart,
  poststart, and poststop tasks.

## Acceptance Criteria

- [x] Task-group detail lists configured volume name, type, source, mode,
      attachment mode, and access mode deterministically.
- [x] Every task reports its lifecycle hook and sidecar state, including an
      explicit normal-task state when no lifecycle hook is configured.
- [x] Empty volume state is truthful and does not invent volume configuration.
- [x] Namespace, region, and ACL-token scope continue through the existing
      request-scoped job read.
- [x] Focused handler/template tests and live Chromium coverage pass.
- [x] Full UI tests, vet/lint, TypeScript checks, and Deslop pass.

## Technical Approach

1. Extend the task-group presentation model with volume metadata and lifecycle
   fields.
2. Project inherited task-group data in `getLiveTaskGroupDetail`, sorting map
   keys before rendering.
3. Render a configured-volumes table and lifecycle metadata within each task.
4. Add API-contract, template, and disposable-cluster browser assertions.

## Testing Strategy

- Use an HTTP API fixture to verify request scope and projected values.
- Render the task-group partial to cover configured and empty volume states,
  normal tasks, and lifecycle tasks.
- Add a deterministic prestart task to the disposable smoke job and assert its
  lifecycle metadata in Chromium.
- Run the complete UI Go suite, vet, lint, Playwright typecheck, and Deslop.

## Risks and Mitigations

- A live host or CSI volume fixture would require environment-specific storage
  configuration. Cover volume rendering with the API-contract fixture and keep
  the disposable cluster portable.
- Disposable raw-exec tasks must request at least the inherited 10 MB memory
  minimum. Keep the prestart fixture at that boundary so registration itself
  remains part of the browser regression test.
- Map iteration could make tests and the UI unstable. Sort volume names before
  projecting them.
- Lifecycle metadata can be misleading if absence is rendered as a hook. Use
  an explicit `Normal` label for tasks without a lifecycle block.
