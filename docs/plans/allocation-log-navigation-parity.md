# Allocation Log Navigation Parity

## Baseline evidence

The approved Nomad 1.6.5 baseline exposes three distinct allocation-log
behaviors:

- live streaming follows new output;
- **Head** performs a finite read from `origin=start&offset=0`;
- **Tail** performs a finite read from `origin=end&offset=50000`;
- word wrapping defaults to off and the user's choice is persisted.

This document records behavior only. The Wander implementation remains an
independent HTMX/Alpine implementation and does not reuse later Nomad UI code.

## Acceptance criteria

- [x] Live, Head, and Tail controls issue the inherited query shapes.
- [x] Finite Head and Tail reads stop cleanly without reconnecting or repeating
      output.
- [x] Live mode retains the existing reconnecting SSE client behavior.
- [x] Word wrap defaults off, can be toggled without changing log content, and
      persists locally.
- [x] Task and stdout/stderr/both changes preserve the selected view mode.
- [x] Invalid `origin`, `follow`, and `offset` values fail with HTTP 400 rather
      than silently changing the requested read.
- [x] Namespace, region, and ACL token scope reach allocation and log reads.
- [x] Pause/resume, clear, download, ANSI rendering, and the bounded viewport
      continue to work.
- [x] Handler, JavaScript, template, and live-browser checks pass. Repository
      security gates are run before commit.

## Follow-on verification

Allocation completion, task restart, forced stream failure, transport retry,
pause/resume, clear, and download behavior are covered by the lifecycle unit in
`allocation-log-lifecycle-parity.md`. Rotated-log behavior remains a separate
focused comparison because the browser/API resume contract has no stable
cross-file cursor.
