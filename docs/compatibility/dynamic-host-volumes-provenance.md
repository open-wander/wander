# Dynamic host volume provenance

This record accompanies [the JSON matrix](dynamic-host-volumes.json).
It captures interface evidence on 2026-11-08, 2026-11-15 and 2026-11-17, not an
implemented feature or a complete verified contract. Every runtime acceptance test is currently planned.
The offline validator tests the ledger itself only.

## Evidence rules {#evidence-rules}

Permitted: official public interface documentation, independently captured
requests/responses and operator-visible black-box behavior, and separately
identified Wander requirements. Public documentation is interface evidence only.
Do not follow source links or inspect example plugin implementations.

Prohibited: post-1.6.5 source, tests, patches, commits, repositories, generated
assets, implementation details, symbols, stack traces, debug artifacts, and
private infrastructure evidence. No such evidence is required by this ledger.
No black-box runs have been conducted for this record; there are no observation
sources. An observation must record the public binary version, synthetic request,
response, date, and limitations here before being referenced.

Future observation sources must point to their own heading anchor beginning
`observation-`. Each such section must contain one fenced JSON record with
`source_id` matching its source entry, `binary_version`, `synthetic: true`,
`request`, `response`, `date` matching its source entry, and `limitations`.
Version, request, response, and limitations must be nonempty strings. Record
only public requests, responses, and operator-visible behavior. Requirement and
policy anchors cannot serve as observation records. No observation record is
present in this document today; synthetic validator fixtures exist only in memory.

All examples, future requests, fixtures, names, IDs, paths, capacities, topology,
and parameter values must be independently invented synthetic data. No private
artifacts were accessed to build this ledger. The validator checks declared
privacy policy and evidence boundaries; it cannot prove arbitrary text contains
no private identifiers. Human review remains required. Never compare fixtures to
private artifacts in public logs or services.

## Wander requirements {#requirements}

The authorized dynamic-host-volume PRD enumerates HTTP, Go client, CLI, client
configuration, plugins, built-in plugin, job fields, ACLs, claims, scheduling,
lifecycle, and UI work. US-001 through US-018, FR-1 through FR-36, and section
6.5 are requirement identifiers, not evidence that upstream implements them.
A requirement assertion records desired Wander behavior only. An unresolved
assertion contains no behavior value and carries an explicit question.
This record does not authorize importing later state or snapshots.

## Sources consulted {#sources}

The JSON sources collection records the official documentation URLs, date,
interface scope, independent summaries, and limitations. Only the public API,
specification, plugin protocol specification, job fields, client configuration,
volume command index, and ACL policy documentation were consulted.
Documentation currently labels several pages v2.1.x; URLs are mutable and no
release-specific fidelity or binary conformance is claimed. Interface tokens
retain compatibility names under [the boundary](../compatibility-boundary.md).
No source repository, generated asset, or later release implementation was used.

## Limitations and open questions {#limitations}

Each row separates its primary contract from inputs/defaults, outputs, errors,
authorization, concurrency, and lifecycle. Null values mean unresolved; absence
of a documented error does not imply success or an invented status code.
The current operation inventory includes requested surfaces whose existence or
exact spelling still requires evidence.

[The synthetic fixture catalog and guard](dynamic-host-volumes-fixtures.md)
provide reusable scenario inputs without accessing private artifacts. The guard
admits only its closed synthetic vocabulary in the registered fixture directory
and checks references against this public matrix. It does not scan other
repository files or prove their privacy. Its passing result demonstrates fixture
admission and redacted diagnostics, not runtime compatibility; matrix runtime
acceptance tests remain planned. Unresolved fixture outcomes remain null.

The Wander-authored internal models in `nomad/structs/dynamic_host_volume.go`
have requirement-scoped tests in `nomad/structs/dynamic_host_volume_test.go`
(`TestDynamicHostVolumeCopyIsolation`, `TestDynamicHostVolumeEqualityAndValueCopies`,
`TestDynamicHostVolumeCollections`, and `TestDynamicHostVolumeSerialization`).
These cover the representation portion of US-001 and FR-7: nil-safe isolated
copies, complete equality, collection representation, zero/unknown labels, and
typed JSON/MessagePack integer preservation. Internal enum labels and codec
field names are independent Wander requirements, not public wire evidence.
These tests do not cover validation, lifecycle transitions, Raft persistence,
snapshots, public API compatibility, or runtime integration.

The JSON matrix currently classifies only public interface surfaces; it has no
internal-model surface. Attaching these implemented model tests to unresolved
public assertions would overstate their evidence. The matrix and synthetic
catalog therefore remain unchanged, with their runtime acceptance tests planned.
Run model tests both with and without `-tags codegen_generated`; the tagged run
exercises the repository-generated codecs for these independently written types.

Internal validation is tested separately in
`nomad/structs/dynamic_host_volume_validation_test.go` by
`TestDynamicHostVolumeOperationValidation`, `TestDynamicHostVolumeCapacityValidation`,
`TestDynamicHostVolumePathValidation`,
`TestDynamicHostVolumeCapabilityAndAttachmentValidation`,
`TestDynamicHostVolumeClaimValidation`, `TestDynamicHostVolumeImmutableValidation`,
and `TestDynamicHostVolumeValidationAggregationAndRedaction`. These implement
Wander-authored representation rules for US-001/FR-7 and an internal identity
guard; they do not establish public validation envelopes or update permissions.

Create and register require name, a caller-resolved nonempty namespace, type
`host`, and supported capabilities. Create requires a plugin ID, permits an
omitted node ID, and ignores the host path. Register requires node ID and host
path, permits an omitted plugin ID, and checks only lexical absoluteness and
absence of NUL. It does not clean, resolve, inspect, create, or test availability
of a path. Node existence, pool membership, and name uniqueness remain deferred.

Internal capacities must be nonnegative and the requested minimum must not
exceed the maximum even when that maximum is zero. This literal comparison is
an internal requirement; omitted/zero public maximum semantics remain unresolved.
Capabilities and attachments currently support only filesystem attachment and
single-node reader-only/writer access. Reader-only attachments require read-only
usage. Unknown strings still serialize unchanged but are rejected by these
current mode validators. Attachments require allocation/node identities, while
claims require their stable identity fields and permit an omitted allocation ID
and zero allocation index. Constraint validation delegates to the existing
validator after a nil check and discards its potentially value-bearing errors.

The immutable guard compares ID, namespace, name, type, node ID, origin, create
index, and create time, including clears and nil operands. Passing this guard
does not authorize changes to paths, plugins, pools, parameters, capacities,
capabilities, or other metadata. No validation method mutates/defaults records,
merges updates, executes plugins, enforces ACLs, or checks attachment conflicts,
ownership, lifecycle, state, or scheduling policy. Errors aggregate in fixed
field order and expose sentinel codes with safe field names; only unsupported
mode values are included, truncated to 32 bytes and ASCII-quoted.

The required initial validation test run failed before implementation with
`ValidateCreate undefined` and `ValidateRegister undefined` using
`go test ./nomad/structs -run '^TestDynamicHostVolume.*Validat' -count=1`.
Public matrix assertions and fixture outcomes remain unchanged/unresolved.
No persisted type shape changed, so no codec generation is needed for this work;
normal and `codegen_generated` package tests exercise the existing serializers.

Internal state storage is tested separately in
`nomad/state/state_store_dynamic_host_volume_test.go`. The initial test-first
run failed because `UpsertDynamicHostVolumes` and `DynamicHostVolumeByID` were
undefined. The completed tests cover namespace-scoped identities, ID/node/plugin
prefixes, deterministic compound-index order, copy isolation, watch forwarding,
create-only and exact-index compare-and-set, monotonic table indexes, immutable
updates, batch rollback, claim references, deletion integrity, and concurrent
writers. These are Wander-authored storage requirements for US-002 and FR-8
through FR-10, not evidence of an upstream implementation.

Volume names are unique for one `(namespace, node ID)` pair. The same name may
exist on another node or in another namespace. This is an internal Wander policy
chosen because records are namespace scoped; cross-namespace public behavior
remains unresolved. Claim IDs are unique within a namespace, while volume and
job indexes are non-unique because scaling and writer exclusivity belong to the
scheduler. New records must not supply a create index: the transaction assigns
both create and modify indexes. Updates and deletes require the stored modify
index, and a failed batch changes neither records nor the table index.

The storage layer does not select create versus register validation, set wall
clock values, invoke plugins, remove paths, authorize requests, enforce
attachment modes, or decide public idempotency. It rejects volume deletion while
a claim references that volume and rejects namespace deletion while any dynamic
volume remains. Snapshot serialization and restore are described in
[Snapshots and downgrade](#snapshots-and-downgrade). Raft FSM commands,
complete blocking-query response behavior, pagination, garbage collection,
public APIs, and lifecycle transitions remain later work. No matrix runtime
outcome is changed by these internal tests.

The HTTP delete table includes a delete suffix while its example omits it.
The claim delete table uses a plural collection while its example uses singular.
These conflicts must be resolved with permitted independent black-box requests
before implementation. Claim read, independent update, deregistration, force,
confirmation, CLI flag details, Go method names, full response schemas,
pagination, wildcard visibility, ACL shorthand, claim retention, and UI flows
remain unresolved where explicitly marked. Never infer host semantics from CSI.
Documented fields alone do not define validation rules or default behavior.

## Snapshots and downgrade {#snapshots-and-downgrade}

Dynamic host volumes use Wander-assigned snapshot type bytes: 29 for volumes
and 30 for sticky claims. They are written after namespaces, volumes before
claims, each in `(namespace, ID)` order. The values are append-only and must
never be renumbered or reused. Persisted order and the decoded record set are
deterministic; the bytes are not guaranteed identical because the shared
msgpack handle is not canonical. Table indexes travel in the existing index
snapshot entries, so blocking queries resume from the saved index.

Restore rejects a nil record, a record that breaks the stored-record
invariants (volume type, required and NUL-free keys, create index set and not
above the modify index, claim validation), a duplicate `(namespace, ID)`, and a
node and name conflict between volumes. It performs no namespace or
claim-to-volume referential checks, so a filtered restore keeps working. A
failed restore leaves the previous state live; the new store is discarded.

Upgrade: a snapshot written before this feature contains neither type and
restores unchanged, leaving both tables empty.

Downgrade limitation: once any dynamic volume or claim exists, every Raft
snapshot contains types 29 and 30. An older Wander binary has no restorer for
them, and its restore code refuses the whole snapshot with
`Unrecognized snapshot type`. That binary cannot start from such a snapshot or
install it from a leader, which affects all state, not only the volumes.
Do not downgrade after creating the first dynamic volume unless every volume
and claim has been deleted and a fresh snapshot taken first. The behaviour is
covered by `TestFSM_SnapshotRestore_DynamicHostVolumes_DowngradeRejectsUnknownType`,
which uses an unregistered type byte to stand in for the older binary.

## ACL capabilities {#acl-capabilities}

The five capabilities `host-volume-create`, `host-volume-register`,
`host-volume-read`, `host-volume-write` and `host-volume-delete` are namespace
capabilities. Their meaning comes from the public ACL policy specification
(`SRC-acl-spec`, read on 2026-11-15) and is recorded in DHV-B-137 to DHV-B-141
and DHV-B-144:

| Capability or shorthand | Grants |
| --- | --- |
| `host-volume-read` | Itself only. |
| `host-volume-delete` | Itself only. |
| `host-volume-create` | Itself and `host-volume-read`. |
| `host-volume-register` | Itself, `host-volume-create` and `host-volume-read`. |
| `host-volume-write` | All five. |
| `policy = "read"` | `host-volume-read`. |
| `policy = "write"` | `host-volume-write`, and therefore all five. |
| `policy = "scale"` | None. |

An earlier version of this record treated the capabilities as independent and
left them out of the shorthands, because the specification had not yet been
recorded. That was corrected on 2026-11-15.

The existing `mount-readonly` and `mount-readwrite` stay `host_volume` block
capabilities keyed by volume name, and a read-only mount grant does not satisfy
a read-write mount check.

Still unresolved, and therefore Wander policy:

- Wildcard and glob namespaces (DHV-B-142) follow the existing namespace rules:
  an exact match beats a glob, the closest glob replaces rather than merges,
  and management tokens pass every check.
- Deny precedence (DHV-B-143) follows the existing rule: `deny` revokes the
  capabilities like any other namespace capability, across merged policies.
- Node policies grant none of them.
- The specification states the implications in the capability descriptions and
  lists only `host-volume-write` in the `write` expansion. Wander applies the
  implications to a capability however it was granted, including through a
  shorthand.

Upgrade: a Wander server without this change rejects any policy that lists a
`host-volume-*` capability as an invalid namespace capability, and its `read`
and `write` shorthands do not include them. During a rolling upgrade, apply
policies that list the capabilities only after every server runs the new
version.

Covered by `TestParse_HostVolumeNamespaceCapabilities` in `acl/policy_test.go`
and `TestHostVolumeNamespaceCapabilities` in `acl/acl_test.go`. Enforcement in
RPC, HTTP and CLI, wildcard-namespace list filtering and the UI are out of
scope here.

## Plugin discovery and fingerprinting (client) {#plugin-fingerprint}

Implemented in `client/hostvolume` and `client/fingerprint/plugins_host_volume.go`.
The public plugin specification (`SRC-plugin`, re-read on 2026-11-15) resolves
these points, recorded in DHV-B-079, DHV-B-080 and DHV-B-084: a candidate is an
executable in `host_volume_plugin_dir`; it is called with first argument
`fingerprint` and `DHV_OPERATION=fingerprint`; stdout is a JSON object with a
`version` field that must be a valid version for the go-version library; the
time limit is five seconds. An earlier version of this record accepted a plain
version line because the output format had not yet been recorded. That was
corrected on 2026-11-15.

Everything marked as policy below is Wander policy chosen from this
repository's own patterns, not taken from any later Nomad release.

| Question | Wander policy |
| --- | --- |
| Candidates | Regular files (Lstat semantics) directly in the directory with any execute bit set. Directories, symlinks, FIFOs, sockets and names starting with a dot are skipped silently; a non-executable regular file is excluded with reason `not_executable`. No recursion. |
| Windows | The execute bit has no meaning, so no external plugins are discovered on Windows (reason `unsupported_platform`). |
| Plugin ID | The file name: 1 to 64 characters from `[A-Za-z0-9._-]`, not starting with a dot. Otherwise excluded with `invalid_id`. At most 32 candidates are run per pass (sorted by ID); the rest get `too_many_plugins`. |
| Reserved ID | `mkdir` (DHV-B-103) is reserved for the built-in plugin. An external file named `mkdir` is excluded with `reserved_id`. The built-in plugin is advertised separately, in-process, as `plugins.host_volume.version.mkdir`; see [Built-in mkdir plugin](#builtin-mkdir). An external `mkdir` never affects that attribute. |
| stdout | Evidence: one JSON object with a string `version`, valid for go-version. Policy: surrounding whitespace is allowed, unknown fields are ignored, data after the object is rejected, and the version is 1 to 64 characters from `[A-Za-z0-9._+-]` and is advertised exactly as printed. Output that is not one JSON object, or a `version` that is not a string, is `invalid_output`; a missing, empty, over-long, unsafe or unparseable version is `invalid_version`. |
| Exit status | Exit 0 is healthy. Any other status is `nonzero_exit`. |
| Output limits | stdout 1024 bytes, stderr 1024 bytes. Exceeding either kills the plugin: `output_too_large`. |
| Timeout | 5 seconds (evidence). The plugin and its process group are killed: `timeout`. |
| Environment | Exactly `DHV_OPERATION=fingerprint` and a fixed `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin` (needed by script plugins). `DHV_OPERATION=fingerprint` is evidence; the fixed `PATH` is policy. `DHV_PLUGIN_DIR` and `DHV_VOLUMES_DIR` are not set because the specification lists them for create and delete only. The agent environment is not inherited. |
| Working directory | The plugin directory. No shell is used; argv is `[path, "fingerprint"]`; stdin is empty. |
| Diagnostics | A reason code plus at most 256 bytes of stderr or stdout with control characters and invalid UTF-8 replaced by spaces. A warning is logged when a plugin's reason changes, otherwise debug. |
| Node representation | Node attribute `plugins.host_volume.version.<id>` = version, mirroring the inherited `plugins.cni.version.<name>`. Present means healthy; absent means unhealthy or not installed. `structs.Node` is unchanged, so no codec change. |
| Health change | The fingerprinter is periodic (one minute). Each pass advertises healthy plugins and removes every `plugins.host_volume.version.*` node attribute that is no longer healthy, so removal is correct after a client restart. It writes and deletes no volume records. |
| Determinism | Candidates are sorted by ID before running, so the inventory does not depend on directory listing order. Plugins run with 4 workers; the worst-case pass is 32/4 timeouts (40 seconds). |
| Empty or missing directory | Empty setting: fingerprinter reports not detected. Missing or empty directory: detected, no plugins, no error. The fingerprinter never returns an error for plugin or directory problems, because the inherited fingerprint manager stops re-running a periodic fingerprinter after one error. |
| Directories differ | The agent rejects a configuration whose resolved plugin and volume directories are equal after making them absolute and cleaning them. The comparison is lexical: equality through a symlink, or directories that do not exist yet, are not resolved. |

Accepted limits: a file swapped for a symlink between the directory check and
execution is not detected (the directory is operator-owned); a plugin that
detaches a grandchild from its process group can outlive its kill, though the
client stops waiting after a one second grace period.

Mixed versions: an older server stores node attributes as an opaque
`map[string]string` and reads them only through job constraints, so it ignores
the new `plugins.host_volume.version.*` attributes. No serialised struct
changed, so there is no encode or decode round trip to test. An older client
does not advertise the attributes, which a server must read as "no plugins".

## Plugin create and delete (client) {#plugin-operations}

Implemented in `client/hostvolume/operation.go` as a library (`Ops.Create`,
`Ops.Delete`). The volume manager is its only caller, and no server RPC
reaches it yet. The built-in `mkdir` plugin is in [its own section](#builtin-mkdir);
restart reconciliation is in [another](#restart-reconciliation) and registering
existing paths in [a third](#register-handshake). `Ops.Create` and
`Ops.Delete` still reject the id `mkdir` (`reserved_id`), because it names no
external executable.

### Recorded evidence

From the public plugin specification (`SRC-plugin`).

| Id | What it settles |
| --- | --- |
| DHV-B-081 | Create is called with first argument `create`. It prints a JSON object with `path` and `bytes` (the example shows a number). The path may be inside `DHV_VOLUMES_DIR` or elsewhere. Time limit 60 seconds. |
| DHV-B-082 | Delete is called with first argument `delete`. Stdout is discarded. Repeated deletion succeeds. Time limit 60 seconds. No capacity variables are listed. |
| DHV-B-083 | Repeating create with the same inputs gives the same result. |
| DHV-B-084 to DHV-B-095 | Variable names: `DHV_OPERATION`, `DHV_VOLUMES_DIR`, `DHV_PLUGIN_DIR`, `DHV_NAMESPACE`, `DHV_VOLUME_NAME`, `DHV_VOLUME_ID`, `DHV_NODE_ID`, `DHV_NODE_POOL`, `DHV_CAPACITY_MIN_BYTES`, `DHV_CAPACITY_MAX_BYTES` (bytes), `DHV_PARAMETERS` (JSON text), `DHV_CREATED_PATH` (delete, the path create returned). |
| DHV-B-096 | A plugin may print a JSON object with an `error` field; the message is included in the error returned to the user. Whether this depends on a non-zero exit is not stated. |
| DHV-B-097 | A failed create cleans up and exits non-zero. No exit code requirement is stated for delete. The plugin runs as the agent's user. |
| DHV-B-098 | The plugin cleans up a failed create; partial creates are not deleted for it. Delete is called when the volume cannot be saved after an initial create. The volume manager makes that call when it cannot store the record of a new volume. |
| DHV-B-099 | Create and delete must be safe to run concurrently per volume name per node. This task serialises per volume ID inside one process. Two different IDs with the same name on one node are the server's responsibility and are not handled here. |
| DHV-B-101 | No automatic retry; the caller retries. A plugin may retry inside its deadline. |

DHV-B-100 (output limits) is unresolved in the evidence; the limits below are
Wander policy.

### Wander policy

#### Wander policy pending black-box verification

Decided by the owner on 2026-11-17 because the evidence does not state them.
Each is one small function or constant in `operation.go` so that it is easy to
change when black-box verification against a real implementation is allowed.

| # | Point | Decision | Evidence ids |
| --- | --- | --- | --- |
| 1 | Capacity variables | Always set, base-10 integers. Unset capacity is `0` (`formatCapacity`). Negative input is rejected as `invalid_input`. | DHV-B-092, DHV-B-093 |
| 2 | `DHV_PARAMETERS` with no parameters | Always set. Nil or empty is exactly `{}` (`encodeParameters`). | DHV-B-094 |
| 3 | Empty values | Every documented variable for the operation is always set. An empty value is the empty string, never omitted (`buildEnv`). | DHV-B-084 to DHV-B-095 |
| 4 | Create output | `path` is required and must be a JSON string. `bytes` is optional: missing is `0`; present it must be a non-negative JSON integer written as `0` or digits without a leading zero, fitting int64. Fractions, exponents, strings, negatives, `-0`, `null` and overflow are `invalid_output` (`parseCreateOutput`). | DHV-B-081 |
| 5 | Error object | Read only when the plugin exits non-zero, for create and delete (`parsePluginError`). For delete this is the only use of stdout. On exit 0, create output is parsed as a result, so an object without a valid `path` is `invalid_output` even if it has an `error` field; delete succeeds on exit 0 whatever stdout says. | DHV-B-096, DHV-B-097 |

#### Other policy

| Question | Wander policy |
| --- | --- |
| Environment | Exactly the documented variables for the operation plus the fixed `PATH` used for fingerprint, in a fixed order. Create has no `DHV_CREATED_PATH`; delete has it and no capacity variables. The agent environment is never inherited. Working directory is the plugin directory, stdin is empty, argv is `[path, operation]`, no shell. |
| Parameters encoding | Compact JSON, keys sorted, no HTML escaping (`<`, `>` and `&` are written as is), no trailing newline, so equal inputs give byte-identical text. Invalid UTF-8 in a key or value is rejected before encoding because `encoding/json` would replace it silently. At most 64 KiB encoded. |
| Input validation | Done before the lock or any process: plugin ID valid and not `mkdir`, volume ID non-empty (it is the lock key), no NUL byte in any field, parameter key or value or in the created path, parameters valid UTF-8, capacities not negative, and for delete the created path passes the same path rules as a create result. Reason `invalid_input` (`reserved_id` for `mkdir`). Messages never carry a rejected value. |
| Output limits | DHV-B-100 is unresolved. Create stdout 16 KiB (more than the 4096 byte path limit plus envelope and escaping); stderr 4 KiB for both operations. Exceeding the create stdout limit or any stderr limit kills the plugin: `output_too_large`. Delete keeps at most 4 KiB of stdout, only to read the error object; more stdout is discarded without killing the plugin. If delete stdout was truncated, the error object is not read and the message is `exit N`. |
| Path rules | A create result path must be non-empty, at most 4096 bytes, valid UTF-8, free of NUL, control (below U+0020, U+007F), non-printable characters and U+FFFD, absolute, equal to its cleaned form (so `..`, `.`, `//` and a trailing slash are rejected) and not a filesystem root. It need not lie under `DHV_VOLUMES_DIR` (evidence). Reason `invalid_path`. The returned `bytes` is not compared with the requested capacity; the evidence gives no rule. |
| JSON details | One object, surrounding whitespace allowed, unknown fields ignored, data after the object rejected, for a duplicate key the last one wins. A malformed envelope, a missing or non-string `path` or an invalid `bytes` is `invalid_output`. |
| Per-volume lock | One operation at a time per volume ID within one client process; different IDs run concurrently. A channel semaphore with a reference count per ID, so waiting honours the caller context and entries are removed when nobody holds or waits. The lock is released on every path. The 60 second deadline starts after the lock is held; waiting is bounded only by the caller context (`canceled`, no plugin started). No cross-process or cross-restart locking. |
| Reason codes | `invalid_input`, `reserved_id`, `invalid_path`, `invalid_output`, `output_too_large`, `nonzero_exit`, `timeout`, `canceled`, `start_failed`. Precedence: output overflow, caller context (a caller deadline is `canceled`), operation deadline (`timeout`), exit status, start failure. Every failure returns the zero result. |
| Plugin error text | The message is the sanitised `error` string of a complete stdout object after a non-zero exit, at most 256 bytes, with control characters and invalid UTF-8 replaced by spaces; otherwise `exit N`. Stderr is never put in the error. |
| Redaction | Every non-empty parameter value is replaced by `[redacted]` in plugin error text, longest first, before it is sanitised and cut to its bound, so a cut value cannot leak a prefix. Values never appear in errors. Keys are not secret. The library does not log. A short value such as `1` makes the replacement visible in unrelated text; that is accepted. |
| Timeout and kill | 60 seconds for both, overridable on `Ops.Timeout` for tests. On expiry or cancellation the process group is killed on Unix, with the one second wait delay used by fingerprint. |
| Retries | None (DHV-B-101). Retrying create with the same request runs the plugin again with identical inputs. |
| Windows | The code compiles on Windows. External plugins are not discovered there, so the operations are not reachable; the shell based tests are excluded by build tag. |

## Node selection for creation (server) {#node-selection}

### Recorded evidence

`node_pool` is optional and must match the node's pool when a node is given
(DHV-B-121). A volume name is required and unique per node (DHV-B-118).

### Wander policy

The evidence does not say how a node is chosen among several eligible ones; the
rules below are Wander policy. Selection is a pure function of a state snapshot
and the request.

| Question | Wander policy |
| --- | --- |
| Explicit node | Only that node is considered. An unknown node ID is rejected with `node_not_found`. A node pool that differs from the node's pool is rejected with `node_pool_mismatch` (DHV-B-121). The node still passes every rule below. |
| Node pool `all` | Treated as every pool, as the scheduler does. With an explicit node, `all` never gives `node_pool_mismatch`. An empty node pool also means every pool. A pool that holds no nodes gives the no-eligible-node diagnostic, not a separate rejection. |
| Rules | Each node is counted once, under the first rule it fails, in this order: `node_pool` (no explicit node only), `node_not_ready`, `node_ineligible`, `node_draining`, `plugin_unhealthy` (node attribute `plugins.host_volume.version.<plugin id>` absent), `constraint` (existing scheduler constraint semantics; `distinct_hosts` and `distinct_property` are ignored), `name_conflict`. |
| Name conflict | A static host volume of that name on the node, a ready dynamic volume in the node's host volumes, or any dynamic volume record for that node with that name in any namespace and any state. |
| Tie-break | Not in the evidence. The eligible node with the fewest dynamic volume records (any namespace and state) wins; equal counts go to the lowest node ID. |
| Diagnostic | Top-level reason `no_eligible_node` plus per-reason counts sorted by reason code. It holds no node IDs, constraint text or parameter values. |
| Existing volume | Selection is for creation only. A volume's own record and its entry in the node's host volumes count as a name conflict, so selection is not re-run for a volume that already has a node. |

## Built-in mkdir plugin (client) {#builtin-mkdir}

Implemented in `client/hostvolume/builtin.go` (`Ops.CreateBuiltin`,
`Ops.DeleteBuiltin`) with `builtin_unix.go` and `builtin_windows.go`. It runs in
the client process, with no executable, and returns the same `CreateResult` as
external plugins. It shares the per-volume lock of `Ops`, so built-in and
external operations on one volume ID never interleave. The volume manager is
its only caller, and no server RPC reaches it yet.

### Recorded evidence

| ID | Evidence |
| --- | --- |
| DHV-B-103 | Plugin id `mkdir` creates a directory in the host volumes directory named after the volume ID. `capacity_min` and `capacity_max` are ignored because they cannot be enforced. Delete behaviour is not stated. |
| DHV-B-104 | Parameter `mode`: optional string in numeric notation applied to the created directory, default `0700`. On Windows only the owner-writable bit is used. |
| DHV-B-105 | Parameter `uid`: optional integer, defaults to the user running the agent. The ID must be present on the node. Ignored on Windows. |
| DHV-B-106 | Parameter `gid`: optional integer, defaults as for `uid`. The ID must be present on the node. Ignored on Windows. |

### Wander policy

Everything below is Wander policy, not recorded evidence.

| Point | Decision |
| --- | --- |
| Create result (DHV-B-103) | `Path` is `<volumes dir>/<volume id>`; `Bytes` is always 0. Capacity is ignored and is never an error. |
| Idempotency (DHV-B-103) | Create for an existing plain directory reuses it: no second directory, same result, and ownership and mode are applied again. Contents are untouched. |
| Delete (DHV-B-103, DHV-B-110) | Removes the volume directory and its contents; succeeds when the directory or the volumes directory is already gone, and never creates the root. It takes no path: it derives `<volumes dir>/<volume id>` itself, so a registered external directory cannot be targeted. Symlinks are never followed. A symlink or regular file found at the volume name is itself removed (harmless and confined), not its target. A directory with no owner write or search permission (mode such as `0500` or `0`) may fail to delete for a non-root agent; that reports `filesystem_failed`. |
| Volume ID (DHV-B-108) | One safe path element, 1 to 64 bytes of `[a-z0-9_-]`. This excludes separators, NUL, empty, `.`, `..`, a leading dot, absolute paths and Windows device names structurally. Upper case is excluded because the lock is keyed on the exact string while macOS and Windows filesystems fold case. Server-generated IDs are lower-case UUIDs. Reason `invalid_input`. |
| Mode (DHV-B-104) | 1 to 4 octal digits (`755`, `0755`). Text with a `0o` prefix, sign, whitespace, or digits 8 and 9 is rejected, as is the empty string. Only permission bits `0777` are accepted: setuid, setgid and sticky are rejected, so the rule is "permission bits and nothing else". The mode is applied with an explicit chmod, so the process umask does not change it. A mode without owner access (for example `0`) is accepted but can make later re-creation or deletion fail for a non-root agent. |
| uid and gid (DHV-B-105, DHV-B-106) | Decimal digits only, no sign or space, in 0 to 2147483647. 4294967295 is rejected because chown reads it as "no change". Absent values default to the agent uid and gid, and ownership is always applied explicitly because BSD and macOS give a new directory the parent group. |
| Presence check (DHV-B-107) | None. The numeric ID is passed to chown as is. `os/user` lookups need cgo or NSS and would reject LDAP or SSSD users in pure-Go builds. That the ID exists on the node is the operator's responsibility; an ID the kernel refuses fails the create with `filesystem_failed`. |
| Failure of chown or chmod (DHV-B-107) | The create fails with `filesystem_failed` and returns the zero result. A directory created by this call is removed (`Remove`, not `RemoveAll`); a directory that existed before is left in place with whatever state it had. If removal fails the message says so. |
| Symlinks (DHV-B-109) | The managed root's final element must be a plain directory, not a symlink (`invalid_path`); ancestors are the operator's and may be links. The root is opened once with `os.OpenRoot` and checked with `os.SameFile` against the earlier `Lstat`. The volume path is `Lstat`ed (a symlink, file or anything else than a directory is `invalid_path`), opened with `O_NOFOLLOW|O_DIRECTORY`, and compared with `SameFile` again. Ownership and mode are applied to the open descriptor, never by path. |
| Managed root | Created with `MkdirAll` mode `0711` if missing on create, because the default `data_dir/host_volumes` does not exist on a fresh agent. |
| Unknown parameters | Ignored, so a specification written for another version still works. Only `mode`, `uid` and `gid` are read. |
| Advertised version | `plugins.host_volume.version.mkdir` = `1.0.0` (`BuiltinMkdirVersion`), set whenever the volumes directory is configured and the platform is supported; not advertised otherwise, and a stale attribute is removed on the next pass. |
| Windows | Disabled: both operations return `unsupported_platform` and nothing is advertised. External plugins are already disabled there, and symlink, junction and reparse-point semantics plus the read-only attribute make the owner-writable-bit-only behaviour hard to confine and to test. |
| Reason codes | `invalid_input` (bad ID, parameter or volumes directory), `invalid_path` (confinement), `filesystem_failed` (any other filesystem error, new), `canceled`, `unsupported_platform`. Messages are fixed text and never carry a parameter value or a raw OS error. |

## Restart reconciliation and availability (client) {#restart-reconciliation}

The only recorded evidence about restart is DHV-B-102: a create failure while
restoring volumes at agent start is logged, the client keeps running and the
volume is not available on the node. DHV-B-118 records that a volume name is
required and unique per node. The rest of this section is Wander policy, not a
statement about Nomad behaviour, and is implemented in `client/hostvolume`
(`manager.go`) with the record type in `client/hostvolume/state`.

### Wander policy

1. Node entry. `ClientHostVolumeConfig` gains `ID`: the volume ID for a dynamic
   volume, empty for a static one. It is not an agent setting (the HCL tag is
   `-` and the agent clears it when it converts `host_volume` blocks). The
   field is not added to the public `api` package. A dynamic volume is
   reported as `{Name, Path, ReadOnly: false, ID}` in the node's existing
   `HostVolumes` map, only while it is ready, so the existing scheduler check
   keeps jobs off a node where the volume is unavailable.
2. Records. The client keeps one persisted record per dynamic volume in the
   client state store (bucket `hostvolumes`, keyed by volume ID, no state
   version change). The manager is the only code that writes or removes
   records. A record that cannot be decoded or fails validation is skipped
   and logged by key, never by content.
3. Names. A ready dynamic record whose name equals a static `host_volume` name
   is not reported (reason `name_conflict`) and the static entry is never
   replaced. Between dynamic records on this node, in any namespace, the lower
   volume ID is reported and the others get `name_conflict`. Records are
   compared only among those that would otherwise be ready.
4. Node. A record whose node ID is not this node is never reported (reason
   `node_mismatch`).
5. Registered records. Restore and reconcile only `os.Stat` the path and never
   write under it (reason `missing_path`).
6. Plugin-created records. Restore runs the plugin's create once with the
   persisted request and volume ID (the built-in mkdir plugin through
   `CreateBuiltin`). A missing plugin or an unhealthy fingerprint gives
   `plugin_unavailable`, a failed create gives `restore_failed`, and a create
   that returns another path gives `path_changed` while the persisted path is
   kept. For the built-in plugin a changed volumes directory gives
   `path_changed` without calling create, so no second directory appears.
7. Periodic pass. Every 30 seconds (fixed, injectable for tests, not a
   setting) every record is checked again. A ready record is only stat-checked
   and create is not re-run for it. An unavailable record is retried as in
   restore, with two exceptions. A record held back only by `name_conflict` is
   stat-checked. A record with `path_changed` is left as it is until the next
   client start, because every retry is another create and a plugin that
   keeps returning new paths would keep making storage that nothing removes.
   A plugin-created volume whose directory disappeared (`missing_path`) is
   retried, so its plugin creates it again, empty.
8. Diagnostics. Each record keeps its last status (`ready` or `unavailable`),
   a reason code, a sanitized message of at most 256 bytes and the time of the
   last check. Parameter values are replaced before the message is cut to its
   bound and never appear in logs, messages or error strings. The status and
   reason are written when they change; the check time is kept in memory on
   every pass and written with the next change, so a quiet node does not write
   to the state store every interval.
9. Start. Restore runs in the client's shutdown group and never delays or
   fails client start. The store is read once, by restore or by the first
   entry point call, whichever comes first, and only records read from the
   store are restored: a volume created in the same run is not created
   again. A record read from the store is not reported until this run has
   checked it, whatever status the earlier run stored, so an entry point
   call that arrives before restore has reached a record reports only the
   volumes created or checked in this run. Restore is bounded by a total
   deadline of 5 minutes and by the Ops timeouts of each plugin call. A
   volume not verified within the deadline is unavailable (`restore_failed`)
   until the next periodic pass. Shutdown cancels it and stores and
   publishes nothing.

### Other policy

- Create entry point. It checks that the name is unused on the node, runs
  create, and persists the record only after create succeeds. If persisting
  fails after a successful create, the matching delete runs as a best-effort
  rollback (not when a record for the volume ID already existed) and the error
  is returned.
- Delete entry point. It applies only to plugin-created records. For a
  registered record it returns an error and changes nothing on disk or in the
  store; removing a registered record is not implemented, and DHV-B-179,
  DHV-B-180 and DHV-B-187 stay unresolved. A failed plugin delete keeps the
  record and its status.
- Registration. The in-process `AddRegistered` entry point requires that the
  path exists and records it. The client RPC that calls it is described under
  [Registration handshake](#register-handshake).
- Locking. The manager has its own per-volume lock, separate from the lock
  table of `Ops`, and re-reads the record after taking it, so a pass that
  waited for a delete does not re-create the volume.

## Registration handshake (client) {#register-handshake}

The server-to-client exchange is internal to Wander and is not a
compatibility question. The only public evidence is the register endpoint,
its ACL capability, that `host_path` is the registration path, that `node_id`
is required and that a name is required and unique per node.

### Wander policy

1. The client RPC method is `HostVolume.Register`. It is for servers only: it
   adds no HTTP route and performs no ACL check, because the server
   authorizes before it calls the node.
2. The request carries VolumeID, Namespace, Name, NodeID, NodePool and
   HostPath. The response carries VolumeID, HostPath, Status, Reason and
   Message.
3. Register never runs a plugin and never creates, renames, copies, truncates,
   chowns, chmods, formats or removes anything at or under the path. It only
   calls `os.Stat`.
4. Path rules are those of the path validator (absolute, clean, no NUL or
   control characters, not a filesystem root, at most 4096 bytes) plus
   existence. A regular file is accepted.
5. NodeID must be this node and is never defaulted. Namespace, name and volume
   ID must not be empty.
6. A name used by a static host volume, or by another dynamic volume on this
   node in any namespace, is rejected.
7. Repeating a registration with the same volume ID and the same namespace,
   name and path succeeds, returns the current status (even if the path has
   since disappeared), writes nothing and triggers no node update. NodePool is
   not compared. A repeat that differs in namespace, name or path is rejected
   and changes nothing. An ID that belongs to a plugin-created volume is
   rejected.
8. A classified rejection returns a nil RPC error with Reason set and an empty
   Status. Only transport or internal failures (store write failure, canceled
   or timed-out lock wait, missing manager) return an RPC error.
9. Check order: field validation, then node, then the existing record for the
   ID, then the name, then `os.Stat`. The first failure decides the code.

| Reason code | Cases |
| --- | --- |
| `invalid_path` | relative, unclean, NUL or otherwise invalid path |
| `missing_path` | the path does not exist |
| `invalid_input` | empty or malformed volume ID, name, namespace or node ID |
| `node_mismatch` | well-formed NodeID of another node |
| `name_conflict` | static or dynamic name already used on this node |
| `volume_kind_conflict` | the ID belongs to a plugin-created volume |
| `registration_mismatch` | a repeat differs in namespace, name or path |

## Extension workflow {#extension-workflow}

1. Identify the operation or field and the PRD references. Add a stable behavior
   ID and at least one synthetic acceptance-test ID before implementing it.
2. Consult allowed public documentation only, or record independent black-box
   requests and responses under these rules. Add a dated scoped source with
   limitations. Separate documentation, observations, and requirements.
3. Populate each assertion with granular evidence references and test IDs.
   Keep unsupported facets null and list the question. Resolve contradictions
   explicitly; never treat a source URL as proof of every facet.
   PRD references must resolve to identifiers actually defined in the active
   repository PRD; identifier spelling alone does not establish a reference.
4. Write and run the failing synthetic runtime test, then implement independently.
   Change a test to implemented only with an existing repository path and test
   name. Mark behavior implemented/verified only when every facet is resolved
   and every linked test is implemented. Keep unrelated questions explicit.
5. Run `python3 -m unittest discover -s ci -p test_dynamic_host_volume_contract.py`
   and `python3 ci/validate_dynamic_host_volume_contract.py`, the license-boundary
   check, diff whitespace check, and required Deslop scan. Review provenance and
   synthetic content before publication. The offline checks do not establish
   runtime compatibility or legal clearance.

The initial ledger test failed before either this record or the matrix existed:
`AssertionError: False is not true : public dynamic-host-volume matrix is missing`.
