# Synthetic dynamic host volume fixtures

[The catalog](../../testdata/dynamic-host-volumes/cases.json) contains invented
data for future runtime tests. It does not create directories, launch plugins,
send requests, or demonstrate compatibility. Matrix runtime test IDs are
marked implemented in the matrix only when a repository test exists; the others
remain planned. The fixture checker and its tests verify this catalog's
boundaries only.

The catalog has a closed schema: `schema_version`, `synthetic_only`, `resources`,
and `cases`. Resources define two namespaces, two nodes in distinct pools, two
distinct volume IDs and host paths, a job/task group, small byte capacities,
plugin parameters, and distinct marker strings. The paths below
`/srv/wander-synthetic/` are descriptive synthetic values, never read by the guard.
No private artifact or private identifier denylist is needed or consulted.

Each case binds a stable fixture ID to a matrix behavior, test ID, and assertion
facet. `inputs` selects the closed synthetic scenario vocabulary. `expected`
contains the referenced assertion's exact status and value. A null value remains
null until permitted evidence resolves that matrix facet; a negative-input case
does not establish an error message, HTTP status, or lifecycle outcome.

| Fixture | Scenario | Matrix behavior / test | Facet |
| --- | --- | --- | --- |
| DHV-F-001 | Create request contract | DHV-B-003 / DHV-T-003 | contract |
| DHV-F-002 | Register an existing synthetic path | DHV-B-004 / DHV-T-004 | contract |
| DHV-F-003 | Request data across synthetic namespaces | DHV-B-012 / DHV-T-012 | outputs, unresolved |
| DHV-F-004 | Missing node input | DHV-B-018 / DHV-T-018 | errors, unresolved |
| DHV-F-005 | Inverted requested capacity range | DHV-B-112 / DHV-T-112 | errors, unresolved |
| DHV-F-006 | Synthetic plugin failure | DHV-B-096 / DHV-T-096 | errors, unresolved |
| DHV-F-007 | In-use deletion restriction | DHV-B-022 / DHV-T-022 | contract |
| DHV-F-008 | Registered external path restriction | DHV-B-110 / DHV-T-110 | lifecycle, Wander requirement |

Run the checks from the repository root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s ci -p 'test_dynamic_host_volume_*.py'
python3 ci/validate_dynamic_host_volume_contract.py
python3 ci/check_dynamic_host_volume_fixtures.py
```

The fixture guard enumerates only `testdata/dynamic-host-volumes/`. It permits
exactly one regular file, `cases.json`, and rejects unknown files, directories,
and symlinks before content reads. It checks path components for symlinks and
uses a no-follow, nonblocking open with file-type and byte limits. It also reads
the fixed public matrix at `docs/compatibility/dynamic-host-volumes.json` to
validate references and outcomes. Catalog contents cannot redirect either read.
The catalog limit is 64 KiB and the matrix limit is 2 MiB. Duplicate JSON keys,
invalid JSON, extra/missing schema fields, unknown vocabulary, incorrect topology,
and invented outcomes are rejected. Numeric vocabulary comparisons also
distinguish integers from floating-point values.

Diagnostics contain only a rule ID, a registered fixture ID (or `catalog`), and
a fixed field name. They never include rejected values, unknown filenames,
paths, raw JSON parser errors, or exception text. CLI tests use an invented
sentinel to verify redaction. Symlink tests use disposable synthetic targets
and verify that no content loader is invoked.

This is a bounded fixture admission guard, not a repository-wide privacy audit.
It does not inspect unrelated files, private infrastructure, or other fixture
trees. Human review still covers documentation, screenshots, logs, and new
artifacts outside this declared scope. Run checks on a stable checkout; they
are not a filesystem monitor or a defense against concurrent checkout mutation.

To extend the catalog, first add or resolve the relevant matrix assertion using
the [provenance workflow](dynamic-host-volumes-provenance.md#extension-workflow).
Assign a new stable fixture ID, independently invent its data, and update the
guard's explicit resource vocabulary and scenario bindings in the same change.
Add mutation tests for newly admitted fields. Review diagnostics for redaction,
run all checks above, the license-boundary check, whitespace checks, and Deslop.
Adding a fixture never changes a runtime test's implementation status.
