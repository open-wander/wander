# Wander Naming and Compatibility Checklist

This checklist covers the product rename without breaking the inherited Nomad
1.6.5 operational contract. It replaces the earlier blanket-renaming list,
which incorrectly treated compatibility identifiers as branding defects.

The authoritative release and UI work lists are:

- [Wander 0.1.0 revival checklist](0.1.0-revival-checklist.md)
- [release readiness](release-readiness.md)
- [UI feature status](ui-feature-status.md)
- [compatibility boundary](../compatibility-boundary.md)

## Wander-owned names

- [x] Go module and internal imports use `github.com/open-wander/wander`.
- [x] The executable and release archive contain a `wander` binary.
- [x] Container and release artifact names use Wander.
- [x] The rewritten UI uses Wander titles and user-facing branding.
- [ ] Finish the release documentation, download links, signing identity, and
      Wander-owned support/security contacts before 0.1.0.
- [x] Review current user-facing help and error text for accidental Nomad
      product branding. Historical attribution and compatibility terminology
      are excluded from this check.

## Names retained for drop-in compatibility

Do not mechanically rename these. Existing Nomad 1.6.5 operators, clients,
jobs, plugins, and automation must continue to work with Wander 0.1.0.

- [x] Existing HCL configuration keys and blocks, including `ui`.
- [x] Existing `/v1/...` HTTP paths and `X-Nomad-*` protocol headers.
- [x] Existing job syntax, API fields, RPC names, plugin protocols, state keys,
      and scheduler fingerprints.
- [x] Verify all inherited `NOMAD_*` environment variables remain accepted.
      Wander aliases may be added, but must not replace the inherited names in
      0.1.0. The source inventory matches the local `v1.6.5` tag, including the
      container entrypoint's `NOMAD_SKIP_DOCKER_IMAGE_WARN` alias.
- [x] Verify inherited default ports, data layout, signals, exit codes, and
      command-line flags against the local `v1.6.5` tag. The packaged lifecycle
      rehearsal compares the top-level CLI surface and exercises the inherited
      data layout and interrupt behavior during rolling replacement.
- [x] Document every intentional incompatibility; an undocumented deviation is
      a 0.1.0 release blocker.

## UI packaging contract

- [x] The official release build selects the `ui` build tag.
- [x] The UI is enabled by default and retains `ui { enabled = false }`.
- [x] `/` returns the inherited 307 redirect to `/ui/` and preserves queries.
- [x] `/ui/` and its static assets are served by the Wander agent process.
- [ ] Verify every packaged platform archive contains one Wander executable
      with the UI build tag and embedded assets.
- [ ] Smoke-test the extracted Linux release executable in CI.

## Historical material

`CHANGELOG.md` and `CHANGELOG-unsupported.md` describe inherited Nomad
releases and should retain their historical product names and attribution.
They are not active Wander task lists.
