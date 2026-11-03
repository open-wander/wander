# Allocation Exec Lifecycle

## Context

Wander provides an interactive allocation exec terminal over a same-origin
WebSocket. Live shell input/output and WSS transport are covered, but the UI
parity matrix still lacks resize, manual disconnect, task-exit, and denied-state
evidence. The current browser state machine also turns unexpected closure into
a normal disconnect, registers another terminal input callback after every
reconnect, and displays raw upstream exec errors.

## Research Findings

- Browser resize frames already use the established `{type, cols, rows}` JSON
  protocol and the Go proxy forwards them to the scheduler terminal-size
  channel.
- A browser `close` frame cancels the upstream exec context, but this boundary
  has no focused transport test.
- The server sends an `exit` control frame after draining command output.
- `onclose` currently overwrites both error and command-completion states.
- The exec tab is permission-gated before rendering, and the WebSocket handler
  independently enforces `alloc-exec` after resolving the allocation namespace.
- The existing embedded Chromium smoke proves a real shell command works, while
  the TLS suite proves the same protocol over WSS.

## Acceptance Criteria

- [x] Resize frames reach the scheduler with the correct rows and columns.
- [x] Manual browser disconnect cancels the upstream exec session and returns
      the terminal to an intentional disconnected state.
- [x] Command exit preserves its exit code and a distinct completed state,
      including non-zero exits, until the user reconnects.
- [x] Unexpected socket closure remains an error state with a reconnect action.
- [x] Reconnecting does not duplicate terminal input frames.
- [x] Denied WebSocket upgrades return a truthful forbidden response without
      establishing an exec session.
- [x] Upstream details remain in server logs and are not rendered in the terminal.
- [x] Go, JavaScript, template, and embedded Chromium tests cover the contract.

## Technical Approach

1. Make the Alpine exec component own one terminal input subscription and
   explicitly distinguish disconnected, connecting, connected, complete, and
   error states.
2. Preserve complete/error state when the WebSocket closes; use manual
   disconnect only for user-requested cleanup.
3. Keep server-side error detail in logs and send a fixed browser-safe message.
4. Add an in-process WebSocket protocol test for resize and close frames, plus
   a forbidden-handshake test scoped to the allocation namespace.
5. Add focused Node tests for reconnect, completion, unexpected closure, and
   resize behavior, then extend the live Chromium shell workflow.

## Testing Strategy

- Go transport tests exchange real Gorilla WebSocket frames with the handler's
  client-reader loop.
- Go handler tests verify namespace-aware `alloc-exec` denial before upgrade.
- Node tests drive the exported Alpine state machine with terminal and WebSocket
  doubles.
- Chromium runs a real command, resizes the terminal, observes successful task
  exit, reconnects, and manually disconnects.

## Risks and Mitigations

- WebSocket callbacks can race during close. Preserve terminal state before
  closing and make `onclose` conditional on whether closure was expected.
- Browser WebSocket APIs do not expose HTTP denial bodies. Render a stable
  connection error while proving the exact 403 contract in the Go handler test.
- Terminal input listeners can outlive sessions. Register exactly one listener
  for the component lifetime and dispose it with the terminal.
