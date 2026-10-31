# Wander UI Feature Status

This inventory is based on Wander's current implementation. It does not use
later Nomad source as an implementation reference. Update it whenever a UI
route is added, removed, or materially changed. The authoritative release
scope and parity work list is the
[Wander 0.1.0 Revival Checklist](0.1.0-revival-checklist.md).

## Implemented for 0.1.0

- Jobs: live list and detail data, definition view, plan/run, stop/start,
  purge, parameterized-job dispatch, scaling endpoint, versions and revert,
  allocations, allocation-observed clients, Nomad-native services,
  deployments, and evaluations. Dispatch derives required and
  optional metadata and payload fields from the live job, uses the distinct
  `dispatch-job` capability, and can open slash-containing dispatched child
  IDs. The scale-confirmation dialog now displays live counts and can be
  dismissed by its close and cancel controls or Escape. Negative counts,
  successful mutation, ACL denial, and cluster rejection have regression
  coverage. The live Chromium smoke also verifies inclusive autoscaling
  minimum and maximum bounds and blocks out-of-range confirmation.
  Existing-job definitions expose retained Job Spec variables and the live Full
  Definition, support plan/review and modify-index-protected submission, reject
  cross-job identity changes, and preserve the selected tab on full-page
  reloads. Version history uses the inherited semantic job difference response,
  reports added/edited/deleted change counts, and shows structured field,
  object, task-group, and task changes with available old/new values and
  annotations.
- Allocations: live detail, files, logs, raw API data, restart/stop actions,
  and same-origin WebSocket exec.
- Task groups: live scaling controls and task resources plus deterministically
  ordered volume requests and explicit normal/prestart/poststart/poststop and
  sidecar lifecycle states. Exact-group allocation history, validated status
  filters and sorting, scheduler health counts, and deterministic scaling-event
  history preserve namespace, region, and ACL scope and expose partial failures.
- Clients: live list and detail data, drain, cancel drain, active allocation
  counts, and allocation reservations against schedulable node capacity.
- Services: live namespace-scoped list and registration detail routes, with
  links to their jobs, allocations, and clients. Job detail includes separate
  service and allocation-observed client tabs without implying scheduler
  eligibility.
- ACL settings: token validation, secure browser cookie handling, sign-in, and
  sign-out.
- Namespaces: live cluster reads and UI namespace selection. Namespace creation,
  editing, and deletion are not exposed in the 0.1 UI.
- Access control: the Nomad 1.6.5-compatible `/ui/access-control` overview and
  policy, token, and role routes provide live counts and complete clean-room
  CRUD/binding workflows. Earlier Wander top-level ACL paths remain aliases.
- Variables: live reads plus create, update, and delete actions; cluster
  failures do not fall back to fabricated records or secrets. A live browser
  test covers the complete create/read/update/delete workflow across isolated
  namespaces, and the create form uses only namespaces visible from the live
  cluster. Job detail also exposes metadata for variables linked beneath the
  inherited `nomad/jobs/{jobID}` path, with strict path boundaries and no
  variable-value disclosure in the list.
- Deployments, evaluations, servers, CSI volumes/plugins, and topology: live
  read paths backed by the inherited cluster API. Job deployment/evaluation
  histories retain namespace, region, and ACL scope; use deterministic
  scheduler-index ordering; expose evaluation priority and placement failures;
  and link to their detail routes without masking API failures as empty data.
  Topology shows active allocation reservations against schedulable node
  capacity and identifies the selected region without inventing heartbeat
  data.
- CSI volumes: live controller/node health, complete capability constraints,
  separate read and read/write allocation claim tables, and truthful denied,
  missing, and unavailable responses. CSI mutations remain inherited API/CLI
  operations and are not part of the Nomad 1.6.5 browser contract.

## Required before the 0.1.0 release

This section records completed UI infrastructure gates. It does not establish
that feature parity is complete; outstanding parity and live defects are
tracked in the revival checklist.

Namespace and region selection is now request/browser scoped. URL selection
takes precedence over HTTP-only fallback cookies, all operational UI handlers
use isolated API-client configuration, and disposable-cluster tests prove two
browsers and a namespace-restricted ACL token cannot leak jobs between
namespaces. Unknown or inaccessible selections are rejected before they are
persisted. Links, HTMX requests, application fetches, log streams, and exec
WebSockets now retain explicit URL scope even when a browser has no selection
cookie. The embedded-browser harness also federates disposable `global` and
`region-a` agents and verifies that switching regions forwards reads to the
remote job while preserving the selected region through navigation.

- [x] Verify a packaged release-format binary embeds this UI at `/ui/`,
  redirects `/` to `/ui/`, and requires no separate UI server or asset
  download. The Darwin ARM64 archive passed locally on 2026-10-18, including
  the inherited UI-disable configuration; CI now repeats the archive metadata
  check for every target and the runtime check for Linux AMD64.
- [x] Remove remaining unreachable prototype data generators and their legacy
  mock-only tests, or isolate fixtures strictly inside test packages.
- [x] Add API-backed handler and route tests that assert real success, empty,
  unavailable, and permission-denied states without assuming fabricated
  records exist. `TestJobsRouteAPIStates` covers the four rendering states
  against an HTTP API transport, while the live browser and ACL suites cover
  those states against disposable Wander agents.
- [x] Replace invented timestamps in deployment, CSI, and topology views with
  an explicit `Unknown` value where the inherited API exposes no timestamp.
- [x] Exercise the opt-in browser release suite (`WANDER_UI_E2E=1`) against a
  disposable Wander development cluster. On 2026-10-17 the harness built
  Wander, started a development cluster, registered a real `raw_exec` fixture,
  and passed the Chromium smoke checks for jobs, allocations, clients, servers,
  logs, and exec routes. Prototype-era specifications and visual baselines are
  explicitly retired from supported test commands.
- [x] Run the embedded UI under the inherited content security policy with
  browser error capture across jobs, allocations, clients, servers, topology,
  variables, CSI, deployments, evaluations, settings, logs, exec, and job
  submission routes.
- [x] Run the opt-in live link crawler (`WANDER_UI_LINK_CHECK=1`) against that
  cluster without modifying its state. The 2026-10-17 crawl covered 22 pages,
  39 links, and 198 asset references with no broken links or missing assets.
- [x] Verify all visible mutating controls against a disposable ACL-enabled
  cluster, including denial paths. The automated `TestACLMutations` release
  gate covers client eligibility/drain/cancel, allocation restart/stop,
  variable create/update/delete, policy update/delete, and deployment
  promote/fail, parameterized-job dispatch, and job purge. Job stop and token
  sign-in/sign-out were also exercised in the 2026-10-18 live audit. Namespace
  selection changes UI context only and is not a cluster mutation.
- [x] Verify logs and exec with a real running allocation over HTTPS/WSS. The
  automated `TestTLSTransports` gate reads live SSE output over HTTPS and runs
  a shell command to completion over WSS.
- [x] Complete keyboard-only and basic screen-reader navigation checks. The
  browser release suite checks landmarks, control names, form labels, unique
  IDs, and keyboard-only navigation.
- [x] Restore CSI plugin detail and allocation routes with live controller/node
  health, provider/version, allocation metadata and allocated resources,
  filtering, sorting, pagination, and truthful backend errors.

## Post-0.1 feature backlog

These are unimplemented capabilities. The
[UI route parity matrix](ui-route-parity-matrix.md) determines whether each is
an inherited 0.1.0 requirement or a post-0.1 enhancement; this list must not
be used to defer baseline behavior without explicit approval:

- Allocation reschedule/preemption history and live resource statistics.
- Richer job allocation filtering, sorting, task expansion, and live resource
  statistics.
- Variable validation, conflict detection, JSON/tree views, copy controls, and
  unsaved-change warnings.
- Richer topology filtering and allocation overlays.
- Additional job/client filters, columns, charts, eligibility analysis, and
  detail panels.
- Consistent copy controls and improved relative time display. The applicable
  Nomad 1.6.5 global keyboard shortcuts are restored and verified; their
  compatibility evidence is tracked in the revival checklist.
- Additional authentication methods beyond the Nomad 1.6.5 behavior contract.
  Configured OIDC browser sign-in is required before 0.1.0; no separate
  interactive JWT action was observed in the approved baseline capture.
- Complete dark-mode coverage and add a persisted system/light/dark theme
  selector. Some components already contain dark-mode utility classes, but the
  0.1.0 shell does not currently expose or claim a complete dark theme. The
  implementation scope and acceptance criteria are recorded in
  [Wander UI Dark Mode](dark-mode.md).

## Removed prototype behavior

The following behavior must not be restored as production fallback behavior:

- invented jobs, clients, namespaces, policies, variables, or secrets;
- hard-coded job specifications and task groups presented as cluster data;
- generated evaluation IDs returned as though a job submission succeeded;
- static sample allocation logs presented as live logs;
- fabricated resource histories or timestamps.

If a cluster/API capability is unavailable, the UI must show an empty,
unsupported, or error state. A future implementation should be backed by real
cluster data and covered by handler and browser tests.
