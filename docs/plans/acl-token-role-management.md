# Feature: ACL Policy, Token, and Role Management

## Context

Wander 0.1.0 must preserve all applicable Nomad 1.6.5 OSS functionality. The
rewritten UI can currently list, inspect, update, and delete ACL policies, but
it cannot create policies or manage ACL tokens and roles. This is a required
clean-room parity gap.

The implementation must use Wander's existing MPL-2.0 API client and UI
patterns. It must not consult or copy any file listed in
`docs/licenses/baseline-busl-exclusions.txt`.

## Research Findings

- `ui/handlers/policies.go` establishes the management-token checks,
  request-scoped API client, full-page/HTMX rendering, and mutation response
  conventions for ACL administration.
- `api/acl.go` already provides policy, token, and role list, create, read,
  update, and delete operations. No server API change is required.
- ACL token secrets are returned on creation and cannot be recovered later.
  The create response must therefore render the secret once without placing it
  in URLs, logs, or list views.
- Token and role mutations require a management token. Anonymous and client
  tokens must fail closed with a truthful forbidden response.
- No JavaScript package is required. Forms and mutations can use the existing
  HTMX and Alpine assets embedded in the binary.

## Acceptance Criteria

- [x] A management-token user can create, list, inspect, update, and delete ACL
      policies, tokens, and roles.
- [x] Role forms can bind one or more existing policies.
- [x] Token forms can bind policies and roles, select client/management type,
      set global scope, and set an optional creation TTL.
- [x] A newly created token's secret is displayed only in the immediate create
      response and is absent from subsequent reads and list pages.
- [x] Anonymous and non-management users cannot read or mutate these screens.
- [x] Missing objects return not found; invalid forms and API failures do not
      report false success.
- [x] Full-page and HTMX navigation work for every route without a separate UI
      process or new npm dependency.
- [x] Handler, route-contract, template, and live ACL mutation tests cover the
      successful and denied paths.

Validation evidence (2026-10-25): `go test ./ui/...` and
`WANDER_UI_E2E=1 go test ./ui/tests/e2e -run '^TestACLMutations$' -count=1 -v`
both pass. The live test uses an ACL-enabled disposable Wander cluster.

## Technical Approach

1. Extend the policy handler with a creation route and form.
2. Add request-scoped token and role handlers using `api.ACLTokens()` and
   `api.ACLRoles()`.
3. Add Templ list and form/detail views with shared ACL navigation links.
4. Register the routes and expose Policies, Tokens, and Roles in the sidebar.
5. Add focused handler and route tests, then extend the live ACL browser audit.
6. Generate Templ output, run focused tests, lint/check, Deslop, and the full
   relevant CI suite.

## Security and Correctness Notes

- Never log token secrets or include them in redirects, query parameters,
  cookies, HTML attributes, or list models.
- Treat policy and role names as data and URL-escape links.
- Validate object identifiers and form values before calling the API.
- Preserve request-scoped region/token configuration; do not mutate the shared
  API client.
- Keep destructive actions behind explicit confirmation and report API errors
  without claiming success.

## Testing Strategy

- Unit: parsing, model mapping, sorting/filtering, secret visibility, and
  management-token denial.
- Handler integration: fake ACL API for create/read/update/delete and failure
  responses, with both full-page and HTMX requests.
- Route contract: supported routes, methods, missing resources, and unknown
  subpaths.
- Live browser: create policies, roles, and tokens; update relationships;
  verify denial with a client token; then delete created resources.

## Risks and Mitigations

- Token secret leakage: use a create-only view field and regression assertions
  that list/detail responses never contain it.
- Accidental self-lockout: identify the active token and require explicit
  confirmation before destructive operations; do not expose its secret.
- Dangling relationships: load policies and roles for forms and let the server
  remain authoritative for validation and conflict errors.
- Licensing contamination: rely only on current MPL-2.0 code, public API
  contracts, and independently authored tests and templates.
