# Scoped UI Navigation

## Context

Namespace and region are resolved per request, with URL parameters taking
priority over browser-scoped cookies. Most rewritten UI links were created
without those parameters, however, so a direct or shared scoped URL could lose
its scope on the next navigation. Selector-driven testing did not expose this
because the selector also stores a cookie.

## Acceptance Criteria

- [x] Direct namespace and region URLs retain both selections across links.
- [x] HTMX reads and mutations retain both selections.
- [x] JavaScript-backed fetch, log, and exec requests retain both selections.
- [x] Existing endpoint query parameters are preserved and explicit scope is
      never overwritten.
- [x] Static assets and external URLs are not modified.
- [x] Browser regression coverage starts without selection cookies.
- [x] The full embedded-binary UI smoke tests pass.

## Technical Approach

1. Render the resolved namespace and region as escaped data attributes on the
   application body.
2. Use a small dependency-free browser helper to add missing scope parameters
   to same-origin UI and API URLs. Apply it to rendered navigation and HTMX
   attributes, dynamically swapped content, and application `fetch` calls.
3. Use the same helper when constructing allocation log and exec streaming
   URLs.
4. Add a browser test that enters through a direct namespace URL in a fresh
   context and verifies subsequent navigation and actions remain explicit.

## Risks and Mitigations

- Selection endpoints intentionally change scope. Existing explicit query
  values are retained, so the target selection is not overwritten.
- Mutation endpoints may have non-scope query parameters. The helper edits via
  `URLSearchParams`, preserving those parameters.
- Content added after initial load may otherwise remain unscoped. HTMX load
  events and a mutation observer cover swapped and client-rendered content.
