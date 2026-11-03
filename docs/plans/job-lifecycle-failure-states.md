# Job Lifecycle Failure States

## Context

Wander has live start, stop, run, and periodic-force mutations, but the route
parity matrix still lacks complete browser failure evidence. The current
confirmation buttons close their dialogs before the response arrives. Server
toasts use a `type` property while the toast store styles only `variant`, and
some mutation paths include raw backend error text in user-visible headers.

## Research Findings

- All four controls use HTMX and return `HX-Trigger` `showToast` events.
- The global `htmx:afterRequest` listener processes those headers for both
  successful and failed responses.
- `ToastContainer` passes server event details to the toast store, whose
  `add` function currently ignores the established `type` property.
- Start and run share `registerExistingJob`; stop uses deregistration; periodic
  force has a separate write. All flow through `prepareJobAction` for local
  permission and client-availability checks.
- Upstream API failures can be classified with `apiResponseHasStatus` without
  exposing response bodies or internal addresses.
- Request clients are region-scoped, but explicit query/write options should
  retain region as part of the mutation contract and its regression tests.

## Acceptance Criteria

- [x] Error toasts use the error visual variant for the established server
      `type` payload as well as the documented `variant` payload.
- [x] Failed start, stop, run, and periodic-force confirmations remain open and
      display an accessible error notification.
- [x] Successful actions close their confirmation after the response.
- [x] Upstream forbidden, missing, and unavailable outcomes return truthful
      status codes and safe messages without leaking backend response text.
- [x] Namespace, region, and ACL token scope are retained for every read/write
      involved in the actions.
- [x] Handler, JavaScript, and embedded Chromium tests cover the contract.

## Technical Approach

1. Normalize toast input once in the Alpine store and retain compatibility
   with both `type` and `variant` event producers.
2. Close lifecycle modals from the CSP-safe native `htmx:afterRequest` event
   only when the response is successful.
3. Route upstream action failures through one status/message classifier and
   log detailed errors only on the server.
4. Add explicit region to lifecycle query and write options.
5. Add table-driven handler outcomes, a focused JavaScript regression test,
   and browser-intercepted failure workflows for all four visible controls.

## Testing Strategy

- Handler tests cover forbidden, missing, and unavailable upstream responses
  for start, stop, run, and periodic force, including safe toast contents.
- JavaScript tests prove `type: error` and `variant: error` normalize equally.
- Chromium intercepts each mutation response, verifies the modal remains open,
  and checks the accessible error toast and its error styling.
- Run templ generation, UI Go and JavaScript suites, lint, security scans,
  Deslop, and the full embedded Chromium release suite.

## Risks and Mitigations

- Closing only on success could leave a dialog open after navigation. These
  actions do not navigate; success is the sole close condition.
- Backend text can contain secrets or infrastructure details. Keep it in
  structured server logs and render only fixed user-facing messages.
- Browser route interception does not exercise the server classifier. Pair it
  with fake-API handler tests so both halves of the contract are covered.
