# Wander Compatibility Boundary

Wander is an independent continuation of the MPL-2.0 Nomad 1.6-era codebase.
Source code from later Nomad releases must not be copied, merged, or used as an
implementation reference. Security fixes are implemented independently from
public vulnerability information and maintained dependencies.

## Wander-owned names

New product behavior, browser state, configuration, release artifacts, and
user-facing branding use the Wander name. For example, the UI stores its ACL
token in the `wander_token` cookie. A new feature must not introduce a Nomad
name merely because the inherited implementation contains similar names.

## Retained compatibility names

Some Nomad names are part of the inherited on-disk or network contract and are
retained so existing 1.6-era clients, jobs, plugins, and state continue to work:

- the `/v1/...` HTTP API and `X-Nomad-Token` request header;
- existing API fields, RPC methods, job syntax, environment variables, and
  plugin protocol identifiers;
- existing Go package paths and internal type names where renaming would break
  compatibility or create a high-risk rewrite;
- the `nomad.version` client fingerprint attribute used by the inherited
  scheduler compatibility checks.

These identifiers are compatibility surface, not an indication that Wander
tracks later Nomad source. New Wander-specific APIs should use Wander names,
while compatibility aliases should be isolated and documented.

## Review rule

When a new or changed line introduces `Nomad` or `nomad`, reviewers should
classify it as either inherited compatibility surface or a naming defect. If it
is compatibility surface, a nearby comment or this document should make the
reason clear. Otherwise, use a Wander-owned name.
