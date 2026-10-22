# UI Definition, Files, and Shell Polish

## Context

Manual testing of the embedded 0.1.0 UI found that the job-definition JSON
button only changed highlighting, allocation directories could not be expanded,
and the new footer occupied only the content column while duplicating the
sidebar version. The source tree also contains partial dark-mode styles without
a complete theme control.

## Acceptance Criteria

- [x] Job definitions switch between the submitted HCL and live job JSON.
- [x] Allocation directories expand and files can be previewed without an
      unshipped Alpine plugin.
- [x] The footer spans the viewport and contains the only shell version label.
- [x] The release smoke test covers all three regressions.
- [x] Dark mode is accurately tracked as incomplete rather than advertised as
      a supported 0.1.0 feature.

## Technical Approach

1. Supply both the submitted definition and the marshalled live job to the
   existing editor, then switch editor content as well as language.
2. Replace the unsupported `x-collapse` directive with the shipped Alpine and
   HTMX primitives and use an explicit one-shot directory-load event.
3. Make the shell a vertical layout containing a horizontal app row and a
   full-width footer; remove the version from the sidebar.
4. Add browser assertions for JSON content, directory expansion/file preview,
   footer width, and the single version label.
5. Record full dark-mode coverage as post-0.1 work because current styling is
   incomplete and enabling it would produce a mixed light/dark interface.

## Testing Strategy

- Focused Go handler/template tests.
- Chromium release smoke against a disposable live cluster.
- Embedded release archive and runtime verification.
- Deslop scan of the changed UI files.

## Risks and Mitigations

- Submitted HCL can be unavailable for older jobs: retain the JSON-only,
  read-only fallback.
- File paths can contain URL-significant characters: construct links with
  encoded query parameters on the server.
- Dark-mode utility classes can imply support where none exists: document the
  actual state and defer the user-facing toggle until every shell and primary
  view is covered.

## Lessons

- A disposable-cluster test can accidentally connect to an older local agent
  if the child process exits on a port conflict. The harness now fails before
  startup when the inherited development ports are occupied.
- A format selector must change the underlying source, not only an editor's
  syntax mode. Browser coverage now asserts meaningful content in both views.
