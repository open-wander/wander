# Wander Release Readiness

## Context

Wander is an independent MPL-2.0 fork whose core behavior is based on the
existing Nomad 1.6-era code. It must not inherit source code from later Nomad
releases. The immediate goal is to produce a secure, usable release with the
new Go/Templ/HTMX UI before adding new scheduler or orchestration features.

## Boundaries

- Do not copy or merge code from later BUSL-licensed Nomad releases.
- Follow the [compatibility boundary](../compatibility-boundary.md): new
  Wander-owned state and APIs use Wander names, while inherited wire and state
  identifiers remain available for compatibility.
- Preserve existing state and API compatibility unless a security fix requires
  a documented break.
- Treat the `ui-modernization-htmx` branch as the release-candidate UI line.
- Keep unrelated working-tree changes intact.
- Treat the
  [Nomad 1.6.5 drop-in contract](nomad-1.6.5-drop-in-compatibility.md) as a
  release gate. New behavior belongs after 0.1.0 unless required for security
  or for the rewritten UI to provide inherited functionality.

## Acceptance Criteria

- [x] `govulncheck` reports no vulnerabilities in reachable Wander code.
- [x] Supported Go toolchains are documented and used consistently by CI.
- [x] The root and API module test suites pass on supported platforms.
- [x] UI handlers do not panic and all UI unit/integration tests pass.
- [x] Every visible mutating UI action either performs the operation or is
      removed/disabled; none may return a false success response.
- [x] Browser token handling is safe for an HTTPS production deployment.
- [x] A development cluster passes API and embedded-UI smoke tests.
- [x] Release artifacts, containers, and documentation consistently use the
      Wander name.
- [x] CI gates dependency, vulnerability, generated-code, and test regressions.
- [x] Backup and restore are tested before the release is called production
      ready.

## Outstanding Before 0.1.0

The complete and authoritative release work list is the
[Wander 0.1.0 Revival Checklist](0.1.0-revival-checklist.md). The items below
are the original release-engineering summary and must not be read as the full
UI parity inventory.

- [ ] Build the release artifacts with the `ui` build tag and verify each
  supported archive contains one Wander binary with the HTMX UI embedded.
- [x] Smoke-test an extracted release binary: `/` redirects to `/ui/`, and
  `/ui/` serves the current Wander UI without a separate web process. The
  Darwin ARM64 release-format archive passed locally on 2026-10-18; CI applies
  the same archive and runtime checks to Linux AMD64.
- [x] Complete the ACL-enabled UI mutation matrix listed in the UI feature
  status document. The matrix is enforced by `TestACLMutations` in CI.
- [x] Verify allocation logs and exec through HTTPS/WSS with a real workload.
  `TestTLSTransports` reads live logs over HTTPS/SSE and completes a shell
  command through WSS using the repository test CA.
- [x] Complete keyboard-only and basic screen-reader checks. The Chromium
  release smoke verifies landmarks, accessible control names and form labels,
  unique IDs, and keyboard-only navigation into the jobs page.
- [x] Run the clean-install, three-server restart, upgrade, and rollback
  rehearsals using packaged release artifacts. The Darwin ARM64 rehearsal
  passed locally on 2026-10-19; the Linux AMD64 build repeats it in CI.
- [x] Explicitly retire the prototype-era Playwright specifications, page
  objects, and visual baselines. They remain as historical reference but are
  excluded by `testMatch`; all supported commands run the live-cluster release
  smoke specification.
- [x] Establish a Wander-owned changelog process with the v1.6.5 fork
      boundary and mandatory release-note generation.

## Technical Approach

1. Upgrade the Go toolchain and vulnerable dependencies in small compatible
   groups, running focused tests and `govulncheck` after each group.
2. Stabilize UI rendering and routing, beginning with allocation handler
   failures and panic paths.
3. Implement or disable placeholder job and client actions, with tests for
   successful and failed API calls.
4. Harden browser authentication and add production configuration tests.
5. Repair packaging and CI, then test a clean install and multi-node lifecycle.
6. Publish a release candidate only after the acceptance criteria above pass.

## Testing Strategy

- Unit: affected Go packages after every dependency or handler change.
- Integration: root module, API module, UI routes, ACL behavior, and generated
  templates.
- Security: `govulncheck`, npm audit for shipped/build inputs, and static
  analysis in CI.
- Manual: build a clean binary, start a development cluster, submit a job, view
  logs, exercise allocation actions, and verify the UI over HTTPS.
- Operational: three-server restart, snapshot, restore, and rollback rehearsal.

## Risks and Mitigations

- Dependency upgrades can alter runtime behavior: upgrade by subsystem and
  retain regression tests for each incompatibility found.
- Old transitive packages may be abandoned: replace only where an in-place
  secure upgrade is unavailable.
- UI tests currently mix generated output and runtime behavior: make fixtures
  deterministic and prevent tests from rewriting tracked reports.
- Later upstream fixes cannot simply be copied: independently analyze and
  implement required behavior from public vulnerability descriptions.
