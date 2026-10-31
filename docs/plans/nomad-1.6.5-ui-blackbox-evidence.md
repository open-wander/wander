# Nomad 1.6.5 UI Black-box Evidence

## Checkpoint: 2026-10-26

This is the continuation record for the behavior-only UI comparison described
in [Nomad 1.6.5 UI Black-box Capture](nomad-1.6.5-ui-blackbox.md). No baseline
UI source, static bundle, page source, templates, scripts, styles, or tests
were inspected.

## Verified baseline

- Archive: official HashiCorp `nomad_1.6.5_darwin_arm64.zip`.
- Published and observed SHA-256:
  `63214ef4f08f44ba7b6e77328012a2ea843e27994ee27ca6fe8aa71be7f3fe3c`.
- Binary output: Nomad `v1.6.5`, build date `2023-12-13T20:00:11Z`, revision
  `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f`.
- Disposable combined server/client contained independently authored service,
  batch, system, periodic, and parameterized jobs.

## First-pass observations

The accessible browser crawl visited 100 URLs and observed 54 distinct public
API request method/path pairs before reaching its safety limit.

Confirmed browser paths and states include:

- `/ui/` redirects to `/ui/jobs`.
- `/ui/jobs` exposes Type, Node Pool, Status, Datacenter, Prefix, and page-size
  controls; sortable fields observed in links include name, status, type, node
  pool, and priority.
- `/ui/jobs/run` exposes Plan and jobspec download controls, with
  `/ui/jobs/run/templates` and `/ui/jobs/run/templates/new` for choosing and
  creating job templates.
- Job identity paths include the namespace suffix, for example
  `/ui/jobs/wander-ui-smoke@default`.
- Visible job sub-routes include `definition`, `versions`, `deployments`,
  `allocations`, `evaluations`, `services`, and `variables`. A system job also
  exposed `clients`.
- Service/system job pages expose Exec and Stop Job; completed batch jobs
  expose Purge Job and Start Job; periodic jobs expose Force Launch;
  parameterized jobs present a launch-oriented empty state.
- Job definition exposes Job Spec, Full Definition, and Edit. Allocation lists
  expose Status, Client, Task Group, Job Version, Scheduling, and View Logs.
- Allocation detail uses `/ui/allocations/{id}` and exposes Exec, Stop Alloc,
  Restart Alloc, Restart All Tasks, CPU, and Memory. Task links use
  `/ui/allocations/{id}/{task}`.
- `/ui/clients` exposes State, Node Pool, Class, Datacenter, Version, Volume,
  and page-size controls. Client detail exposes Drain, allocation filtering,
  attribute editing, and dynamic metadata creation.
- `/ui/servers` exposes sortable name, status, address, Serf port, datacenter,
  and version links; detail uses `/ui/servers/{name}.{region}` and identifies
  the leader.
- `/ui/topology` exposes Metrics and Allocation Status together with Node Pool,
  Datacenter, Class, State, and Version controls.
- `/ui/evaluations` exposes namespace, status, trigger, type, page-size, and
  refresh controls.
- `/ui/variables` showed a truthful empty state; `/ui/variables/new` exposes
  JSON/value editing and save controls.
- `/ui/csi` redirects to `/ui/csi/volumes`; both `/ui/csi/volumes` and
  `/ui/csi/plugins` showed truthful empty states.
- `/ui/settings/tokens` exposes token sign-in.
- `/ui/storage` rendered Not Found in this OSS binary.
- `/ui/optimize` redirected to `/ui/jobs`, evidence that the optimization UI
  is unavailable in this OSS configuration rather than an applicable route.

Observed reads covered jobs, job summaries/submissions/versions/deployments,
allocations and statistics, nodes and node pools, services, variables,
volumes, plugins, evaluations, regions, namespaces, agent members/self,
leader status, ACL self/auth-method checks, license detection, and fuzzy
search. The capture performed no mutating API requests.

## Important limitations

- The first crawler treated sort/filter query variants as distinct pages and
  reached its 100-page cap.
- Several late pages rendered the UI's generic Error state after many rapid
  navigations. These results must not be interpreted as missing capabilities;
  rerun them in a fresh browser context.
- The keyboard shortcut panel was not captured in the first two runs because
  the UI had not finished booting before the keystroke was sent. A later
  focused run resolved this.
- CSI plugin detail, multi-namespace,
  multi-region, error, and degraded states still require focused captures.

## Corrected path-deduplicated pass

The corrected crawler completed a fresh-agent pass and was then hardened to
close each browser context to terminate blocking API requests. It observed 73
distinct UI paths and 66 public API method/path pairs with no generic Error
pages. `/ui/storage` was the only observed Not Found page.

In addition to the routes above, visible navigation discovered:

- parameterized dispatch at `/ui/jobs/{id}@{namespace}/dispatch`;
- template management at `/ui/jobs/run/templates/manage`;
- client and server monitors at `/ui/clients/{id}/monitor` and
  `/ui/servers/{name}.{region}/monitor`;
- service detail at `/ui/jobs/{id}@{namespace}/services/{service}`;
- task-group detail at `/ui/jobs/{id}@{namespace}/{group}`;
- allocation-level files at `/ui/allocations/{id}/fs`;
- task detail, logs, and files at `/ui/allocations/{id}/{task}`,
  `/ui/allocations/{id}/{task}/logs`, and
  `/ui/allocations/{id}/{task}/fs`.

Several routes discovered late in the crawl need fresh-context validation
because their accessible content had not settled within the generic crawl
delay, even though their paths came from visible baseline navigation.

## Focused ACL and keyboard capture

An ACL-enabled fresh agent established these additional behaviors:

- Anonymous access keeps the requested browser route, renders `Not
  Authorized`, and provides Sign In/token links rather than fabricating empty
  data.
- The global `?` shortcut opens a `Keyboard Shortcuts` panel. Observed bindings
  include `g` navigation chords, Shift plus arrow navigation, `Shift + ?` help,
  and Escape dismissal.
- `/ui/namespaces` is Not Found even for a management token in the Nomad 1.6.5
  OSS binary; namespace CRUD is not an OSS UI requirement.
- Management-token navigation exposes `/ui/access-control` with Tokens, Roles,
  and Policies sub-routes, plus `/new` and resource-detail paths. Wander has
  the clean-room CRUD functionality but still needs compatible baseline paths
  and the access-control overview.
- A configured JWT auth method alone keeps the manual-token sign-in UI and
  adds no separate interactive JWT action.
- A configured OIDC auth method adds `Sign in with SSO` and a method-specific
  sign-in action on `/ui/settings/tokens`. Wander therefore has a required OIDC
  browser-flow gap.
- `/ui/allocations` is a valid baseline route, while the probed global
  `/ui/services` and `/ui/deployments` paths are Not Found; their baseline
  equivalents are job-scoped.

After the capture, the Nomad agent was stopped and the temporary archive,
checksum manifest, extracted binary, cluster state, fixtures, and raw JSON
captures were deleted. The repository retains only this
implementation-neutral evidence and the reusable capture tool.

Final validation passed JavaScript syntax checking, `git diff --check`, and
the repository license-boundary check. Deslop's fresh repository scan reported
no duplicate clusters involving the capture tool.

## Resume procedure

1. Download and verify the same official archive; never inspect its bundled UI
   files.
2. Start a fresh disposable 1.6.5 combined agent with the five representative
   job types.
3. Run `ui/tests/e2e/blackbox/capture-nomad-1.6.5.mjs` with
   `NOMAD_1_6_5_URL` and a temporary `NOMAD_1_6_5_CAPTURE` path.
4. Perform focused fresh-context captures for the remaining CSI plugin detail,
   dynamic resource-detail, multi-namespace, multi-region, error, and degraded
   states noted above.
5. Update `ui-route-parity-matrix.md` only with directly observed behavior and
   mark each evidence acceptance criterion as it is satisfied.
6. Run the license-boundary check and Deslop before committing this audit.
