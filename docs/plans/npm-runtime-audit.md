# NPM and Node Runtime Audit

## Context

Wander minimizes npm use to browser asset generation and browser tests. The
0.1.0 release must audit those active dependency graphs without implying that
operators need Node.js to run Wander.

## Active Release Inputs

- `ui/package-lock.json` supplies pinned Alpine and Tailwind build inputs. The
  resulting JavaScript and CSS are checked into `ui/static` and embedded by
  `ui/static_embed.go`.
- `ui/tests/e2e/package-lock.json` supplies Playwright and TypeScript for the
  release browser suite. These packages are test-only.

The inherited website, legacy `e2e/ui`, and screenshot utility package graphs
are repository tooling and are not inputs to Wander 0.1.0 binaries or release
archives. They remain independently visible to dependency automation and must
not be silently treated as production dependencies.

## Acceptance Criteria

- [x] `npm audit --audit-level=moderate` reports zero vulnerabilities for the
      UI build lockfile.
- [x] `npm audit --audit-level=moderate` reports zero vulnerabilities for the
      Playwright/TypeScript lockfile.
- [x] CI installs and audits both active lockfiles from clean state.
- [x] The shipped server serves browser assets from Go `embed.FS`; it does not
      invoke Node, npm, or a separate UI process at runtime.
- [x] Node is classified as build/test tooling only and is not included in the
      release archive runtime contract.

## Evidence

The local audits passed on 2026-11-03. The `test-core` security job repeats
both audits, while release workflows generate the static assets before the Go
binary is built. Archive verification separately proves the single-binary
package shape.
