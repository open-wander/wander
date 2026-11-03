# CI and Release Supply-Chain Policy

## Threat Model

GitHub Actions and release tools execute code with access to the checked-out
source, workflow token, build artifacts, and—in release jobs—package and
release publication permissions. A mutable tag can therefore change release
behavior without a Wander commit. The 0.1.0 pipeline treats every executable
third-party action and container as a release input.

## Policy

- External GitHub Actions are pinned to a full 40-character commit. A version
  comment remains beside the pin for review and Dependabot updates.
- Container actions and compatibility-test images are pinned to an OCI digest.
- Local actions and reusable workflows use relative paths and are covered by
  the same Wander commit under test.
- Go build and release tools use explicit module versions or commits. In
  particular, `go-changelog` no longer uses `@latest`.
- npm inputs use committed lockfiles and `npm ci`; Go inputs use `go.mod` and
  `go.sum`; the Nomad 1.6.5 lifecycle baseline is verified by SHA-256 before
  use.
- `ci/check-workflow-pins.sh` rejects new mutable action, container-action, or
  `go install` references in active workflows and composite actions.

Dependabot's weekly `github-actions` configuration covers the repository root
and `.github/actions`. Pin updates must retain the readable version comment,
pass actionlint, and be reviewed like source changes.

## Residual Trust Boundaries

GitHub-hosted runner images and operating-system package repositories remain
managed external services rather than byte-for-byte pinned inputs. They do not
publish by themselves: release publication still depends on the pinned action
graph, explicit workflow permissions, archive verification, and the exact
build commit. Disabled inherited workflows are not executable and must pass
this policy before they are re-enabled.

## Acceptance Criteria

- [x] Every external action in an active workflow or composite action is pinned
      to a commit or digest.
- [x] The actionlint and minimum-OS containers are digest-pinned.
- [x] Mutable `@latest` release-tool installation is removed.
- [x] CI enforces the policy and Dependabot manages reviewed pin updates.
