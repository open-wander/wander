# Client Statistics Honesty

## Context

Client detail reads the inherited `/v1/client/stats` endpoint and projects its
current CPU, memory, and disk sample. Failures are silently converted to a nil
value, while the page always renders empty charts labelled “Last 24 hours” even
though the API supplies no historical samples. That can make an unreachable
client look like a healthy zero-load client and implies data Wander does not
have.

This unit verifies the live sample projection and makes the absence of current
or historical statistics explicit. It does not introduce a metrics store or
fabricate history.

## Acceptance Criteria

- [x] Project non-zero CPU, memory, disk, and uptime values from the live host
      statistics response with deterministic core and disk ordering.
- [x] Preserve ACL token and region on the client statistics read.
- [x] Distinguish denied, unavailable, and empty current samples from genuine
      zero utilization without hiding the rest of client detail.
- [x] Remove the empty “Last 24 hours” charts and state that the inherited API
      provides only a current sample.
- [x] Cover live values, empty values, read failures, scope, and browser output.

## Technical Approach

1. Return the statistics read error separately from the projected sample.
2. Add availability/error state to client detail and render it alongside the
   current-sample section.
3. Sort CPU cores and disks before rendering, and retain zero values exactly
   when the API actually returns them.
4. Remove unused historical data structures and chart serialization from the
   client detail template.
5. Extend handler and embedded Chromium coverage, then update the parity
   records.

## Risks and Mitigations

- Host statistics are a client RPC and can fail independently of node detail.
  Keep all other detail sections usable and label the partial failure.
- An empty but successful response differs from a failed response. Render a
  dedicated no-current-samples state rather than reusing an error message.
- Historical charts require an external time-series source. Record that as a
  post-baseline enhancement instead of displaying empty chart furniture.
