# Deployment Region and Canary Compatibility

## Context

Wander exposes deployment list and detail pages in addition to the job-scoped
deployment views inherited from the Nomad 1.6.5 behavior baseline. The detail
page reads the selected namespace and region, but its polling, job navigation,
and promote/fail actions currently discard that scope. The mutation handlers
also do not consistently resolve the deployment namespace before checking ACLs
and can expose upstream error text.

## Acceptance Criteria

- [x] Deployment detail polling preserves the selected namespace and region.
- [x] Job navigation from a deployment preserves the selected namespace and
      region.
- [x] Promote and fail actions resolve the deployment in the selected region,
      authorize its actual namespace, and mutate in that namespace and region.
- [x] Mutation errors use truthful 403, 404, and 502 statuses without exposing
      backend response text.
- [x] A running canary deployment renders its promotion control and multi-region
      metadata when supplied by the scheduler.
- [x] Handler, template, and embedded-browser tests cover the scoped contract.

## Technical Approach

1. Carry request region into the deployment detail view model and build every
   detail URL from the namespace/region pair.
2. Resolve deployment identity before either mutation, then use the resolved
   namespace for authorization and explicit scoped write options.
3. Centralize safe JSON mutation failures so promote and fail cannot drift.
4. Add focused API-contract tests and a remote-region canary browser fixture.

## Boundary

The Nomad 1.6.5 black-box baseline has job-scoped deployment behavior but no
global `/ui/deployments` route. Wander's global deployment pages are therefore
an additive UI surface. This work preserves inherited deployment semantics and
tests the additive surface without treating it as copied newer-UI behavior.
