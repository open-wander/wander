# Dynamic host volume provenance

This record accompanies [the JSON matrix](dynamic-host-volumes.json).
It captures interface evidence on 2026-11-08, 2026-11-15, 2026-11-17 and
2026-11-20, not an
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
single-node access in the four modes reader-only, writer, single-writer and
multi-writer. Reader-only attachments require read-only
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
an allocation uses that volume, deletes a volume's claims with the volume and a
job's claims with the job (see [Sticky claims](#sticky-claims)), and rejects
namespace deletion while any dynamic volume remains. Snapshot serialization and
restore are described in [Snapshots and downgrade](#snapshots-and-downgrade).
Raft FSM commands, complete blocking-query response behavior, pagination,
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

#### Points the specification does not state

The owner decided these on 2026-11-17 because the public specification does
not state them. They were compared with an official release binary on
2026-11-20 ([observation](#observation-plugin-inputs)). Each is one small
function or constant in `operation.go`.

| # | Point | Wander | Observed | Evidence ids |
| --- | --- | --- | --- | --- |
| 1 | Capacity variables | Set on create as base-10 integers; unset capacity is `0` (`formatCapacity`). Not set on delete. Negative input is rejected as `invalid_input`. | The same. | DHV-B-092, DHV-B-093 |
| 2 | `DHV_PARAMETERS` with no parameters | The JSON text `null` (`encodeParameters`). It was `{}` until the observation; the owner chose to match on 2026-11-20. | `null`, on create and delete. | DHV-B-094 |
| 3 | Empty values | Every variable of the operation is set; an empty value is the empty string (`buildEnv`). `DHV_CREATED_PATH` is not set on create. | The same for the cases seen: `DHV_CREATED_PATH` is not set on create and is the empty string on delete when create returned no path. | DHV-B-084 to DHV-B-095 |
| 4 | Create output | `path` is required and must be a JSON string. `bytes` is optional: missing is `0`; present it must be a non-negative JSON integer written as `0` or digits without a leading zero, fitting int64. Fractions, exponents, strings, negatives, `-0`, `null` and overflow are `invalid_output` (`parseCreateOutput`). | A result without `bytes` is accepted and the capacity is reported as 0. | DHV-B-081 |
| 5 | Error object | Read only when the plugin exits non-zero, for create and delete (`parsePluginError`). On exit 0, create output is parsed as a result, so an object without a valid `path` is `invalid_output` even if it has an `error` field; delete succeeds on exit 0 whatever stdout says. | An error object with exit 0 is treated as a successful create: the volume is reported ready with no host path. Wander deliberately differs (owner decision, 2026-11-20), because a ready volume without a path cannot be mounted. | DHV-B-096, DHV-B-097 |

Two more differences were seen and are kept (owner decision, 2026-11-20). The
observed agent passes no `PATH` to the plugin and runs it in the agent's
working directory. Wander passes a fixed `PATH` and runs the plugin in the
plugin directory.

#### Observation of plugin inputs {#observation-plugin-inputs}

A synthetic plugin that records its arguments and environment was installed on
a development-mode agent of an official release binary, on a workstation with
no other cluster. Four synthetic volumes were created and deleted. The plugin
and the volumes held no real data, and identifiers are not recorded here.

```json
{
  "source_id": "SRC-observe-plugin-inputs",
  "binary_version": "Nomad v2.0.2 (official release binary, darwin/arm64)",
  "synthetic": true,
  "request": "Create and delete of four synthetic host volumes through a synthetic plugin that records its arguments and environment: one with no capacity and no parameters, one with both, one whose create result omits bytes, and one whose create prints an error object and exits 0.",
  "response": "Fingerprint: one argument, only DHV_OPERATION set. Create: one argument; DHV_CAPACITY_MIN_BYTES and DHV_CAPACITY_MAX_BYTES are 0 when unset and byte counts when set; DHV_PARAMETERS is null with no parameters and a JSON object otherwise; DHV_CREATED_PATH is not set. Delete: capacity variables are not set; DHV_PARAMETERS as on create; DHV_CREATED_PATH is the created path, or the empty string when create returned none. No PATH is passed and the working directory is the agent's. A create result without bytes succeeds with capacity 0. A create that prints an error object and exits 0 succeeds and the volume is ready with no host path.",
  "date": "2026-11-20",
  "limitations": "One client, one version, one platform, development mode; behaviour may differ elsewhere."
}
```

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

## Scheduler feasibility (server) {#scheduler-feasibility}

Implemented in `HostVolumeChecker` (`scheduler/feasible.go`) and
`VolumeRequest.Validate` and `VolumeRequest.HostVolumeModes`
(`nomad/structs/volumes.go`).

### Recorded evidence

| ID | Evidence |
| --- | --- |
| DHV-B-129 | `attachment_mode` on a host volume request defaults to `file-system`. |
| DHV-B-130 | `access_mode` defaults to `single-node-writer`, or to `single-node-reader-only` when `read_only` is set. |

### Rules and their basis

Each rule is labelled documented, observed or Wander policy. Documented means
stated in public documentation, observed means seen in the behaviour of a public
release binary, and Wander policy means a choice made here.

| Rule | Basis | Wander behaviour |
| --- | --- | --- |
| Access modes | Documented | Four modes exist: `single-node-reader-only`, `single-node-writer`, `single-node-single-writer` and `single-node-multi-writer`. `attachment_mode` is empty or `file-system`. |
| Defaults | Documented | The defaults above are applied before matching (DHV-B-129, DHV-B-130). |
| Reader-only | Documented | `single-node-reader-only` requires `read_only`; otherwise validation fails. |
| Single-writer and multi-writer direction | Documented and observed | Both accept read/write and read-only requests. |
| Writer direction | Observed | A `read_only` request is not placed on a `single-node-writer` capability, even when the request names `single-node-writer`. Job submission accepts it; scheduling never places it. |
| Static volume | Observed | A volume from the agent configuration declares no capabilities. `attachment_mode` and `access_mode` on the request are ignored, and only the name and read-only rules apply. |
| Rejected modes | Wander policy | `block-device` and the `multi-node-*` modes, and unknown strings, are rejected at job validation. The observed binary accepts `block-device` at submission and never places it. |
| Mount options | Wander policy | Mount options on a host volume request stay rejected. |
| Dynamic volume | Wander policy | A node advertising a volume entry with an ID is feasible only if the record with that ID exists in the job's namespace, belongs to the candidate node, has the requested source as its name, is `ready`, and lists a capability that equals the effective modes of every request for it. The direction rule above then applies to the matched access mode, not to the access mode as written on the request. |
| Lookup error | Wander policy | A state error makes the node infeasible. |
| Per-alloc | Wander policy | The suffixed source is both the advertised name and the record name. |
| Metric reasons | Wander policy | The existing `missing compatible host volumes` stays for a missing name and for static mismatches. Dynamic failures give `dynamic host volume <source> is missing or not usable by this job` (no record, other namespace, other node, other name or lookup error), `... is not ready`, or `... has no matching capability`. A direction refusal uses the capability reason. Only the requested source is named; host paths, volume IDs and error text never appear. |
| Evaluation | Wander policy | Per node, not class-cached: the checker is among the availability checkers, because the computed node class leaves out host volumes and the volume ID. |

## Writer accounting (server) {#writer-accounting}

Implemented in `nomad/structs/dynamic_host_volume.go`
(`DynamicHostVolumeWriterConflict`, `DynamicHostVolumeUsableBy`), called from
the scheduler host volume checker and from `evaluateNodePlan` in
`nomad/plan_apply.go`.

| Topic | Basis | Rule |
| --- | --- | --- |
| Readers and writers | Observed | On a volume used through `single-node-single-writer`, several readers run together, at most one writer runs, a reader is not placed while a writer holds the volume, and a writer is not placed while readers hold it. |
| Other modes | Observed | `single-node-writer`, `single-node-multi-writer` and `single-node-reader-only` never limit the number of allocations. |
| Count above one | Observed | A job with `count = 2` writers gets one placement and one placement failure. |
| Wake-up | Observed | When the holding job stops, the waiting job is placed. |
| Mixed modes | Wander policy | The evidence covers users of one mode only. Two users of the same volume conflict when at least one reaches it through `single-node-single-writer` and at least one of them is read/write. Readers of that mode do not conflict with one another, and the other modes never conflict with each other. |
| Derivation | Wander policy | The holders are derived from allocations; nothing is stored in Raft and `Attachments` stays unused. The users of a volume are the proposed live allocations of the node whose own namespace is the volume namespace and whose task group, from the job version the allocation runs, has a host volume request whose source (with the per-alloc suffix of the allocation name) equals the volume name. An allocation never conflicts with itself. |
| Scheduler | Wander policy | Evaluated per node after the record checks, on the stored allocations of the node with the plan applied, so a stop in the same plan frees the volume, a sibling placed earlier in the same evaluation holds it, and an allocation the plan preempts still holds it (see Preemption). Skipped when the record lists no `single-node-single-writer` capability. The reason is `dynamic host volume <source> has no free writer claim`. A state error makes the node infeasible. |
| Plan applier | Wander policy | Two evaluations can find the volume free and both commit, so the applier re-checks each new placement against the snapshot allocations plus the plan: the record still exists in the job namespace, belongs to the node, is ready, still matches the capability and passes the direction rule, and no other allocation of the node, counted as the scheduler counts them, conflicts. A failure rejects the node's part of the plan with `dynamic host volume not usable` or `dynamic host volume writer conflict`, without naming a source, ID or path. Updates of existing allocations and plans with no dynamic volume requests are not re-checked and read no further state. |
| Live allocations | Wander policy | The scheduler counts every allocation that is not client-terminal, minus those the plan stops, so an allocation stopped by the server but still running counts until its client reports it complete. The applier uses non-terminal allocations, which is slightly looser; the scheduler is the stricter check. An allocation in `unknown` status on a disconnected node still holds the volume. |
| Updates | Wander policy | In-place updates stage the old allocation as stopped before selection, and the applier skips existing allocations, so neither conflicts with itself. A destructive update places the replacement because the old allocation is in the plan stop list. A rescheduled allocation is client-terminal and does not count. |
| Canaries | Wander policy | A canary writer next to a running `single-node-single-writer` writer on the same node is infeasible, so a deployment cannot progress on that node. |
| Preemption | Wander policy | An allocation that holds the volume is never preempted to make room for another user of the same volume: the checker runs before ranking, so a node with a holder is not ranked for a conflicting placement. An allocation that the plan preempts for another placement still holds the volume until its client reports it stopped, so a conflicting writer, or a reader of a writer, is not placed beside it in the same plan, and the applier rejects such a plan for the node with `dynamic host volume writer conflict`. Only a stop in the job's own update (`NodeUpdate`) frees the volume within the plan. Resource fitting is unchanged and still treats preempted allocations as gone. A plan that used to place a writer beside a preempted holder now gets a placement failure or an applier rejection, and with plan rejection tracking enabled the rejection counts towards node ineligibility. The preempted allocation's stored record is used, because the plan carries only a stub of it. |
| Blocked evaluations | Existing mechanism, no new code | The checker is a per-node availability check, so a filtered node does not mark its computed class ineligible and the blocked evaluation records the class as eligible. When the holder's client reports it complete or failed, `applyAllocClientUpdate` unblocks the class and the node, which re-queues the waiting evaluation. A job stop alone does not wake it; the wake comes when the client reports the allocation terminal. |
| Plan rejection tracking | Wander policy | With node plan rejection tracking enabled (off by default), repeated writer conflicts count towards marking a node ineligible, as with capacity races. |
| Release point | Wander policy | An attachment is released when the client reports the allocation complete, failed or lost. An allocation the server has stopped but that still runs on its client keeps the volume in the scheduler, so a second writer is not placed while the first can still have the volume mounted. |
| Unlinked replacement in one plan | Known gap | The scheduler removes the allocations a plan stops before it counts holders. A plan that stops a holder and places a writer that is not its replacement (another task group on the same source, or a system job) is accepted, and nothing on the client makes the new allocation wait for the old one. A linked replacement does wait, through the previous-allocation watcher. Counting stopped allocations would refuse every same-node destructive update, so the gap is left open. |
| Lost holder | Known gap | A holder reported lost does not wake a blocked evaluation by itself. The waiting evaluation is re-queued when the node returns or by another unblocking event. |

### Observation of scheduling {#observation-scheduling}

Synthetic jobs were run against a development-mode agent of an official
release binary, on a workstation with no other cluster, in three rounds. The
jobs and volumes held no real data, and identifiers are not recorded here.

```json
{
  "source_id": "SRC-observe-scheduling",
  "binary_version": "Nomad v2.0.2 (official release binary, darwin/arm64)",
  "synthetic": true,
  "request": "Synthetic service jobs with a group-level host volume request and no task mount were submitted to a one-node development agent holding one built-in-plugin volume per access mode and one static host volume. Jobs were submitted alone, in pairs and with count 2, read/write and read-only, with and without modes named in the request.",
  "response": "single-node-writer: two jobs and one job with count 2 all ran as writers; a read_only request was not placed, with or without the mode named. single-node-reader-only: two readers ran; a request without read_only that named the mode was rejected at submission. single-node-multi-writer: two writers and a reader ran together. single-node-single-writer: two readers ran together; one writer ran and a second writer was not placed; a reader was not placed while a writer ran; a writer was not placed within 20 seconds while two readers ran, with no placement failure reported; count 2 gave one placement and one placement failure; when the holding job was stopped the waiting job was placed. A request with the default mode was not placed on a single-writer volume. Static volume: requests naming an access mode, or an access and attachment mode, were placed, as was a read_only request. A multi-node access mode was rejected at submission; block-device was accepted and not placed. Every placement failure carried the same single reason text.",
  "date": "2026-11-21",
  "limitations": "One node, one version, one platform, development mode; group-level volume requests without a task mount; behaviour may differ elsewhere."
}
```

Known differences that are kept. Wander rejects `block-device` on a host
volume request at submission, where the observed binary accepts it and never
places it. Wander reports a separate reason for each kind of dynamic volume
failure, where the observed binary reports one reason for all of them.

### Observation of sticky claims {#observation-sticky}

Synthetic jobs were run against two development agents of an official release
binary on one workstation, on private ports, with no other cluster. The jobs
and volumes held no real data, and identifiers are not recorded here.

```json
{
  "source_id": "SRC-observe-sticky",
  "binary_version": "Nomad v2.0.2 (official release binary, darwin/arm64)",
  "synthetic": true,
  "request": "Synthetic service jobs with a group-level host volume request and sticky = true were run against a two-node development cluster on one workstation, with one built-in-plugin volume of the same name on each node. Allocations were stopped, jobs were updated, scaled, stopped, re-run, purged and garbage collected, a node was made ineligible, a claim and a claimed volume were deleted, and the claims API was read after each step.",
  "response": "A non-sticky job moved between the nodes across replacements. A sticky job got one claim at placement and stayed on the claimed volume across four replacements, three destructive updates and four re-runs after a stop. With the node of the claimed volume ineligible, the replacement was not placed (missing compatible host volumes) and returned to that volume when the node was eligible again. With count 2 on two claimed volumes, one node ineligible and its allocation stopped, the replacement was not placed on the other claimed volume. Scaling from 1 to 3 with two volumes placed one allocation on the unclaimed volume with a new claim and one on an already claimed volume with no new claim; scaling down kept the claims. Claims stayed after a job stop, were gone after a purge, and were gone with the job after a stop and a forced garbage collection. Deleting a claim returned 200 with an empty body, the allocation kept running, no claim appeared within 15 seconds, and the next replacement moved to the other volume and created a claim there. Deleting the same claim again returned 500 with the text: Task group volume claim does not exist. The singular path /v1/volume/claim/<id> returned 404. Deleting a claimed volume was refused while an allocation used it and succeeded while the job was stopped; the claim stayed listed, and the re-run job was placed on the other volume with a second claim. sticky with per_alloc was rejected at submission; sticky on a static host volume was accepted and created no claim. A claim lists AllocID, CreateIndex, ID, JobID, ModifyIndex, Namespace, TaskGroupName, VolumeID and VolumeName. The list filters job_id, task_group, volume_name and namespace worked, including the namespace wildcard.",
  "date": "2026-11-21",
  "limitations": "Two nodes, one version, one platform, development mode; one namespace; one volume per node; each case run once or a few times; behaviour may differ elsewhere."
}
```

The rule read from these results: a sticky group may only be placed on a
claimed volume while it has a claim whose volume exists and is not used by one
of its own live allocations. When it has none, any compatible volume of the
name is allowed, and a claim is created if the group has none on that volume.
This reading fits every case above; it is an inference, not a documented rule.

Known difference, decided by the project owner on 2026-11-21. The observed
binary deleted a claimed volume of a stopped job and kept the claim. Wander
allows the delete when no allocation uses the volume and removes the
volume's claims in the same step, so no stale claim remains.

### Observation of a node going down {#observation-node-down}

Synthetic jobs were run against two development agents of an official release
binary on one workstation, on private ports, with no other cluster. The jobs
and volumes held no real data, and identifiers are not recorded here.

```json
{
  "source_id": "SRC-observe-node-down",
  "binary_version": "Nomad v2.0.2 (official release binary, darwin/arm64)",
  "synthetic": true,
  "request": "On a two-node development cluster on one workstation, one built-in-plugin volume of a shared name was created on each node and one volume of a second name on the second node only. A sticky service job on the shared name and a plain service job on the second name were both running on the second node. The second node's agent was stopped without warning, started again, stopped again, and garbage collected after the jobs were purged, then started once more. Node status, the volume list, job status and the claims list were read after each step, and a volume delete was tried while the node was down and after it was garbage collected.",
  "response": "The node was marked down about 20 seconds after its agent stopped. Its three volume records stayed listed with state ready. Both allocations were marked lost, and neither job was placed on the first node: each reported one node excluded by the missing compatible host volumes filter, including the sticky job, whose volume name also existed on the first node. The claim was unchanged. Deleting a volume of the down node failed with status 500 and the text: volume cannot be removed from unknown node without force=true. When the agent started again both jobs were placed on the second node on the same volumes. After the second stop, a purge of both jobs and a forced garbage collection, the node was no longer listed, its volume records were still listed with state ready, no claim was listed, and the delete failed with the same text. When the agent started again the node was ready, the records were unchanged, a new job was placed on one of its volumes, and a volume on the first node was deleted normally.",
  "date": "2026-11-22",
  "limitations": "Two nodes, one version, one platform, development mode; one namespace; the node was stopped without warning, not drained; states were read within about 30 seconds of each change; each case run once; behaviour may differ elsewhere."
}
```

## Sticky claims (server) {#sticky-claims}

Implemented in `nomad/structs/dynamic_host_volume.go`
(`DynamicHostVolumeStickyAllowed` and its two helpers), called from the
scheduler host volume checker and from the plan applier, with claim creation
in `stickyClaimsForPlan` in `nomad/plan_apply.go`.

| Topic | Basis | Rule |
| --- | --- | --- |
| Field | Documented, observed | `sticky` on a group volume request, default false. Rejected with `per_alloc` (`volume cannot be per_alloc and sticky at the same time`) and on a type other than host. A change of `sticky` is a destructive update, as for `per_alloc`. |
| Static volumes | Observed | A sticky request on a static host volume follows no rule and creates no claim. |
| Placement rule | Inferred from the observation | The free claims of a group for a source are its claims whose volume record exists and that no live allocation of the group uses. With free claims, only a node whose volume is one of them is feasible. With none, every node that passes the other checks is feasible. |
| Scale-up | Wander policy | The observed binary placed a new allocation on a volume the group had no claim on. Wander does not implement a preference for unclaimed volumes: the feasibility rule (`DynamicHostVolumeStickyAllowed`, used by the scheduler host volume checker) has no ordering, and ranking is unchanged. With free claims a placement must use one. With none, a new allocation may share a claimed volume or take an unclaimed one, and an unclaimed one gets a claim at plan apply. Scaling down keeps the claims. |
| Live allocations | Wander policy | For this rule an allocation is live when it is terminal neither on the server nor on the client, and its client status is not `unknown`. An allocation the server has stopped but that still runs does not hold its claim, so a re-run straight after a stop returns to the claimed volume; on a single-writer volume it waits there for the writer rule. An allocation in `unknown` status is not live for this rule, unlike the writer rule above, where it still holds a single-writer volume. |
| Disconnected client | Decided by the project owner | The claim of a sticky group stays with its volume while the client is disconnected. The allocation in `unknown` status does not hold the claim, so the claim is free and the replacement for that allocation is forced onto its volume, whose node is not ready: it is not placed on another volume of the name and waits. Any other placement of the group, including a scale-up, also waits while that claim is the only free one. The plan applier applies the same rule, in the evaluation that marks the allocation unknown as well as in later ones. On reconnect the original allocation keeps its claim and its volume and no second claim is made; until the client reports, the allocation is still `unknown`, so a concurrent placement of the group may land on that volume next to the original, which does not split the data and is still held back by the writer rule on a single-writer volume. When `max_client_disconnect` ends the allocation is lost, and the replacement keeps waiting for the volume while the node stays away, as for a down node. |
| Which volume is used | Wander policy | One record exists per namespace, node and name, so a claimed volume is used when a live allocation of the group that requests the source is on the node of its record. Cost is bounded by the allocations of the job. |
| Same evaluation | Wander policy | The claim view is read again for every placement, with the plan's stops, preemptions and earlier placements applied. |
| Reason | Observed | A node filtered by the rule reports the existing `missing compatible host volumes` reason. |
| Plan applier | Wander policy | Each new placement is checked again against the snapshot plus the plan. A failure rejects the node's part of the plan with `dynamic host volume sticky claim`, naming nothing. |
| Claim creation | Observed; mechanism is Wander policy | A claim is created for each dynamic volume a new allocation of a sticky request is placed on and the group has no claim on. The leader generates the claim ID and sends the claims in the plan result, and they are written in the same transaction as the allocations. Several placements on one volume give one claim. |
| Claim contents | Wander policy | A claim keeps the allocation that created it. A later placement on the volume does not change the claim. `AllocationIndex` is the name index of that allocation. |
| Skipped claims | Wander policy | When the plan result is applied, a claim is skipped if the group already has one on the volume, if the volume record is gone, or if the job is gone. The allocations are still written. Skipping the claim of a removed job stops a plan that races a purge from leaving a claim that nothing would remove. |
| Removal with the job | Observed; mechanism is Wander policy | A job stop keeps the claims. Removing the job from state, by a purge or by job garbage collection, deletes its claims in the same transaction. A child job (periodic or dispatched) holds claims under its own ID, and they go when the child is removed. |
| Removal with the volume | Decided by the project owner | Deleting a volume deletes the claims on it in the same transaction. This differs from the observed binary, which keeps the claim. |
| Volume in use | Requirement, Wander policy | The state store refuses to delete a volume while an allocation on its node, in its namespace, requests its name and has not been reported complete, failed or lost by the client. This is the release point of writer accounting. An allocation the server has stopped but that still runs blocks the delete. Refusing a delete because the node is down or unknown, as observed, belongs to the delete operation and is not implemented yet. |
| Node down or collected | Observed | Volume records and claims are not changed when their node goes down or is garbage collected. A down or missing node is not a placement candidate, so a sticky group with a claim there waits and is not placed on another volume of the name, and it returns to the volume when the node is ready. The record's state is not changed by the server; the observed binary also kept listing it as ready. |
| No volume collection | Observed | No volume record is removed automatically. Only an operator delete removes one. |
| Canaries | Wander policy | A canary beside a running allocation of the group finds the claim in use, so it may take another volume and get a claim there. |
| Partial plan | Known gap | Nodes are judged separately. If one node of a plan is rejected, a placement on another node that was valid only because of the rejected one can still commit and create a second claim. Later evaluations see both claims. |
| Plan rejection tracking | Known gap | As for writer conflicts, a sticky rejection counts towards marking a node ineligible when node plan rejection tracking is enabled (off by default). |
| Applier liveness | Known gap | The plan applier re-checks writers against allocations that are not terminal on the server, while the scheduler and the volume delete count an allocation until the client reports it terminal. A plan made from an older snapshot can therefore pass the applier next to a writer the server has stopped but that still runs. |
| Mixed versions | Known gap | A server without this change drops the claims in a plan result and writes the allocations alone, so claim state differs between servers until all are upgraded. No version gate is possible: servers advertise the same compatibility version as before. For the disconnected-client rule, an older server's applier accepts a replacement on another volume that a new scheduler never proposes, and a new applier rejects one from an older scheduler. |

## Task mounts (client) {#task-mounts}

Implemented in `client/allocrunner/taskrunner/volume_hook.go`,
`client/node_updater.go` (`hostVolumesFor`) and `client/hostvolume/manager.go`
(`Restored`, `Namespace`).

### Recorded evidence

A group volume request of type `host` whose source names a dynamic host volume
is mounted through the task's `volume_mount`, as for a static host volume
(`SRC-job`). No public source describes the timing of the mount on the client.

### Wander policy

| Topic | Rule |
| --- | --- |
| Current set | The volume hook reads the host volumes of the node as they are at prestart, through a function the client passes to the task runner. Without that function it uses the configuration captured when the task runner was built, which holds static volumes only. |
| Agent restart | A restored allocation can reach prestart before the volume manager has restored the dynamic volumes. When a requested source is missing, the hook waits until the first restore has ended, bounded by the restore timeout, and reads the set again. When the task was restored with a recovered driver handle and the source is still missing, the task keeps running: the missing sources are logged at warn level (names and count, no paths), no mounts are recorded and prestart returns without an error, because the driver already holds the mounts. A task that still has to start fails with `host volume validation error`, as does a restored task that restarts later in the same allocation while the volume is still missing, since the handle has been cleared by then. The hook cannot tell a static name that is not configured from a dynamic volume that is not ready, so a missing static name on a restored running task is also tolerated; this changes the inherited behaviour, where such a task was killed once the restart policy was exhausted. The restored-running case is detected by the driver handle, not by the task state. When the task is killed or the agent stops during the wait, the hook returns without an error, without mounts and without marking itself done, so the task is neither failed nor started and the hook runs again at the next prestart; cancellation wins over a restore that ends at the same moment. A task whose volumes are all present does not wait. |
| Missing volume | A source that is still missing after the wait fails the task with `missing <source>`, as before, except for a task restored with a recovered driver handle (see Agent restart). A static name that is not configured fails with the same message, after the same wait when the agent has just started. |
| Read-only | The mount is read-only when the volume entry, the group request or the task mount says so. A dynamic entry is published read/write, so for it the request and the mount decide. Propagation mode is passed through unchanged. |
| Namespace | A node's host volume entries carry no namespace. The client offers a dynamic entry to an allocation only when the manager's record for its ID exists and is in the allocation's namespace. Another entry counts as missing. The server already places an allocation only on a volume of its namespace; this is a second check on the client. |
| Logging | A missing volume is logged with the sorted missing sources and their count. The set of host volumes, which holds paths, is no longer logged. |

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

## Create handshake (client) {#create-handshake}

The server-to-client exchange is internal to Wander and is not a
compatibility question. The public evidence covers only the create endpoint
and that a volume has a name, a plugin and capacity bounds. The rules below
are Wander policy, implemented in `client/host_volume_endpoint.go`,
`client/hostvolume/manager.go` and `nomad/client_host_volume_endpoint.go`.

### Wander policy

1. The client RPC method is `HostVolume.Create`. Like `HostVolume.Register`
   it is for servers only: it adds no HTTP route and performs no ACL check.
2. The request carries VolumeID, Namespace, Name, NodeID, NodePool, PluginID,
   both capacity bounds and Parameters. The response carries VolumeID,
   HostPath, CapacityBytes, Status, Reason and Message. CapacityBytes is what
   the plugin reported and is stored with the client record, so a repeat
   answers with the same value. Zero means unknown.
3. A repeat with the same volume ID, namespace, name, node and plugin returns
   the stored status and runs no plugin, writes nothing and publishes
   nothing, so a retry after a lost reply or a leadership change cannot
   provision twice. Node pool, capacities and parameters are not compared.
   The stored status can be `unavailable`; the client's own reconcile pass,
   not the repeat, repairs it.
4. A repeat that differs in namespace, name or plugin is rejected with
   `create_mismatch` and changes nothing. An ID that belongs to a registered
   volume is rejected with `invalid_input`, as before.
5. The record is validated before the plugin runs, so a request that could
   never be stored (for example an empty name) provisions nothing and returns
   `invalid_input`.
6. A classified rejection returns a nil RPC error with Reason set and an empty
   Status. Only transport or internal failures return an RPC error. A
   canceled wait is an error, not a rejection. One create is bounded by two
   minutes.
7. `ClientHostVolume.Create` and `ClientHostVolume.Register` on the server
   route a call to the node: directly when this server holds the node
   connection, otherwise by forwarding to the server that does, as the CSI
   client endpoint does. They require a server certificate and check no
   namespace capability, because the server HostVolume endpoints authorize
   before they call.

| Reason code | Cases |
| --- | --- |
| `create_mismatch` | a repeat differs in namespace, name or plugin |
| `invalid_input` | malformed ID, name, namespace, node or plugin ID |
| `node_mismatch` | well-formed NodeID of another node |
| `name_conflict` | static or dynamic name already used on this node |
| `plugin_unavailable` | the plugin is missing, unhealthy or unsupported |

## Delete and deregister handshake (client) {#delete-handshake}

This exchange is internal to Wander and is not a compatibility question, so
it has no public-interface row. The rules below are Wander policy,
implemented in `client/host_volume_endpoint.go`,
`client/hostvolume/manager.go` and `nomad/client_host_volume_endpoint.go`.

### Wander policy

1. The client RPC methods are `HostVolume.Delete` and `HostVolume.Deregister`.
   They are separate methods, not one method with a flag, so that a wrong
   value can never run a plugin against registered external storage. Each
   also checks the kind of the stored record on the client. Like Create they
   are for servers only: no HTTP route and no ACL check.
2. A request carries VolumeID, Namespace and NodeID and nothing else. The
   client never takes a plugin, a path or a kind from the server: it uses its
   own stored record. A reply carries VolumeID, Reason and Message.
3. VolumeID, Namespace and NodeID are validated before the per-volume lock is
   taken, so untrusted input is bounded before it becomes a lock table key.
   The node is never defaulted.
4. A volume this client does not hold answers success and runs nothing, so a
   repeat after a lost reply, a client restart or a leadership change is safe.
5. A record this client holds under another namespace or node is rejected with
   `delete_mismatch` and nothing runs.
6. `Delete` runs the delete operation of the record's plugin, then removes the
   record and unpublishes the volume. A plugin failure (for example
   `non_zero_exit`, `timeout` or `plugin_unavailable`) is a classified reply
   and keeps the record and the published volume, so the server can retry.
7. `Delete` of a registered record is refused with `volume_kind_conflict`.
   `Deregister` of a plugin-created record is refused with
   `volume_kind_conflict`. Neither changes anything.
8. `Deregister` removes the record and unpublishes the volume. It never runs
   a plugin and never inspects, changes or removes the directory.
9. If the record store could not be read at all when the client started, both
   methods return an error, not success, until the client restarts with a
   readable store. Otherwise the server would drop the record of a directory
   that still exists. While the failure persists, the server's delete retries
   keep failing with an internal error until the client restarts, and the
   leader's retry diagnostic is where an operator will see it. A single
   record that was unreadable or invalid at start is skipped and logged with
   its volume ID; it counts as not held, so its directory may stay on disk
   and nothing advertises it.
10. A classified rejection returns a nil RPC error with Reason set. Only
    transport or internal failures return an RPC error. A canceled wait is an
    error, not a rejection. One call is bounded by two minutes.
11. `ClientHostVolume.Delete` and `ClientHostVolume.Deregister` on the server
    route to the node exactly like Create and Register: directly or by
    forwarding, server certificate only.

| Reason code | Cases |
| --- | --- |
| `delete_mismatch` | the client holds the ID under another namespace or node |
| `invalid_input` | malformed volume ID or empty namespace |
| `node_mismatch` | NodeID of another node, or empty |
| `volume_kind_conflict` | delete of a registered volume, or deregister of a plugin-created one |
| `non_zero_exit`, `timeout`, `plugin_unavailable` | the plugin delete failed; the record is kept |
| `volume_deleted` | (create and register only) the ID was deleted or deregistered here; see [Late create after a delete](#late-create) |

## Read API (server and HTTP) {#read-api}

### Recorded evidence

Listing is `GET /v1/volumes?type=host` (DHV-B-001) and reading is
`GET /v1/volume/host/:volume_id` (DHV-B-002). Both need the namespace
capability `host-volume-read` (DHV-B-139). The create, register and write
capabilities imply it (DHV-B-137, DHV-B-138, DHV-B-140). The namespace
`read` shorthand includes it (DHV-B-144). The evidence does not give the list
fields, the filters or the server calls. The rules below are Wander policy,
implemented in `nomad/host_volume_endpoint.go` and
`command/agent/host_volume_endpoint.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| Server calls | `HostVolume.List` and `HostVolume.Get`, with rate metric label `host_volume`. Both forward like the other read calls. The ACL check runs on the server that answers, after forwarding. Both are blocking queries. |
| Capability | `host-volume-read` in the request namespace, checked with `ResolveACL`. `host-volume-delete` does not imply read, so a delete-only token cannot list or read. A token without the capability gets permission denied. |
| Wildcard namespace | A list with namespace `*` returns only volumes in namespaces where the token holds `host-volume-read`. The namespace filter runs before the filter expression and pagination, so neither results nor page tokens reveal other namespaces. A token that holds the capability only for namespaces that do not exist gets an empty list. A token without it anywhere gets permission denied. |
| List fields | ID, Name, Namespace, PluginID, NodeID, NodePool, CapacityBytes, State, CreateIndex, ModifyIndex, CreateTime and ModifyTime. The host path, parameters, constraints, attachments and requested capabilities are left out of the list. |
| Read | Returns the full record, host path included, since `host-volume-read` is the inspect capability. An unknown ID returns HTTP 404. A missing ID, a NUL byte in the ID, and namespace `*` are rejected with HTTP 400 and a fixed message. |
| Filters | `prefix` matches the volume ID. `node_id` and `node_pool` are exact matches. The filter expression, `per_page` and `next_token` follow the other list calls. A NUL byte in the prefix or namespace is rejected with HTTP 400. The list is in namespace and ID order. |
| Index | Taken from the dynamic host volume table only, so a claim write does not wake a blocked list or read. A table that has never been written reports index 1. |
| HTTP | The list returns an empty array, not null. `/v1/volume/host/` with no ID or an extra path segment returns 404. Methods other than GET return 405 until the write operations exist. `type=csi`, no type and an unknown type behave as before. |

## Raft commands and mixed versions (server) {#raft-commands}

### Recorded evidence

None. The public contract does not describe the server log. The state machine
is the one in the PRD, section 6.4. The mixed-version rule is an owner
decision of 2026-11-22. The tag and the minimum version are owner decisions of
2026-11-25. Everything below is Wander policy, implemented in
`nomad/state/state_store_dynamic_host_volume_transition.go`,
`nomad/dynamic_host_volume_raft.go` and `nomad/fsm.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| Commands | Six Raft message types, 100 to 105: upsert (create or register), outcome (the client's answer), update, mark deleting, remove and claim delete (see [Claim list and delete](#claim-api)). The numbers stay clear of the 1.6.5 types, of 127 and of the `IgnoreUnknownTypeFlag` bit. |
| Unknown type | The commands do not carry `IgnoreUnknownTypeFlag`. A server that does not know them stops in apply rather than skipping the entry and silently diverging from the leader. |
| Version tag | Every server gossips the Wander product version, for example `0.1.2-dev`, in the serf tag `wander_version`. The `build` tag stays on the Nomad compatibility version 1.6.5, so Nomad 1.6.5 peers keep working. |
| Gate | A command is submitted only when every server in the region that is alive or failed carries `wander_version` at 0.1.2 or later. Prerelease and metadata are ignored, so `0.1.2-dev` passes. A missing or unreadable tag blocks, so a Nomad 1.6.5 server or an older Wander server stops the commands. Servers that have left and servers of other regions are not counted. No servers at all blocks. The check is made when a command is submitted. A server that joins between the check and the commit is a window that is accepted, not closed. |
| Transitions | Checked inside the apply, so a replayed or duplicated log entry cannot make an illegal move. A refused transition returns an error naming the current state, or `none` for a missing record, and changes nothing. A batch is all or nothing. `ready` to `unavailable` and back stay allowed for the node-derived rule. |
| Create and register | Create starts in `requested` without a node or `pending` with one. Register starts in `pending` with its host path. Both reach `ready` or `failed` only through the outcome command; there is no shortcut from nothing to `ready`. Assigning the node moves `requested` to `pending`. The only other change allowed in that step is the node pool: a record stored with no pool or `all` takes the pool of the chosen node, and a named pool never changes. Nothing else may change. |
| Repeats | A repeated create or register with the same content changes nothing, even after the volume has moved on. Different content is refused. Marking a volume that is already deleting changes nothing, so its modify index still matches a later removal. A repeated outcome changes nothing. Removing a volume that is already gone changes nothing. |
| Outcome | `ready` is accepted from `pending` only. It needs an absolute host path without a NUL byte, and a capacity of zero or more, since a registered volume may not know its size. It clears the diagnostic. `failed` is accepted from `requested` or `pending` and needs a diagnostic. `deleting` is accepted from `deleting` only, needs a diagnostic, and changes only the status description: the state, the times and every other field stay, and the modify index moves with the write. Repeating the stored description changes nothing. It is how the leader stores why a removal is waiting (see [Leader retry of deleting volumes](#deleting-retry)). This changes the apply of the existing outcome type (102); that is acceptable only because 0.1.2 is not released, so no log written by an earlier apply exists. |
| Update | Only a `ready` volume, with compare-and-set on the modify index and the immutable-field rules. The host path, capacity, diagnostic and attachments belong to the server and the client: an update that differs in any of them is refused, naming the field. |
| Diagnostic | Stored as `StatusDescription`, cut to 1024 bytes at a character boundary, with invalid UTF-8 replaced. It travels in the existing snapshot record. Keeping host paths of other namespaces out of it is the job of the code that writes it. |
| Remove | Only from `deleting`. The sticky claims of the volume are deleted in the same apply, and a volume an allocation still uses is refused. |
| Node availability | A volume moves between `ready` and `unavailable` in the same Raft apply as the node write: a node register or fingerprint update, a node status update, and a node deregister or batch deregister. Nothing else moves between these two states, and no other path is hooked: drain, eligibility and node events change neither status nor advertised volumes. |
| Down | A node counts as down when its status is `down` or `disconnected`, or when the node record has been deleted. Counting `disconnected` as down is an owner decision of 2026-11-26. `initializing` and `ready` do not count as down, so a restarting agent whose fingerprint already lists a volume does not keep it unavailable until the status update arrives. |
| Matching | A volume is advertised when an entry of the node's host volumes carries the volume ID. The entry's name and map key are never compared, and a static volume, which has an empty ID, never counts, so it cannot keep a dynamic volume of the same name ready. |
| Which volumes move | Only `ready` becomes `unavailable` (node down or gone, or volume not advertised) and only `unavailable` becomes `ready` (node not down and volume advertised). Volumes in `requested`, `pending`, `deleting` and `failed`, and volumes on other nodes, are never changed. A deleted node leaves its volume records in place. |
| Restart flap | A restarted client registers as `initializing` before it has restored its volumes, so the first update may omit them. The volume then goes `ready`, `unavailable`, `ready` within seconds. This is deterministic and accepted: a grace period would need a clock in apply. Running allocations are not stopped, since the volume state is not consulted for them; only new placement is held back. |
| Fields and indexes | The derived change keeps `ModifyTime` and `StatusDescription` as they are, because apply reads no clock and a deregister carries no time. `ModifyIndex` is the apply index. The volume table index moves only when at least one record changes, and never backwards. |
| Apply-time re-check | The plan applier re-reads, from the snapshot it evaluates, the record of every dynamic volume a new placement requests, and refuses the placement unless the record is `ready`, belongs to the node of the placement and carries the requested name. `deleting`, `unavailable`, `pending`, `requested`, `failed`, a removed record and a record of another node are all refused. A record the node no longer advertises is refused too, found by namespace, node and name. Only that node's part of the plan is rejected, the other nodes keep theirs, and the evaluation is refreshed. The refusal reason is the bounded string `dynamic host volume not usable`, which never names a volume, a source or a path. A static volume of the same name, which has an empty ID, is not affected, and neither is a source with no record. Allocations already on the node, such as in-place updates, are not re-checked, so running work keeps running. A node that is down or disconnected is refused by the inherited node check (`node is not ready for placements`), not by the volume re-check. |
| Release | The derivation must ship in the same release as the 0.1.2 version gate on the commands. Until every server runs it, no volume record exists, so an older server computes the same state. If 0.1.2 is released without it, the minimum version of the gate must be raised to the release that contains it. |

### Node read API {#node-read-api}

Wander policy, from the owner decision that the node read API lists a dynamic
host volume only when the token may read host volumes in its namespace.

| Question | Wander policy |
| --- | --- |
| Rule | A dynamic host volume (an entry whose `ID` is set) appears in `Node.HostVolumes` and `NodeListStub.HostVolumes` only when a volume record with that ID exists on the node and the token holds `host-volume-read` (or a capability that implies it) in the namespace of that record. Static volumes, which have no ID, are listed as before. Namespace matching is the existing ACL matching, so wildcard and glob namespace policies apply. |
| Unattributed ID | An advertised ID with no record on that node is hidden from every token except management, because it cannot be attributed to a namespace. A record that exists only on another node does not make the entry visible. If records in two namespaces carry the same ID on the node, the entry is shown only when both are readable. |
| Bypass | Management tokens, a server with ACLs disabled, and client identity calls (a node secret, which resolves to no ACL) see every entry unchanged. Client identities are trusted between nodes as before. |
| Paths filtered | `Node.GetNode` (`/v1/node/:id`), `Node.List` with or without fields (`/v1/nodes`) and `NodePool.ListNodes` (`/v1/node/pool/:name/nodes`). The filter runs on the server that answers, after forwarding, and builds a new map: the stored node is never modified. |
| Blocking | The reply index stays the node index. The volume lookup adds no watch, so a change to a volume record alone does not wake a blocked node query. |
| Inherited code | The three endpoints above are inherited. Each gains one call to `nodeHostVolumesForRead` (`nomad/host_volume_node_read.go`); `Node.List` now keeps the resolved ACL instead of discarding it, and `Node.GetNode` copies the node before replacing the map. |
| Out of scope | Search and fuzzy search return node IDs and names only. Operator snapshot save is management only. `/v1/agent/self` returns only the local agent's static `host_volume` configuration. `Node.GetClientAllocs`, `Node.UpdateStatus` and `Node.Register` replies do not return the node. The scheduler, plan applier, Go API, CLI and UI only render what these endpoints return. |
| Node event stream | The `Node` event topic carries static host volumes only, for every subscriber, management included. The broker builds one payload for all subscribers and the topic needs only `node:read`, so it cannot follow the per-namespace rule above. Dynamic entries are removed from node registration, update and deregistration events in `nomad/state/events_dynamic_host_volume.go`; dynamic volumes are read through the node and volume APIs instead. Owner decision of 2026-11-26. Inherited `nomad/state/events.go` changes in the two places that build a node event. |

## Create and register (server) {#create-register-api}

### Recorded evidence

The public contract names the create and register operations and their
capabilities (DHV-B-137, DHV-B-138, DHV-B-140) and says that the server, not
the client, keeps a volume name unique on a node (DHV-B-099). It gives no
request shape, error codes or update rules. Everything below is Wander policy,
implemented in `nomad/host_volume_create.go`, with the request and reply types
in `nomad/structs/dynamic_host_volume_endpoint.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| Calls | `HostVolume.Create` and `HostVolume.Register`, rate metric label `host_volume` (write). Both forward to the leader first and check the capability on the server that applies the write, after forwarding. The request carries a volume and `PolicyOverride`; the reply carries the volume read back from the state and a write index. |
| Capability | Create needs `host-volume-create` and register needs `host-volume-register`, in the request namespace. `host-volume-register` implies create, and `host-volume-write` implies both, through the implied capabilities. Read and delete grant neither. A capability held only in another namespace grants nothing, and a deny overrides an allow. A refused call never reaches the client and writes nothing. |
| Namespace | The body namespace may be empty, which means the request namespace. A different value gives HTTP 400 `volume namespace does not match the request namespace` and writes nothing, so the capability check on the request namespace cannot be bypassed through the body. The wildcard namespace gives 400 `a volume write needs a single namespace`. A namespace that does not exist gives 400 `namespace not found`. |
| New volume | A volume without an ID is validated, given a generated UUID, the type `host` and the origin (`plugin-created` for create, `registered` for register), and stamped with the server clock. The request cannot set the state, host path of a create, capacity, diagnostic, attachments, indexes or times. Validation errors are 400 and name fields and codes only (`invalid volume: PluginID: required`), never values. |
| Placement | A create selects the node with the node selection rules. A named node that does not exist gives 400 `node_not_found`, and one outside the requested pool 400 `node_pool_mismatch`; neither writes anything. The record stores the node's own pool. |
| No eligible node | The volume is written as `requested` and then `failed`, and the reply is a normal one carrying the failed record. The description is `no_eligible_node` followed by `: ` and comma-separated `reason=count` pairs in reason order, for example `no_eligible_node: node_not_ready=2,plugin_unhealthy=1`. It holds codes and counts only, never node IDs, paths, parameters or constraint text. A name that every candidate node already uses is not a placement failure: it gives 400 `name_conflict` and writes nothing. |
| Register node checks | The node must exist (400 `node_not_found`), be in the requested pool when one other than `all` is given (400 `node_pool_mismatch`), be ready (400 `node_not_ready`), eligible (400 `node_ineligible`) and not draining (400 `node_draining`), and the name must be unused on that node in any namespace and any state, as a record or as a static host volume (400 `name_conflict`). Register names its node, so nothing is selected, and a failed node check never records a failed volume. |
| Same name | One server mutex covers the node check or selection and the first write, so two requests for one name on one node cannot both pass the check; the state has no unique index on node and name. The loser gets 400 `name_conflict` and writes nothing. The mutex is released before the client call. |
| Client call | After the pending record is written the server calls `ClientHostVolume.Create` or `ClientHostVolume.Register` with no lock, snapshot or write held. The volume is marked in flight, keyed by namespace and ID, until the request returns on every path. A second request for that volume gets 409 `operation in progress`; it does not wait. |
| Client outcome | A reply with a status of `ready` or `unavailable` is recorded as `ready` with the reported path and capacity; `unavailable` is a stored state derived from the node, not a rejection. A create records the reported capacity. A register records capacity 0, because the reply carries none, and the host path that was requested. A reply with a reason is recorded as `failed` with the description `<reason>: <message>`, or the reason alone when the message is empty. Nothing else is stored. The directory of a register is never modified by the server. |
| Failed reply | A recorded failure is a normal reply: the failed volume with its description and the write index, with no RPC error, because an error would drop the reply. The HTTP layer returns it as HTTP 200 with the failed volume; see [HTTP create, register and update](#http-write-api). |
| Transport error | An RPC error from the client call, a reply with neither reason nor status, or an outcome the state refuses is not a rejection: the client may have finished the work. The record stays `pending` (a later retry decides) and the call returns a coded error naming the state: 502 `client call failed, volume state is "pending"` or 500 `outcome not recorded, volume state is "pending"`. The error text of the transport failure is never stored or returned. If another writer has moved the record, for example to `deleting`, the error names that state with 409. |
| Read back | After every write the record is read from a new snapshot and compared with the expected state. A volume found in another state, such as `deleting` after a concurrent delete, gives 409 `volume state is "<state>"`. A volume moved from `ready` to `unavailable` by a node update is accepted. |
| Reply index | The index of the last Raft write made for the request, or the stored modify index of the volume for an identical update. |
| Update by ID | A volume with an ID updates the stored volume of that namespace. An unknown ID gives 404 `volume not found`. A stored volume that is not `ready` gives 409 `volume state is "<state>"`, so a repeated create or register on `failed`, `deleting`, `requested`, `pending` or `unavailable` is an error naming the stored state, never success. |
| Update fields | The candidate is the stored record with only the requested capacity bounds, capabilities, constraints and parameters taken from the request; the server-owned fields, node, pool, path, capacity, state, origin, indexes and creation time stay. The modify index of the stored record is the compare-and-set value. A conflict gives 409 `volume changed concurrently, retry` and is not retried. The update is recorded in Raft only and is not sent to the plugin, so it needs `host-volume-create` through create and `host-volume-register` through register. |
| Immutable fields | Name, type, node ID, node pool, plugin ID and, on register, host path. An omitted type, node ID, host path or node pool counts as unchanged, and a node pool of `all` is unchanged. A different value gives 400 `immutable field <Name>` and writes nothing. The plugin ID must equal the stored one because the plugin is not called and a delete runs the stored plugin. The node pool cannot move because it must stay the pool of the node. |
| Origin | Create by the ID of a registered volume, or register by the ID of a plugin-created one, gives 400 `volume origin mismatch` and writes nothing. |
| Identical update | An update whose candidate equals the stored record apart from the modify time, treating an empty map or list as absent, writes nothing: the reply is the stored record with its modify index, and the table index does not move. |
| Version gate | If a server of the region is below 0.1.2, the first write is refused before any client call with 503 `dynamic host volumes need every server to run Wander 0.1.2 or later`, and no record is left. A refusal of the outcome write after the first write has succeeded leaves the record in the state that first write gave it and is reported like any other outcome the state refuses, not as 503: with an eligible node the record stays `pending` and the call returns 500 `outcome not recorded, volume state is "pending"`; with no eligible node the failed outcome is not written, the record stays `requested` and the call returns 500 `outcome not recorded, volume state is "requested"`. |
| Lost leadership | A leader that loses leadership after the pending write leaves the record `pending`. It is not rolled back; the retry after a leader change owns it. |
| Policy override | `PolicyOverride` is accepted on the request and ignored. Wander has no Sentinel or policy engine, so there is nothing to override. |
| Coded errors | Every refusal that crosses the forwarding hop is a coded error with a fixed text (`NewErrRPCCoded`), because a sentinel error cannot be matched on the forwarding side. A permission failure is the usual permission denied. |
| Leader retry | The leader scans the whole volume table and drives every record left `requested` or `pending` (after a leader change or a lost client reply) to `ready` or `failed`. The table has no index on the state, so a full scan at a modest interval is used. The scan interval is 10 seconds. The loop runs a first pass at once when leadership is established. |
| Retry of pending | A `pending` record is sent to the client again by its origin with the stored request: the same volume ID, name, node, plugin, capacity bounds and parameters (create), or the stored host path (register). The retry is safe because the client's create answers a repeat from its stored record (see the client create handshake), so the plugin runs at most once for a volume. The server tests show that the repeated request is identical and that the outcome is recorded once; the plugin-runs-once property rests on the client tests of that handshake. |
| Retry of requested | A `requested` record gets node selection again, under the placement lock and from a new snapshot. If a node is eligible, the record moves to `pending` (compare-and-set on its modify index, with the node's pool) and is handled as pending. If none is, it is recorded `failed` with the same `no_eligible_node` description as a first request. A name conflict as the only reason, or a node or pool rejection, gives the fixed code `name_conflict`, `node_not_found` or `node_pool_mismatch`, because the record already exists and no 400 can be returned. A `requested` record of a registered volume cannot arise; if one appears it is recorded `failed` with `invalid_requested_origin`. The explicit node of a first request is not kept on a `requested` record, so the retry selects among all nodes of the stored pool. |
| In-flight rule | The reconciler marks a volume in flight through the same coordinator as the RPCs before it reads the record, and skips a volume that is already marked. An RPC refuses a volume the reconciler holds with 409 `operation in progress`. After the mark the record is read again from a new snapshot, and the reconciler acts only if it is still `requested`, `pending` or `deleting` with the modify index it scanned, so a volume an RPC settled between the scan and the mark is not called twice. The in-flight set is per server: an RPC still running on a deposed leader is invisible to the new leader, and safety then rests on the repeat-safe client create and on the state refusing a second, different outcome. |
| Client call deadline | Each client call has a deadline of 60 seconds, so one node cannot hold up the loop. The call runs in its own goroutine. A call that passes its deadline may still act on the node, so the loop moves on but the volume stays in flight and is neither retried nor failed while the call runs, even past the bound. When the call returns, its answer is recorded as if it had come in time: `ready` on success, `failed` with the client's reason on a rejection, `failed` with `node_not_found` for a node that is gone, and the transient handling below for a transient error. A call still outstanding at stop, or one that has returned but is not yet recorded, is abandoned: no outcome is written for it, and the volume stays in flight until the call returns. |
| Backoff and bound | A transient error is a missing node connection, a transport error, a timeout error returned by the call itself (the deadline of the loop is not one; see the client call deadline), a reply with neither a reason nor a usable status, or an outcome the state finds invalid. Each volume is retried with an exponential delay that starts at 1 second, doubles to a cap of 60 seconds and carries jitter. The delays are kept in memory on the leader and start again after a leadership change. The bound is 10 minutes, measured from the modify time of the record, which survives a leader change, or from the last refusal of the version gate when that is later (see the version gate wait). After the bound the volume is recorded `failed` with the fixed code `retry_limit_reached`, never with the error text. At least one attempt is made on the current leader before the age counts, so a cluster that was down for longer than the bound does not fail volumes it never retried. |
| Down node | A node that is down or disconnected has no connection, so it is a transient error and the volume waits until the bound. This differs from `ready` to `unavailable`, which follows the node at once. A node that is no longer in the state store (garbage collected) cannot come back, and the volume is recorded `failed` with `node_not_found` at once. |
| Version gate wait | A refusal by the version gate on a Raft write of the reconciler (the pending write or an outcome, including a `failed` outcome) makes it wait with the usual backoff and try again; a refusal never fails the volume. The leader keeps, per volume in memory, the time of the last refusal. The bound is measured from the later of the record's modify time and that time, so time spent refused by the gate does not count toward it: after the gate lifts, a volume fails only once a transient error comes more than the bound after the last refusal. The time is lost on a leadership change, where the modify time alone applies again, and at least one attempt is still made on the new leader before a volume fails. |
| Refused outcome | An outcome the state refuses because the record moved on (a refused transition, a compare-and-set conflict or an identity conflict), for example to `deleting` during the call, is logged and dropped. The next scan reads the record again, and a record that is no longer `requested`, `pending` or `deleting` is not picked up. |
| Logging | The reconciler logs the namespace, the ID, the state and a code only, never a host path, a parameter or the text of a client error. |
| Clock | Only the reconciler reads the clock: the modify time goes into the command and the age comes from the record. The apply reads no clock. |
| Inherited code | `nomad/server.go` gains one field, `dhvCoordinator`, and its construction in `NewServer`. `nomad/leader.go` gains two calls: `establishLeadership` starts the reconciler and `revokeLeadership` stops it and waits for it to exit. Start stops a running instance first, because the reassert path of the leader loop runs revoke and then establish with the same stop channel. No other inherited code changes. |

## HTTP create, register and update {#http-write-api}

### Recorded evidence

The public API documentation (SRC-api-bodies, read 2026-11-26, interface
evidence only; the page shows no last-updated date) gives these facts. Create
is `PUT /v1/volume/host/create` and register is `PUT /v1/volume/host/register`;
neither is a blocking query. The body has a top-level `Volume` (required) and
`PolicyOverride` (boolean, default false, optional). The `Volume` carries its
`ID` when it updates an existing volume, and the same request without an ID
creates a new one; there is no separate update path. The reply is
`{"Volume": <volume>, "Warnings": null}`. The create sample shows the state
`pending` and the register sample `ready`. The page gives no status codes, no
error bodies and no warning text, so every status code and text below is
Wander policy. The matrix entries are DHV-B-003, DHV-B-004, DHV-B-005 and
DHV-B-042. Implemented in `command/agent/host_volume_endpoint.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| Routes | `create` and `register` are routed before the read by ID. Volume IDs are UUIDs, so reserving the two words costs nothing. PUT and POST are accepted; every other method on these paths, GET included, is 405. `GET /v1/volume/host/:id` and 405 for other methods on an ID are unchanged, and an extra path segment is 404. |
| Server calls | `HostVolume.Create` and `HostVolume.Register`. A volume with an ID updates the stored volume with the rules of the RPC (see [Create and register](#create-register-api)); the handler adds none. |
| Body decoding | The body is decoded into a type of the agent package holding `Volume` and `PolicyOverride`, not into the RPC request. A top-level `Namespace`, `Region`, `AuthToken` or `IdempotencyToken` key therefore sets nothing; in particular a token in the body is not a credential. Key matching is case-insensitive, as for every JSON body of the API. The body is read up to 1 MiB and must be one JSON object: data after it is refused, as in the plugin output rule. A top-level key may appear once, compared under the same case folding as field matching (`Volume` and `volume` are the same key): the decoder merges repeated occurrences into one value while the unknown-field check sees only the last, so an unknown key in an earlier occurrence would go unnamed. Keys inside the volume are not checked for repeats. The shared `decodeBody` is inherited and unchanged. |
| Fixed 400 texts | `invalid request body` for an empty body, a body over 1 MiB, malformed JSON, data after the object, a body that is not an object, a repeated top-level key, or a field of the wrong type; `missing volume` for no volume or a null volume. The decoder text is never returned, because it carries offsets and input characters. Both are checked before any RPC call and write nothing. |
| Namespace | The request namespace is `?namespace`, else `Volume.Namespace`, else `default`, mirroring the inherited `namespaceForJob`. The ACL check runs on that namespace. When both are set and differ, the RPC answers 400 `volume namespace does not match the request namespace`. Without this the documented sample, which has a body namespace and no query, would fail in any namespace but `default`. The region follows the usual `?region`. |
| Unknown fields | Chosen option: they are accepted, ignored and named in Warnings, so they are not silently taken as implemented. Top-level keys appear as `Name` and keys of the volume as `Volume.Name`; nested objects (constraints, requested capabilities) are not checked. The names are sorted, without duplicates, at most 14 are shown then `and N more`, each at most 64 characters then `...`, with unprintable characters replaced by `?`. A field of the volume that the model has is never unknown, whether or not the server ignores it (for example `State`). Strict refusal was rejected because clients that send fields of other releases would break. |
| PolicyOverride | Passed to the RPC, which ignores it. The reply Warnings says `PolicyOverride has no effect: Wander has no policy engine`. The documentation lists `sentinel-override` as required with it; Wander does not require it, because there is no policy engine to override and a capability for a no-op would only refuse valid requests. This is a divergence from the documentation. |
| Warnings | A string or null, never an empty string. Several messages are joined by a newline, the PolicyOverride message first. A warning never changes a status or a stored record. |
| Reply | `{"Volume": ..., "Warnings": ...}` with HTTP 200. The volume has the same field names as the read API. `X-Nomad-Index` is the write index of the reply; no query metadata is sent because the reply carries none. |
| Failed create or register | A volume the RPC recorded as `failed` (no eligible node, or a refusal by the client) is a normal reply: HTTP 200 with the failed volume and its `StatusDescription`. A successful create is `ready`, not `pending` as in the documented sample, because the RPC replies after the client outcome. |
| Coded errors | A coded RPC error (400, 403, 404, 409, 500, 502, 503) keeps its status and fixed text, also when forwarded to another server or region, because the code is carried in the error text and `wrap` reads it from the start of the text. A 502 or 500 after the first write leaves a pending record, so those two do not mean no state change; the decode and validation 400s do. |
| Capability | As the RPC: create needs `host-volume-create`, register `host-volume-register`, `host-volume-write` implies both, and an update needs the capability of the endpoint it goes through. Read or delete alone, or no token, is 403. |
| Inherited code | none changed. `command/agent/http.go` is untouched: the existing `/v1/volume/host/` prefix route reaches the handler. |

## Claim list and delete (server) {#claim-api}

### Recorded evidence

The public API documentation (SRC-api-claims, read 2026-11-26, interface
evidence only; the page shows no last-updated date) gives these facts. List
claims is `GET /v1/volumes/claims`, a blocking query needing
`namespace:host-volume-read`, with the query keys `claim_id` (a prefix),
`namespace`, `job_id`, `task_group`, `volume_name`, `next_token` and `per_page`;
the reply is an array of objects with `AllocID`, `CreateIndex`, `ID`, `JobID`,
`ModifyIndex`, `Namespace`, `TaskGroupName`, `VolumeID` and `VolumeName`.
Delete claim is `DELETE /v1/volumes/claim/:claim_id` (the page's sample shows
`/v1/volume/claim/:id`), not a blocking query, needing
`namespace:host-volume-write`, with an empty response body. No claim read
endpoint and no status code is documented. The matrix entries are DHV-B-147,
DHV-B-148 and DHV-B-156. The plural and singular path conflict is a matter for
the HTTP task. Implemented in `nomad/host_volume_claim.go` (`HostVolume.ListClaims`
and `HostVolume.DeleteClaim`), `nomad/fsm.go` and
`nomad/state/state_store_dynamic_host_volume.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| List ACL | `host-volume-read` in the request namespace, which `host-volume-create`, `-register` and `-write` imply, as for the volume list. A wildcard namespace returns only claims in namespaces where the token holds it, across pages; a token with it in no existing namespace gets an empty list, and a token with it nowhere is denied. The namespace filter runs before every other filter and before the page token is built. |
| List filters | The claim ID prefix, `job_id`, `task_group` and `volume_name`. The prefix is matched by the ID index in a single namespace and in memory for a wildcard, because the state helpers treat `*` as a literal namespace. The other three are exact matches. |
| List paging | `per_page` and `next_token` on namespace and claim ID. The ID index is always the one iterated, so the order does not depend on the filters and a token stays valid when a filter changes. |
| List blocking | The query blocks on the index of the claims table, floored at 1. The volume list reports the volumes table only, and reusing it would leave a claim list asleep through a claim delete. |
| List fields | The RPC returns the stored claim, including `AllocationIndex`, which the public reply lacks. The stored field keeps the name `AllocationID`; the public name is `AllocID`. No codec tag is added and no rename is made. Whether and how the HTTP reply maps it belongs to the HTTP task. |
| Delete ACL | `host-volume-write` in the claim's namespace; no other capability implies it, so read alone is denied. The permission check comes before the lookup. |
| Delete namespace | A wildcard namespace is 400 `a claim delete needs a single namespace`. An empty ID is 400 `missing claim ID`, and an ID with a NUL byte is 400 `invalid claim ID`. |
| Unknown claim | 404 `claim not found`. The observed binary answered 500 with its own text for a missing claim; Wander uses 404, which the HTTP task can map directly. The permission check comes first, so the answer reveals nothing across namespaces. A repeated delete is therefore 404, not success. |
| Claim in use | 409 `claim in use`, fixed text. A claim is held while any allocation of the claim's namespace, job and task group that is not client-terminal uses the claim's volume on the volume's node, by the rule of `DynamicHostVolumeAllocRequests` and the release point of the volume delete (an allocation counts until the client reports it complete, failed or lost). `AllocationID` records only the allocation that created the claim and is not updated when a replacement or rescheduled allocation reuses the claim, so checking it alone would let a claim in use be deleted. A claim whose volume record is missing, which removing a volume prevents, falls back to the allocation recorded in the claim. |
| Where the rule runs | In the state store apply, which returns the in-use error and leaves the claim and the table index unchanged, so a replayed or racing entry cannot remove a held claim. The RPC repeats the check before submitting, only to refuse without writing a log entry; both answers are the same 409. |
| Concurrent change | The entry carries the modify index read with the claim as compare-and-set value, chosen before the command is submitted. A claim rewritten or removed in between is 409 `volume changed concurrently, retry`, never 500. |
| Raft command | Message type 105, `DynamicHostVolumeClaimDeleteRequestType`, submitted only through the version gate, so a region with a server older than 0.1.2 gets 503 with the gate text and nothing written. It carries no `IgnoreUnknownTypeFlag`, for the reason given under Raft commands. One claim per entry, and an entry is never batched with other claim writes, because a write must be above the claims table index. |
| Determinism | Applying the same entry to two fresh state machines gives identical claim tables and table index; the apply reads no clock and no random source. |
| Snapshot | No change. Snapshot types 29 and 30 already persist and restore the claims table, and a delete only removes rows. No snapshot type is added. |
| Forwarding | Coded errors keep their code and fixed text when the call is forwarded to the leader or another region. |
| Inherited code | Two inherited files change by one line each: `helper/raftutil/msgtypes.go` lists type 105 by name, and `nomad/fsm.go` registers it in the apply switch. No other inherited code changes. |

## Delete and deregister (server) {#delete-deregister-api}

### Recorded evidence

The public contract documents deleting a dynamic host volume: the capability
is `host-volume-write` or `host-volume-delete`, and deleting a volume that is
in use is an error (SRC-api). It documents no force, no deregister call, no
error codes and no body. A black-box observation shows that deleting a volume
whose node is down or garbage collected is refused without force
(SRC-observe-node-down). Everything below that goes beyond those facts is
Wander policy, implemented in `nomad/host_volume_delete.go`, with the request
types in `nomad/structs/dynamic_host_volume_endpoint.go`.

### Wander policy

| Question | Wander policy |
| --- | --- |
| Calls | `HostVolume.Delete` for a plugin-created volume and `HostVolume.Deregister` for a registered one, rate metric label `host_volume` (write). Both take `VolumeID`, `Force` and the request namespace, forward to the leader first, and reply with the write index of the removal. Delete marks the volume deleting, calls `ClientHostVolume.Delete` with no lock held other than the in-flight mark, and removes the record. Deregister does the same with `ClientHostVolume.Deregister` and never runs a plugin. |
| Deregister capability | The public contract has no deregister capability, so deregister needs `host-volume-delete`, as delete does; `host-volume-write` implies it. Create, register and read grant neither. A capability held only in another namespace grants nothing, a deny overrides an allow, and the wildcard namespace is refused with 400 `a volume write needs a single namespace`. A refused call never reaches the client and writes nothing. |
| Force | Force only skips the call to the node, and only when the node is down, disconnected or not in the state store: the record is removed with no client call. Without force, such a node gives 400 `node is <status>, force is needed to remove the volume` before any write. Force on a node in any other status gives 400 `force is only for a down, disconnected or unknown node, node status is "<status>"`, with the status taken from a fixed list. Force works for delete and deregister. Force never skips the in-use check. |
| Cross-kind calls | Delete of a registered volume is refused with 400 `volume was registered, use deregister`, and deregister of a plugin-created volume with 400 `volume was created by a plugin, use delete`. A volume with an unknown origin is refused with 400 `volume origin is unknown` by both. The check is made on the stored origin, before any write, and the client is never called. |
| Volume in use | A volume that a non-terminal allocation uses is refused with 409 `volume in use`, with and without force, before anything is written. The allocations of a disconnected node are unknown, not terminal, so force on such a node usually still gives 409; the allocations of a down node are lost, which is terminal. The mark that follows makes the same check inside the Raft apply, because the plan applier can commit an allocation between the first check and the mark, and the client deletes the directory after the mark. That apply refuses with the same 409, writes nothing and never calls the client. A volume that is already deleting is not checked again by the mark, so its removal refuses instead. |
| No node | A requested or failed volume with no node has nothing on disk. It is removed with no client call and without force. A failed volume that has a node gets the normal client call, which answers success for a volume it no longer holds. |
| Repeat of a deleting volume | A volume that is already deleting is accepted again: the mark is a no-op and the removal runs the client call again. This is how a retry finishes after a rejection, a transport failure or a lost reply. |
| Unknown volume | 404 `volume not found`, also for a repeat after the removal and a volume in another namespace. An empty ID is 400 `missing volume ID`. |
| Concurrency | The volume is marked in flight for the whole call. A second delete or deregister, an update, or the leader's retry for the same volume gets 409 `operation in progress`. One of two concurrent deletes succeeds, and a later one is 404. |
| Client rejection | A rejection by the client leaves the volume deleting and returns 502 `client rejected: <code>, volume state is "deleting"`. The code is the client reason when it is on a fixed list (`delete_mismatch`, `volume_kind_conflict`, the plugin reasons) and `client_rejected` otherwise. The client message is never returned: it can carry host paths. Nothing else is written. |
| Transport failure | A client call that fails in transit returns 502 `client call failed, volume state is "deleting"`. The text of the failure is never returned. |
| Version gate | A gate refusal of the mark is 503 `dynamic host volumes need every server to run Wander 0.1.2 or later` and writes nothing. A gate that refuses the removal after the mark leaves the record deleting and returns 503 with that text followed by `, volume state is "deleting"`. A repeat once the gate opens finishes the removal. Any other failure to record the removal is 500 `outcome not recorded, volume state is "deleting"`. |
| Inherited code | none changed. `MarkDynamicHostVolumesDeleting` in `nomad/state/state_store_dynamic_host_volume_transition.go` is Wander code from the Raft command work; it now refuses a volume in use, and `StateStore.DynamicHostVolumeInUse` is a new exported read over the same check. Existing tests changed for that rule: `TestStateStore_DynamicHostVolumeRemoveRefusedWhileInUse` marks the volume before the allocation arrives, and `TestPlanApply_EvalNodePlan_DynamicHostVolumeHolderKept` in `nomad/plan_apply_test.go` degrades the volume before the holder allocation is upserted. The changed apply result of type 103 is safe only because nothing ran 103 before this endpoint and the version gate needs Wander 0.1.2, which is unreleased; a build that had applied 103 with the old rule would diverge on replay or in a mixed cluster. |

## Leader retry of deleting volumes (server) {#deleting-retry}

### Wander policy

The retry is Wander policy and has no public-interface row. It is implemented
in `nomad/host_volume_reconcile.go` beside the retry of requested and pending
volumes; the client call is shared with the delete RPC through `removalCall` in
`nomad/host_volume_delete.go`.

| Question | Wander policy |
| --- | --- |
| Scope | A volume left `deleting`, whether a delete or deregister RPC failed after the mark, the leader changed, or a record was seeded that way, is driven by the leader's reconciler: a plugin-created volume through `ClientHostVolume.Delete`, a registered one through `ClientHostVolume.Deregister`, and then the removal command (type 104). A volume with no node has nothing on a client and is removed with no call, as the RPC does. The retry of requested and pending volumes is unchanged. |
| Forever retry | A deleting volume is never recorded `failed`, however long the call fails: the ten minute bound of requested and pending volumes does not apply. The delay starts at 1 second, doubles to a cap of 60 seconds and carries jitter, the same backoff as the other retries, kept in memory and started again after a leadership change. Owner decision of 2026-11-26. |
| Stored codes | The reason a removal is waiting is stored as the status description, with the outcome command, and is always a fixed code, never a client message, a path or an address: `client_unreachable` (a transport failure, no connection, or a client that could not read its records), `node_not_found` (the node is not in the state store), `volume_in_use`, or the client's rejection reason when it is on the fixed list of the delete RPC and `client_rejected` otherwise. The code is written only when it differs from the stored one, so a volume that keeps failing the same way costs no Raft write. A write does not reset the backoff, so the delay reaches its cap. A refused write (version gate, or the record moved on) never fails the volume. |
| Unknown node | A node that is no longer in the state store is retried too, with `node_not_found`, because the delete RPC with force removes such a record. This differs from a requested or pending volume, which is failed at once. |
| Down node | The status of the node is not checked. The call fails as `client_unreachable` and is retried forever. Force through the delete or deregister RPC, which skips the client call for a down, disconnected or unknown node, is the operator's way out. |
| In-use check | Before the client call the reconciler checks, on a new snapshot, that no live allocation uses the volume. A volume in use is stored as `volume_in_use`, no call is made, and it is checked again at the next attempt. Without the check, the plugin delete would wipe the directory and the removal command would refuse the record afterwards. The removal command checks again inside the apply, and a refusal there is stored as `volume_in_use` too. |
| In-flight rule | As for the other states: the volume is marked in flight first, an RPC that holds it is skipped, and the record is read again after the mark. A call past its deadline keeps the volume in flight until it returns, and its answer is then recorded. A call outstanding at stop is abandoned and writes nothing. |
| Logging | Namespace and ID only. |
| Orphan detection | Not added. A client that holds a volume the server does not know would need a scan of every node against every volume on each pass, with state to avoid repeating a warning, and a log line on every server from the node-update path. The tombstone below prevents the orphan that matters. |
| Inherited code | none changed. |

## Late create after a delete (client) {#late-create}

### Wander policy

A create that an old leader sent before a delete can reach the client after
the new leader's delete answered success for a volume the client did not hold.
Without a mark the late create would provision a volume no server record
describes. The rules are Wander policy, implemented in
`client/hostvolume/manager.go`, `client/hostvolume/state/state.go` and the
client state stores.

| Question | Wander policy |
| --- | --- |
| Tombstone | `Delete` and `Deregister` store a tombstone (volume ID and time) before the plugin delete or the removal of the record, and also when the volume is not held. The write is under the per-volume lock. A write that fails fails the call with an error, so the server keeps the volume deleting and retries; the client never answers success without a tombstone. A repeat stores it again with a new time. A mismatch, a delete of a registered volume and a deregister of a plugin-created volume store none. A plugin delete that then fails keeps the tombstone. |
| Refusal | `Create` and `AddRegistered` refuse an ID with a tombstone with the reason `volume_deleted`, and a fixed message, after taking the per-volume lock and before the check for a held volume, the name check, the plugin lookup or any plugin run. Nothing is written or published. A volume the client still holds under a tombstone is refused too. On the server the refusal arrives as a failed outcome that the state refuses (the volume is `deleting` or gone), so it is dropped and the volume is not marked failed. |
| Storage | A separate bucket (`hostvolumetombstones`), so reading the records never returns a tombstone. The refusal therefore survives an agent restart. A store that has no such bucket reads as empty. |
| Retention | A tombstone is kept for 24 hours (`DefaultTombstoneRetention`, a field of the manager configuration). The server's call deadline is one minute and an abandoned call ends with the client operation timeouts, so a day is far longer than an old leader's request can live. Expired tombstones are removed when the records are loaded and on each periodic pass, using the manager's clock. |
| Failed load | A store whose records or tombstones could not be read at start refuses `Create` and `AddRegistered` with the error that `Delete` already returns, because a tombstone may be among what was not read. A tombstone that cannot be decoded or is invalid still refuses its own volume ID, and is kept until a later delete of that ID writes a good one. |
| Reason code | `volume_deleted` is a reason of a create or register only; it is not on the delete reply list. |
| Inherited code | Changed: `client/state/interface.go` gains `PutHostVolumeTombstone`, `GetHostVolumeTombstones` and `DeleteHostVolumeTombstone`, implemented in `db_bolt.go` (new bucket, existing buckets unchanged), `db_mem.go`, `db_noop.go` and `db_error.go`. |

## HTTP delete, deregister and claims {#http-delete-claims-api}

### Recorded evidence

The public API documentation (SRC-api and SRC-api-claims, read 2026-11-26,
interface evidence only) gives these facts. Delete is
`DELETE /v1/volume/host/:volume_id/delete`, not a blocking query, needing
`namespace:host-volume-write` or `namespace:host-volume-delete`; the ID must
be the full ID; it is an error to delete a volume in use. The page's sample
omits `/delete`. It shows no query parameter, no force, no body and no status
code. Deregister is not documented. List claims is `GET /v1/volumes/claims`,
blocking, with the query keys and reply fields recorded under
[Claim list and delete](#claim-api). Delete claim is
`DELETE /v1/volumes/claim/:claim_id` with an empty body; the sample shows
`/v1/volume/claim/:id`. No claim read is documented. An observation
(SRC-observe-sticky) found that the official binary answered 404 for the
singular claim path and 200 with an empty body for the plural one. The matrix
entries are DHV-B-006, DHV-B-008, DHV-B-009, DHV-B-010, DHV-B-022, DHV-B-147
and DHV-B-148. Implemented in `command/agent/host_volume_endpoint.go`. Every
route, parameter and status below that is not in this paragraph is Wander
policy.

### Wander policy

| Question | Decision | Basis |
| --- | --- | --- |
| Delete route | `DELETE /v1/volume/host/:id/delete` calls `HostVolume.Delete`. | Public documentation (operation table). |
| Deregister route | `DELETE /v1/volume/host/:id/deregister` calls `HostVolume.Deregister`. | Wander policy; no public deregister call. |
| Bare ID | `DELETE /v1/volume/host/:id` is 405, as are other methods on `/delete` and `/deregister`. The sample path conflicts with the operation table, and Wander serves the table. | Wander policy. |
| Other paths | A second segment other than `delete` and `deregister` is 404, and so is a third segment. `create` and `register` with `/delete` go to the RPC as IDs and give 404 `volume not found`. | Wander policy. |
| Full ID | The RPC looks the ID up exactly, so a prefix is 404 `volume not found`. No UUID check is added: seeded test IDs are not UUIDs. | Public documentation (full ID); the 404 is Wander policy. |
| Force | The `force` query parameter is optional; absent is false. Exactly one value of `true` or `false` is accepted. An empty value, `1`, `t`, `TRUE`, `yes` or a repeated parameter is 400 `invalid force value`, a fixed text that does not echo the value. The lenient `strconv.ParseBool` is not used for a flag that skips a client call. | Wander policy; no public force. |
| Body and namespace | The request body is never read. `?namespace`, `?region` and the token follow the usual write request parsing; a wildcard namespace is refused by the RPC with 400. | Wander policy. |
| Reply | 200 with an empty body and `X-Nomad-Index` set to the index of the removal. | Wander policy; the documentation shows no response. |
| Coded errors | Every coded RPC error passes through unchanged with its fixed text: 400, 403, 404, 409, 500, 502 and 503 for delete and deregister, and 400, 403, 404, 409 and 503 for claim delete. The table test through the mux covers every status except 500 (`outcome not recorded`), which is passed through the same way but is not tested at HTTP level. The handler maps nothing and adds no text of its own except the force text above. No response carries a host path, the request body or a request value. | Wander policy. |
| List route | `GET /v1/volumes/claims`; other methods are 405 and `/v1/volumes/claims/` is 404. It is a blocking query: `index` and `wait` work, and `X-Nomad-Index`, `X-Nomad-KnownLeader`, `X-Nomad-LastContact` and `X-Nomad-NextToken` are set. | Public documentation (route, blocking); headers are Wander policy. |
| List parameters | `namespace` (a wildcard is allowed), `job_id`, `task_group`, `volume_name`, `per_page` and `next_token`. | Public documentation. |
| claim_id and prefix | `claim_id` is the claim ID prefix and is applied after the generic query parsing, which fills the prefix from `prefix`. Both together with different values are 400 `claim_id and prefix differ`; equal values are accepted. | `claim_id` is public documentation; the rule for both is Wander policy. |
| Filter | `filter` is 400 `filter is not supported for claims`: the expression would run against the stored field names (`AllocationID`), not the public ones. | Wander policy. |
| List reply | An array of exactly `AllocID`, `CreateIndex`, `ID`, `JobID`, `ModifyIndex`, `Namespace`, `TaskGroupName`, `VolumeID` and `VolumeName`. `AllocID` is the stored `AllocationID`; `AllocationIndex` is left out. The mapping is a type of the agent package; `nomad/structs` is unchanged. An empty list is `[]`, never null. | Fields are public documentation; the mapping is Wander policy. |
| Claim delete route | `DELETE /v1/volumes/claim/:claim_id`, one non-empty segment, else 404; other methods are 405 (no claim read is documented). The reply is 200 with an empty body and `X-Nomad-Index`. | Public documentation (route, empty body); methods are Wander policy. |
| Sample path | Only the plural path is served. `/v1/volume/claim/:id` has no handler and is 404, which matches the observation that the official binary answered 404 for it. | Observed fact (SRC-observe-sticky) and Wander policy. |
| Redirect | `/v1/volumes/claim` without a trailing slash is redirected by the inherited mux to the subtree path; it is not special-cased. | Wander policy. |
| CSI routes | `/v1/volumes`, `/v1/volume/csi/` and the other CSI routes are unchanged; `/v1/volumes/claims` was a 404 before. | Wander policy. |
| Inherited code | Changed: two lines are added to `command/agent/http.go`, which register `/v1/volumes/claims` and `/v1/volumes/claim/` beside the existing `/v1/volumes` routes. No CSI handler changes. Tests changed: `TestHTTP_HostVolume_Routes` gains cases for the new second segment (404 for an unknown one, 405 for GET on `/delete` and `/deregister`). |  |

## Audit log of volume operations (server) {#audit-log}

### Wander policy

The audit trail is Wander policy and has no public-interface row. The
Enterprise-style audit block (sinks, filters and their configuration) is not
provided; the trail is an ordinary server log line. It is implemented in
`nomad/host_volume_audit.go`, called from the RPCs in
`nomad/host_volume_create.go`, `nomad/host_volume_delete.go` and
`nomad/host_volume_claim.go` and from the reconciler in
`nomad/host_volume_reconcile.go`. Owner decision of 2026-11-27.

| Question | Wander policy |
| --- | --- |
| Line | One Info line, message `volume operation`, under the logger `dynamic_host_volume.audit`. Under the agent's server logger, which is named `nomad`, the full name is `nomad.dynamic_host_volume.audit`, as for the other server subsystems. Keys: `actor`, `namespace`, `volume_id` (`claim_id` for a claim delete), `operation`, `outcome`, `reason` only for a client rejection, and `force=true` only when force was used. |
| Operations | `create`, `register`, `update` (a create or register that carries a volume ID), `delete`, `deregister`, `delete_claim`. Force is reported for delete and deregister, including a forced call that is refused. |
| Outcomes | A closed set chosen from the kind of the result, never from an error text: `ok`, `ready` and `failed` (the state of the volume a create, register or update returns, also used by the reconciler), `removed` (reconciler), `denied`, `invalid` (400), `not_found` (404), `in_use`, `in_progress` and `conflict` (409), `version_gate` (503), `client_rejected` with a `reason` from the fixed list of the delete RPC (`client_rejected` otherwise), `client_unreachable` (any other 502), `outcome_not_recorded` (500) and `error`. A create the client rejected is `failed`, not `ok`. |
| Actor | The token accessor ID, `anonymous` for the anonymous token, `management` when ACLs are disabled, `workload:<allocation ID>` for a workload identity, `client:<node ID>` for a node secret, `unauthenticated` when authentication failed, and `leader` for the leader's own settlements (the accessor ID of the leader's internal token). The token secret is never read. |
| Not logged | A token secret, a host path, a plugin parameter, plugin output, a client message and error text. IDs and namespaces come from the caller and are logged as values, which the logger quotes, so one cannot end the line or start another. |
| Every path | The line is written by a deferred call straight after the forwarding step, so authentication failures, ACL errors, ACL denials, validation refusals and every later refusal are covered. A volume ID that is generated by a create is logged only once the record exists, so a refusal before that logs an empty `volume_id`. |
| Forwarding | A call forwarded from a follower is logged once, by the server that runs it, which authenticates it again. A call that cannot be forwarded because there is no leader logs nowhere. |
| Reconciler | The leader logs once when it settles a volume: ready or failed for a requested or pending volume (`create` or `register` by origin) and removed for a deleting one (`delete` or `deregister` by origin), after the write succeeded. An attempt that settles nothing, a code written for a waiting removal and a wait for the version gate log nothing. A create left pending by the RPC and settled later by the reconciler gives two lines with different actors, because they are two events. |
| Unauthenticated callers | A call that fails authentication and reaches the server that runs it writes a `denied` line, so an unauthenticated caller can cause log lines. The rate metric for the RPC applies as before. |
| Inherited code | none changed. |

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
