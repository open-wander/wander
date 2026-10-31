# CSI volume detail compatibility plan

## Context

Wander already exposes live CSI volume list and detail routes, but the views do
not yet implement the full Nomad 1.6.5 browser contract. The inherited UI API
documentation in `website/content/api-docs/ui.mdx` defines a read-only browser
surface:

- the list includes volume health, controller health, node health, provider,
  namespace, and attached allocation count;
- detail includes schedulability, provider, external ID, namespace, capability
  constraints, and separate read and read/write allocation claim tables.

CSI register, deregister, delete, and detach operations are public API and CLI
capabilities, but they are not part of the documented 1.6.5 browser surface.
They are therefore post-0.1 enhancements unless a focused, license-safe
black-box capture proves otherwise.

## Acceptance criteria

- [x] Preserve `/ui/csi/volumes` and `/ui/csi/volumes/{id}` with namespace and
      region selection.
- [x] Show controller and node plugin health in the volume list using live API
      counts, without converting unavailable backend data into an empty state.
- [x] Show every requested access/attachment capability on volume detail.
- [x] Show attached read claims separately from read/write claims and link each
      known allocation, job, and client.
- [x] Render unknown values explicitly rather than as plausible zero values.
- [x] Preserve truthful 403, 404, and 503 responses for list/detail failures.
- [x] Cover full-page and HTMX rendering, namespace propagation, claims,
      constraints, empty states, and backend errors with clean-room tests.
- [x] Update the route matrix and revival checklist after verification.

Completed on 2026-10-27. Focused and full UI tests, Go vet, Playwright
typecheck, targeted gosec and govulncheck scans, the license-boundary check,
and post-change Deslop review pass. The release browser suite checks volume
detail when a CSI fixture is present and skips explicitly when it is absent.

## Technical approach

1. Consolidate handler and template volume view models so API fields are mapped
   once.
2. Return classified errors from volume list/detail reads instead of silently
   returning empty data.
3. Map requested capabilities and allocation claims directly from
   `api.CSIVolume`, preserving whether each claim is read-only.
4. Replace the non-functional detail subroute tabs with a single baseline
   detail page containing constraints and the two claim tables.
5. Add handler/route tests and a conditional browser assertion for clusters
   that have a CSI volume fixture.

## Verification

- `templ fmt` and `templ generate`
- focused CSI volume handler tests
- `go test ./ui/...`
- `go vet ./ui/...`
- Playwright typecheck
- targeted `gosec` and `govulncheck`
- license-boundary check
- Deslop scan of changed code files

## Deferred enhancements

- browser forms for CSI registration or creation;
- browser deregister, delete, or detach actions;
- external-provider volume and snapshot management.

These remain supported through the inherited API and CLI. They may be designed
after 0.1.0 without being misclassified as missing 1.6.5 UI parity.
