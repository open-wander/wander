# Wander Release Tag Namespace

## Context

The repository retains historical Nomad tags for provenance. Some of those
tags, including `v0.1.1` and `v0.1.2`, overlap Wander's independent product
version sequence. Reusing or moving them would make historical source
references ambiguous and could cause the release workflow to publish from the
wrong commit.

Wander 0.1.0 predates this policy and remains published as `v0.1.0`.

## Decision

All Wander releases after 0.1.0 use the Git tag namespace
`wander-v<version>`. For example, Wander 0.1.1 is tagged
`wander-v0.1.1`, while the binary and release title continue to report
`0.1.1` and `Wander v0.1.1` respectively.

Historical Nomad tags are immutable and must not be deleted, moved, or reused.
The `PROTO_COMPARE_TAG` setting remains `v0.1.0` because it identifies the
immutable Wander protocol baseline rather than the latest release.

## Acceptance Criteria

- [x] Published releases create a `wander-v<version>` tag.
- [x] Release titles and binary versions do not include the tag namespace.
- [x] Publication fails if either the release or its namespaced tag already
      exists.
- [x] Stable-release changelog bookkeeping records the namespaced tag.
- [x] Historical tags and the Wander 0.1.0 protocol baseline remain unchanged.

## Release Process

1. Run the release workflow from a non-main release branch with the plain
   product version, such as `0.1.1`.
2. The workflow generates the stable binary version and changelog section.
3. Publication creates `wander-v0.1.1` and a release titled
   `Wander v0.1.1`.
4. Post-release bookkeeping advances `LAST_RELEASE` to
   `wander-v0.1.1` for the next changelog calculation.

## Verification

- Parse both workflow files as YAML.
- Assert the build workflow emits the namespaced tag and checks for an
  existing tag before publication.
- Assert the release workflow writes a namespaced `LAST_RELEASE` value.
- Run repository formatting and static-diff checks before commit.
