# Allocation Log Rotation Parity

## Context

The allocation log viewer covers Live, Head, and Tail behavior, transport
retries, task restarts, and terminal allocations. The remaining release-matrix
gap is log rotation: the client stores task output in indexed files and purges
old indices according to the task log policy. Wander delegates traversal of
those indices to the scheduler log API, but needs live evidence that its UI
continues to behave correctly after a purge.

## Acceptance Criteria

- [x] The disposable browser fixture produces multiple indexed stdout files
      and proves that the oldest file was purged.
- [x] Head completes from the oldest retained segment without resurrecting
      purged output.
- [x] Tail returns the newest bounded output after rotation.
- [x] Live continues following new output after the rotation boundary.
- [x] The existing fixture log, pause/resume, clear, download, and exec checks
      remain green.

## Technical Approach

1. Add a separate low-resource task to the existing smoke allocation with a
   two-file, one-megabyte rotation policy.
2. Emit deterministic bulk markers until at least three file indices have been
   created, then switch to slow live markers.
3. During fixture setup, inspect the allocation log directory through the
   public allocation-files API and wait until two retained files exist with an
   index greater than one.
4. Exercise the existing task selector and Head/Tail/Live controls in Chromium.

## Boundary

No browser-side cursor is introduced. File selection, rollover, and purge-race
handling remain the scheduler client's responsibility, matching the inherited
API contract. Wander only translates the resulting stream into scoped SSE.
