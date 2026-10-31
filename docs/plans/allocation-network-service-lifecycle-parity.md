# Allocation Network, Service, and Lifecycle Parity

## Context

Allocation detail currently shows task state and reserved resources, but omits
the allocated network ports, live service registrations, and task lifecycle
hooks exposed by the inherited API. This is the next bounded part of allocation
detail parity for the 0.1.0 revival.

This work uses inherited API endpoints and Wander-owned templates only. It does
not use later Nomad UI source.

## Research Findings

- Allocation `Info` includes allocated shared ports and the embedded job
  definition needed to associate lifecycle hooks with task state.
- Live service registrations are available from the inherited allocation
  services endpoint and can fail independently of allocation `Info`.
- Namespace, region, and ACL token must be preserved on both reads.
- The existing smoke job already has a dynamic port, a Nomad service, and a
  prestart task, so browser coverage does not need a second fixture.

## Acceptance Criteria

- [x] Allocation detail shows allocated ports in deterministic order, including
      label, host address and port, and mapped task port when present.
- [x] Allocation detail shows live service registrations with scoped service
      links, address, port, and tags.
- [x] Task cards identify lifecycle hook and sidecar status from the allocation's
      embedded job definition.
- [x] Empty states are explicit and a failed service read remains truthful
      without hiding the rest of the allocation.
- [x] Focused fixture/template tests and the live Chromium allocation scenario
      pass, followed by the relevant full checks.
- [x] Deslop reports no newly added duplication for this change. Two warning-level
      test patterns were restructured; the final changed-test scan reports only
      informational loose similarities, and the changed browser range reports
      no warning/error clusters.

## Technical Approach

1. Add small presentation models for ports and services and lifecycle fields on
   task state.
2. Scope allocation `Info` and `Services` reads to the request namespace,
   region, and ACL token.
3. Match the allocation task group and task names in the embedded job, then
   project lifecycle data without inventing defaults.
4. Sort projected ports and registrations deterministically and render them in
   the Tasks tab with explicit empty/error states.
5. Cover projection, scope, ordering, degraded service reads, and rendered
   output with API fixtures; exercise the live smoke allocation in Chromium.

## Testing Strategy

- Handler fixtures assert request scope, deterministic projection, lifecycle
  matching, and a non-fatal services endpoint failure.
- Template tests assert fields, scoped links, and empty/error states.
- The disposable-cluster browser test asserts the smoke allocation's port,
  service, and prestart lifecycle marker.
- Run UI tests, vet, lint, TypeScript checking, and Deslop.

## Risks and Mitigations

- Embedded job data may be absent on historical allocations. Render lifecycle
  only when the API provides it.
- Service registration reads can fail due to ACLs or transient cluster state.
  Keep that section independently available and show the read failure.
- Allocated port addresses can be empty. Present a neutral fallback instead of
  constructing a misleading endpoint.
