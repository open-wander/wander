# Norman Tasks

Task status lives only in this file and `TASKS-archive.md`. All implementation
must follow the clean-room and private-data rules in the linked PRD and
`docs/plans/license-safe-parity-method.md`.

## Phase 1: Clean-room foundation — 6 tasks completed 2026-11-15 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 2: Client and plugins

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 2.1 | Add plugin and managed-volume client configuration | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added host_volume_plugin_dir and host_volumes_dir with merge, data_dir defaults, validation, and a static-volume lookup seam |
| 2.2 | Implement executable discovery, fingerprinting, and health reporting | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added bounded plugin discovery and fingerprinting, advertised as node attributes; fingerprint output corrected to the documented JSON format |
| 2.3 | Implement the bounded and cancellable plugin operation runner | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-17) | Added client-side create and delete plugin calls with validation and per-volume locking; five input and output encodings are policy pending black-box verification |
| 2.4 | Implement the confined built-in directory plugin | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-17) | Added the confined in-process mkdir plugin with mode, uid and gid, advertised on the node; disabled on Windows |
| 2.5 | Add restart reconciliation and availability recovery | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.6, 2.3 |
| 2.6 | Implement the client handshake for registering existing paths without modifying contents | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.6, 2.5 |

## Phase 3: Scheduling and claims

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 3.1 | Add the complete host-volume ACL capability model | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added the five host-volume namespace capabilities with the documented implications and read/write shorthands, and a table-driven allow/deny matrix |
| 3.2 | Implement node selection for volume creation | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.5, 2.2, 3.1 |
| 3.3 | Add scheduler feasibility checks for capabilities and access modes | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.5, 2.5, 3.1 |
| 3.4 | Add atomic writer and attachment accounting | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.3 |
| 3.5 | Mount selected dynamic paths and release attachments correctly | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 2.5, 3.4 |
| 3.6 | Implement sticky task-group claims | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.5, 3.4 |
| 3.7 | Add claim and volume garbage collection and unavailable-node handling | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 2.5, 3.5, 3.6 |

## Phase 4: Public APIs

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 4.1 | Implement HTTP list and read operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.6, 3.1 |
| 4.2 | Implement HTTP create, register, and update operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 2.6, 3.2, 3.3, 4.1 |
| 4.3 | Implement HTTP delete, deregister, and claim operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.6, 3.7, 4.2 |
| 4.4 | Add the complete typed Go API client | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.1, 4.2, 4.3 |

## Phase 5: Operator interfaces

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 5.1 | Implement CLI parsing, initialization, create, and register workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.4 |
| 5.2 | Implement CLI list, status, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.4, 5.1 |
| 5.3 | Add UI dynamic-volume list and detail pages | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.1, 4.4 |
| 5.4 | Add UI create, register, update, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.2, 4.3, 5.3 |

## Phase 6: Hardening and release

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 6.1 | Harden concurrency, failover, idempotency, and destructive operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.7, 4.3 |
| 6.2 | Add bounded metrics, diagnostics, and audit logging | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.3, 6.1 |
| 6.3 | Run synthetic end-to-end, namespace, ACL, restart, failover, upgrade, amd64, and arm64 verification | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: all implementation tasks |
| 6.4 | Complete provenance review, migration runbook, compatibility documentation, and release gate | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 6.3 |
