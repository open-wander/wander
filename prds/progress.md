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

## 2026-11-17 — Task 2.3 complete

- The owner chose to proceed with a tolerant policy for the five points the
  public specification does not state, and to verify them later by black-box
  observation on an official binary. An observation kit (synthetic recording
  plugin, two volume specifications, record template) was handed to the owner;
  it is not committed.
- Added `Ops.Create` and `Ops.Delete` in `client/hostvolume/operation.go`. The
  environment is an explicit allowlist of the documented variables plus the
  fixed PATH. The deadline is 60 seconds and starts after the per-volume lock
  is held; waiting for the lock honours the caller's context. The lock is a
  reference-counted semaphore whose entries are removed when unused.
- The five pending points each sit in one small function: `formatCapacity`
  (unset is 0), `encodeParameters` (none is `{}`), `buildEnv` (every variable
  always set, empty as an empty string), `parseCreateOutput` (path required,
  bytes optional), `parsePluginError` (read only on a non-zero exit).
- Create results are validated: the path must be absolute, clean, bounded, free
  of control characters and not the root. It need not lie under the volumes
  directory. Returned bytes are not checked against the requested capacity.
- Delete stdout is truncated and discarded, so a chatty plugin is not killed.
- Parameter values are redacted from every surfaced message before
  truncation. A very short value is redacted wherever it appears.
- `plugin.go` changed additively only; the fingerprint path is unchanged.
- The tests-first run failed before implementation. Advisor review returned
  MINOR. The orchestrator tightened the redaction test so that it fails when
  truncation runs before redaction (confirmed by mutation). The worker's
  mutation checks covered the lock, the path clean check, the environment
  allowlist and redaction.
- Tests pass in normal and race-enabled modes (repeated runs). Build, Windows
  compile, vet, guards, licence boundary and whitespace passed.
- Follow-up: when the observation arrives, record it as a black-box source,
  update DHV-B-081, 092 to 094 and 096, and adjust the five functions.

PATTERN: redaction - Redact sensitive values before truncating, and test with
a value that straddles the cut so a surviving prefix is detected.

PATTERN: locking - Start an operation deadline only after the per-key lock is
held, bound the wait by the caller's context, and remove unused lock entries.

## 2026-11-17 — Task 2.4 complete

- The public volume specification was read for the built-in plugin before the
  task started and recorded in DHV-B-103 to DHV-B-106: plugin id `mkdir`, a
  directory in `host_volumes_dir` named after the volume ID, `mode` (string,
  default 0700), `uid` and `gid` (integers, default the agent's user, ignored
  on Windows), capacity ignored. Delete behaviour and unknown parameters are
  not stated.
- Added `Ops.CreateBuiltin` and `Ops.DeleteBuiltin`, sharing the per-volume
  lock with external operations. Delete takes no path: it derives the
  directory from the volume ID, so it cannot be aimed at a registered
  external directory.
- Confinement: the volume ID must match `[a-z0-9_-]{1,64}`; the root's final
  element must not be a symlink; the root is opened once and the volume
  directory is opened without following symlinks, with same-file checks;
  ownership and mode are applied on the open descriptor, never by path.
- Wander policy, recorded in the provenance document and still to be confirmed
  by the owner: delete removes the directory and its contents and succeeds
  when it is gone; create is idempotent; bytes is always 0; unknown parameters
  are ignored; setuid, setgid and sticky bits are rejected; no uid or gid
  lookup is made; a missing managed root is created with mode 0711; the
  advertised version is 1.0.0; the plugin is disabled on Windows.
- The fingerprinter advertises `plugins.host_volume.version.mkdir` when the
  volumes directory is set, and now clears stale attributes whenever the node
  is known.
- The tests-first run failed before implementation. Advisor review returned
  MINOR. The orchestrator made the dangling-symlink case assert its reason
  code, added a regular-file case, and marked two tests as not parallel.
- Mutation checks by the worker turned the intended tests red for ID
  validation, the root check, uid range, special mode bits and chown failure
  handling. Removing a single symlink guard stays green because the guards
  overlap; that redundancy is deliberate.
- gosec reports G301 for the 0711 managed root. Other users need search
  permission on the root to reach a volume directory they own.
- Tests pass in normal and race-enabled modes. Build, Windows compile, vet,
  guards, licence boundary and whitespace passed.

PATTERN: confinement - Validate the untrusted name as one safe path element,
open the root once, open children without following symlinks, and change
mode and owner on the open descriptor; derive delete targets, never accept
them.

## 2026-11-19 — Phase 1 merged; sweep of Phase 1

- Phase 1 merged to `main` as `775094619e` (PR #59). `feat/dhv-phase-2` was
  rebased onto `main`.
- Comments: no change. The Phase 1 Go files hold 43 comment lines against
  1,785 lines of code, the longest block is three lines, and no task, phase or
  requirement numbers remain in comments.
- Invariants: 71 guards were broken one at a time, Go through `go test
  -overlay` and Python in a throwaway checkout. 61 are caught by an existing
  test. 10 survived, and two independent verifiers confirmed all 10 with an
  input that the unmodified code rejects and the broken code accepts. None was
  refuted. These are gaps in test coverage, not defects: the code is correct
  today.
- State store (`nomad/state`), none reachable yet because only tests call
  these functions:
  - `DynamicHostVolumeClaimsByNamespace` can return every namespace's claims
    and no test notices. No test stores a claim outside `default`, so the
    other claim listers are probably in the same position. Medium.
  - `UpsertDynamicHostVolumes` can accept a type other than `host` and no test
    notices. Restore rejects such a record, so one stored record would make
    every later snapshot fail to restore. Medium.
  - `UpsertDynamicHostVolumeClaims` can skip `Validate` and no test notices.
    Only an empty `TaskGroupName` becomes storable; the other five fields are
    still rejected, three of them with a different error code. Same snapshot
    cost. Low.
- Evidence ledger validator (`ci/validate_dynamic_host_volume_contract.py`):
  - A source with a prohibited category passes when its locator is the
    provenance record. The eight "prohibited category" test cases are rejected
    by a different rule. High.
  - A local source can point at any file that contains the anchor. Medium.
  - A behaviour can be marked implemented or verified while citing planned
    tests. Medium.
  - A test can be marked implemented against a function that does not exist.
    The existing case fails the name pattern first. Medium.
- Fixture guard (`ci/check_dynamic_host_volume_fixtures.py`):
  - A fixture can state an invented expected status or value on a resolved
    case. High.
  - A repeated JSON key lets a shadowed first value go unchecked, including
    `synthetic_only: false`. The contract validator does not check for
    repeated keys at all. High.
  - The catch-all error branch can print exception text. Only type and key
    names leak today. Low.
- Shared cause for five of the ten: the tests assert that some error is
  raised, not which rule raised it, so a case rejected by a different rule
  still passes.
- No tests were added. Which gaps are worth a test is the owner's decision.

PATTERN: testing - A negative test that asserts only "an error was returned"
keeps passing when a different rule rejects the input. Assert the specific
error or rule, and give each guard one input that is valid in every other
respect.

## 2026-11-19 — Task 2.5 complete

- Owner decision: a client reports each ready dynamic volume in the node's
  existing `HostVolumes` map (option A), so the inherited scheduler check
  keeps jobs off a node where the volume is unavailable. No new RPC and no
  new top-level node field.
- Added `client/hostvolume/manager.go`, the only writer of client volume
  records, with a record type in `client/hostvolume/state` and three new
  methods on the client state store (bucket `hostvolumes`, no state version
  change). The manager restores records at start, reconciles them every 30
  seconds and publishes the ready set only when it changes. The client runs
  restore and the periodic loop in its shutdown group.
- `ClientHostVolumeConfig` gained `ID`, set to the volume ID for a dynamic
  volume and empty for a static one. An agent `host_volume` block cannot set
  it. Without a marker the scheduler could not tell a dynamic volume from a
  static one and so could not enforce its namespace.
- Wander policy, recorded in the provenance document and still to be
  confirmed by the owner: names are unique per node and a static volume wins
  a conflict; between dynamic volumes the lower volume ID wins; a record for
  another node is never reported; a registered path is only stat-checked;
  restore re-runs the plugin's create once; a create that returns another
  path is not retried until the next client start; a plugin-created volume
  whose directory disappeared is created again, empty, on the next pass;
  restore runs in the background with a five minute bound; the manager's
  delete applies only to plugin-created volumes.
- First run reported BLOCKED before writing code: regenerating
  `structs.generated.go` reordered about 3,300 lines. Two causes: the
  generator's input order came from `ls` and so depended on the locale, and
  the pinned go-msgpack 1.1.5 generator panics on current Go. With a C locale
  and a patched temporary copy of the module, the unmodified tree regenerates
  byte for byte and the new field changes 75 added and 26 removed lines, all
  in the `ClientHostVolumeConfig` functions. PR #63 puts that fix into
  `make generate-structs`, together with a bump of the pinned `stringer`,
  which also crashed.
- The tests-first run failed before implementation. Advisor review returned
  MINOR. The orchestrator then:
  - made the store load happen once, before restore or the first entry point
    call, so a volume created before the background restore finishes is not
    replaced by a later read of the store, and restored only records read
    from the store;
  - stopped periodic retries of a `path_changed` volume, because each retry
    is another create;
  - added tests for the request restore passes to create, a plugin installed
    after start, a logged unreadable record, the `path_changed` rule and a
    create that runs before restore;
  - raised the timeout of two process-kill tests from 500 ms to 3 s. They
    read a PID file the plugin writes, and on a loaded machine the plugin was
    killed before it wrote it.
- Known limits: the diagnostic for an unavailable volume stays on the client
  (logs and the manager); the server only sees that the volume is absent from
  the node. No client-level test asserts that the host volume goroutine stops
  on shutdown; the manager-level test covers the loop.
- Build, Windows compile, vet, focused tests in normal, race-enabled and
  `codegen_generated` modes, both guards, the licence boundary and whitespace
  checks passed. gosec reports only the findings accepted earlier.

## 2026-11-19 — Follow-ups opened outside the task list

- PR #62 adds a test for each of the ten gaps the Phase 1 sweep confirmed.
  Each test fails when its guard is removed.
- PR #63 fixes `make generate-structs` on current Go.

PATTERN: generation - Run the codec generator with `LC_ALL=C`; its output
order follows the order of the input files. Require a byte-identical
regeneration of the unmodified tree before trusting a diff.

PATTERN: concurrency - When restore runs in the background, load persisted
state once before any entry point can write, and mark which records came from
the store so the restore pass does not repeat work done in the same run.

PATTERN: testing - A test that reads a file its child process writes must
give the child time to start on a loaded machine; a short operation timeout
turns it into a flaky test.

## 2026-11-19 — Owner decisions

- The owner accepted the Wander policy choices of task 2.4 (built-in mkdir
  plugin) and task 2.5 (restart reconciliation) as recorded in the provenance
  document.
- The black-box observation of plugin inputs (five points from task 2.3) is
  deferred until the owner can run it. The orchestrator does not run an
  official Nomad binary itself without the owner's explicit approval.
- PR #62 (sweep gap tests) and PR #63 (`make generate-structs`) merged to
  `main`.

## 2026-11-19 — Task 2.6 complete

- Added the client RPC endpoint `HostVolume.Register` in
  `client/host_volume_endpoint.go`, with request and response types in
  `client/structs/host_volume.go` (outside the code generator's input, so no
  generated file changed). It is for servers only, adds no HTTP route and
  performs no ACL check. Nothing on the server calls it yet.
- `Manager.AddRegistered` now validates the request before taking the volume
  lock and applies the repeat rules: an identical repeat returns the current
  status and writes nothing; a repeat that differs in namespace, name or path
  is rejected; an ID that belongs to a plugin-created volume is rejected.
- Advisor verdict was REVISE, because a typed error does not survive the RPC
  transport. Adopted as Wander policy: a classified rejection returns no RPC
  error and carries a reason code and message in the response; only internal
  failures return an RPC error. The reason-code table and the check order are
  in the provenance document. The owner accepted this policy on 2026-11-20.
- Registration never runs a plugin and only calls `os.Stat` on the path. A
  regular file is accepted, as for static host volumes.
- The tests-first run failed before implementation. Advisor review returned
  MINOR. The orchestrator added a check to every rejection case that no plugin
  was invoked and corrected the reason-code table for a malformed node ID.
- Not covered at endpoint level, only at manager level: the persisted record,
  restart, and the plugin-volume-ID rejection.
- Build, Windows compile, the `codegen_generated` build, vet, focused tests in
  normal and race-enabled modes, both guards, the licence boundary and
  whitespace checks passed. gosec reports only the findings accepted earlier.

PATTERN: rpc - An error returned from a client RPC reaches the caller as text
only. Carry a rejection the caller must classify in the response, and keep
the RPC error for failures of the call itself.

## 2026-11-20 — Phase 2 merged; sweep of Phase 2

- Phase 2 merged to `main` as `ce8b08584b` (PR #66). CI found three things
  the local checks had not covered: the repository's Semgrep rule for loop
  variables in goroutines, the rule that every package with tests is in a CI
  test group, and `hclfmt` formatting of a test configuration file. Semgrep
  and the steps of `make check` are now run locally before a pull request.
- Comments: the 26 new Go files hold 588 comment lines against 5,278 lines of
  code (ratio 0.11). Two comments and five passages of the provenance record
  still described the work as pending ("a later task", "nothing calls it
  yet") and were rewritten to state the current facts. The one block over ten
  lines, on `Manager.AddRegistered`, was left alone: it is the check order and
  the repeat rules, and every sentence is a separate promise.
- Invariants: four agents listed 94 guards with an exact one-edit break each;
  the orchestrator ran the breaks one at a time through `go test -overlay`.
  71 are caught by an existing test. 23 survived their whole package and went
  to three verifiers told to refute them: 21 are confirmed gaps, 1 is
  confirmed but cannot be tested without a new test seam, and 1 was refuted
  (a second guard still stops it).
- A defect, found while reading the manager and confirmed on unmodified code:
  records loaded at start keep the status the previous run stored, and the
  create, register and delete entry points publish the ready set. A call that
  arrives before the background restore has re-checked the records publishes
  volumes whose path is gone or whose plugin has not been asked again. Restore
  corrects it when it finishes, which can take up to five minutes. No server
  calls the client yet, so it cannot happen today. Suggested fix: publish a
  record only after it was created or checked in the current run.
- Confirmed gaps, plugin execution and configuration:
  - the five second fingerprint default is never exercised; every test sets
    its own timeout. This path is live: a hung plugin holds the node lock at
    client start for as long as the default says. High.
  - redaction replaces the longest parameter value first; no test has one
    value inside another, and with the order reversed most of the longer
    value leaks. High.
  - the sixty second operation default is never exercised. Medium.
  - the one second wait after a kill is never exercised by a helper that
    leaves the process group while holding the output pipes. Medium.
  - an unclean plugin directory equal to the derived volumes directory is not
    tested. Low.
- Confirmed gaps, volume manager, store and endpoint:
  - a repeated create whose store write fails could run the plugin's delete
    on a live volume. High impact, low likelihood.
  - at the restore deadline, records the pass never reached keep the stored
    status. Medium.
  - a result computed before a re-create can overwrite the new record.
    Medium.
  - a pass waiting on a delete could re-create the deleted volume; the
    existing test uses a ready record created in the same run, which neither
    create branch would touch. Medium.
  - a changed path or owner under an already published name is not
    republished. Medium.
  - a cancelled lock wait in the register endpoint could be answered as a
    final rejection. Low to medium.
  - a record of another node could be published in the window of the defect
    above; it becomes an equivalent mutant once the defect is fixed. Low.
  - a name-conflicted plugin volume is created again on every pass. Low.
  - a stored record under another key is returned. Low.
- Confirmed gaps, built-in mkdir plugin:
  - a relative symlink inside the managed root is followed when both the
    directory check and the same-file check are removed; the existing symlink
    cases use absolute targets, which `os.Root` refuses by itself. Medium.
  - a failed chmod is not tested; only chown is. Medium.
  - a failed removal on delete is not tested. Medium.
  - an inspection error on the managed root is not tested; delete would
    report success. Medium.
  - an inspection error on the volume directory is not tested; only a macOS
    ACL reaches it without a new seam. Low.
  - create and delete holding the volume lock for their whole duration is not
    tested at the `Ops` level. The manager's own lock masks both today. Low.
- Confirmed, not testable today: without `O_DIRECTORY` a FIFO swapped in at
  the volume name blocks create while it holds the volume lock. Reaching it
  needs a hook between the inspection and the open. Low.
- Refuted: removing only the plain-directory check at the volume name; the
  open flags, `os.Root` and the same-file check still reject every
  non-directory.
- Unit tests run as root in CI, so tests that rely on permission bits protect
  local runs only unless a seam is added.
- Unrelated to the sweep: `wander executor` and `logmon` processes from
  `TestE2E` runs on 2026-11-14 are still running on the development machine.
- No tests were added and the defect was not fixed. Both wait for the owner.

PATTERN: sweep - Let agents write the breaks as exact one-edit entries that
only have to compile, then run them from one place, one at a time; concurrent
test runs on a package with timing-sensitive tests would turn noise into
verdicts.

PATTERN: testing - A default that every test overrides is untested. Exercise
the zero value at least once, or assert the default through a small function.

PATTERN: state - A status restored from disk is a claim from the previous
run. Do not act on it, or publish it, until the current run has checked it.

## Resume point

- Phases 1 and 2 are merged and swept. Waiting for the owner: which Phase 2
  gaps get a test, and the fix for the early publish defect.
- Next task: 3.2 (node selection for volume creation).
