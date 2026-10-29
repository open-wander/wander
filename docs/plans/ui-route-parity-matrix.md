# Wander 0.1.0 UI route and capability parity matrix

## Context

This is the authoritative working inventory for UI parity. It converts the
broad revival checklist into testable release units without using prohibited
UI source. Evidence must follow the
[license-safe parity method](license-safe-parity-method.md).

The browser route column records Wander's current contract. `Capture pending`
means that the exact 1.6.5 browser URL must still be recorded by an approved
black-box run; it does not make the capability optional.

## Status legend

- **Ready**: implemented with live data and release-level evidence.
- **Partial**: useful implementation exists, but inherited behavior or test
  states remain incomplete.
- **Required gap**: baseline behavior that must be independently restored for
  0.1.0 unless the project owner approves a documented deviation.
- **Decision needed**: applicability or exact baseline behavior still requires
  approved evidence.
- **Wander support**: implementation needed to operate the rewritten UI, not a
  claim about an inherited browser route.

## Matrix

| Area | Baseline route or behavior | Wander route(s) | API operations and mutation | ACL and scope | Evidence | Status / next action |
| --- | --- | --- | --- | --- | --- | --- |
| Shell | `/` redirects to `/ui/`; embedded UI can be disabled | `/`, `/ui/`, `/ui/static/*` | Read only | Cluster configuration | Unit tests, packaged binary smoke | **Ready** |
| Scope | Select region and namespace without leaking between browsers | All operational routes plus `/api/region/select`, `/api/namespace/select` | Selection cookie and explicit query scope | Validated against visible regions/namespaces | Concurrent browsers, restricted ACL token, federated cluster | **Ready** |
| Search | Global jobs, allocations, and nodes search | `/ui/search` | Fuzzy search read | Token, namespace, region | Handler and browser smoke | **Partial**: capture baseline result types and empty/error states |
| Keyboard shortcuts | Navigate major areas and page sections from the keyboard | Global help opened with `?`, major-area chords, section/tab movement, parent navigation, and search focus | Browser-only navigation state with versioned local preferences | All identities | Nomad 1.6.5 anonymous ACL black-box capture, Node tests, Go render tests, embedded-cluster Chromium release suite | **Ready**: applicable shortcuts, help, enable/disable, reset, and collision-safe rebinding restored |
| Jobs list | Browse, filter, sort, and inspect jobs | `/ui/jobs` | One metadata-enabled job-list read | Namespace and region | Handler/API projection tests plus live Chromium composed-filter and job-kind matrix smoke | **Ready**: inherited columns, filters, sorting, page-size selection, summaries, truthful Pack metadata, and service/batch/system/sysbatch/periodic/parameterized/dispatched states are covered |
| Job overview | Job summary and task-group information | `/ui/jobs/{id}`, `/overview` | Job, summary, scale-status reads | Namespace and region | Live browser smoke | **Partial**: allocations, services, volumes, lifecycle, metadata, and child-job parity |
| Definition | View HCL/JSON and edit or submit a definition | `/ui/jobs/{id}/definition`, `/ui/jobs/run` | Parse, plan, register | Submit-job capability; namespace/region | Live definition and ACL mutation tests | **Partial**: file upload, HCL variables, templates, and edit/review workflow |
| Job plan/run | Plan and submit a job | `/ui/jobs/{id}/plan`, `/ui/jobs/{id}/run` | POST plan/register | Submit-job capability; namespace/region | ACL mutation suite and live job-kind browser matrix | **Ready** for the approved baseline |
| Lifecycle | Start, stop, periodic force, and run existing jobs | `/api/jobs/{id}/{start,stop,run}`, `/periodic/force` | POST mutations | Submit-job capability; namespace/region | ACL mutation suite plus live service, batch, system, sysbatch, and periodic browser coverage | **Partial**: complete browser failure-state coverage for each action |
| Dispatch | Dispatch a parameterized job | Baseline `/ui/jobs/{id}@{namespace}/dispatch`; Wander transport at `/api/jobs/{id}/dispatch` | POST dispatch with live payload and metadata contract | Dispatch-job capability; namespace/region | Nomad 1.6.5 black-box navigation; handler/template tests, live Chromium dispatch, ACL success/denial | **Ready** |
| Purge | Stop and permanently purge a job | Baseline Purge Job control on `/ui/jobs/{id}@{namespace}`; Wander transport at `/api/jobs/{id}/purge` | Deregister with `purge=true` | Submit-job capability; namespace/region | Nomad 1.6.5 completed-batch black-box capture; handler tests, live Chromium child purge, ACL success/denial | **Ready** |
| Scaling | Scale a task group within policy bounds | `/ui/jobs/{id}/taskgroups/{group}`, `/scale` | Scale-status read and POST scale | Scale-job capability; namespace/region | Unit and browser mutation tests with live inclusive minimum/maximum constraints | **Ready** |
| Versions | Browse, diff, and revert job versions | `/ui/jobs/{id}/versions`, `/versions/{v1}/diff/{v2}`, `/versions/{v}/revert` | Version reads and POST revert | Read/submit-job; namespace/region | Handler and mutation tests | **Partial**: structured field-level differences and change annotations |
| Job allocations | Browse allocations belonging to a job/task group | Baseline `/ui/jobs/{id}@{namespace}/allocations` and `/ui/jobs/{id}@{namespace}/{group}`; Wander `/ui/jobs/{id}/allocations` and `/taskgroups/{group}` | Allocation reads | Namespace and region | Nomad 1.6.5 service/system black-box capture; Browser smoke | **Partial**: inherited task expansion, filtering, statistics, and task-group table |
| Job deployments | Browse job deployments | `/ui/jobs/{id}/deployments` | Deployment reads | Namespace and region | Browser smoke | **Partial**: sorting, priority, and state coverage |
| Job evaluations | Browse job evaluations | `/ui/jobs/{id}/evaluations` | Evaluation reads | Namespace and region | Browser smoke | **Partial**: sorting, priority, blocked/failed states |
| Job clients | Inspect clients running or previously allocated for a job | Baseline `/ui/jobs/{id}@{namespace}/clients`; Wander tab at `/ui/jobs/{id}/clients` | Job allocation-history read | Read-job; namespace and region | Nomad 1.6.5 system-job black-box capture; handler scope/deduplication tests and live Chromium fixture | **Ready** for allocation-observed clients; scheduler-eligibility semantics remain a focused comparison |
| Services | Browse service registrations and service details | Baseline `/ui/jobs/{id}@{namespace}/services` and `/services/{name}`; Wander routes at `/ui/services`, `/ui/services/{name}`, and `/ui/jobs/{id}/services` | Service registration and job-service reads | Read-job; namespace and region | Nomad 1.6.5 service-job black-box capture; full-page/HTMX state tests and live Chromium service fixture | **Ready** |
| Allocations list | Browse, filter, and sort allocations | `/ui/allocations` | Allocation list read | Namespace and region | Live browser smoke | **Partial**: verify inherited columns, filters, pagination, and job-kind states |
| Allocation detail | Tasks, raw allocation, restart, and stop | `/ui/allocations/{id}`, `/tasks`, `/raw`, `/restart`, `/stop` | Allocation read; POST restart/stop | Read-job and lifecycle capabilities; namespace/region | Browser and ACL mutation tests | **Partial**: ports, services, reschedule/preemption events, lifecycle, and metrics |
| Allocation logs | Follow/download stdout and stderr | `/ui/allocations/{id}/logs`, `/logs/stream` | Streaming and download reads | Read logs; namespace/region | Chromium plus HTTPS/SSE tests | **Partial**: wrapping, head/tail navigation, reconnect matrix, and rotated logs |
| Allocation exec | Interactive task shell | `/ui/allocations/{id}/exec`, `/exec/ws` | WebSocket exec | Alloc-exec; namespace/region; same-origin | Browser plus WSS test | **Partial**: resize, disconnect, task exit, and denied-state coverage |
| Allocation files | Browse, preview, and download allocation files | `/ui/allocations/{id}/files/{list,preview,download}` | Filesystem reads | Read filesystem; namespace/region | Deep-directory browser test | **Partial**: task-scoped paths, special files, binary/image policy, large/streamed files |
| Clients | List/detail, drain/cancel, eligibility | `/ui/clients`, `/ui/clients/{id}`, `/drain`, `/drain/cancel`, `/api/clients/{id}/eligibility` | Node reads and POST mutations | Node read/write | Browser and ACL mutation tests | **Partial**: node pools, versions, volumes, drivers, attributes, events, and pagination |
| Client monitor | Stream client agent logs with level selection and pause/resume | `/ui/clients/{id}/monitor`, `/ui/clients/{id}/monitor/stream` | Node identity read and agent monitor stream | Node read for identity; agent read for logs; region | Handler, route-contract, browser, and live ACL tests | **Ready** |
| Servers | Server list and detail with inherited columns, sorting, pagination, RPC address, and copyable tags | `/ui/servers`, `/ui/servers/{name}` | Agent members and configuration reads | Agent read | Handler and live browser tests | **Ready** |
| Server monitor | Stream server agent logs with level selection and pause/resume | `/ui/servers/{name}/monitor`, `/ui/servers/{name}/monitor/stream` | Agent member identity and agent monitor stream | Agent read; target server region | Handler, route-contract, browser, and live ACL tests | **Ready** |
| Topology | Datacenter/node resource topology | `/ui/topology`, `/ui/topology/data` | Node and allocation reads | Node read; region | Live reservation and federated tests | **Partial**: multiple datacenters/nodes, allocation overlays, edges, and selection |
| Variables | List and CRUD variable values | `/ui/variables`, `/new`, `/ui/variables/{path}` | GET/POST/PUT/DELETE | Variables capabilities; namespace/region | Isolated browser CRUD and ACL tests | **Partial**: path tree, JSON/list views, lock/conflict handling, validation, and copy controls |
| Namespaces | List and select namespaces | Nomad 1.6.5 OSS `/ui/namespaces` is Not Found; Wander `/ui/namespaces`, `/api/namespace/select` | Namespace reads; UI selection only | Namespace visibility | Nomad 1.6.5 management-token black-box capture; browser and restricted-token tests | **Ready** for selection; namespace CRUD is not applicable to the OSS baseline |
| ACL settings | Sign in/out and validate browser token | `/ui/settings/tokens` with `/ui/settings` retained as an alias; `/ui/settings/token`, `/api/v1/auth/*` | Token self/read and secure cookie mutation | Anonymous/client/management | Auth unit tests, route contracts, and live audit | **Ready** |
| ACL policies | Create, list, inspect, update, and delete policies | `/ui/access-control`, `/ui/access-control/policies`, `/new`, and detail; earlier `/ui/policies` paths remain aliases | GET/POST/DELETE policy | Management token | Nomad 1.6.5 management-token black-box capture; overview counts, route-contract, and live ACL mutation tests | **Ready** |
| ACL tokens and roles | Manage tokens, roles, bindings, and policy relationships | `/ui/access-control/{tokens,roles}` with `/new` and detail; earlier `/ui/{tokens,roles}` paths remain aliases | ACL token/role CRUD | Management token | Nomad 1.6.5 management-token black-box capture; overview counts, route-contract, secret-leak regression, and live ACL mutation tests | **Ready** |
| OIDC/JWT | Browser authentication flow where supported | `/ui/settings/tokens`: configured OIDC methods add an SSO action; configured JWT alone retains manual token sign-in | Auth-method list, OIDC auth URL, and OIDC complete-auth APIs | ACL configuration | Nomad 1.6.5 ACL-enabled black-box capture; handler/API-transport, route, secure-cookie, tamper, expiry, missing/replayed transaction, and provider-error tests | **Ready**; no separate interactive JWT action is applicable to the approved baseline |
| CSI volumes | List health and inspect constraints plus read/read-write allocation claims | `/ui/csi/volumes`, `/ui/csi/volumes/{id}` | CSI volume reads | Plugin/volume read; namespace/region | Inherited 1.6.5 UI API contract; handler and conditional browser tests | **Ready**: live controller/node health, complete constraints, separated claims, and truthful backend states |
| CSI plugins | List and inspect plugin health/allocations | `/ui/csi/plugins`, `/ui/csi/plugins/{id}`, `/ui/csi/plugins/{id}/allocations` | CSI plugin reads | Plugin read; region | Handler, route-contract, and conditional browser tests | **Ready**: live provider/version, controller/node health, allocation metadata/resources, filters, and bounded pagination |
| Evaluations | List and inspect evaluations | `/ui/evaluations`, `/ui/evaluations/{id}` | Evaluation reads | Namespace and region | API-contract state tests and browser smoke | **Partial**: pagination controls remain; empty, denied, unavailable, missing, failed, blocked, and completed states are covered |
| Deployments | List/detail, promote, and fail deployments | `/ui/deployments`, `/ui/deployments/{id}`, `/api/deployments/{id}/{promote,fail}` | Reads and POST mutations | Namespace and region | API-contract state tests plus browser and ACL mutation tests | **Partial**: multi-region and canary browser scenarios remain; empty, denied, unavailable, missing, running, successful, failed, cancelled, and blocked states are covered |
| Recommendations | Optimization/recommendation UI | Nomad 1.6.5 OSS `/ui/optimize` redirects to `/ui/jobs` and exposes no navigation entry | Enterprise optimization APIs | Edition-specific | Nomad 1.6.5 OSS black-box capture | **Not applicable** to the OSS-compatible release; no Wander implementation required |
| Error contract | Loading, empty, forbidden, not-found, unavailable, and disconnected behavior | All supported routes | Truthful HTTP/HTMX responses | Must not leak restricted resources | Router contract, focused handler state tests, error renderer tests, and Chromium smoke | **Ready** for route shape, methods, unknown paths, and representative missing resources; capability rows retain their specific state gaps |
| Responsive/accessibility | Phone through wide desktop; keyboard and screen-reader semantics | All supported routes | None | All identities | Basic Chromium accessibility smoke | **Partial**: responsive matrix and complex-control coverage remain |

## First implementation batches

1. Add route-contract tests generated from the Wander route inventory so every
   supported method has full-page/HTMX/not-found evidence. **Complete** with a
   table-driven router inventory and focused handler evidence.
2. Perform an approved black-box capture of the 1.6.5 browser route map and
   replace every `Capture pending` entry with evidence. **Complete** for
   directly reachable OSS navigation; resource-specific detail routes remain
   classified in their rows.
3. Restore services and job-client reads. **Complete** with live handler,
   Chromium, and exact baseline-path evidence.
4. Independently implement ACL token/role management without consulting the
   excluded BUSL source. **Complete** with clean-room handlers, templates, and
   disposable-cluster CRUD evidence.

## Acceptance criteria

- [x] Current Wander routes, operations, mutation status, ACL scope, and test
  evidence are inventoried.
- [x] Known baseline gaps are no longer silently described as enhancements.
- [ ] Every baseline browser path has approved black-box evidence.
- [ ] Every row is classified as ready, required, not applicable, or an
  explicitly approved deviation.
- [ ] Every ready row has automated success and failure-state evidence.
