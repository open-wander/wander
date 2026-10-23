# CI (unit testing)

This README describes how the core GitHub Actions CI provides Wander with
continuous integration testing.

## Release artifact checks

The build workflow verifies that every release archive contains exactly one
Wander executable built with the embedded UI, the Wander license, generated Go
dependency material, and the retained browser notices. The same notice bundle
is installed into OCI images and native Linux packages. Its Linux AMD64 job
additionally:

1. exercises the root redirect, UI assets, API, CSP, and disabled-UI behavior;
2. starts a clean three-server Wander cluster and performs a rolling restart;
3. rolls a Nomad 1.6.5 cluster forward to Wander and back to Nomad 1.6.5; and
4. verifies persisted state after every server replacement.

The compatibility baseline is downloaded from HashiCorp's release service and
checked against a pinned SHA-256 digest before use. The rehearsal itself is
`ci/rehearse-release-lifecycle.sh`; it accepts the Wander archive and Nomad
1.6.5 archive as arguments, so the same check can be run against local release
artifacts.

## License boundary check

The security job runs `ci/check-license-boundary.sh` before dependency scans.
It rejects implementation files containing BUSL 1.1 or Business Source License
headers. It also verifies that consul-template's optional Vault Kubernetes
authentication dependency resolves to the approved MPL-2.0 `v0.4.1` release,
rather than the BUSL-1.1 `v0.5.0` requested transitively, and rejects the
incompletely licensed `go-spin` package from the embedded-UI release closure.
These checks prevent accidental reintroduction of the known prohibited
boundary, but do not replace the provenance review recorded in
`docs/plans/license-provenance-audit.md`.

## Steps

1. When a branch is pushed, GHA triggers `.github/workflows/test-core.yaml`.

2. The first job is `mods` which creates a pre-cache of Go modules.
  - Only useful for the followup jobs on Linux runners
  - Is keyed on `hash(go.sum)`, so a cache is re-used until deps are modified.

3. The `checks`, `test-api`, `test-*` jobs are started.
  - The checks job runs `make check`
  - The test job runs groups of tests, see below

3i. The check step also runs `make missing`
  - Invokes `tools/missing` to scan `ci/test-core.json` and the Wander source.
  - Fails the build if any packages in Nomad are not covered.

4a. The `test-*` jobs are run.
  - Configured as a matrix of "groups"; each group is a set of packages.
  - The GHA invokes `test-wander` with $GOTEST_GROUP for each group.
  - The makefile uses `tools/missing` to translate the group into packages
  - Package groups are configured in `ci/test-core.json`

4b. The `test-api` job is run.
  - Because `api` is a submodule, invokation of test command is special.
  - The GHA invokes `test-wander-module` with the name of the submodule.

5. The `compile` jobs are run
  - Waits on checks to complete first
  - Runs on each of `linux`, `macos`, `windows`
