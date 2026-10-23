# License-safe parity method

## Context

Wander 0.1.0 targets the runtime behavior of the local Nomad 1.6.5 release at
commit `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f`, while remaining an independent
MPL-2.0 project. That tag is a mixed-license behavioral baseline and is not a
blanket source-code baseline.

This is an engineering process, not legal advice. Project-owner approval of
the provenance audit and allowed source baseline remains a release gate.

## Permitted evidence

- Black-box requests, responses, navigation, and operator-visible behavior
  from an unmodified official Nomad 1.6.5 binary.
- The inherited `/v1/...` API contract and MPL-2.0 server/client code after its
  file-level provenance has been approved.
- Public documentation that describes 1.6.5 behavior without exposing later
  implementation source.
- Current Wander source, tests, release artifacts, and Git history after the
  documented BUSL cleanup boundary.
- User-reported behavior and independently written acceptance criteria.

## Prohibited implementation inputs

- The 45 BUSL-1.1 files identified by the provenance audit, including the 37
  files from the former UI.
- Source, patches, commits, generated assets, tests, or implementation details
  from later Nomad releases.
- Copying DOM, templates, JavaScript, styles, route implementations, or tests
  from the mixed-license baseline UI.
- Treating a matching filename, route name, screenshot, or API operation as
  permission to copy its implementation.

## Feature workflow

1. Record the baseline behavior and its evidence in the parity matrix.
2. Describe the behavior as an implementation-neutral acceptance test.
3. Confirm that the inherited API can support it without a new incompatible
   server contract.
4. Implement it independently using Wander's Go, Templ, HTMX, and vendored
   dependency patterns.
5. Test success, empty, forbidden, unavailable, namespace, and region states
   where applicable.
6. Record the test evidence and classification in the matrix.
7. Run the licence-boundary check and Deslop before committing code.

## Decision rules

- A capability present in the approved 1.6.5 behavior is required for 0.1.0
  unless the project owner explicitly approves and documents a deviation.
- BUSL provenance prohibits copying an implementation; it does not silently
  convert inherited behavior into a post-0.1 enhancement.
- Enterprise-only or unavailable behavior must be recorded as not applicable
  with evidence rather than inferred from a missing Wander route.
- New convenience features and new server behavior remain post-0.1.

## Review record

- Engineering method recorded: 2026-10-22.
- Project-owner provenance approval: pending.
- Complete source/dependency engineering inventory: recorded 2026-10-23 in
  `docs/licenses`; blocker remediation and project-owner approval remain
  pending.
