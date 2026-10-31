# Feature: Server Agent Monitor

## Context

Nomad 1.6.5 exposed live agent logs from each server at
`/ui/servers/:server_name/monitor`. Wander's replacement server detail page
did not include that route, leaving an inherited operational workflow missing.

## Provenance and behavior

- The exact 1.6.5 MPL-2.0 templates and controller use the shared agent monitor
  with `server_id`, the server's region, and an `info` default level.
- The inherited public API endpoint is `/v1/agent/monitor`; it requires
  `agent:read` and already supports remote server targeting.
- The applicable 1.6.5 files are not in
  `docs/licenses/baseline-busl-exclusions.txt`.
- Client and server monitors intentionally share the same text-only, 50 KB
  bounded browser controller and request-scoped SSE proxy.

## Acceptance criteria

- [x] `/ui/servers/{name}/monitor` renders full-page and HTMX responses and is
      linked from server detail navigation.
- [x] `/ui/servers/{name}/monitor/stream` forwards the selected server name as
      `server_id`, its region, the selected level, and the browser ACL token.
- [x] The inherited error, warn, info, debug, and trace levels, URL persistence,
      pause/resume behavior, scrollback marker, and 50 KB cap match the client
      monitor.
- [x] Unknown servers, malformed paths, invalid levels, and denied agent reads
      return truthful responses without fabricated output.
- [x] Handler, route-contract, live browser, and disposable ACL-cluster tests
      cover successful and denied behavior.

## Verification

- Run `go test ./ui/...` and `make check`.
- Run the release Chromium suite against a disposable combined server/client.
- Run the live ACL audit to prove page and stream denial without `agent:read`.
- Run vulnerability scanning and Deslop before committing.
