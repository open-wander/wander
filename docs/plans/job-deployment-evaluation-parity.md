# Job Deployment and Evaluation Parity

## Context

Wander has live job Deployment and Evaluation tabs plus standalone detail
routes. The tabs currently discard evaluation priority, rely partly on API and
Go map iteration order, and turn API failures into plausible empty histories.
That prevents the revival checklist from treating the inherited job-history
behavior as verified.

This work uses Wander's existing API types and clean-room UI patterns. It does
not use later Nomad source as an implementation reference.

## Research Findings

- Job deployments expose scheduler `CreateIndex`, job version, status, and
  task-group state, but no trustworthy creation timestamp.
- Job evaluations expose `CreateIndex`, priority, status, trigger, creation
  time, and placement failures.
- Standalone evaluation/deployment detail routes already preserve terminal,
  blocked, denied, missing, and unavailable states.
- The job tabs currently swallow API errors, omit explicit region scope,
  discard evaluation priority and scheduler indices, and iterate deployment
  task groups from an unordered map.
- Existing cards do not link to the already-supported detail routes.

## Acceptance Criteria

- [x] Deployment and evaluation reads carry request namespace, region, and ACL
      token scope.
- [x] Backend failures propagate through the job tab error path and are not
      rendered as empty histories.
- [x] Histories are ordered newest-first by scheduler `CreateIndex`, with
      deterministic tie-breaking.
- [x] Deployment task groups are ordered deterministically by name.
- [x] Evaluation cards show priority and both deployment/evaluation cards link
      to their existing detail routes.
- [x] Pending, blocked, failed, complete/successful, and placement-failure
      projections remain truthful.
- [x] Focused handler/template tests, full UI tests, vet/lint, live Chromium,
      and Deslop pass.

## Technical Approach

1. Change job-tab loaders to return errors and let the existing tab error
   renderer map cluster failures truthfully.
2. Preserve API `CreateIndex` and evaluation priority in presentation models.
3. Sort deployments and evaluations by index descending, then stable semantic
   fields and ID; project task-group maps into sorted slices.
4. Add detail links and priority metadata without changing the established
   card layout.
5. Add API-backed fixtures for scope, order, states, deterministic task groups,
   links, and backend errors; extend the disposable-cluster browser smoke.

## Testing Strategy

- Handler tests for API request scope, projection, deterministic ordering, and
  propagated errors.
- Template assertions for priority, status, task-group ordering, and detail
  links.
- Full UI Go tests, UI vet/lint, TypeScript typecheck, and supported Chromium
  release smoke.
- Deslop review over every changed source and generated Go file.

## Risks and Mitigations

- Deployment timestamps cannot be reconstructed from Raft indices. Continue
  showing `Time unavailable`; use indices only for ordering.
- Equal indices should not produce unstable pages. Tie-break using version or
  creation time, then stable IDs.
- Scope regressions could expose the wrong region or namespace. Assert all
  scope query values and the ACL header in focused transport tests.
