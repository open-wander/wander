# Client Metadata Editing

## Context

Nomad 1.6.5 client detail exposes dynamic metadata creation and attribute
editing. Wander currently renders only the merged node metadata returned by the
node information endpoint, so operators cannot distinguish configured static
metadata from runtime overrides or manage those overrides.

This unit restores the inherited dynamic metadata contract through Wander's
existing client detail page. It uses the inherited node metadata API and does
not import excluded UI source.

## Acceptance Criteria

- [x] Read and display static and dynamic node metadata separately while
      retaining a truthful merged view when the metadata RPC is unavailable.
- [x] Create or update one dynamic metadata key/value pair from client detail.
- [x] Unset an existing dynamic metadata key, including a static-key override.
- [x] Require `node:write`, propagate ACL token and region, and preserve region
      and namespace in the redirect back to client detail.
- [x] Reject malformed requests and distinguish permission, validation, client
      connection, and upstream failures without claiming success.
- [x] Cover reads, writes, unsets, scope, ACL denial, validation, partial
      failure, and the live browser form.

## Technical Approach

1. Read `Nodes().Meta().Read` alongside node detail and project its static and
   dynamic maps without mutating the API response.
2. Add a scoped POST action accepting an explicit `set` or `unset` operation
   and a single metadata key/value pair.
3. Call `Nodes().Meta().Apply` with the request ACL token and region, relying on
   the inherited API for dotted-identifier validation.
4. Render static values read-only and dynamic overrides with add/update and
   unset controls only when the request permissions allow node writes.
5. Add handler and embedded-browser coverage, then update the parity records.

## Risks and Mitigations

- Metadata reads are client RPCs and can fail while node information remains
  available. Keep the rest of client detail visible and show an explicit
  metadata-unavailable state.
- A nil dynamic value masks a static key. Label that state as an override so an
  operator does not mistake it for deletion of agent configuration.
- Metadata changes are eventually reflected in the node information endpoint.
  Redirect after the authoritative metadata API response rather than claiming
  the node list has already converged.
