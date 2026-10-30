# Job Template Parity

## Goal

Restore the Nomad 1.6.5 job-template workflow without copying the baseline UI
implementation. Wander must expose truthful, namespace-scoped templates backed
by Wander variables rather than a browser-only example library.

## Behavioral contract

- `/ui/jobs/run/templates` lists variables below
  `nomad/job-templates/` in the active namespace.
- `/ui/jobs/run/templates/new` creates a variable at
  `nomad/job-templates/<name>` with the job source in its `template` item.
- `/ui/jobs/run/templates/manage` allows an existing template to be updated or
  deleted.
- Selecting a template opens `/ui/jobs/run` with its stored HCL source loaded
  into the editor.
- Malformed variables without a `template` item are identified as malformed
  and cannot be selected as job source.
- Variable read, list, write, and destroy ACL capabilities are enforced for the
  exact namespace and path involved.
- Namespace selection is retained across template and run-job routes.
- The hard-coded JavaScript template library is removed.

## Verification

- Handler tests cover list/read/create/update/delete API contracts, namespace,
  token forwarding, validation, and malformed variables.
- Template tests cover route links, initial editor content, and the absence of
  the former hard-coded library.
- Browser smoke tests exercise create, select, update, and delete against a
  live development agent.
- Go tests, JavaScript syntax/type checks, lint, security checks, and Deslop
  complete without new findings.

## Provenance boundary

The route names and observable behavior come from the approved Nomad 1.6.5
black-box capture. The storage contract is independently documented by
Wander's MPL-2.0 CLI implementation (`wander job init -template`). No baseline
UI source, assets, or implementation details are used.
