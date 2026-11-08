# Repository Test Baseline Repair

## Context

The repository-wide Go test run currently reports failures outside the website
work. These failures need to be repaired so release work is validated against a
clean, reproducible baseline rather than being waived as unrelated.

The repository's canonical test configuration is serial package execution with
a 20-minute package timeout (`-p=1 -timeout=20m -count=1`). A plain
`go test ./...` runs packages concurrently and produces misleading timeouts for
integration-heavy packages.

## Research Findings

- `GNUmakefile` defines the canonical serial test flags.
- `version/version.go` identifies the current development build as `0.1.1-dev`,
  while `version/version_test.go` still expects `0.1.0-dev`.
- `client/vaultclient` tests launch independent Vault development servers through
  `testutil/vault.go`. Parallel Vault processes currently share Vault's default
  token-file side effect and can collide while writing `~/.vault-token`.
- `client/fingerprint` and `client/allocrunner/taskrunner/template` pass when run
  with the canonical flags; their earlier timeouts came from the non-canonical
  concurrent test invocation.
- `nomad` contained three timing/order defects: cutoff equality in blocked-eval
  pruning, keyring initialization races, and delayed worker state changes that
  could arrive out of order.
- Docker 29 moved configured MAC addresses to per-network endpoint settings and
  changed inspect responses. Docker Desktop also exposes its own DNS and unified
  cgroup namespaces rather than those of the macOS host.
- Several Docker tests encoded obsolete representation details: a mutable
  architecture-specific image ID, an empty bind-options pointer, the host DNS
  resolver, and the legacy top-level inspect MAC field.
- The exec streaming conformance test used a CI-specific root bypass on macOS,
  where its chroot bind mount is unsupported. It now uses the same root-on-Linux
  prerequisite gate as the rest of the exec driver integration suite.
- Java version unit tests replaced the Java command but still invoked macOS's
  real `java_home` probe. They now stub both commands and no longer require a
  locally installed JVM to test synthetic output.
- The `TestNodes_GC` UDP collision was caused by an interrupted earlier test
  process. It passed both the automatic rerun and a clean isolated rerun.

## Acceptance Criteria

- [x] The version test matches the current `0.1.1-dev` source version and retains
      the intended Nomad compatibility assertion.
- [x] Vault integration tests can run without sharing a developer token file or
      colliding on listener setup.
- [x] The exact failing Nomad tests are identified from structured test output and
      repaired at its root cause.
- [x] Docker 29 and Docker Desktop compatibility assertions retain their semantic
      coverage without assuming obsolete daemon representations.
- [x] All affected packages pass using `-p=1 -timeout=20m -count=1`.
- [x] The repository's canonical Wander package groups complete successfully, or
      any remaining failure is documented with exact test evidence.
- [x] New code passes formatting, vet/security checks appropriate to the changed
      files, and the required Deslop scan.

## Technical Approach

1. Update the stale version expectation.
2. Inspect the installed Vault development-server options and remove its
   user-home token-file side effect in the shared test helper.
3. Run heavyweight packages with `go test -json` and filter test lifecycle events
   to identify precise failures without losing them among server logs.
4. Reproduce and fix only confirmed failures; rerun suspected environmental
   collisions independently before changing code.
5. Validate affected packages first, then run the canonical repository target.

## Testing Strategy

- Unit: `go test -p=1 -timeout=20m -count=1 ./version`
- Integration: the same command for `./client/vaultclient` and `./nomad`
- Regression: the canonical Wander test target from `GNUmakefile`
- Static review: `go vet` for changed Go packages and the required Deslop diff
  scan before commit

## Verification Results

- The full canonical run completed 9,890 tests and exposed only three driver
  prerequisite failures; every non-driver package passed its first run.
- After repairing those prerequisites, the canonical `drivers` group passed 443
  tests with 44 intentional platform skips and no reruns or failures.
- The complete Docker package passed independently and within the canonical
  driver group against Docker 29.9.2 on Docker Desktop.
- The root Nomad package passed its canonical run at 77.3% statement coverage;
  the root client package passed at 76.5%.

## Risks and Mitigations

- Vault CLI behavior can vary by release. Verify the installed CLI supports the
  selected development flag before changing the helper.
- Nomad tests exercise real sockets and timing. Diagnose from exact structured
  failure events and avoid weakening assertions or globally increasing timeouts.
- Existing website work is uncommitted. Keep baseline repairs focused and review
  the combined diff carefully before any commit.
