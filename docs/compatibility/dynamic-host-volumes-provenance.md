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

The HTTP delete table includes a delete suffix while its example omits it.
The claim delete table uses a plural collection while its example uses singular.
These conflicts must be resolved with permitted independent black-box requests
before implementation. Claim read, independent update, deregistration, force,
confirmation, CLI flag details, Go method names, full response schemas,
pagination, wildcard visibility, ACL shorthand, claim retention, and UI flows
remain unresolved where explicitly marked. Never infer host semantics from CSI.
Documented fields alone do not define validation rules or default behavior.

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
