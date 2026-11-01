# Allocation Log Lifecycle Parity

## Context

The log viewer already supports inherited Live, Head, and Tail reads, but a
clean end to a live upstream stream is ambiguous. A terminal allocation must
stop reconnecting, while a running allocation whose task restarted must resume
following without discarding the output already shown.

## Baseline and current evidence

- Finite Head and Tail reads already emit `complete` and close intentionally.
- Wander's shared SSE client already reconnects transport failures with bounded
  backoff.
- The inherited allocation API exposes terminal client state and task-specific
  restart operations.
- Pause/resume, clear, download, ANSI stripping, and manual recovery controls
  exist but need direct lifecycle regression coverage.

## Acceptance criteria

- [x] A clean live-stream end checks the current allocation state.
- [x] Terminal allocations emit `complete` and do not reconnect.
- [x] Running allocations emit a task-restart signal and reconnect without
      clearing previously displayed output.
- [x] Upstream stream errors remain visible and manually recoverable.
- [x] Pause/resume preserves buffered lines; clear empties visible and buffered
      lines; download uses plain log text and the selected allocation/task/type.
- [x] Running-task, all-task, and individual-task restart requests preserve
      namespace, region, and ACL token scope.
- [x] Handler, JavaScript, template, and real-agent browser tests remain green.

## Technical approach

1. Classify a closed live stream from the allocation's current client status.
2. Emit explicit SSE lifecycle events rather than relying on browser EOF
   heuristics.
3. Add a small Alpine restart handler that reopens Live mode without clearing
   logs.
4. Add table-driven Go state tests and JavaScript behavior tests.

## Risks

- The allocation status lookup can itself fail. In that case the handler will
  close without an explicit lifecycle event so the existing SSE transport
  retry remains the fallback.
- Restarting a stream can duplicate a small tail window. The current API does
  not expose a stable browser resume cursor; retaining prior output is safer
  than silently losing logs and matches the preview-release scope.
