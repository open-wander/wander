# UI route contracts

## Context

The route parity matrix inventories Wander's UI capabilities, but most route
tests currently live with individual handlers. A regression can therefore
change a full-page route into an HTMX fragment, accept an unsupported method,
or allow an unknown path to fall through without one release gate noticing.

This work tests Wander's own routes and does not use legacy UI source.

## Acceptance criteria

- [x] Every top-level rendered page has full-page and HTMX response-shape
      coverage against an empty live-API contract.
- [x] Every registered route family rejects an unsupported HTTP method.
- [x] Unknown UI, nested resource, and action paths return truthful 404s.
- [x] Dynamic detail and mutation routes have representative success,
      not-found, and denied coverage in their owning handler tests.
- [x] The route inventory is easy to extend when a route is added.
- [x] The complete UI, Chromium, license-boundary, and Deslop gates pass.

## Technical approach

1. Add table-driven router tests for top-level full-page/HTMX reads using a
   deterministic empty API backend.
2. Add method and unknown-path tables at the real router boundary.
3. Reuse focused handler and live-browser tests as the evidence for dynamic
   detail, stream, WebSocket, and mutation routes rather than duplicating
   their domain fixtures.
4. Record the combined evidence in the route parity matrix and revival
   checklist only when all route families are accounted for.

## Evidence

- `ui/handlers/route_contract_test.go` is the router-boundary inventory for
  rendered pages, redirects, supported route families, unsupported methods,
  unknown paths, and representative missing resources.
- Focused handler suites remain the source of truth for successful and denied
  mutations, SSE/WebSocket behavior, and resource-specific rendering.
- `WANDER_UI_E2E=1 go test ./ui/tests/e2e -run '^TestE2E$'` exercises the
  embedded UI against disposable default and federated Wander clusters.
- `go test ./ui/...`, the TypeScript check, the license-boundary check, and a
  scoped Deslop rescan form the supporting release gate.

The contract run also corrected three defects it exposed: missing allocations
were rendered as empty successful pages, API 404s for services became 502s,
and the topology JSON feed was implemented but not registered with the router.

## Risks and mitigations

- A 200 response alone does not prove the right response shape. Assert the
  document shell for full pages and its absence from HTMX fragments.
- A fake backend can conceal live integration errors. Keep the Chromium and
  ACL disposable-cluster suites as required complementary gates.
- Stream, WebSocket, and destructive routes have different contracts. Keep
  those in their focused tests and reference them from the inventory.
