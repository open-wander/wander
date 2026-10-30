# Existing Job Edit and Review

## Context

Wander renders the retained HCL job submission and the live JSON definition,
and its definition tab exposes edit, plan, and run controls. The edit workflow
is incomplete: Ctrl+S calls an unregistered prototype endpoint, retained HCL2
variables are not loaded or submitted, successful updates leave the displayed
version stale, and an edited definition can name a different job than the
route being edited.

The approved Nomad 1.6.5 black-box evidence exposes Job Spec, Full Definition,
and Edit controls. This slice restores that operator-visible behavior using
Wander's existing plan and registration APIs without consulting excluded UI
source.

## Acceptance criteria

- [x] Job Spec shows the retained HCL submission and its HCL2 variables when
      available; Full Definition shows the live JSON job.
- [x] Users with `submit-job` can edit an available representation, review the
      real plan, cancel safely, and submit the reviewed definition.
- [x] Ctrl+S opens the same review flow and never calls a prototype endpoint.
- [x] Existing-job plan and submit reject a definition whose job ID differs
      from the route being edited.
- [x] Existing-job submit uses the displayed job modify index so a concurrent
      update cannot be overwritten silently.
- [x] ACL, namespace, and region scope remain request-local, and successful
      submission reloads the server-backed definition and version.
- [x] Missing retained HCL remains truthful: Full Definition stays usable and
      Wander does not fabricate source or variables.

## Outcome

The embedded-cluster Chromium release suite now edits a variable-backed HCL
job, reviews its live scheduler plan, submits it with enforced modify-index
registration, reloads the deep-linked Definition tab, and verifies the new
version, source, and retained variables without browser-console errors.
Full-page job tab URLs also hydrate their selected server-backed content, so a
refresh no longer silently resets the operator to Overview.

## Technical approach

1. Extend the definition view model with retained variables, edit permission,
   and job modify index.
2. Replace the dead client-side save method with the existing plan/review/run
   requests, including variables and optimistic-concurrency metadata.
3. Share backend validation for existing-job identity and registration
   options so plan and run do not acquire another duplicated branch.
4. Add focused handler, template, JavaScript-contract, and live browser
   coverage, then update the parity inventory.

## Testing strategy

- Handler tests inspect retained submission data, job-ID validation, namespace
  scope, and enforced modify-index registration.
- Template tests verify Job Spec, Full Definition, variables, permission, and
  concurrency data are rendered without embedding source in Alpine code.
- The embedded Chromium suite edits the live fixture, reviews its plan,
  submits it, and verifies the new server-backed version.
- Run UI Go tests, JavaScript syntax checks, focused security scans, the
  license-boundary check, and Deslop on every changed code file.

## Risks and mitigations

- **Concurrent overwrite:** require the job modify index returned with the
  definition and use the inherited enforced-registration API.
- **Source unavailable:** keep the live JSON representation available and
  clearly disable only the missing Job Spec representation.
- **Wrong job mutation:** compare the parsed definition ID with the existing
  route target before either planning or registering it.
- **Scope loss:** continue using the global scoped-fetch wrapper and verify
  namespace/region query propagation in browser coverage.
