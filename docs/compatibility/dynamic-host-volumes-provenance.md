# Dynamic host volume provenance

This record accompanies [the JSON matrix](dynamic-host-volumes.json).
It captures interface evidence on 2026-11-08, not an implemented feature or a
complete verified contract. Every runtime acceptance test is currently planned.
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
capabilities, as the evidence records them in the HTTP API authorization
assertions. The existing `mount-readonly` and `mount-readwrite` stay
`host_volume` block capabilities keyed by volume name, and a read-only mount
grant does not satisfy a read-write mount check. The ACL-surface assertions
(DHV-B-137 to DHV-B-144) remain unresolved, so the model below is Wander policy,
not recorded public behaviour.

Wander policy:

- The capabilities are independent. No capability implies another, including
  `host-volume-write`.
- The namespace shorthands `read`, `write` and `scale` grant none of them.
  `deny` revokes them like any other namespace capability, and deny takes
  precedence across merged policies.
- Node policies grant none of them.
- Wildcard and glob namespaces follow the existing namespace rules: an exact
  match beats a glob, the closest glob replaces rather than merges, and
  management tokens pass every check.
- The "X or `host-volume-write`" alternatives in the API authorization
  assertions are endpoint-level alternatives. Enforcement (a later phase)
  checks them with OR semantics; they are not an implication in the policy
  compiler.

Consequence: an operator with `policy = "write"` on a namespace cannot manage
host volumes until the policy also lists the explicit capabilities. This is the
deny-by-default choice while the evidence is silent.

Upgrade: a Wander server without this change rejects any policy containing a
`host-volume-*` capability as an invalid namespace capability. During a rolling
upgrade, token resolution for such a policy fails on servers not yet upgraded,
and an upsert accepted by an upgraded leader replicates to them. Apply such
policies only after every server runs the new version.

Covered by `TestParse_HostVolumeNamespaceCapabilities` in `acl/policy_test.go`
and `TestHostVolumeNamespaceCapabilities` in `acl/acl_test.go`. Enforcement in
RPC, HTTP and CLI, wildcard-namespace list filtering and the UI are out of
scope here.

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
