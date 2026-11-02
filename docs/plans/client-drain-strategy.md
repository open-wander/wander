# Client Drain Strategy Parity

## Context

Wander can start and cancel a client drain, but its current form exposes only a
free-form deadline and ignore-system-jobs. An empty deadline is described as
unlimited while the handler silently applies one hour. It also omits the
inherited immediate-force, explicit no-deadline, drain message, and
keep-ineligible cancellation choices.

The Nomad 1.6.5 command and API contract define these behaviors without
requiring excluded UI source. This unit restores those strategy choices through
the existing client detail controls.

## Acceptance Criteria

- [x] Apply the inherited one-hour default when no explicit strategy is chosen.
- [x] Support a validated positive custom duration, explicit no-deadline drain,
      and immediate force drain as mutually exclusive choices.
- [x] Preserve the ignore-system-jobs option and attach an optional operator
      message to drain metadata.
- [x] Allow drain cancellation to either mark the node eligible or retain its
      ineligible state, with an optional cancellation message.
- [x] Preserve ACL token and region scope and return truthful validation,
      permission, connection, and upstream failures.
- [x] Cover every strategy payload, invalid combinations, cancellation choices,
      ACL mutation behavior, and the browser controls.

## Technical Approach

1. Parse one explicit `deadline_mode` (`default`, `custom`, `none`, or `force`)
   and retain compatibility with existing posts that provide only `deadline`.
2. Map the modes to the inherited API durations: one hour, a positive parsed
   duration, zero, or negative one second.
3. Use `UpdateDrainOpts` so message/cancel-message metadata and cancellation
   eligibility are sent in the same scoped write.
4. Replace the ambiguous modal hint with explicit strategy controls and expose
   keep-ineligible and cancellation-message inputs.
5. Extend handler, real ACL-cluster, and Chromium coverage.

## Risks and Mitigations

- Zero and negative durations have special inherited meanings. Keep their
  mapping in one parser and assert the exact API payloads.
- Force drain is destructive to running allocations. Label it explicitly in
  the confirmation form; the existing confirmation modal remains required.
- Existing automation may post only `deadline`. Infer custom mode when that
  field is present so compatibility is retained.
