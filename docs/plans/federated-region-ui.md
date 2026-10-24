# Federated region UI verification

## Goal

Prove that the embedded Wander UI switches between real federated regions and
that subsequent navigation continues to use the selected region.

## Acceptance criteria

- The release browser harness starts disposable `global` and `region-a`
  development agents on separate HTTP, RPC, and Serf ports.
- The agents are federated through the inherited server-join command and the
  primary agent reports both regions before browser tests begin.
- Each region contains a distinguishable running job.
- A browser can switch from `global` to `region-a`, see only the remote job,
  open that job through the primary HTTP endpoint, and retain `region-a` when
  navigating back to the jobs list.
- External-cluster test runs remain supported and skip the federated assertion
  unless the harness created the known two-region fixture.

## Verification

- Run the embedded Chromium release smoke suite.
- Run the UI Go tests and TypeScript syntax/test discovery checks.
- Run Deslop against every changed code file and address new findings before
  committing.
