# Feature: Client Agent Monitor

## Context

Wander's client detail page exposes node state, allocations, events, drivers,
and resource information, but it does not expose the inherited client-agent
log monitor. Nomad 1.6.5 provided that surface at
`/ui/clients/:client_id/monitor`; the page was equivalent to
`nomad monitor -node-id` and accepted a `level` query parameter.

The replacement must preserve that public behavior without adding fabricated
data or depending on the removed Ember application.

## Research findings

- The public UI contract documents `/ui/clients/:client_id/monitor` and the
  `trace`, `debug`, `info`, `warn`, and `error` levels.
- The public agent API streams framed output from `/v1/agent/monitor` and
  requires `agent:read`.
- The inherited MPL-2.0 API client already exposes `Agent.Monitor`; no protocol
  changes are needed.
- The exact 1.6.5 MPL-2.0 UI behavior starts at `info`, persists level changes
  in `?level=`, preserves the current tail when changing level, supports
  pause/resume, and limits retained output to 50 KB.
- None of the inspected monitor files are in
  `docs/licenses/baseline-busl-exclusions.txt`.
- Wander already has a request-scoped API client, SSE transport, stream status
  handling, and an allocation-log viewer whose security and lifecycle patterns
  can be reused.

## Acceptance criteria

- [x] `GET /ui/clients/{id}/monitor` renders a full page, and an HTMX request
      renders only the monitor content.
- [x] The client detail surface links to the monitor route and the monitor page
      links back to the client overview.
- [x] The monitor immediately streams the selected client's agent logs at
      `info` unless a supported `level` query parameter is supplied.
- [x] Changing level updates the URL, retains the existing tail with a visible
      transition marker, and reconnects using the new level.
- [x] Pause closes the upstream stream; resume reconnects. Output is capped at
      the most recent 50 KB and is rendered as text, never executable HTML.
- [x] The upstream request carries the browser ACL token and request-selected
      region, targets the client through `node_id`, and relies on the inherited
      `agent:read` authorization check.
- [x] Invalid levels and malformed monitor paths fail truthfully, and stream
      failures appear as explicit UI errors without synthetic log output.
- [x] Focused handler, route-contract, JavaScript/browser, and disposable live
      cluster coverage exercise the successful and denied paths.

## Technical approach

1. Add a dedicated client-monitor handler for the page and SSE stream routes.
2. Validate the route and log level before opening a stream; fetch the node for
   page identity and return 404 for an unknown client.
3. Proxy `api.Agent.Monitor` frames into same-origin SSE events using the
   request-scoped client, ACL token, region, and `node_id` parameter.
4. Add a Templ page/partial with Overview and Monitor navigation, level
   selection, connection state, pause/resume, and a text-only log surface.
5. Add a small Alpine monitor controller that owns stream lifecycle, history
   updates, auto-scroll, transition markers, and the 50 KB cap.
6. Update the parity matrix and revival checklist only after the tests prove
   the contract.

## Testing strategy

- Unit tests cover level validation, stream frame forwarding, errors, and
  request query/token/region propagation.
- Route-contract tests cover full-page/HTMX rendering, method rejection,
  unknown clients, and malformed paths.
- Browser coverage checks initial connection, level switching, pausing, safe
  text rendering, and ACL denial against a disposable ACL-enabled cluster.
- Run `go test ./ui/...`, `make check`, the focused live UI suite, vulnerability
  scanning, and Deslop before commit.

## Risks and mitigations

- Agent logs may contain hostile text. The browser inserts output with
  `textContent`/`x-text`; the SSE payload remains JSON.
- A browser disconnect could leave the upstream monitor running. Request
  context cancellation closes the stop channel and the API response body.
- `node:read` and `agent:read` are different privileges. Node lookup controls
  page identity; the monitor endpoint remains fail-closed under the inherited
  `agent:read` API check.
- Unbounded output would grow browser memory. Retain only the last 50 KB, which
  matches the inherited behavior.
