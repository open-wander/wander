# Global Search Parity

## Context

Wander renders the inherited global-search control, but its input is named
`global-search` while the handler only reads `q`, so browser searches issue an
empty query. Non-empty handler requests also omit namespace, region, and ACL
token scope; map iteration makes groups nondeterministic; several links are
unescaped or lose scope; and backend failures replace the dropdown with a raw
HTTP error instead of a usable state.

The 0.1.0 route matrix requires the live fuzzy-search behavior used by the
Nomad 1.6.5 UI without adding fabricated results.

## Research Findings

- The inherited `PUT /v1/search/fuzzy` API returns matches grouped by context,
  with parent scope ordered from namespace toward the matched object.
- Wander has supported detail routes for jobs, allocations, clients,
  evaluations, deployments, services, variables, CSI volumes/plugins, task
  groups, and tasks.
- Request-scoped namespace, region, and ACL token helpers are already used by
  other UI handlers; search currently passes an empty `api.QueryOptions`.
- `SearchBar` currently uses one value for both input name and ID. The global
  field needs `name=q` for the endpoint while retaining `id=global-search` for
  keyboard and layout contracts.
- Search is an HTMX-only partial. Rendering explicit short-query, empty,
  unavailable, and result states keeps the dropdown truthful without exposing
  backend error text.

## Acceptance Criteria

- [x] The rendered global field submits `q` while retaining the
      `global-search` DOM ID and keyboard-focus behavior.
- [x] Fuzzy requests carry request-scoped namespace, region, and ACL token.
- [x] Result groups and items have deterministic ordering and omit matches that
      cannot map to a supported Wander route.
- [x] Result URLs escape identifiers and retain applicable namespace/region
      scope.
- [x] Empty, one-character, no-match, and backend-unavailable states are
      distinct, accessible, and contain no fabricated records or raw errors.
- [x] Handler, template/component, and embedded Chromium tests cover the live
      request and navigation contract.

## Technical Approach

1. Give `SearchBarProps` an optional independent ID and fix its clear action to
   retrigger the configured `keyup` HTMX event.
2. Normalize the query and build scoped `api.QueryOptions` in `SearchHandler`.
3. Project fuzzy matches through a deterministic context order, sort matches,
   discard unsupported links, and build escaped scoped URLs.
4. Extend the search partial with short-query and unavailable states plus
   dark-mode and accessibility semantics already used by the current UI.
5. Add fake-API request-contract tests, URL/order tests, render tests, and a
   live Chromium job search/navigation workflow.

## Testing Strategy

- Handler tests assert request body, namespace, region, token, deterministic
  projection, safe URLs, and unavailable-state rendering.
- Component tests assert independent search name/ID behavior.
- Chromium searches for the disposable live job and follows the scoped result.
- Run templ generation, UI Go tests, JavaScript tests, lint, security scans,
  Deslop, and the embedded Chromium release suite.

## Risks and Mitigations

- Search matches can contain path delimiters. Escape every route segment before
  rendering a URL.
- Nested matches require complete scope. Omit malformed matches rather than
  guessing a parent route.
- The API may add contexts. Unknown contexts remain omitted until Wander has a
  truthful destination for them.
