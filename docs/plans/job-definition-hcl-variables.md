# Job-definition HCL variables

## Context

The run-job editor can plan and submit HCL jobs, but it has no variables input
and its requests discard the variables support already exposed by Wander's
jobs API. Registration also omits the original HCL submission, preventing the
server from retaining the source and variables used to create the job.

## Research findings

- The approved Nomad 1.6.5 black-box evidence shows variables as part of the
  run-job workflow.
- `api.JobsParseRequest.Variables` passes opaque HCL2 variable-file content to
  `/v1/jobs/parse`.
- `api.JobSubmission` records the original source, `hcl2` format, and variables
  on registration.
- Variables apply only to HCL2 definitions; accepting them for JSON would be
  misleading.

## Acceptance criteria

- [x] The run-job page provides an HCL variables input with clear HCL-only
      behavior.
- [x] Planning passes the variables text to `/v1/jobs/parse`.
- [x] Submission passes the variables text to parsing and records the original
      HCL2 source and variables in the registration request.
- [x] JSON requests containing variables are rejected instead of silently
      discarding them.
- [x] Handler and template tests cover the request contract and visible input.
- [x] The parity matrix and revival checklist record the restored slice without
      claiming that templates or full-definition editing are complete.

## Technical approach

1. Add `variables` to the plan and run request bodies.
2. Share definition parsing between plan and run so HCL2 and JSON validation
   cannot drift.
3. Use `ParseHCLOpts` for HCL and `RegisterOpts` with `JobSubmission` for run.
4. Add an HCL-only variables textarea to the Alpine editor and include its
   value in both requests.

## Testing strategy

- Exercise plan and run handlers against an HTTP test server and inspect the
  parse and registration payloads.
- Render the run template and assert the variables control and HCL-only state.
- Generate templates, run focused UI tests, run JavaScript type/loading checks,
  and scan changed files for duplication.

## Risks and mitigations

- Existing JSON callers may omit `format`; preserve the established default of
  HCL and reject only non-empty variables paired explicitly with JSON.
- Plan and run previously had different parsing branches and error wording;
  centralize only parsing while preserving endpoint-specific responses.
- Source retention changes registration metadata, not the parsed job itself;
  verify both independently in the backend contract test.

## Verification

Verified on 2026-10-29 with the full UI Go suite, JavaScript syntax and
TypeScript checks, and the embedded Chromium release suite. Chromium completed
with 15 passing scenarios and two environment-dependent CSI scenarios skipped.
Deslop found no new duplicate block in the changed regions; the shared parser
removes the former plan/run parsing duplication.
