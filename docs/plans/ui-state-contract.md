# UI State Contract

## Context

The 0.1.0 UI must not turn a failed API read into an empty table or fabricated
zero value. State coverage is route-specific: all data routes need successful,
empty, denied, missing, and unavailable behavior where those states apply;
loading indicators apply to asynchronous HTMX reads; disconnected and
reconnecting states apply only to streams, logs, monitors, and exec sessions.

## Acceptance Criteria

- [x] Every data-backed top-level route distinguishes an unavailable backend
      from a successful empty response for full-page and HTMX requests.
- [x] Server, namespace, topology, and variable list failures no longer render
      fabricated empty data.
- [x] Namespace job-count failures render `Unavailable` instead of zero while
      retaining successfully loaded namespace metadata.
- [x] Missing dynamic resources, permission denials, and unsupported routes are
      covered at the router boundary and in their owning handler suites.
- [x] Asynchronous controls expose loading indicators; stream and interactive
      routes retain explicit connecting, connected, disconnected, complete,
      and error states as applicable.

## Evidence

- `ui/handlers/route_contract_test.go` exercises empty route shapes, backend
  degradation, missing resources, unsupported methods, and unknown paths.
- Capability-specific handler tests cover partial failures and ACL outcomes.
- The embedded Chromium release smoke covers visible loading, failure,
  disconnect, reconnect, and recovery behavior for interactive routes.

## Boundary

Static forms and synchronous pages do not invent a disconnected state. Empty
and missing are distinct: collections may be successfully empty, while a
missing detail resource returns not found. Partial reads keep valid data only
when unavailable fields are labelled rather than represented as zero.
