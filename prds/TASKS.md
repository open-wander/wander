# Norman Tasks

Task status lives only in this file and `TASKS-archive.md`. All implementation
must follow the clean-room and private-data rules in the linked PRD and
`docs/plans/license-safe-parity-method.md`.

## Phase 1: Clean-room foundation — 6 tasks completed 2026-11-15 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 2: Client and plugins — 6 tasks completed 2026-11-19 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 3: Scheduling and claims — 7 tasks completed 2026-11-22 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 3 follow-up: Owner decisions of 2026-11-22

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 3.8 | Never preempt the writer of a volume to place another writer of that volume | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-22) | The scheduler and the plan applier count an allocation the plan preempts as still holding its volume, through one shared helper; a stop in the job's own update still frees it |
| 3.9 | Keep a sticky claim with its volume while the client is disconnected | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-22) | An allocation in client status unknown no longer holds its sticky claim, so the claim is free and every placement of the group, the replacement included, waits for that volume; the writer rule still counts it |
| 3.10 | Keep a running task when its volume is not ready after an agent restart | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-22) | A task restored with a live driver handle keeps running when a requested host volume is still missing after the restore wait; a task that has to start, including a later restart, still fails |

## Phase 4: Server lifecycle and public APIs

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 4.1 | Let the ledger validator mark single facets of a row implemented | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-22) | An assertion may carry `implementation`, which needs a resolved assertion and implemented tests. DHV-B-167 states the scale-up rule and DHV-T-167 is implemented |
| 4.2 | Implement the server read RPCs and the HTTP list and read operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-25) | `HostVolume.List` and `HostVolume.Get`, `GET /v1/volumes?type=host` and `GET /v1/volume/host/:id`. host-volume-read checked after forwarding; a wildcard list shows only readable namespaces, also across pages. Policy in the provenance record, Read API |
| 4.3 | Add the Raft commands and state transitions of PRD section 6.4 | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-26) | Message types 100-104, transitions checked in the apply, `wander_version` serf tag and a gate at Wander 0.1.2 through `dynamicHostVolumeRaftApply`. Policy in the provenance record, Raft commands |
| 4.4 | Derive ready and unavailable from node updates | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-26) | Derived inside the node upsert, status and delete applies; down means down, disconnected or deleted; matching by volume ID. Must ship in the same release as the 0.1.2 gate. Policy in the provenance record, Raft commands |
| 4.5 | Re-check the volume in the plan applier at apply time | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-26) | Needs: 4.3. Closes the window between plan evaluation and a delete; rejects that node's part of the plan. Proven through real entry points; a source the node no longer advertises is now refused when a dynamic record exists |
| 4.6 | Implement create, register, and update: server RPCs, client calls, and HTTP operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 2.6, 3.2, 3.3, 4.2, 4.3. Includes the client RPC for create, leader retry of pending records, and ACL enforcement. Submit only through `dynamicHostVolumeRaftApply`. A repeated create on a deleting or failed volume is a no-op in the apply, so the RPC reads the state back. Update sends the stored server-owned fields unchanged, or the apply refuses it |
| 4.7 | Implement delete, deregister, and claim operations: server RPCs, client calls, and HTTP operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.6, 3.7, 4.6. Includes the client RPC for delete, the in-use refusal, force on a down or unknown node, and leader retry of deleting records |
| 4.8 | Filter dynamic host volumes in the node read API by namespace | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-26) | Needs: 3.1. PRD owner decisions, node read (US-014). Node get, node list and node pool list filtered by namespace; node events strip dynamic volumes for everyone (owner decision 2026-11-26) |
| 4.9 | Add the complete typed Go API client | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.2, 4.6, 4.7 |

## Phase 5: Operator interfaces

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 5.1 | Implement CLI parsing, initialization, create, and register workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.9 |
| 5.2 | Implement CLI list, status, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.9, 5.1 |
| 5.3 | Add UI dynamic-volume list and detail pages | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.2, 4.9 |
| 5.4 | Add UI create, register, update, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.6, 4.7, 5.3 |

## Phase 6: Hardening and release

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 6.1 | Harden concurrency, failover, idempotency, and destructive operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.7, 4.7. Includes a benchmark of sticky state reads for large groups and a bound on their cost |
| 6.2 | Add bounded metrics, diagnostics, and audit logging | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.7, 6.1 |
| 6.3 | Run synthetic end-to-end, namespace, ACL, restart, failover, upgrade, amd64, and arm64 verification | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: all implementation tasks |
| 6.4 | Complete provenance review, migration runbook, compatibility documentation, and release gate | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 6.3 |
