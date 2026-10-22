# Namespace and Region Isolation

## Context

The rewritten UI currently stores the selected namespace and region in shared
process state and mutates the shared API client. A selection made by one browser
can therefore change the data read or modified by another browser. This is a
release-blocking correctness and security issue for multi-user clusters.

The 1.6-era UI expressed region and namespace through URL query parameters.
Wander will retain that observable behavior and use an HTTP-only cookie only as
a browser-scoped fallback when navigation does not carry the query parameter.

## Acceptance Criteria

- [x] Namespace and region selection never mutate process-global request state.
- [x] URL `namespace` and `region` values override browser cookies for a request.
- [x] Selection endpoints persist browser-scoped HTTP-only cookies and refresh.
- [x] Every UI API call uses a request-scoped client carrying both selections.
- [x] Two concurrent browser sessions can select different namespaces without
      leaking jobs, actions, or displayed selection between sessions.
- [x] Unknown or inaccessible selections are rejected before they are stored,
      while namespace-restricted ACL tokens can select their allowed namespace.
- [x] Existing default-namespace behavior remains compatible.
- [x] Unit, race, and live UI tests pass.
- [x] The rebuilt release archive passes its packaged-binary smoke test.

## Technical Approach

1. Add selection middleware that resolves query parameter, cookie, then safe
   default and places the result in the request context.
2. Add API client cloning that copies configuration while reusing the existing
   HTTP client/transport.
3. Replace shared-client use in UI handlers with request-scoped clients.
4. Make layout props read the request context rather than shared current values.
5. Change selection endpoints to validate input and set browser-scoped cookies.
6. Add unit tests for precedence, isolation, and client immutability, followed by
   a disposable-cluster multi-namespace browser test.

## Risks and Mitigations

- A handler may accidentally keep using the shared client. A repository scan and
  regression test will reject remaining request-path uses.
- Long-lived log and exec connections may lose scope. They receive the cloned
  client at connection setup and retain that immutable scope.
- Cookie state may hide URL behavior. Query parameters always take precedence,
  and tests exercise independent cookie jars.

## Decision Record

Selections are request context, not application state. The shared API client is
an immutable transport/configuration template after handler construction. This
keeps concurrent requests isolated without creating a new TCP transport for
every request.
