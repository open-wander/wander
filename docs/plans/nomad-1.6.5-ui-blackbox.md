# Nomad 1.6.5 UI Black-box Capture

## Context

Wander 0.1.0 must preserve every applicable Nomad 1.6.5 OSS capability, but
the baseline UI source is excluded from implementation work by the project's
license boundary. Several rows in the UI parity matrix still say `Capture
pending`, so the release cannot yet prove its route and behavior coverage.

This audit observes an unmodified official Nomad 1.6.5 binary only. It must not
inspect, extract, decompile, or copy the baseline UI's bundled source, DOM
implementation, templates, scripts, styles, or tests. Recorded evidence is
limited to operator-visible navigation, URLs, labels, states, and network API
operations.

## Research findings

- The license-safe parity method explicitly permits black-box requests,
  responses, navigation, and operator-visible behavior from the official
  binary.
- HashiCorp still publishes signed Nomad 1.6.5 archives and checksums for
  Darwin ARM64, the local test platform.
- The existing Wander Chromium harness already provides a pinned browser and
  live job fixtures, so the capture can reuse its installed tooling without
  adding runtime dependencies.
- Official Nomad UI documentation confirms `/ui`, job detail sub-routes,
  allocation/client navigation, topology, optimization, ACL token settings,
  and inherited keyboard shortcuts; the running 1.6.5 binary remains the
  authoritative version-specific evidence.

## Acceptance criteria

- [x] Download the official Nomad 1.6.5 Darwin ARM64 archive and verify it
      against HashiCorp's published SHA-256 manifest.
- [x] Start an isolated Nomad 1.6.5 development agent with disposable state
      and representative service, batch, system, periodic, and parameterized
      jobs where the platform supports them.
- [x] Capture the visible primary navigation, job sub-navigation, allocation,
      client, server, topology, storage/CSI, variable, ACL, and evaluation
      routes without reading bundled UI implementation files.
- [x] Record applicable empty and unavailable/edition-specific states, plus
      browser paths reached by visible controls.
- [x] Record operator-visible keyboard-shortcut behavior.
- [x] Convert every observation into implementation-neutral evidence in the
      parity matrix; replace `Capture pending` where the run supplies evidence.
- [x] Classify enterprise-only or unavailable capabilities from observed
      behavior without inferring implementation details.
- [x] Keep downloaded binaries, extracted files, screenshots, traces, and
      disposable cluster state outside the repository.
- [x] Run documentation checks, the license-boundary check, and Deslop before
      committing any repository changes.

## Technical approach

1. Fetch the archive and checksum manifest from HashiCorp Releases into a
   temporary directory, verify the archive, and print the binary version.
2. Start the binary as a disposable combined development agent on dedicated
   loopback ports with no connection to an existing Wander cluster.
3. Register independently authored fixtures through the public CLI/API.
4. Drive Chromium through visible controls. Capture final URLs, visible
   navigation labels, headings, tab labels, control names, status text, and
   same-origin `/v1/...` request paths. Do not retain page source or inspect
   JavaScript bundles.
5. Summarize the evidence in a human-reviewable capture record and update the
   route/capability matrix with only directly observed claims.
6. Stop the agent and remove the entire temporary workspace.

## Testing strategy

- Verify the baseline binary version and archive SHA-256 before execution.
- Require every visited route to return a successful UI document or record
  its truthful not-found, denied, or unavailable state.
- Cross-check observed URLs against the public 1.6.5 behavior contract and
  Wander's registered route inventory.
- Review captured labels and URLs manually before changing a parity status.

## Risks and mitigations

- **License contamination:** never open baseline source or static bundles;
  retain only implementation-neutral observations.
- **Version drift:** use the exact official 1.6.5 archive and record its hash
  and `version` output.
- **False absence:** seed representative resources and distinguish an empty
  state from a missing capability.
- **Edition ambiguity:** record what the OSS binary exposes and leave unclear
  enterprise behavior as a decision rather than guessing.
- **Repository contamination:** create all binary, state, browser, and capture
  artifacts in a unique temporary directory and delete it after review.
