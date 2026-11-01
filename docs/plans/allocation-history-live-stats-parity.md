# Allocation History and Live Statistics Parity

## Context

Allocation detail still omits reschedule and preemption relationships and labels
reserved task resources as zero-percent usage because it does not read the
inherited allocation statistics endpoint. This is the remaining allocation
detail item in the 0.1.0 revival backlog.

This work uses inherited API endpoints and Wander-owned templates only. It does
not use later Nomad UI source.

## Research Findings

- Allocation `Info` contains previous/next allocation relationships,
  reschedule events, preempted allocation IDs, and the preempting allocation.
- `Allocations().Stats` returns aggregate and per-task CPU/memory statistics
  from the client and can fail independently of the server-side allocation
  read.
- Statistics timestamps are Unix nanoseconds. CPU usage is an API percentage;
  memory usage is bytes and can be compared with the reserved task memory.
- Service registrations and statistics are only rendered by the Tasks view;
  fetching them for logs, exec, files, or raw adds unnecessary client-runtime
  dependencies.

## Acceptance Criteria

- [x] Allocation detail shows previous/next, reschedule, and preemption
      relationships with deterministic ordering and scoped links.
- [x] Aggregate and per-task live CPU/memory statistics are rendered when the
      client endpoint succeeds, including the sample timestamp.
- [x] Missing or failed statistics never appear as fabricated zero-percent
      usage; reservations remain clearly labelled and the allocation stays
      usable.
- [x] Live service/statistics reads preserve namespace, region, and ACL token
      and run only for the Tasks view.
- [x] Focused API/template tests and the live Chromium allocation scenario pass,
      followed by the relevant full checks.
- [x] Deslop reports no newly added duplication for this change. Three handler
      warnings were removed with a typed link sorter and focused helpers; the
      final changed-range scans report no warning/error clusters.

## Technical Approach

1. Add presentation models for allocation relationships, reschedule events,
   and aggregate/per-task live statistics.
2. Project history from allocation `Info`, sort events newest-first and
   preempted allocation IDs lexicographically, and build scoped links.
3. Add optional Tasks-view enrichment that reads services and statistics after
   the required allocation read; retain independent ready/error states.
4. Replace the fabricated zero-percent bars with reserved-resource labels when
   task statistics are absent and real values when present.
5. Render allocation history and live aggregate statistics with explicit empty
   and unavailable states.

## Testing Strategy

- Handler fixtures cover scope, ordering, history projection, aggregate/task
  statistics, independent service/statistics failures, and Tasks-only reads.
- Template assertions cover real values, reservations, timestamps, scoped
  links, and empty/error states.
- The disposable-cluster browser test verifies live smoke-allocation statistics
  without requiring a synthetic preemption event.
- Run UI tests, vet, lint, TypeScript checking, and Deslop.

## Risks and Mitigations

- Stats route to the allocation's client and may be unavailable for terminal or
  unreachable allocations. Treat this as a section-level state, not a page
  failure.
- CPU percentage may exceed 100 on multi-core workloads. Preserve the reported
  value in text and clamp only the visual progress width.
- Historical relationship targets may already be garbage-collected. Keep links
  truthful; the existing allocation not-found view handles stale targets.
