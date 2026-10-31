# Services and job clients

## Context

Wander exposes allocations and clients, but the 0.1.0 parity inventory still
lacks the read-only service registry and a job-scoped view of the clients on
which that job has allocations. These views must use live API data and retain
the request namespace and region.

This implementation is clean-room work based on Wander's MPL-licensed Go API
and existing Wander UI patterns. Prohibited legacy UI source is not an
implementation input.

## Acceptance criteria

- [x] `/ui/services` lists live service names and tags for the selected scope.
- [x] `/ui/services/{name}` shows the live registrations for that service and
      links to the associated job, allocation, and client.
- [x] A job Services tab shows registrations returned by the job services API.
- [x] A job Clients tab truthfully shows clients observed in that job's
      allocation history; it does not claim scheduler eligibility.
- [x] Full-page and HTMX responses preserve namespace and region selection.
- [x] Empty, not-found, unavailable, and ACL-denied states are covered without
      fabricated fallback data.
- [x] Handler, template, and live-browser regression tests pass.

## Technical approach

1. Add a request-scoped read-only services handler and templates using
   `Services().List` and `Services().Get`.
2. Add job tab data sourced from `Jobs().Services` and
   `Jobs().Allocations(all=true)`, deduplicating allocation nodes by node ID.
3. Reuse established links, tables, badges, breadcrumbs, and HTMX rendering.
4. Extend the disposable smoke job with a Nomad-native service registration
   and exercise the global and job-scoped views in Chromium.
5. Update the authoritative revival checklist, feature status, and route
   matrix only after the tests provide evidence.

## Risks and mitigations

- A job's allocation history is not the set of all scheduler-compatible
  clients. Label the view precisely and never infer eligibility.
- Service names are path values. Escape generated links and reject malformed
  nested paths rather than treating them as a service name.
- Service visibility is namespace-scoped. Use the isolated request client and
  explicit query options for every read.
