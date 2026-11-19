# Norman Tasks Archive

Fully completed phases, moved out of `TASKS.md`. Phase and task numbers are
preserved.

## Phase 1: Clean-room foundation

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 1.1 | Establish the public compatibility matrix and provenance record | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-08) | Added validated 200-entry clean-room evidence ledger and provenance record |
| 1.2 | Build synthetic contract fixtures and private-data guards | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-09) | Added a closed synthetic catalog, bounded redacting guard, mutation tests, documentation, and CI enforcement |
| 1.3 | Define independent dynamic-volume, capability, constraint, state, and claim models | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-09) | Added internal Wander models, deep-copy/equality contracts, typed serialization tests, and generated codecs |
| 1.4 | Implement model validation and immutable-field rules | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-09) | Added pure create/register, capability, attachment, claim, capacity, path, and eight-field identity validation |
| 1.5 | Add Raft state tables, indexes, and CRUD transactions | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-09) | Added namespace-scoped state tables, compound indexes, copy-safe CAS CRUD, claim references, and atomic rollback |
| 1.6 | Add snapshots, restore, blocking queries, and upgrade compatibility | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added snapshot types 29/30 with validated restore, blocking-query watch tests, pre-feature snapshot restore, and the documented downgrade limit |
