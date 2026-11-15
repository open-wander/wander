# Norman Progress

Append-only execution log for crash recovery and reusable project patterns.

## 2026-11-08 — Dynamic host volumes imported

- Imported the approved clean-room dynamic-host-volumes PRD into
  `prds/backlog/prd-dynamic-host-volumes.md`.
- Created 31 dependency-ordered tasks across six phases in `prds/TASKS.md`.
- Recorded project commands, model strategy, verification rigor, and mandatory
  clean-room/privacy constraints in `prds/config.md`.
- Left the unrelated update, security advisory, and usage reporting PRD in
  `prds/research/`.
- Next task: 1.1, establish the public compatibility matrix and provenance
  record.

PATTERN: provenance - Post-1.6.5 public documentation and independently
observed black-box behavior may define interfaces, but never implementation.

PATTERN: privacy - Private migration evidence stays local and is translated
into synthetic acceptance cases before anything is committed or published.

PATTERN: migration - Existing directories are adopted through explicit
registration into new Wander Raft records; registration does not move, copy,
or mutate directory contents.

## 2026-11-08 — Task 1.1 complete

- Added `docs/compatibility/dynamic-host-volumes.json`, a 200-entry interface
  ledger spanning all 11 required product surfaces. Every runtime test remains
  explicitly planned and unresolved behavior remains null rather than inferred.
- Added `docs/compatibility/dynamic-host-volumes-provenance.md` with permitted
  evidence, prohibited inputs, mutable-documentation limitations, public
  contradictions, and the extension workflow.
- Added an offline validator and five unit tests with 42 mutation cases. The
  required red test first failed because the matrix was absent; later red tests
  exposed invalid PRD references and insufficient observation provenance.
- Final evidence: five contract tests passed; direct validation passed; the
  licence-boundary check passed; Deslop reported no new duplication.
- Advisor plan verdict: PROCEED. Advisor code review: PASS after both minor
  hardening findings were fixed and re-reviewed.
- The shared PRD remains in `prds/active/` because later project tasks still
  depend on it.
- Next ready tasks: 1.2 and 1.3.

PATTERN: provenance - Record operations separately from inputs, outputs,
errors, authorization, concurrency, and lifecycle; unsupported facets remain
explicitly unresolved.

PATTERN: provenance - Validate requirement references against their actual
definitions and require structured observation records rather than accepting
arbitrary anchors.

PATTERN: tooling - `git diff HEAD` excludes untracked additions, so supplement
the required scanner with no-index input until new files are tracked.

## 2026-11-09 — Task 1.2 complete

- Added a closed eight-scenario synthetic fixture catalog covering creation,
  registration, namespace separation, invalid inputs, plugin failure, and
  destructive-operation restrictions without exercising runtime behavior.
- Added a bounded offline fixture guard that reads only the registered catalog
  and public compatibility matrix, rejects unknown entries and symlinks before
  content reads, bounds file sizes, and emits only fixed diagnostic tokens.
- Added seven privacy and mutation tests. The required red test first failed
  because the fixture catalog was absent; the completed dynamic-volume suite
  passes all 12 contract and fixture tests.
- Added fixture scope and extension documentation and wired the offline checks
  into the core security workflow.
- Final evidence: fixture guard, contract validator, workflow validation,
  licence-boundary checks, whitespace checks, and Deslop passed. Independent
  review returned PASS with no findings.
- Runtime compatibility tests remain planned and unresolved matrix outcomes
  remain null. Next ready task: 1.3.

PATTERN: privacy - Admit explicit synthetic vocabulary and registered files
instead of consulting private identifier denylists; keep diagnostics independent
of rejected content.

PATTERN: compatibility - Bind fixtures to individual matrix facets and retain
null outcomes until permitted evidence resolves them.

## 2026-11-09 — Task 1.3 complete

- Added independent internal models for dynamic host volumes, capabilities,
  constraints, lifecycle labels, allocation attachments, origins, and sticky
  claims. Public API models and behavior remain deliberately separate.
- Added nil-safe deep-copy and complete equality contracts that preserve nil
  versus empty collections, nil elements, order, unknown labels, timestamps,
  and Raft indexes.
- Added tests-first typed JSON and MessagePack round trips, including integer
  values above 2^53, zero/unknown fields, collection representation, and copy
  isolation. The required red test initially failed on undefined model symbols.
- Generated codecs for the Wander-authored models and audited all existing
  generated functions for semantic stability. A temporary workspace was needed
  because the pinned retracted go-msgpack v1.1.5 generator uses a duplicate
  base64 alphabet rejected by modern Go; no repository dependency, cache source,
  or global tool was changed.
- Final evidence: focused and complete `nomad/structs` tests passed with both
  reflection and `codegen_generated` codecs; dynamic contract/privacy checks,
  licence boundary, whitespace checks, and Deslop passed. Independent review
  returned PASS with no findings.
- Validation and immutable-field rules remain task 1.4. No lifecycle operation,
  Raft state, public API, scheduler behavior, or runtime compatibility is claimed.

PATTERN: provenance - Internal representation tests establish Wander model
requirements without verifying unresolved public interface claims.

PATTERN: generation - Compare generated functions by signature and body so
relocation and formatting churn cannot conceal semantic changes.

## 2026-11-09 — Task 1.4 complete

- Added pure operation-specific validation for internal create and register
  records, capabilities, attachments, claims, capacities, constraints, and
  lexical registration paths without defaulting or mutating records.
- Added deterministic aggregated errors with stable sentinel classifications,
  safe field names, bounded mode values, and redaction of constraint contents,
  paths, identifiers, parameters, and records.
- Added an eight-field immutable identity guard covering ID, namespace, name,
  type, node assignment, origin, create index, and create time. Passing the
  guard does not authorize any other change.
- The tests-first run initially failed because the validation methods and error
  sentinels were absent. Focused and complete `nomad/structs` tests now pass in
  normal and `codegen_generated` modes, including existing static and CSI tests.
- Dynamic contract/privacy checks, licence boundary, and whitespace passed.
  Deslop findings were reviewed: transaction logic is shared, while the
  remaining typed query wrappers, schema rows, and test assertions are retained
  for type safety and clarity. Independent review returned PASS with no findings.
- Public validation envelopes, omitted-capacity defaults, update permissions,
  lifecycle rules, Raft state, API behavior, scheduler decisions, and ACLs
  remain unresolved or assigned to later tasks. Next task: 1.5.

PATTERN: validation - Preserve error classification without retaining sensitive
underlying errors or values.

PATTERN: validation - Test both prohibited identity changes and unrelated
changes outside an invariant guard's authority.

## 2026-11-09 — Task 1.5 complete

- Added namespace-scoped state-store tables for dynamic host volumes and sticky
  claims, with deterministic compound indexes for IDs, node and plugin prefixes,
  node/name uniqueness, volume references, and jobs.
- Added copy-safe transactional CRUD with create-only and exact-index CAS,
  monotonic per-table indexes, immutable identity enforcement, duplicate-batch
  rejection, claim referential integrity, and complete rollback on failure.
- Exact reads and iterators return deep copies while preserving MemDB watch
  channels. Namespace deletion now rejects namespaces containing dynamic host
  volumes without exposing volume metadata.
- The tests-first run initially failed because the state methods were undefined.
  Focused normal, race-enabled, and `codegen_generated` state tests now cover
  namespace isolation, exact and prefix scans, optional indexes, stale writes,
  failed-batch index preservation, copy isolation, watches, claims, deletion,
  and concurrent CAS.
- Dynamic contract/privacy checks, licence boundary, whitespace, and Deslop
  passed. Independent review returned PASS with no findings.
- This checkpoint establishes internal MemDB transaction behavior only. Snapshot
  serialization, restore, full blocking-query responses, FSM commands, public
  APIs, lifecycle operations, scheduling, ACLs, and runtime compatibility remain
  later tasks. Next task: 1.6.

PATTERN: state - Keep CAS, uniqueness, references, record writes, and table-index
updates in one transaction so every failure rolls back completely.

PATTERN: state - Return isolated records without discarding iterator watches;
record namespace-scoped node/name uniqueness as Wander policy, not upstream
compatibility evidence.

## 2026-11-15 — Task 1.6 complete

- Added Wander-assigned snapshot types 29 (dynamic host volumes) and 30 (sticky
  claims), persisted after namespaces with volumes before claims, each in
  `(namespace, ID)` order through the copy-returning state iterators.
- Added `DynamicHostVolumeRestore` and `DynamicHostVolumeClaimRestore`. Restore
  rejects nil and invalid records, a duplicate `(namespace, ID)`, and a node and
  name conflict, inserts copies, and returns sentinel errors only. It performs
  no namespace or claim-to-volume referential checks so a filtered restore keeps
  working. Table indexes come from the existing index snapshot entries.
- No production change was needed for blocking queries. New tests cover every
  read and list method for create, update and delete with strictly increasing
  table indexes, and correct results for unrelated namespaces and tables.
- FSM tests cover the full round trip including indexes, a pre-feature
  snapshot, failed restores that leave the old state live and not abandoned,
  the downgrade simulation with an unregistered type byte, and deterministic
  persist order without mutating live records.
- Documented the downgrade limitation in
  `docs/compatibility/dynamic-host-volumes-provenance.md`: an older binary
  refuses any snapshot that contains types 29 or 30.
- The advisor revised the task before work began: determinism means record
  order and decoded record set, not bytes, because the shared msgpack handle is
  not canonical. `helper/raftutil` was left out of scope.
- The tests-first run failed before implementation. Focused tests pass in
  normal, race-enabled and `codegen_generated` modes. Build, vet, the contract
  and fixture guards, the licence boundary, whitespace and gosec passed. Deslop
  findings were reviewed: the two persist functions and the new tests follow
  the existing per-type patterns in `fsm.go` and its tests and are retained.
- Advisor review returned MINOR. Improvement notes: the persist isolation test
  cannot observe aliasing because state reads already return copies; the copy
  and index guards in restore are caught by the state-level tests, not the FSM
  tests. Phase 1 is complete and collapsed into `TASKS-archive.md`. Next
  unblocked tasks: 2.1 and 3.1.

PATTERN: snapshot - Wander snapshot type bytes are append-only below the
reserved 64+ range; an older binary refuses the whole snapshot on an unknown
type, so every new type is a downgrade barrier that must be documented.

PATTERN: restore - Check duplicates and unique keys explicitly because a MemDB
insert replaces silently; keep cross-table checks out of restore so filtered
restores still work.

PATTERN: testing - Assert snapshot determinism on record order and decoded
content, not bytes; simulate an older binary with an unregistered type byte.

## 2026-11-15 — Task 3.1 complete

- Added the namespace capabilities `host-volume-create`, `host-volume-register`,
  `host-volume-read`, `host-volume-write` and `host-volume-delete` to the ACL
  policy parser. The compiled ACL needed no change: the existing namespace
  evaluation already handles exact, glob and wildcard namespaces, deny
  precedence and management tokens.
- The capabilities are independent, and the `read`, `write` and `scale`
  namespace shorthands grant none of them. This is recorded as Wander policy
  because the ACL-surface assertions DHV-B-137 to DHV-B-144 are unresolved. An
  operator with `policy = "write"` must list the capabilities explicitly.
  Revisit when the evidence resolves the shorthands.
- The "X or host-volume-write" alternatives in the API evidence are
  endpoint-level checks for phase 4, not implications in the policy compiler.
- Mixed-version note recorded: a server without this change rejects a policy
  that contains a `host-volume-*` capability, so such policies are applied only
  after every server is upgraded.
- The tests-first run failed to compile on the undefined constants. The parser
  test and the table-driven matrix pass in normal and race-enabled modes.
  Build, vet, contract and fixture guards, licence boundary and whitespace
  passed. Advisor review returned PASS.
- Ran in parallel with task 2.1 in a separate worktree. The harness created
  the worktrees from `main`, not from the feature branch.

PATTERN: acl - When evidence does not resolve a shorthand or implication, keep
capabilities independent and deny by default, and record the choice as Wander
policy with its operator-visible consequence.

PATTERN: tooling - Workflow worktrees start from `main`; tell workers to
fast-forward to the feature branch first, or export the diff against the
worktree's own base.

## 2026-11-15 — Task 2.1 complete

- Added the client settings `host_volume_plugin_dir` and `host_volumes_dir`
  (names and default suffixes from evidence DHV-B-074 and DHV-B-075) to the
  agent configuration, with merging, and carried the resolved values into
  `client/config.Config`. Defaults are `data_dir/host_volume_plugins` and
  `data_dir/host_volumes`, derived in `convertClientConfig` only.
- Validation is pure and touches no filesystem: a relative path is rejected and
  names the key, and identical explicit plugin and volume directories are
  rejected as Wander policy.
- Added `Config.StaticHostVolume` as the lookup seam for static and dynamic name
  collisions. DHV-B-078 is unresolved, so no contract wording was added.
- The criterion "empty explicitly required paths" was resolved against the
  evidence: an empty value means use the default, so it is not an error.
- A change to either directory needs an agent restart, because the client
  reload path applies only TLS and fingerprinting. Recorded in field comments.
- The tests-first run failed before implementation. Advisor review rejected the
  first attempt (missing licence headers on new test files, a displaced doc
  comment, and no test through `IsValidConfig`); the fix round passed with
  MINOR notes. Focused tests pass in normal and race-enabled modes. Build, vet,
  contract and fixture guards, licence boundary and whitespace passed.
- The orchestrator dropped the worker's addition to
  `website/content/docs/configuration/client.mdx`: that tree is inherited,
  unpublished and under provenance review. Operator documentation for these
  settings is still owed, in Wander's own docs.
- Open items for 2.2 and 2.3: with no data_dir and no explicit value both
  resolved directories are empty and must be treated as "dynamic host volumes
  unavailable"; the equal-directory check compares explicit values only, so
  check the resolved values too; directories are not created yet.

PATTERN: config - Keep operator-written values in the agent config and derive
defaults only in the converted client config, so merge and reload comparisons
stay clean.

PATTERN: docs - Do not add new material to the inherited `website/` tree; write
operator documentation in Wander's own docs.

## 2026-11-15 — Task 2.2 complete

- Added `client/hostvolume`: discovery of regular executable files directly in
  the plugin directory (no recursion, symlinks and other non-regular entries
  skipped, sorted by ID, capped at 32) and a bounded runner. The runner uses an
  explicit argv with no shell, an environment of `DHV_OPERATION=fingerprint`
  plus a fixed PATH, a 5 second timeout, 1024-byte caps on stdout and stderr,
  and a process-group kill on Unix.
- Added the periodic `plugins_host_volume` client fingerprinter. Healthy
  plugins are advertised as node attributes
  `plugins.host_volume.version.<id>`. `structs.Node` is unchanged, so an older
  server ignores the new data and no codecs were regenerated.
- `convertClientConfig` now rejects resolved plugin and volume directories that
  are equal. The check is lexical.
- The built-in `mkdir` plugin is not advertised and the ID is reserved; that is
  task 2.4. No external plugins are discovered on Windows.
- Wander policy recorded in the provenance document for every point the
  evidence leaves open: node representation, ID and version rules, exit
  status, output limits, environment and working directory.
- OPEN, must be settled before task 2.3: the fingerprint stdout encoding is
  unresolved in the evidence (DHV-B-080 only says the version comes back on
  stdout). The implementation accepts one plain version line. If the public
  plugin specification defines a structured format, existing plugins will be
  reported unhealthy. Record the public evidence and adjust the parser.
- The tests-first run failed before implementation. Advisor review returned
  MINOR. The orchestrator added a test that a timeout kills the plugin's child
  processes (confirmed by mutation: it fails without the group kill) and
  lowered the expected unsupported-platform diagnostic to debug level.
- Improvement notes: the first fingerprint pass runs synchronously during
  client start under the node lock, so slow plugins can delay start by up to
  40 seconds; the inherited fingerprint manager never re-runs a periodic
  fingerprinter after one error, so this fingerprinter never returns one.
  gosec reports G204 for the plugin exec, which is inherent to running a
  plugin from the configured directory.
- Focused tests pass in normal and race-enabled modes. Build, Windows compile,
  vet, contract and fixture guards, licence boundary and whitespace passed.

PATTERN: exec - Run plugins with an explicit argv and environment, bounded
output writers, a deadline, and a process-group kill; test the group kill by
checking that a child process is gone.

PATTERN: fingerprint - A periodic fingerprinter must not return an error for
expected faults, and should clear stale attributes by scanning the node's
current attributes so it is correct after a restart.

## 2026-11-15 — Corrections to tasks 3.1 and 2.2 from public documentation

- The project owner authorised reading the public documentation to settle two
  points the evidence file left unresolved. Read on 2026-11-15: the plugin
  authoring specification (`SRC-plugin`), the API volumes page (`SRC-api`) and
  the ACL policy specification (new source `SRC-acl-spec`). The evidence cutoff
  moved to 2026-11-15 and the validator allow-list gained the ACL policy
  specification path. Only interface facts were recorded; example plugin code
  on the page was not read into the record.
- Task 3.1 corrected: `host-volume-write` grants all five capabilities,
  `host-volume-register` grants create and read, `host-volume-create` grants
  read, the `read` shorthand includes `host-volume-read` and the `write`
  shorthand includes `host-volume-write`. The earlier independent,
  deny-by-default model would have stopped tokens with `policy = "write"` from
  managing volumes.
- Task 2.2 corrected: fingerprint stdout is one JSON object with a string
  `version` that go-version accepts. A plain version line is now rejected.
  Added reason code `invalid_output`.
- Recorded for task 2.3 in DHV-B-081, 082 and 096 to 102: the create and delete
  environment variables, the create output (`path`, `bytes`), the discarded
  delete output, the optional `error` object, 60 second timeouts, idempotency,
  per-name concurrency safety, no automatic retries, and cleanup rules.
- A validator test assumed the first source's date equals the evidence cutoff;
  it now uses the source's own date.
- Tests were changed first and failed before each fix. ACL, plugin, fingerprint
  and dependent server tests pass, with race detection where applicable.

PATTERN: provenance - When the evidence file leaves a public interface point
unresolved, stop and resolve it from the permitted public documentation before
implementing, instead of choosing a narrower Wander policy. A deny-by-default
or stricter-format guess breaks compatibility as surely as a wrong one.

## 2026-11-15 — Task 2.3 blocked on evidence

- The advisor returned REVISE and the worker reported BLOCKED before writing
  code: five plugin-compatibility points were not settled by the evidence.
- The public plugin specification was re-read with those questions. Settled
  and recorded: capacity variables are byte counts; `DHV_PARAMETERS` is the
  parameters as JSON text; the create example shows `bytes` as a number and
  `path` may lie within `DHV_VOLUMES_DIR` or elsewhere; the optional `error`
  object applies to create and delete and its message is returned to the user;
  a failed create exits non-zero; the plugin runs as the agent's user.
- Not stated by the documentation, so still open: the value of the capacity
  variables when capacity is unset; the value of `DHV_PARAMETERS` when there
  are no parameters; whether a variable with an empty value is set to an empty
  string or omitted; whether `bytes` or `path` may be omitted; whether the
  `error` object counts on a zero exit.
- These need either a black-box observation with a synthetic plugin on an
  official binary, or an owner decision to accept a tolerant Wander policy.
- Task-text corrections to apply when 2.3 resumes: the orchestrator, not the
  worker, updates `prds/`; delete stdout overflow truncates and does not kill
  the plugin; create needs a larger stdout limit than fingerprint; the 60
  second deadline starts after the per-volume lock is acquired.
