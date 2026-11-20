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

## Phase 2: Client and plugins

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 2.1 | Add plugin and managed-volume client configuration | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added host_volume_plugin_dir and host_volumes_dir with merge, data_dir defaults, validation, and a static-volume lookup seam |
| 2.2 | Implement executable discovery, fingerprinting, and health reporting | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added bounded plugin discovery and fingerprinting, advertised as node attributes; fingerprint output corrected to the documented JSON format |
| 2.3 | Implement the bounded and cancellable plugin operation runner | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-17) | Added client-side create and delete plugin calls with validation and per-volume locking; five input and output encodings are policy pending black-box verification |
| 2.4 | Implement the confined built-in directory plugin | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-17) | Added the confined in-process mkdir plugin with mode, uid and gid, advertised on the node; disabled on Windows |
| 2.5 | Add restart reconciliation and availability recovery | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-19) | Added a client volume manager that persists records, restores and reconciles them after restart, and reports ready volumes in the node host volume list with their volume ID |
| 2.6 | Implement the client handshake for registering existing paths without modifying contents | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-19) | Added the client RPC endpoint HostVolume.Register, which adopts an existing path through the volume manager without running a plugin or touching the path |
