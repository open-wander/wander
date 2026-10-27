# CSI plugin detail compatibility plan

## Goal

Restore the Nomad 1.6.5 OSS CSI plugin detail flow for Wander 0.1.0 without
copying the legacy Ember implementation. The implementation must use Wander's
existing Go, Templ, HTMX, and browser-test stack and the inherited Nomad API.

## Baseline contract

Nomad 1.6.5 exposes these browser routes:

- `/ui/csi/plugins/{id}` for plugin identity, provider/version, controller and
  node health, and recent controller/node allocations;
- `/ui/csi/plugins/{id}/allocations` for the combined allocation list with
  type and health filters, sorting, and pagination.

The plugin API response already supplies controller/node `CSIInfo` maps and
allocation stubs. Wander must derive its view from those live values and must
not fabricate plugins, allocations, health, resource values, or timestamps.

## Implementation

- [x] Route plugin list rows to the plugin detail route while preserving the
      selected region.
- [x] Add full-page and HTMX handlers for plugin detail and allocation routes.
- [x] Return truthful 404 responses for missing plugins and preserve API error
      status where it can be classified safely.
- [x] Render provider, plugin version, controller requirement, controller/node
      health, and recent controller/node allocation tables.
- [x] Join CSI health records to the API allocation stubs by allocation ID and
      display known allocation identity, job, client, version, allocated CPU
      and memory, create/modify times, and CSI health. Unknown fields must use
      an explicit unavailable state rather than zero-looking fabricated data.
- [x] Add allocation type/health filters, deterministic sorting, and bounded
      pagination at `/allocations`.
- [x] Cover routing, API mapping, empty/degraded states, filters, pagination,
      HTMX rendering, and unsupported methods with focused tests.
- [x] Add a live-browser route check that is skipped truthfully when the
      disposable cluster has no CSI plugin.
- [x] Update the route matrix and revival checklist only after the verified
      behavior is complete.

Completed on 2026-10-27. The focused and full UI tests, Go vet, Playwright
typecheck, targeted gosec and govulncheck scans, license-boundary check, and
post-change Deslop scan pass. The release browser suite contains an opt-in
plugin-detail check and skips it explicitly when the disposable cluster has no
CSI driver fixture.

## Verification

- `templ fmt .` and `templ generate`
- focused handler and template tests
- `go test ./ui/...`
- `go vet ./ui/...`
- targeted `gosec` and `govulncheck`
- license-boundary check
- Deslop scan of every changed code file
- live browser release suite when a suitable CSI fixture is available

## Out of scope for this slice

- Creating or managing CSI plugins;
- adding a CSI driver fixture to the default smoke environment;
- live CPU/memory utilization polling when only allocated capacity is available;
- CSI volume registration/deregistration and claim mutations, which remain a
  separate checklist item.
