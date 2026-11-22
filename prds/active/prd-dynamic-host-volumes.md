# PRD: Clean-Room Dynamic Host Volumes

## 1. Introduction and Overview

Wander inherits static host volumes and CSI volumes from its Nomad 1.6.5
baseline, but it does not support dynamic host volumes. Operators migrating
from newer Nomad clusters need to recreate volume records in Wander's Raft
state while continuing to mount existing directories on their client nodes.
They also need a complete, compatible management surface rather than a
deployment-specific shortcut.

This feature adds dynamic host volumes across the Wander server, Raft state,
client, scheduler, plugin lifecycle, HTTP API, Go API client, CLI, ACL system,
job specification, and web UI. It includes both creation through a storage
plugin and explicit registration of an existing host path. Registration must
adopt the path into Wander's state without moving, copying, formatting, or
deleting its contents.

The feature must be independently implemented. Publicly documented
post-1.6.5 interfaces and independently observed black-box behavior may be
used only as clean-room compatibility evidence. Post-1.6.5 Nomad source code,
tests, patches, generated assets, commits, and implementation details are
prohibited. Private infrastructure artifacts may be used locally to derive
acceptance criteria, but must never be committed, published, included in test
output, or sent to an external service. All committed examples, fixtures,
screenshots, and tests must use synthetic data.

### Owner decisions

- Target complete public dynamic-host-volume interface and behavior
  compatibility, not only the immediate migration subset.
- Include the server API, Raft state, scheduler, client and plugin lifecycle,
  Go API client, CLI, ACLs, job specification, and web UI.
- Adopt existing directories through an explicit registration operation.
- Match publicly documented destructive-operation behavior, including its
  safety checks, confirmations, ACL requirements, and in-use restrictions.

Decisions of 2026-11-22. Where the public contract leaves a case open or a
matching behaviour would be unsafe, Wander decides the behaviour on its own
merits and records it as Wander policy in the provenance record.

- Preemption: an allocation that holds the writer slot of a volume is never
  preempted to make room for another writer of that volume. The volume counts
  as held until the holder has stopped; the other job waits.
- Disconnected client: the claim of a sticky group stays with its volume
  while the client is disconnected. A replacement waits for that volume and
  is not placed on another volume of the same name.
- Agent restart: a task that is already running with its mount keeps running
  when its volume is not ready at the end of the first restore. Only a task
  that still has to start fails on a missing volume.
- Node read: the node read API lists a dynamic host volume only when the
  token may read host volumes in the namespace of that volume. Static host
  volumes are listed as before.
- Mixed versions: the Raft commands for volumes and claims are refused until
  every server meets the minimum version that understands them.

### Key terms

- **Create:** Ask a dynamic host volume plugin to provision storage, then
  persist and advertise the resulting volume.
- **Register:** Create a Wander Raft record for storage that already exists at
  a specified path on a specified client node. Registration does not provision
  or mutate the underlying storage.
- **Delete:** Ask the managing plugin to delete the underlying storage, then
  remove its Wander registration, subject to compatibility safety rules.
- **Deregister:** Remove a volume from Wander state without deleting its
  underlying storage, where the public compatibility contract supports this
  distinction.
- **Claim:** A durable association between a sticky task group and a specific
  dynamic host volume.
- **Plugin:** An executable discovered by a Wander client that implements the
  publicly documented dynamic-host-volume operation contract.

## 2. Goals

- Implement the complete publicly documented dynamic-host-volume contract
  across all Wander product surfaces.
- Allow an operator to register an existing host path and remount that same
  path in a Wander allocation without copying its contents.
- Persist volumes and sticky claims in Raft, snapshots, and state restore.
- Discover, fingerprint, execute, and monitor dynamic host volume plugins on
  clients, including a built-in directory-creation implementation.
- Place allocations only on nodes that expose a compatible requested volume.
- Enforce attachment mode, access mode, read-only state, sticky claims, and
  conflicting writer restrictions.
- Provide namespace-scoped ACL capabilities for creating, registering,
  reading, writing, deleting, and mounting host volumes.
- Provide compatible API, Go client, CLI, and UI workflows for volume and
  claim lifecycle management.
- Preserve existing static host volume and CSI behavior.
- Make destructive operations explicit, authorized, observable, and safe
  during concurrent allocation or node activity.
- Supply synthetic unit, integration, end-to-end, upgrade, and failure tests
  that can be run without any private infrastructure.
- Produce a migration runbook for rebuilding volume records in Wander while
  retaining existing on-disk data.

## 3. User Stories

### US-001: Represent and validate a dynamic host volume

**Description:** As an API or CLI user, I want a stable volume model so that I
can describe provisioned or pre-existing host storage unambiguously.

**Acceptance Criteria:**

- [ ] A synthetic host volume can round-trip through JSON, MessagePack, Raft
  state, and snapshots without losing its ID, name, namespace, node ID, node
  pool, host path, plugin ID, capacities, capabilities, constraints,
  parameters, state, timestamps, or Raft indexes.
- [ ] A create request without a name, type `host`, plugin ID, or at least one
  supported capability is rejected with a field-specific error.
- [ ] A register request without a name, type `host`, node ID, host path, or at
  least one supported capability is rejected with a field-specific error.
- [ ] A request with a minimum capacity greater than its maximum capacity is
  rejected before any Raft or plugin operation occurs.
- [ ] A request containing an unsupported attachment or access mode is
  rejected and identifies the unsupported value.
- [ ] Existing static host volume and CSI validation tests continue to pass.

### US-002: Persist volume lifecycle state in Raft

**Description:** As a Wander server operator, I want dynamic volumes stored in
Raft so that all servers agree on their lifecycle and recover after restart.

**Acceptance Criteria:**

- [ ] Creating or registering a synthetic volume writes one namespace-scoped
  record with monotonically increasing create and modify indexes.
- [ ] Reading by full ID returns the same record after leader change, process
  restart, snapshot save, and snapshot restore.
- [ ] List queries support namespace, ID prefix, node ID prefix, plugin ID
  prefix, pagination, blocking index, and deterministic ordering as specified
  by the public API contract.
- [ ] Updating an allowed field changes the modify index but preserves the
  create index and immutable fields.
- [ ] Updating an immutable name, type, or node assignment is rejected without
  changing state.
- [ ] Older Wander snapshots that do not contain dynamic volumes still restore
  successfully.

### US-003: Register an existing directory in place

**Description:** As a migrating operator, I want to register an existing
directory so that Wander jobs mount the same data under a newly created Wander
volume record.

**Acceptance Criteria:**

- [ ] Registering synthetic path `/srv/wander-volumes/data-a` for synthetic
  `node-a` creates a new Wander volume record without invoking a plugin create
  or delete operation.
- [ ] Registration fails when the node does not exist, is ineligible, or does
  not report the path as usable under the compatibility contract.
- [ ] Successful registration does not create, rename, copy, truncate,
  chown, chmod, format, or remove the target directory or its test marker.
- [ ] A task using the registered volume reads a pre-existing synthetic marker
  file through its configured task destination.
- [ ] Re-registering with the returned volume ID performs only a permitted
  update and does not create a second record.
- [ ] Registering the same volume name twice on one node is rejected, while
  the documented namespace and cross-node behavior is preserved.

### US-004: Configure plugin and managed-volume directories

**Description:** As a client operator, I want explicit plugin and managed
volume directories so that dynamic volume behavior is predictable and
confined.

**Acceptance Criteria:**

- [ ] The client configuration accepts the publicly documented plugin
  directory and managed host-volume directory settings.
- [ ] Relative paths, empty explicitly required paths, and paths that violate
  platform rules produce actionable configuration errors.
- [ ] Default paths are derived deterministically from the client data
  directory when the settings are omitted.
- [ ] Reloading supported configuration rescans plugins without dropping
  registered volume records or disrupting running allocations.
- [ ] Existing static `host_volume` configuration remains accepted and cannot
  silently shadow a dynamic volume of the same name on the same node.

### US-005: Discover and fingerprint plugins

**Description:** As a client operator, I want Wander to discover valid plugin
executables so that servers can place volume operations on capable nodes.

**Acceptance Criteria:**

- [ ] Only executable regular files in the configured plugin directory are
  candidates for discovery.
- [ ] A synthetic plugin that returns a valid fingerprint is advertised by ID
  and version in the node's client update.
- [ ] A non-executable file, malformed fingerprint, timeout, oversized output,
  or non-zero exit is excluded and produces a bounded diagnostic without
  crashing the client.
- [ ] Removing or invalidating a plugin changes its advertised health while
  preserving the state of already registered volumes.
- [ ] Plugin discovery after client restart produces the same deterministic
  plugin inventory.

### US-006: Execute the plugin lifecycle safely

**Description:** As a storage operator, I want create and delete operations
passed to a plugin using the public contract so that custom storage backends
can be integrated independently.

**Acceptance Criteria:**

- [ ] A synthetic plugin receives the documented operation argument and only
  the documented, allowlisted environment variables for a create request.
- [ ] A successful create response is validated before its host path and
  capacity are committed to Raft.
- [ ] Plugin timeout, cancellation, malformed output, path escape, or non-zero
  exit leaves the volume in the documented failure state and never advertises
  it as ready.
- [ ] At most one mutating plugin operation for a given volume ID executes at
  a time, including across retries and leader changes.
- [ ] Retrying a request with the same volume ID is idempotent and does not
  create a second underlying synthetic directory.
- [ ] Plugin standard output, standard error, and parameters are bounded and
  redact values classified as sensitive by Wander's logging rules.

### US-007: Provide a safe built-in directory plugin

**Description:** As an operator with local directory storage, I want a built-in
plugin so that I can create dynamic host volumes without another executable.

**Acceptance Criteria:**

- [ ] Creating a volume with the built-in directory plugin creates exactly one
  directory beneath the configured managed-volume root.
- [ ] The resulting path remains within the managed root after cleaning,
  symlink evaluation, and filesystem race checks.
- [ ] Synthetic mode, UID, and GID parameters are validated and applied on
  supported platforms; invalid or unavailable identities fail without
  advertising a ready volume.
- [ ] Capacity requests that the directory plugin cannot enforce are handled
  according to the public compatibility contract and reported consistently.
- [ ] Deleting a managed synthetic volume removes only that volume's confined
  directory and cannot target the managed root, a parent, a sibling, or a
  registered external path.
- [ ] Registered external directories are never deleted by the built-in
  plugin merely because their volume record names that plugin.

### US-008: Restore client volume availability

**Description:** As an operator, I want volumes restored after restart so that
stateful jobs resume without manual re-registration.

**Acceptance Criteria:**

- [ ] After a client restart, a registered external path that still exists is
  reported ready with the same volume ID and path.
- [ ] After a client restart, a plugin-created volume is reconciled using its
  persisted record without creating a duplicate directory.
- [ ] A missing path, unhealthy plugin, or failed restore marks the volume
  unavailable and prevents new placement while retaining diagnostic state.
- [ ] When the missing dependency returns, reconciliation moves the volume
  back to ready without changing its ID.
- [ ] A node fingerprint update never exposes a host path from a different
  node or namespace.

### US-009: Schedule allocations on compatible volumes

**Description:** As a job author, I want allocations placed only where their
requested host volumes are ready and compatible.

**Acceptance Criteria:**

- [ ] A group requesting synthetic volume name `data-a` is eligible only on a
  node advertising a ready compatible volume of that name.
- [ ] Node pool and constraint filters are applied before a create operation
  chooses a node and produce deterministic eligibility diagnostics.
- [ ] Requested attachment mode, access mode, read-only state, and mount
  propagation are checked during plan and apply.
- [ ] A single-node-single-writer volume rejects a conflicting second writer
  while allowing combinations explicitly permitted by its capabilities.
- [ ] Concurrent evaluations cannot both acquire an exclusive writer slot for
  the same volume.
- [ ] A stopped or terminal allocation releases its non-sticky attachment at
  the documented lifecycle point.
- [ ] Scheduler metrics and evaluation explanations identify volume
  incompatibility without exposing host paths to unauthorized users.

### US-010: Support sticky task-group claims

**Description:** As a stateful job author, I want a task group to remain bound
to one dynamic volume so that replacements use the same data.

**Acceptance Criteria:**

- [ ] A job volume with `sticky = true` creates one claim associating its
  namespace, job, task group, volume name, and selected volume ID.
- [ ] Replacement and rescheduled allocations reuse the claimed volume while
  it remains eligible.
- [ ] Scaling a group creates distinct claims according to the public
  compatibility rules and never assigns one exclusive volume to two writers.
- [ ] Claim list and read results are namespace filtered and deterministically
  ordered.
- [ ] Deleting a claim requires confirmation and the correct ACL, and the next
  eligible placement may select a different volume.
- [ ] Job purge and garbage collection retain or remove claims at the publicly
  documented lifecycle points.

### US-011: Expose the compatible HTTP API

**Description:** As an automation author, I want the documented HTTP endpoints
so that existing tools can manage dynamic host volumes against Wander.

**Acceptance Criteria:**

- [ ] The list, read, create, register, update, delete, deregister, claim list,
  and claim delete operations use the documented verbs, paths, query fields,
  request bodies, response bodies, status codes, and index headers.
- [ ] Create without an ID creates a new volume; create or register with a
  valid existing ID performs only a permitted update.
- [ ] Blocking list and read calls return after a matching index change or
  timeout.
- [ ] A malformed ID, missing node, invalid state transition, in-use volume,
  or plugin failure returns a stable machine-testable error and does not
  partially mutate state.
- [ ] Namespace and region forwarding match Wander's existing API conventions.
- [ ] Unknown JSON fields follow Wander's compatibility-warning policy rather
  than being silently treated as implemented.

### US-012: Extend the Go API client

**Description:** As a Go integrator, I want typed client operations so that I
do not need to hand-build HTTP requests.

**Acceptance Criteria:**

- [ ] Public Go types encode and decode every documented dynamic volume and
  claim field without exposing server-internal state types.
- [ ] Typed methods cover list, read, create, register, update, delete,
  deregister, claim list, and claim delete.
- [ ] Query and write metadata expose Raft indexes and request timing in the
  same way as existing Wander API methods.
- [ ] Contract tests run the Go client against a Wander HTTP test server using
  synthetic requests and assert exact method, path, query, and body shapes.

### US-013: Provide compatible CLI workflows

**Description:** As an operator, I want volume CLI commands so that I can use
the same operational flow for dynamic host volumes and CSI volumes.

**Acceptance Criteria:**

- [ ] `wander volume init` can emit a synthetic host-volume specification in
  HCL and JSON without changing its existing CSI behavior.
- [ ] `wander volume create` accepts a file or standard input, supports the
  documented detach and monitoring behavior, and prints the returned ID.
- [ ] `wander volume register` adopts an existing host path and clearly states
  that no underlying storage was created.
- [ ] `wander volume status` and list output distinguish host and CSI volumes
  and support documented prefix, namespace, node, plugin, and verbose options.
- [ ] `wander volume delete` and deregistration workflows display the exact
  target and consequences, require compatible confirmation unless explicitly
  bypassed, and reject in-use volumes.
- [ ] Claim list and claim delete commands support documented filtering,
  prefix resolution, confirmation, namespace, and ACL behavior.
- [ ] Ambiguous prefixes list matches and perform no mutation.

### US-014: Enforce namespace and host-volume ACLs

**Description:** As a security administrator, I want granular capabilities so
that volume responsibilities can be delegated safely.

**Acceptance Criteria:**

- [ ] The ACL parser accepts the documented host-volume create, register,
  read, write, delete, read-only mount, and read-write mount capabilities.
- [ ] Each API and CLI mutation succeeds with its minimum documented
  capability and returns permission denied when that capability is absent.
- [ ] Wildcard namespace queries return only volumes and claims visible to the
  token.
- [ ] Node-read permission does not implicitly grant access to namespace-bound
  volume details or host paths beyond the documented contract.
- [ ] A read-only mount capability cannot submit or run a read-write request.
- [ ] ACL decisions are covered by a table-driven synthetic allow/deny matrix.

### US-015: Manage dynamic volumes in the Wander UI

**Description:** As a web UI operator, I want to inspect and manage dynamic
host volumes without using the CLI.

**Acceptance Criteria:**

- [ ] The volume list distinguishes dynamic host volumes from CSI volumes and
  displays synthetic name, state, namespace, node, plugin, capacity, and claim
  count only when authorized.
- [ ] The detail page displays capabilities, allocation references, claims,
  lifecycle indexes, and diagnostics while hiding unauthorized host paths and
  parameters.
- [ ] Create and register forms validate required fields and show a review step
  before submission.
- [ ] The registration review explicitly says the existing path will be added
  to Wander state and its contents will not be copied or modified.
- [ ] Delete, deregister, and claim-delete dialogs identify whether underlying
  storage is affected and require explicit confirmation.
- [ ] UI actions disappear or become disabled when the session lacks the
  required capability, while server-side ACL enforcement remains authoritative.
- [ ] Empty, loading, unavailable, forbidden, validation-error, plugin-error,
  and success states have browser-level tests using synthetic data.

### US-016: Protect destructive operations and concurrent state

**Description:** As an operator, I want lifecycle mutations serialized and
guarded so that a race cannot detach or destroy live data.

**Acceptance Criteria:**

- [ ] Delete fails while any non-terminal allocation or active claim prevents
  deletion under the public compatibility contract.
- [ ] The documented force option changes only the documented state checks and
  never bypasses ACL enforcement or plugin path confinement.
- [ ] Two concurrent delete requests result in one successful lifecycle
  transition and one stable already-deleted or not-found response.
- [ ] Server leadership loss during a plugin operation reconciles to one
  durable outcome without issuing an unsafe duplicate delete.
- [ ] Audit logs contain actor, namespace, volume ID, operation, and outcome,
  but do not contain tokens, private host-path contents, or unredacted
  sensitive parameters.

### US-017: Preserve compatibility during upgrades and mixed operation

**Description:** As a cluster operator, I want a safe rollout so that adding
dynamic volumes does not destabilize existing workloads.

**Acceptance Criteria:**

- [ ] An upgraded server can restore and replicate a pre-feature snapshot.
- [ ] Feature-gated protocol fields are not sent to an older peer that cannot
  decode them during a supported rolling-upgrade window.
- [ ] Static host volume jobs and CSI jobs produce the same plans and mounts
  before and after the feature is enabled.
- [ ] A dynamic volume is not reported ready until the assigned client can
  support the required protocol and operation.
- [ ] Downgrade limitations are documented before an operator creates the
  first dynamic volume.

### US-018: Document clean-room provenance and migration

**Description:** As a project maintainer, I want auditable evidence and a safe
migration runbook so that the implementation remains license-safe and useful.

**Acceptance Criteria:**

- [ ] A compatibility matrix maps every public interface behavior to an
  official public documentation URL, an independent black-box observation, or
  a Wander-authored requirement.
- [ ] No matrix entry links to or quotes post-1.6.5 source, tests, patches,
  commits, generated assets, or implementation details.
- [ ] Repository scanning confirms that committed fixtures contain none of the
  private migration artifact's names, IDs, paths, topology, or values.
- [ ] The runbook registers a fully synthetic pre-existing directory, runs a
  job that verifies a marker, restarts the client, verifies the marker again,
  deregisters without deleting data, and separately demonstrates deletion of
  a plugin-created disposable directory.
- [ ] Documentation warns that Wander creates new Raft records and does not
  import or convert later Nomad Raft state or snapshots.

## 4. Functional Requirements

### Clean-room and privacy boundary

- **FR-1:** Implementation work must follow
  `docs/plans/license-safe-parity-method.md`.
- **FR-2:** Post-1.6.5 Nomad source, tests, patches, commits, generated assets,
  and implementation details must not be accessed or used for this feature.
- **FR-3:** Approved post-1.6.5 evidence is limited to publicly documented
  interfaces and independently observed black-box behavior.
- **FR-4:** Private infrastructure artifacts must remain local and must not be
  copied into source, tests, logs, screenshots, issues, pull requests, CI, or
  external services.
- **FR-5:** Every committed fixture and example must use synthetic names,
  identifiers, paths, node layouts, capacities, and parameters.
- **FR-6:** Each implementation phase must record evidence provenance and pass
  the repository's licence-boundary checks before commit.

### Data and state

- **FR-7:** Wander must define independent public API and internal state models
  for dynamic host volumes, capabilities, constraints, states, allocations,
  and sticky claims.
- **FR-8:** Volume and claim records must be namespace scoped and persisted in
  Raft with create and modify indexes.
- **FR-9:** State operations must support deterministic list, prefix,
  pagination, blocking query, snapshot, restore, and garbage-collection
  behavior.
- **FR-10:** State transitions must be explicit and reject invalid or stale
  compare-and-set operations without partial writes.
- **FR-11:** Registration must persist a new Wander record for an existing
  node/path without mutating the path's contents.

### Client and plugins

- **FR-12:** Client configuration must support plugin discovery and managed
  volume roots without changing static host-volume semantics.
- **FR-13:** Clients must fingerprint plugins, report their identity and
  health, and reconcile volume readiness after restart or reload.
- **FR-14:** Plugin processes must receive only the publicly documented
  operation contract, run with time and output bounds, and be cancellable.
- **FR-15:** Plugin results must be validated before state becomes ready.
- **FR-16:** Mutating operations for one volume must be serialized and
  idempotent across retries and leadership changes.
- **FR-17:** The built-in directory plugin must confine all managed paths below
  its configured root and defend against traversal, symlink, and filesystem
  race attacks.
- **FR-18:** External paths adopted through registration must never be treated
  as plugin-created disposable storage without an explicit compatible
  lifecycle transition.

### Scheduling and allocation lifecycle

- **FR-19:** Node selection for creation must enforce node ID, node pool,
  constraints, plugin health, and per-node name uniqueness.
- **FR-20:** Allocation placement must require a ready volume with compatible
  name, node, capability, access mode, attachment mode, and read-only state.
- **FR-21:** Plan and apply must atomically prevent incompatible concurrent
  writers.
- **FR-22:** The client must mount the selected dynamic host path using the
  existing task-driver mount abstraction without weakening path or propagation
  validation.
- **FR-23:** Sticky task groups must create, reuse, list, and delete durable
  claims according to the public behavior contract.
- **FR-24:** Terminal allocations, rescheduling, scaling, job purge, node
  unavailability, and garbage collection must each have tested claim and
  attachment behavior.

### API, Go client, CLI, ACLs, and UI

- **FR-25:** The HTTP API must implement the complete documented dynamic host
  volume and claim lifecycle with compatible verbs, paths, fields, status
  codes, query options, and index metadata.
- **FR-26:** The Go API package must provide typed models and methods for every
  supported HTTP operation.
- **FR-27:** CLI commands must support HCL and JSON input, stdin, prefix
  resolution, monitoring, filtering, confirmation, and machine-readable error
  behavior where publicly documented.
- **FR-28:** Namespace ACLs must include and enforce the complete documented
  host-volume capability set at both HTTP and RPC boundaries.
- **FR-29:** The web UI must provide list, detail, create, register, update,
  delete, deregister, and claim workflows permitted by the user's ACLs.
- **FR-30:** UI and CLI destructive actions must explain whether storage or
  only Wander state is affected before confirmation.

### Reliability, observability, and testing

- **FR-31:** Volume lifecycle operations must remain safe under duplicate
  requests, server failover, client disconnect, plugin timeout, and process
  restart.
- **FR-32:** Metrics must cover plugin health, reconciliation, lifecycle
  duration, failures, ready/unavailable counts, placement rejections, and
  claims without using volume names or host paths as unbounded labels.
- **FR-33:** Audit and diagnostic logs must identify operations sufficiently
  for troubleshooting while redacting secrets and private data.
- **FR-34:** Tests must be written before implementation for each user story
  and use only synthetic data.
- **FR-35:** Security-critical negative tests for ACLs, path confinement,
  concurrency, state transitions, and destructive operations must be mutation
  tested.
- **FR-36:** Release validation must cover Linux amd64 and arm64, supported
  task drivers, client restart, server failover, snapshot restore, and a
  multi-namespace ACL matrix.

## 5. Non-Goals

- Reading, converting, importing, or modifying post-1.6.5 Nomad Raft state,
  snapshots, encryption keys, or internal database records.
- Copying or adapting any post-1.6.5 Nomad implementation material.
- Embedding any private infrastructure artifact or identifier in Wander.
- Automatically discovering arbitrary existing directories and registering
  them without an explicit operator request.
- Moving or copying the contents of a registered external directory.
- Providing a cloud-specific storage plugin as part of the core feature.
- Replacing CSI volumes or changing their public lifecycle behavior.
- Automatically converting static host volumes into dynamic volumes.
- Importing ACL tokens with chosen IDs or migrating unrelated cluster state.
- Guaranteeing data durability supplied by the underlying filesystem or
  plugin; Wander manages orchestration and compatibility, not storage media.
- Building post-compatibility convenience features that are absent from the
  approved public interface unless separately approved.

## 6. Technical Considerations

### 6.1 Independent implementation boundaries

Design should begin with Wander's current 1.6.5-derived abstractions:

- static client host-volume configuration and allocation mount hooks;
- CSI endpoint, state, CLI, and ACL patterns where their Wander-owned current
  implementation provides a general repository convention;
- Raft FSM commands, state-store tables, snapshots, blocking queries, RPC
  forwarding, and scheduler feasibility checks;
- existing Go API, command registration, Templ/HTMX UI, and browser-test
  conventions.

These are architectural integration points, not an instruction to force
dynamic host volumes through CSI semantics. New types and state transitions
should be designed for the documented host-volume contract.

### 6.2 Proposed component areas

- Public and internal volume/claim types, validation, copying, hashing, and
  generated serialization.
- State-store tables and indexes, Raft messages, FSM application, snapshot,
  restore, and garbage collection.
- Server RPC endpoints, HTTP routes, forwarding, blocking queries, and ACL
  checks.
- Client configuration, plugin manager, fingerprinting, operation runner,
  reconciliation, and node updates.
- Scheduler feasibility, plan/apply attachment accounting, sticky claims, and
  evaluation diagnostics.
- Allocation runner integration for dynamic host paths.
- Go API models and client methods.
- CLI parsers, commands, monitoring, output, and confirmations.
- UI routes, views, forms, ACL presentation, and end-to-end tests.
- Documentation, migration runbook, compatibility matrix, and release notes.

### 6.3 Registration and migration model

The migration-safe path is an explicit register operation:

1. The operator identifies a client node and an existing absolute host path.
2. Wander validates authorization, specification, node eligibility, path
   availability, capabilities, and name conflicts.
3. The assigned client confirms the path according to the public contract.
4. Wander writes a new dynamic-volume record to its own Raft state.
5. The node advertises the volume as ready.
6. A job referring to the volume name is placed on that node and mounts the
   exact registered directory.

The operation must not depend on, read, or translate another scheduler's Raft
state. A migration rehearsal should compare application-visible synthetic
marker data before and after registration and restart.

### 6.4 Lifecycle state machine

The implementation plan must define an explicit state machine for requested,
pending, ready, unavailable, deleting, and terminal failure conditions based
on the approved public behavior evidence. Each transition must identify:

- the authoritative actor (server, assigned client, or plugin);
- the durable Raft mutation;
- idempotency key and retry behavior;
- timeout and failover behavior;
- whether allocation placement is permitted;
- whether delete, update, or deregister is permitted;
- operator-visible diagnostics.

State machine, defined 2026-11-22. The server record is the authority; every
change of state is one Raft command. The client is the authority for what
exists on disk and reports it with its node fingerprint.

| State | Meaning | Placement | Update | Delete or deregister |
|-------|---------|-----------|--------|----------------------|
| requested | Accepted and stored; no node chosen yet | no | no | yes |
| pending | Node chosen; the client has not confirmed the volume | no | no | yes |
| ready | The client confirmed the volume and advertises it | yes | yes | yes, when not in use |
| unavailable | Was ready; the client no longer advertises it | no | no | yes, when not in use |
| deleting | Removal accepted; the client has not confirmed it | no | no | repeat is accepted |
| failed | Create or register failed for good | no | no | yes |

| From | To | Trigger | Actor | Retry and failover |
|------|----|---------|-------|--------------------|
| none | requested | Create or register request passes validation and ACL checks | server | The request carries the volume ID; a repeat with the same ID and the same content is a no-op, with different content it is an update or a conflict |
| requested | pending | Node selection succeeds (create) or the named node is accepted (register) | server | Same Raft command as the step above when the node is known at once |
| requested | failed | No node qualifies | server | The diagnostic of node selection is stored on the record |
| pending | ready | The client RPC returns the path and capacity | server, on the client's answer | The client call is idempotent per volume ID. A new leader repeats the call for every pending record; a client that already holds the volume answers with the same result |
| pending | failed | The plugin or the path check fails, or the call times out for good | server, on the client's answer | A timeout is retried with backoff up to a bound, then failed. The client rolls back a half-made volume |
| ready | unavailable | The node fingerprint no longer lists the volume, or the node is down | server | Derived from the node update in the same Raft apply |
| unavailable | ready | The node fingerprint lists the volume again | server | As above |
| ready, unavailable, failed, pending, requested | deleting | Delete or deregister request; refused while an allocation uses the volume; on a down or unknown node only with force | server | A repeat is accepted and changes nothing |
| deleting | removed | The client confirms removal, or force on a down or unknown node | server, on the client's answer | A new leader repeats the client call for every deleting record. Sticky claims of the volume go in the same Raft apply |
| ready | ready | Update of the mutable fields | server | Compare-and-set on the modify index |

Rules that follow from the table:

- Only `ready` permits placement. The plan applier checks again, at apply
  time, that the volume exists, is ready and is on the node of the placement,
  and rejects that node's part of the plan otherwise. This closes the window
  between plan evaluation and a delete.
- `deleting` is entered before the client is called, so a placement can never
  be accepted for a volume whose removal is under way.
- Deregister follows the same path as delete; the client drops its record and
  leaves the directory and its contents alone.
- Every refused transition returns an error that names the current state.
  The diagnostic of the last failed step is stored on the record without
  host paths of other namespaces.

### 6.5 Compatibility contract capture

Before implementing each public operation, add a matrix row containing:

- method, path, command, configuration field, or job field;
- accepted inputs and defaults;
- response or output shape;
- error and authorization behavior;
- concurrency and lifecycle observations;
- source category and public URL when applicable;
- synthetic Wander acceptance-test identifier.

Black-box observations must record only requests, responses, and
operator-visible behavior. They must not inspect files, symbols, stack traces,
debug artifacts, or implementation internals of a post-1.6.5 binary.

### 6.6 Security model

Threats requiring explicit design and tests include:

- path traversal and symlink escape from the managed root;
- time-of-check/time-of-use filesystem races;
- command or environment injection into executable plugins;
- hostile, hanging, or excessively verbose plugins;
- deletion of registered external storage by lifecycle confusion;
- ACL bypass through RPC forwarding or namespace wildcards;
- host path disclosure to unauthorized users;
- conflicting writers caused by concurrent scheduling;
- replayed, duplicated, or stale lifecycle requests;
- node identity changes and stale plugin advertisements;
- unbounded state, parameters, output, metrics labels, or claims;
- secrets or private values appearing in logs and UI errors.

### 6.7 Test strategy

Tests should be layered and tests-first:

1. Pure validation and state-machine unit tests.
2. State-store, Raft, snapshot, restore, and blocking-query tests.
3. Plugin runner and confined filesystem tests using temporary directories.
4. Scheduler plan/apply and concurrency tests with synthetic nodes.
5. Client restart and reconciliation integration tests.
6. HTTP, Go client, CLI, ACL, namespace, and region contract tests.
7. UI handler, template, accessibility, and browser tests.
8. End-to-end job tests using a disposable directory and marker file.
9. Failover, timeout, unavailable-node, in-use delete, and recovery tests.
10. Licence/provenance and private-fixture scanning before release.

Tests must not call an external service or require access to private storage.
Any optional black-box compatibility harness must be local, excluded from the
repository, and emit only reviewed generic contract results.

### 6.8 Delivery strategy

The implementation should be split into independently reviewable phases while
keeping incomplete public surfaces feature-gated:

1. Evidence matrix, types, validation, state model, and Raft persistence.
2. Read/list API and Go client.
3. Client configuration, plugin discovery, runner, and built-in directory
   plugin.
4. Create/register/update reconciliation and restart recovery.
5. Scheduler integration, access-mode enforcement, and allocation mounts.
6. Sticky claims and lifecycle garbage collection.
7. Delete/deregister safety, failure recovery, and auditability.
8. ACL capability matrix.
9. Complete CLI surface.
10. Complete UI surface.
11. Synthetic end-to-end, failover, upgrade, security, and release tests.
12. Documentation, migration rehearsal, compatibility review, and release.

No phase may expose a mutation endpoint before its state, ACL, idempotency,
failure, and destructive-operation tests pass.

## 7. Success Metrics

- Every row in the approved public dynamic-host-volume interface matrix has at
  least one passing synthetic acceptance test.
- A synthetic existing directory is registered, mounted by a job, survives a
  client restart and server snapshot restore, and retains its marker contents.
- A plugin-created synthetic volume completes create, mount, restart, unmount,
  delete, and state-removal lifecycle tests.
- Scheduler stress tests never assign incompatible simultaneous writers to one
  exclusive volume.
- The full ACL matrix passes across at least two synthetic namespaces with no
  cross-namespace data disclosure.
- Existing static host volume and CSI test suites show no behavioral
  regression.
- Linux amd64 and arm64 release validation passes for the supported dynamic
  host volume workflow.
- Licence/provenance review finds no prohibited post-1.6.5 implementation
  material.
- Repository privacy review finds no values from private migration artifacts.
- A migration rehearsal using disposable synthetic storage completes without
  copying or modifying pre-existing marker contents during registration.

## 8. Open Questions

- Which Wander release will first advertise the compatibility surface as
  stable rather than experimental?
- Should the compatibility matrix and optional local black-box harness live in
  the main repository or in a separate public conformance repository containing
  only synthetic cases?
- Which supported platforms besides Linux must provide the built-in directory
  plugin in the first stable release?
- Which task drivers form the required release gate beyond the existing driver
  mount abstraction tests?
- Should the operator migration runbook be shipped in this repository, the
  public website, or both?

## References

Approved public interface evidence, accessed 2026-11-08:

- [Host volumes](https://developer.hashicorp.com/nomad/docs/architecture/storage/host-volumes)
- [Dynamic host volume specification](https://developer.hashicorp.com/nomad/docs/other-specifications/volume/host)
- [Volumes HTTP API](https://developer.hashicorp.com/nomad/api-docs/volumes)
- [Create a host volume plugin](https://developer.hashicorp.com/nomad/plugins/author/host-volume)
- [Use dynamic host volumes](https://developer.hashicorp.com/nomad/docs/stateful-workloads/dynamic-host-volumes)
- [Stateful deployments](https://developer.hashicorp.com/nomad/docs/concepts/stateful-deployments)
- [Job volume block](https://developer.hashicorp.com/nomad/docs/job-specification/volume)
- [Volume CLI](https://developer.hashicorp.com/nomad/commands/volume)
- [ACL policies](https://developer.hashicorp.com/nomad/docs/secure/acl/policies)

Private owner-supplied migration artifacts are deliberately not identified or
linked here. They are local-only acceptance evidence and are not part of the
public implementation record.
