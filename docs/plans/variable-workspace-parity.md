# Variable Workspace Parity

## Context

Wander provides live namespace-scoped variable CRUD, but the workspace remains
below the approved 0.1.0 behavior contract. The revival checklist requires
path-tree navigation, JSON/list editing, validation, duplicate detection,
conflict handling, related entities, copy/reveal controls, and unsaved-change
protection.

The current update handler rereads the latest variable immediately before an
update and submits that latest `ModifyIndex`. A stale browser therefore cannot
observe a compare-and-set conflict and may overwrite a newer value. Creation
uses unchecked upsert semantics, duplicate item keys collapse silently into a
map, and delete is not protected by the version shown to the user. These data
integrity issues must be fixed before adding alternate editor views.

## Research Findings

- The inherited API already exposes `CheckedCreate`, `CheckedUpdate`, and
  `CheckedDelete`, returning `api.ErrCASConflict` with the expected and current
  modify indexes.
- The server contract permits paths matching `[a-zA-Z0-9-_~/]{1,128}`, reserves
  most of the top-level `nomad` directory, requires at least one item, and
  limits combined key/value bytes to 64 KiB.
- Variable metadata list responses do not expose item values, so a path tree
  can be built without N+1 secret reads.
- The existing Alpine variable editor owns stable list state and can be
  extended with JSON synchronization, dirty tracking, reveal state, and copy
  controls without adding npm dependencies.
- `copyText()` already provides clipboard and fallback behavior used elsewhere
  in the UI.

## Acceptance Criteria

### Integrity and validation

- [x] Create uses checked-create semantics and reports an existing path as a
      `409 Conflict` without overwriting it.
- [x] Update and delete submit the modify index rendered to the browser and
      report stale writes as `409 Conflict` without exposing current values.
- [x] Validate path, namespace, non-empty items, blank and duplicate keys, and
      the inherited 64 KiB item limit before issuing a mutation.
- [x] Preserve namespace, region, and ACL token scope for every mutation.

### Workspace presentation

- [x] Build deterministic path-tree navigation from metadata only and retain
      namespace/region scope in every path link.
- [x] Provide synchronized list and JSON item editors with parse/type errors
      that never submit ambiguous data.
- [x] Show related job navigation only for exact `nomad/jobs/{job}` paths and
      descendants, without disclosing values in list/tree views.
- [x] Provide per-value reveal/copy and whole-variable JSON copy controls with
      accessible feedback.
- [x] Warn before browser or in-app navigation when create/detail edits are
      unsaved; reset and successful save clear the dirty state.

### Evidence

- [x] Cover mutation request contracts, validation, duplicates, conflicts,
      scope, path-tree ordering/boundaries, editor synchronization, copy/reveal,
      and unsaved-change behavior.
- [x] Exercise the complete workflow in the embedded Chromium release suite.

## Technical Approach

1. Add shared server-side path/item validation and structured form parsing.
2. Replace unchecked mutations with the API checked variants, using the
   browser-rendered modify index for update/delete and mapping CAS conflicts to
   HTTP 409 responses.
3. Project variable metadata into a deterministic path tree and add a
   URL-persistent list/tree selector.
4. Extend the Alpine editor with synchronized list/JSON models, validation,
   dirty tracking, reveal state, and existing `copyText` behavior.
5. Add scoped related-job links and update handler/template/browser coverage.
6. Update the revival checklist and parity matrix only after every acceptance
   criterion and release gate passes.

## Testing Strategy

- Handler tests use a fake variable API to assert checked query parameters,
  submitted indexes, status mapping, no mutation after invalid input, and
  namespace/region/token scope.
- Template/JavaScript tests cover deterministic tree projection, editor mode
  synchronization, visibility, clipboard fallback, and dirty-state lifecycle.
- Embedded Chromium extends the existing isolated-namespace CRUD workflow with
  JSON/list switching, conflicts, copy/reveal, tree navigation, and unsaved
  navigation protection.
- Gates include templ generation, complete UI Go tests, JavaScript tests,
  golangci-lint, gosec, govulncheck, Deslop, and Chromium.

## Risks and Mitigations

- Variable values are secrets. Never include the conflicting server value in
  an error, list/tree projection, log, or test failure.
- JSON object keys are unique by definition, so duplicate detection must occur
  in list mode before conversion to a map.
- Browser clipboard permissions vary. Reuse the existing clipboard fallback
  and expose deterministic success/error feedback.
- Path-tree nodes can represent prefixes that are not variables. Render them
  as navigation groups and only link metadata records returned by the API.
