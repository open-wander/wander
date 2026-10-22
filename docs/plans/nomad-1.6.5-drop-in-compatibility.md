# Nomad 1.6.5 Drop-in Compatibility Contract

## Goal

Wander 0.1.0 is a drop-in replacement for the inherited Nomad 1.6.5
codebase, with Wander branding and the rewritten UI. Feature development that
changes this operational contract starts only after a stable 0.1.0 release.

The behavioral compatibility baseline is the repository's local `v1.6.5` tag
and packaged runtime. It is not blanket authorization to reuse every source
file in that tag: the tag contains mixed-license UI files, including files
marked `BUSL-1.1`. Only source whose file-level license and provenance are
approved for Wander may be used as implementation source. BUSL-marked source
must not be copied into Wander without explicit permission. Source from later
Nomad versions must not be copied, merged, or used as an implementation
reference.

## UI runtime contract

The local `v1.6.5` tag establishes this behavior:

- Official release archives contain one executable with the UI embedded.
- The embedded UI is enabled by default.
- The existing `ui { enabled = false }` agent configuration disables it.
- There is no inherited runtime `-ui` command-line flag.
- An exact request for `/` receives a 307 redirect to `/ui/` and retains its
  query string.
- `/ui/` is the canonical browser path and does not require a separate UI
  process, asset server, or network download.
- Unknown paths outside registered API and UI routes return 404.
- The configured UI content-security-policy header applies to UI pages and
  assets.

Wander retains those semantics. The UI implementation and visible branding
may change, but operators must not need new flags, configuration, services, or
deployment topology.

## Intentional 0.1.0 differences

The approved differences from Nomad 1.6.5 are limited to:

- the `wander` executable, archive, package, container, service, documentation,
  and Go module names;
- Wander product wording in user-visible output and a rewritten embedded UI;
- Wander-owned aliases such as `WANDER_ADDR`, with inherited `NOMAD_*` names
  retained as compatibility fallbacks;
- Wander-owned browser state, including the `wander_token` cookie; and
- supported toolchain and dependency updates made for security and maintenance.

Inherited API paths, protocol headers, configuration keys, `.nomad` job file
support, plugin identifiers, state keys, and scheduler fingerprints are not
branding and remain unchanged.

## Server and operator contract

Before 0.1.0, compare Wander with the local `v1.6.5` tag and verify:

- command names, flags, exit codes, and signal behavior;
- HCL configuration keys, defaults, and reload behavior;
- inherited `NOMAD_*` environment variables and compatibility aliases;
- default HTTP, RPC, and Serf ports and bind/advertise behavior;
- `/v1/...` API paths, protocol headers, status codes, and response fields;
- job syntax, ACL behavior, plugin protocols, state layout, snapshots, and
  rolling restart behavior.

Wander branding, executable/package names, module paths, and explicitly
Wander-owned browser state are allowed differences. Every other difference
must either be fixed or documented and approved as an intentional 0.1.0
incompatibility.

## Release gates

- [x] Unit tests cover `/` redirect and query retention.
- [x] UI-tagged tests cover default enablement, embedded static assets,
      configured disablement, and the inherited CSP header.
- [ ] Every supported archive contains exactly one `wander` executable and its
      Go build metadata contains the `ui` tag.
- [ ] An extracted Linux archive starts without a separate UI process and
      serves `/`, `/ui/`, a static asset, and `/v1/agent/self` as expected.
- [ ] The same smoke test proves `ui { enabled = false }` returns the disabled
      UI response while the server API remains available.
- [x] A clean install and three-server rolling restart preserve cluster state.
      `ci/rehearse-release-lifecycle.sh` verifies the cluster after every
      server replacement and checks a persisted variable as its state marker.
- [x] Upgrade and rollback rehearsals use packaged artifacts, not development
      binaries. The Darwin ARM64 rehearsal passed locally on 2026-10-19 using
      the Wander release-format archive and HashiCorp's checksum-verified Nomad
      1.6.5 archive; Linux AMD64 repeats the rehearsal in the build workflow.
- [x] The compatibility comparison above has no unexplained deviations. The
      rehearsal compares inherited top-level commands, command flags, and
      representative exit codes; the source inventory retains every inherited
      `NOMAD_*` variable; the remaining approved differences are listed above.
