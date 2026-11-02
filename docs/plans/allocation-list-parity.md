# Allocation List Parity

## Context

Wander's allocation list can search, filter, sort, and paginate live API data,
but it does not yet preserve the full request scope or expose all fields
observed in the approved Nomad 1.6.5 black-box comparison. API failures also
collapse into the same empty state as a cluster with no allocations.

The approved behavior evidence records allocation-list fields for status,
client, task group, job version, scheduling, and log navigation. Wander also
needs the job kind in this global view so service, batch, system, and sysbatch
allocations remain distinguishable.

## Research Findings

- `ui/handlers/allocations.go` already performs one allocation-list read and
  owns filtering, sorting, and pagination.
- `ui/templates/partials/allocationspartials/list_content.templ` already uses
  the shared table and pagination components.
- The API `AllocationListStub` contains `JobType`, `JobVersion`,
  `DesiredStatus`, and `DesiredDescription`; no per-row API read is required.
- The jobs list provides the established compact filter form and page-size
  selector pattern.
- The license-safe black-box evidence is recorded in
  `docs/plans/nomad-1.6.5-ui-blackbox-evidence.md`; no excluded UI source is
  required for this work.

## Acceptance Criteria

- [x] Preserve namespace, region, and ACL token on the allocation-list read.
- [x] Render job kind, job version, client, client status, scheduling state,
      and a direct log action from live list-stub data.
- [x] Filter by client status, job, job kind, task group, client, and
      scheduling state without losing the available filter choices.
- [x] Support deterministic sorting and 10/25/50/100 row page-size choices.
- [x] Preserve namespace and region on allocation, job, client, and log links.
- [x] Distinguish a true empty list from an unavailable cluster response.
- [x] Cover service, batch, system, and sysbatch allocation rows plus composed
      filters, sorting, pagination, scoping, and failure states.

## Technical Approach

1. Project the existing allocation-list stub fields into the handler summary
   without adding per-allocation reads.
2. Fetch the scoped list once, derive complete filter options, then apply
   validated local filters, deterministic sorting, and pagination.
3. Replace the loose dropdown row with the established compact GET/HTMX form,
   add the inherited columns/actions, and keep the table horizontally
   scrollable on narrower screens.
4. Return the existing cluster-unavailable response when the API client or
   allocation-list read fails.
5. Add handler and template regression tests, then update the revival matrix.

## Testing Strategy

- Handler/API tests assert region, namespace, token, live field projection,
  composed filtering, pagination, sorting, and unavailable behavior.
- Handler rendering tests assert the four job kinds, observed columns, scoped
  links, filter controls, and page-size selection.
- The embedded Chromium release smoke will exercise the live allocation list
  after the focused tests pass.

## Risks and Mitigations

- Large clusters still require the API list response before local pagination;
  this matches the existing request model and avoids N+1 reads. A future API
  cursor optimization can be added without changing the browser contract.
- Scheduling and client status are different states. They remain separate
  columns and filters so the UI does not imply that one replaces the other.
- Sorting fields can contain equal values. A stable comparison with an ID
  tiebreaker prevents rows from changing order between requests.
