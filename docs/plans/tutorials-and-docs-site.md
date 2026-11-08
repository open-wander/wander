# Wander website and tutorial rebuild

## Context

The Wander homepage is live, but the historical `/wander/docs` and
`/wander/tutorials` routes return 404. The tracked `website/` tree is inherited
reference material rather than a publishable Wander site: it contains 593
tracked files, its package metadata targets Node 14–16, most of its content
still names Nomad or HashiCorp, and `website/LICENSE.md` says the text is
MPL-2.0 while the design is proprietary HashiCorp material.

The replacement must therefore be independently designed and implemented. It
must not copy the inherited layouts, styles, illustrations, screenshots, or
generated site output. Until the replacement is deployed, public repository
links continue to target versioned in-repository documentation.

## Goals and acceptance criteria

- [x] Build a clean, Wander-owned static site in `site/`.
- [x] Reuse Wander's own UI color, type, spacing, icon, and focus conventions.
- [x] Keep the frontend dependency surface small: Go/templ, the existing
  standalone Tailwind CLI, and plain browser JavaScript only where necessary.
- [x] Build useful pages at `/`, `/wander/docs/`,
  `/wander/tutorials/`, and `/wander/tutorials/getting-started/`.
- [x] Make the first tutorial accurate for the released single-binary workflow
  and the Nomad 1.6.5 migration boundary.
- [x] Support keyboard navigation, reduced motion, responsive layouts, and
  light/dark/system color preferences.
- [x] Produce deterministic static output that can be served by any ordinary
  static host without Node.js at runtime.
- [x] Add automated build, structure, and internal-link checks.
- [x] Document preview and deployment without assuming an unconfirmed hosting
  provider.
- [ ] Restore public README links only after the production URLs pass checks.

## Technical approach

The new `site/` tree is separate from the inherited `website/` tree. Templ
components render static HTML through a small Go builder. Tailwind scans only
the new templates and inherits the independently developed Wander UI theme.
The already-managed standalone Tailwind binary compiles CSS; the site adds no
package manifest and no new npm dependency graph. A small, CSP-friendly script
handles the mobile navigation and saved light/dark/system theme preference.

The generated `site/dist/` directory is disposable build output and is not a
source of truth. Initial content is written from the Wander release/runbook
facts already reviewed in this repository. No inherited website content is
migrated implicitly.

Published configuration examples live in `site/examples` so their owning
Wander packages can validate them without importing generated presentation
code. Repository-wide CI uses the aggregate `templ-generate` target before
commands such as `go mod tidy` and `govulncheck ./...` inspect every package.

### Historical tutorial research

The repository contains a pre-1.6.5 getting-started history that is useful as a
coverage checklist. Commit `807c765c6a` removed the six in-repository pages in
December 2020 because the tutorials had moved to HashiCorp Learn. The Nomad
`v1.6.5` tag therefore contains navigation links to Learn, but not the tutorial
bodies. Wander tutorials must use only behavior available in 1.6.5 or earlier,
and must be newly written with new examples and Wander-owned presentation. Old
prose, screenshots, and layouts are not copied automatically.

### Initial publishable slice

1. A product landing page explaining the compatibility boundary and linking to
   releases, documentation, source, discussions, and security reporting.
2. A documentation landing page organised around installing, operating,
   migrating, and contributing.
3. A tutorial index and a getting-started tutorial covering download,
   checksum verification, `wander agent -dev`, the embedded UI, and next steps.
4. Shared navigation, footer, brand mark, theme control, code blocks, callouts,
   and responsive content layouts.
5. Build and link-check commands suitable for local use and CI.

### Tutorial sequence

- [x] Install and run a local development agent.
- [x] Submit, inspect, follow, and stop a first job.
- [x] Explore the job and allocation workflows in the embedded Wander UI.
- [x] Build a disposable multi-node development cluster.
- [ ] Plan a production server/client topology.
- [ ] Migrate a Nomad 1.6.5 cluster with backup and rollback checkpoints.
- [ ] Bootstrap ACLs and manage policies, roles, and tokens.
- [ ] Separate workloads with namespaces and variables.
- [ ] Register and inspect services with Consul-compatible discovery.
- [ ] Use Vault-compatible secrets and document OpenBao once supported.
- [ ] Operate host and CSI volumes.
- [ ] Back up, restore, upgrade, and troubleshoot a cluster.
- [ ] Federate multiple regions.

## Validation

- Generate templ sources and build the static output from a clean checkout.
- Compile minified CSS with the pinned standalone Tailwind toolchain.
- Verify required routes, page titles, landmark structure, and internal links.
- Run Go tests for the site builder and renderer.
- Run the repository static checks that apply to changed files.
- Preview locally at phone, tablet, and desktop widths before publication.
- Run Deslop against the current diff before commit.

## Risks and boundaries

- The deployment target for `openwander.org` has not yet been established in
  this repository. The rebuild can be release-ready before DNS/hosting wiring,
  but README links must not point at it until production is verified.
- The 0.1.0 application UI remains light-only. The documentation site's theme
  control must not imply that the embedded application has dark mode.
- Screenshots can leak tokens, usernames, hostnames, or development-only UI.
  The first slice uses code and original vector composition; screenshots enter
  only through the documented capture and review process below.
- Legacy content may describe features outside the Wander 0.1.x compatibility
  boundary. Retained prose requires behavioral and licensing review.

## Legacy website disposition

- [ ] Inventory tracked text useful for the Nomad 1.6.5 behavioral baseline.
- [ ] Inventory branding, images, generated files, and design material that
  must be removed rather than reused.
- [ ] Confirm provenance and licensing for every retained documentation asset.
- [ ] Rewrite or migrate approved facts into the clean Wander structure.
- [ ] Remove the legacy tree only after retained content has migrated and the
  removal has passed licensing review.

## Publication and operations

- [ ] Confirm the canonical hosting repository, provider, and base path.
- [x] Add CI that builds the site and checks internal links on relevant changes.
- [ ] Publish a preview and verify every canonical route over HTTPS.
- [ ] Record rollback steps for the currently deployed homepage.
- [ ] Restore website links in README only after production verification.

## Tutorial images

- [ ] Capture new screenshots from an official Wander release binary, not a
  development-only UI server.
- [ ] Cover installation, job submission, job definition, allocations, logs,
  exec, files, namespaces, variables, services, and topology.
- [ ] Capture each documented responsive layout and write useful alternative
  text.
- [ ] Record the Wander version, viewport, sample job, and capture procedure.
- [ ] Remove or replace Nomad-branded and obsolete screenshots.
- [ ] Review every image for tokens, hostnames, usernames, and private data.
