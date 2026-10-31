# Structured Job Version Differences

## Context

Wander's Versions tab lists live job versions, but its Diff action currently
serializes two API job objects and counts changed JSON lines. Those line counts
are representation-dependent and do not describe the scheduler's semantic job
changes. Wander's inherited API already returns a structured `JobDiff` for each
adjacent pair when the versions endpoint is called with `diffs=true`.

This work restores the applicable 1.6.5 behavior without copying later Nomad
UI source. The implementation uses only Wander's existing API contract and the
rewritten UI's established Templ/HTMX patterns.

## Research Findings

- `api.Jobs.Versions` returns versions newest-first and an adjacent structured
  diff for every version except the oldest.
- The server computes contextual diffs, so records with type `None` provide
  context and must not inflate change totals.
- Existing plan rendering already understands `JobDiff`, but version history
  currently discards it and performs a second API read for a JSON line diff.
- The versions table and modal have no focused regression coverage today.

## Acceptance Criteria

- [x] The versions request asks the Wander API for structured differences.
- [x] Each adjacent difference is associated with the correct newer and older
      version, even when version numbers are not consecutive.
- [x] Version rows show deterministic total, added, edited, and deleted change
      counts derived only from semantic changes.
- [x] The Diff modal identifies each changed job field, object, task group, or
      task and displays available old/new values and annotations.
- [x] Context-only records do not appear or contribute to counts.
- [x] The oldest version truthfully reports that it has no prior version.
- [x] Missing versions, malformed paths, API failures, and inconsistent API
      version/diff responses fail truthfully rather than showing fabricated
      output.
- [x] Focused handler/template tests, UI tests, lint/vet, and Deslop pass.

## Technical Approach

1. Add a presentation projection that flattens `api.JobDiff` into semantic
   changes with stable scope paths and per-type counts.
2. Fetch versions with `diffs=true`, validate the response relationship, and
   attach each projected difference to its newer version row.
3. Reuse the same projection in the HTMX detail route instead of serializing
   job structs and running a text LCS.
4. Replace the line-number modal with an accessible structured change table and
   add change summaries to the version list.
5. Cover response mapping, contextual filtering, non-consecutive versions,
   rendering, and failure states with focused tests.

## Testing Strategy

- Unit-test nested diff flattening and counts.
- Handler-test the versions tab and detailed diff route against an HTTP API
  fixture, including request scope and `diffs=true`.
- Run the complete UI Go tests and UI lint/vet checks.
- Run Deslop over every changed Go/Templ-generated file before committing.

## Risks and Mitigations

- The API may return an inconsistent number of diffs. Treat that as unavailable
  data and surface an error instead of associating changes with the wrong row.
- Contextual diff nodes may look changed. Filter strictly on the API's `Type`
  value and recurse through edited containers only.
- Large changes may make the modal unwieldy. Use a bounded, scrollable table
  while retaining all semantic records in the response.

## Verification

- Focused structured-difference handler tests pass, including contextual
  filtering, nested changes, non-consecutive versions, scope propagation, and
  inconsistent API responses.
- `go test ./ui/...`, `go vet ./ui/...`, and `golangci-lint run ./ui/...` pass.
- The supported Chromium release suite passes against disposable federated
  Wander agents. Its existing-job workflow creates version 0, submits version
  1, verifies the structured Priority change and values, and closes the modal.
- The end-to-end TypeScript check passes without adding a runtime dependency.
