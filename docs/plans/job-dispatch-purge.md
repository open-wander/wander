# Job dispatch and purge restoration

## Goal

Restore two bounded Nomad 1.6-era job operations without using prohibited UI
source: dispatching a parameterized job and permanently purging a job. The
implementation uses the inherited API contract and Wander's existing
Go/Templ/HTMX patterns.

## Dispatch behavior

- Show `Dispatch` only for a parameterized parent job, not for a dispatched
  child.
- Render required and optional metadata fields from the live job definition.
- Render payload input only when payload is optional or required, and mark it
  required when the job requires it.
- Authorize the operation with `dispatch-job`, preserve namespace, region, and
  token scope, and let the cluster perform the final contract validation.
- On success, navigate to the new dispatched job. On failure, keep the dialog
  available and show a truthful error.
- Continue accepting the existing JSON endpoint contract while adding the
  form encoding needed by the server-rendered UI. Payload text is raw browser
  input; the inherited Go API client encodes `[]byte` for the wire request.

## Purge behavior

- Offer purge as a separate destructive operation from stop.
- Explain that purge immediately removes job history and cannot be undone.
- Require submit-job authorization, preserve request scope, and call job
  deregistration with `purge=true`.
- On success, return to the scoped jobs list. On failure, keep the user on the
  job and show an error.

## Verification

- Unit-test visibility and fields for parameterized job variants.
- Unit-test dispatch form and JSON parsing, ACL denial, raw payload handling,
  cluster request shape, and success redirect.
- Unit-test purge ACL denial, `purge=true`, cluster failure, and scoped success
  redirect.
- Exercise dispatch and purge against the disposable embedded-UI cluster.
- Run UI handler/template tests, the embedded Chromium suite, the license
  boundary check, and Deslop before commit.
